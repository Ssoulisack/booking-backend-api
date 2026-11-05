package service

import (
	"backend/internal/core/domain"
	"context"
	"log"
	"time"
)

// What the payments module asks of the booking service: the facts about a booking, a longer hold
// while a customer pays online, and "this booking is paid" recorded once.

func factsOf(bk *domain.Booking) *domain.PayFacts {
	return &domain.PayFacts{
		ID: bk.ID, BranchID: bk.BranchID, BranchSlug: bk.Branch.Slug, Code: bk.Code, Total: bk.TotalPrice, Currency: bk.Currency,
		Status: bk.Status, PaymentStatus: bk.PaymentStatus, PayBy: bk.PayBy, SlipSent: bk.SlipSentAt != nil,
	}
}

// PayFactsByToken is for a guest, who owns the booking through its private link
func (s *Service) PayFactsByToken(ctx context.Context, slug, token string) (*domain.PayFacts, error) {
	bk, err := s.guestBooking(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return factsOf(bk), nil
}

// PayFactsByCustomer is for a logged-in customer's own booking
func (s *Service) PayFactsByCustomer(ctx context.Context, userID, id uint) (*domain.PayFacts, error) {
	bk, err := s.bookings.GetForCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return factsOf(bk), nil
}

// PayFactsByID is for the team and for a gateway's webhook (the payment already names the booking)
func (s *Service) PayFactsByID(ctx context.Context, branchID, id uint) (*domain.PayFacts, error) {
	bk, err := s.bookings.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	return factsOf(bk), nil
}

// SetHold makes an unpaid event booking's hold end when the customer's online payment (its QR) runs out;
// the one-minute job then cancels it if it was not paid
func (s *Service) SetHold(ctx context.Context, bookingID uint, until time.Time) error {
	return s.bookings.SetPayBy(ctx, bookingID, until)
}

// SettlePaid records that a booking was paid online. It happens once: a booking that is already paid
// (or was cancelled) is refused, and the payments module marks the money to be given back.
func (s *Service) SettlePaid(ctx context.Context, branchID, bookingID uint, ref string) error {
	bk, err := s.bookings.GetByID(ctx, branchID, bookingID)
	if err != nil {
		return err
	}
	switch {
	case bk.Status == domain.BookingCancelled:
		return domain.ErrNotPayable
	case bk.PaymentStatus == domain.PaymentPaid:
		return domain.ErrAlreadyPaid
	}
	now := s.now()
	bk.PaymentStatus, bk.PaymentMethod, bk.PaidAt, bk.PaidByID = domain.PaymentPaid, domain.MethodOnline, &now, nil
	if err := s.bookings.SetPayment(ctx, bk); err != nil {
		return err
	}
	if s.team != nil {
		s.team.PaymentReceived(ctx, branchID, bookingID)
	}
	return s.afterPaid(ctx, bk)
}

// customerOf gives where a booking's emails to the customer go: the account's email, or the one a guest gave
func customerOf(bk *domain.Booking) (to, name string) {
	to, name = bk.GuestEmail, bk.GuestName
	if bk.Customer != nil {
		to, name = bk.Customer.Email, bk.Customer.Name
	}
	return to, name
}

// BookingReleased emails the customer that their unpaid event booking was released (the events job calls it
// after cancelling it). No email on file: nothing to send. A failed email is logged and changes nothing.
func (s *Service) BookingReleased(ctx context.Context, branchID, bookingID uint) {
	if s.team != nil {
		s.team.Cancelled(ctx, branchID, bookingID, "unpaid")
	}
	bk, err := s.bookings.GetByID(ctx, branchID, bookingID)
	if err != nil || bk == nil {
		return
	}
	to, name := customerOf(bk)
	if to == "" {
		return
	}
	if err := s.mailer.SendBookingReleased(to, name, bk.Branch.Name, bk.Code, s.frontendURL+"/b/"+bk.Branch.Slug); err != nil {
		log.Printf("⚠️  released email for booking %s failed: %v", bk.Code, err)
	}
}

// NotifyRefund emails the customer that the bank was asked to refund a payment (done false) or confirmed it
// (done true). It runs after the answer is ready; a failed email is logged and changes nothing.
func (s *Service) NotifyRefund(ctx context.Context, branchID, bookingID uint, amount int64, currency string, done bool) {
	bk, err := s.bookings.GetByID(ctx, branchID, bookingID)
	if err != nil || bk == nil {
		return
	}
	to, name := customerOf(bk)
	if to == "" {
		return
	}
	if err := s.mailer.SendRefund(to, name, bk.Branch.Name, bk.Code, moneyText(amount, currency), s.frontendURL+"/b/"+bk.Branch.Slug, done); err != nil {
		log.Printf("⚠️  refund email for booking %s failed: %v", bk.Code, err)
	}
}
