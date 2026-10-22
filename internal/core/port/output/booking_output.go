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
	ListForCustomer(ctx context.Context, customerID uint, q ListQuery) ([]domain.Booking, int64, error)
	// Cancel marks the booking cancelled and frees its items, in one transaction
	Cancel(ctx context.Context, b *domain.Booking, reason string, at time.Time) error

	// ActiveItems lists active items of a resource or coach overlapping [from, to)
	ActiveItems(ctx context.Context, branchID uint, ownerType string, ownerID uint, from, to time.Time) ([]domain.BookingItem, error)
	// CountActive counts pending / confirmed bookings ending after now, by customer or by guest phone
	CountActiveForCustomer(ctx context.Context, branchID, customerID uint, now time.Time) (int64, error)
	CountActiveForGuestPhone(ctx context.Context, branchID uint, phone string, now time.Time) (int64, error)
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
}

// CustomerRow is one customer of a branch
type CustomerRow struct {
	UserID        uint
	MemberNo      *string
	Name          string
	Email         string
	Phone         string
	Bookings      int64
	LastBookingAt time.Time
}
