// Package domain: what an event day is. An event takes a day (or a few) over at a restaurant: its
// tables are sold, each at the price of its zone or its own, and an unpaid booking holds them for a
// while. Nothing here knows about HTTP or the database.
package domain

import (
	"time"

	"gorm.io/gorm"
)

// Event is one special day (or a few days) at a restaurant branch
type Event struct {
	gorm.Model
	BranchID    uint   `gorm:"not null;index"`
	Name        string `gorm:"type:varchar(120);not null"`
	Description string `gorm:"type:text"`
	// the days it runs, YYYY-MM-DD in the branch's time zone, both included (one day: the same date)
	StartsOn string `gorm:"type:varchar(10);not null;index"`
	EndsOn   string `gorm:"type:varchar(10);not null"`
	// pictures from the media library: the day's page background, and the small ad (falls back to the background)
	BackgroundKey string `gorm:"type:varchar(255)"`
	AdKey         string `gorm:"type:varchar(255)"`
	// an unpaid booking holds its tables this long; a booking of MORE than BigBookingTables tables
	// must be paid sooner (BigHoldMinutes)
	HoldMinutes      int `gorm:"not null;default:1440"`
	BigBookingTables int `gorm:"not null;default:4"`
	BigHoldMinutes   int `gorm:"not null;default:60"`
	// price of a table that is in no priced zone and has no price of its own; 0 = not for sale
	DefaultPrice int64 `gorm:"not null;default:0"`
	Published    bool  `gorm:"not null;default:false"`
}

// Price kinds
const (
	PriceZone  = "zone"  // RefID is a floor item (zone) id: every table inside pays it
	PriceTable = "table" // RefID is a table id: this table's own price
)

// Price is the price of a zone or of one table at one event (0: those tables are not for sale)
type Price struct {
	ID      uint   `gorm:"primaryKey"`
	EventID uint   `gorm:"not null;uniqueIndex:idx_event_price,priority:1"`
	Kind    string `gorm:"type:varchar(5);not null;uniqueIndex:idx_event_price,priority:2"`
	RefID   uint   `gorm:"not null;uniqueIndex:idx_event_price,priority:3"`
	Amount  int64  `gorm:"not null"`
}

func (Price) TableName() string { return "event_prices" }

// PayInfo is how customers pay a branch for event tables: its WhatsApp number (they send the slip
// there), the QR picture and the account text
type PayInfo struct {
	BranchID uint   `gorm:"primaryKey"`
	Whatsapp string `gorm:"type:varchar(30)"`
	QRKey    string `gorm:"column:qr_key;type:varchar(255)"`
	Note     string `gorm:"type:text"`
	// the account customers transfer to: who it belongs to and its number (each has a copy button)
	AccountName   string `gorm:"type:varchar(120)"`
	AccountNumber string `gorm:"type:varchar(60)"`
	// TransferOff turns the branch's own way to pay (QR, transfer, WhatsApp) off; false = on, so rows
	// made before this column existed stay on
	TransferOff bool
	UpdatedAt   time.Time
}

func (PayInfo) TableName() string { return "event_pay_info" }

// Covers reports whether the event runs on day (YYYY-MM-DD)
func (e Event) Covers(day string) bool { return day >= e.StartsOn && day <= e.EndsOn }

// Days lists the dates the event runs, first to last
func (e Event) Days() []string {
	from, err1 := time.Parse("2006-01-02", e.StartsOn)
	to, err2 := time.Parse("2006-01-02", e.EndsOn)
	if err1 != nil || err2 != nil {
		return nil
	}
	var out []string
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

// HoldFor is how long a booking of n tables holds them while unpaid
func (e Event) HoldFor(n int) time.Duration {
	if n > e.BigBookingTables {
		return time.Duration(e.BigHoldMinutes) * time.Minute
	}
	return time.Duration(e.HoldMinutes) * time.Minute
}
