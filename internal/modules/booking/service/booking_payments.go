package service

import (
	"backend/internal/core/domain"
	"context"
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
	return s.afterPaid(ctx, bk)
}
