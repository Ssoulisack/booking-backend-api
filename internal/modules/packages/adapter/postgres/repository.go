package postgres

import (
	"backend/internal/modules/packages/domain"
	"backend/internal/modules/packages/port"
	"backend/internal/platform/postgres/exception"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresRepo struct{ db *gorm.DB }

// New stores packages in PostgreSQL
func New(db *gorm.DB) port.Repository { return &postgresRepo{db: db} }

func (r *postgresRepo) Save(ctx context.Context, p *domain.Package, options []domain.Option) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if p.ID == 0 {
			if err := tx.Create(p).Error; err != nil {
				return exception.HandleGormError(err)
			}
		} else if err := tx.Save(p).Error; err != nil {
			return exception.HandleGormError(err)
		}
		keep := []uint{}
		for i := range options {
			options[i].PackageID = p.ID
			var err error
			if options[i].ID == 0 {
				err = tx.Create(&options[i]).Error
			} else {
				// only an option of this package may be changed
				res := tx.Model(&domain.Option{}).Where("id = ? AND package_id = ?", options[i].ID, p.ID).
					Select("Kind", "Amount", "Price", "ValidDays", "Active").Updates(&options[i])
				err = res.Error
				if err == nil && res.RowsAffected != 1 {
					err = gorm.ErrRecordNotFound
				}
			}
			if err != nil {
				return exception.HandleGormError(err)
			}
			keep = append(keep, options[i].ID)
		}
		q := tx.Where("package_id = ?", p.ID)
		if len(keep) > 0 {
			q = q.Where("id NOT IN ?", keep)
		}
		return exception.HandleGormError(q.Delete(&domain.Option{}).Error)
	})
}

func (r *postgresRepo) Get(ctx context.Context, branchID, id uint) (*domain.Package, error) {
	var p domain.Package
	if err := r.db.WithContext(ctx).Where("branch_id = ? AND id = ?", branchID, id).First(&p).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &p, nil
}

func (r *postgresRepo) List(ctx context.Context, branchID uint, activeOnly bool) ([]domain.Package, error) {
	var out []domain.Package
	q := r.db.WithContext(ctx).Where("branch_id = ?", branchID)
	if activeOnly {
		q = q.Where("active = ?", true)
	}
	return out, exception.HandleGormError(q.Order("id DESC").Find(&out).Error)
}

func (r *postgresRepo) Options(ctx context.Context, packageIDs []uint, activeOnly bool) ([]domain.Option, error) {
	var out []domain.Option
	if len(packageIDs) == 0 {
		return out, nil
	}
	q := r.db.WithContext(ctx).Where("package_id IN ?", packageIDs)
	if activeOnly {
		q = q.Where("active = ?", true)
	}
	return out, exception.HandleGormError(q.Order("package_id, kind DESC, amount").Find(&out).Error)
}

func (r *postgresRepo) Delete(ctx context.Context, branchID, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("branch_id = ? AND id = ?", branchID, id).Delete(&domain.Package{})
		if res.Error != nil {
			return exception.HandleGormError(res.Error)
		}
		if res.RowsAffected != 1 {
			return exception.HandleGormError(gorm.ErrRecordNotFound)
		}
		return exception.HandleGormError(tx.Where("package_id = ?", id).Delete(&domain.Option{}).Error)
	})
}

func (r *postgresRepo) Option(ctx context.Context, packageID, id uint) (*domain.Option, error) {
	var o domain.Option
	if err := r.db.WithContext(ctx).Where("package_id = ? AND id = ?", packageID, id).First(&o).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &o, nil
}

// CreatePurchase saves the package with the next number of its branch. The branch row is locked while the number is
// taken, so two sales at the same moment do not get the same one.
func (r *postgresRepo) CreatePurchase(ctx context.Context, cp *domain.CustomerPackage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT id FROM branches WHERE id = ? FOR UPDATE", cp.BranchID).Error; err != nil {
			return exception.HandleGormError(err)
		}
		var last int
		if err := tx.Raw("SELECT COALESCE(MAX(seq), 0) FROM customer_packages WHERE branch_id = ?", cp.BranchID).Scan(&last).Error; err != nil {
			return exception.HandleGormError(err)
		}
		cp.Seq = last + 1
		return exception.HandleGormError(tx.Create(cp).Error)
	})
}

func (r *postgresRepo) SavePurchase(ctx context.Context, cp *domain.CustomerPackage) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(cp).Error)
}

// AdjustPurchase reads the package with FOR UPDATE, lets change edit it, then saves it and its log line together
func (r *postgresRepo) AdjustPurchase(ctx context.Context, branchID, id uint, change func(cp *domain.CustomerPackage) (*domain.Visit, error)) (*domain.CustomerPackage, error) {
	var out domain.CustomerPackage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("branch_id = ? AND id = ?", branchID, id).First(&out).Error; err != nil {
			return exception.HandleGormError(err)
		}
		line, err := change(&out)
		if err != nil {
			return err
		}
		if err := tx.Save(&out).Error; err != nil {
			return exception.HandleGormError(err)
		}
		if line != nil {
			line.CustomerPackageID = out.ID
			return exception.HandleGormError(tx.Create(line).Error)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *postgresRepo) Purchase(ctx context.Context, branchID, id uint) (*domain.CustomerPackage, error) {
	var cp domain.CustomerPackage
	if err := r.db.WithContext(ctx).Where("branch_id = ? AND id = ?", branchID, id).First(&cp).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &cp, nil
}

func (r *postgresRepo) PurchaseForCustomer(ctx context.Context, userID, id uint) (*domain.CustomerPackage, error) {
	var cp domain.CustomerPackage
	if err := r.db.WithContext(ctx).Where("customer_id = ? AND id = ?", userID, id).First(&cp).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &cp, nil
}

func (r *postgresRepo) Purchases(ctx context.Context, branchID, customerID uint, search string, limit int) ([]domain.CustomerPackage, error) {
	var out []domain.CustomerPackage
	q := r.db.WithContext(ctx).Where("branch_id = ?", branchID)
	if customerID != 0 {
		q = q.Where("customer_id = ?", customerID)
	}
	if search = strings.TrimSpace(search); search != "" {
		// a code (P-12, p12, or a short number like 12) is an exact match: the customer reads it off their phone
		if m := codeRe.FindStringSubmatch(search); m != nil && (len(m[1]) < 6 || strings.ContainsAny(strings.ToLower(search), "p-")) {
			n, _ := strconv.Atoi(m[1])
			return out, exception.HandleGormError(q.Where("customer_packages.seq = ?", n).Order("id DESC").Limit(limit).Find(&out).Error)
		}
		// words: each word typed must START a word of the package name or of the customer's name ("package b" finds
		// Package B only, "package" finds them all, "pa" finds Package, "ckage" finds nothing). One letter alone is too
		// short to mean anything. Email only when an @ is typed; the customer number only when typed like CNO0001.
		var conds []string
		var args []any
		if words := searchWords(search); len(words) > 0 && (len(words) > 1 || len([]rune(words[0])) >= 2) {
			var each []string
			for _, w := range words {
				start, inside := w+"%", "% "+w+"%"
				each = append(each, "(customer_packages.name ILIKE ? OR customer_packages.name ILIKE ? OR customer_id IN (SELECT id FROM users WHERE name ILIKE ? OR name ILIKE ?))")
				args = append(args, start, inside, start, inside)
			}
			conds = append(conds, "("+strings.Join(each, " AND ")+")")
		}
		if strings.Contains(search, "@") {
			conds = append(conds, "customer_id IN (SELECT id FROM users WHERE email ILIKE ?)")
			args = append(args, escapeLike(search)+"%")
		}
		if memberRe.MatchString(search) {
			conds = append(conds, "customer_id IN (SELECT id FROM users WHERE member_no ILIKE ?)")
			args = append(args, escapeLike(search)+"%")
		}
		// a phone typed any way: 020 5550 0001, +856 20 55500001, 2055500001 (country code and a leading 0 dropped)
		if d := phoneDigits(search); len(d) >= 6 {
			conds = append(conds, "customer_id IN (SELECT id FROM users WHERE phone LIKE ?)")
			args = append(args, "%"+d+"%")
		}
		if len(conds) == 0 {
			return out, nil // nothing to look for (one letter)
		}
		q = q.Where("("+strings.Join(conds, " OR ")+")", args...)
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	return out, exception.HandleGormError(q.Order("id DESC").Limit(limit).Find(&out).Error)
}

func (r *postgresRepo) PurchasesOf(ctx context.Context, userID uint) ([]domain.CustomerPackage, error) {
	var out []domain.CustomerPackage
	err := r.db.WithContext(ctx).Where("customer_id = ?", userID).Order("id DESC").Limit(200).Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) PendingCount(ctx context.Context, branchID, customerID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&domain.CustomerPackage{}).
		Where("branch_id = ? AND customer_id = ? AND status = ?", branchID, customerID, domain.StatusPendingPayment).Count(&n).Error
	return n, exception.HandleGormError(err)
}

func (r *postgresRepo) TakeVisit(ctx context.Context, id uint) (bool, error) {
	res := r.db.WithContext(ctx).Model(&domain.CustomerPackage{}).
		Where("id = ? AND status = ? AND kind = ? AND times_left > 0", id, domain.StatusActive, domain.OptionTimes).
		UpdateColumn("times_left", gorm.Expr("times_left - 1"))
	return res.RowsAffected == 1, exception.HandleGormError(res.Error)
}

func (r *postgresRepo) ReturnVisit(ctx context.Context, id uint) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Model(&domain.CustomerPackage{}).
		Where("id = ? AND kind = ?", id, domain.OptionTimes).
		UpdateColumn("times_left", gorm.Expr("times_left + 1")).Error)
}

func (r *postgresRepo) AddVisit(ctx context.Context, v *domain.Visit) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(v).Error)
}

func (r *postgresRepo) Visits(ctx context.Context, customerPackageID uint) ([]domain.Visit, error) {
	var out []domain.Visit
	err := r.db.WithContext(ctx).Where("customer_package_id = ?", customerPackageID).Order("id DESC").Limit(500).Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) VisitSum(ctx context.Context, customerPackageID, bookingID uint) (int, error) {
	var sum int
	err := r.db.WithContext(ctx).Model(&domain.Visit{}).Where("customer_package_id = ? AND booking_id = ?", customerPackageID, bookingID).
		Select("COALESCE(SUM(change), 0)").Scan(&sum).Error
	return sum, exception.HandleGormError(err)
}

var codeRe = regexp.MustCompile(`(?i)^\s*p?-?\s*(\d{1,9})\s*$`)

// phoneDigits keeps the digits of what was typed and drops the country code (856) and a leading 0, so it matches the
// way phones are stored (20xxxxxxxx)
func phoneDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	d = strings.TrimPrefix(d, "856")
	return strings.TrimPrefix(d, "0")
}

var memberRe = regexp.MustCompile(`(?i)^[ce]no\d*$`)

// searchWords splits what was typed into words with the SQL wildcards taken out
func searchWords(s string) []string {
	var out []string
	for _, w := range strings.Fields(s) {
		if w = escapeLike(w); w != "" {
			out = append(out, w)
		}
	}
	return out
}

func escapeLike(s string) string { return strings.NewReplacer("%", "", "_", "").Replace(s) }

func (r *postgresRepo) PaidBetween(ctx context.Context, branchID uint, from, to time.Time) ([]domain.CustomerPackage, error) {
	var out []domain.CustomerPackage
	err := r.db.WithContext(ctx).Where("branch_id = ? AND payment_status = ? AND paid_at >= ? AND paid_at < ?", branchID, "paid", from, to).
		Order("paid_at ASC, id").Find(&out).Error
	return out, exception.HandleGormError(err)
}

func (r *postgresRepo) PurchasesByIDs(ctx context.Context, branchID uint, ids []uint) ([]domain.CustomerPackage, error) {
	var out []domain.CustomerPackage
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.WithContext(ctx).Where("branch_id = ? AND id IN ?", branchID, ids).Find(&out).Error
	return out, exception.HandleGormError(err)
}
