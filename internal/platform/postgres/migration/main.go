package migration

import (
	"backend/internal/core/domain"
	"backend/internal/platform/postgres/numbering"
	"errors"
	"log"

	"gorm.io/gorm"
)

// DatabaseMigrations creates or updates every table, then adds the rules gorm cannot express.
// The company and its first owner are created afterwards by the install service (config.yaml).
// modules are the tables of the modules (each module owns its own; the app lists them).
func DatabaseMigrations(db *gorm.DB, modules ...any) error {
	if err := refuseOldPayments(db); err != nil {
		return err
	}
	if len(modules) > 0 {
		if err := db.AutoMigrate(modules...); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Company{},
		&domain.Branch{},
		&domain.Membership{},
		&domain.PasswordReset{},
		&domain.Media{},
		&domain.Sport{},
		&domain.Resource{},
		&domain.Coach{},
		&domain.OpeningHour{},
		&domain.Closure{},
		&domain.Price{},
		&domain.ClassSession{},
		&domain.ClassTimetable{},
		&domain.BookingRule{},
		&domain.Booking{},
		&domain.BookingItem{},
		&domain.FloorItem{},
	); err != nil {
		return err
	}
	if err := numbering.EnsureBookingDays(db); err != nil {
		return err
	}
	if err := numberPackages(db); err != nil {
		return err
	}
	if err := labelPackageBookings(db); err != nil {
		return err
	}
	if err := noDoubleBooking(db); err != nil {
		return err
	}
	if err := onePhonePerAccount(db); err != nil {
		return err
	}
	// member numbers (CNO / ENO): the counters, then a number for every account that lacks one
	if err := numbering.EnsureSequences(db); err != nil {
		return err
	}
	if err := numbering.SyncAll(db); err != nil {
		return err
	}
	log.Println("🌟 GORM database migrations completed successfully 🚀")
	return nil
}

// noDoubleBooking adds the database rules that make two active items of the same resource, or of
// the same coach, with overlapping times impossible, even when requests arrive at the same moment.
// Safe to run on every start.
func noDoubleBooking(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS btree_gist").Error; err != nil {
		return err
	}
	constraints := []struct{ name, column string }{
		{"booking_items_resource_no_overlap", "resource_id"},
		{"booking_items_coach_no_overlap", "coach_id"},
	}
	for _, c := range constraints {
		var exists int64
		if err := db.Raw("SELECT COUNT(*) FROM pg_constraint WHERE conname = ?", c.name).Scan(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		sql := "ALTER TABLE booking_items ADD CONSTRAINT " + c.name + " EXCLUDE USING gist (" +
			c.column + " WITH =, tstzrange(starts_at, ends_at, '[)') WITH &&) WHERE (active AND " + c.column + " IS NOT NULL)"
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// onePhonePerAccount makes a phone number (20xxxxxxxx) belong to one live account only, because
// people log in with it. Safe to run on every start.
func onePhonePerAccount(db *gorm.DB) error {
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON users (phone) WHERE phone <> '' AND deleted_at IS NULL").Error
}

// refuseOldPayments stops the migration when the database still has `payment_charges`: the short-lived two-table
// layout (a payment plus its charges). Payments are one row per try again; migrating never deletes data, so the
// two old tables, which only ever held test payments, are dropped by hand.
func refuseOldPayments(db *gorm.DB) error {
	var old int64
	err := db.Raw(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name = 'payment_charges'`).Scan(&old).Error
	if err != nil {
		return err
	}
	if old > 0 {
		return errors.New("the payments tables have the old two-table layout (payments + payment_charges). They held test payments only: " +
			"run  DROP TABLE payment_charges; DROP TABLE payments;  in the database, then run the migration again")
	}
	return nil
}

// numberPackages gives every customer package that has no number yet the next one of its branch (in the order they were
// bought), then makes a number unique within its branch. Safe to run on every start.
func numberPackages(db *gorm.DB) error {
	if !db.Migrator().HasTable("customer_packages") {
		return nil
	}
	if err := db.Exec(`UPDATE customer_packages cp SET seq = x.n FROM (
			SELECT id, (SELECT COALESCE(MAX(seq), 0) FROM customer_packages o WHERE o.branch_id = c.branch_id)
				+ ROW_NUMBER() OVER (PARTITION BY branch_id ORDER BY id) AS n
			FROM customer_packages c WHERE seq = 0) x
		WHERE cp.id = x.id`).Error; err != nil {
		return err
	}
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_packages_branch_seq ON customer_packages (branch_id, seq) WHERE seq > 0").Error
}

// labelPackageBookings fills in which package paid a booking made before the label was saved with it. Safe to run on
// every start.
func labelPackageBookings(db *gorm.DB) error {
	if !db.Migrator().HasTable("customer_packages") || !db.Migrator().HasColumn(&domain.Booking{}, "package_label") {
		return nil
	}
	return db.Exec(`UPDATE bookings b SET package_label = cp.name || ' · ' || cp.amount || ' ' || cp.kind, package_code = 'P-' || LPAD(cp.seq::text, 4, '0')
		FROM customer_packages cp
		WHERE b.customer_package_id = cp.id AND b.package_label = '' AND cp.seq > 0`).Error
}
