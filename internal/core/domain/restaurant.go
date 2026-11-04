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

// Booking styles of a restaurant branch
const (
	BookingStyleTime     = "time"      // customers pick a start time
	BookingStyleArriveBy = "arrive_by" // a bar: customers book the evening and must arrive by a set time
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

// Holds reports whether a zone holds the table: the table's centre lies inside the zone
func (f FloorItem) Holds(t Resource) bool {
	if f.Kind != FloorItemZone {
		return false
	}
	zx, zy, zw, zh := f.Footprint()
	tx, ty, tw, th := t.Footprint()
	// doubled to stay in whole numbers: centre = x + w/2
	cx, cy := 2*tx+tw, 2*ty+th
	return cx > 2*zx && cx < 2*(zx+zw) && cy > 2*zy && cy < 2*(zy+zh)
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

// ---------- choosing tables ----------

// JoinGap is how far apart (in grid squares) two tables may stand and still be joined for one group
const JoinGap = 1

// TableChoice is how a customer's tables seat their group
type TableChoice struct {
	Seats      int  // real seats of the chosen tables
	ExtraUsed  bool // the group only fits with extra chairs (allowed when nothing else can seat it)
	ExtraTotal int  // seats + extra chairs
}

// sumSeats adds up the real seats, or the seats with extra chairs
func sumSeats(tables []Resource, withExtra bool) int {
	n := 0
	for _, t := range tables {
		n += t.Seats
		if withExtra {
			n += t.ExtraChairs
		}
	}
	return n
}

// connected reports whether the tables form one group, each next to another, in one area
func connected(tables []Resource) bool {
	if len(tables) == 0 {
		return false
	}
	seen := map[int]bool{0: true}
	queue := []int{0}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		for j := range tables {
			if !seen[j] && tables[j].SportID == tables[i].SportID && tables[i].NextTo(tables[j], JoinGap) {
				seen[j] = true
				queue = append(queue, j)
			}
		}
	}
	return len(seen) == len(tables)
}

// groups splits tables into the groups that stand together (same area, each next to another)
func groups(tables []Resource) [][]Resource {
	var out [][]Resource
	used := make([]bool, len(tables))
	for i := range tables {
		if used[i] {
			continue
		}
		used[i] = true
		group := []Resource{tables[i]}
		for k := 0; k < len(group); k++ {
			for j := range tables {
				if !used[j] && tables[j].SportID == group[k].SportID && group[k].NextTo(tables[j], JoinGap) {
					used[j] = true
					group = append(group, tables[j])
				}
			}
		}
		out = append(out, group)
	}
	return out
}

// RealSeatsChoice reports whether the free tables can seat the group on real seats (one table,
// or tables next to each other); while they can, extra chairs are not offered
func RealSeatsChoice(free []Resource, guests int) bool {
	for _, g := range groups(free) {
		if sumSeats(g, false) >= guests {
			return true
		}
	}
	return false
}

// AnyChoice reports whether the group fits anywhere, with extra chairs if need be
func AnyChoice(free []Resource, guests int) bool {
	if RealSeatsChoice(free, guests) {
		return true
	}
	for _, g := range groups(free) {
		if sumSeats(g, true) >= guests {
			return true
		}
	}
	return false
}

// CheckTables applies the rules to a customer's tables for a group, given the free tables of the
// branch at that time (the chosen ones included). It returns the reason as an error code when
// the choice is not allowed:
//   - TABLES_NOT_TOGETHER: not one group of tables next to each other in one area
//   - TABLES_TOO_SMALL: not enough seats (extra chairs count only when nothing else is left)
//   - TABLE_IS_EXTRA: one table could be left out and the rest would still be enough
func CheckTables(chosen []Resource, guests int, free []Resource) (*TableChoice, string) {
	if len(chosen) == 0 {
		return nil, "TABLE_REQUIRED"
	}
	if !connected(chosen) {
		return nil, "TABLES_NOT_TOGETHER"
	}
	seats := sumSeats(chosen, false)
	if seats >= guests {
		for _, t := range chosen {
			if seats-t.Seats >= guests {
				return nil, "TABLE_IS_EXTRA"
			}
		}
		return &TableChoice{Seats: seats, ExtraTotal: sumSeats(chosen, true)}, ""
	}
	// short of real seats: extra chairs only as a last resort
	if RealSeatsChoice(free, guests) {
		return nil, "TABLES_TOO_SMALL"
	}
	total := sumSeats(chosen, true)
	if total < guests {
		return nil, "TABLES_TOO_SMALL"
	}
	for _, t := range chosen {
		if total-t.Seats-t.ExtraChairs >= guests {
			return nil, "TABLE_IS_EXTRA"
		}
	}
	return &TableChoice{Seats: seats, ExtraUsed: true, ExtraTotal: total}, ""
}
