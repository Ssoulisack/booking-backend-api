package output

import (
	"backend/internal/core/domain"
	"context"
	"time"
)

// BookingReportFilter picks the bookings of the table and event reports
type BookingReportFilter struct {
	Event         bool       // false: table bookings (no event); true: event bookings
	EventID       uint       // event reports: one event, 0 = all events
	From, To      *time.Time // the booking's day: starts_at in [From, To)
	Status        string
	PaymentStatus string
	Bank          string // the bank of the online payment ("bcel", ...)
	Keyword       string // table name, booking code, phone or full name (part of it)
	Oldest        bool   // sort by booking time, oldest first (newest first by default)
}

// BookingReportRow is one booking with what the report needs besides the booking itself
type BookingReportRow struct {
	Booking     domain.Booking
	Bank        string // the bank of its online payment, "" when there is none
	MadeByAdmin bool   // a staff booking made by an owner / admin / branch manager
}

// EventAmounts: what the filtered event bookings are worth, per currency
type EventAmounts struct {
	Currency  string
	Success   int64 // confirmed + completed
	Cancelled int64
}

// BookingReportRepository reads bookings for the reports (read-only)
type BookingReportRepository interface {
	// Rows returns one page, newest booking time first unless f.Oldest, and the total number of matches
	Rows(ctx context.Context, branchID uint, f BookingReportFilter, page, limit int) ([]BookingReportRow, int64, error)
	// Amounts adds up the matches by currency, ignoring f.Status (both totals are always wanted)
	Amounts(ctx context.Context, branchID uint, f BookingReportFilter) ([]EventAmounts, error)
	// TablesOnDay counts the branch's tables and how many are taken (pending, confirmed or completed
	// bookings, event bookings too) at any time in [from, to)
	TablesOnDay(ctx context.Context, branchID uint, from, to time.Time) (total, taken int, err error)
}
