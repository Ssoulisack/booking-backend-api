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
	"sync"
	"time"
)

func badRequest(code string) error { return response.ErrorBadRequest(code) }
func conflict(code string) error   { return response.NewError(http.StatusConflict, code) }
func notFound(code string) error   { return response.NewError(http.StatusNotFound, code) }

const (
	defaultChargeMinutes = 30 // how long a customer has to pay a QR / page we created, unless the branch chose (30 = how long PhaJay keeps a QR payable)
	minChargeMinutes     = 5
	maxChargeMinutes     = 30
	pollAfter            = time.Minute      // a webhook gets this long before the status check steps in
	pollWindow           = 24 * time.Hour   // a payment is asked about until a day after its time ran out
	askEvery             = 10 * time.Second // the customer's page asks the gateway about one payment at most this often
)

// Deps are what the module is built from
type Deps struct {
	Repo      port.Repository
	Providers []port.Provider
	Branches  output.BranchRepository
	Bookings  port.Bookings
	// Purchases: course passes paid online (nil: not offered)
	Purchases port.Purchases
	Now       func() time.Time
}

// Service holds the payment rules
type Service struct {
	repo      port.Repository
	providers map[string]port.Provider
	branches  output.BranchRepository
	bookings  port.Bookings
	purchases port.Purchases
	now       func() time.Time

	// asked remembers when the gateway was last asked about a payment for a customer's page, so the page's
	// frequent checks are answered from our own data in between (the webhook keeps it current)
	askMu sync.Mutex
	asked map[uint]time.Time

	// startLocks: one start at a time for the same booking or package (striped by its key), so pressing Pay many
	// times at once finds the first one's open payment and does not make another. They live in this process: with
	// several API instances the same single-instance limit as the rate limiter applies.
	startLocks [256]sync.Mutex
}

var _ port.Service = (*Service)(nil)

// New builds the service
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	s := &Service{repo: d.Repo, providers: map[string]port.Provider{}, branches: d.Branches, bookings: d.Bookings, purchases: d.Purchases, now: d.Now, asked: map[uint]time.Time{}}
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

// toDTO shows a payment as the customer's page needs it: the QR or page to pay with, and when it runs out
func toDTO(p domain.Payment) dto.PaymentDTO {
	return dto.PaymentDTO{
		ID: p.ID, Provider: p.Provider, Method: p.Method, Status: p.Status, Amount: p.Amount, Currency: p.Currency,
		QRCode: p.QRCode, Deeplink: p.Deeplink, RedirectURL: p.RedirectURL, ExpiresAt: p.ExpiresAt,
	}
}

// seen is a payment as the customer's page gets it. Once our time for the QR is up the page is told it ran out
// and gets no QR or link, so a code is not paid for a booking we are about to release, even though the gateway
// keeps the code alive for a while longer (PhaJay: 30 minutes). The team's list shows the real state.
func (s *Service) seen(p domain.Payment) dto.PaymentDTO {
	d := toDTO(p)
	if p.Status == domain.StatusPending && !p.ExpiresAt.After(s.now()) {
		d.Status, d.QRCode, d.Deeplink, d.RedirectURL = domain.StatusExpired, "", "", ""
	}
	return d
}

func toStaffDTO(p domain.Payment) dto.StaffPaymentDTO {
	return dto.StaffPaymentDTO{
		PaymentDTO: toDTO(p), BookingID: p.BookingID, PurchaseID: p.PurchaseID, OrderNo: p.OrderNo, ProviderRef: p.ProviderRef,
		PaidAt: p.PaidAt, RefundRef: p.RefundRef, Note: p.Note, CreatedAt: p.CreatedAt,
	}
}

// ---------- customers ----------

// Options lists the gateways (and ways to pay) a branch's customers can use in the branch's currency
func (s *Service) Options(ctx context.Context, slug string, forPackage bool) ([]dto.OptionDTO, error) {
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
	// a sport branch can keep its bookings pay-at-the-venue while still selling packages online
	if forPackage {
		if ok, err := s.packagesOnline(ctx, b.ID); err != nil {
			return nil, err
		} else if !ok {
			return out, nil
		}
	} else if ok, err := s.bookingsOnline(ctx, b); err != nil {
		return nil, err
	} else if !ok {
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

// StartByToken and StartByCustomer make the QR or payment link of a booking (or return the open one)
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
	if f.Purchase { // a package: it has its own switch
		if ok, err := s.packagesOnline(ctx, f.BranchID); err != nil {
			return nil, err
		} else if !ok {
			return nil, conflict("ONLINE_PAYMENT_OFF")
		}
	} else { // a booking
		b, err := s.branches.GetByID(ctx, f.BranchID)
		if err != nil {
			return nil, err
		}
		if ok, err := s.bookingsOnline(ctx, b); err != nil {
			return nil, err
		} else if !ok {
			return nil, conflict("ONLINE_PAYMENT_OFF")
		}
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
	// the hold ran out (the countdown of the last QR, or the event's own hold): the booking is about to be
	// released, so no new QR is made for it, even before the job has cancelled it
	if f.PayBy != nil && !f.PayBy.After(now) {
		return nil, conflict("PAY_TIME_UP")
	}
	// from here to the end the same booking starts one payment at a time: the first creates the QR, the next ones
	// that were waiting here then see it open and get the same one
	unlock := s.lockStart(f)
	defer unlock()
	tries, err := s.triesOf(ctx, f)
	if err != nil {
		return nil, err
	}
	// a QR or payment link is open: the same bank gives the same one (pressing Pay twice, coming back to the
	// page: no new transaction at the gateway); any other way to pay is refused, the choice is final. The
	// customer who chose the wrong bank cancels the booking and books again. A try that ran out or failed
	// leaves the choice free.
	for i := range tries { // newest first
		o := &tries[i]
		if !o.Open(now) {
			continue
		}
		if o.Provider != req.Provider || o.Method != req.Method {
			return nil, conflict("PAYMENT_METHOD_LOCKED")
		}
		if err := s.hold(ctx, o, f); err != nil {
			return nil, err
		}
		d := s.seen(*o)
		return &d, nil
	}
	n, err := s.countOf(ctx, f)
	if err != nil {
		return nil, err
	}
	pay := &domain.Payment{
		BranchID: f.BranchID, Provider: req.Provider, Method: req.Method,
		OrderNo: fmt.Sprintf("%s-%d", orderBase(f), n+1), Amount: f.Total, Currency: f.Currency,
		Status: domain.StatusPending, ExpiresAt: now.Add(life),
	}
	if f.Purchase {
		pay.PurchaseID = f.ID
	} else {
		pay.BookingID = f.ID
	}
	if err := s.repo.Create(ctx, pay); err != nil {
		return nil, err
	}
	issued, err := p.Charge(ctx, domain.ChargeRequest{
		OrderNo: pay.OrderNo, Amount: pay.Amount, Currency: pay.Currency, Method: req.Method,
		Description: what(f) + " " + f.Code, Tags: [3]string{f.BranchSlug, fmt.Sprint(f.ID), fmt.Sprint(pay.ID)},
	})
	if err != nil {
		pay.Status, pay.Note = domain.StatusFailed, shorten(err.Error())
		_ = s.repo.Update(ctx, pay)
		log.Printf("⚠️  %s charge for booking %s failed: %v", req.Provider, f.Code, err)
		s.dropPurchase(ctx, pay, "ການຊຳລະອອນລາຍບໍ່ສຳເລັດ")
		// the gateway's own daily limit (a test account): say so, so the customer can pay another way or tomorrow
		if errors.Is(err, domain.ErrRateLimited) {
			return nil, response.NewError(http.StatusTooManyRequests, "PAYMENT_LIMIT_REACHED")
		}
		return nil, response.NewError(http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR")
	}
	pay.ProviderRef, pay.QRCode, pay.Deeplink, pay.RedirectURL = issued.ProviderRef, issued.QRCode, issued.Deeplink, issued.RedirectURL
	if err := s.repo.Update(ctx, pay); err != nil {
		return nil, err
	}
	if err := s.hold(ctx, pay, f); err != nil {
		return nil, err
	}
	d := s.seen(*pay)
	return &d, nil
}

// lockStart takes the start lock of the booking (or package) in f and returns how to let it go
func (s *Service) lockStart(f *coredomain.PayFacts) func() {
	kind := byte('b')
	if f.Purchase {
		kind = 'p'
	}
	m := &s.startLocks[(uint(f.ID)*31+uint(kind))%uint(len(s.startLocks))]
	m.Lock()
	return m.Unlock
}

// hold: an unpaid event booking is held exactly as long as its open QR lives: not paid by then, it is released
// (the customer may come back to the page until then)
func (s *Service) hold(ctx context.Context, pay *domain.Payment, f *coredomain.PayFacts) error {
	if f.PayBy != nil {
		if f.Purchase {
			return s.purchases.SetHold(ctx, f.ID, pay.ExpiresAt)
		}
		return s.bookings.SetHold(ctx, f.ID, pay.ExpiresAt)
	}
	return nil
}

// orderBase is what the order number starts with: the booking's code, or a package's branch-wide reference
func orderBase(f *coredomain.PayFacts) string {
	if f.OrderRef != "" {
		return f.OrderRef
	}
	return f.Code
}

// what names the thing being paid, for the gateway's description
func what(f *coredomain.PayFacts) string {
	if f.Purchase {
		return "Package"
	}
	return "Booking"
}

// triesOf and countOf: the payments made for the booking or package in f (newest first) and how many
func (s *Service) triesOf(ctx context.Context, f *coredomain.PayFacts) ([]domain.Payment, error) {
	if f.Purchase {
		return s.repo.ByPurchase(ctx, f.BranchID, f.ID)
	}
	return s.repo.ByBooking(ctx, f.BranchID, f.ID)
}

func (s *Service) countOf(ctx context.Context, f *coredomain.PayFacts) (int64, error) {
	if f.Purchase {
		return s.repo.CountByPurchase(ctx, f.ID)
	}
	return s.repo.CountByBooking(ctx, f.ID)
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
	list, err := s.triesOf(ctx, f)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range list { // newest first
		p := &list[i]
		if p.Open(now) {
			if p.ProviderRef != "" {
				p = s.refresh(ctx, p) // it may have been paid meanwhile
			}
			if p.Status == domain.StatusPending || p.Status == domain.StatusPaid {
				d := s.seen(*p)
				return &d, nil
			}
			continue
		}
		if p.Status == domain.StatusPaid {
			d := s.seen(*p)
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
	if p == nil || (f.Purchase && p.PurchaseID != f.ID) || (!f.Purchase && p.BookingID != f.ID) {
		return nil, notFound("PAYMENT_NOT_FOUND")
	}
	if p.Status == domain.StatusPending && p.ProviderRef != "" {
		p = s.refresh(ctx, p)
	}
	d := s.seen(*p)
	return &d, nil
}

// refresh asks the gateway about a payment and applies the answer; a gateway that cannot be reached changes
// nothing. The customer's page asks often, so the gateway is asked at most every askEvery per payment.
func (s *Service) refresh(ctx context.Context, p *domain.Payment) *domain.Payment {
	prov, ok := s.providers[p.Provider]
	if !ok || !s.mayAsk(p.ID) {
		return p
	}
	n, err := prov.Status(ctx, p.ProviderRef)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			log.Printf("⚠️  %s status of %s: %v", p.Provider, p.OrderNo, err)
		}
		return p
	}
	if err := s.apply(ctx, p, n); err != nil {
		log.Printf("⚠️  %s payment %s: %v", p.Provider, p.OrderNo, err)
	}
	return p
}

// mayAsk says whether the gateway may be asked about this payment now: not if it was asked less than askEvery ago
func (s *Service) mayAsk(paymentID uint) bool {
	s.askMu.Lock()
	defer s.askMu.Unlock()
	now := s.now()
	if last, ok := s.asked[paymentID]; ok && now.Sub(last) < askEvery {
		return false
	}
	if len(s.asked) > 1000 { // forget the old ones
		for id, at := range s.asked {
			if now.Sub(at) >= askEvery {
				delete(s.asked, id)
			}
		}
	}
	s.asked[paymentID] = now
	return true
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
	return s.Notify(ctx, provider, n)
}

// Notify applies what a gateway says about a payment (a verified webhook, or the dev mock's Pay button): the
// payment is found by the gateway's id, or by the order number we sent
func (s *Service) Notify(ctx context.Context, provider string, n *domain.Notice) error {
	p, err := s.repo.ByRef(ctx, provider, n.ProviderRef)
	if err != nil {
		return err
	}
	if p == nil { // a payment page: first seen by the order number, now we learn the gateway's id
		if p, err = s.repo.ByOrderNo(ctx, provider, n.OrderNo); err != nil {
			return err
		}
	}
	if p == nil {
		log.Printf("⚠️  %s webhook for an unknown payment (%s / %s)", provider, n.ProviderRef, n.OrderNo)
		return nil
	}
	return s.apply(ctx, p, n)
}

// apply moves a payment to what the gateway says. It is safe to apply the same event twice: money is never
// settled twice, and an amount that does not match the booking settles nothing. Money that arrives for a
// booking that is already paid (the customer paid an older QR too) or cancelled is marked to be given back.
func (s *Service) apply(ctx context.Context, p *domain.Payment, n *domain.Notice) error {
	if p.ProviderRef == "" && n.ProviderRef != "" {
		p.ProviderRef = n.ProviderRef
	}
	refundedNow := false // the bank just confirmed the refund: the customer is told once it is saved
	switch n.State {
	case domain.Paid:
		if p.Settled() {
			return nil
		}
		if n.Amount != p.Amount {
			p.Status = domain.StatusMismatch
			p.Note = shorten(fmt.Sprintf("gateway says %d, booking is %d", n.Amount, p.Amount))
			break
		}
		now := s.now()
		paidAt := now
		if !n.PaidAt.IsZero() {
			paidAt = n.PaidAt
		}
		p.PaidAt = &paidAt
		err := s.settle(ctx, p)
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
		refundedNow = true
	case domain.Disputed:
		if p.Status == domain.StatusDisputed {
			return nil
		}
		p.Status = domain.StatusDisputed
	default:
		// still waiting: only the gateway's id may be new
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return err
	}
	if p.Status == domain.StatusExpired {
		s.dropPurchase(ctx, p, "ໝົດເວລາຊຳລະ")
	}
	if refundedNow && p.PurchaseID == 0 {
		go s.bookings.NotifyRefund(context.WithoutCancel(ctx), p.BranchID, p.BookingID, p.Amount, p.Currency, true)
	}
	return nil
}

// dropPurchase cancels a customer package whose online payment failed or ran out, unless another payment of it is still
// open or already paid (the customer may have made a newer QR). A booking is not touched here: events release theirs.
func (s *Service) dropPurchase(ctx context.Context, p *domain.Payment, why string) {
	if p.PurchaseID == 0 || s.purchases == nil {
		return
	}
	list, err := s.repo.ByPurchase(ctx, p.BranchID, p.PurchaseID)
	if err != nil {
		log.Printf("⚠️  package %d: %v", p.PurchaseID, err)
		return
	}
	now := s.now()
	for _, o := range list {
		if o.ID != p.ID && (o.Open(now) || o.Settled()) {
			return
		}
	}
	if err := s.purchases.CancelUnpaid(ctx, p.BranchID, p.PurchaseID, why); err != nil {
		log.Printf("⚠️  cancelling unpaid package %d: %v", p.PurchaseID, err)
	}
}

// settle records the money on what it paid: the booking, or the customer's package
func (s *Service) settle(ctx context.Context, p *domain.Payment) error {
	if p.PurchaseID != 0 {
		if s.purchases == nil {
			return errors.New("packages are not connected")
		}
		return s.purchases.SettlePaid(ctx, p.BranchID, p.PurchaseID, p.OrderNo)
	}
	return s.bookings.SettlePaid(ctx, p.BranchID, p.BookingID, p.OrderNo)
}

// ---------- course passes (customer packages) ----------

func (s *Service) purchaseFacts(ctx context.Context, userID, id uint) (*coredomain.PayFacts, error) {
	if s.purchases == nil {
		return nil, notFound("PACKAGE_NOT_FOUND")
	}
	return s.purchases.PayFactsByCustomer(ctx, userID, id)
}

func (s *Service) StartPackage(ctx context.Context, userID, id uint, req dto.StartRequest) (*dto.PaymentDTO, error) {
	f, err := s.purchaseFacts(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.start(ctx, f, req)
}

func (s *Service) OpenPackage(ctx context.Context, userID, id uint) (*dto.PaymentDTO, error) {
	f, err := s.purchaseFacts(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.open(ctx, f)
}

func (s *Service) CheckPackage(ctx context.Context, userID, packageID, id uint) (*dto.PaymentDTO, error) {
	f, err := s.purchaseFacts(ctx, userID, packageID)
	if err != nil {
		return nil, err
	}
	return s.check(ctx, f, id)
}

// ForPurchase lists a customer package's payment attempts
func (s *Service) ForPurchase(ctx context.Context, branchID, purchaseID uint) ([]dto.StaffPaymentDTO, error) {
	list, err := s.repo.ByPurchase(ctx, branchID, purchaseID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.StaffPaymentDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toStaffDTO(p))
	}
	return out, nil
}

// Poll is the fallback for a webhook that never came: payments still waiting are asked about at the
// gateway, as are refunds still being made. Runs every few minutes.
func (s *Service) Poll(ctx context.Context) error {
	now := s.now()
	list, err := s.repo.Pending(ctx, now.Add(-pollAfter), now.Add(-pollWindow))
	if err != nil {
		return err
	}
	s.pollList(ctx, list, now)
	return nil
}

// PollOpen asks the gateway about the QRs that are still open (the customer may have paid and the webhook not come):
// a payment is then settled within seconds instead of at the next 5 minute check. Runs every 30 seconds; only open
// payments, so the gateway is not asked about old ones, and refunds are left to Poll.
func (s *Service) PollOpen(ctx context.Context) error {
	now := s.now()
	list, err := s.repo.Pending(ctx, now.Add(-15*time.Second), now)
	if err != nil {
		return err
	}
	open := list[:0]
	for _, p := range list {
		if p.Status == domain.StatusPending {
			open = append(open, p)
		}
	}
	s.pollList(ctx, open, now)
	return nil
}

// pollList asks the gateway about each payment and applies the answer
func (s *Service) pollList(ctx context.Context, list []domain.Payment, now time.Time) {
	for i := range list {
		p := list[i]
		prov, ok := s.providers[p.Provider]
		if !ok {
			continue
		}
		n, err := prov.Status(ctx, p.ProviderRef)
		if err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				log.Printf("⚠️  %s status of %s: %v", p.Provider, p.OrderNo, err)
			}
			continue
		}
		if err := s.apply(ctx, &p, n); err != nil {
			log.Printf("⚠️  %s payment %s: %v", p.Provider, p.OrderNo, err)
			continue
		}
		// the gateway still says "waiting" but our time for this QR is up: it is expired for us too
		if p.PurchaseID != 0 && p.Status == domain.StatusPending && !p.ExpiresAt.After(now) {
			p.Status = domain.StatusExpired
			if err := s.repo.Update(ctx, &p); err != nil {
				log.Printf("⚠️  %s payment %s: %v", p.Provider, p.OrderNo, err)
				continue
			}
			s.dropPurchase(ctx, &p, "ໝົດເວລາຊຳລະ")
		}
	}
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
	if p.PurchaseID == 0 {
		go s.bookings.NotifyRefund(context.WithoutCancel(ctx), p.BranchID, p.BookingID, p.Amount, p.Currency, false)
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

// bookingsOnline: a sport branch that closed online payment for bookings (the setting is only read for sport
// branches; restaurant events always follow the main switch)
func (s *Service) bookingsOnline(ctx context.Context, b *coredomain.Branch) (bool, error) {
	if b.Type != coredomain.BranchTypeSport {
		return true, nil
	}
	st, err := s.repo.Setting(ctx, b.ID)
	if err != nil {
		return false, err
	}
	return st == nil || !st.OnlineBookingsOff, nil
}

// packagesOnline: the branch sells packages with online payment
func (s *Service) packagesOnline(ctx context.Context, branchID uint) (bool, error) {
	st, err := s.repo.Setting(ctx, branchID)
	if err != nil {
		return false, err
	}
	return st == nil || !st.OnlinePackagesOff, nil
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
	st, err := s.repo.Setting(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return &dto.SettingsDTO{OnlineEnabled: on, OnlineBookings: st == nil || !st.OnlineBookingsOff, OnlinePackages: st == nil || !st.OnlinePackagesOff, GatewayAvailable: len(s.providers) > 0, ChargeMinutes: int(life / time.Minute)}, nil
}

func (s *Service) SaveSettings(ctx context.Context, branchID uint, req dto.SettingsRequest) (*dto.SettingsDTO, error) {
	minutes := req.ChargeMinutes
	if minutes == 0 {
		minutes = defaultChargeMinutes
	}
	if minutes < minChargeMinutes || minutes > maxChargeMinutes {
		return nil, badRequest("INVALID_PAYMENT_WINDOW")
	}
	// a switch that is not sent keeps what the branch chose before
	bookingsOff, packagesOff := false, false
	if prev, err := s.repo.Setting(ctx, branchID); err != nil {
		return nil, err
	} else if prev != nil {
		bookingsOff, packagesOff = prev.OnlineBookingsOff, prev.OnlinePackagesOff
	}
	if req.OnlineBookings != nil {
		bookingsOff = !*req.OnlineBookings
	}
	if req.OnlinePackages != nil {
		packagesOff = !*req.OnlinePackages
	}
	if err := s.repo.SaveSetting(ctx, &domain.Setting{BranchID: branchID, OnlineOff: !req.OnlineEnabled, OnlineBookingsOff: bookingsOff, OnlinePackagesOff: packagesOff, ChargeMinutes: minutes}); err != nil {
		return nil, err
	}
	return s.Settings(ctx, branchID)
}
