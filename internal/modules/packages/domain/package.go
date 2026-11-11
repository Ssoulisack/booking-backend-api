// Package domain: what a package (course pass) is. A branch sells packages next to single bookings: a
// bundle of visits (10 or 20 times) or a pass for months, for group classes or for one space. A customer
// who bought one books with it at price 0. Nothing here knows about HTTP or the database.
package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// What a package covers
const (
	CoversClass = "group_class" // the group classes of its sport
	CoversSpace = "space"       // bookings of one space (a private class)
)

// How an option is counted
const (
	OptionTimes  = "times"  // a number of visits (10, 20), may run out by a date
	OptionMonths = "months" // unlimited visits until months after the start
)

// Package is something a sport branch sells, e.g. "Group class" or "Space A private class"
type Package struct {
	gorm.Model
	BranchID    uint   `gorm:"not null;index"`
	SportID     uint   `gorm:"not null;index"`
	Name        string `gorm:"type:varchar(120);not null"`
	Description string `gorm:"type:text"`
	Covers      string `gorm:"type:varchar(12);not null"`
	// ResourceID is the space a CoversSpace package is for; nil for group classes
	ResourceID *uint `gorm:"index"`
	// ClassIDs: the weekly classes (timetables) a group-class package covers, "3,5"; empty = every class of the sport
	ClassIDs string `gorm:"type:text;not null;default:''"`
	// CoachIDs: a space package only: the coaches it includes ("2,7"), booked with the space at no extra cost; empty =
	// no coach (a booking with a coach is paid normally)
	CoachIDs string `gorm:"type:text;not null;default:''"`
	Active   bool   `gorm:"not null"` // always set explicitly: gorm would write the default for false
}

func (Package) TableName() string { return "packages" }

// Option is one way to buy a package: "10 times 900,000", "1 month 700,000"
type Option struct {
	gorm.Model
	PackageID uint   `gorm:"not null;index"`
	Kind      string `gorm:"type:varchar(6);not null"`
	// Amount: the number of visits (times) or of months (months)
	Amount int   `gorm:"not null"`
	Price  int64 `gorm:"not null"` // whole currency units
	// ValidDays: a times option must be used within this many days of the purchase; 0 = no expiry (months: always 0)
	ValidDays int `gorm:"not null;default:0"`
	// MinutesPerVisit: how long one visit of a times option books a space (60: 1 h = 1 visit, 2 h = 2 visits; a
	// length that is not a whole number of visits is not covered, it is paid). 0 = one visit per booking, any length.
	// Group classes always take one visit; months: always 0.
	MinutesPerVisit int  `gorm:"not null;default:0"`
	Active          bool `gorm:"not null"`
}

func (Option) TableName() string { return "package_options" }

// Customer package statuses
const (
	StatusPendingPayment = "pending_payment"
	StatusActive         = "active"
	StatusUsedUp         = "used_up"
	StatusExpired        = "expired"
	StatusCancelled      = "cancelled"
)

// CustomerPackage is a package a customer bought. The name, what it covers and the price are copied from
// the package and option at the time of purchase, so later edits do not change it.
type CustomerPackage struct {
	gorm.Model
	BranchID uint `gorm:"not null;index"`
	// Seq is the package's number at its branch (1, 2, 3, ...), saved when it is bought: it makes the code P-0001.
	// Unique per branch (a partial index made by the migration); 0 only before the migration numbered old rows.
	Seq        int    `gorm:"not null;default:0"`
	CustomerID uint   `gorm:"not null;index"`
	PackageID  uint   `gorm:"not null;index"`
	OptionID   uint   `gorm:"not null"`
	SportID    uint   `gorm:"not null;index"`
	Name       string `gorm:"type:varchar(120);not null"`
	Covers     string `gorm:"type:varchar(12);not null"`
	ResourceID *uint
	// ClassIDs: copied from the package: the weekly classes it covers ("3,5"); empty = every class of the sport
	ClassIDs  string `gorm:"type:text;not null;default:''"`
	Kind      string `gorm:"type:varchar(6);not null"`
	Amount    int    `gorm:"not null"`
	ValidDays int    `gorm:"not null;default:0"` // copied from the option: a times package must be used within this many days
	// CoachIDs: the coaches the package includes, kept in step with the package (like MinutesPerVisit); "" = none
	CoachIDs string `gorm:"type:text;not null;default:''"`
	// MinutesPerVisit: always the option's current setting (see Option), kept in step when the package is saved and by
	// the migration: unlike the price or the visits, it is not fixed at purchase. 0 = one visit per booking, any length
	MinutesPerVisit int    `gorm:"not null;default:0"`
	Price           int64  `gorm:"not null"`
	Currency        string `gorm:"type:varchar(3);not null"`
	// TimesLeft: visits left (times); unused for months
	TimesLeft int `gorm:"not null;default:0"`
	// StartsOn / EndsOn: the days a months pass works, YYYY-MM-DD in the branch time zone;
	// ExpiresOn: the last day a times option can be used ("" = never). Set when the package becomes active.
	StartsOn  string `gorm:"type:varchar(10);not null;default:''"`
	EndsOn    string `gorm:"type:varchar(10);not null;default:''"`
	ExpiresOn string `gorm:"type:varchar(10);not null;default:''"`
	Status    string `gorm:"type:varchar(16);not null;index"`

	// paid like a booking: how it was paid and when
	PaymentStatus string `gorm:"type:varchar(12);not null;default:unpaid"`
	PaymentMethod string `gorm:"type:varchar(10)"`
	PaidAt        *time.Time
	// PaidByID: the team member who took the money or marked it paid (nil: paid online)
	PaidByID *uint
	// SlipSentAt: the customer said they paid outside the system and sent the slip; the team checks it
	SlipSentAt *time.Time
	// SlipVia: how the slip came: "whatsapp" (the customer sent it there) or "telegram" (uploaded here, put in the group)
	SlipVia string `gorm:"type:varchar(10);not null;default:''"`
	// who sold it: "customer" (online) or "staff" (at the desk) and the member who did
	SoldByType string `gorm:"type:varchar(10);not null"`
	SoldByID   *uint
	Notes      string `gorm:"type:text"`
	// PayBy: a package a customer bought online must be paid by this time or it cancels itself (nil: no limit, as for
	// packages the team sells). The customer's online payment moves it to the end of its QR's life.
	PayBy *time.Time
}

func (CustomerPackage) TableName() string { return "customer_packages" }

// Visit is one line of a customer package's log: a visit used (-1), given back (+1) or adjusted by staff
type Visit struct {
	ID                uint `gorm:"primaryKey"`
	CustomerPackageID uint `gorm:"not null;index"`
	BookingID         *uint
	Change            int    `gorm:"not null"`
	Note              string `gorm:"type:varchar(255)"`
	ByUserID          *uint
	CreatedAt         time.Time `gorm:"not null"`
}

func (Visit) TableName() string { return "package_visits" }

// IncludesCoach says whether the package pays this coach with its space
func (c CustomerPackage) IncludesCoach(coachID uint) bool {
	for _, id := range ClassList(c.CoachIDs) {
		if id == coachID {
			return true
		}
	}
	return false
}

// VisitsFor is how many visits a space booking of this many minutes takes: 1 when the package has no minutes per
// visit, else minutes / minutes per visit; ok is false when the length is not a whole number of visits (it is paid)
func (c CustomerPackage) VisitsFor(minutes int) (visits int, ok bool) {
	if c.MinutesPerVisit <= 0 {
		return 1, true
	}
	if minutes <= 0 || minutes%c.MinutesPerVisit != 0 {
		return 0, false
	}
	return minutes / c.MinutesPerVisit, true
}

// Code is how people refer to the package (P-0001, P-0002, ... at its branch)
func (c CustomerPackage) Code() string { return CodeOf(c.Seq) }

// CodeOf writes a package number as a code: 1 is P-0001
func CodeOf(seq int) string { return fmt.Sprintf("P-%04d", seq) }

// OrderRef is the package's order number base for the bank. The code repeats from branch to branch, the order
// number must not, so the branch is part of it (P3-0001).
func (c CustomerPackage) OrderRef() string { return fmt.Sprintf("P%d-%04d", c.BranchID, c.Seq) }

// StatusOn is the status as it stands on a day (YYYY-MM-DD, branch time zone): an active package whose
// time ran out reads as expired, one with no visits left as used up. Nothing is written for that.
func (c CustomerPackage) StatusOn(today string) string {
	if c.Status != StatusActive {
		return c.Status
	}
	if c.Kind == OptionTimes && c.TimesLeft <= 0 {
		return StatusUsedUp
	}
	end := c.ExpiresOn
	if c.Kind == OptionMonths {
		end = c.EndsOn
	}
	if end != "" && today > end {
		return StatusExpired
	}
	return StatusActive
}

// ClassList reads a ClassIDs text ("3,5") as numbers; empty text = none listed (every class)
func ClassList(text string) []uint {
	var out []uint
	for _, part := range strings.Split(text, ",") {
		if n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64); err == nil && n > 0 {
			out = append(out, uint(n))
		}
	}
	return out
}

// ClassText writes class ids as a ClassIDs text, sorted and without repeats
func ClassText(ids []uint) string {
	seen := map[uint]bool{}
	var list []uint
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			list = append(list, id)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	parts := make([]string, len(list))
	for i, id := range list {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}

// CoversClassOf says whether a class-covering purchase pays a class: no list = every class of the sport; a list =
// only those weekly classes (a one-off class that is not part of a timetable is covered only when no list is set)
func CoversClassOf(classIDs string, timetableID *uint) bool {
	list := ClassList(classIDs)
	if len(list) == 0 {
		return true
	}
	if timetableID == nil {
		return false
	}
	for _, id := range list {
		if id == *timetableID {
			return true
		}
	}
	return false
}
