package domain

import (
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
