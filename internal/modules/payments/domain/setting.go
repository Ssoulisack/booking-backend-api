package domain

import "time"

// Setting is a branch's choice about online payment. OnlineOff turns it off for the whole branch (customers
// then only see the branch's own way to pay); false = on, so a branch that never chose stays on.
type Setting struct {
	BranchID  uint `gorm:"primaryKey"`
	OnlineOff bool
	// ChargeMinutes is how long a customer has to pay a QR or page we made (0: the default, 30)
	ChargeMinutes int
	UpdatedAt     time.Time
}

func (Setting) TableName() string { return "payment_settings" }
