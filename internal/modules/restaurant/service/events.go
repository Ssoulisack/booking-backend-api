package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"context"
	"time"
)

// Event days. The events module decides what is sold, at which price and for how long it is held;
// this file stores the booking like any other (who, code, private link, emails) through the booking
// module's kernel.

// SetEventDays gives the booking service a way to ask whether an event takes over a day
func (s *Service) SetEventDays(e output.EventDays) { s.eventDays = e }

// eventOn returns the event that takes over the day (YYYY-MM-DD) of a restaurant, or nil
func (s *Service) eventOn(ctx context.Context, branchID uint, day string) (*domain.EventDay, error) {
	if s.eventDays == nil {
		return nil, nil
	}
	return s.eventDays.EventOn(ctx, branchID, day)
}

// CreateEventBooking books tables for an event day. The booking waits for payment (pending, unpaid)
// and holds its tables from opening to closing of that day until PayBy.
func (s *Service) CreateEventBooking(ctx context.Context, in dto.EventBookingInput) (*dto.CreatedBookingDTO, error) {
	b, err := s.kernel.Published(ctx, in.Slug)
	if err != nil {
		return nil, err
	}
	switch {
	case b.Type != domain.BranchTypeRestaurant:
		return nil, conflict("NOT_A_RESTAURANT")
	case in.Guests < 1:
		return nil, badRequest("GUESTS_REQUIRED")
	case len(in.Tables) == 0:
		return nil, badRequest("TABLE_REQUIRED")
	case len([]rune(in.Notes)) > maxNotesChars:
		return nil, badRequest("NOTES_TOO_LONG")
	}
	who, err := s.kernel.WhoBooks(b, in.Actor, in.Guest)
	if err != nil {
		return nil, err
	}
	loc := locationOf(b)
	day, err := time.ParseInLocation("2006-01-02", in.Date, loc)
	if err != nil {
		return nil, badRequest("INVALID_DATE")
	}
	// the event day runs from opening to closing
	open, err := s.kernel.OpenIntervals(ctx, b.ID, domain.OwnerVenue, 0, day)
	if err != nil {
		return nil, err
	}
	if len(open) == 0 {
		return nil, conflict("VENUE_CLOSED")
	}
	start, end := open[0].Start, open[len(open)-1].End
	span := domain.Interval{Start: start, End: end}
	closures, err := s.closures.Overlapping(ctx, b.ID, start, end)
	if err != nil {
		return nil, err
	}
	closedTables := map[uint]bool{}
	for _, c := range closures {
		if !domain.AnyOverlap(span, []domain.Interval{{Start: c.StartsAt, End: c.EndsAt}}) {
			continue
		}
		switch c.OwnerType {
		case domain.OwnerVenue:
			return nil, conflict("VENUE_CLOSED")
		case domain.OwnerResource:
			closedTables[c.OwnerID] = true
		}
	}

	live, _, err := s.liveTables(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	byID := map[uint]domain.Resource{}
	for _, t := range live {
		byID[t.ID] = t
	}
	var chosen []domain.Resource
	var total int64
	seen := map[uint]bool{}
	for _, sale := range in.Tables {
		t, ok := byID[sale.ID]
		if !ok || seen[sale.ID] || closedTables[sale.ID] || sale.Price < 0 {
			return nil, conflict("TABLE_NOT_AVAILABLE")
		}
		seen[sale.ID] = true
		chosen = append(chosen, t)
		total += sale.Price
	}

	rule, err := s.rules.Effective(ctx, b.ID, 0, 0)
	if err != nil {
		return nil, err
	}
	bk, rawToken, err := s.kernel.NewBooking(b, who, chosen[0].SportID, start, end, rule, in.Notes)
	if err != nil {
		return nil, err
	}
	now := s.now()
	payBy := now.Add(time.Duration(in.HoldMinutes) * time.Minute)
	if payBy.After(end) {
		payBy = end
	}
	eventID := in.EventID
	bk.Status = domain.BookingPending // waits for the money, then staff confirm it paid
	bk.Guests, bk.Currency, bk.TotalPrice = in.Guests, b.Currency, total
	bk.EventID, bk.PayBy = &eventID, &payBy
	bk.Event = &domain.EventBrief{ID: eventID, Name: in.EventName, StartTime: in.EventStartTime, EndTime: in.EventEndTime}
	bk.CancellableUntil = now // a customer cannot cancel online: they talk to the team, who can cancel any time
	bk.Zones = s.zonesOf(ctx, b.ID, chosen)
	for i, t := range chosen {
		id := t.ID
		bk.Items = append(bk.Items, domain.BookingItem{BranchID: b.ID, Kind: domain.OwnerResource, ResourceID: &id,
			Seats: t.Seats, StartsAt: start, EndsAt: end, Price: in.Tables[i].Price, Active: true})
	}
	// the database refuses the insert if someone took one of the tables meanwhile (409)
	if err := s.bookings.Create(ctx, bk); err != nil {
		return nil, err
	}
	return s.kernel.Created(ctx, b, bk, rawToken, in.Actor, who), nil
}

// OpenOn tells when the venue is open on a day (YYYY-MM-DD, branch time zone): from the first opening
// to the last closing; open is false when it is closed that day
func (s *Service) OpenOn(ctx context.Context, branchID uint, date string) (start, end time.Time, open bool, err error) {
	_, loc, err := s.kernel.Branch(ctx, branchID)
	if err != nil {
		return start, end, false, err
	}
	day, err := parseDay(date, loc)
	if err != nil {
		return start, end, false, err
	}
	intervals, err := s.kernel.OpenIntervals(ctx, branchID, domain.OwnerVenue, 0, day)
	if err != nil || len(intervals) == 0 {
		return start, end, false, err
	}
	return intervals[0].Start, intervals[len(intervals)-1].End, true, nil
}
