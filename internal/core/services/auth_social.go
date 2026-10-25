package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"net/http"
	"strings"
	"time"
)

func (s *AuthServiceImpl) SocialConfig() dto.SocialConfigDTO {
	p := s.social.Public()
	return dto.SocialConfigDTO{GoogleClientID: p.GoogleClientID, FacebookAppID: p.FacebookAppID}
}

// SocialLogin signs a customer in with Google or Facebook.
//   - A provider account already linked to a user signs that user in.
//   - A verified provider email that matches an existing customer links to that account.
//   - An email that is not verified (Facebook) and matches an account is refused: the person logs
//     in with their password instead, so nobody can claim an account with an email they typed.
//   - Otherwise a new customer account is made. Without an email it gets a stand-in; the
//     nationality (and a Lao phone) are asked on the next screen, like after an invitation.
//
// The team (owner, admins, managers, staff) keeps its password: those accounts are refused.
func (s *AuthServiceImpl) SocialLogin(ctx context.Context, provider, token string) (*dto.AuthResponseDTO, error) {
	if !domain.ValidProvider(provider) {
		return nil, response.NewError(http.StatusNotFound, "SOCIAL_LOGIN_NOT_AVAILABLE")
	}
	profile, err := s.social.Verify(ctx, provider, token)
	if err != nil {
		return nil, err
	}

	if linked, err := s.socialIDs.Find(ctx, provider, profile.UID); err != nil {
		return nil, err
	} else if linked != nil {
		user, err := s.userRepo.FindByID(ctx, linked.UserID)
		if err != nil {
			return nil, err
		}
		return s.socialSignIn(ctx, user)
	}

	if profile.Email != "" {
		user, err := s.userRepo.FindByEmail(ctx, profile.Email)
		switch {
		case err == nil:
			if !profile.EmailVerified {
				return nil, conflict("EMAIL_ALREADY_REGISTERED")
			}
			if err := s.link(ctx, user, provider, profile.UID, profile.Email); err != nil {
				return nil, err
			}
			return s.socialSignIn(ctx, user)
		case !isNotFound(err):
			return nil, err
		}
	}

	user, err := s.newSocialUser(ctx, provider, profile)
	if err != nil {
		return nil, err
	}
	if err := s.link(ctx, user, provider, profile.UID, profile.Email); err != nil {
		return nil, err
	}
	return issueTokens(user)
}

// socialSignIn refuses disabled accounts and the team, then issues tokens
func (s *AuthServiceImpl) socialSignIn(ctx context.Context, user *domain.User) (*dto.AuthResponseDTO, error) {
	if !user.Status {
		return nil, forbidden("ACCOUNT_DISABLED")
	}
	if user.RunsCompany() {
		return nil, forbidden("SOCIAL_LOGIN_CUSTOMERS_ONLY")
	}
	memberships, err := s.memberships.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(memberships) > 0 {
		return nil, forbidden("SOCIAL_LOGIN_CUSTOMERS_ONLY")
	}
	return issueTokens(user)
}

func (s *AuthServiceImpl) link(ctx context.Context, user *domain.User, provider, uid, email string) error {
	// the team is refused before anything is linked
	if user.RunsCompany() {
		return forbidden("SOCIAL_LOGIN_CUSTOMERS_ONLY")
	}
	if memberships, err := s.memberships.ListByUser(ctx, user.ID); err != nil {
		return err
	} else if len(memberships) > 0 {
		return forbidden("SOCIAL_LOGIN_CUSTOMERS_ONLY")
	}
	return s.socialIDs.Create(ctx, &domain.SocialIdentity{UserID: user.ID, Provider: provider, ProviderUID: uid, Email: email})
}

// newSocialUser makes a customer without a password. Nationality stays empty on purpose: the
// "one more step" page asks for it (and a Lao phone) on the first visit.
func (s *AuthServiceImpl) newSocialUser(ctx context.Context, provider string, p *output.SocialProfile) (*domain.User, error) {
	email, generated := p.Email, false
	if email == "" {
		email, generated = domain.SocialEmail(provider, p.UID), true
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	user := &domain.User{
		Email:          email,
		Username:       email,
		Name:           name,
		EmailGenerated: generated,
		Verified:       p.EmailVerified,
		Status:         true,
		LastActive:     time.Now(),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	if err := syncMemberNo(ctx, s.userRepo, user); err != nil {
		return nil, err
	}
	return user, nil
}
