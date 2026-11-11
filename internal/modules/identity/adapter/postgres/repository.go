// Package postgres: the identity module's tables in PostgreSQL — invitations and the links between a
// person and their Google / Facebook account.
package postgres

import (
	iddomain "backend/internal/modules/identity/domain"
	"backend/internal/modules/identity/port"
	"backend/internal/platform/postgres/exception"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

func firstOrNil[T any](tx *gorm.DB) (*T, error) {
	var m T
	err := tx.First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &m, nil
}

func requireAffected(res *gorm.DB) error {
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected == 0 {
		return exception.HandleGormError(gorm.ErrRecordNotFound)
	}
	return nil
}

// ---------- invitations ----------

type invitationRepository struct{ db *gorm.DB }

func NewInvitations(db *gorm.DB) port.InvitationRepository {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) pending(ctx context.Context, now time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&iddomain.Invitation{}).Where("accepted_at IS NULL AND expires_at > ?", now)
}

func (r *invitationRepository) Create(ctx context.Context, inv *iddomain.Invitation) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(inv).Error)
}

func (r *invitationRepository) GetByTokenHash(ctx context.Context, hash string) (*iddomain.Invitation, error) {
	return firstOrNil[iddomain.Invitation](r.db.WithContext(ctx).Where("token_hash = ?", hash))
}

func (r *invitationRepository) GetByID(ctx context.Context, id uint) (*iddomain.Invitation, error) {
	var inv iddomain.Invitation
	if err := r.db.WithContext(ctx).First(&inv, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &inv, nil
}

func (r *invitationRepository) ListPending(ctx context.Context, now time.Time) ([]iddomain.Invitation, error) {
	var items []iddomain.Invitation
	err := r.pending(ctx, now).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *invitationRepository) PendingForEmail(ctx context.Context, email string, now time.Time) (*iddomain.Invitation, error) {
	return firstOrNil[iddomain.Invitation](r.pending(ctx, now).Where("email = ?", email))
}

func (r *invitationRepository) Update(ctx context.Context, inv *iddomain.Invitation) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(inv).Error)
}

func (r *invitationRepository) Delete(ctx context.Context, id uint) error {
	return requireAffected(r.db.WithContext(ctx).Delete(&iddomain.Invitation{}, id))
}

// ---------- social identities ----------

// socialRepository links provider accounts (Google, Facebook) to users
type socialRepository struct{ db *gorm.DB }

func NewSocialIdentities(db *gorm.DB) port.SocialIdentityRepository {
	return &socialRepository{db: db}
}

func (r *socialRepository) Find(ctx context.Context, provider, uid string) (*iddomain.SocialIdentity, error) {
	var identity iddomain.SocialIdentity
	err := r.db.WithContext(ctx).Where("provider = ? AND provider_uid = ?", provider, uid).First(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &identity, nil
}

func (r *socialRepository) Create(ctx context.Context, identity *iddomain.SocialIdentity) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(identity).Error)
}

// ---------- email codes ----------

type emailCodeRepository struct{ db *gorm.DB }

func NewEmailCodes(db *gorm.DB) port.EmailCodeRepository { return &emailCodeRepository{db: db} }

func (r *emailCodeRepository) Replace(ctx context.Context, code *iddomain.EmailCode) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", code.UserID).Delete(&iddomain.EmailCode{}).Error; err != nil {
			return err
		}
		return tx.Create(code).Error
	}))
}

func (r *emailCodeRepository) Find(ctx context.Context, userID uint) (*iddomain.EmailCode, error) {
	return firstOrNil[iddomain.EmailCode](r.db.WithContext(ctx).Where("user_id = ?", userID))
}

func (r *emailCodeRepository) AddAttempt(ctx context.Context, id uint) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Model(&iddomain.EmailCode{}).Where("id = ?", id).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error)
}

func (r *emailCodeRepository) Delete(ctx context.Context, userID uint) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&iddomain.EmailCode{}).Error)
}
