// Package port: the module's two edges. Input is what the module offers; output is what it needs.
package port

import (
	coredto "backend/internal/core/dto"
	"backend/internal/modules/events/domain"
	"context"
	"time"
)

// Repository stores the module's tables
type Repository interface {
	Create(ctx context.Context, e *domain.Event) error
	Update(ctx context.Context, e *domain.Event) error
	// Get returns the event of the branch, or a not-found error
	Get(ctx context.Context, branchID, id uint) (*domain.Event, error)
	// List: the branch's events, newest first
	List(ctx context.Context, branchID uint) ([]domain.Event, error)
	Delete(ctx context.Context, branchID, id uint) error
	// PublishedOverlapping: published events of the branch that run on any day of [from, to], except exceptID
	PublishedOverlapping(ctx context.Context, branchID uint, from, to string, exceptID uint) ([]domain.Event, error)
	// On: the published event running on day, or nil
	On(ctx context.Context, branchID uint, day string) (*domain.Event, error)
	// Upcoming: published events not over yet (ends_on >= today), soonest first
	Upcoming(ctx context.Context, branchID uint, today string) ([]domain.Event, error)

	// OnDay: every published event running on day (more than one when they are in different areas)
	OnDay(ctx context.Context, branchID uint, day string) ([]domain.Event, error)

	// AreaIDs: the areas the event is held in (none = every area)
	AreaIDs(ctx context.Context, eventID uint) ([]uint, error)
	ReplaceAreas(ctx context.Context, eventID uint, areaIDs []uint) error
	// Lineup: the event's program, in order
	Lineup(ctx context.Context, eventID uint) ([]domain.LineupRow, error)
	ReplaceLineup(ctx context.Context, eventID uint, rows []domain.LineupRow) error

	Prices(ctx context.Context, eventID uint) ([]domain.Price, error)
	ReplacePrices(ctx context.Context, eventID uint, prices []domain.Price) error

	// BookingCounts: the event's bookings by status
	BookingCounts(ctx context.Context, eventID uint) (map[string]int64, error)

	PayInfo(ctx context.Context, branchID uint) (*domain.PayInfo, error) // nil when never set
	SavePayInfo(ctx context.Context, p *domain.PayInfo) error
}

// Booker is what the module needs from the booking service: store a booking, and when the venue is
// open on a day
type Booker interface {
	CreateEventBooking(ctx context.Context, in coredto.EventBookingInput) (*coredto.CreatedBookingDTO, error)
	OpenOn(ctx context.Context, branchID uint, date string) (start, end time.Time, open bool, err error)
}

// Notifier tells a customer what happened to their booking (the booking service sends the email)
type Notifier interface {
	// BookingReleased: the unpaid booking was cancelled by the job because it was not paid in time
	BookingReleased(ctx context.Context, branchID, bookingID uint)
}
