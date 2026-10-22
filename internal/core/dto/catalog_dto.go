package dto

import "time"

// ListQuery carries pagination and the optional filters of the admin list endpoints.
// Each service reads only the fields that apply to it.
type ListQuery struct {
	Page      int    `query:"page"`
	Limit     int    `query:"limit"`
	Keyword   string `query:"keyword"`
	SportID   uint   `query:"sport_id"`
	OwnerType string `query:"owner_type"`
	OwnerID   uint   `query:"owner_id"`
	Status    string `query:"status"`
	Active    *bool  `query:"active"`
	// class lists: one day (date) or a range (from, to: both days included), YYYY-MM-DD in the branch timezone
	Date string `query:"date"`
	From string `query:"from"`
	To   string `query:"to"`
}

// PagedResult is one page of a list
type PagedResult[T any] struct {
	Items []T
	Meta  Meta
}

type SportRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageKey    string `json:"image_key"`
	Active      *bool  `json:"active"`
	// Type is one of domain.SportTypes (empty: "other")
	Type string `json:"type"`
}

type SportDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ImageKey    string `json:"image_key"`
	ImageURL    string `json:"image_url"`
	Active      bool   `json:"active"`
	Type        string `json:"type"`
}

type ResourceRequest struct {
	SportID uint   `json:"sport_id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Active  *bool  `json:"active"`
}

type ResourceDTO struct {
	ID      uint   `json:"id"`
	SportID uint   `json:"sport_id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Active  bool   `json:"active"`
}

type CoachRequest struct {
	Name     string `json:"name"`
	Bio      string `json:"bio"`
	Phone    string `json:"phone"`
	ImageKey string `json:"image_key"`
	SportIDs []uint `json:"sport_ids"`
	Active   *bool  `json:"active"`
}

type SportRefDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type CoachDTO struct {
	ID       uint          `json:"id"`
	Name     string        `json:"name"`
	Bio      string        `json:"bio"`
	Phone    string        `json:"phone"`
	ImageKey string        `json:"image_key"`
	ImageURL string        `json:"image_url"`
	Active   bool          `json:"active"`
	Sports   []SportRefDTO `json:"sports"`
}

type MediaDTO struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// OpeningHourDTO is one open interval; weekday 0 = Sunday, times are "HH:MM"
type OpeningHourDTO struct {
	Weekday int    `json:"weekday"`
	Open    string `json:"open"`
	Close   string `json:"close"`
}

type OpeningHoursRequest struct {
	Hours []OpeningHourDTO `json:"hours"`
}

type ClosureRequest struct {
	OwnerType string    `json:"owner_type"`
	OwnerID   uint      `json:"owner_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Reason    string    `json:"reason"`
}

type ClosureDTO struct {
	ID        uint      `json:"id"`
	OwnerType string    `json:"owner_type"`
	OwnerID   uint      `json:"owner_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Reason    string    `json:"reason"`
}

type PriceRequest struct {
	PricePerHour int64  `json:"price_per_hour"`
	Currency     string `json:"currency"`
}

type PriceDTO struct {
	OwnerType    string `json:"owner_type"`
	OwnerID      uint   `json:"owner_id"`
	PricePerHour int64  `json:"price_per_hour"`
	Currency     string `json:"currency"`
}

type ClassSessionRequest struct {
	SportID    uint      `json:"sport_id"`
	CoachID    *uint     `json:"coach_id"`
	ResourceID *uint     `json:"resource_id"`
	Title      string    `json:"title"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Capacity   int       `json:"capacity"`
	Price      int64     `json:"price"`
	Currency   string    `json:"currency"`
	Active     *bool     `json:"active"`
}

type ClassSessionDTO struct {
	ID         uint      `json:"id"`
	SportID    uint      `json:"sport_id"`
	CoachID    *uint     `json:"coach_id"`
	ResourceID *uint     `json:"resource_id"`
	Title      string    `json:"title"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Capacity   int       `json:"capacity"`
	Price      int64     `json:"price"`
	Currency   string    `json:"currency"`
	Active     bool      `json:"active"`
	SeatsLeft  *int      `json:"seats_left,omitempty"` // public listing only
}

// RuleRequest sets a booking rule; a null field inherits from the next wider scope
type RuleRequest struct {
	MinDurationMin               *int `json:"min_duration_min"`
	MaxDurationMin               *int `json:"max_duration_min"`
	DurationStepMin              *int `json:"duration_step_min"`
	StartStepMin                 *int `json:"start_step_min"`
	CancelBeforeHours            *int `json:"cancel_before_hours"`
	MaxDaysAhead                 *int `json:"max_days_ahead"`
	MinNoticeMin                 *int `json:"min_notice_min"`
	Capacity                     *int `json:"capacity"`
	MaxActiveBookingsPerCustomer *int `json:"max_active_bookings_per_customer"`
}

type RuleDTO struct {
	ScopeType string `json:"scope_type"`
	ScopeID   uint   `json:"scope_id"`
	RuleRequest
}

type UserStatusRequest struct {
	Status bool `json:"status"`
}

// RepeatClassRequest creates the same class on every chosen weekday between two dates, e.g.
// Monday to Friday 18:00-20:00 from 2026-11-01 to 2026-11-30. Weekday 0 = Sunday.
type RepeatClassRequest struct {
	SportID    uint   `json:"sport_id"`
	CoachID    *uint  `json:"coach_id"`
	ResourceID *uint  `json:"resource_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"` // HH:MM
	EndTime    string `json:"end_time"`   // HH:MM, "24:00" allowed
	Weekdays   []int  `json:"weekdays"`
	From       string `json:"from"` // YYYY-MM-DD, included
	To         string `json:"to"`   // YYYY-MM-DD, included
	Capacity   int    `json:"capacity"`
	Price      int64  `json:"price"`
	Currency   string `json:"currency"`
	Active     *bool  `json:"active"`
}

type SkippedDate struct {
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

type RepeatClassResultDTO struct {
	Created  int               `json:"created"`
	Sessions []ClassSessionDTO `json:"sessions"`
	Skipped  []SkippedDate     `json:"skipped"` // days closed for the venue, court or coach
}
