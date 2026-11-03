package domain

import "gorm.io/gorm"

// Restaurant tables: shapes, floor-plan sizes and the owner's table settings.

// Table shapes on the floor plan
const (
	TableShapeRound  = "round"
	TableShapeSquare = "square"
	TableShapeLong   = "long"
)

// ValidTableShape reports whether s is a known table shape
func ValidTableShape(s string) bool {
	return s == TableShapeRound || s == TableShapeSquare || s == TableShapeLong
}

// Floor plan limits, in grid squares
const (
	DefaultPlanWidth  = 20
	DefaultPlanHeight = 12
	MinPlanSize       = 4
	MaxPlanSize       = 60
	MaxTableSize      = 12 // one side of a table
	MaxTableSeats     = 50
	MaxExtraChairs    = 20
)

// Table modes: how long a booking holds its table for online booking
const (
	// TableModeOneAtATime holds the table until the bill is closed (or closing time)
	TableModeOneAtATime = "one_at_a_time"
	// TableModeHold holds it HoldMinutes after the booking time, so a table serves several bookings a day
	TableModeHold = "hold"
)

// Footprint is the table's rectangle on the plan, turned by its rotation
func (r Resource) Footprint() (x, y, w, h int) {
	w, h = r.PlanW, r.PlanH
	if r.Rotation == 90 || r.Rotation == 270 {
		w, h = h, w
	}
	return r.PlanX, r.PlanY, w, h
}

// Overlaps reports whether two tables cover the same grid square
func (r Resource) Overlaps(o Resource) bool {
	ax, ay, aw, ah := r.Footprint()
	bx, by, bw, bh := o.Footprint()
	return ax < bx+bw && bx < ax+aw && ay < by+bh && by < ay+ah
}

// NextTo reports whether two tables are at most gap grid squares apart (and do not overlap):
// they can be joined for one group without moving other tables
func (r Resource) NextTo(o Resource, gap int) bool {
	ax, ay, aw, ah := r.Footprint()
	bx, by, bw, bh := o.Footprint()
	dx := max(bx-(ax+aw), ax-(bx+bw), 0)
	dy := max(by-(ay+ah), ay-(by+bh), 0)
	return dx <= gap && dy <= gap && !r.Overlaps(o)
}

// FitsIn reports whether the table lies inside a plan of width × height
func (r Resource) FitsIn(width, height int) bool {
	x, y, w, h := r.Footprint()
	return x >= 0 && y >= 0 && w >= 1 && h >= 1 && x+w <= width && y+h <= height
}

// FloorItem is something drawn on an area's floor plan that is not booked: a bar counter, a
// pole, the stage, the toilets, the entrance, a text label, or a coloured zone (Smoking, VIP).
// Customers see them too, to find their way around the plan.
type FloorItem struct {
	gorm.Model
	BranchID uint   `gorm:"not null;index"`
	AreaID   uint   `gorm:"not null;index"` // the area (a restaurant's "sport") it is drawn on
	Kind     string `gorm:"type:varchar(10);not null"`
	Label    string `gorm:"type:varchar(40)"`
	Color    string `gorm:"type:varchar(10)"` // zones: one of ZoneColors
	PlanX    int    `gorm:"not null"`
	PlanY    int    `gorm:"not null"`
	PlanW    int    `gorm:"not null"`
	PlanH    int    `gorm:"not null"`
	Rotation int    `gorm:"not null;default:0"`
	// stairs only: the floor they lead to (nil: not set)
	TargetFloor *int
}

// Floors an area can be on
const (
	MinFloor = -3
	MaxFloor = 50
)

// Floor item kinds
const (
	FloorItemBar    = "bar"
	FloorItemPole   = "pole"
	FloorItemStage  = "stage"
	FloorItemToilet = "toilet"
	FloorItemEntry  = "entry"
	FloorItemLabel  = "label"
	FloorItemZone   = "zone"
	FloorItemStairs = "stairs"
)

// floorItemBlocks: the kinds that take floor space, so no table can stand on them
var floorItemBlocks = map[string]bool{
	FloorItemBar: true, FloorItemPole: true, FloorItemStage: true, FloorItemToilet: true,
	FloorItemStairs: true, FloorItemEntry: false, FloorItemLabel: false, FloorItemZone: false,
}

// ValidFloorItemKind reports whether k is a known floor item kind
func ValidFloorItemKind(k string) bool {
	_, ok := floorItemBlocks[k]
	return ok
}

// ZoneColors are the colours a zone can have (the website draws them)
var ZoneColors = []string{"green", "blue", "amber", "rose", "violet", "slate"}

// Blocks reports whether tables may not stand on the item (zones, labels and the entrance do not block)
func (f FloorItem) Blocks() bool { return floorItemBlocks[f.Kind] }

// Footprint is the item's rectangle on the plan, turned by its rotation
func (f FloorItem) Footprint() (x, y, w, h int) {
	w, h = f.PlanW, f.PlanH
	if f.Rotation == 90 || f.Rotation == 270 {
		w, h = h, w
	}
	return f.PlanX, f.PlanY, w, h
}

// FitsIn reports whether the item lies inside a plan of width × height
func (f FloorItem) FitsIn(width, height int) bool {
	x, y, w, h := f.Footprint()
	return x >= 0 && y >= 0 && w >= 1 && h >= 1 && x+w <= width && y+h <= height
}

// Covers reports whether a blocking item stands on any square of the table
func (f FloorItem) Covers(t Resource) bool {
	if !f.Blocks() {
		return false
	}
	ax, ay, aw, ah := f.Footprint()
	bx, by, bw, bh := t.Footprint()
	return ax < bx+bw && bx < ax+aw && ay < by+bh && by < ay+ah
}
