package input

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
)

// BookingService is how customers and guests see free times and book on a branch's public page
type BookingService interface {
	Availability(ctx context.Context, slug string, q dto.AvailabilityQuery) (*dto.AvailabilityDTO, error)
	// Create books for actor (a logged-in customer, or nil) or for req.Guest, as the branch's booking mode allows
	Create(ctx context.Context, slug string, actor *domain.User, req dto.CreateBookingRequest) (*dto.CreatedBookingDTO, error)

	GuestView(ctx context.Context, slug, token string) (*dto.BookingDTO, error)
	GuestCancel(ctx context.Context, slug, token string, req dto.CancelBookingRequest) (*dto.BookingDTO, error)

	MyBookings(ctx context.Context, userID uint, q dto.ListQuery) (*dto.PagedResult[dto.BookingDTO], error)
	MyBooking(ctx context.Context, userID, id uint) (*dto.BookingDTO, error)
	CancelMine(ctx context.Context, userID, id uint, req dto.CancelBookingRequest) (*dto.BookingDTO, error)
}
