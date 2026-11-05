// Package domain: what a payment is. A payment is the money owed for one booking; every try to pay it
// through a gateway (each bank the customer picks) is a charge of that payment. The gateway-facing words
// (charge request, notice, state) live here too, so every gateway adapter speaks the same language.
package domain

import "time"

// Payment statuses
const (
	StatusPending         = "pending"          // waiting for the customer: a charge may be open
	StatusPaid            = "paid"             // money received and the booking settled
	StatusExpired         = "expired"          // every charge ran out of time
	StatusFailed          = "failed"           // the gateway refused to create a charge
	StatusMismatch        = "mismatch"         // the gateway reports a different amount: NOT settled, the team looks at it
	StatusRefundDue       = "refund_due"       // money arrived for a booking that cannot take it (cancelled, or paid already)
	StatusDisputed        = "disputed"         // the gateway reports a dispute
	StatusRefundRequested = "refund_requested" // the team asked the gateway for the money back
	StatusRefunded        = "refunded"
)

// Charge statuses
const (
	ChargePending = "pending" // created, the customer may still pay it
	ChargePaid    = "paid"    // the money of this charge arrived
	ChargeExpired = "expired" // the gateway says the time ran out
	ChargeFailed  = "failed"  // the gateway refused to create it
)

// Payment is what a booking owes. A customer who changes bank adds a charge to the SAME payment; only one
// charge ever settles it (money from a second charge becomes a payment of its own, to give back).
type Payment struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	BranchID  uint   `gorm:"not null;index"`
	BookingID uint   `gorm:"not null;index"`
	OrderNo   string `gorm:"type:varchar(40);not null;uniqueIndex"` // ours: "<booking code>-<n>"

	Amount   int64  `gorm:"not null"`
	Currency string `gorm:"type:varchar(3);not null"`
	Status   string `gorm:"type:varchar(20);not null;index"`

	// ChargeID is the charge in use: the one the customer is paying now, then the one that was paid
	ChargeID  *uint
	PaidAt    *time.Time
	RefundRef string `gorm:"type:varchar(80)"`
	Note      string `gorm:"type:varchar(255)"`

	Charges []Charge `gorm:"foreignKey:PaymentID"`
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

// Current is the charge the payment is about: the one in use, else the newest that did not fail, else the
// newest. The zero Charge when there is none.
func (p Payment) Current() Charge {
	var newest, newestOK Charge
	for _, c := range p.Charges {
		if p.ChargeID != nil && c.ID == *p.ChargeID {
			return c
		}
		if c.ID > newest.ID {
			newest = c
		}
		if c.Status != ChargeFailed && c.ID > newestOK.ID {
			newestOK = c
		}
	}
	if newestOK.ID != 0 {
		return newestOK
	}
	return newest
}

// Live is the newest charge of the given way to pay that can still be paid, or nil
func (p Payment) Live(provider, method string, now time.Time) *Charge {
	var found *Charge
	for i := range p.Charges {
		c := &p.Charges[i]
		if c.Status == ChargePending && c.Provider == provider && c.Method == method && c.ExpiresAt.After(now) && (found == nil || c.ID > found.ID) {
			found = c
		}
	}
	return found
}

// AnyLive: some charge besides `except` can still be paid
func (p Payment) AnyLive(except uint, now time.Time) bool {
	for _, c := range p.Charges {
		if c.ID != except && c.Status == ChargePending && c.ExpiresAt.After(now) {
			return true
		}
	}
	return false
}

// Charge is one transaction at a gateway: a QR or a payment page for one way to pay (one bank)
type Charge struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	PaymentID uint   `gorm:"not null;index"`
	Provider  string `gorm:"type:varchar(20);not null;index:idx_charge_ref,priority:1"`
	Method    string `gorm:"type:varchar(20);not null"`
	// ProviderRef is the gateway's id; GatewayOrderNo is the order number we sent it (unique per charge)
	ProviderRef    string `gorm:"type:varchar(80);index:idx_charge_ref,priority:2"`
	GatewayOrderNo string `gorm:"type:varchar(50);not null;uniqueIndex"`

	QRCode      string `gorm:"type:text"`
	Deeplink    string `gorm:"type:text"`
	RedirectURL string `gorm:"type:text"`
	ExpiresAt   time.Time
	Status      string `gorm:"type:varchar(20);not null;index"`
}

func (Charge) TableName() string { return "payment_charges" }
