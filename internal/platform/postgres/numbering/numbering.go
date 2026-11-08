// Package numbering gives every account its member number (CNO000042 / ENO000007) from two
// database counters. The repository and the start-up migration both use it.
package numbering

import (
	"backend/internal/core/domain"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// one counter per kind of number, so customers and the team each count from 1
var sequences = map[string]string{
	domain.CustomerNoPrefix: "customer_no_seq",
	domain.EmployeeNoPrefix: "employee_no_seq",
}

// EnsureSequences creates the two counters; safe on every start
func EnsureSequences(db *gorm.DB) error {
	for _, seq := range sequences {
		if err := db.Exec("CREATE SEQUENCE IF NOT EXISTS " + seq).Error; err != nil {
			return err
		}
	}
	return nil
}

// Sync gives the account the number for its current role and returns it. The team is anyone with
// a company role (owner, admin) or a branch membership (manager, staff); everyone else is a
// customer. A number of the right kind is kept; otherwise the next one of that kind is taken, so
// a customer who joins the team gets a new ENO number and the old CNO number is retired.
func Sync(db *gorm.DB, userID uint) (string, error) {
	var no string
	err := db.Transaction(func(tx *gorm.DB) error {
		var row struct {
			MemberNo *string
			Team     bool
		}
		// FOR UPDATE: two requests for the same account cannot both take a number
		err := tx.Raw(`SELECT member_no,
				(company_role <> '' OR EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = users.id)) AS team
			FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&row).Error
		if err != nil {
			return err
		}
		prefix := domain.CustomerNoPrefix
		if row.Team {
			prefix = domain.EmployeeNoPrefix
		}
		if row.MemberNo != nil && strings.HasPrefix(*row.MemberNo, prefix) {
			no = *row.MemberNo
			return nil
		}
		var n int64
		if err := tx.Raw("SELECT nextval('" + sequences[prefix] + "')").Scan(&n).Error; err != nil {
			return err
		}
		no = domain.FormatMemberNo(prefix, n)
		return tx.Exec("UPDATE users SET member_no = ? WHERE id = ?", no, userID).Error
	})
	return no, err
}

// SyncAll numbers every live account that has no number yet or one of the wrong kind, oldest
// account first (the owner from the install gets ENO000001). Safe on every start: accounts that
// are already right are left alone.
func SyncAll(db *gorm.DB) error {
	var ids []uint
	err := db.Raw(`SELECT id FROM users u WHERE deleted_at IS NULL AND (member_no IS NULL
			OR (member_no LIKE 'ENO%') <> (company_role <> '' OR EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.id)))
		ORDER BY id`).Scan(&ids).Error
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := Sync(db, id); err != nil {
			return err
		}
	}
	return nil
}

// bookingDayLock is the first key of the advisory lock that makes the bookings of one day take turns for a number
const bookingDayLock = 7301

// maxPerDay: the code column holds 12 characters, the date takes 8
const maxPerDay = 9999

// NextBookingCode gives the next booking number of the day: the date and the count of that day, "20261108001",
// "20261108002"... and "20261109001" on the next day. Nothing is stored for it: the count is read from the bookings
// themselves (the highest number of today, plus one), under a lock that lasts until the insert's transaction ends, so
// two bookings never get the same number and a booking that is rolled back gives its number back. The date is the
// Laos date (the shop's day). Past 999 the count simply grows to four digits (202611081000) and the longer number
// counts as the higher one.
func NextBookingCode(tx *gorm.DB, now time.Time) (string, error) {
	loc, err := time.LoadLocation("Asia/Vientiane")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	today := now.In(loc)
	day, tomorrow := today.Format("20060102"), today.AddDate(0, 0, 1).Format("20060102")
	if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", bookingDayLock, int32(today.Year()*10000+int(today.Month())*100+today.Day())).Error; err != nil {
		return "", err
	}
	var last int
	// the index on code serves the range; only the bookings of this one day are looked at (cancelled ones included)
	err = tx.Raw(`SELECT COALESCE(max(substr(code, 9)::int), 0) FROM bookings
		WHERE code >= ? AND code < ? AND code ~ ('^' || ?::text || '[0-9]+$')`, day, tomorrow, day).Scan(&last).Error
	if err != nil {
		return "", err
	}
	if last >= maxPerDay {
		return "", errors.New("BOOKING_NUMBERS_OF_THE_DAY_USED_UP")
	}
	return fmt.Sprintf("%s%03d", day, last+1), nil
}
