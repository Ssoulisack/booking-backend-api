package postgres

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/internal/platform/postgres/exception"
	"context"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

type bookingReportRepository struct{ db *gorm.DB }

// NewBookingReportRepository reads bookings for the table and event reports
func NewBookingReportRepository(db *gorm.DB) output.BookingReportRepository {
	return &bookingReportRepository{db: db}
}

var (
	nonDigits      = regexp.MustCompile(`\D`)
	looksLikePhone = regexp.MustCompile(`^[\d\s+()-]+$`)
)

// escapeLike makes % and _ in what someone typed match themselves
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s)
}

// payments that took money (the gateway's statuses; kept as words here so the platform does not depend on the module)
const moneyTaken = "('paid','refund_due','refund_requested','refunded')"

func (r *bookingReportRepository) filtered(ctx context.Context, branchID uint, f output.BookingReportFilter, withStatus bool) *gorm.DB {
	tx := r.db.WithContext(ctx).Model(&domain.Booking{}).Where("bookings.branch_id = ?", branchID)
	if f.Event {
		tx = tx.Where("bookings.event_id IS NOT NULL")
		if f.EventID != 0 {
			tx = tx.Where("bookings.event_id = ?", f.EventID)
		}
	} else {
		tx = tx.Where("bookings.event_id IS NULL").
			Where("EXISTS (SELECT 1 FROM booking_items bi JOIN resources r ON r.id = bi.resource_id "+
				"WHERE bi.booking_id = bookings.id AND r.kind = ?)", domain.ResourceKindTable)
	}
	if f.From != nil {
		tx = tx.Where("bookings.starts_at >= ?", *f.From)
	}
	if f.To != nil {
		tx = tx.Where("bookings.starts_at < ?", *f.To)
	}
	if withStatus && f.Status != "" {
		tx = tx.Where("bookings.status = ?", f.Status)
	}
	if f.PaymentStatus != "" {
		tx = tx.Where("bookings.payment_status = ?", f.PaymentStatus)
	}
	if f.Keyword != "" {
		like := "%" + escapeLike(f.Keyword) + "%"
		// a phone is found however it is written: "+856 20 5555 7040", "020 5555 7040" and "2055557040" all match
		// (only when it looks like a phone: "A5" must not find every number with a 5 in it)
		phone := like
		if looksLikePhone.MatchString(f.Keyword) {
			if digits := strings.TrimLeft(nonDigits.ReplaceAllString(f.Keyword, ""), "0"); digits != "" {
				phone = "%" + digits + "%"
			}
		}
		tx = tx.Where("(bookings.code ILIKE ? OR bookings.guest_name ILIKE ? OR bookings.guest_phone LIKE ? OR "+
			"bookings.customer_id IN (SELECT id FROM users WHERE name ILIKE ? OR phone LIKE ?) OR "+
			"bookings.id IN (SELECT bi.booking_id FROM booking_items bi JOIN resources r ON r.id = bi.resource_id WHERE r.name ILIKE ?))",
			like, like, phone, like, phone, like)
	}
	if f.Bank != "" {
		tx = tx.Where("EXISTS (SELECT 1 FROM payments p WHERE p.booking_id = bookings.id AND p.method = ? AND p.status IN "+moneyTaken+")", f.Bank)
	}
	return tx
}

func (r *bookingReportRepository) Rows(ctx context.Context, branchID uint, f output.BookingReportFilter, page, limit int) ([]output.BookingReportRow, int64, error) {
	if branchID == 0 {
		return nil, 0, errNoScope
	}
	var total int64
	if err := r.filtered(ctx, branchID, f, true).Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	order := "bookings.created_at DESC, bookings.id DESC"
	if f.Oldest {
		order = "bookings.created_at ASC, bookings.id ASC"
	}
	var bookings []domain.Booking
	tx := r.filtered(ctx, branchID, f, true).Preload("Customer").Order(order)
	tx = withNames(tx)
	if limit > 0 {
		tx = tx.Offset((max(page, 1) - 1) * limit).Limit(limit)
	}
	if err := tx.Find(&bookings).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}

	rows := make([]output.BookingReportRow, len(bookings))
	ids := make([]uint, 0, len(bookings))
	staffIDs := []uint{}
	for i, b := range bookings {
		rows[i].Booking = b
		ids = append(ids, b.ID)
		if b.CreatedByType == domain.CreatedByStaff && b.CreatedByID != nil {
			staffIDs = append(staffIDs, *b.CreatedByID)
		}
	}
	if len(ids) == 0 {
		return rows, total, nil
	}

	// the bank of each booking's latest payment that took money
	var banks []struct {
		BookingID uint
		Method    string
	}
	err := r.db.WithContext(ctx).Raw("SELECT DISTINCT ON (booking_id) booking_id, method FROM payments "+
		"WHERE booking_id IN ? AND status IN "+moneyTaken+" ORDER BY booking_id, id DESC", ids).Scan(&banks).Error
	if err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	bank := map[uint]string{}
	for _, b := range banks {
		bank[b.BookingID] = b.Method
	}

	// staff who count as admins: owners, company admins and this branch's managers
	admin := map[uint]bool{}
	if len(staffIDs) > 0 {
		var admins []uint
		err := r.db.WithContext(ctx).Raw("SELECT u.id FROM users u WHERE u.id IN ? AND (u.company_role IN ('owner','admin') "+
			"OR EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.id AND m.branch_id = ? AND m.role = ?))",
			staffIDs, branchID, domain.MemberManager).Scan(&admins).Error
		if err != nil {
			return nil, 0, exception.HandleGormError(err)
		}
		for _, id := range admins {
			admin[id] = true
		}
	}
	for i := range rows {
		b := rows[i].Booking
		rows[i].Bank = bank[b.ID]
		rows[i].MadeByAdmin = b.CreatedByID != nil && admin[*b.CreatedByID]
	}
	return rows, total, nil
}

func (r *bookingReportRepository) Amounts(ctx context.Context, branchID uint, f output.BookingReportFilter) ([]output.EventAmounts, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	var out []output.EventAmounts
	err := r.filtered(ctx, branchID, f, false).
		Select("bookings.currency AS currency, " +
			"COALESCE(SUM(bookings.total_price) FILTER (WHERE bookings.status IN ('confirmed','completed')), 0) AS success, " +
			"COALESCE(SUM(bookings.total_price) FILTER (WHERE bookings.status = 'cancelled'), 0) AS cancelled").
		Group("bookings.currency").Order("bookings.currency").Scan(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *bookingReportRepository) TablesOnDay(ctx context.Context, branchID uint, from, to time.Time) (int, int, error) {
	if branchID == 0 {
		return 0, 0, errNoScope
	}
	var total, taken int64
	db := r.db.WithContext(ctx)
	if err := db.Model(&domain.Resource{}).
		Where("branch_id = ? AND kind = ? AND active = ?", branchID, domain.ResourceKindTable, true).Count(&total).Error; err != nil {
		return 0, 0, exception.HandleGormError(err)
	}
	err := db.Raw("SELECT COUNT(DISTINCT bi.resource_id) FROM booking_items bi "+
		"JOIN bookings b ON b.id = bi.booking_id AND b.deleted_at IS NULL "+
		"JOIN resources r ON r.id = bi.resource_id AND r.kind = ? AND r.active = ? AND r.deleted_at IS NULL "+
		"WHERE b.branch_id = ? AND b.status IN ('pending','confirmed','completed') AND bi.starts_at < ? AND bi.ends_at > ?",
		domain.ResourceKindTable, true, branchID, to, from).Scan(&taken).Error
	return int(total), int(taken), exception.HandleGormError(err)
}
