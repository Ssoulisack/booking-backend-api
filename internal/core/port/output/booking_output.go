package output

import (
	"backend/internal/core/domain"
	"context"
	"time"
)

type BookingRepository interface {
	// Create stores the booking with its items in one transaction. Items overlapping an
	// active item of the same resource or coach are refused by the database (409 SLOT_NOT_AVAILABLE).
	Create(ctx context.Context, b *domain.Booking) error
	// CreateClassBooking locks the class session, checks that seats are still free, then stores
	// the booking; two customers cannot take the last seat at the same time (409 CLASS_FULL)
	CreateClassBooking(ctx context.Context, b *domain.Booking, classID uint, seats int) error
	GetByID(ctx context.Context, branchID, id uint) (*domain.Booking, error)
	GetForCustomer(ctx context.Context, customerID, id uint) (*domain.Booking, error)
	// GetByGuestToken returns (nil, nil) when no booking of the branch has that token
	GetByGuestToken(ctx context.Context, branchID uint, tokenHash string) (*domain.Booking, error)
	// GetByCode returns (nil, nil) when no booking of the branch has that code
	GetByCode(ctx context.Context, branchID uint, code string) (*domain.Booking, error)
	// GetByCodeAnywhere returns (nil, nil) when no booking at all has that code (codes are unique across branches)
	GetByCodeAnywhere(ctx context.Context, code string) (*domain.Booking, error)
	// SetCustomer attaches a booking made without an account to an account; only while it has none (409 otherwise)
	SetCustomer(ctx context.Context, b *domain.Booking, customerID uint) error
	ListForCustomer(ctx context.Context, customerID uint, q ListQuery) ([]domain.Booking, int64, error)
	// Cancel marks the booking cancelled and frees its items, in one transaction
	Cancel(ctx context.Context, b *domain.Booking, reason string, at time.Time) error

	// ActiveItems lists active items of a resource or coach overlapping [from, to)
	ActiveItems(ctx context.Context, branchID uint, ownerType string, ownerID uint, from, to time.Time) ([]domain.BookingItem, error)
	// CountActive counts pending / confirmed bookings ending after now, by customer or by guest phone
	CountActiveForCustomer(ctx context.Context, branchID, customerID uint, now time.Time) (int64, error)
	CountActiveForGuestPhone(ctx context.Context, branchID uint, phone string, now time.Time) (int64, error)
	// CountOpenWaitingForPhone: bookings (not event tables) still waiting for approval and unpaid, of a guest with this
	// phone or an account with it, at every branch
	CountOpenWaitingForPhone(ctx context.Context, phone string, now time.Time) (int64, error)
	// SeatsTaken sums the active seats booked per class session
	SeatsTaken(ctx context.Context, branchID uint, classIDs []uint) (map[uint]int, error)
	// CountFutureForOwner counts active customer items of a resource or coach ending after now
	CountFutureForOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint, now time.Time) (int64, error)

	// ---- branch dashboard ----

	// List filters a branch's bookings, soonest start first
	List(ctx context.Context, branchID uint, f BookingFilter, q ListQuery) ([]domain.Booking, int64, error)
	// InRange returns every booking of any status overlapping [from, to), with items and people loaded
	InRange(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Booking, error)
	// PaidBetween returns bookings marked paid in [from, to), with who took the payment
	PaidBetween(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Booking, error)
	// SetStatus changes the status of a booking that keeps its time (confirm, complete, no-show)
	SetStatus(ctx context.Context, b *domain.Booking, status string) error
	// SetPayment saves the payment fields
	SetPayment(ctx context.Context, b *domain.Booking) error
	// SettlePaidOnline records an online (bank) payment in ONE transaction with the booking row locked, so a cancel or
	// another payment arriving at the same moment cannot leave half of it: a cancelled booking gives ErrNotPayable,
	// an already paid one ErrAlreadyPaid, and nothing is written then. An event booking still waiting is confirmed
	// in the same step (confirmed = true). It returns the booking as saved.
	SettlePaidOnline(ctx context.Context, branchID, id uint, at time.Time) (bk *domain.Booking, confirmed bool, err error)

	// ---- event days ----

	// ActiveResourceItems lists the active table / court items of a branch overlapping [from, to)
	ActiveResourceItems(ctx context.Context, branchID uint, from, to time.Time) ([]domain.BookingItem, error)
	// NormalTableBookings lists the codes of waiting or confirmed NORMAL table bookings (not event bookings) that hold a
	// table in [from, to); areaIDs limits it to tables of those areas (none: every table). An event may not take over
	// those days while these exist.
	NormalTableBookings(ctx context.Context, branchID uint, from, to time.Time, areaIDs []uint) ([]string, error)
	// EventHoldsExpired lists unpaid event bookings whose pay-by time has passed without a slip sent
	EventHoldsExpired(ctx context.Context, now time.Time) ([]domain.Booking, error)
	// ReleaseUnpaid cancels a booking only if it is STILL waiting and unpaid, in one transaction with its row locked (the
	// same lock a payment takes), so a payment arriving at the same moment is never cancelled. false: it was paid,
	// confirmed or cancelled meanwhile, and nothing was changed.
	ReleaseUnpaid(ctx context.Context, b *domain.Booking, reason string, at time.Time) (bool, error)
	// SetPayBy makes the pay-by time of an event booking still waiting for money (no slip sent) the moment the
	// customer's online payment (its QR) runs out: not paid by then, the tables go back on sale
	SetPayBy(ctx context.Context, id uint, until time.Time) error
	// SetSlipSent records when the customer said the slip was sent (only a pending, unpaid event booking)
	// (via: "whatsapp" or "telegram"; a picture sent again just updates the time)
	SetSlipSent(ctx context.Context, b *domain.Booking, at time.Time, via string) error

	// ---- restaurant tables ----

	// Finish completes a booking (bill closed) or marks it no-show and frees its tables from `at` on
	Finish(ctx context.Context, b *domain.Booking, status string, at time.Time) error
	// SetArrived records when a confirmed group came in
	SetArrived(ctx context.Context, b *domain.Booking, at time.Time) error
	// MoveItems frees the booking's tables from `at` on and stores the new ones (b.SportID and b.Zones
	// saved too); 409 SLOT_NOT_AVAILABLE when one of them is taken
	MoveItems(ctx context.Context, b *domain.Booking, at time.Time, items []domain.BookingItem) error
	// TablesToSettle lists confirmed table bookings of every branch whose group is late (not arrived
	// by arrive_by) or whose time is over (closing time passed)
	TablesToSettle(ctx context.Context, now time.Time) ([]domain.Booking, error)
	// Customers lists accounts that booked with the branch, with their booking counts
	// Customers lists accounts who booked at the branch (0: at any branch), most recent first
	Customers(ctx context.Context, branchID uint, keyword string, q ListQuery) ([]CustomerRow, int64, error)
}

// BookingFilter narrows a branch's booking list; zero values do not filter
type BookingFilter struct {
	From, To      *time.Time // bookings overlapping [From, To)
	SportID       uint
	ResourceID    uint
	CoachID       uint
	CustomerID    uint
	Status        string
	PaymentStatus string
	Keyword       string // code, guest name or phone, customer name or email
	EventID       uint   // bookings of one event
	// Waiting: "approval" = pending bookings the team decides on; "payment" = event tables held and not paid, no slip
	// sent yet (the customer still has to pay). Every pending booking is one or the other.
	Waiting string
}

// CustomerRow is one customer of a branch
type CustomerRow struct {
	UserID        uint
	MemberNo      *string
	Name          string
	Email         string
	Verified      bool // the email was proven with a code
	Phone         string
	Bookings      int64
	LastBookingAt time.Time
}

// EventDays tells the booking service whether an event takes over a day of a restaurant (the events
// module implements it); nil, nil means no event that day
type EventDays interface {
	EventOn(ctx context.Context, branchID uint, day string) (*domain.EventDay, error)
}
