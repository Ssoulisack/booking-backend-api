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
	// restaurant branches: the chosen tables and the group size (no duration)
	TableIDs       []uint     `json:"table_ids"`
	Guests         int        `json:"guests"`
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

// FindBookingRequest finds a booking without its private link: its code and the phone it was made with
type FindBookingRequest struct {
	Code   string `json:"code"`
	Phone  string `json:"phone"`
	Reason string `json:"reason"` // cancel only
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
	// false once let go: a cancelled booking, or a table the group was moved away from before using it
	Active bool `json:"active"`
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
	MemberNo         string           `json:"member_no,omitempty"` // the booker's number (CNO / ENO); none for a guest
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
	// restaurant tables: the group size, and the time the group loses its table if not there
	Guests int `json:"guests,omitempty"`
	// event days: the event bought, pay by (hold ends), and when the customer said the slip was sent
	EventID    *uint      `json:"event_id,omitempty"`
	PayBy      *time.Time `json:"pay_by,omitempty"`
	SlipSentAt *time.Time `json:"slip_sent_at,omitempty"`
	ArriveBy   *time.Time `json:"arrive_by,omitempty"`
	ArrivedAt  *time.Time `json:"arrived_at,omitempty"`
	// arrive_by: a bar booking for the evening (its start is only "from opening"); time: a start time
	BookingStyle string `json:"booking_style,omitempty"`
	// restaurant tables: the floor of the area (sport_name) the tables are in; 0 = ground floor
	AreaFloor *int `json:"area_floor,omitempty"`
	// the zones the tables stand in (e.g. "Smoking"), when booked
	Zones string `json:"zones,omitempty"`
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
	EventID       uint   `query:"event_id"`
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
	UserID   uint   `json:"user_id"`
	MemberNo string `json:"member_no"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email,omitempty"`
}

type StatusChangeRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// MoveTablesRequest puts a restaurant group at other tables
type MoveTablesRequest struct {
	TableIDs []uint `json:"table_ids"`
}

// WalkInRequest seats a group that came without booking; name and phone are optional
type WalkInRequest struct {
	TableIDs []uint `json:"table_ids"`
	Guests   int    `json:"guests"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Notes    string `json:"notes"`
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
	Guests     int       `json:"guests,omitempty"`  // restaurant bookings: people in the group
	Arrived    bool      `json:"arrived,omitempty"` // staff marked the group as arrived
}

type CalendarLane struct {
	OwnerType string `json:"owner_type"`
	OwnerID   uint   `json:"owner_id"`
	Name      string `json:"name"`
	// the sport (sport branches) or the area (restaurants) a court / table belongs to; 0 for coaches
	GroupID   uint            `json:"group_id"`
	GroupName string          `json:"group_name"`
	Floor     int             `json:"floor"` // the area's floor (restaurants)
	Seats     int             `json:"seats"` // restaurant tables
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
	MemberNo      string    `json:"member_no"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	Bookings      int64     `json:"bookings"`
	LastBookingAt time.Time `json:"last_booking_at"`
}

// ---------- restaurant tables ----------

// TableAvailabilityQuery asks for the free times and tables of a day for a group
type TableAvailabilityQuery struct {
	Date   string `query:"date"`
	Guests int    `query:"guests"`
}

type TableAvailabilityDTO struct {
	Date            string `json:"date"`
	Timezone        string `json:"timezone"`
	Guests          int    `json:"guests"`
	MaxGuestsOnline int    `json:"max_guests_online"`
	LateMinutes     int    `json:"late_minutes"`
	// set when an event takes over this day: tables are sold for the event, not booked free
	Event *EventRefDTO `json:"event,omitempty"`
	// time: customers pick one of Times; arrive_by: Times has the one evening, nothing to pick
	BookingStyle string         `json:"booking_style"`
	Times        []TableTimeDTO `json:"times"` // only times where the group fits somewhere
}

// TableTimeDTO is one start time with the tables free for the whole booking
type TableTimeDTO struct {
	StartsAt     time.Time `json:"starts_at"`
	ArriveBy     time.Time `json:"arrive_by"` // not there by then: the table goes to others
	FreeTableIDs []uint    `json:"free_table_ids"`
	// no free tables can seat the group on real seats: one with extra chairs may be chosen
	ExtraChairsAllowed bool `json:"extra_chairs_allowed"`
}

// PublicAreaDTO is a restaurant area as customers see it: its plan, tables and objects
type PublicAreaDTO struct {
	ID         uint             `json:"id"`
	Name       string           `json:"name"`
	Floor      int              `json:"floor"`
	PlanWidth  int              `json:"plan_width"`
	PlanHeight int              `json:"plan_height"`
	Tables     []PublicTableDTO `json:"tables"`
	Items      []FloorItemDTO   `json:"items"`
}

type PublicTableDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	TableFields
}

// EventRefDTO tells a customer that an event takes over the day they asked about
type EventRefDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// EventBookingInput is what the events module hands the booking service to store: who, which tables
// at which price, and how long to hold them. The events module decided all of that already.
type EventBookingInput struct {
	Slug        string
	Actor       *domain.User
	Guest       *GuestInfo
	Notes       string
	EventID     uint
	Date        string // YYYY-MM-DD, branch time zone
	Guests      int
	HoldMinutes int
	Tables      []EventTableSale
}

// EventTableSale is one table sold, with its price
type EventTableSale struct {
	ID    uint
	Price int64
}
