package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// firstOrNil maps "not found" to (nil, nil) for lookups where absence is a normal answer
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

// ---------- company ----------

type companyRepository struct{ db *gorm.DB }

func NewCompanyRepository(db *gorm.DB) output.CompanyRepository { return &companyRepository{db: db} }

func (r *companyRepository) Get(ctx context.Context) (*domain.Company, error) {
	var c domain.Company
	if err := r.db.WithContext(ctx).Order("id ASC").First(&c).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &c, nil
}

func (r *companyRepository) Create(ctx context.Context, c *domain.Company) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(c).Error)
}

func (r *companyRepository) Update(ctx context.Context, c *domain.Company) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(c).Error)
}

// ---------- branches ----------

type branchRepository struct{ db *gorm.DB }

func NewBranchRepository(db *gorm.DB) output.BranchRepository { return &branchRepository{db: db} }

// withCompany fills the settings in force of loaded branches from the one company row
func (r *branchRepository) withCompany(ctx context.Context, branches ...*domain.Branch) error {
	var c domain.Company
	if err := r.db.WithContext(ctx).Order("id ASC").First(&c).Error; err != nil {
		return exception.HandleGormError(err)
	}
	for _, b := range branches {
		b.WithCompany(c)
	}
	return nil
}

func (r *branchRepository) Create(ctx context.Context, b *domain.Branch) error {
	if err := r.db.WithContext(ctx).Create(b).Error; err != nil {
		return exception.HandleGormError(err)
	}
	return r.withCompany(ctx, b)
}

func (r *branchRepository) get(ctx context.Context, tx *gorm.DB) (*domain.Branch, error) {
	var b domain.Branch
	if err := tx.First(&b).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	if err := r.withCompany(ctx, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *branchRepository) GetByID(ctx context.Context, id uint) (*domain.Branch, error) {
	return r.get(ctx, r.db.WithContext(ctx).Where("id = ?", id))
}

func (r *branchRepository) GetBySlug(ctx context.Context, slug string) (*domain.Branch, error) {
	return r.get(ctx, r.db.WithContext(ctx).Where("slug = ?", slug))
}

func (r *branchRepository) List(ctx context.Context, q output.ListQuery) ([]domain.Branch, int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.Branch{})
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("(name ILIKE ? OR slug ILIKE ?)", like, like)
	}
	tx = applyFilters(tx, q.Filters)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var items []domain.Branch
	if err := paginate(tx.Order("id ASC"), q).Find(&items).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	ptrs := make([]*domain.Branch, len(items))
	for i := range items {
		ptrs[i] = &items[i]
	}
	if len(items) > 0 {
		if err := r.withCompany(ctx, ptrs...); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func (r *branchRepository) Update(ctx context.Context, b *domain.Branch) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(b).Error)
}

func (r *branchRepository) SlugTaken(ctx context.Context, slug string, exceptID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Branch{}).Where("slug = ? AND id <> ?", slug, exceptID).Count(&count).Error
	return count > 0, exception.HandleGormError(err)
}

// ---------- memberships ----------

type membershipRepository struct{ db *gorm.DB }

func NewMembershipRepository(db *gorm.DB) output.MembershipRepository {
	return &membershipRepository{db: db}
}

func (r *membershipRepository) Create(ctx context.Context, m *domain.Membership) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Omit("User", "Branch").Create(m).Error)
}

func (r *membershipRepository) Get(ctx context.Context, branchID, userID uint) (*domain.Membership, error) {
	return firstOrNil[domain.Membership](r.db.WithContext(ctx).Where("branch_id = ? AND user_id = ?", branchID, userID))
}

func (r *membershipRepository) ListByBranch(ctx context.Context, branchID uint) ([]domain.Membership, error) {
	var items []domain.Membership
	err := r.db.WithContext(ctx).Preload("User").Where("branch_id = ?", branchID).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *membershipRepository) ListByUser(ctx context.Context, userID uint) ([]domain.Membership, error) {
	var items []domain.Membership
	err := r.db.WithContext(ctx).Preload("Branch").Where("user_id = ?", userID).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *membershipRepository) ListAll(ctx context.Context) ([]domain.Membership, error) {
	var items []domain.Membership
	err := r.db.WithContext(ctx).Preload("User").Preload("Branch").Order("user_id ASC, branch_id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *membershipRepository) Replace(ctx context.Context, userID uint, role string, branchIDs []uint, active bool) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&domain.Membership{}).Error; err != nil {
			return err
		}
		for _, id := range branchIDs {
			m := domain.Membership{UserID: userID, BranchID: id, Role: role, Active: active}
			if err := tx.Omit("User", "Branch").Create(&m).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

// ---------- invitations ----------

type invitationRepository struct{ db *gorm.DB }

func NewInvitationRepository(db *gorm.DB) output.InvitationRepository {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) pending(ctx context.Context, now time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&domain.Invitation{}).Where("accepted_at IS NULL AND expires_at > ?", now)
}

func (r *invitationRepository) Create(ctx context.Context, inv *domain.Invitation) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(inv).Error)
}

func (r *invitationRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.Invitation, error) {
	return firstOrNil[domain.Invitation](r.db.WithContext(ctx).Where("token_hash = ?", hash))
}

func (r *invitationRepository) GetByID(ctx context.Context, id uint) (*domain.Invitation, error) {
	var inv domain.Invitation
	if err := r.db.WithContext(ctx).First(&inv, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &inv, nil
}

func (r *invitationRepository) ListPending(ctx context.Context, now time.Time) ([]domain.Invitation, error) {
	var items []domain.Invitation
	err := r.pending(ctx, now).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *invitationRepository) PendingForEmail(ctx context.Context, email string, now time.Time) (*domain.Invitation, error) {
	return firstOrNil[domain.Invitation](r.pending(ctx, now).Where("email = ?", email))
}

func (r *invitationRepository) Update(ctx context.Context, inv *domain.Invitation) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(inv).Error)
}

func (r *invitationRepository) Delete(ctx context.Context, id uint) error {
	return requireAffected(r.db.WithContext(ctx).Delete(&domain.Invitation{}, id))
}

// ---------- password resets ----------

type passwordResetRepository struct{ db *gorm.DB }

func NewPasswordResetRepository(db *gorm.DB) output.PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

func (r *passwordResetRepository) Create(ctx context.Context, reset *domain.PasswordReset) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(reset).Error)
}

func (r *passwordResetRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordReset, error) {
	return firstOrNil[domain.PasswordReset](r.db.WithContext(ctx).Where("token_hash = ?", hash))
}

func (r *passwordResetRepository) UseAll(ctx context.Context, userID uint, at time.Time) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Model(&domain.PasswordReset{}).
		Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", at).Error)
}
