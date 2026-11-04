package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	bookingport "backend/internal/modules/booking/port"
	"context"
	"slices"
	"strings"
	"time"
)

// Restaurant branches: customers book tables for a group. A booking has no length: it holds its
// tables until the bill is closed (one at a time) or for the branch's hold minutes (several a day),
// and at the latest until closing time. The booking items carry that end, so the database's
// double-booking guard keeps working unchanged.

// liveTables lists the tables customers can book: active tables with seats in active areas
func (s *Service) liveTables(ctx context.Context, branchID uint) ([]domain.Resource, map[uint]domain.Sport, error) {
	areas, _, err := s.sports.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"active": true}})
	if err != nil {
		return nil, nil, err
	}
	byID := map[uint]domain.Sport{}
	for _, a := range areas {
		byID[a.ID] = a
	}
	all, _, err := s.resources.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"active": true}})
	if err != nil {
		return nil, nil, err
	}
	tables := slices.DeleteFunc(all, func(r domain.Resource) bool {
		_, ok := byID[r.SportID]
		return !ok || r.Seats < 1
	})
	return tables, byID, nil
}

// ---------- the public floor plans ----------

// FloorPlans returns every bookable area of a restaurant with its tables and objects, lowest floor first
func (s *Service) FloorPlans(ctx context.Context, slug string) ([]dto.PublicAreaDTO, error) {
	b, err := s.kernel.Published(ctx, slug)
	if err != nil {
		return nil, err
	}
	if b.Type != domain.BranchTypeRestaurant {
		return nil, conflict("NOT_A_RESTAURANT")
	}
	tables, areas, err := s.liveTables(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	out := []dto.PublicAreaDTO{}
	for _, a := range areas {
		area := dto.PublicAreaDTO{ID: a.ID, Name: a.Name, Floor: a.Floor, PlanWidth: a.PlanWidth, PlanHeight: a.PlanHeight,
			Tables: []dto.PublicTableDTO{}, Items: []dto.FloorItemDTO{}}
		for _, t := range tables {
			if t.SportID == a.ID {
				area.Tables = append(area.Tables, dto.PublicTableDTO{ID: t.ID, Name: t.Name, TableFields: dto.TableFields{
					Seats: t.Seats, ExtraChairs: t.ExtraChairs, Shape: t.Shape,
					PlanX: t.PlanX, PlanY: t.PlanY, PlanW: t.PlanW, PlanH: t.PlanH, Rotation: t.Rotation,
				}})
			}
		}
		if s.floorItems != nil {
			items, _, err := s.floorItems.List(ctx, b.ID, output.ListQuery{Filters: map[string]any{"area_id": a.ID}})
			if err != nil {
				return nil, err
			}
			area.Items = mapSlice(items, floorItemDTO)
		}
		out = append(out, area)
	}
	slices.SortFunc(out, func(x, y dto.PublicAreaDTO) int {
		if x.Floor != y.Floor {
			return x.Floor - y.Floor
		}
		return int(x.ID) - int(y.ID)
	})
	return out, nil
}

// ---------- when tables are free ----------

// tableTime is one start time of a day: until when a booking then holds its tables, and the
// tables free for that whole time
type tableTime struct {
	start, end time.Time
	arriveBy   time.Time // not there by then: no-show, the tables go to others
	free       []domain.Resource
}

// tableTimes lists the start times of day for a restaurant with the free tables at each.
// staff skip the notice, days-ahead and last-call limits (never a time that is over).
func (s *Service) tableTimes(ctx context.Context, b *domain.Branch, day time.Time, staff bool) ([]tableTime, *domain.EffectiveRule, error) {
	rule, err := s.rules.Effective(ctx, b.ID, 0, 0)
	if err != nil {
		return nil, nil, err
	}
	tables, _, err := s.liveTables(ctx, b.ID)
	if err != nil {
		return nil, nil, err
	}
	open, err := s.kernel.OpenIntervals(ctx, b.ID, domain.OwnerVenue, 0, day)
	if err != nil {
		return nil, nil, err
	}
	from, to := day, day.AddDate(0, 0, 2) // a booking may run past midnight
	closures, err := s.closures.Overlapping(ctx, b.ID, from, to)
	if err != nil {
		return nil, nil, err
	}
	busy := map[uint][]domain.Interval{}
	var venueClosed []domain.Interval
	for _, c := range closures {
		iv := domain.Interval{Start: c.StartsAt, End: c.EndsAt}
		switch c.OwnerType {
		case domain.OwnerVenue:
			venueClosed = append(venueClosed, iv)
		case domain.OwnerResource:
			busy[c.OwnerID] = append(busy[c.OwnerID], iv)
		}
	}
	for _, t := range tables {
		taken, err := s.kernel.BusyIntervals(ctx, b.ID, domain.OwnerResource, t.ID, from, to)
		if err != nil {
			return nil, nil, err
		}
		busy[t.ID] = append(busy[t.ID], taken...)
	}

	now := s.now()
	earliest := now.Add(time.Duration(rule.MinNoticeMin) * time.Minute)
	latest := startOfDay(now, day.Location()).AddDate(0, 0, rule.MaxDaysAhead+1)
	lastCall := time.Duration(b.LastCallMinutes) * time.Minute
	if b.BookingStyle == domain.BookingStyleArriveBy {
		earliest = now // a bar takes a booking for tonight right away
	}
	if staff {
		earliest, latest, lastCall = now.Add(-15*time.Minute), now.AddDate(1, 0, 1), 0
	}
	step := time.Duration(max(rule.StartStepMin, 15)) * time.Minute
	freeFor := func(span domain.Interval) []domain.Resource {
		return slices.DeleteFunc(slices.Clone(tables), func(r domain.Resource) bool {
			return domain.AnyOverlap(span, busy[r.ID])
		})
	}

	// a bar: no start time to pick, the evening from now (or opening) until closing; arrive by
	// the set time, or within the late-booking minutes when booking after it
	if b.BookingStyle == domain.BookingStyleArriveBy {
		deadlineMin, _ := parseClock(b.ArriveByTime, false)
		for _, iv := range open {
			if !iv.Start.Before(latest) || !now.Before(iv.End.Add(-lastCall)) {
				continue
			}
			start := iv.Start
			if now.After(start) {
				start = now.Truncate(time.Minute)
			}
			arriveBy := day.Add(time.Duration(deadlineMin) * time.Minute)
			if late := now.Add(time.Duration(b.LateBookingMinutes) * time.Minute); late.After(arriveBy) {
				arriveBy = late.Truncate(time.Minute)
			}
			if arriveBy.Before(start) {
				arriveBy = start
			}
			span := domain.Interval{Start: start, End: iv.End}
			if domain.AnyOverlap(span, venueClosed) {
				continue
			}
			return []tableTime{{start: start, end: iv.End, arriveBy: arriveBy, free: freeFor(span)}}, rule, nil
		}
		return nil, rule, nil
	}

	var out []tableTime
	for _, iv := range open {
		for t := iv.Start; t.Before(iv.End) && !t.After(iv.End.Add(-lastCall)); t = t.Add(step) {
			if t.Before(earliest) || !t.Before(latest) {
				continue
			}
			end := iv.End // one at a time: until the bill is closed, at the latest closing time
			if b.TableMode == domain.TableModeHold {
				end = minTime(t.Add(time.Duration(b.HoldMinutes)*time.Minute), iv.End)
			}
			span := domain.Interval{Start: t, End: end}
			if domain.AnyOverlap(span, venueClosed) {
				continue
			}
			arriveBy := t.Add(time.Duration(b.LateMinutes) * time.Minute)
			out = append(out, tableTime{start: t, end: end, arriveBy: arriveBy, free: freeFor(span)})
		}
	}
	return out, rule, nil
}

// TableAvailability answers "which times and tables are free for this many guests on this day"
func (s *Service) TableAvailability(ctx context.Context, slug string, q dto.TableAvailabilityQuery) (*dto.TableAvailabilityDTO, error) {
	b, err := s.kernel.Published(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.tableAvailability(ctx, b, q, false)
}

func (s *Service) tableAvailability(ctx context.Context, b *domain.Branch, q dto.TableAvailabilityQuery, staff bool) (*dto.TableAvailabilityDTO, error) {
	if b.Type != domain.BranchTypeRestaurant {
		return nil, conflict("NOT_A_RESTAURANT")
	}
	if q.Guests < 1 {
		return nil, badRequest("GUESTS_REQUIRED")
	}
	if !staff && q.Guests > b.MaxGuestsOnline {
		return nil, badRequest("TOO_MANY_GUESTS_ONLINE")
	}
	loc := locationOf(b)
	day, err := time.ParseInLocation("2006-01-02", q.Date, loc)
	if err != nil {
		return nil, badRequest("INVALID_DATE")
	}
	// an event takes the day over: its tables are sold, not booked free
	if !staff {
		ev, err := s.eventOn(ctx, b.ID, q.Date)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			return &dto.TableAvailabilityDTO{
				Date: q.Date, Timezone: loc.String(), Guests: q.Guests, MaxGuestsOnline: b.MaxGuestsOnline,
				LateMinutes: b.LateMinutes, BookingStyle: b.BookingStyle, Times: []dto.TableTimeDTO{},
				Event: &dto.EventRefDTO{ID: ev.ID, Name: ev.Name},
			}, nil
		}
	}
	times, _, err := s.tableTimes(ctx, b, day, staff)
	if err != nil {
		return nil, err
	}
	out := &dto.TableAvailabilityDTO{
		Date: q.Date, Timezone: loc.String(), Guests: q.Guests, MaxGuestsOnline: b.MaxGuestsOnline,
		LateMinutes: b.LateMinutes, BookingStyle: b.BookingStyle, Times: []dto.TableTimeDTO{},
	}
	for _, tt := range times {
		if !domain.AnyChoice(tt.free, q.Guests) {
			continue
		}
		out.Times = append(out.Times, dto.TableTimeDTO{
			StartsAt:           tt.start.In(loc),
			ArriveBy:           tt.arriveBy.In(loc),
			FreeTableIDs:       mapSlice(tt.free, func(r domain.Resource) uint { return r.ID }),
			ExtraChairsAllowed: !domain.RealSeatsChoice(tt.free, q.Guests),
		})
	}
	return out, nil
}

// zonesOf names the zones the tables stand in, in plan order ("Smoking, VIP"); "" when none
func (s *Service) zonesOf(ctx context.Context, branchID uint, tables []domain.Resource) string {
	if s.floorItems == nil || len(tables) == 0 {
		return ""
	}
	items, _, err := s.floorItems.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"area_id": tables[0].SportID}})
	if err != nil {
		return "" // a name on the receipt is not worth failing a booking for
	}
	var names []string
	for _, z := range items {
		if z.Label != "" && !slices.Contains(names, z.Label) &&
			slices.ContainsFunc(tables, func(t domain.Resource) bool { return z.Holds(t) }) {
			names = append(names, z.Label)
		}
	}
	return strings.Join(names, ", ")
}

// ---------- booking tables ----------

// createTableBooking books the chosen tables for the group at one of the free start times
func (s *Service) createTableBooking(ctx context.Context, b *domain.Branch, who *bookingport.Booker, req dto.CreateBookingRequest) (*domain.Booking, string, error) {
	switch {
	case req.Guests < 1:
		return nil, "", badRequest("GUESTS_REQUIRED")
	case who.Staff == nil && req.Guests > b.MaxGuestsOnline:
		return nil, "", badRequest("TOO_MANY_GUESTS_ONLINE")
	case len(req.TableIDs) == 0:
		return nil, "", badRequest("TABLE_REQUIRED")
	case len(req.TableIDs) > 20:
		return nil, "", badRequest("TOO_MANY_TABLES")
	case req.StartsAt.IsZero():
		return nil, "", badRequest("STARTS_AT_REQUIRED")
	}
	loc := locationOf(b)
	start := req.StartsAt.In(loc)
	if who.Staff == nil {
		ev, err := s.eventOn(ctx, b.ID, start.Format("2006-01-02"))
		if err != nil {
			return nil, "", err
		}
		if ev != nil {
			return nil, "", conflict("EVENT_DAY") // that day tables are sold for the event
		}
	}
	times, rule, err := s.tableTimes(ctx, b, startOfDay(start, loc), who.Staff != nil)
	if err != nil {
		return nil, "", err
	}
	i := slices.IndexFunc(times, func(tt tableTime) bool { return tt.start.Equal(start) })
	if b.BookingStyle == domain.BookingStyleArriveBy && len(times) > 0 {
		i = 0 // a bar books the evening of that day: the start is "now or opening", not picked
	}
	if i < 0 {
		return nil, "", conflict("SLOT_NOT_AVAILABLE")
	}
	tt := times[i]
	var chosen []domain.Resource
	for _, id := range req.TableIDs {
		k := slices.IndexFunc(tt.free, func(r domain.Resource) bool { return r.ID == id })
		if k < 0 || slices.ContainsFunc(chosen, func(r domain.Resource) bool { return r.ID == id }) {
			return nil, "", conflict("TABLE_NOT_AVAILABLE")
		}
		chosen = append(chosen, tt.free[k])
	}
	if _, code := domain.CheckTables(chosen, req.Guests, tt.free); code != "" {
		return nil, "", badRequest(code)
	}
	if err := s.kernel.CheckBookingLimit(ctx, b.ID, who, rule); err != nil {
		return nil, "", err
	}

	bk, rawToken, err := s.kernel.NewBooking(b, who, chosen[0].SportID, tt.start, tt.end, rule, req.Notes)
	if err != nil {
		return nil, "", err
	}
	if who.Staff == nil && b.ApprovalAboveGuests != nil && req.Guests > *b.ApprovalAboveGuests {
		bk.Status = domain.BookingPending // a big group waits for the team's OK
	}
	arriveBy := tt.arriveBy
	bk.Guests, bk.Currency, bk.ArriveBy = req.Guests, b.Currency, &arriveBy
	bk.Zones = s.zonesOf(ctx, b.ID, chosen)
	for _, t := range chosen {
		id := t.ID
		bk.Items = append(bk.Items, domain.BookingItem{BranchID: b.ID, Kind: domain.OwnerResource, ResourceID: &id,
			Seats: t.Seats, StartsAt: tt.start, EndsAt: tt.end, Active: true})
	}
	// the database refuses the insert if someone took one of the tables meanwhile (409)
	if err := s.bookings.Create(ctx, bk); err != nil {
		return nil, "", err
	}
	return bk, rawToken, nil
}
