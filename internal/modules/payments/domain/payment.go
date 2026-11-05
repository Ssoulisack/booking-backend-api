// Package domain: what a payment is. A payment is one try to pay a booking through a gateway: the QR or payment
// link made for one bank, its time, and how it ended. A booking has one open payment at a time (the way to pay
// is final once a QR exists); a try that ran out or failed is followed by a new one. The gateway-facing words
// (charge request, notice, state) live here too, so every gateway adapter speaks the same language.
package domain

import "time"

// Payment statuses
const (
	StatusPending         = "pending"          // a QR / payment link was made, nothing paid yet
	StatusPaid            = "paid"             // money received and the booking settled
	StatusExpired         = "expired"          // the gateway says the time ran out
	StatusFailed          = "failed"           // the gateway refused to create it
	StatusMismatch        = "mismatch"         // the gateway reports a different amount: NOT settled, the team looks at it
	StatusRefundDue       = "refund_due"       // money arrived for a booking that cannot take it (cancelled, or paid already)
	StatusDisputed        = "disputed"         // the gateway reports a dispute
	StatusRefundRequested = "refund_requested" // the team asked the gateway for the money back
	StatusRefunded        = "refunded"
)

// Payment is one try to pay a booking online: what to scan or open, and where it stands
type Payment struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	BranchID  uint   `gorm:"not null;index"`
	BookingID uint   `gorm:"not null;index"`
	Provider  string `gorm:"type:varchar(20);not null;index:idx_payment_ref,priority:1"`
	Method    string `gorm:"type:varchar(20);not null"`
	// OrderNo is ours and unique ("<booking code>-<try>"): it is also the order number sent to the gateway.
	// ProviderRef is the gateway's id ("" for a payment page until the gateway tells us).
	OrderNo     string `gorm:"type:varchar(40);not null;uniqueIndex"`
	ProviderRef string `gorm:"type:varchar(80);index:idx_payment_ref,priority:2"`

	Amount   int64  `gorm:"not null"`
	Currency string `gorm:"type:varchar(3);not null"`
	Status   string `gorm:"type:varchar(20);not null;index"`

	QRCode      string `gorm:"type:text"`
	Deeplink    string `gorm:"type:text"`
	RedirectURL string `gorm:"type:text"`
	ExpiresAt   time.Time
	PaidAt      *time.Time
	RefundRef   string `gorm:"type:varchar(80)"`
	Note        string `gorm:"type:varchar(255)"`
}

func (Payment) TableName() string { return "payments" }

// Settled reports whether the payment can no longer change by itself
func (p Payment) Settled() bool {
	switch p.Status {
	case StatusPaid, StatusRefundDue, StatusRefundRequested, StatusRefunded:
		return true
	}
	return false
}

// Open: the QR or link can still be paid (waiting, and our time for it is not up)
func (p Payment) Open(now time.Time) bool {
	return p.Status == StatusPending && p.ExpiresAt.After(now)
}
