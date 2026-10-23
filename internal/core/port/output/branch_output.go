package output

import (
	"backend/internal/core/domain"
	"context"
	"time"
)

type CompanyRepository interface {
	// Get returns the install's company; NOT_FOUND before the first start created it
	Get(ctx context.Context) (*domain.Company, error)
	Create(ctx context.Context, c *domain.Company) error
	Update(ctx context.Context, c *domain.Company) error
}

type BranchRepository interface {
	Create(ctx context.Context, b *domain.Branch) error
	// Clone creates b and copies the setup of branch sourceID into it in one transaction: sports,
	// spaces, coaches, opening hours, prices, booking rules, weekly class timetables and closed
	// days still to come. Bookings, customers, team, made classes and uploaded pictures stay behind.
	Clone(ctx context.Context, sourceID uint, b *domain.Branch) error
	// GetByID, GetBySlug and List fill the settings in force from the company (Branch.WithCompany)
	GetByID(ctx context.Context, id uint) (*domain.Branch, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Branch, error)
	// List supports the filters "published" and "closed" and a keyword on name / slug; oldest first
	List(ctx context.Context, q ListQuery) ([]domain.Branch, int64, error)
	Update(ctx context.Context, b *domain.Branch) error
	SlugTaken(ctx context.Context, slug string, exceptID uint) (bool, error)
}

type MembershipRepository interface {
	Create(ctx context.Context, m *domain.Membership) error
	// Get returns (nil, nil) when the user does not work at the branch
	Get(ctx context.Context, branchID, userID uint) (*domain.Membership, error)
	ListByBranch(ctx context.Context, branchID uint) ([]domain.Membership, error)
	ListByUser(ctx context.Context, userID uint) ([]domain.Membership, error)
	// ListAll is every membership with its user and branch (the team page)
	ListAll(ctx context.Context) ([]domain.Membership, error)
	// Replace sets a person's branches and role in one transaction (no branches: removed from all)
	Replace(ctx context.Context, userID uint, role string, branchIDs []uint, active bool) error
}

type InvitationRepository interface {
	Create(ctx context.Context, inv *domain.Invitation) error
	// GetByTokenHash returns (nil, nil) when no invitation matches
	GetByTokenHash(ctx context.Context, hash string) (*domain.Invitation, error)
	GetByID(ctx context.Context, id uint) (*domain.Invitation, error)
	// ListPending lists invitations not yet accepted and not expired at now
	ListPending(ctx context.Context, now time.Time) ([]domain.Invitation, error)
	PendingForEmail(ctx context.Context, email string, now time.Time) (*domain.Invitation, error)
	Update(ctx context.Context, inv *domain.Invitation) error
	Delete(ctx context.Context, id uint) error
}

type PasswordResetRepository interface {
	Create(ctx context.Context, r *domain.PasswordReset) error
	// GetByTokenHash returns (nil, nil) when no reset matches
	GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordReset, error)
	// UseAll marks every unused reset of the user as used
	UseAll(ctx context.Context, userID uint, at time.Time) error
}

// Mailer sends the system's transactional emails; companyName signs them
type Mailer interface {
	// SendInvitation asks someone to join the team; where is "every branch" or the branch names
	SendInvitation(to, companyName, inviterName, role, where, acceptURL string) error
	SendPasswordReset(to, name, resetURL string) error
	// SendWelcome gives the first owner the link to choose a password (first start)
	SendWelcome(to, name, companyName, setPasswordURL string) error
	// SendBookingLink gives a guest the private link to view or cancel their booking
	SendBookingLink(to, branchName, code, link string) error
	// SendBookingUpdate tells a customer that staff confirmed or cancelled their booking
	SendBookingUpdate(to, name, branchName, code, status, reason, link string) error
}
