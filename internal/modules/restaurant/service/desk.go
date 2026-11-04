package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	bookingport "backend/internal/modules/booking/port"
	"context"
	"log"
	"slices"
	"strings"
	"time"
)

// Restaurant desk: staff mark a group arrived, close the bill (the tables are free at once), mark a
// no-show, move a group to other tables and seat walk-ins. A job settles late groups and evenings
// that are over.

// tableBooking loads an active restaurant booking of the branch
func (s *Service) tableBooking(ctx context.Context, branchID, id uint) (*domain.Booking, error) {
	bk, err := s.bookings.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if bk.Guests < 1 {
		return nil, badRequest("NOT_A_TABLE_BOOKING")
	}
	if !bk.IsActive() {
		return nil, conflict("BOOKING_NOT_ACTIVE")
	}
	return bk, nil
}

// MarkArrived records that the group came in (a confirmed table booking whose time is not over)
func (s *Service) MarkArrived(ctx context.Context, branchID, id uint) (*dto.BookingDTO, error) {
	bk, err := s.tableBooking(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	now := s.now()
	switch {
	case bk.Status != domain.BookingConfirmed:
		return nil, conflict("CONFIRM_FIRST")
	case bk.ArrivedAt != nil:
		return nil, conflict("ALREADY_ARRIVED")
	case !now.Before(bk.EndsAt):
		return nil, conflict("BOOKING_OVER")
	}
	if err := s.bookings.SetArrived(ctx, bk, now); err != nil {
		return nil, err
	}
	return s.kernel.GetBooking(ctx, branchID, id)
}

// MoveTables puts the group at other tables from now on (or from its start when it has not begun).
// Staff may join any tables with enough chairs (extra chairs count); a table taken at that time is refused.
func (s *Service) MoveTables(ctx context.Context, branchID, id uint, req dto.MoveTablesRequest) (*dto.BookingDTO, error) {
	bk, err := s.tableBooking(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	now := s.now().Truncate(time.Minute)
	if !now.Before(bk.EndsAt) {
		return nil, conflict("BOOKING_OVER")
	}
	if len(req.TableIDs) == 0 {
		return nil, badRequest("TABLE_REQUIRED")
	}
	tables, _, err := s.liveTables(ctx, branchID)
	if err != nil {
		return nil, err
	}
	var chosen []domain.Resource
	seats := 0
	for _, tid := range req.TableIDs {
		k := slices.IndexFunc(tables, func(r domain.Resource) bool { return r.ID == tid })
		if k < 0 || slices.ContainsFunc(chosen, func(r domain.Resource) bool { return r.ID == tid }) {
			return nil, badRequest("TABLE_NOT_AVAILABLE")
		}
		chosen = append(chosen, tables[k])
		seats += tables[k].Seats + tables[k].ExtraChairs
	}
	if seats < bk.Guests {
		return nil, badRequest("TABLES_TOO_SMALL")
	}
	from := bk.StartsAt
	if now.After(from) {
		from = now
	}
	items := make([]domain.BookingItem, 0, len(chosen))
	for _, t := range chosen {
		tid, bid := t.ID, bk.ID
		items = append(items, domain.BookingItem{BookingID: &bid, BranchID: branchID, Kind: domain.OwnerResource,
			ResourceID: &tid, Seats: t.Seats, StartsAt: from, EndsAt: bk.EndsAt, Active: true})
	}
	bk.SportID = chosen[0].SportID
	bk.Zones = s.zonesOf(ctx, branchID, chosen)
	if err := s.bookings.MoveItems(ctx, bk, from, items); err != nil {
		return nil, err
	}
	return s.kernel.GetBooking(ctx, branchID, id)
}

// WalkIn seats a group that came without booking at free tables, from now until the tables are
// next needed: closing time (one at a time) or the hold minutes. A table booked later tonight is
// refused, unless the owner allows walk-ins before a booking: then they must leave by its start.
func (s *Service) WalkIn(ctx context.Context, branchID uint, actor *domain.User, req dto.WalkInRequest) (*dto.BookingDTO, error) {
	b, loc, err := s.kernel.Branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	switch {
	case b.Type != domain.BranchTypeRestaurant:
		return nil, conflict("NOT_A_RESTAURANT")
	case req.Guests < 1:
		return nil, badRequest("GUESTS_REQUIRED")
	case len(req.TableIDs) == 0:
		return nil, badRequest("TABLE_REQUIRED")
	case len([]rune(req.Notes)) > maxNotesChars:
		return nil, badRequest("NOTES_TOO_LONG")
	}
	guest := dto.GuestInfo{Name: strings.TrimSpace(req.Name), Phone: strings.TrimSpace(req.Phone)}
	if guest.Phone != "" {
		if guest.Name == "" {
			guest.Name = "Walk-in"
		}
		clean, err := s.kernel.CleanGuest(guest)
		if err != nil {
			return nil, err
		}
		guest = *clean
	} else if len([]rune(guest.Name)) > maxGuestName {
		return nil, badRequest("GUEST_NAME_REQUIRED")
	}
	if guest.Name == "" {
		guest.Name = "Walk-in"
	}

	now := s.now() // not rounded down: a table whose bill was closed a moment ago is free at once
	// the opening stretch we are in (yesterday's, when it runs past midnight)
	var end time.Time
	for _, day := range []time.Time{startOfDay(now, loc), startOfDay(now, loc).AddDate(0, 0, -1)} {
		open, err := s.kernel.OpenIntervals(ctx, b.ID, domain.OwnerVenue, 0, day)
		if err != nil {
			return nil, err
		}
		for _, iv := range open {
			if !now.Before(iv.Start) && now.Before(iv.End) {
				end = iv.End
			}
		}
		if !end.IsZero() {
			break
		}
	}
	if end.IsZero() {
		return nil, conflict("VENUE_CLOSED")
	}
	if b.TableMode == domain.TableModeHold {
		end = minTime(now.Add(time.Duration(b.HoldMinutes)*time.Minute), end)
	}

	tables, _, err := s.liveTables(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	var chosen []domain.Resource
	seats := 0
	for _, tid := range req.TableIDs {
		k := slices.IndexFunc(tables, func(r domain.Resource) bool { return r.ID == tid })
		if k < 0 || slices.ContainsFunc(chosen, func(r domain.Resource) bool { return r.ID == tid }) {
			return nil, badRequest("TABLE_NOT_AVAILABLE")
		}
		t := tables[k]
		busy, err := s.kernel.BusyIntervals(ctx, b.ID, domain.OwnerResource, t.ID, now, end)
		if err != nil {
			return nil, err
		}
		for _, iv := range busy {
			if !iv.Start.After(now) {
				return nil, conflict("TABLE_NOT_AVAILABLE") // someone sits there now
			}
			if !b.WalkinBeforeBooking {
				return nil, conflict("TABLE_BOOKED_LATER")
			}
			end = minTime(end, iv.Start) // they must leave before the booking
		}
		chosen = append(chosen, t)
		seats += t.Seats + t.ExtraChairs
	}
	if seats < req.Guests {
		return nil, badRequest("TABLES_TOO_SMALL")
	}
	if end.Sub(now) < 15*time.Minute {
		return nil, conflict("TABLE_BOOKED_LATER")
	}

	rule, err := s.rules.Effective(ctx, b.ID, 0, 0)
	if err != nil {
		return nil, err
	}
	who := &bookingport.Booker{Staff: actor, Guest: &guest}
	bk, _, err := s.kernel.NewBooking(b, who, chosen[0].SportID, now, end, rule, req.Notes)
	if err != nil {
		return nil, err
	}
	bk.Status = domain.BookingConfirmed
	bk.Guests, bk.Currency, bk.ArrivedAt = req.Guests, b.Currency, &now
	bk.Zones = s.zonesOf(ctx, b.ID, chosen)
	for _, t := range chosen {
		tid := t.ID
		bk.Items = append(bk.Items, domain.BookingItem{BranchID: b.ID, Kind: domain.OwnerResource, ResourceID: &tid,
			Seats: t.Seats, StartsAt: now, EndsAt: end, Active: true})
	}
	if err := s.bookings.Create(ctx, bk); err != nil {
		return nil, err
	}
	return s.kernel.GetBooking(ctx, branchID, bk.ID)
}

// SettleTables runs every few minutes: a group not there by its arrive-by time becomes a no-show
// (its tables free at once); a group whose time is over (closing time) has its bill closed.
func (s *Service) SettleTables(ctx context.Context) error {
	now := s.now()
	list, err := s.bookings.TablesToSettle(ctx, now)
	if err != nil {
		return err
	}
	for i := range list {
		bk := &list[i]
		status := domain.BookingCompleted
		if bk.ArrivedAt == nil {
			status = domain.BookingNoShow
		}
		if err := s.bookings.Finish(ctx, bk, status, now); err != nil {
			log.Printf("⚠️  table booking %d: %v", bk.ID, err)
		}
	}
	return nil
}
