// Package numbering gives every account its member number (CNO000042 / ENO000007) from two
// database counters. The repository and the start-up migration both use it.
package numbering

import (
	"backend/internal/core/domain"
	"strings"

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
