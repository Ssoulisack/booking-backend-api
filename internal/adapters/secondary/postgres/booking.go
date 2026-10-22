package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var activeStatuses = []string{domain.BookingPending, domain.BookingConfirmed}

type bookingRepository struct{ db *gorm.DB }

func NewBookingRepository(db *gorm.DB) output.BookingRepository {
	return &bookingRepository{db: db}
}

func (r *bookingRepository) Create(ctx context.Context, b *domain.Booking) error {
	if b.BranchID == 0 {
		return errNoScope
	}
	return exception.HandleGormError(r.db.WithContext(ctx).Omit("Branch", "Customer", "PaidBy", "Sport").Create(b).Error)
}

func (r *bookingRepository) CreateClassBooking(ctx context.Context, b *domain.Booking, classID uint, seats int) error {
	if b.BranchID == 0 {
		return errNoScope
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// the row lock makes concurrent bookings of the same class wait for each other
		var class domain.ClassSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("branch_id = ?", b.BranchID).First(&class, classID).Error; err != nil {
			return err
		}
		var taken int64
		if err := tx.Model(&domain.BookingItem{}).
			Where("class_session_id = ? AND kind = ? AND active", classID, domain.ItemClassSession).
			Select("COALESCE(SUM(seats), 0)").Scan(&taken).Error; err != nil {
			return err
		}
		if taken+int64(seats) > int64(class.Capacity) {
			return response.NewError(http.StatusConflict, "CLASS_FULL")
		}
		return tx.Omit("Branch", "Customer", "PaidBy", "Sport").Create(b).Error
	})
	return keepAppError(err)
}

func (r *bookingRepository) withDetails(ctx context.Context) *gorm.DB {
	return withNames(r.db.WithContext(ctx).Preload("Items").Preload("Branch").Preload("Customer").Preload("PaidBy"))
}

// withNames loads what each booking is for (sport, court, coach, class) so answers can show names;
// unscoped, so a court or coach deleted later still names its old bookings
func withNames(tx *gorm.DB) *gorm.DB {
	unscoped := func(db *gorm.DB) *gorm.DB { return db.Unscoped() }
	return tx.Preload("Sport", unscoped).
		Preload("Items.Resource", unscoped).
		Preload("Items.Coach", unscoped).
		Preload("Items.ClassSession", unscoped)
}

func (r *bookingRepository) GetByID(ctx context.Context, branchID, id uint) (*domain.Booking, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var b domain.Booking
	if err := r.withDetails(ctx).Where("branch_id = ?", branchID).First(&b, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &b, nil
}

func (r *bookingRepository) GetForCustomer(ctx context.Context, customerID, id uint) (*domain.Booking, error) {
	var b domain.Booking
	if err := r.withDetails(ctx).Where("customer_id = ?", customerID).First(&b, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &b, nil
}

func (r *bookingRepository) GetByGuestToken(ctx context.Context, branchID uint, tokenHash string) (*domain.Booking, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	return firstOrNil[domain.Booking](r.withDetails(ctx).
		Where("branch_id = ? AND guest_token_hash = ? AND guest_token_hash <> ''", branchID, tokenHash))
}

func (r *bookingRepository) ListForCustomer(ctx context.Context, customerID uint, q output.ListQuery) ([]domain.Booking, int64, error) {
	tx := applyFilters(r.db.WithContext(ctx).Model(&domain.Booking{}).Where("customer_id = ?", customerID), q.Filters)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var items []domain.Booking
	err := paginate(withNames(tx.Preload("Items").Preload("Branch")).Order("starts_at DESC"), q).Find(&items).Error
	return items, total, exception.HandleGormError(err)
}

func (r *bookingRepository) Cancel(ctx context.Context, b *domain.Booking, reason string, at time.Time) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.Booking{}).
			Where("id = ? AND branch_id = ? AND status IN ?", b.ID, b.BranchID, activeStatuses).
			Updates(map[string]any{"status": domain.BookingCancelled, "cancelled_at": at, "cancel_reason": reason})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return response.NewError(http.StatusConflict, "BOOKING_NOT_ACTIVE")
		}
		return tx.Model(&domain.BookingItem{}).Where("booking_id = ?", b.ID).Update("active", false).Error
	})
	if err != nil {
		return keepAppError(err)
	}
	b.Status, b.CancelledAt, b.CancelReason = domain.BookingCancelled, &at, reason
	return nil
}

// ownerColumn maps an owner type to its booking_items column
func ownerColumn(ownerType string) (string, error) {
	switch ownerType {
	case domain.OwnerResource:
		return "resource_id", nil
	case domain.OwnerCoach:
		return "coach_id", nil
	}
	return "", response.NewError(http.StatusBadRequest, "INVALID_OWNER")
}

func (r *bookingRepository) ActiveItems(ctx context.Context, branchID uint, ownerType string, ownerID uint, from, to time.Time) ([]domain.BookingItem, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	column, err := ownerColumn(ownerType)
	if err != nil {
		return nil, err
	}
	var items []domain.BookingItem
	err = r.db.WithContext(ctx).
		Where("branch_id = ? AND active AND "+column+" = ? AND starts_at < ? AND ends_at > ?", branchID, ownerID, to, from).
		Order("starts_at ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *bookingRepository) countActive(ctx context.Context, where string, args ...any) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Booking{}).Where(where, args...).Count(&count).Error
	return count, exception.HandleGormError(err)
}

func (r *bookingRepository) CountActiveForCustomer(ctx context.Context, branchID, customerID uint, now time.Time) (int64, error) {
	return r.countActive(ctx, "branch_id = ? AND customer_id = ? AND status IN ? AND ends_at > ?",
		branchID, customerID, activeStatuses, now)
}

func (r *bookingRepository) CountActiveForGuestPhone(ctx context.Context, branchID uint, phone string, now time.Time) (int64, error) {
	return r.countActive(ctx, "branch_id = ? AND customer_id IS NULL AND guest_phone = ? AND status IN ? AND ends_at > ?",
		branchID, phone, activeStatuses, now)
}

func (r *bookingRepository) SeatsTaken(ctx context.Context, branchID uint, classIDs []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(classIDs))
	if len(classIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ClassSessionID uint
		Seats          int
	}
	err := r.db.WithContext(ctx).Model(&domain.BookingItem{}).
		Select("class_session_id, COALESCE(SUM(seats), 0) AS seats").
		Where("branch_id = ? AND kind = ? AND active AND class_session_id IN ?", branchID, domain.ItemClassSession, classIDs).
		Group("class_session_id").Scan(&rows).Error
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	for _, row := range rows {
		out[row.ClassSessionID] = row.Seats
	}
	return out, nil
}

func (r *bookingRepository) CountFutureForOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint, now time.Time) (int64, error) {
	column, err := ownerColumn(ownerType)
	if err != nil {
		return 0, err
	}
	var count int64
	err = r.db.WithContext(ctx).Model(&domain.BookingItem{}).
		Where("branch_id = ? AND active AND kind <> ? AND "+column+" = ? AND ends_at > ?", branchID, domain.ItemClassHold, ownerID, now).
		Count(&count).Error
	return count, exception.HandleGormError(err)
}

// ---------- closures ----------

type closureRepository struct {
	*scopedRepository[domain.Closure]
}

func NewClosureRepository(db *gorm.DB) output.ClosureRepository {
	return &closureRepository{newScoped[domain.Closure](db, "")}
}

func (r *closureRepository) Overlapping(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Closure, error) {
	tx, err := r.scope(ctx, branchID)
	if err != nil {
		return nil, err
	}
	var items []domain.Closure
	err = tx.Where("starts_at < ? AND ends_at > ?", to, from).Find(&items).Error
	return items, exception.HandleGormError(err)
}

// ---------- branch dashboard ----------

func (r *bookingRepository) filtered(ctx context.Context, branchID uint, f output.BookingFilter) *gorm.DB {
	tx := r.db.WithContext(ctx).Model(&domain.Booking{}).Where("bookings.branch_id = ?", branchID)
	if f.From != nil {
		tx = tx.Where("bookings.ends_at > ?", *f.From)
	}
	if f.To != nil {
		tx = tx.Where("bookings.starts_at < ?", *f.To)
	}
	if f.SportID != 0 {
		tx = tx.Where("bookings.sport_id = ?", f.SportID)
	}
	if f.CustomerID != 0 {
		tx = tx.Where("bookings.customer_id = ?", f.CustomerID)
	}
	if f.Status != "" {
		tx = tx.Where("bookings.status = ?", f.Status)
	}
	if f.PaymentStatus != "" {
		tx = tx.Where("bookings.payment_status = ?", f.PaymentStatus)
	}
	if f.ResourceID != 0 {
		tx = tx.Where("bookings.id IN (SELECT booking_id FROM booking_items WHERE resource_id = ?)", f.ResourceID)
	}
	if f.CoachID != 0 {
		tx = tx.Where("bookings.id IN (SELECT booking_id FROM booking_items WHERE coach_id = ?)", f.CoachID)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		tx = tx.Where("(bookings.code ILIKE ? OR bookings.guest_name ILIKE ? OR bookings.guest_phone LIKE ? OR "+
			"bookings.customer_id IN (SELECT id FROM users WHERE name ILIKE ? OR email ILIKE ?))", like, like, like, like, like)
	}
	return tx
}

func (r *bookingRepository) List(ctx context.Context, branchID uint, f output.BookingFilter, q output.ListQuery) ([]domain.Booking, int64, error) {
	if branchID == 0 {
		return nil, 0, errNoScope
	}
	tx := r.filtered(ctx, branchID, f)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var items []domain.Booking
	err := paginate(withNames(tx.Preload("Items").Preload("Customer").Preload("PaidBy")).Order("bookings.starts_at ASC, bookings.id ASC"), q).
		Find(&items).Error
	return items, total, exception.HandleGormError(err)
}

func (r *bookingRepository) InRange(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Booking, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var items []domain.Booking
	err := r.filtered(ctx, branchID, output.BookingFilter{From: &from, To: &to}).
		Preload("Items").Preload("Customer").Preload("PaidBy").Order("bookings.starts_at ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *bookingRepository) PaidBetween(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Booking, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var items []domain.Booking
	err := r.db.WithContext(ctx).Preload("Customer").Preload("PaidBy").
		Where("branch_id = ? AND payment_status = ? AND paid_at >= ? AND paid_at < ?", branchID, domain.PaymentPaid, from, to).
		Order("paid_at ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *bookingRepository) SetStatus(ctx context.Context, b *domain.Booking, status string) error {
	// the WHERE on the old status makes two staff clicking at once safe: only one change applies
	res := r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("id = ? AND branch_id = ? AND status = ?", b.ID, b.BranchID, b.Status).Update("status", status)
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected != 1 {
		return response.NewError(http.StatusConflict, "BOOKING_CHANGED_MEANWHILE")
	}
	b.Status = status
	return nil
}

func (r *bookingRepository) SetPayment(ctx context.Context, b *domain.Booking) error {
	return requireAffected(r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("id = ? AND branch_id = ?", b.ID, b.BranchID).
		Updates(map[string]any{
			"payment_status": b.PaymentStatus, "payment_method": b.PaymentMethod,
			"paid_at": b.PaidAt, "paid_by_id": b.PaidByID,
		}))
}

func (r *bookingRepository) Customers(ctx context.Context, branchID uint, keyword string, q output.ListQuery) ([]output.CustomerRow, int64, error) {
	tx := r.db.WithContext(ctx).Table("bookings").
		Joins("JOIN users ON users.id = bookings.customer_id").
		Where("bookings.deleted_at IS NULL")
	if branchID != 0 { // 0: every branch (the company's customers)
		tx = tx.Where("bookings.branch_id = ?", branchID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("(users.name ILIKE ? OR users.email ILIKE ? OR users.phone LIKE ?)", like, like, like)
	}
	var total int64
	if err := tx.Distinct("users.id").Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var rows []output.CustomerRow
	err := paginate(tx.Select(`users.id AS user_id, users.name, users.email, users.phone,
			COUNT(bookings.id) AS bookings, MAX(bookings.starts_at) AS last_booking_at`).
		Group("users.id, users.name, users.email, users.phone").Order("MAX(bookings.starts_at) DESC"), q).
		Scan(&rows).Error
	return rows, total, exception.HandleGormError(err)
}
