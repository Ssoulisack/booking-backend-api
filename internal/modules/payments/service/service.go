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
	defaultChargeMinutes = 30 // how long a customer has to pay a QR / page we created, unless the branch chose
	minChargeMinutes     = 5
	maxChargeMinutes     = 120
	pollAfter            = time.Minute    // a webhook gets this long before the status check steps in
	pollWindow           = 24 * time.Hour // a charge is asked about until a day after its time ran out
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

// toDTO shows a payment through its current charge: the QR or page to pay with, and when it runs out
func toDTO(p domain.Payment) dto.PaymentDTO {
	c := p.Current()
	return dto.PaymentDTO{
		ID: p.ID, Provider: c.Provider, Method: c.Method, Status: p.Status, Amount: p.Amount, Currency: p.Currency,
		QRCode: c.QRCode, Deeplink: c.Deeplink, RedirectURL: c.RedirectURL, ExpiresAt: c.ExpiresAt,
	}
}

func toStaffDTO(p domain.Payment) dto.StaffPaymentDTO {
	charges := make([]dto.ChargeDTO, 0, len(p.Charges))
	for _, c := range p.Charges {
		charges = append(charges, dto.ChargeDTO{
			ID: c.ID, Provider: c.Provider, Method: c.Method, Status: c.Status, ProviderRef: c.ProviderRef,
			ExpiresAt: c.ExpiresAt, CreatedAt: c.CreatedAt,
		})
	}
	return dto.StaffPaymentDTO{
		PaymentDTO: toDTO(p), BookingID: p.BookingID, OrderNo: p.OrderNo, ProviderRef: p.Current().ProviderRef,
		PaidAt: p.PaidAt, RefundRef: p.RefundRef, Note: p.Note, CreatedAt: p.CreatedAt, Charges: charges,
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
	// the branch turned online payment off: customers only see the branch's own way to pay
	if on, err := s.onlineOn(ctx, b.ID); err != nil {
		return nil, err
	} else if !on {
		return out, nil
	}
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
	on, life, err := s.setting(ctx, f.BranchID)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, conflict("ONLINE_PAYMENT_OFF")
	}
	// the customer already sent a slip for this booking: they do not pay a second way, the team checks the slip
	if f.SlipSent {
		return nil, conflict("SLIP_ALREADY_SENT")
	}
	if !f.Payable() {
		return nil, conflict("NOT_PAYABLE")
	}
	if !slices.ContainsFunc(p.Methods(f.Currency), func(m domain.Method) bool { return m.Key == req.Method }) {
		return nil, badRequest("INVALID_PAYMENT_METHOD")
	}
	now := s.now()
	open, err := s.repo.ByBooking(ctx, f.BranchID, f.ID)
	if err != nil {
		return nil, err
	}
	// the booking's payment still being paid (or that ran out of time): another bank is one more charge of it
	var pay *domain.Payment
	for i := range open { // newest first
		o := &open[i]
		if (o.Status == domain.StatusPending || o.Status == domain.StatusExpired || o.Status == domain.StatusFailed) &&
			o.Amount == f.Total && o.Currency == f.Currency {
			pay = o
			break
		}
	}
	if pay != nil {
		// the customer pressed Pay twice, came back to the page, or went back to a bank they already chose:
		// that charge is still good, so no new one is made at the gateway
		if c := pay.Live(req.Provider, req.Method, now); c != nil && pay.Status == domain.StatusPending {
			if err := s.use(ctx, pay, c, f); err != nil {
				return nil, err
			}
			d := toDTO(*pay)
			return &d, nil
		}
	} else {
		n, err := s.repo.CountByBooking(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		pay = &domain.Payment{
			BranchID: f.BranchID, BookingID: f.ID, OrderNo: fmt.Sprintf("%s-%d", f.Code, n+1),
			Amount: f.Total, Currency: f.Currency, Status: domain.StatusPending,
		}
		if err := s.repo.CreatePayment(ctx, pay); err != nil {
			return nil, err
		}
	}
	// the gateway sees a unique order number for every charge: ours, then ours-2, ours-3…
	gatewayOrder := pay.OrderNo
	if len(pay.Charges) > 0 {
		gatewayOrder = fmt.Sprintf("%s-%d", pay.OrderNo, len(pay.Charges)+1)
	}
	c := &domain.Charge{
		PaymentID: pay.ID, Provider: req.Provider, Method: req.Method, GatewayOrderNo: gatewayOrder,
		Status: domain.ChargePending, ExpiresAt: now.Add(life),
	}
	if err := s.repo.CreateCharge(ctx, c); err != nil {
		return nil, err
	}
	issued, err := p.Charge(ctx, domain.ChargeRequest{
		OrderNo: gatewayOrder, Amount: pay.Amount, Currency: pay.Currency, Method: req.Method,
		Description: "Booking " + f.Code, Tags: [3]string{f.BranchSlug, fmt.Sprint(f.ID), fmt.Sprint(pay.ID)},
	})
	if err != nil {
		c.Status = domain.ChargeFailed
		_ = s.repo.SaveCharge(ctx, c)
		// a charge still open keeps the payment as it was; with none, the payment itself failed
		if !pay.AnyLive(c.ID, now) {
			pay.Status, pay.Note = domain.StatusFailed, shorten(err.Error())
			_ = s.repo.SavePayment(ctx, pay)
		}
		log.Printf("⚠️  %s charge for booking %s failed: %v", req.Provider, f.Code, err)
		// the gateway's own daily limit (a test account): say so, so the customer can pay another way or tomorrow
		if errors.Is(err, domain.ErrRateLimited) {
			return nil, response.NewError(http.StatusTooManyRequests, "PAYMENT_LIMIT_REACHED")
		}
		return nil, response.NewError(http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR")
	}
	c.ProviderRef, c.QRCode, c.Deeplink, c.RedirectURL = issued.ProviderRef, issued.QRCode, issued.Deeplink, issued.RedirectURL
	if err := s.repo.SaveCharge(ctx, c); err != nil {
		return nil, err
	}
	pay.Charges = append(pay.Charges, *c)
	pay.Status, pay.Note = domain.StatusPending, ""
	if err := s.use(ctx, pay, c, f); err != nil {
		return nil, err
	}
	d := toDTO(*pay)
	return &d, nil
}

// use makes the charge the one the customer is paying now. An unpaid event booking is held exactly as long
// as that charge lives: not paid by then, it is released (the customer may come back until then).
func (s *Service) use(ctx context.Context, pay *domain.Payment, c *domain.Charge, f *coredomain.PayFacts) error {
	pay.ChargeID = &c.ID
	if err := s.repo.SavePayment(ctx, pay); err != nil {
		return err
	}
	if f.PayBy != nil {
		return s.bookings.SetHold(ctx, f.ID, c.ExpiresAt)
	}
	return nil
}

func shorten(s string) string {
	if len(s) > 250 {
		return s[:250]
	}
	return s
}

// OpenByToken and OpenByCustomer give the customer's page the payment they can still carry on with: the
// newest one that is waiting and has time left (the same QR as before), or one that just got paid; nil when none
func (s *Service) OpenByToken(ctx context.Context, slug, token string) (*dto.PaymentDTO, error) {
	f, err := s.bookings.PayFactsByToken(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return s.open(ctx, f)
}

func (s *Service) OpenByCustomer(ctx context.Context, userID, id uint) (*dto.PaymentDTO, error) {
	f, err := s.bookings.PayFactsByCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.open(ctx, f)
}

func (s *Service) open(ctx context.Context, f *coredomain.PayFacts) (*dto.PaymentDTO, error) {
	list, err := s.repo.ByBooking(ctx, f.BranchID, f.ID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range list { // newest first
		p := &list[i]
		if p.Status == domain.StatusPending {
			if c := p.Current(); c.Status == domain.ChargePending && c.ExpiresAt.After(now) {
				if c.ProviderRef != "" {
					p = s.refresh(ctx, p, c) // it may have been paid meanwhile
				}
				if p.Status == domain.StatusPending || p.Status == domain.StatusPaid {
					d := toDTO(*p)
					return &d, nil
				}
				continue
			}
		}
		if p.Status == domain.StatusPaid {
			d := toDTO(*p)
			return &d, nil
		}
	}
	return nil, nil
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
	if c := p.Current(); p.Status == domain.StatusPending && c.Status == domain.ChargePending && c.ProviderRef != "" {
		p = s.refresh(ctx, p, c)
	}
	d := toDTO(*p)
	return &d, nil
}

// refresh asks the gateway about a charge and applies the answer, and gives back the payment as it is
// now; a gateway that cannot be reached changes nothing
func (s *Service) refresh(ctx context.Context, p *domain.Payment, c domain.Charge) *domain.Payment {
	prov, ok := s.providers[c.Provider]
	if !ok {
		return p
	}
	n, err := prov.Status(ctx, c.ProviderRef)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			log.Printf("⚠️  %s status of %s: %v", c.Provider, c.GatewayOrderNo, err)
		}
		return p
	}
	if err := s.apply(ctx, &c, n); err != nil {
		log.Printf("⚠️  %s payment %s: %v", c.Provider, c.GatewayOrderNo, err)
		return p
	}
	if fresh, err := s.repo.Find(ctx, p.ID); err == nil && fresh != nil {
		return fresh
	}
	return p
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
	c, err := s.repo.ChargeByRef(ctx, provider, n.ProviderRef)
	if err != nil {
		return err
	}
	if c == nil { // a payment page: first seen by the order number, now we learn the gateway's id
		if c, err = s.repo.ChargeByOrderNo(ctx, provider, n.OrderNo); err != nil {
			return err
		}
	}
	if c == nil {
		log.Printf("⚠️  %s webhook for an unknown payment (%s / %s)", provider, n.ProviderRef, n.OrderNo)
		return nil
	}
	return s.apply(ctx, c, n)
}

// apply moves a charge, and the payment it belongs to, to what the gateway says. It is safe to apply the
// same event twice: money is never settled twice, and an amount that does not match the booking settles
// nothing.
func (s *Service) apply(ctx context.Context, c *domain.Charge, n *domain.Notice) error {
	if c.ProviderRef == "" && n.ProviderRef != "" {
		c.ProviderRef = n.ProviderRef
		if err := s.repo.SaveCharge(ctx, c); err != nil {
			return err
		}
	}
	p, err := s.repo.Find(ctx, c.PaymentID)
	if err != nil || p == nil {
		return err
	}
	switch n.State {
	case domain.Paid:
		if c.Status == domain.ChargePaid {
			return nil
		}
		if p.Settled() { // another charge of this payment already took the money: this one is to give back
			return s.paidTwice(ctx, p, c, n)
		}
		if n.Amount != p.Amount {
			p.Status = domain.StatusMismatch
			p.Note = shorten(fmt.Sprintf("gateway says %d, booking is %d", n.Amount, p.Amount))
			return s.repo.SavePayment(ctx, p)
		}
		now := s.now()
		paidAt := now
		if !n.PaidAt.IsZero() {
			paidAt = n.PaidAt
		}
		p.PaidAt, p.ChargeID = &paidAt, &c.ID
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
		c.Status = domain.ChargePaid
		if err := s.repo.SaveCharge(ctx, c); err != nil {
			return err
		}
		return s.repo.SavePayment(ctx, p)
	case domain.Expired:
		if c.Status != domain.ChargePending {
			return nil
		}
		c.Status = domain.ChargeExpired
		if err := s.repo.SaveCharge(ctx, c); err != nil {
			return err
		}
		// the payment runs out only when no other bank's charge can still be paid
		if p.Status != domain.StatusPending || p.AnyLive(c.ID, s.now()) {
			return nil
		}
		p.Status = domain.StatusExpired
	case domain.Refunded:
		if p.ChargeID == nil || *p.ChargeID != c.ID || p.Status == domain.StatusRefunded {
			return nil
		}
		p.Status = domain.StatusRefunded
	case domain.Disputed:
		if p.ChargeID == nil || *p.ChargeID != c.ID || p.Status == domain.StatusDisputed {
			return nil
		}
		p.Status = domain.StatusDisputed
	default:
		return nil // still waiting: only the gateway's id may be new
	}
	return s.repo.SavePayment(ctx, p)
}

// paidTwice: money arrived on a charge after another charge of the payment was paid (or the payment was
// already given back). That money becomes a payment of its own, waiting to be given back.
func (s *Service) paidTwice(ctx context.Context, p *domain.Payment, c *domain.Charge, n *domain.Notice) error {
	paidAt := s.now()
	if !n.PaidAt.IsZero() {
		paidAt = n.PaidAt
	}
	amount := n.Amount
	if amount <= 0 {
		amount = p.Amount
	}
	c.Status = domain.ChargePaid
	return s.repo.SplitOff(ctx, c, &domain.Payment{
		BranchID: p.BranchID, BookingID: p.BookingID, OrderNo: fmt.Sprintf("%s-R%d", p.OrderNo, c.ID),
		Amount: amount, Currency: p.Currency, Status: domain.StatusRefundDue, PaidAt: &paidAt, ChargeID: &c.ID,
		Note: "paid a second time",
	})
}

// Poll is the fallback for a webhook that never came: payments still waiting are asked about at the
// gateway, as are refunds still being made. Runs every few minutes.
func (s *Service) Poll(ctx context.Context) error {
	now := s.now()
	list, err := s.repo.Pending(ctx, now.Add(-pollAfter), now.Add(-pollWindow))
	if err != nil {
		return err
	}
	for i := range list {
		c := list[i]
		if prov, ok := s.providers[c.Provider]; ok {
			if n, err := prov.Status(ctx, c.ProviderRef); err == nil {
				if err := s.apply(ctx, &c, n); err != nil {
					log.Printf("⚠️  %s payment %s: %v", c.Provider, c.GatewayOrderNo, err)
				}
			} else if !errors.Is(err, domain.ErrNotFound) {
				log.Printf("⚠️  %s status of %s: %v", c.Provider, c.GatewayOrderNo, err)
			}
		}
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
	paid := p.Current() // the charge that took the money
	prov, ok := s.providers[paid.Provider]
	if !ok {
		return nil, conflict("PAYMENT_PROVIDER_OFF")
	}
	ref, err := prov.Refund(ctx, paid.ProviderRef)
	if err != nil {
		if errors.Is(err, domain.ErrUnsupported) {
			return nil, conflict("REFUND_NOT_SUPPORTED")
		}
		log.Printf("⚠️  %s refund of %s: %v", paid.Provider, p.OrderNo, err)
		return nil, response.NewError(http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR")
	}
	p.Status, p.RefundRef = domain.StatusRefundRequested, strings.TrimSpace(ref)
	if err := s.repo.SavePayment(ctx, p); err != nil {
		return nil, err
	}
	d := toStaffDTO(*p)
	return &d, nil
}

// setting reads the branch's choice: online payment on (the default until it turns it off) and how long a
// customer has to pay
func (s *Service) setting(ctx context.Context, branchID uint) (bool, time.Duration, error) {
	st, err := s.repo.Setting(ctx, branchID)
	if err != nil {
		return false, 0, err
	}
	if st == nil {
		return true, defaultChargeMinutes * time.Minute, nil
	}
	minutes := st.ChargeMinutes
	if minutes <= 0 {
		minutes = defaultChargeMinutes
	}
	return !st.OnlineOff, time.Duration(minutes) * time.Minute, nil
}

// onlineOn: the branch takes online payment
func (s *Service) onlineOn(ctx context.Context, branchID uint) (bool, error) {
	on, _, err := s.setting(ctx, branchID)
	return on, err
}

// Settings is what the branch chose, and whether a gateway is switched on at all
func (s *Service) Settings(ctx context.Context, branchID uint) (*dto.SettingsDTO, error) {
	on, life, err := s.setting(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return &dto.SettingsDTO{OnlineEnabled: on, GatewayAvailable: len(s.providers) > 0, ChargeMinutes: int(life / time.Minute)}, nil
}

func (s *Service) SaveSettings(ctx context.Context, branchID uint, req dto.SettingsRequest) (*dto.SettingsDTO, error) {
	minutes := req.ChargeMinutes
	if minutes == 0 {
		minutes = defaultChargeMinutes
	}
	if minutes < minChargeMinutes || minutes > maxChargeMinutes {
		return nil, badRequest("INVALID_PAYMENT_WINDOW")
	}
	if err := s.repo.SaveSetting(ctx, &domain.Setting{BranchID: branchID, OnlineOff: !req.OnlineEnabled, ChargeMinutes: minutes}); err != nil {
		return nil, err
	}
	return s.Settings(ctx, branchID)
}
