package postgres

import (
	"backend/internal/modules/telegram/domain"
	"backend/internal/modules/telegram/port"
	"backend/internal/platform/postgres/exception"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repo struct{ db *gorm.DB }

// New stores the Telegram settings in PostgreSQL
func New(db *gorm.DB) port.Repository { return &repo{db: db} }

func (r *repo) Get(ctx context.Context, branchID uint) (*domain.Setting, error) {
	var s domain.Setting
	err := r.db.WithContext(ctx).Where("branch_id = ?", branchID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &s, nil
}

func (r *repo) Save(ctx context.Context, s *domain.Setting) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(s).Error)
}

func (r *repo) Delete(ctx context.Context, branchID uint) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Where("branch_id = ?", branchID).Delete(&domain.Setting{}).Error)
}

func (r *repo) Result(ctx context.Context, branchID uint, lastError string) error {
	cols := map[string]any{"last_error": lastError}
	if lastError == "" {
		cols["last_ok_at"] = time.Now()
	}
	return exception.HandleGormError(r.db.WithContext(ctx).Model(&domain.Setting{}).Where("branch_id = ?", branchID).Updates(cols).Error)
}
