package dto

import (
	"backend/internal/core/domain"
	"time"
)

// AvailabilityQuery asks for free starts on one date (YYYY-MM-DD, branch timezone) for a
// resource, a coach, or both together. sport_id is needed when only a coach is given.
type AvailabilityQuery struct {
	Date       string `query:"date"`
	ResourceID uint   `query:"resource_id"`
	CoachID    uint   `query:"coach_id"`
	SportID    uint   `query:"sport_id"`
}

type SlotDTO struct {
	StartsAt  time.Time `json:"starts_at"`
	Durations []int     `json:"durations_min"`
}

type AvailabilityDTO struct {
	Date     string               `json:"date"`
	Timezone string               `json:"timezone"`
	Rule     domain.EffectiveRule `json:"rule"`
	Slots    []SlotDTO            `json:"slots"`
	// the price of each allowed length (court + coach together); empty when a price is not set yet
	Currency string           `json:"currency,omitempty"`
	Prices   []LengthPriceDTO `json:"prices"`
}

type LengthPriceDTO struct {
	DurationMin int   `json:"duration_min"`
	Price       int64 `json:"price"`
}

type GuestInfo struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// CreateBookingRequest books a resource and/or coach (starts_at + duration_min), or seats in a
// group class (class_session_id + seats). Guests fill guest; logged-in customers leave it empty.
type CreateBookingRequest struct {
	ResourceID     *uint      `json:"resource_id"`
	CoachID        *uint      `json:"coach_id"`
	SportID        uint       `json:"sport_id"`
	StartsAt       time.Time  `json:"starts_at"`
	DurationMin    int        `json:"duration_min"`
	ClassSessionID *uint      `json:"class_session_id"`
	Seats          int        `json:"seats"`
	Guest          *GuestInfo `json:"guest"`
	Notes          string     `json:"notes"`
}

type CancelBookingRequest struct {
	Reason string `json:"reason"`
}

type BookingItemDTO struct {
	Kind           string    `json:"kind"`
	ResourceID     *uint     `json:"resource_id,omitempty"`
	CoachID        *uint     `json:"coach_id,omitempty"`
	ClassSessionID *uint     `json:"class_session_id,omitempty"`
	ResourceName   string    `json:"resource_name,omitempty"`
	CoachName      string    `json:"coach_name,omitempty"`
	ClassTitle     string    `json:"class_title,omitempty"`
	Seats          int       `json:"seats"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	Price          int64     `json:"price"`
}

type BookingDTO struct {
	ID               uint             `json:"id"`
	Code             string           `json:"code"`
	SportID          uint             `json:"sport_id"`
	SportName        string           `json:"sport_name"`
	BranchName       string           `json:"branch_name"`
	BranchSlug       string           `json:"branch_slug"`
	Status           string           `json:"status"`
	StartsAt         time.Time        `json:"starts_at"`
	EndsAt           time.Time        `json:"ends_at"`
	TotalPrice       int64            `json:"total_price"`
	Currency         string           `json:"currency"`
	PaymentStatus    string           `json:"payment_status"`
	CancellableUntil time.Time        `json:"cancellable_until"`
	CustomerID       *uint            `json:"customer_id,omitempty"`
	CustomerName     string           `json:"customer_name,omitempty"`
	CustomerEmail    string           `json:"customer_email,omitempty"`
	CustomerPhone    string           `json:"customer_phone,omitempty"`
	CustomerNo       string           `json:"customer_no,omitempty"`
	GuestName        string           `json:"guest_name,omitempty"`
	GuestPhone       string           `json:"guest_phone,omitempty"`
	PaymentMethod    string           `json:"payment_method,omitempty"`
	PaidAt           *time.Time       `json:"paid_at,omitempty"`
	CreatedByType    string           `json:"created_by"`
	Notes            string           `json:"notes"`
	CancelledAt      *time.Time       `json:"cancelled_at,omitempty"`
	CancelReason     string           `json:"cancel_reason,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	Items            []BookingItemDTO `json:"items"`
}

// CreatedBookingDTO adds the guest's private link, returned only once at booking time
type CreatedBookingDTO struct {
	BookingDTO
	GuestToken string `json:"guest_token,omitempty"`
	GuestLink  string `json:"guest_link,omitempty"`
}

// ---------- branch dashboard ----------

// BookingQuery filters the dashboard's booking list; from / to are dates (YYYY-MM-DD, branch timezone)
type BookingQuery struct {
	Page          int    `query:"page"`
	Limit         int    `query:"limit"`
	Keyword       string `query:"keyword"`
	From          string `query:"from"`
	To            string `query:"to"`
	SportID       uint   `query:"sport_id"`
	ResourceID    uint   `query:"resource_id"`
	CoachID       uint   `query:"coach_id"`
	CustomerID    uint   `query:"customer_id"`
	Status        string `query:"status"`
	PaymentStatus string `query:"payment_status"`
}

// StaffBookingRequest is a booking made by a member: for an existing customer (customer_email)
// or for a walk-in (guest: name + phone)
type StaffBookingRequest struct {
	CreateBookingRequest
	// customer finds an account by phone, customer number (C0042) or email; customer_email still works
	Customer      string `json:"customer"`
	CustomerEmail string `json:"customer_email"`
}

// CustomerMatchDTO is the account staff found, shown before they book for it
type CustomerMatchDTO struct {
	UserID     uint   `json:"user_id"`
	CustomerNo string `json:"customer_no"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Email      string `json:"email,omitempty"`
}

type StatusChangeRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// PaymentRequest marks a booking paid (with a method) or unpaid
type PaymentRequest struct {
	Status string `json:"status"`
	Method string `json:"method"`
}

// CalendarEntry is one block on a court's or coach's line: a booking, a class or a closure
type CalendarEntry struct {
	Kind       string    `json:"kind"` // booking, class, closure
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	BookingID  uint      `json:"booking_id,omitempty"`
	Code       string    `json:"code,omitempty"`
	Status     string    `json:"status,omitempty"`
	Name       string    `json:"name,omitempty"` // customer or walk-in
	Paid       bool      `json:"paid,omitempty"`
	ClassID    uint      `json:"class_id,omitempty"`
	Title      string    `json:"title,omitempty"`
	SeatsTaken int       `json:"seats_taken,omitempty"`
	Capacity   int       `json:"capacity,omitempty"`
	Reason     string    `json:"reason,omitempty"`
}

type CalendarLane struct {
	OwnerType string          `json:"owner_type"`
	OwnerID   uint            `json:"owner_id"`
	Name      string          `json:"name"`
	Entries   []CalendarEntry `json:"entries"`
}

type CalendarDTO struct {
	From     time.Time      `json:"from"`
	To       time.Time      `json:"to"`
	Timezone string         `json:"timezone"`
	Lanes    []CalendarLane `json:"lanes"`
}

type CustomerDTO struct {
	UserID        uint      `json:"user_id"`
	CustomerNo    string    `json:"customer_no"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	Bookings      int64     `json:"bookings"`
	LastBookingAt time.Time `json:"last_booking_at"`
}
