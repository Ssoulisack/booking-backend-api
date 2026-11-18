package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	iddomain "backend/internal/modules/identity/domain"
	"backend/internal/modules/identity/port"
	"backend/internal/platform/kit"
	"backend/internal/platform/sms"
	"backend/pkg/response"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	phoneCodeTTL   = 10 * time.Minute
	phoneCodeTries = 5
	// a new code for the same phone and purpose waits this long (each SMS costs money)
	phoneCodeGap = time.Minute
	// no link, domain or www in an SMS: the operator refuses them
	phoneCodeText = "Your verification code is %s. It expires in 10 minutes."
)

// PhoneCodeService sends the 6-digit SMS codes and checks them (register, reset by phone, guest bookings)
type PhoneCodeService struct {
	codes  port.PhoneCodeRepository
	users  output.UserRepository
	sender sms.Sender // nil: SMS codes are off
	now    func() time.Time
}

// NewPhoneCodeService: a nil sender switches SMS codes off
func NewPhoneCodeService(codes port.PhoneCodeRepository, users output.UserRepository, sender sms.Sender) *PhoneCodeService {
	return &PhoneCodeService{codes: codes, users: users, sender: sender, now: time.Now}
}

var _ output.PhoneCodes = (*PhoneCodeService)(nil)

// On reports SMS codes are switched on
func (s *PhoneCodeService) On() bool { return s != nil && s.sender != nil }

func phoneCodeHash(phone, purpose, code string) string {
	return kit.HashToken(fmt.Sprintf("%s:%s:%s", phone, purpose, code))
}

// Send texts a new code to the phone for the purpose. Register refuses a phone that already has an account (no SMS
// is paid for); reset answers the same whether or not the phone has an account, so it cannot reveal who is registered.
func (s *PhoneCodeService) Send(ctx context.Context, phoneIn, purpose string) error {
	if !s.On() {
		return badRequest("SMS_OFF")
	}
	if !domain.ValidPhonePurpose(purpose) {
		return badRequest("INVALID_PURPOSE")
	}
	phone, ok := domain.NormalizePhone(phoneIn)
	if !ok {
		return badRequest("INVALID_PHONE")
	}
	switch purpose {
	case domain.PhonePurposeRegister:
		taken, err := s.users.PhoneTaken(ctx, phone, 0)
		if err != nil {
			return err
		}
		if taken {
			return conflict("PHONE_ALREADY_REGISTERED")
		}
	case domain.PhonePurposeReset:
		u, err := s.users.FindByPhone(ctx, phone)
		if err != nil && !isNotFound(err) {
			return err
		}
		if u == nil || !u.Status {
			return nil // nobody to reset: the same answer, and no SMS
		}
	}
	now := s.now()
	if old, err := s.codes.Find(ctx, phone, purpose); err != nil {
		return err
	} else if old != nil && now.Sub(old.CreatedAt) < phoneCodeGap {
		return response.NewError(http.StatusTooManyRequests, "CODE_SENT_RECENTLY")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	rec := iddomain.PhoneCode{Phone: phone, Purpose: purpose, CodeHash: phoneCodeHash(phone, purpose, code),
		ExpiresAt: now.Add(phoneCodeTTL)}
	if err := s.codes.Replace(ctx, &rec); err != nil {
		return err
	}
	if err := s.sender.Send(ctx, phone, fmt.Sprintf(phoneCodeText, code)); err != nil {
		log.Printf("❌ SMS code (%s) to %s failed: %v", purpose, maskPhone(phone), err)
		_ = s.codes.Delete(ctx, phone, purpose)
		return response.NewError(http.StatusBadGateway, "SMS_SEND_FAILED")
	}
	return nil
}

// Check verifies a code; a right code is not used up (see Use)
func (s *PhoneCodeService) Check(ctx context.Context, phoneIn, purpose, code string) error {
	if !s.On() {
		return nil
	}
	phone, ok := domain.NormalizePhone(phoneIn)
	if !ok {
		return badRequest("INVALID_PHONE")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return badRequest("PHONE_CODE_REQUIRED")
	}
	rec, err := s.codes.Find(ctx, phone, purpose)
	if err != nil {
		return err
	}
	if rec == nil || s.now().After(rec.ExpiresAt) {
		return badRequest("CODE_EXPIRED")
	}
	if rec.Attempts >= phoneCodeTries {
		return badRequest("CODE_TOO_MANY_TRIES")
	}
	got := phoneCodeHash(phone, purpose, code)
	if subtle.ConstantTimeCompare([]byte(got), []byte(rec.CodeHash)) != 1 {
		if err := s.codes.AddAttempt(ctx, rec.ID); err != nil {
			return err
		}
		return badRequest("CODE_WRONG")
	}
	return nil
}

// Use ends the code after what it was for succeeded (an account made, a password reset)
func (s *PhoneCodeService) Use(ctx context.Context, phoneIn, purpose string) {
	if !s.On() {
		return
	}
	if phone, ok := domain.NormalizePhone(phoneIn); ok {
		_ = s.codes.Delete(ctx, phone, purpose)
	}
}

// maskPhone keeps the log free of whole numbers: 2055551234 → 20****1234
func maskPhone(p string) string {
	if len(p) < 6 {
		return "****"
	}
	return p[:2] + strings.Repeat("*", len(p)-6) + p[len(p)-4:]
}
