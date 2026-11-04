package port

import (
	"backend/internal/modules/identity/domain"
	"context"
	"time"
)

type InvitationRepository interface {
	Create(ctx context.Context, inv *domain.Invitation) error
	// GetByTokenHash returns (nil, nil) when no invitation matches
	GetByTokenHash(ctx context.Context, hash string) (*domain.Invitation, error)
	GetByID(ctx context.Context, id uint) (*domain.Invitation, error)
	// ListPending lists invitations not yet accepted and not expired at now
	ListPending(ctx context.Context, now time.Time) ([]domain.Invitation, error)
	PendingForEmail(ctx context.Context, email string, now time.Time) (*domain.Invitation, error)
	Update(ctx context.Context, inv *domain.Invitation) error
	Delete(ctx context.Context, id uint) error
}

type SocialIdentityRepository interface {
	// Find returns (nil, nil) when the provider account is not linked to anyone yet
	Find(ctx context.Context, provider, uid string) (*domain.SocialIdentity, error)
	Create(ctx context.Context, identity *domain.SocialIdentity) error
}
