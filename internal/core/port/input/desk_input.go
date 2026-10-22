package input

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
)

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

// ReportService summarises a branch's bookings and money for a period
type ReportService interface {
	Bookings(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.BookingsReportDTO, error)
	Utilisation(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.UtilisationDTO, error)
	Finance(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.FinanceDTO, error)
	Unpaid(ctx context.Context, branchID uint, q dto.ReportQuery) ([]dto.UnpaidRow, error)
	CashUp(ctx context.Context, branchID uint, date string) (*dto.CashUpDTO, error)
	// Company adds up every branch (closed ones included) for the period, with one row per branch
	Company(ctx context.Context, q dto.ReportQuery) (*dto.CompanyReportDTO, error)
}
