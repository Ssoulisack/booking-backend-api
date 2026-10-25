package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"errors"

	"gorm.io/gorm"
)

// socialRepository links provider accounts (Google, Facebook) to users
type socialRepository struct{ db *gorm.DB }

func NewSocialIdentityRepository(db *gorm.DB) output.SocialIdentityRepository {
	return &socialRepository{db: db}
}

func (r *socialRepository) Find(ctx context.Context, provider, uid string) (*domain.SocialIdentity, error) {
	var identity domain.SocialIdentity
	err := r.db.WithContext(ctx).Where("provider = ? AND provider_uid = ?", provider, uid).First(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &identity, nil
}

func (r *socialRepository) Create(ctx context.Context, identity *domain.SocialIdentity) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(identity).Error)
}
