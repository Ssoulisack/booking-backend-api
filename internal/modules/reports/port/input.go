// Package port: what the reports module offers to the rest of the app (input ports)
package port

import (
	"backend/internal/core/dto"
	"context"
)

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
