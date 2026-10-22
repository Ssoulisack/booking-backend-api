package migration

import (
	"backend/internal/core/domain"
	"log"

	"gorm.io/gorm"
)

// DatabaseMigrations creates or updates every table, then adds the rules gorm cannot express.
// The company and its first owner are created afterwards by the install service (config.yaml).
func DatabaseMigrations(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Company{},
		&domain.Branch{},
		&domain.Membership{},
		&domain.Invitation{},
		&domain.PasswordReset{},
		&domain.Media{},
		&domain.Sport{},
		&domain.Resource{},
		&domain.Coach{},
		&domain.OpeningHour{},
		&domain.Closure{},
		&domain.Price{},
		&domain.ClassSession{},
		&domain.BookingRule{},
		&domain.Booking{},
		&domain.BookingItem{},
	); err != nil {
		return err
	}
	if err := noDoubleBooking(db); err != nil {
		return err
	}
	if err := onePhonePerAccount(db); err != nil {
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
