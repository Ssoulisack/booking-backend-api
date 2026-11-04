package dto

import (
	coredto "backend/internal/core/dto"
	"time"
)

// ---------- staff ----------

// EventRequest creates or changes an event
type EventRequest struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	StartsOn         string `json:"starts_on"`
	EndsOn           string `json:"ends_on"` // empty: one day (the same as starts_on)
	BackgroundKey    string `json:"background_key"`
	AdKey            string `json:"ad_key"`
	HoldMinutes      int    `json:"hold_minutes"`       // 0: 24 hours
	BigBookingTables int    `json:"big_booking_tables"` // 0: 4
	BigHoldMinutes   int    `json:"big_hold_minutes"`   // 0: 60
	DefaultPrice     int64  `json:"default_price"`
	Published        bool   `json:"published"`
}

// EventDTO is an event as the team sees it
type EventDTO struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	StartsOn         string `json:"starts_on"`
	EndsOn           string `json:"ends_on"`
	BackgroundKey    string `json:"background_key"`
	BackgroundURL    string `json:"background_url,omitempty"`
	AdKey            string `json:"ad_key"`
	AdURL            string `json:"ad_url,omitempty"`
	HoldMinutes      int    `json:"hold_minutes"`
	BigBookingTables int    `json:"big_booking_tables"`
	BigHoldMinutes   int    `json:"big_hold_minutes"`
	DefaultPrice     int64  `json:"default_price"`
	Published        bool   `json:"published"`
	Currency         string `json:"currency"`
	// counts of the event's bookings: waiting for payment, paid (confirmed) and cancelled
	Waiting   int64 `json:"waiting"`
	Paid      int64 `json:"paid"`
	Cancelled int64 `json:"cancelled"`
}

// CopyRequest makes a new, unpublished event from an old one (with its prices)
type CopyRequest struct {
	Name     string `json:"name"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`
}

// PricesRequest replaces every price of an event
type PricesRequest struct {
	DefaultPrice int64           `json:"default_price"`
	Zones        []ZonePriceReq  `json:"zones"`
	Tables       []TablePriceReq `json:"tables"`
}

type ZonePriceReq struct {
	ItemID uint  `json:"item_id"`
	Price  int64 `json:"price"`
}

type TablePriceReq struct {
	TableID uint  `json:"table_id"`
	Price   int64 `json:"price"`
}

// PricesDTO is the event's prices with what they apply to, and the price every table ends up with
type PricesDTO struct {
	DefaultPrice int64           `json:"default_price"`
	Currency     string          `json:"currency"`
	Zones        []ZonePriceDTO  `json:"zones"`
	Tables       []TablePriceDTO `json:"tables"`
}

// ZonePriceDTO: a zone with its price (null: none set)
type ZonePriceDTO struct {
	ItemID uint   `json:"item_id"`
	AreaID uint   `json:"area_id"`
	Label  string `json:"label"`
	Color  string `json:"color"`
	Price  *int64 `json:"price"`
}

// TablePriceDTO: a table, its own price (null: none) and the price it is sold at (0: not for sale)
type TablePriceDTO struct {
	TableID uint   `json:"table_id"`
	AreaID  uint   `json:"area_id"`
	Name    string `json:"name"`
	Seats   int    `json:"seats"`
	Own     *int64 `json:"own"`
	Price   int64  `json:"price"`
}

// PayInfoRequest and PayInfoDTO: how customers pay the branch for event tables
type PayInfoRequest struct {
	Whatsapp string `json:"whatsapp"`
	QRKey    string `json:"qr_key"`
	Note     string `json:"note"`
}

type PayInfoDTO struct {
	Whatsapp string `json:"whatsapp"`
	QRKey    string `json:"qr_key"`
	QRURL    string `json:"qr_url,omitempty"`
	Note     string `json:"note"`
}

// ---------- public ----------

// PublicEventDTO is an event card (the small ad) on the home and branch pages
type PublicEventDTO struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	StartsOn      string `json:"starts_on"`
	EndsOn        string `json:"ends_on"`
	BackgroundURL string `json:"background_url,omitempty"`
	AdURL         string `json:"ad_url,omitempty"`
	FromPrice     int64  `json:"from_price"` // the cheapest table (0: none for sale)
	Currency      string `json:"currency"`
}

// SaleTableDTO is one table of the event day: its price, and whether it is already taken
type SaleTableDTO struct {
	ID    uint  `json:"id"`
	Price int64 `json:"price"` // 0: not for sale
	Sold  bool  `json:"sold"`
}

// PublicEventDetailDTO is an event day for the customer: the days, the open hours of the chosen day
// and every table with its price. The geometry comes from the branch's floor plans.
type PublicEventDetailDTO struct {
	PublicEventDTO
	Days             []string       `json:"days"`
	Date             string         `json:"date"`
	OpensAt          *time.Time     `json:"opens_at,omitempty"`
	ClosesAt         *time.Time     `json:"closes_at,omitempty"`
	Tables           []SaleTableDTO `json:"tables"`
	HoldMinutes      int            `json:"hold_minutes"`
	BigBookingTables int            `json:"big_booking_tables"`
	BigHoldMinutes   int            `json:"big_hold_minutes"`
}

// BookRequest buys tables for one day of an event
type BookRequest struct {
	Date     string             `json:"date"` // empty: the first day still to come
	TableIDs []uint             `json:"table_ids"`
	Guests   int                `json:"guests"`
	Guest    *coredto.GuestInfo `json:"guest"`
	Notes    string             `json:"notes"`
}

// BookedDTO is the booking and how to pay for it
type BookedDTO struct {
	coredto.CreatedBookingDTO
	Pay PayDTO `json:"pay"`
}

// PayDTO: the total, how long the tables are held, and where to pay
type PayDTO struct {
	Total      int64      `json:"total"`
	Currency   string     `json:"currency"`
	HoldUntil  *time.Time `json:"hold_until,omitempty"`
	BigBooking bool       `json:"big_booking"` // more tables than the event's limit: a shorter hold
	Whatsapp   string     `json:"whatsapp"`
	QRURL      string     `json:"qr_url,omitempty"`
	Note       string     `json:"note"`
}
