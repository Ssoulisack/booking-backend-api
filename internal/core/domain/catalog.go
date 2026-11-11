package domain

import (
	"gorm.io/gorm"
	"sort"
	"strconv"
	"strings"
)

// Resource kinds: what a bookable space is, shown to customers ("Choose a ring"). The team calls
// them all "spaces".
const (
	ResourceKindCourt = "court" // tennis, badminton, basketball…
	ResourceKindPitch = "pitch" // football, futsal
	ResourceKindRing  = "ring"  // boxing, Muay Thai
	ResourceKindLane  = "lane"  // swimming, bowling
	ResourceKindBay   = "bay"   // golf range
	ResourceKindTable = "table" // table tennis, billiards
	ResourceKindRoom  = "room"  // yoga, dance, studio
	ResourceKindOther = "other"
)

// Sport is something clients can book (tennis, boxing, soccer, ...)
type Sport struct {
	gorm.Model
	BranchID    uint   `gorm:"not null;index;uniqueIndex:idx_sports_slug,priority:1,where:deleted_at IS NULL"`
	Name        string `gorm:"type:varchar(120);not null"`
	Slug        string `gorm:"type:varchar(140);not null;uniqueIndex:idx_sports_slug,priority:2,where:deleted_at IS NULL"`
	Description string `gorm:"type:text"`
	ImageKey    string `gorm:"type:varchar(255)"` // object key in file storage, never a full URL
	Active      bool   `gorm:"not null"`          // always set explicitly: gorm would write the default for false
	// Type is one of SportTypes: what customers filter places by
	Type string `gorm:"type:varchar(30);not null;default:other;index"`
	// In a restaurant a "sport" is an area (Indoor, Terrace, VIP room) with a floor plan of
	// PlanWidth × PlanHeight grid squares; 0 at sport branches
	PlanWidth  int `gorm:"not null;default:0"`
	PlanHeight int `gorm:"not null;default:0"`
	// Floor the area is on: 0 = ground floor, 1 = first floor, -1 = basement (restaurants only)
	Floor int `gorm:"not null;default:0"`
}

// Resource is a bookable place: a tennis court, a soccer pitch, a boxing ring
type Resource struct {
	gorm.Model
	BranchID uint   `gorm:"not null;index"`
	SportID  uint   `gorm:"not null;index"`
	Name     string `gorm:"type:varchar(120);not null"`
	Kind     string `gorm:"type:varchar(20);not null"`
	Active   bool   `gorm:"not null"` // always set explicitly: gorm would write the default for false

	// restaurant tables only (0 / empty at sport branches)
	Seats       int    `gorm:"not null;default:0"`
	ExtraChairs int    `gorm:"not null;default:0"` // used only when no real-seat choice is left
	Shape       string `gorm:"type:varchar(10);not null;default:''"`
	// place on the area's floor plan, in grid squares; Rotation 0 / 90 / 180 / 270
	PlanX    int `gorm:"not null;default:0"`
	PlanY    int `gorm:"not null;default:0"`
	PlanW    int `gorm:"not null;default:0"`
	PlanH    int `gorm:"not null;default:0"`
	Rotation int `gorm:"not null;default:0"`
	// WalkInOnly: kept for walk-ins. Customers cannot book it online (it shows as taken) and events do not sell it; staff
	// seat walk-ins and move groups there as usual. Stays until the team switches it off.
	WalkInOnly bool `gorm:"not null;default:false"`
	// CoachIDs: sport spaces only: the coaches included free with this space ("2,7"): booked together, the coach costs
	// nothing (an internal class); other coaches are paid at their hourly price. "" = none
	CoachIDs string `gorm:"type:text;not null;default:''"`
}

// IncludesCoach says whether the coach comes free with this space
func (r Resource) IncludesCoach(coachID uint) bool {
	for _, id := range IDList(r.CoachIDs) {
		if id == coachID {
			return true
		}
	}
	return false
}

// IDList reads a list of ids kept as text ("2,7"); IDText writes one, sorted and without repeats
func IDList(text string) []uint {
	var out []uint
	for _, part := range strings.Split(text, ",") {
		if n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64); err == nil && n > 0 {
			out = append(out, uint(n))
		}
	}
	return out
}

func IDText(ids []uint) string {
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

// Coach is a trainer managed by the branch; coaches do not log in
type Coach struct {
	gorm.Model
	BranchID uint    `gorm:"not null;index"`
	Name     string  `gorm:"type:varchar(120);not null"`
	Bio      string  `gorm:"type:text"`
	Phone    string  `gorm:"type:varchar(20)"`
	ImageKey string  `gorm:"type:varchar(255)"`
	Active   bool    `gorm:"not null"` // always set explicitly: gorm would write the default for false
	Sports   []Sport `gorm:"many2many:coach_sports"`
}
