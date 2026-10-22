package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"errors"

	"gorm.io/gorm"
)

// requireAffected maps "no row changed" to not found
func requireAffected(res *gorm.DB) error {
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected == 0 {
		return exception.HandleGormError(gorm.ErrRecordNotFound)
	}
	return nil
}

type openingHourRepository struct{ db *gorm.DB }

func NewOpeningHourRepository(db *gorm.DB) output.OpeningHourRepository {
	return &openingHourRepository{db: db}
}

func (r *openingHourRepository) ListByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) ([]domain.OpeningHour, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var hours []domain.OpeningHour
	err := r.db.WithContext(ctx).
		Where("branch_id = ? AND owner_type = ? AND owner_id = ?", branchID, ownerType, ownerID).
		Order("weekday ASC, open_time ASC").Find(&hours).Error
	return hours, exception.HandleGormError(err)
}

// ReplaceForOwner hard-deletes the old schedule: hours have no history worth keeping
func (r *openingHourRepository) ReplaceForOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint, hours []domain.OpeningHour) error {
	if branchID == 0 {
		return errNoScope
	}
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("branch_id = ? AND owner_type = ? AND owner_id = ?", branchID, ownerType, ownerID).
			Delete(&domain.OpeningHour{}).Error; err != nil {
			return err
		}
		if len(hours) == 0 {
			return nil
		}
		return tx.Create(&hours).Error
	}))
}

type priceRepository struct{ db *gorm.DB }

func NewPriceRepository(db *gorm.DB) output.PriceRepository {
	return &priceRepository{db: db}
}

func (r *priceRepository) GetByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) (*domain.Price, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var p domain.Price
	err := r.db.WithContext(ctx).
		Where("branch_id = ? AND owner_type = ? AND owner_id = ?", branchID, ownerType, ownerID).First(&p).Error
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &p, nil
}

func (r *priceRepository) Upsert(ctx context.Context, p *domain.Price) error {
	if p.BranchID == 0 {
		return errNoScope
	}
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.Price
		err := tx.Where("branch_id = ? AND owner_type = ? AND owner_id = ?", p.BranchID, p.OwnerType, p.OwnerID).
			First(&existing).Error
		switch {
		case err == nil:
			p.ID = existing.ID
			p.CreatedAt = existing.CreatedAt
			return tx.Save(p).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			return tx.Create(p).Error
		default:
			return err
		}
	}))
}

func (r *priceRepository) DeleteByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) error {
	if branchID == 0 {
		return errNoScope
	}
	return requireAffected(r.db.WithContext(ctx).
		Where("branch_id = ? AND owner_type = ? AND owner_id = ?", branchID, ownerType, ownerID).
		Delete(&domain.Price{}))
}

type ruleRepository struct{ db *gorm.DB }

func NewRuleRepository(db *gorm.DB) output.RuleRepository {
	return &ruleRepository{db: db}
}

// GetByScope reads one rule; branch 0 is allowed only for the company rule
func (r *ruleRepository) GetByScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) (*domain.BookingRule, error) {
	if (branchID == 0) != (scopeType == domain.ScopeCompany) {
		return nil, errNoScope
	}
	var rule domain.BookingRule
	err := r.db.WithContext(ctx).
		Where("branch_id = ? AND scope_type = ? AND scope_id = ?", branchID, scopeType, scopeID).First(&rule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &rule, nil
}

// Upsert replaces the whole rule, so a nil field is stored as NULL (inherit)
func (r *ruleRepository) Upsert(ctx context.Context, rule *domain.BookingRule) error {
	if (rule.BranchID == 0) != (rule.ScopeType == domain.ScopeCompany) {
		return errNoScope
	}
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.BookingRule
		err := tx.Where("branch_id = ? AND scope_type = ? AND scope_id = ?", rule.BranchID, rule.ScopeType, rule.ScopeID).
			First(&existing).Error
		switch {
		case err == nil:
			rule.ID = existing.ID
			rule.CreatedAt = existing.CreatedAt
			return tx.Save(rule).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			return tx.Create(rule).Error
		default:
			return err
		}
	}))
}

func (r *ruleRepository) List(ctx context.Context, branchID uint) ([]domain.BookingRule, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var rules []domain.BookingRule
	err := r.db.WithContext(ctx).Where("branch_id = ?", branchID).
		Order("scope_type ASC, scope_id ASC").Find(&rules).Error
	return rules, exception.HandleGormError(err)
}

func (r *ruleRepository) DeleteByScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) error {
	if branchID == 0 {
		return errNoScope
	}
	return requireAffected(r.db.WithContext(ctx).
		Where("branch_id = ? AND scope_type = ? AND scope_id = ?", branchID, scopeType, scopeID).
		Delete(&domain.BookingRule{}))
}

// mediaRepository records uploaded files; branch 0 holds the company's files (logo, cover)
type mediaRepository struct{ db *gorm.DB }

func NewMediaRepository(db *gorm.DB) output.MediaRepository {
	return &mediaRepository{db: db}
}

func (r *mediaRepository) Create(ctx context.Context, m *domain.Media) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(m).Error)
}

func (r *mediaRepository) Get(ctx context.Context, branchID uint, key string) (*domain.Media, error) {
	var m domain.Media
	err := r.db.WithContext(ctx).Where("branch_id = ? AND key = ?", branchID, key).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &m, nil
}

func (r *mediaRepository) Delete(ctx context.Context, branchID uint, key string) error {
	return requireAffected(r.db.WithContext(ctx).Where("branch_id = ? AND key = ?", branchID, key).Delete(&domain.Media{}))
}
