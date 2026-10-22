package domain

import (
	"time"

	"gorm.io/gorm"
)

// Owner types for opening hours, closures and prices
const (
	OwnerResource = "resource"
	OwnerCoach    = "coach"
	OwnerVenue    = "venue" // closures only: closes the whole branch
)

// OpeningHour is one open interval of a resource or coach on a weekday.
// Weekday follows time.Weekday (0 = Sunday). Times are "HH:MM" in the branch timezone.
type OpeningHour struct {
	gorm.Model
	BranchID  uint   `gorm:"not null;index"`
	OwnerType string `gorm:"type:varchar(20);not null;index:idx_hours_owner,priority:1"`
	OwnerID   uint   `gorm:"not null;index:idx_hours_owner,priority:2"`
	Weekday   int    `gorm:"not null"`
	OpenTime  string `gorm:"type:varchar(5);not null"`
	CloseTime string `gorm:"type:varchar(5);not null"`
}

// Closure blocks a resource, a coach or the whole branch for a period (holiday, maintenance)
type Closure struct {
	gorm.Model
	BranchID  uint      `gorm:"not null;index"`
	OwnerType string    `gorm:"type:varchar(20);not null;index:idx_closures_owner,priority:1"`
	OwnerID   uint      `gorm:"not null;default:0;index:idx_closures_owner,priority:2"` // 0 for the venue
	StartsAt  time.Time `gorm:"not null"`
	EndsAt    time.Time `gorm:"not null"`
	Reason    string    `gorm:"type:varchar(255)"`
}

// Price is the flat hourly price of a resource or coach, in whole currency units
type Price struct {
	gorm.Model
	BranchID     uint   `gorm:"not null;index"`
	OwnerType    string `gorm:"type:varchar(20);not null;uniqueIndex:idx_prices_owner,priority:1,where:deleted_at IS NULL"`
	OwnerID      uint   `gorm:"not null;uniqueIndex:idx_prices_owner,priority:2,where:deleted_at IS NULL"`
	PricePerHour int64  `gorm:"not null"`
	Currency     string `gorm:"type:varchar(3);not null"`
}

// ClassSession is a one-off group session with a capacity
type ClassSession struct {
	gorm.Model
	BranchID   uint      `gorm:"not null;index"`
	SportID    uint      `gorm:"not null;index"`
	CoachID    *uint     `gorm:"index"`
	ResourceID *uint     `gorm:"index"`
	Title      string    `gorm:"type:varchar(150);not null"`
	StartsAt   time.Time `gorm:"not null;index"`
	EndsAt     time.Time `gorm:"not null"`
	Capacity   int       `gorm:"not null"`
	Price      int64     `gorm:"not null;default:0"` // per person, whole currency units
	Currency   string    `gorm:"type:varchar(3);not null"`
	Active     bool      `gorm:"not null"` // always set explicitly: gorm would write the default for false
}
