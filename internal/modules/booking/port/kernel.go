package port

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
	"time"
)

// Booker is who a booking is for: a logged-in customer or a guest, and the team member booking on
// their behalf (if any)
type Booker struct {
	Customer *domain.User
	Guest    *dto.GuestInfo
	Staff    *domain.User
}

// Kernel is what a module that books its own kind of thing (restaurant tables) borrows from the
// booking module: the branch, the opening hours and busy times, who is booking, the booking row with
// its code and private link, the per-customer limit, and the answer for the customer
type Kernel interface {
	Published(ctx context.Context, slug string) (*domain.Branch, error)
	Branch(ctx context.Context, branchID uint) (*domain.Branch, *time.Location, error)
	OpenIntervals(ctx context.Context, branchID uint, ownerType string, ownerID uint, day time.Time) ([]domain.Interval, error)
	BusyIntervals(ctx context.Context, branchID uint, ownerType string, ownerID uint, from, to time.Time) ([]domain.Interval, error)
	WhoBooks(b *domain.Branch, actor *domain.User, guest *dto.GuestInfo) (*Booker, error)
	CleanGuest(g dto.GuestInfo) (*dto.GuestInfo, error)
	NewBooking(b *domain.Branch, who *Booker, sportID uint, start, end time.Time, rule *domain.EffectiveRule, notes string) (*domain.Booking, string, error)
	CheckBookingLimit(ctx context.Context, branchID uint, who *Booker, rule *domain.EffectiveRule) error
	Created(ctx context.Context, b *domain.Branch, bk *domain.Booking, rawToken string, actor *domain.User, who *Booker) *dto.CreatedBookingDTO
	// GetBooking is the desk's view of one booking (the restaurant desk answers with it)
	GetBooking(ctx context.Context, branchID, id uint) (*dto.BookingDTO, error)
}

// TableBooker books restaurant tables for a branch's public page and for staff; the restaurant
// module implements it and the booking module's Create hands restaurant branches to it
type TableBooker interface {
	CreateTableBooking(ctx context.Context, b *domain.Branch, who *Booker, req dto.CreateBookingRequest) (*domain.Booking, string, error)
}
