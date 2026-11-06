package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/platform/kit"
	"backend/pkg/response"
	"context"
	"log"
	"net/http"
)

// Event days. The events module decides what is sold, at which price and for how long it is held;
// this file stores the booking like any other (who, code, private link, emails) and answers the
// customer's "I sent the slip".

// SlipSent is the customer's answer to "I paid and sent the slip" (by their private link): the hold no
// longer runs out by itself while the team checks the money. The slip is either sent on WhatsApp (nothing to
// upload) or a picture that goes to the branch's Telegram group.
func (s *Service) SlipSent(ctx context.Context, slug, token string, slip dto.SlipUpload) (*dto.BookingDTO, error) {
	bk, err := s.guestBooking(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return s.markSlipSent(ctx, bk, slip)
}

// MySlipSent is the same for a logged-in customer's own booking
func (s *Service) MySlipSent(ctx context.Context, userID, id uint, slip dto.SlipUpload) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetForCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.markSlipSent(ctx, bk, slip)
}

func (s *Service) markSlipSent(ctx context.Context, bk *domain.Booking, slip dto.SlipUpload) (*dto.BookingDTO, error) {
	if bk.EventID == nil || bk.Status != domain.BookingPending || bk.PaymentStatus != domain.PaymentUnpaid {
		return nil, conflict("NOT_WAITING_FOR_PAYMENT")
	}
	via := slip.Via
	if via == "" {
		via = dto.SlipWhatsapp
	}
	switch via {
	case dto.SlipWhatsapp:
		// nothing to upload: the customer says they sent it there. Said once.
		if bk.SlipSentAt != nil {
			d := toBookingDTO(*bk)
			return &d, nil
		}
	case dto.SlipTelegram:
		if err := s.sendSlipPicture(ctx, bk, slip); err != nil {
			return nil, err
		}
	default:
		return nil, badRequest("INVALID_SLIP_WAY")
	}
	if err := s.bookings.SetSlipSent(ctx, bk, s.now(), via); err != nil {
		return nil, err
	}
	// a WhatsApp slip is only a claim: tell the team to look. A picture in the group already says it.
	if via == dto.SlipWhatsapp && s.team != nil {
		s.team.SlipSent(ctx, bk.BranchID, bk.ID)
	}
	d := toBookingDTO(*bk)
	return &d, nil
}

// sendSlipPicture checks the way is on and the picture is real, then puts it in the branch's Telegram group. An
// error means the team did not get it: nothing is recorded and the customer may try again.
func (s *Service) sendSlipPicture(ctx context.Context, bk *domain.Booking, slip dto.SlipUpload) error {
	if s.slipWays == nil || s.slipDesk == nil {
		return conflict("SLIP_WAY_OFF")
	}
	if _, tg, err := s.slipWays.SlipWays(ctx, bk.BranchID); err != nil {
		return err
	} else if !tg {
		return conflict("SLIP_WAY_OFF") // the branch does not take slips that way (any more)
	}
	ext, err := kit.CheckSlipImage(slip.Data)
	if err != nil {
		return err
	}
	if err := s.slipDesk.SendBookingSlip(ctx, bk.BranchID, bk.ID, slip.Data, "slip-"+bk.Code+"."+ext); err != nil {
		log.Printf("⚠️  slip of booking %s was not delivered to Telegram: %v", bk.Code, err)
		return response.NewError(http.StatusBadGateway, "SLIP_NOT_DELIVERED")
	}
	return nil
}
