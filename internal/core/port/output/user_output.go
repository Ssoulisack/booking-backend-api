package output

import (
	"backend/internal/core/domain"
	"context"
	"time"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id uint) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// FindByPhone looks up a normalized phone number (2055551234)
	FindByPhone(ctx context.Context, phone string) (*domain.User, error)
	// PhoneTaken reports whether another account (not exceptID) already has this number
	PhoneTaken(ctx context.Context, phone string, exceptID uint) (bool, error)
	// EmailTaken reports whether another account (not exceptID) already has this email
	EmailTaken(ctx context.Context, email string, exceptID uint) (bool, error)
	// UpdateProfile saves name, email (and username), phone, nationality and the stand-in flag
	UpdateProfile(ctx context.Context, u *domain.User) error
	// List supports the filter "company_role" and a keyword matched against name and email, or exactly
	// against the phone (any spelling) or the customer number (C0042); newest accounts first
	List(ctx context.Context, q ListQuery) ([]domain.User, int64, error)
	SetStatus(ctx context.Context, id uint, status bool) error
	// SetCompanyRole sets "admin" or "" (the owner changes only through TransferOwnership)
	SetCompanyRole(ctx context.Context, id uint, role string) error
	// TransferOwnership makes toID the owner and fromID an admin, in one transaction
	TransferOwnership(ctx context.Context, fromID, toID uint) error
	SetPassword(ctx context.Context, id uint, hash []byte, changedAt time.Time) error
}
