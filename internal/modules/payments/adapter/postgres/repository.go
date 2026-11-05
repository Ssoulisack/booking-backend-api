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

// withCharges loads a payment's charges, newest first
func withCharges(tx *gorm.DB) *gorm.DB {
	return tx.Preload("Charges", func(db *gorm.DB) *gorm.DB { return db.Order("id DESC") })
}

func (r *postgresRepo) CreatePayment(ctx context.Context, p *domain.Payment) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Omit(clause.Associations).Create(p).Error)
}

func (r *postgresRepo) SavePayment(ctx context.Context, p *domain.Payment) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Omit(clause.Associations).Save(p).Error)
}

func (r *postgresRepo) CreateCharge(ctx context.Context, c *domain.Charge) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(c).Error)
}

func (r *postgresRepo) SaveCharge(ctx context.Context, c *domain.Charge) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(c).Error)
}

func (r *postgresRepo) SplitOff(ctx context.Context, c *domain.Charge, p *domain.Payment) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(p).Error; err != nil {
			return err
		}
		c.PaymentID = p.ID
		return tx.Save(c).Error
	}))
}

func (r *postgresRepo) firstPayment(tx *gorm.DB) (*domain.Payment, error) {
	var p domain.Payment
	err := withCharges(tx).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &p, nil
}

func (r *postgresRepo) Get(ctx context.Context, branchID, id uint) (*domain.Payment, error) {
	return r.firstPayment(r.db.WithContext(ctx).Where("branch_id = ? AND id = ?", branchID, id))
}

func (r *postgresRepo) Find(ctx context.Context, id uint) (*domain.Payment, error) {
	return r.firstPayment(r.db.WithContext(ctx).Where("id = ?", id))
}

func (r *postgresRepo) firstCharge(tx *gorm.DB) (*domain.Charge, error) {
	var c domain.Charge
	err := tx.First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &c, nil
}

func (r *postgresRepo) ChargeByRef(ctx context.Context, provider, ref string) (*domain.Charge, error) {
	if ref == "" {
		return nil, nil
	}
	return r.firstCharge(r.db.WithContext(ctx).Where("provider = ? AND provider_ref = ?", provider, ref))
}

func (r *postgresRepo) ChargeByOrderNo(ctx context.Context, provider, orderNo string) (*domain.Charge, error) {
	if orderNo == "" {
		return nil, nil
	}
	return r.firstCharge(r.db.WithContext(ctx).Where("provider = ? AND gateway_order_no = ?", provider, orderNo))
}

func (r *postgresRepo) ByBooking(ctx context.Context, branchID, bookingID uint) ([]domain.Payment, error) {
	var out []domain.Payment
	err := withCharges(r.db.WithContext(ctx)).Where("branch_id = ? AND booking_id = ?", branchID, bookingID).Order("id DESC").Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) CountByBooking(ctx context.Context, bookingID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&domain.Payment{}).Where("booking_id = ?", bookingID).Count(&n).Error
	return n, exception.HandleGormError(err)
}

func (r *postgresRepo) Pending(ctx context.Context, before, since time.Time) ([]domain.Charge, error) {
	var out []domain.Charge
	err := r.db.WithContext(ctx).Table("payment_charges AS c").Select("c.*").
		Joins("JOIN payments AS p ON p.id = c.payment_id").
		Where("c.provider_ref <> '' AND c.created_at < ?", before).
		Where("(c.status = ? AND p.status = ? AND c.expires_at > ?) OR (p.status = ? AND p.charge_id = c.id)",
			domain.ChargePending, domain.StatusPending, since, domain.StatusRefundRequested).
		Order("c.id").Limit(200).Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) Attention(ctx context.Context, branchID uint, statuses []string) ([]domain.Payment, error) {
	var out []domain.Payment
	err := withCharges(r.db.WithContext(ctx)).Where("branch_id = ? AND status IN ?", branchID, statuses).Order("id DESC").Limit(200).Find(&out).Error
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
