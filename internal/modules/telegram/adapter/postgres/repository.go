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
	db := r.db.WithContext(ctx)
	// the two newer switches have a column default of true (so existing branches get them on). On insert gorm replaces
	// a false in the struct by that default, so their real values are kept here and written back explicitly
	staffCancel, noShow := s.OnStaffCancel, s.OnNoShow
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(s).Error; err != nil {
		return exception.HandleGormError(err)
	}
	s.OnStaffCancel, s.OnNoShow = staffCancel, noShow
	return exception.HandleGormError(db.Model(&domain.Setting{}).Where("branch_id = ?", s.BranchID).
		UpdateColumns(map[string]any{"on_staff_cancel": staffCancel, "on_no_show": noShow}).Error)
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
