// Package service: connecting a branch to a Telegram group, and telling the group what happens to bookings.
package service

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/telegram/domain"
	"backend/internal/modules/telegram/dto"
	"backend/internal/modules/telegram/port"
	"backend/internal/platform/secretbox"
	"backend/pkg/response"
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func badRequest(code string) error { return response.ErrorBadRequest(code) }
func conflict(code string) error   { return response.NewError(http.StatusConflict, code) }

var (
	tokenRe = regexp.MustCompile(`^\d{5,}:[A-Za-z0-9_-]{30,}$`)
	chatRe  = regexp.MustCompile(`^(-?\d{5,20}|@[A-Za-z][A-Za-z0-9_]{3,})$`)
)

// Deps are what the service is built from
type Deps struct {
	Repo     port.Repository
	Bot      port.Bot
	Box      *secretbox.Box
	Bookings output.BookingRepository
	Now      func() time.Time
}

// Service holds the use cases (it implements port.Service and core's output.TeamNotifier)
type Service struct{ Deps }

var (
	_ port.Service        = (*Service)(nil)
	_ output.TeamNotifier = (*Service)(nil)
)

// New builds the service
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{d}
}

// fresh is a setting for a branch that has none yet: everything on once it is connected
func fresh(branchID uint) *domain.Setting {
	return &domain.Setting{BranchID: branchID, Enabled: false, OnBooking: true, OnEvent: true, OnPayment: true, OnCancel: true}
}

func toDTO(s *domain.Setting) *dto.SettingDTO {
	return &dto.SettingDTO{
		HasBot: s.TokenEnc != "", BotName: s.BotName, HasGroup: s.ChatIDEnc != "", ChatTitle: s.ChatTitle,
		Enabled: s.Enabled && s.TokenEnc != "" && s.ChatIDEnc != "", OnBooking: s.OnBooking, OnEvent: s.OnEvent,
		OnPayment: s.OnPayment, OnCancel: s.OnCancel, HidePhone: s.HidePhone, LastError: s.LastError, LastOKAt: s.LastOKAt,
	}
}

func (s *Service) load(ctx context.Context, branchID uint) (*domain.Setting, error) {
	cur, err := s.Repo.Get(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		cur = fresh(branchID)
	}
	return cur, nil
}

func (s *Service) Get(ctx context.Context, branchID uint) (*dto.SettingDTO, error) {
	cur, err := s.load(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return toDTO(cur), nil
}

// open decrypts the token and the group id; both must be there
func (s *Service) open(cur *domain.Setting) (token, chat string, err error) {
	if cur.TokenEnc == "" {
		return "", "", conflict("TELEGRAM_BOT_REQUIRED")
	}
	if token, err = s.Box.Open(cur.TokenEnc); err != nil {
		return "", "", conflict("TELEGRAM_KEY_CHANGED") // the server secret changed: the bot must be connected again
	}
	if cur.ChatIDEnc != "" {
		if chat, err = s.Box.Open(cur.ChatIDEnc); err != nil {
			return "", "", conflict("TELEGRAM_KEY_CHANGED")
		}
	}
	return token, chat, nil
}

// telegramError turns what Telegram (or the network) said into one of our messages
func telegramError(err error, wrong string) error {
	var api *port.APIError
	if errors.As(err, &api) {
		switch api.Code {
		case http.StatusUnauthorized, http.StatusNotFound:
			return badRequest("TELEGRAM_BAD_TOKEN")
		case http.StatusConflict:
			return conflict("TELEGRAM_WEBHOOK_SET")
		case http.StatusBadRequest, http.StatusForbidden:
			return badRequest(wrong)
		}
	}
	return response.NewError(http.StatusBadGateway, "TELEGRAM_UNREACHABLE")
}

func (s *Service) Save(ctx context.Context, branchID uint, req dto.SaveRequest) (*dto.SettingDTO, error) {
	cur, err := s.load(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if req.BotToken != nil && strings.TrimSpace(*req.BotToken) != "" {
		token := strings.TrimSpace(*req.BotToken)
		if !tokenRe.MatchString(token) {
			return nil, badRequest("TELEGRAM_BAD_TOKEN_FORMAT")
		}
		name, err := s.Bot.Me(ctx, token)
		if err != nil {
			return nil, telegramError(err, "TELEGRAM_BAD_TOKEN")
		}
		if cur.TokenEnc, err = s.Box.Seal(token); err != nil {
			return nil, err
		}
		cur.BotName = name
	}
	if req.ChatID != nil && strings.TrimSpace(*req.ChatID) != "" {
		chat := strings.TrimSpace(*req.ChatID)
		if !chatRe.MatchString(chat) {
			return nil, badRequest("TELEGRAM_BAD_CHAT_FORMAT")
		}
		token, _, err := s.open(cur)
		if err != nil {
			return nil, err
		}
		title, err := s.Bot.Chat(ctx, token, chat)
		if err != nil {
			return nil, telegramError(err, "TELEGRAM_BAD_CHAT")
		}
		if cur.ChatIDEnc, err = s.Box.Seal(chat); err != nil {
			return nil, err
		}
		cur.ChatTitle = title
	}
	set := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	set(&cur.OnBooking, req.OnBooking)
	set(&cur.OnEvent, req.OnEvent)
	set(&cur.OnPayment, req.OnPayment)
	set(&cur.OnCancel, req.OnCancel)
	set(&cur.HidePhone, req.HidePhone)
	set(&cur.Enabled, req.Enabled)
	if cur.Enabled && (cur.TokenEnc == "" || cur.ChatIDEnc == "") {
		return nil, conflict("TELEGRAM_NOT_CONNECTED") // nothing to send to yet
	}
	if cur.TokenEnc == "" && cur.ChatIDEnc == "" && req.BotToken == nil {
		return toDTO(cur), nil // nothing was connected and nothing was asked for: no empty row
	}
	cur.LastError = ""
	if err := s.Repo.Save(ctx, cur); err != nil {
		return nil, err
	}
	return toDTO(cur), nil
}

func (s *Service) FindGroups(ctx context.Context, branchID uint) (*dto.GroupsDTO, error) {
	cur, err := s.load(ctx, branchID)
	if err != nil {
		return nil, err
	}
	token, chat, err := s.open(cur)
	if err != nil {
		return nil, err
	}
	found, err := s.Bot.FindGroups(ctx, token)
	if errors.Is(err, port.ErrNoGroup) {
		return nil, response.NewError(http.StatusNotFound, "TELEGRAM_NO_GROUP_FOUND")
	}
	if err != nil {
		return nil, telegramError(err, "TELEGRAM_UNREACHABLE")
	}
	out := &dto.GroupsDTO{Groups: make([]dto.GroupDTO, len(found))}
	for i, g := range found {
		out.Groups[i] = dto.GroupDTO{ID: g.ID, Title: g.Title, Current: chat != "" && g.ID == chat}
	}
	return out, nil
}

func (s *Service) Test(ctx context.Context, branchID uint) (*dto.SettingDTO, error) {
	cur, err := s.load(ctx, branchID)
	if err != nil {
		return nil, err
	}
	token, chat, err := s.open(cur)
	if err != nil {
		return nil, err
	}
	if chat == "" {
		return nil, conflict("TELEGRAM_NOT_CONNECTED")
	}
	text := "✅ ເຊື່ອມຕໍ່ Telegram ສຳເລັດ\nກຸ່ມນີ້ຈະໄດ້ຮັບຂໍ້ຄວາມການຈອງຈາກສາຂານີ້."
	if err := s.Bot.Send(ctx, token, chat, text); err != nil {
		_ = s.Repo.Result(ctx, branchID, errorText(err))
		return nil, telegramError(err, "TELEGRAM_BAD_CHAT")
	}
	_ = s.Repo.Result(ctx, branchID, "")
	return s.Get(ctx, branchID)
}

func (s *Service) Disconnect(ctx context.Context, branchID uint) error {
	return s.Repo.Delete(ctx, branchID)
}

// errorText is what the page shows after a failed message: Telegram's own words, never an address or a token
func errorText(err error) string {
	var api *port.APIError
	if errors.As(err, &api) {
		return truncate(api.Description, 250)
	}
	return "Telegram could not be reached from this server"
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---------- telling the team (output.TeamNotifier) ----------

type kind int

const (
	kindNew kind = iota
	kindSlip
	kindPaid
	kindCancelled
)

func (s *Service) NewBooking(ctx context.Context, branchID, bookingID uint) {
	s.later(ctx, branchID, bookingID, kindNew, "")
}
func (s *Service) SlipSent(ctx context.Context, branchID, bookingID uint) {
	s.later(ctx, branchID, bookingID, kindSlip, "")
}
func (s *Service) PaymentReceived(ctx context.Context, branchID, bookingID uint) {
	s.later(ctx, branchID, bookingID, kindPaid, "")
}
func (s *Service) Cancelled(ctx context.Context, branchID, bookingID uint, why string) {
	s.later(ctx, branchID, bookingID, kindCancelled, why)
}

// later does the work in the background: the booking never waits for Telegram
func (s *Service) later(ctx context.Context, branchID, bookingID uint, k kind, why string) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := s.deliver(ctx, branchID, bookingID, k, why); err != nil {
			log.Printf("⚠️  telegram message for booking %d failed: %v", bookingID, err)
		}
	}()
}

func (s *Service) deliver(ctx context.Context, branchID, bookingID uint, k kind, why string) error {
	cur, err := s.Repo.Get(ctx, branchID)
	if err != nil || cur == nil || !cur.Enabled || cur.TokenEnc == "" || cur.ChatIDEnc == "" {
		return err
	}
	bk, err := s.Bookings.GetByID(ctx, branchID, bookingID)
	if err != nil || bk == nil {
		return err
	}
	isEvent := bk.EventID != nil
	switch {
	case k == kindNew && isEvent && !cur.OnEvent, k == kindNew && !isEvent && !cur.OnBooking,
		(k == kindSlip || k == kindPaid) && !cur.OnPayment, k == kindCancelled && !cur.OnCancel:
		return nil
	}
	token, chat, err := s.open(cur)
	if err != nil {
		_ = s.Repo.Result(ctx, branchID, "The server secret changed: connect the bot again")
		return err
	}
	if err := s.Bot.Send(ctx, token, chat, compose(*bk, k, why, cur.HidePhone)); err != nil {
		_ = s.Repo.Result(ctx, branchID, errorText(err))
		return errors.New(errorText(err))
	}
	return s.Repo.Result(ctx, branchID, "")
}
