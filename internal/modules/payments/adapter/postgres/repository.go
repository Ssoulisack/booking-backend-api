package postgres

import (
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/port"
	"backend/internal/platform/postgres/exception"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresRepo struct{ db *gorm.DB }

// New stores payments in PostgreSQL
func New(db *gorm.DB) port.Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Create(ctx context.Context, p *domain.Payment) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(p).Error)
}

func (r *postgresRepo) Update(ctx context.Context, p *domain.Payment) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(p).Error)
}

func (r *postgresRepo) first(tx *gorm.DB) (*domain.Payment, error) {
	var p domain.Payment
	err := tx.First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &p, nil
}

func (r *postgresRepo) Get(ctx context.Context, branchID, id uint) (*domain.Payment, error) {
	return r.first(r.db.WithContext(ctx).Where("branch_id = ? AND id = ?", branchID, id))
}

func (r *postgresRepo) ByRef(ctx context.Context, provider, ref string) (*domain.Payment, error) {
	if ref == "" {
		return nil, nil
	}
	return r.first(r.db.WithContext(ctx).Where("provider = ? AND provider_ref = ?", provider, ref))
}

func (r *postgresRepo) ByOrderNo(ctx context.Context, provider, orderNo string) (*domain.Payment, error) {
	if orderNo == "" {
		return nil, nil
	}
	return r.first(r.db.WithContext(ctx).Where("provider = ? AND order_no = ?", provider, orderNo))
}

func (r *postgresRepo) ByBooking(ctx context.Context, branchID, bookingID uint) ([]domain.Payment, error) {
	var out []domain.Payment
	err := r.db.WithContext(ctx).Where("branch_id = ? AND booking_id = ?", branchID, bookingID).Order("id DESC").Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) CountByBooking(ctx context.Context, bookingID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&domain.Payment{}).Where("booking_id = ?", bookingID).Count(&n).Error
	return n, exception.HandleGormError(err)
}

func (r *postgresRepo) Pending(ctx context.Context, before, since time.Time) ([]domain.Payment, error) {
	var out []domain.Payment
	err := r.db.WithContext(ctx).
		Where("provider_ref <> '' AND created_at < ?", before).
		Where("((status = ? AND expires_at > ?) OR status = ?)", domain.StatusPending, since, domain.StatusRefundRequested).
		Order("id").Limit(200).Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) Attention(ctx context.Context, branchID uint, statuses []string) ([]domain.Payment, error) {
	var out []domain.Payment
	err := r.db.WithContext(ctx).Where("branch_id = ? AND status IN ?", branchID, statuses).Order("id DESC").Limit(200).Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) Setting(ctx context.Context, branchID uint) (*domain.Setting, error) {
	var st domain.Setting
	err := r.db.WithContext(ctx).Where("branch_id = ?", branchID).First(&st).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &st, nil
}

func (r *postgresRepo) SaveSetting(ctx context.Context, st *domain.Setting) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(st).Error)
}
