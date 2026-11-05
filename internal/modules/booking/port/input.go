// Package port: the module's input edge. Handlers (and the other modules) call these interfaces.
package port

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
	// without the private link: the booking code and the phone it was made with
	FindBooking(ctx context.Context, slug string, req dto.FindBookingRequest) (*dto.BookingDTO, error)
	FindCancel(ctx context.Context, slug string, req dto.FindBookingRequest) (*dto.BookingDTO, error)
	GuestCancel(ctx context.Context, slug, token string, req dto.CancelBookingRequest) (*dto.BookingDTO, error)
	// event days: "I paid and sent the slip" (by the private link, or for a logged-in customer's own booking)
	SlipSent(ctx context.Context, slug, token string) (*dto.BookingDTO, error)
	MySlipSent(ctx context.Context, userID, id uint) (*dto.BookingDTO, error)

	MyBookings(ctx context.Context, userID uint, q dto.ListQuery) (*dto.PagedResult[dto.BookingDTO], error)
	MyBooking(ctx context.Context, userID, id uint) (*dto.BookingDTO, error)
	CancelMine(ctx context.Context, userID, id uint, req dto.CancelBookingRequest) (*dto.BookingDTO, error)
	// ClaimMine adds a booking made without an account (a guest) to the customer's account: the booking's phone
	// must be the phone on the account
	ClaimMine(ctx context.Context, userID uint, req dto.ClaimBookingRequest) (*dto.BookingDTO, error)
}

// DeskService is the branch team's day-to-day work on bookings
type DeskService interface {
	ListBookings(ctx context.Context, branchID uint, q dto.BookingQuery) (*dto.PagedResult[dto.BookingDTO], error)
	GetBooking(ctx context.Context, branchID, id uint) (*dto.BookingDTO, error)
	// DeskAvailability lists the free starts staff may book (no notice or days-ahead limit)
	DeskAvailability(ctx context.Context, branchID uint, q dto.AvailabilityQuery) (*dto.AvailabilityDTO, error)
	// CreateForCustomer books on behalf of a customer or a walk-in; staff skip only the timing limits
	CreateForCustomer(ctx context.Context, branchID uint, actor *domain.User, req dto.StaffBookingRequest) (*dto.BookingDTO, error)
	ChangeStatus(ctx context.Context, branchID, id uint, req dto.StatusChangeRequest) (*dto.BookingDTO, error)
	SetPayment(ctx context.Context, branchID uint, actor *domain.User, id uint, req dto.PaymentRequest) (*dto.BookingDTO, error)
	Calendar(ctx context.Context, branchID uint, date string, days int) (*dto.CalendarDTO, error)
	Customers(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.CustomerDTO], error)
	// FindCustomer finds an account by phone, customer number or email (shown before booking for it)
	FindCustomer(ctx context.Context, branchID uint, q string) (*dto.CustomerMatchDTO, error)
}
