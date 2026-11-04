package domain

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Booking statuses. Pending and confirmed bookings hold their time; cancelled ones free it.
const (
	BookingPending   = "pending"
	BookingConfirmed = "confirmed"
	BookingCancelled = "cancelled"
	BookingCompleted = "completed"
	BookingNoShow    = "no_show"
)

// Booking item kinds. A class hold is not part of a customer booking: it makes a group class
// occupy its court and coach so nobody can rent them at the same time.
const (
	ItemResource     = "resource"
	ItemCoach        = "coach"
	ItemClassSession = "class_session"
	ItemClassHold    = "class_hold"
)

// Who created a booking
const (
	CreatedByCustomer = "customer"
	CreatedByGuest    = "guest"
	CreatedByStaff    = "staff"
)

// Payment status and the ways a branch can be paid at the venue
const (
	PaymentUnpaid = "unpaid"
	PaymentPaid   = "paid"

	MethodCash     = "cash"
	MethodTransfer = "transfer"
	MethodQR       = "qr"
	MethodOther    = "other"
	MethodOnline   = "online" // paid through a payment gateway (never chosen by staff)
)

// statusChanges lists the allowed one-way moves; cancelled, completed and no-show are final
var statusChanges = map[string][]string{
	BookingPending:   {BookingConfirmed, BookingCancelled},
	BookingConfirmed: {BookingCancelled, BookingCompleted, BookingNoShow},
}

// CanChangeStatus tells whether a booking may move from one status to another
func CanChangeStatus(from, to string) bool {
	for _, s := range statusChanges[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Booking is one customer's or guest's reservation at a branch
type Booking struct {
	gorm.Model
	BranchID uint   `gorm:"not null;index"`
	Branch   Branch `gorm:"foreignKey:BranchID"`
	Code     string `gorm:"type:varchar(12);not null;uniqueIndex"` // short reference shown to people
	SportID  uint   `gorm:"not null;default:0;index"`
	// read-only, for names in answers; "-:migration" keeps it from adding a foreign key
	Sport *Sport `gorm:"foreignKey:SportID;-:migration"`

	CustomerID     *uint  `gorm:"index"` // nil for guests
	Customer       *User  `gorm:"foreignKey:CustomerID"`
	GuestName      string `gorm:"type:varchar(120)"`
	GuestPhone     string `gorm:"type:varchar(30);index"`
	GuestEmail     string `gorm:"type:varchar(255)"`
	GuestTokenHash string `gorm:"type:varchar(64);index"` // private link for guests; only the hash is stored

	Status           string     `gorm:"type:varchar(12);not null;index"`
	StartsAt         time.Time  `gorm:"not null;index"`
	EndsAt           time.Time  `gorm:"not null"`
	TotalPrice       int64      `gorm:"not null"`
	Currency         string     `gorm:"type:varchar(3);not null"`
	PaymentStatus    string     `gorm:"type:varchar(12);not null;default:unpaid"`
	PaymentMethod    string     `gorm:"type:varchar(10)"`
	PaidAt           *time.Time `gorm:"index"`
	PaidByID         *uint
	PaidBy           *User     `gorm:"foreignKey:PaidByID"`
	CancellableUntil time.Time `gorm:"not null"` // rule snapshot: later rule changes do not affect this booking
	Notes            string    `gorm:"type:text"`
	CreatedByType    string    `gorm:"type:varchar(10);not null"`
	CreatedByID      *uint     // the member who made a staff booking
	CancelledAt      *time.Time
	CancelReason     string `gorm:"type:varchar(255)"`
	// restaurant tables: the group size; staff tap Arrived when it comes (0 / nil for courts and classes)
	Guests    int `gorm:"not null;default:0"`
	ArrivedAt *time.Time
	// ArriveBy: not there by then, the group loses its tables (no-show)
	ArriveBy *time.Time
	// event days (events module): the event whose tables this booking bought, the time to pay by, and
	// when the customer said they sent the payment slip (the hold then no longer runs out by itself)
	EventID    *uint `gorm:"index"`
	PayBy      *time.Time
	SlipSentAt *time.Time
	// Zones the tables stood in when booked (e.g. "Smoking"), joined by ", "; a snapshot
	Zones string `gorm:"type:varchar(120);not null;default:''"`

	Items []BookingItem `gorm:"foreignKey:BookingID"`
}

// IsActive tells whether the booking still holds its time
func (b Booking) IsActive() bool {
	return b.Status == BookingPending || b.Status == BookingConfirmed
}

// BookingItem is one thing a booking (or a class hold) occupies. Active items of the same
// resource or the same coach can never overlap: the database enforces it.
type BookingItem struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	BookingID      *uint  `gorm:"index"` // nil for class holds
	BranchID       uint   `gorm:"not null;index"`
	Kind           string `gorm:"type:varchar(15);not null"`
	ResourceID     *uint  `gorm:"index"`
	CoachID        *uint  `gorm:"index"`
	ClassSessionID *uint  `gorm:"index"`
	// read-only, for names in answers; "-:migration" keeps them from adding foreign keys
	Resource     *Resource     `gorm:"foreignKey:ResourceID;-:migration"`
	Coach        *Coach        `gorm:"foreignKey:CoachID;-:migration"`
	ClassSession *ClassSession `gorm:"foreignKey:ClassSessionID;-:migration"`
	Seats        int           `gorm:"not null;default:1"`
	StartsAt     time.Time     `gorm:"not null"`
	EndsAt       time.Time     `gorm:"not null"`
	Price        int64         `gorm:"not null;default:0"`
	Active       bool          `gorm:"not null;default:true"`
}

// EventDay is an event that takes over a day of a restaurant: normal table booking is off that day
type EventDay struct {
	ID   uint
	Name string
}

// PayFacts is what the payments module needs to know about a booking
type PayFacts struct {
	ID            uint
	BranchID      uint
	BranchSlug    string
	Code          string
	Total         int64
	Currency      string
	Status        string
	PaymentStatus string
	PayBy         *time.Time
}

// Payable: money can still be taken for it (not paid, not cancelled, something to pay)
func (f PayFacts) Payable() bool {
	return f.PaymentStatus == PaymentUnpaid && f.Total > 0 &&
		(f.Status == BookingPending || f.Status == BookingConfirmed)
}

var (
	// ErrNotPayable: the booking was cancelled, so money that arrived for it must be given back
	ErrNotPayable = errors.New("booking cannot take a payment")
	// ErrAlreadyPaid: the booking was already paid (by staff or another payment)
	ErrAlreadyPaid = errors.New("booking is already paid")
)
