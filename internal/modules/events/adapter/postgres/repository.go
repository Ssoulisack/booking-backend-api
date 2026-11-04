package postgres

import (
	"backend/internal/core/domain"
	evdomain "backend/internal/modules/events/domain"
	"backend/internal/modules/events/port"
	"backend/internal/platform/postgres/exception"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresRepo struct{ db *gorm.DB }

// New stores events in PostgreSQL
func New(db *gorm.DB) port.Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Create(ctx context.Context, e *evdomain.Event) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(e).Error)
}

func (r *postgresRepo) Update(ctx context.Context, e *evdomain.Event) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(e).Error)
}

func (r *postgresRepo) Get(ctx context.Context, branchID, id uint) (*evdomain.Event, error) {
	var e evdomain.Event
	err := r.db.WithContext(ctx).Where("branch_id = ? AND id = ?", branchID, id).First(&e).Error
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &e, nil
}

func (r *postgresRepo) List(ctx context.Context, branchID uint) ([]evdomain.Event, error) {
	var out []evdomain.Event
	err := r.db.WithContext(ctx).Where("branch_id = ?", branchID).Order("starts_on DESC, id DESC").Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) Delete(ctx context.Context, branchID, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("branch_id = ? AND id = ?", branchID, id).Delete(&evdomain.Event{})
		if res.Error != nil {
			return exception.HandleGormError(res.Error)
		}
		if res.RowsAffected != 1 {
			return exception.HandleGormError(gorm.ErrRecordNotFound)
		}
		return tx.Where("event_id = ?", id).Delete(&evdomain.Price{}).Error
	})
}

func (r *postgresRepo) PublishedOverlapping(ctx context.Context, branchID uint, from, to string, exceptID uint) ([]evdomain.Event, error) {
	var out []evdomain.Event
	tx := r.db.WithContext(ctx).Where("branch_id = ? AND published AND starts_on <= ? AND ends_on >= ?", branchID, to, from)
	if exceptID != 0 {
		tx = tx.Where("id <> ?", exceptID)
	}
	return out, exception.HandleGormError(tx.Find(&out).Error)
}

func (r *postgresRepo) On(ctx context.Context, branchID uint, day string) (*evdomain.Event, error) {
	var e evdomain.Event
	err := r.db.WithContext(ctx).
		Where("branch_id = ? AND published AND starts_on <= ? AND ends_on >= ?", branchID, day, day).
		Order("id").First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &e, nil
}

func (r *postgresRepo) Upcoming(ctx context.Context, branchID uint, today string) ([]evdomain.Event, error) {
	var out []evdomain.Event
	err := r.db.WithContext(ctx).
		Where("branch_id = ? AND published AND ends_on >= ?", branchID, today).
		Order("starts_on, id").Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) Prices(ctx context.Context, eventID uint) ([]evdomain.Price, error) {
	var out []evdomain.Price
	err := r.db.WithContext(ctx).Where("event_id = ?", eventID).Order("id").Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) ReplacePrices(ctx context.Context, eventID uint, prices []evdomain.Price) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("event_id = ?", eventID).Delete(&evdomain.Price{}).Error; err != nil {
			return exception.HandleGormError(err)
		}
		if len(prices) == 0 {
			return nil
		}
		return exception.HandleGormError(tx.Create(&prices).Error)
	})
}

func (r *postgresRepo) BookingCounts(ctx context.Context, eventID uint) (map[string]int64, error) {
	var rows []struct {
		Status string
		N      int64
	}
	err := r.db.WithContext(ctx).Model(&domain.Booking{}).Select("status, COUNT(*) AS n").
		Where("event_id = ?", eventID).Group("status").Scan(&rows).Error
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Status] = row.N
	}
	return out, nil
}

func (r *postgresRepo) PayInfo(ctx context.Context, branchID uint) (*evdomain.PayInfo, error) {
	var p evdomain.PayInfo
	err := r.db.WithContext(ctx).Where("branch_id = ?", branchID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &p, nil
}

func (r *postgresRepo) SavePayInfo(ctx context.Context, p *evdomain.PayInfo) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(p).Error)
}
