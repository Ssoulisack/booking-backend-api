package service

import (
	coredomain "backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/dto"
	"backend/internal/modules/payments/port"
	"backend/pkg/response"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
)

func badRequest(code string) error { return response.ErrorBadRequest(code) }
func conflict(code string) error   { return response.NewError(http.StatusConflict, code) }
func notFound(code string) error   { return response.NewError(http.StatusNotFound, code) }

const (
	chargeLife = 30 * time.Minute // how long a customer has to pay a QR / page we created
	holdSlack  = 5 * time.Minute  // the hold lasts a little longer than the charge
	pollAfter  = time.Minute      // a webhook gets this long before the status check steps in
)

// Deps are what the module is built from
type Deps struct {
	Repo      port.Repository
	Providers []port.Provider
	Branches  output.BranchRepository
	Bookings  port.Bookings
	Now       func() time.Time
}

// Service holds the payment rules
type Service struct {
	repo      port.Repository
	providers map[string]port.Provider
	branches  output.BranchRepository
	bookings  port.Bookings
	now       func() time.Time
}

var _ port.Service = (*Service)(nil)

// New builds the service
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	s := &Service{repo: d.Repo, providers: map[string]port.Provider{}, branches: d.Branches, bookings: d.Bookings, now: d.Now}
	for _, p := range d.Providers {
		if p != nil {
			s.providers[p.Name()] = p
		}
	}
	return s
}

// Enabled lists the gateways that are switched on
func (s *Service) Enabled() []string {
	var names []string
	for n := range s.providers {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

func toDTO(p domain.Payment) dto.PaymentDTO {
	return dto.PaymentDTO{
		ID: p.ID, Provider: p.Provider, Method: p.Method, Status: p.Status, Amount: p.Amount, Currency: p.Currency,
		QRCode: p.QRCode, Deeplink: p.Deeplink, RedirectURL: p.RedirectURL, ExpiresAt: p.ExpiresAt,
	}
}

func toStaffDTO(p domain.Payment) dto.StaffPaymentDTO {
	return dto.StaffPaymentDTO{
		PaymentDTO: toDTO(p), BookingID: p.BookingID, OrderNo: p.OrderNo, ProviderRef: p.ProviderRef,
		PaidAt: p.PaidAt, RefundRef: p.RefundRef, Note: p.Note, CreatedAt: p.CreatedAt,
	}
}

// ---------- customers ----------

// Options lists the gateways (and ways to pay) a branch's customers can use in the branch's currency
func (s *Service) Options(ctx context.Context, slug string) ([]dto.OptionDTO, error) {
	b, err := s.branches.GetBySlug(ctx, slug)
	if err != nil || !b.IsPublic() {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	out := []dto.OptionDTO{}
	for _, name := range s.Enabled() {
		p := s.providers[name]
		if m := p.Methods(b.Currency); len(m) > 0 {
			out = append(out, dto.OptionDTO{Provider: name, Label: p.Label(), Methods: m})
		}
	}
	return out, nil
}

// StartByToken and StartByCustomer create (or return the open) online payment of a booking
func (s *Service) StartByToken(ctx context.Context, slug, token string, req dto.StartRequest) (*dto.PaymentDTO, error) {
	f, err := s.bookings.PayFactsByToken(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return s.start(ctx, f, req)
}

func (s *Service) StartByCustomer(ctx context.Context, userID, id uint, req dto.StartRequest) (*dto.PaymentDTO, error) {
	f, err := s.bookings.PayFactsByCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.start(ctx, f, req)
}

func (s *Service) start(ctx context.Context, f *coredomain.PayFacts, req dto.StartRequest) (*dto.PaymentDTO, error) {
	p, ok := s.providers[req.Provider]
	if !ok {
		return nil, badRequest("INVALID_PAYMENT_PROVIDER")
	}
	if !f.Payable() {
		return nil, conflict("NOT_PAYABLE")
	}
	if !slices.ContainsFunc(p.Methods(f.Currency), func(m domain.Method) bool { return m.Key == req.Method }) {
		return nil, badRequest("INVALID_PAYMENT_METHOD")
	}
	now := s.now()
	// the customer pressed Pay twice, or came back to the page: the open payment is still good
	open, err := s.repo.ByBooking(ctx, f.BranchID, f.ID)
	if err != nil {
		return nil, err
	}
	for _, o := range open {
		if o.Status == domain.StatusPending && o.Provider == req.Provider && o.Method == req.Method && o.ExpiresAt.After(now) {
			d := toDTO(o)
			return &d, nil
		}
	}
	n, err := s.repo.CountByBooking(ctx, f.ID)
	if err != nil {
		return nil, err
	}
	pay := &domain.Payment{
		BranchID: f.BranchID, BookingID: f.ID, Provider: req.Provider, Method: req.Method,
		OrderNo: fmt.Sprintf("%s-%d", f.Code, n+1), Amount: f.Total, Currency: f.Currency,
		Status: domain.StatusPending, ExpiresAt: now.Add(chargeLife),
	}
	if err := s.repo.Create(ctx, pay); err != nil {
		return nil, err
	}
	charge, err := p.Charge(ctx, domain.ChargeRequest{
		OrderNo: pay.OrderNo, Amount: pay.Amount, Currency: pay.Currency, Method: req.Method,
		Description: "Booking " + f.Code, Tags: [3]string{f.BranchSlug, fmt.Sprint(f.ID), fmt.Sprint(pay.ID)},
	})
	if err != nil {
		pay.Status, pay.Note = domain.StatusFailed, shorten(err.Error())
		_ = s.repo.Update(ctx, pay)
		log.Printf("⚠️  %s charge for booking %s failed: %v", req.Provider, f.Code, err)
		return nil, response.NewError(http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR")
	}
	pay.ProviderRef, pay.QRCode, pay.Deeplink, pay.RedirectURL = charge.ProviderRef, charge.QRCode, charge.Deeplink, charge.RedirectURL
	if err := s.repo.Update(ctx, pay); err != nil {
		return nil, err
	}
	// an unpaid event booking would be released at its pay-by time: it waits while the customer pays
	if f.PayBy != nil {
		if err := s.bookings.ExtendHold(ctx, f.ID, pay.ExpiresAt.Add(holdSlack)); err != nil {
			return nil, err
		}
	}
	d := toDTO(*pay)
	return &d, nil
}

func shorten(s string) string {
	if len(s) > 250 {
		return s[:250]
	}
	return s
}

// CheckByToken and CheckByCustomer give the customer's page the payment's state; a payment still waiting
// is also asked about at the gateway, so the page does not depend on the webhook alone
func (s *Service) CheckByToken(ctx context.Context, slug, token string, id uint) (*dto.PaymentDTO, error) {
	f, err := s.bookings.PayFactsByToken(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return s.check(ctx, f, id)
}

func (s *Service) CheckByCustomer(ctx context.Context, userID, bookingID, id uint) (*dto.PaymentDTO, error) {
	f, err := s.bookings.PayFactsByCustomer(ctx, userID, bookingID)
	if err != nil {
		return nil, err
	}
	return s.check(ctx, f, id)
}

func (s *Service) check(ctx context.Context, f *coredomain.PayFacts, id uint) (*dto.PaymentDTO, error) {
	p, err := s.repo.Get(ctx, f.BranchID, id)
	if err != nil {
		return nil, err
	}
	if p == nil || p.BookingID != f.ID {
		return nil, notFound("PAYMENT_NOT_FOUND")
	}
	if p.Status == domain.StatusPending && p.ProviderRef != "" {
		s.refresh(ctx, p)
	}
	d := toDTO(*p)
	return &d, nil
}

// refresh asks the gateway and applies the answer; a gateway that cannot be reached changes nothing
func (s *Service) refresh(ctx context.Context, p *domain.Payment) {
	prov, ok := s.providers[p.Provider]
	if !ok {
		return
	}
	n, err := prov.Status(ctx, p.ProviderRef)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			log.Printf("⚠️  %s status of %s: %v", p.Provider, p.OrderNo, err)
		}
		return
	}
	if err := s.apply(ctx, p, n); err != nil {
		log.Printf("⚠️  %s payment %s: %v", p.Provider, p.OrderNo, err)
	}
}

// ---------- the gateway ----------

// HandleWebhook verifies and applies a gateway's event. An event that is not signed by the gateway
// returns domain.ErrBadSignature (the handler answers 401); anything else we cannot use is answered 200 and
// ignored, so the gateway does not keep resending it.
func (s *Service) HandleWebhook(ctx context.Context, provider string, header func(string) string, body []byte) error {
	p, ok := s.providers[provider]
	if !ok {
		return domain.ErrNotFound
	}
	n, err := p.VerifyWebhook(header, body)
	if err != nil {
		if errors.Is(err, domain.ErrBadSignature) {
			return err
		}
		log.Printf("⚠️  %s webhook unreadable: %v", provider, err)
		return nil
	}
	pay, err := s.repo.ByRef(ctx, provider, n.ProviderRef)
	if err != nil {
		return err
	}
	if pay == nil { // a payment page: first seen by the order number, now we learn the gateway's id
		if pay, err = s.repo.ByOrderNo(ctx, provider, n.OrderNo); err != nil {
			return err
		}
	}
	if pay == nil {
		log.Printf("⚠️  %s webhook for an unknown payment (%s / %s)", provider, n.ProviderRef, n.OrderNo)
		return nil
	}
	return s.apply(ctx, pay, n)
}

// apply moves a payment to what the gateway says. It is safe to apply the same event twice: money is
// never settled twice, and an amount that does not match the booking settles nothing.
func (s *Service) apply(ctx context.Context, p *domain.Payment, n *domain.Notice) error {
	if p.ProviderRef == "" && n.ProviderRef != "" {
		p.ProviderRef = n.ProviderRef
	}
	switch n.State {
	case domain.Paid:
		if p.Settled() {
			return nil
		}
		if n.Amount != p.Amount {
			p.Status = domain.StatusMismatch
			p.Note = shorten(fmt.Sprintf("gateway says %d, booking is %d", n.Amount, p.Amount))
			return s.repo.Update(ctx, p)
		}
		now := s.now()
		paidAt := now
		if !n.PaidAt.IsZero() {
			paidAt = n.PaidAt
		}
		p.PaidAt = &paidAt
		err := s.bookings.SettlePaid(ctx, p.BranchID, p.BookingID, p.OrderNo)
		switch {
		case err == nil:
			p.Status = domain.StatusPaid
		case errors.Is(err, coredomain.ErrNotPayable), errors.Is(err, coredomain.ErrAlreadyPaid):
			// the money arrived but the booking cannot take it: the team gives it back
			p.Status = domain.StatusRefundDue
			p.Note = shorten(err.Error())
		default:
			return err // try again at the next event or status check
		}
		return s.repo.Update(ctx, p)
	case domain.Expired:
		if p.Status != domain.StatusPending {
			return nil
		}
		p.Status = domain.StatusExpired
	case domain.Refunded:
		if p.Status == domain.StatusRefunded {
			return nil
		}
		p.Status = domain.StatusRefunded
	case domain.Disputed:
		if p.Status == domain.StatusDisputed {
			return nil
		}
		p.Status = domain.StatusDisputed
	default:
		return s.repo.Update(ctx, p) // still waiting: only the gateway's id may be new
	}
	return s.repo.Update(ctx, p)
}

// Poll is the fallback for a webhook that never came: payments still waiting are asked about at the
// gateway, as are refunds still being made. Runs every few minutes.
func (s *Service) Poll(ctx context.Context) error {
	list, err := s.repo.Pending(ctx, s.now().Add(-pollAfter))
	if err != nil {
		return err
	}
	for i := range list {
		s.refresh(ctx, &list[i])
	}
	return nil
}

// ---------- the team ----------

// ForBooking lists a booking's payment attempts
func (s *Service) ForBooking(ctx context.Context, branchID, bookingID uint) ([]dto.StaffPaymentDTO, error) {
	list, err := s.repo.ByBooking(ctx, branchID, bookingID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.StaffPaymentDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toStaffDTO(p))
	}
	return out, nil
}

// Attention lists what needs a person: money to give back, amounts that did not match, disputes and
// refunds in progress
func (s *Service) Attention(ctx context.Context, branchID uint) ([]dto.StaffPaymentDTO, error) {
	list, err := s.repo.Attention(ctx, branchID, []string{domain.StatusRefundDue, domain.StatusMismatch, domain.StatusDisputed, domain.StatusRefundRequested})
	if err != nil {
		return nil, err
	}
	out := make([]dto.StaffPaymentDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toStaffDTO(p))
	}
	return out, nil
}

// Refund asks the gateway to give the money back. The booking is cancelled separately (the team may
// want to keep it); the payment moves to "refunded" when the gateway confirms.
func (s *Service) Refund(ctx context.Context, branchID, id uint) (*dto.StaffPaymentDTO, error) {
	p, err := s.repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, notFound("PAYMENT_NOT_FOUND")
	}
	if p.Status != domain.StatusPaid && p.Status != domain.StatusRefundDue {
		return nil, conflict("NOT_REFUNDABLE")
	}
	prov, ok := s.providers[p.Provider]
	if !ok {
		return nil, conflict("PAYMENT_PROVIDER_OFF")
	}
	ref, err := prov.Refund(ctx, p.ProviderRef)
	if err != nil {
		if errors.Is(err, domain.ErrUnsupported) {
			return nil, conflict("REFUND_NOT_SUPPORTED")
		}
		log.Printf("⚠️  %s refund of %s: %v", p.Provider, p.OrderNo, err)
		return nil, response.NewError(http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR")
	}
	p.Status, p.RefundRef = domain.StatusRefundRequested, strings.TrimSpace(ref)
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	d := toStaffDTO(*p)
	return &d, nil
}
