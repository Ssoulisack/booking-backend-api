package output

import (
	"backend/internal/core/domain"
	"context"
)

// SocialProfile is what a provider vouches for after checking the person's token
type SocialProfile struct {
	UID           string
	Email         string // empty when the person did not share one
	EmailVerified bool   // only a verified email may be matched to an existing account
	Name          string
}

// SocialVerifier checks a token from the browser with the provider and returns who it belongs to.
// It answers INVALID_SOCIAL_TOKEN-style errors for a bad or foreign token.
type SocialVerifier interface {
	Verify(ctx context.Context, provider, token string) (*SocialProfile, error)
	// Public returns the ids the login page needs; a provider that is not configured has an empty id
	Public() SocialPublic
}

// SocialPublic are the public ids of the configured providers (never the Facebook app secret)
type SocialPublic struct{ GoogleClientID, FacebookAppID string }

type SocialIdentityRepository interface {
	// Find returns (nil, nil) when the provider account is not linked to anyone yet
	Find(ctx context.Context, provider, uid string) (*domain.SocialIdentity, error)
	Create(ctx context.Context, identity *domain.SocialIdentity) error
}
