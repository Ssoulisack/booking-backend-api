package domain

import core "backend/internal/core/domain"

// PriceBook works out what each table costs at an event: its own price, else the price of the first
// priced zone that holds it, else the event's default price
type PriceBook struct {
	zones     []core.FloorItem
	zonePrice map[uint]int64
	own       map[uint]int64
	fallback  int64
}

// NewPriceBook reads the event's prices; zones are the zones of the branch, lowest id first
func NewPriceBook(e Event, prices []Price, zones []core.FloorItem) PriceBook {
	pb := PriceBook{zones: zones, zonePrice: map[uint]int64{}, own: map[uint]int64{}, fallback: e.DefaultPrice}
	for _, p := range prices {
		switch p.Kind {
		case PriceZone:
			pb.zonePrice[p.RefID] = p.Amount
		case PriceTable:
			pb.own[p.RefID] = p.Amount
		}
	}
	return pb
}

// Of returns the table's price at this event (0: not for sale)
func (pb PriceBook) Of(t core.Resource) int64 {
	if v, ok := pb.own[t.ID]; ok {
		return v
	}
	for _, z := range pb.zones {
		if z.AreaID != t.SportID || !z.Holds(t) {
			continue
		}
		if v, ok := pb.zonePrice[z.ID]; ok {
			return v
		}
	}
	return pb.fallback
}

// ZonePrice is the price set on a zone (false: none set)
func (pb PriceBook) ZonePrice(zoneID uint) (int64, bool) {
	v, ok := pb.zonePrice[zoneID]
	return v, ok
}

// OwnPrice is the price set on one table (false: none set)
func (pb PriceBook) OwnPrice(tableID uint) (int64, bool) {
	v, ok := pb.own[tableID]
	return v, ok
}
