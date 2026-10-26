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

// MediaUseDTO is one place a file is shown: kind is sport, coach, cover or logo
type MediaUseDTO struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// MediaItemDTO is a file in the media library with where it is used (empty: safe to delete)
type MediaItemDTO struct {
	Key       string        `json:"key"`
	URL       string        `json:"url"`
	SizeBytes int64         `json:"size_bytes"`
	CreatedAt time.Time     `json:"created_at"`
	UsedBy    []MediaUseDTO `json:"used_by"`
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
	// on create: timetable classes in the closed time that already have bookings, so they were
	// kept (classes without bookings are removed); staff contact those customers
	ClassesKept []ClassSessionDTO `json:"classes_kept,omitempty"`
	// on create: how many timetable classes without bookings were removed
	ClassesRemoved int `json:"classes_removed,omitempty"`
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
	// the weekly timetable that made it; null for a class added on its own
	TimetableID *uint `json:"timetable_id"`
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

// TimetableRequest is a weekly class that keeps running: weekdays 0 (Sunday) to 6, times HH:MM in
// the branch time zone, starts_on YYYY-MM-DD, ends_on empty for no end
type TimetableRequest struct {
	SportID    uint   `json:"sport_id"`
	CoachID    *uint  `json:"coach_id"`
	ResourceID *uint  `json:"resource_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Weekdays   []int  `json:"weekdays"`
	StartsOn   string `json:"starts_on"`
	EndsOn     string `json:"ends_on"`
	Capacity   int    `json:"capacity"`
	Price      int64  `json:"price"`
	Currency   string `json:"currency"`
	Active     *bool  `json:"active"`
}

type TimetableDTO struct {
	ID         uint   `json:"id"`
	SportID    uint   `json:"sport_id"`
	CoachID    *uint  `json:"coach_id"`
	ResourceID *uint  `json:"resource_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Weekdays   []int  `json:"weekdays"`
	StartsOn   string `json:"starts_on"`
	EndsOn     string `json:"ends_on"` // "" = no end
	Capacity   int    `json:"capacity"`
	Price      int64  `json:"price"`
	Currency   string `json:"currency"`
	Active     bool   `json:"active"`
	// classes are made up to this day (today + 30 days, or the end); "" before the start
	FilledUntil string `json:"filled_until"`
}

// TimetableResultDTO answers a create or an edit: what was made now and which days were skipped
type TimetableResultDTO struct {
	Timetable TimetableDTO  `json:"timetable"`
	Created   int           `json:"created"`
	Skipped   []SkippedDate `json:"skipped"` // "closed" or "taken" (the space or coach is booked)
	// classes that already had bookings keep their old details (time, space, seats)
	KeptBooked int `json:"kept_booked"`
}

// TimetableStopDTO answers a stop: upcoming classes without bookings are removed, booked ones stay
type TimetableStopDTO struct {
	Removed    int `json:"removed"`
	KeptBooked int `json:"kept_booked"`
}
