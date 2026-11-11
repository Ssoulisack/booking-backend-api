package service

import (
	"backend/internal/core/dto"
	iddomain "backend/internal/modules/identity/domain"
	"backend/internal/platform/kit"
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
	emailCodeTTL   = 10 * time.Minute
	emailCodeTries = 5
)

// the code is stored hashed with the person and the email, so a code only works for that email
func emailCodeHash(userID uint, email, code string) string {
	return kit.HashToken(fmt.Sprintf("%d:%s:%s", userID, email, code))
}

// SendEmailCode emails a 6-digit code that proves the email is the person's (verify later is fine: until then the
// account shows "email not verified")
func (s *AuthServiceImpl) SendEmailCode(ctx context.Context, userID uint) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailGenerated {
		return badRequest("NO_EMAIL_TO_VERIFY")
	}
	if user.Verified {
		return badRequest("EMAIL_ALREADY_VERIFIED")
	}
	// how many codes a person may ask for is limited per window by the route (5 in 15 minutes)
	now := s.now()
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	rec := iddomain.EmailCode{UserID: user.ID, Email: user.Email, CodeHash: emailCodeHash(user.ID, user.Email, code),
		ExpiresAt: now.Add(emailCodeTTL)}
	if err := s.emailCodes.Replace(ctx, &rec); err != nil {
		return err
	}
	if err := s.mailer.SendEmailCode(user.Email, user.Name, code, s.frontendURL+"/account/profile"); err != nil {
		log.Printf("❌ email code to user %d failed: %v", user.ID, err)
		_ = s.emailCodes.Delete(ctx, user.ID)
		return response.NewError(http.StatusBadGateway, "EMAIL_SEND_FAILED")
	}
	return nil
}

// VerifyEmail checks the code: right → the email is marked verified
func (s *AuthServiceImpl) VerifyEmail(ctx context.Context, userID uint, code string) (*dto.UserDTO, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Verified && !user.EmailGenerated {
		d := kit.UserDTO(user)
		return &d, nil
	}
	rec, err := s.emailCodes.Find(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	// no code, a code for an email the account no longer has, or too old: ask for a new one
	if rec == nil || rec.Email != user.Email || s.now().After(rec.ExpiresAt) {
		return nil, badRequest("CODE_EXPIRED")
	}
	if rec.Attempts >= emailCodeTries {
		return nil, badRequest("CODE_TOO_MANY_TRIES")
	}
	got := emailCodeHash(user.ID, user.Email, strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(got), []byte(rec.CodeHash)) != 1 {
		if err := s.emailCodes.AddAttempt(ctx, rec.ID); err != nil {
			return nil, err
		}
		return nil, badRequest("CODE_WRONG")
	}
	if err := s.userRepo.SetVerified(ctx, user.ID, true); err != nil {
		return nil, err
	}
	_ = s.emailCodes.Delete(ctx, user.ID)
	user.Verified = true
	d := kit.UserDTO(user)
	return &d, nil
}
