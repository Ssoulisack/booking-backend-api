package postgres

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/internal/platform/postgres/exception"
	"backend/internal/platform/postgres/numbering"
	"backend/pkg/response"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// bookingInsertLock is the first key of the advisory lock that serializes booking inserts per branch
const bookingInsertLock = 7001

var activeStatuses = []string{domain.BookingPending, domain.BookingConfirmed}

type bookingRepository struct{ db *gorm.DB }

func NewBookingRepository(db *gorm.DB) output.BookingRepository {
	return &bookingRepository{db: db}
}

func (r *bookingRepository) Create(ctx context.Context, b *domain.Booking) error {
	if b.BranchID == 0 {
		return errNoScope
	}
	// Many people booking the same place at once can make Postgres abort some inserts as a deadlock (each waits for
	// the other's not yet committed item). Nothing was saved then, so try again: the retry meets the winner's
	// committed row and gets the normal "taken" answer instead of a server error.
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		// the advisory lock (one per branch, released when the insert ends) makes the inserts of one branch take
		// turns, so they do not wait on each other's items at all; the exclusion constraints stay as the safety net
		err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", bookingInsertLock, int32(b.BranchID)).Error; err != nil {
				return err
			}
			if err := numberBooking(tx, b); err != nil {
				return err
			}
			return tx.Omit("Branch", "Customer", "PaidBy", "Sport", "Event").Create(b).Error
		})
		if !isDeadlock(err) {
			break
		}
		b.ID, b.Code = 0, ""
		for i := range b.Items {
			b.Items[i].ID, b.Items[i].BookingID = 0, nil
		}
		time.Sleep(time.Duration(attempt+1) * 15 * time.Millisecond)
	}
	return exception.HandleGormError(err)
}

// numberBooking gives a new booking its number: the date and the count of the day (20261108001). It is taken inside
// the insert's transaction, so a booking that does not get saved gives its number back.
func numberBooking(tx *gorm.DB, b *domain.Booking) error {
	if b.Code != "" {
		return nil
	}
	code, err := numbering.NextBookingCode(tx, time.Now())
	if err != nil {
		return err
	}
	b.Code = code
	return nil
}

// isDeadlock: Postgres chose this transaction to stop because of a deadlock (40P01) or a serialization clash (40001)
func isDeadlock(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001")
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
		if err := numberBooking(tx, b); err != nil {
			return err
		}
		return tx.Omit("Branch", "Customer", "PaidBy", "Sport", "Event").Create(b).Error
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
		Preload("Event", unscoped).
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

func (r *bookingRepository) GetByCode(ctx context.Context, branchID uint, code string) (*domain.Booking, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	return firstOrNil[domain.Booking](r.withDetails(ctx).Preload("Customer").
		Where("branch_id = ? AND code = ?", branchID, code))
}

func (r *bookingRepository) GetByCodeAnywhere(ctx context.Context, code string) (*domain.Booking, error) {
	return firstOrNil[domain.Booking](r.withDetails(ctx).Where("code = ?", code))
}

func (r *bookingRepository) SetCustomer(ctx context.Context, b *domain.Booking, customerID uint) error {
	res := r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("id = ? AND customer_id IS NULL", b.ID).Update("customer_id", customerID)
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected == 0 {
		return response.NewError(http.StatusConflict, "BOOKING_ALREADY_LINKED")
	}
	b.CustomerID = &customerID
	return nil
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

func (r *bookingRepository) ReleaseUnpaid(ctx context.Context, b *domain.Booking, reason string, at time.Time) (bool, error) {
	released := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur domain.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND branch_id = ?", b.ID, b.BranchID).First(&cur).Error; err != nil {
			return err
		}
		// looked at AFTER the lock: a payment that settled a moment ago has made it confirmed + paid
		if cur.Status != domain.BookingPending || cur.PaymentStatus != domain.PaymentUnpaid {
			return nil
		}
		if err := tx.Model(&domain.Booking{}).Where("id = ?", cur.ID).
			Updates(map[string]any{"status": domain.BookingCancelled, "cancelled_at": at, "cancel_reason": reason}).Error; err != nil {
			return err
		}
		released = true
		return tx.Model(&domain.BookingItem{}).Where("booking_id = ?", cur.ID).Update("active", false).Error
	})
	if err != nil {
		return false, keepAppError(err)
	}
	if released {
		b.Status, b.CancelledAt, b.CancelReason = domain.BookingCancelled, &at, reason
	}
	return released, nil
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

func (r *bookingRepository) CountOpenWaitingForPhone(ctx context.Context, phone string, now time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("status = ? AND payment_status = ? AND ends_at > ? AND event_id IS NULL", domain.BookingPending, domain.PaymentUnpaid, now).
		Where("guest_phone = ? OR customer_id IN (SELECT id FROM users WHERE phone = ? AND deleted_at IS NULL)", phone, phone).
		Count(&n).Error
	return n, exception.HandleGormError(err)
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
	if f.EventID != 0 {
		tx = tx.Where("bookings.event_id = ?", f.EventID)
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

func (r *bookingRepository) SettlePaidOnline(ctx context.Context, branchID, id uint, at time.Time) (*domain.Booking, bool, error) {
	var bk domain.Booking
	confirmed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND branch_id = ?", id, branchID).First(&bk).Error; err != nil {
			return err
		}
		switch {
		case bk.Status == domain.BookingCancelled:
			return domain.ErrNotPayable
		case bk.PaymentStatus == domain.PaymentPaid:
			return domain.ErrAlreadyPaid
		}
		bk.PaymentStatus, bk.PaymentMethod, bk.PaidAt, bk.PaidByID = domain.PaymentPaid, domain.MethodOnline, &at, nil
		cols := map[string]any{"payment_status": bk.PaymentStatus, "payment_method": bk.PaymentMethod, "paid_at": at, "paid_by_id": nil}
		if bk.EventID != nil && bk.Status == domain.BookingPending {
			cols["status"] = domain.BookingConfirmed
			bk.Status = domain.BookingConfirmed
			confirmed = true
		}
		return tx.Model(&domain.Booking{}).Where("id = ?", bk.ID).Updates(cols).Error
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotPayable) || errors.Is(err, domain.ErrAlreadyPaid) {
			return nil, false, err // the payments module tells these two apart to give the money back
		}
		return nil, false, keepAppError(err)
	}
	return &bk, confirmed, nil
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
		no, _ := domain.ParseMemberNo(keyword) // "" never matches a stored number
		tx = tx.Where("(users.name ILIKE ? OR users.email ILIKE ? OR users.phone LIKE ? OR users.member_no = ?)", like, like, like, no)
	}
	var total int64
	if err := tx.Distinct("users.id").Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var rows []output.CustomerRow
	err := paginate(tx.Select(`users.id AS user_id, users.member_no, users.name, users.email, users.phone,
			COUNT(bookings.id) AS bookings, MAX(bookings.starts_at) AS last_booking_at`).
		Group("users.id, users.member_no, users.name, users.email, users.phone").Order("MAX(bookings.starts_at) DESC"), q).
		Scan(&rows).Error
	return rows, total, exception.HandleGormError(err)
}

// ---------- restaurant tables at the desk ----------

// endItems frees a booking's tables from `at` on: items not started yet are let go, started ones end at `at`
func endItems(tx *gorm.DB, bookingID uint, at time.Time) error {
	if err := tx.Model(&domain.BookingItem{}).
		Where("booking_id = ? AND active AND starts_at >= ?", bookingID, at).Update("active", false).Error; err != nil {
		return err
	}
	return tx.Model(&domain.BookingItem{}).
		Where("booking_id = ? AND active AND ends_at > ?", bookingID, at).Update("ends_at", at).Error
}

func (r *bookingRepository) Finish(ctx context.Context, b *domain.Booking, status string, at time.Time) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changes := map[string]any{"status": status}
		if at.After(b.StartsAt) && at.Before(b.EndsAt) {
			changes["ends_at"] = at
		}
		res := tx.Model(&domain.Booking{}).
			Where("id = ? AND branch_id = ? AND status = ?", b.ID, b.BranchID, b.Status).Updates(changes)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return response.NewError(http.StatusConflict, "BOOKING_CHANGED_MEANWHILE")
		}
		return endItems(tx, b.ID, at)
	})
	if err != nil {
		return keepAppError(err)
	}
	b.Status = status
	if at.After(b.StartsAt) && at.Before(b.EndsAt) {
		b.EndsAt = at
	}
	return nil
}

func (r *bookingRepository) SetArrived(ctx context.Context, b *domain.Booking, at time.Time) error {
	res := r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("id = ? AND branch_id = ? AND status = ? AND arrived_at IS NULL", b.ID, b.BranchID, domain.BookingConfirmed).
		Update("arrived_at", at)
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected != 1 {
		return response.NewError(http.StatusConflict, "BOOKING_CHANGED_MEANWHILE")
	}
	b.ArrivedAt = &at
	return nil
}

func (r *bookingRepository) MoveItems(ctx context.Context, b *domain.Booking, at time.Time, items []domain.BookingItem) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.Booking{}).
			Where("id = ? AND branch_id = ? AND status IN ?", b.ID, b.BranchID, activeStatuses).
			Updates(map[string]any{"sport_id": b.SportID, "zones": b.Zones})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return response.NewError(http.StatusConflict, "BOOKING_NOT_ACTIVE")
		}
		if err := endItems(tx, b.ID, at); err != nil {
			return err
		}
		// the database refuses a table someone else holds at that time (409 SLOT_NOT_AVAILABLE)
		return tx.Create(&items).Error
	})
	return keepAppError(err)
}

func (r *bookingRepository) TablesToSettle(ctx context.Context, now time.Time) ([]domain.Booking, error) {
	var out []domain.Booking
	err := r.db.WithContext(ctx).
		Where("guests > 0 AND status = ? AND ((arrived_at IS NULL AND arrive_by < ?) OR ends_at <= ?)",
			domain.BookingConfirmed, now, now).
		Order("id").Limit(500).Find(&out).Error
	return out, exception.HandleGormError(err)
}

// ---------- event days ----------

func (r *bookingRepository) ActiveResourceItems(ctx context.Context, branchID uint, from, to time.Time) ([]domain.BookingItem, error) {
	var out []domain.BookingItem
	err := r.db.WithContext(ctx).
		Where("branch_id = ? AND active AND resource_id IS NOT NULL AND starts_at < ? AND ends_at > ?", branchID, to, from).
		Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *bookingRepository) NormalTableBookings(ctx context.Context, branchID uint, from, to time.Time, areaIDs []uint) ([]string, error) {
	q := r.db.WithContext(ctx).Table("bookings AS b").
		Joins("JOIN booking_items bi ON bi.booking_id = b.id AND bi.active").
		Joins("JOIN resources r ON r.id = bi.resource_id").
		Where("b.branch_id = ? AND b.deleted_at IS NULL AND b.event_id IS NULL AND b.guests > 0 AND b.status IN ?", branchID, activeStatuses).
		Where("bi.starts_at < ? AND bi.ends_at > ?", to, from)
	if len(areaIDs) > 0 {
		q = q.Where("r.sport_id IN ?", areaIDs)
	}
	var codes []string
	err := q.Distinct().Order("b.code").Limit(20).Pluck("b.code", &codes).Error
	return codes, exception.HandleGormError(err)
}

func (r *bookingRepository) EventHoldsExpired(ctx context.Context, now time.Time) ([]domain.Booking, error) {
	var out []domain.Booking
	err := r.db.WithContext(ctx).
		Where("event_id IS NOT NULL AND status = ? AND payment_status = ? AND slip_sent_at IS NULL AND pay_by < ?",
			domain.BookingPending, domain.PaymentUnpaid, now).
		Order("id").Limit(500).Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *bookingRepository) SetSlipSent(ctx context.Context, b *domain.Booking, at time.Time, via string) error {
	res := r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("id = ? AND branch_id = ? AND status = ? AND payment_status = ? AND event_id IS NOT NULL",
			b.ID, b.BranchID, domain.BookingPending, domain.PaymentUnpaid).
		Updates(map[string]any{"slip_sent_at": at, "slip_via": via})
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected != 1 {
		return response.NewError(http.StatusConflict, "NOT_WAITING_FOR_PAYMENT")
	}
	b.SlipSentAt, b.SlipVia = &at, via
	return nil
}

func (r *bookingRepository) SetPayBy(ctx context.Context, id uint, until time.Time) error {
	// only a booking still waiting for the team's money check (no slip sent): once a slip is sent the hold is the team's
	err := r.db.WithContext(ctx).Model(&domain.Booking{}).
		Where("id = ? AND event_id IS NOT NULL AND status = ? AND payment_status = ? AND slip_sent_at IS NULL",
			id, domain.BookingPending, domain.PaymentUnpaid).
		Update("pay_by", until).Error
	return exception.HandleGormError(err)
}
