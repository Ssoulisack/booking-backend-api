package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
)

// Event days. The events module decides what is sold, at which price and for how long it is held;
// this file stores the booking like any other (who, code, private link, emails) and answers the
// customer's "I sent the slip".

// SlipSent is the customer's answer to "I paid and sent the slip" (by their private link): the hold no
// longer runs out by itself while the team checks the money
func (s *Service) SlipSent(ctx context.Context, slug, token string) (*dto.BookingDTO, error) {
	bk, err := s.guestBooking(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return s.markSlipSent(ctx, bk)
}

// MySlipSent is the same for a logged-in customer's own booking
func (s *Service) MySlipSent(ctx context.Context, userID, id uint) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetForCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.markSlipSent(ctx, bk)
}

func (s *Service) markSlipSent(ctx context.Context, bk *domain.Booking) (*dto.BookingDTO, error) {
	if bk.EventID == nil || bk.Status != domain.BookingPending || bk.PaymentStatus != domain.PaymentUnpaid {
		return nil, conflict("NOT_WAITING_FOR_PAYMENT")
	}
	if bk.SlipSentAt == nil {
		if err := s.bookings.SetSlipSent(ctx, bk, s.now()); err != nil {
			return nil, err
		}
	}
	d := toBookingDTO(*bk)
	return &d, nil
}
