package dto

import (
	coredto "backend/internal/core/dto"
	"time"
)

// OptionRequest is one way to buy a package. ID 0 adds a new option; an option the request leaves out is removed.
type OptionRequest struct {
	ID        uint   `json:"id"`
	Kind      string `json:"kind"`       // times | months
	Amount    int    `json:"amount"`     // visits, or months
	Price     int64  `json:"price"`      // whole currency units
	ValidDays int    `json:"valid_days"` // times only; 0 = no expiry
	Active    *bool  `json:"active"`     // not sent: yes
}

// PackageRequest creates or changes a package with its options
type PackageRequest struct {
	SportID     uint            `json:"sport_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Covers      string          `json:"covers"`      // group_class | space
	ResourceID  *uint           `json:"resource_id"` // the space, when covers = space
	ClassIDs    []uint          `json:"class_ids"`   // the weekly classes (timetables) a group-class package covers; none = every class
	Active      *bool           `json:"active"`      // not sent: yes on a new package, unchanged otherwise
	Options     []OptionRequest `json:"options"`
}

// OptionDTO is an option in answers
type OptionDTO struct {
	ID        uint   `json:"id"`
	Kind      string `json:"kind"`
	Amount    int    `json:"amount"`
	Price     int64  `json:"price"`
	ValidDays int    `json:"valid_days"`
	Active    bool   `json:"active"`
}

// PackageDTO is a package in answers
type PackageDTO struct {
	ID           uint        `json:"id"`
	SportID      uint        `json:"sport_id"`
	SportName    string      `json:"sport_name"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Covers       string      `json:"covers"`
	ResourceID   *uint       `json:"resource_id"`
	ResourceName string      `json:"resource_name"`
	ClassIDs     []uint      `json:"class_ids"`   // empty = every class of the sport
	ClassNames   []string    `json:"class_names"` // the same classes by name
	Active       bool        `json:"active"`
	Currency     string      `json:"currency"`
	Options      []OptionDTO `json:"options"`
}

// BuyRequest: a customer buys one option of a package
type BuyRequest struct {
	OptionID uint `json:"option_id"`
}

// SellRequest: staff sell a package to a customer at the desk. With a payment method the package is paid and
// active at once; without one it waits for payment.
type SellRequest struct {
	CustomerID    uint   `json:"customer_id"`
	PackageID     uint   `json:"package_id"`
	OptionID      uint   `json:"option_id"`
	PaymentMethod string `json:"payment_method"` // cash | transfer | qr | other, or empty
	Notes         string `json:"notes"`
}

// MarkPaidRequest: staff record that a waiting package was paid
type MarkPaidRequest struct {
	Method string `json:"method"` // cash | transfer | qr | other
}

// CustomerPackageDTO is a package a customer owns
type CustomerPackageDTO struct {
	ID            uint       `json:"id"`
	Code          string     `json:"code"` // "P-12": shown to people, the order number of an online payment
	BranchID      uint       `json:"branch_id"`
	BranchName    string     `json:"branch_name,omitempty"`
	BranchSlug    string     `json:"branch_slug,omitempty"`
	CustomerID    uint       `json:"customer_id"`
	CustomerName  string     `json:"customer_name,omitempty"`
	CustomerPhone string     `json:"customer_phone,omitempty"`
	PackageID     uint       `json:"package_id"`
	SportID       uint       `json:"sport_id"`
	Name          string     `json:"name"`
	Covers        string     `json:"covers"`
	ResourceID    *uint      `json:"resource_id"`
	ResourceName  string     `json:"resource_name"`
	ClassIDs      []uint     `json:"class_ids"`
	ClassNames    []string   `json:"class_names"`
	Kind          string     `json:"kind"`
	Amount        int        `json:"amount"`
	Price         int64      `json:"price"`
	Currency      string     `json:"currency"`
	TimesLeft     int        `json:"times_left"`
	StartsOn      string     `json:"starts_on"`
	EndsOn        string     `json:"ends_on"`
	ExpiresOn     string     `json:"expires_on"`
	Status        string     `json:"status"` // pending_payment | active | used_up | expired | cancelled
	PaymentStatus string     `json:"payment_status"`
	PaymentMethod string     `json:"payment_method"`
	PaidAt        *time.Time `json:"paid_at"`
	SlipSentAt    *time.Time `json:"slip_sent_at"`
	SlipVia       string     `json:"slip_via"`
	PayBy         *time.Time `json:"pay_by,omitempty"` // until when an unpaid online purchase is kept
	SoldBy        string     `json:"sold_by"`          // customer | staff
	Notes         string     `json:"notes"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Slip ways and the upload are shared with event bookings (core dto)
const (
	SlipWhatsapp = coredto.SlipWhatsapp
	SlipTelegram = coredto.SlipTelegram
)

// SlipUpload: "I paid, here is the slip"
type SlipUpload = coredto.SlipUpload

// AdjustRequest: staff add or remove visits of a times package, and/or extend its last day, with a reason
type AdjustRequest struct {
	Visits     int    `json:"visits"`      // +3 adds three, -1 removes one; times packages only
	ExtendDays int    `json:"extend_days"` // days added to the last day (a months pass, or a times package that has one)
	Note       string `json:"note"`        // required: why
}

// VisitDTO is one line of a package's log
type VisitDTO struct {
	ID        uint      `json:"id"`
	Change    int       `json:"change"` // -1 a visit used, +1 given back or added, 0 a month pass visit or an extension
	BookingID *uint     `json:"booking_id"`
	Note      string    `json:"note"`
	ByName    string    `json:"by_name,omitempty"` // the team member who adjusted it
	CreatedAt time.Time `json:"created_at"`
}
