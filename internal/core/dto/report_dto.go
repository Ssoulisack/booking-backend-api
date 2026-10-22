package dto

import "time"

// ReportQuery is a date range (YYYY-MM-DD, both days included, branch timezone)
type ReportQuery struct {
	From string `query:"from"`
	To   string `query:"to"`
}

type Money struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type DayCount struct {
	Date     string         `json:"date"`
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

type SportCount struct {
	SportID uint   `json:"sport_id"`
	Name    string `json:"name"`
	Total   int    `json:"total"`
}

type TopCustomer struct {
	Name     string  `json:"name"`
	Contact  string  `json:"contact"` // email for accounts, phone for walk-ins and guests
	Bookings int     `json:"bookings"`
	Value    []Money `json:"value"`
}

// BookingsReportDTO counts bookings starting in the period
type BookingsReportDTO struct {
	From         string         `json:"from"`
	To           string         `json:"to"`
	Total        int            `json:"total"`
	ByStatus     map[string]int `json:"by_status"`
	PerDay       []DayCount     `json:"per_day"`
	PerSport     []SportCount   `json:"per_sport"`
	BookedValue  []Money        `json:"booked_value"` // all but cancelled
	TopCustomers []TopCustomer  `json:"top_customers"`
}

type ResourceUse struct {
	ResourceID    uint    `json:"resource_id"`
	Name          string  `json:"name"`
	OpenMinutes   int     `json:"open_minutes"`
	BookedMinutes int     `json:"booked_minutes"`
	Percent       float64 `json:"percent"`
}

// UtilisationDTO is booked time over open time per resource (classes count as booked)
type UtilisationDTO struct {
	From      string        `json:"from"`
	To        string        `json:"to"`
	Resources []ResourceUse `json:"resources"`
}

// FinanceTotals uses the definitions in the phase 4 timeline file
type FinanceTotals struct {
	Currency       string `json:"currency"`
	BookedValue    int64  `json:"booked_value"`
	Paid           int64  `json:"paid"`
	Outstanding    int64  `json:"outstanding"`
	UpcomingUnpaid int64  `json:"upcoming_unpaid"`
	LostCancelled  int64  `json:"lost_cancelled"`
	LostNoShow     int64  `json:"lost_no_show"`
}

type FinanceDay struct {
	Date        string `json:"date"`
	Currency    string `json:"currency"`
	BookedValue int64  `json:"booked_value"`
	Paid        int64  `json:"paid"`
}

type FinanceDTO struct {
	From   string          `json:"from"`
	To     string          `json:"to"`
	Totals []FinanceTotals `json:"totals"`
	PerDay []FinanceDay    `json:"per_day"`
}

// UnpaidRow is a booking that has ended but is not paid yet
type UnpaidRow struct {
	BookingID uint      `json:"booking_id"`
	Code      string    `json:"code"`
	StartsAt  time.Time `json:"starts_at"`
	Status    string    `json:"status"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Amount    int64     `json:"amount"`
	Currency  string    `json:"currency"`
}

type CashUpMethod struct {
	Method   string `json:"method"`
	Currency string `json:"currency"`
	Count    int    `json:"count"`
	Amount   int64  `json:"amount"`
}

type CashUpStaff struct {
	UserID   uint   `json:"user_id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Count    int    `json:"count"`
	Amount   int64  `json:"amount"`
}

type CashUpPayment struct {
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Method   string    `json:"method"`
	Amount   int64     `json:"amount"`
	Currency string    `json:"currency"`
	PaidAt   time.Time `json:"paid_at"`
	PaidBy   string    `json:"paid_by"`
}

// CashUpDTO is the money marked paid on one day, to check against the till
type CashUpDTO struct {
	Date     string          `json:"date"`
	Totals   []Money         `json:"totals"`
	ByMethod []CashUpMethod  `json:"by_method"`
	ByStaff  []CashUpStaff   `json:"by_staff"`
	Payments []CashUpPayment `json:"payments"`
}

// CompanyReportDTO adds up every branch for a period: money per currency and bookings
type CompanyReportDTO struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	Bookings int               `json:"bookings"`
	Totals   []FinanceTotals   `json:"totals"`
	Branches []BranchReportRow `json:"branches"`
}

// BranchReportRow is one branch's share of the company report
type BranchReportRow struct {
	BranchID uint            `json:"branch_id"`
	Name     string          `json:"name"`
	Status   string          `json:"status"`
	Bookings int             `json:"bookings"`
	Totals   []FinanceTotals `json:"totals"`
}
