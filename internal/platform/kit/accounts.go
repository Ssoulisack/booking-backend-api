package kit

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"context"
	"net/mail"
	"strings"
	"time"
)

// MinPasswordLength is the shortest password an account may have
const MinPasswordLength = 8

// ValidPassword refuses a password that is too short
func ValidPassword(password string) error {
	if len(password) < MinPasswordLength {
		return BadRequest("PASSWORD_TOO_SHORT")
	}
	return nil
}

// NormalizeEmail lower-cases and trims an address
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidEmail accepts a plain address such as "info@club.la"
func ValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// SyncMemberNo gives the account the member number for its role (CNO customer, ENO team) and
// keeps it on u for the response. Call it once an account is created or its role has changed.
func SyncMemberNo(ctx context.Context, users output.UserRepository, u *domain.User) error {
	no, err := users.SyncMemberNo(ctx, u.ID)
	if err != nil {
		return err
	}
	u.MemberNo = &no
	return nil
}

// UserDTO is the account as the API shows it
func UserDTO(u *domain.User) dto.UserDTO {
	return dto.UserDTO{
		ID:             u.ID,
		Email:          u.Email,
		Avatar:         u.Avatar,
		Name:           u.Name,
		Username:       u.Username,
		Phone:          u.Phone,
		CompanyRole:    u.CompanyRole,
		Nationality:    u.Nationality,
		EmailGenerated: u.EmailGenerated,
		MemberNo:       domain.MemberNoOf(u.MemberNo),
		Status:         u.Status,
		CreatedAt:      u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      u.UpdatedAt.Format(time.RFC3339),
	}
}

// OpenBranchIDs checks that every id is an existing, open branch (at least one, no repeats)
func OpenBranchIDs(ctx context.Context, branches output.BranchRepository, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, BadRequest("CHOOSE_AT_LEAST_ONE_BRANCH")
	}
	seen := map[uint]bool{}
	out := []uint{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		b, err := branches.GetByID(ctx, id)
		if err != nil {
			return nil, OrBadRequest(err, "BRANCH_NOT_FOUND")
		}
		if b.Closed {
			return nil, BadRequest("BRANCH_CLOSED")
		}
		out = append(out, id)
	}
	return out, nil
}
