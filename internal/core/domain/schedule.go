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
	BranchID uint `gorm:"not null;index"`
	// TimetableID is the weekly timetable that made this class; nil for a class added on its own
	TimetableID *uint     `gorm:"index"`
	SportID     uint      `gorm:"not null;index"`
	CoachID     *uint     `gorm:"index"`
	ResourceID  *uint     `gorm:"index"`
	Title       string    `gorm:"type:varchar(150);not null"`
	StartsAt    time.Time `gorm:"not null;index"`
	EndsAt      time.Time `gorm:"not null"`
	Capacity    int       `gorm:"not null"`
	Price       int64     `gorm:"not null;default:0"` // per person, whole currency units
	Currency    string    `gorm:"type:varchar(3);not null"`
	Active      bool      `gorm:"not null"` // always set explicitly: gorm would write the default for false
}

// ClassTimetable is a class that repeats every week with no need to set it up again: the
// classes of the next weeks are made from it automatically (see ClassWindowDays), skipping closed
// days, until it is stopped or reaches EndsOn. Dates and times are in the branch time zone.
type ClassTimetable struct {
	gorm.Model
	BranchID   uint   `gorm:"not null;index"`
	SportID    uint   `gorm:"not null;index"`
	CoachID    *uint  `gorm:"index"`
	ResourceID *uint  `gorm:"index"`
	Title      string `gorm:"type:varchar(150);not null"`
	StartTime  string `gorm:"type:varchar(5);not null"`             // HH:MM
	EndTime    string `gorm:"type:varchar(5);not null"`             // HH:MM, "24:00" allowed
	Weekdays   string `gorm:"type:varchar(20);not null"`            // "1,2,3,4,5,6" (0 = Sunday)
	StartsOn   string `gorm:"type:varchar(10);not null"`            // YYYY-MM-DD, the first day
	EndsOn     string `gorm:"type:varchar(10);not null;default:''"` // YYYY-MM-DD, last day; "" = no end
	Capacity   int    `gorm:"not null"`
	Price      int64  `gorm:"not null;default:0"`
	Currency   string `gorm:"type:varchar(3);not null"`
	Active     bool   `gorm:"not null"` // the classes it makes are shown on the booking page
	// FilledUntil is the last day classes were made for ("" = none yet)
	FilledUntil string `gorm:"type:varchar(10);not null;default:''"`
}

// ClassWindowDays is how far ahead timetable classes are kept made: the booking page's range
const ClassWindowDays = 60
