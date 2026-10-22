package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"context"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"backend/pkg/response"
)

// ---------- weekly class timetables ----------
//
// A timetable is a class that repeats every week. Its classes are made automatically for the next
// domain.ClassWindowDays days and the window rolls forward every hour (RollTimetables), so the
// owner never has to set the class up again. Closed days are skipped; a day whose space or coach is
// already booked is skipped too. A closed branch is left alone until it reopens.

// timetablePlan is a checked timetable request
type timetablePlan struct {
	weekdays         map[time.Weekday]bool
	startMin, endMin int
}

func toTimetableDTO(m domain.ClassTimetable) dto.TimetableDTO {
	return dto.TimetableDTO{
		ID: m.ID, SportID: m.SportID, CoachID: m.CoachID, ResourceID: m.ResourceID, Title: m.Title,
		StartTime: m.StartTime, EndTime: m.EndTime, Weekdays: parseWeekdays(m.Weekdays),
		StartsOn: m.StartsOn, EndsOn: m.EndsOn, Capacity: m.Capacity, Price: m.Price, Currency: m.Currency,
		Active: m.Active, FilledUntil: m.FilledUntil,
	}
}

func parseWeekdays(s string) []int {
	out := []int{}
	for _, part := range strings.Split(s, ",") {
		if d, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// checkTimetable validates a request like one class (sport, space, coach, seats, price) plus the
// weekly pattern and the dates, and tidies it (title, currency, sorted weekdays)
func (s *ScheduleServiceImpl) checkTimetable(ctx context.Context, branchID uint, req *dto.TimetableRequest) (*timetablePlan, error) {
	loc, err := s.location(ctx, branchID)
	if err != nil {
		return nil, err
	}
	startMin, err := parseClock(req.StartTime, false)
	if err != nil {
		return nil, err
	}
	endMin, err := parseClock(req.EndTime, true)
	if err != nil {
		return nil, err
	}
	if endMin <= startMin {
		return nil, badRequest("END_TIME_MUST_BE_AFTER_START_TIME")
	}
	plan := &timetablePlan{weekdays: map[time.Weekday]bool{}, startMin: startMin, endMin: endMin}
	for _, d := range req.Weekdays {
		if d < 0 || d > 6 {
			return nil, badRequest("INVALID_WEEKDAY")
		}
		plan.weekdays[time.Weekday(d)] = true
	}
	if len(plan.weekdays) == 0 {
		return nil, badRequest("WEEKDAYS_REQUIRED")
	}
	first, err := parseDay(req.StartsOn, loc)
	if err != nil {
		return nil, err
	}
	req.EndsOn = strings.TrimSpace(req.EndsOn)
	if req.EndsOn != "" {
		last, err := parseDay(req.EndsOn, loc)
		if err != nil {
			return nil, err
		}
		if last.Before(first) {
			return nil, badRequest("TO_BEFORE_FROM")
		}
	}
	sample := dto.ClassSessionRequest{
		SportID: req.SportID, CoachID: req.CoachID, ResourceID: req.ResourceID, Title: req.Title,
		StartsAt: first.Add(time.Duration(startMin) * time.Minute), EndsAt: first.Add(time.Duration(endMin) * time.Minute),
		Capacity: req.Capacity, Price: req.Price, Currency: req.Currency, Active: req.Active,
	}
	if err := s.validateClassSession(ctx, branchID, &sample); err != nil {
		return nil, err
	}
	req.Title, req.Currency = sample.Title, sample.Currency
	return plan, nil
}

func weekdayList(plan *timetablePlan) string {
	days := make([]int, 0, len(plan.weekdays))
	for d := range plan.weekdays {
		days = append(days, int(d))
	}
	sort.Ints(days)
	parts := make([]string, len(days))
	for i, d := range days {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, ",")
}

func applyTimetable(m *domain.ClassTimetable, req dto.TimetableRequest, plan *timetablePlan) {
	m.SportID, m.CoachID, m.ResourceID, m.Title = req.SportID, req.CoachID, req.ResourceID, req.Title
	m.StartTime, m.EndTime, m.Weekdays = req.StartTime, req.EndTime, weekdayList(plan)
	m.StartsOn, m.EndsOn = req.StartsOn, req.EndsOn
	m.Capacity, m.Price, m.Currency, m.Active = req.Capacity, req.Price, req.Currency, isActive(req.Active)
}

func (s *ScheduleServiceImpl) ListTimetables(ctx context.Context, branchID uint) ([]dto.TimetableDTO, error) {
	items, _, err := s.timetables.List(ctx, branchID, output.ListQuery{}) // a branch has a handful
	if err != nil {
		return nil, err
	}
	return mapSlice(items, toTimetableDTO), nil
}

// CreateTimetable saves the timetable and makes its classes for the next weeks straight away
func (s *ScheduleServiceImpl) CreateTimetable(ctx context.Context, branchID uint, req dto.TimetableRequest) (*dto.TimetableResultDTO, error) {
	plan, err := s.checkTimetable(ctx, branchID, &req)
	if err != nil {
		return nil, err
	}
	m := domain.ClassTimetable{BranchID: branchID}
	applyTimetable(&m, req, plan)
	if err := s.timetables.Create(ctx, &m); err != nil {
		return nil, err
	}
	return s.fillTimetable(ctx, &m)
}

// UpdateTimetable changes the timetable. Upcoming classes without bookings are made again from the
// new details; classes that already have bookings keep theirs, so no customer is surprised.
func (s *ScheduleServiceImpl) UpdateTimetable(ctx context.Context, branchID, id uint, req dto.TimetableRequest) (*dto.TimetableResultDTO, error) {
	m, err := s.timetables.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, orNotFound(err, "TIMETABLE_NOT_FOUND")
	}
	plan, err := s.checkTimetable(ctx, branchID, &req)
	if err != nil {
		return nil, err
	}
	_, kept, err := s.clearUpcoming(ctx, m)
	if err != nil {
		return nil, err
	}
	applyTimetable(m, req, plan)
	m.FilledUntil = ""
	if err := s.timetables.Update(ctx, m); err != nil {
		return nil, err
	}
	out, err := s.fillTimetable(ctx, m)
	if err != nil {
		return nil, err
	}
	out.KeptBooked = kept
	return out, nil
}

// DeleteTimetable stops the timetable: no more classes are made, upcoming classes without bookings
// are removed, and booked ones stay as ordinary classes
func (s *ScheduleServiceImpl) DeleteTimetable(ctx context.Context, branchID, id uint) (*dto.TimetableStopDTO, error) {
	m, err := s.timetables.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, orNotFound(err, "TIMETABLE_NOT_FOUND")
	}
	removed, kept, err := s.clearUpcoming(ctx, m)
	if err != nil {
		return nil, err
	}
	if err := s.timetables.Delete(ctx, branchID, id); err != nil {
		return nil, err
	}
	return &dto.TimetableStopDTO{Removed: removed, KeptBooked: kept}, nil
}

// clearUpcoming removes the timetable's classes from now on that have no bookings
func (s *ScheduleServiceImpl) clearUpcoming(ctx context.Context, m *domain.ClassTimetable) (removed, kept int, err error) {
	upcoming, err := s.classes.ListOfTimetable(ctx, m.BranchID, m.ID, s.clock())
	if err != nil {
		return 0, 0, err
	}
	return s.removeUnbooked(ctx, m.BranchID, upcoming)
}

// removeUnbooked deletes the classes nobody has booked and counts the ones that stay
func (s *ScheduleServiceImpl) removeUnbooked(ctx context.Context, branchID uint, classes []domain.ClassSession) (removed, kept int, err error) {
	if len(classes) == 0 {
		return 0, 0, nil
	}
	taken, err := s.bookings.SeatsTaken(ctx, branchID, mapSlice(classes, func(c domain.ClassSession) uint { return c.ID }))
	if err != nil {
		return 0, 0, err
	}
	for _, c := range classes {
		if taken[c.ID] > 0 {
			kept++
			continue
		}
		if err := s.classes.Delete(ctx, branchID, c.ID); err != nil && !isNotFound(err) {
			return removed, kept, err
		}
		removed++
	}
	return removed, kept, nil
}

// fillTimetable makes the timetable's missing classes from its start (or today) up to today +
// ClassWindowDays (or its end), and remembers how far it got
func (s *ScheduleServiceImpl) fillTimetable(ctx context.Context, m *domain.ClassTimetable) (*dto.TimetableResultDTO, error) {
	out := &dto.TimetableResultDTO{Skipped: []dto.SkippedDate{}}
	loc, err := s.location(ctx, m.BranchID)
	if err != nil {
		return nil, err
	}
	startMin, _ := parseClock(m.StartTime, false)
	endMin, _ := parseClock(m.EndTime, true)
	weekdays := map[time.Weekday]bool{}
	for _, d := range parseWeekdays(m.Weekdays) {
		weekdays[time.Weekday(d)] = true
	}

	today := startOfDay(s.clock(), loc)
	from, _ := parseDay(m.StartsOn, loc)
	if from.Before(today) {
		from = today
	}
	if m.FilledUntil != "" {
		if done, err := parseDay(m.FilledUntil, loc); err == nil && !done.Before(from) {
			from = done.AddDate(0, 0, 1)
		}
	}
	until := today.AddDate(0, 0, domain.ClassWindowDays)
	if m.EndsOn != "" {
		if end, err := parseDay(m.EndsOn, loc); err == nil && end.Before(until) {
			until = end
		}
	}

	if !from.After(until) {
		closures, err := s.closures.Overlapping(ctx, m.BranchID, from, until.AddDate(0, 0, 1))
		if err != nil {
			return nil, err
		}
		// days that already have a class from this timetable (kept because booked)
		existing, err := s.classes.ListOfTimetable(ctx, m.BranchID, m.ID, from)
		if err != nil {
			return nil, err
		}
		has := map[string]bool{}
		for _, c := range existing {
			has[c.StartsAt.In(loc).Format("2006-01-02")] = true
		}
		now := s.clock()
		for day := from; !day.After(until); day = day.AddDate(0, 0, 1) {
			date := day.Format("2006-01-02")
			start, end := day.Add(time.Duration(startMin)*time.Minute), day.Add(time.Duration(endMin)*time.Minute)
			if !weekdays[day.Weekday()] || has[date] || !start.After(now) {
				continue
			}
			if closedFor(closures, m.ResourceID, m.CoachID, start, end) {
				out.Skipped = append(out.Skipped, dto.SkippedDate{Date: date, Reason: "closed"})
				continue
			}
			timetableID := m.ID
			c := domain.ClassSession{
				BranchID: m.BranchID, TimetableID: &timetableID, SportID: m.SportID, CoachID: m.CoachID,
				ResourceID: m.ResourceID, Title: m.Title, StartsAt: start, EndsAt: end, Capacity: m.Capacity,
				Price: m.Price, Currency: m.Currency, Active: m.Active,
			}
			if err := s.classes.Create(ctx, &c); err != nil {
				if isConflict(err) { // the space or coach is already booked then
					out.Skipped = append(out.Skipped, dto.SkippedDate{Date: date, Reason: "taken"})
					continue
				}
				return nil, err
			}
			out.Created++
		}
	}
	if done := until.Format("2006-01-02"); m.FilledUntil != done {
		m.FilledUntil = done
		if err := s.timetables.Update(ctx, m); err != nil {
			return nil, err
		}
	}
	out.Timetable = toTimetableDTO(*m)
	return out, nil
}

// closedFor reports whether a closed time covers the venue, the space or the coach then
func closedFor(closures []domain.Closure, resourceID, coachID *uint, start, end time.Time) bool {
	for _, c := range closures {
		applies := c.OwnerType == domain.OwnerVenue ||
			(c.OwnerType == domain.OwnerResource && resourceID != nil && c.OwnerID == *resourceID) ||
			(c.OwnerType == domain.OwnerCoach && coachID != nil && c.OwnerID == *coachID)
		if applies && c.StartsAt.Before(end) && start.Before(c.EndsAt) {
			return true
		}
	}
	return false
}

func isConflict(err error) bool {
	var appErr response.AppError
	return errors.As(err, &appErr) && appErr.Status == http.StatusConflict
}

// RollTimetables keeps every running timetable's classes made ClassWindowDays ahead. It runs on
// start and every hour; branches that are closed are skipped until they reopen.
func (s *ScheduleServiceImpl) RollTimetables(ctx context.Context) error {
	running, err := s.timetables.ListRunning(ctx)
	if err != nil {
		return err
	}
	for i := range running {
		if _, err := s.fillTimetable(ctx, &running[i]); err != nil {
			log.Printf("⚠️  timetable %d (%s): %v", running[i].ID, running[i].Title, err)
		}
	}
	return nil
}

// clearClosedClasses runs after a closed time is added: timetable classes in it without bookings
// are removed; booked ones are returned so staff can contact the customers
func (s *ScheduleServiceImpl) clearClosedClasses(ctx context.Context, c domain.Closure) (removed int, kept []dto.ClassSessionDTO, err error) {
	inside, err := s.classes.ListBetween(ctx, c.BranchID, c.StartsAt, c.EndsAt)
	if err != nil {
		return 0, nil, err
	}
	var affected []domain.ClassSession
	for _, cl := range inside {
		if cl.TimetableID != nil && closedFor([]domain.Closure{c}, cl.ResourceID, cl.CoachID, cl.StartsAt, cl.EndsAt) {
			affected = append(affected, cl)
		}
	}
	if len(affected) == 0 {
		return 0, []dto.ClassSessionDTO{}, nil
	}
	taken, err := s.bookings.SeatsTaken(ctx, c.BranchID, mapSlice(affected, func(cl domain.ClassSession) uint { return cl.ID }))
	if err != nil {
		return 0, nil, err
	}
	kept = []dto.ClassSessionDTO{}
	for _, cl := range affected {
		if taken[cl.ID] > 0 {
			kept = append(kept, toClassSessionDTO(cl))
			continue
		}
		if err := s.classes.Delete(ctx, c.BranchID, cl.ID); err != nil && !isNotFound(err) {
			return removed, kept, err
		}
		removed++
	}
	return removed, kept, nil
}
