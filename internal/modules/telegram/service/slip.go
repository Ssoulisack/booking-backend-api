package service

import (
	"context"
	"errors"
	"strings"
)

// slipCaptionMax is Telegram's limit for a picture's caption
const slipCaptionMax = 1000

// SlipAvailable says whether the branch has a bot and a group to receive payment slips. The switch for messages
// (Enabled) does not matter here: a slip is the customer's way to pay, not a notice, so it keeps working while the
// notices are off.
func (s *Service) SlipAvailable(ctx context.Context, branchID uint) bool {
	cur, err := s.Repo.Get(ctx, branchID)
	return err == nil && cur != nil && cur.TokenEnc != "" && cur.ChatIDEnc != ""
}

// SendSlip puts a customer's payment slip (the picture) in the branch's group. The title and lines say what it
// is for (Lao, written by the caller); the customer's name and phone follow, the phone unless the branch hides it.
// An error means the team did not get it, and the customer is told to try again.
func (s *Service) SendSlip(ctx context.Context, branchID uint, title string, lines []string, name, phone string, photo []byte, filename string) error {
	cur, err := s.Repo.Get(ctx, branchID)
	if err != nil {
		return err
	}
	if cur == nil || cur.TokenEnc == "" || cur.ChatIDEnc == "" {
		return errors.New("telegram is not connected")
	}
	token, chat, err := s.open(cur)
	if err != nil {
		_ = s.Repo.Result(ctx, branchID, "The server secret changed: connect the bot again")
		return err
	}
	if cur.HidePhone {
		phone = ""
	}
	out := append([]string{title, ""}, lines...)
	if who := strings.TrimSpace(name + " " + phone); who != "" {
		out = append(out, "ລູກຄ້າ: "+who)
	}
	caption := truncate(strings.Join(out, "\n"), slipCaptionMax)
	if err := s.Bot.SendPhoto(ctx, token, chat, caption, photo, filename); err != nil {
		_ = s.Repo.Result(ctx, branchID, errorText(err))
		return errors.New(errorText(err))
	}
	return s.Repo.Result(ctx, branchID, "")
}

// SendBookingSlip puts a customer's payment slip for an event booking in the branch's group: the picture, with the
// same facts as the "slip sent" message as its caption (event, tables, amount, customer; no phone when hidden).
func (s *Service) SendBookingSlip(ctx context.Context, branchID, bookingID uint, photo []byte, filename string) error {
	cur, err := s.Repo.Get(ctx, branchID)
	if err != nil {
		return err
	}
	if cur == nil || cur.TokenEnc == "" || cur.ChatIDEnc == "" {
		return errors.New("telegram is not connected")
	}
	bk, err := s.Bookings.GetByID(ctx, branchID, bookingID)
	if err != nil || bk == nil {
		return errors.New("booking not found")
	}
	token, chat, err := s.open(cur)
	if err != nil {
		_ = s.Repo.Result(ctx, branchID, "The server secret changed: connect the bot again")
		return err
	}
	caption := truncate(compose(*bk, kindSlip, "", cur.HidePhone), slipCaptionMax)
	if err := s.Bot.SendPhoto(ctx, token, chat, caption, photo, filename); err != nil {
		_ = s.Repo.Result(ctx, branchID, errorText(err))
		return errors.New(errorText(err))
	}
	return s.Repo.Result(ctx, branchID, "")
}
