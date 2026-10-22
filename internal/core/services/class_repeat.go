package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"context"
	"time"
)

const maxRepeatSessions = 200

// classRange reads ?date= (one day) or ?from=&to= (both days included) in the branch timezone.
// Nothing given means no limit.
func classRange(q dto.ListQuery, loc *time.Location) (from, to *time.Time, err error) {
	if q.Date != "" {
		day, err := parseDay(q.Date, loc)
		if err != nil {
			return nil, nil, err
		}
		end := day.AddDate(0, 0, 1)
		return &day, &end, nil
	}
	if q.From != "" {
		f, err := parseDay(q.From, loc)
		if err != nil {
			return nil, nil, err
		}
		from = &f
	}
	if q.To != "" {
		t, err := parseDay(q.To, loc)
		if err != nil {
			return nil, nil, err
		}
		end := t.AddDate(0, 0, 1)
		to = &end
	}
	if from != nil && to != nil && !to.After(*from) {
		return nil, nil, badRequest("TO_BEFORE_FROM")
	}
	return from, to, nil
}

// RepeatClassSessions creates a session on every chosen weekday between from and to. Days closed
// for the venue, the court or the coach are skipped and reported. If any session would clash with a
// booking or another class, nothing is created.
func (s *ScheduleServiceImpl) RepeatClassSessions(ctx context.Context, branchID uint, req dto.RepeatClassRequest) (*dto.RepeatClassResultDTO, error) {
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
	weekdays := map[time.Weekday]bool{}
	for _, d := range req.Weekdays {
		if d < 0 || d > 6 {
			return nil, badRequest("INVALID_WEEKDAY")
		}
		weekdays[time.Weekday(d)] = true
	}
	if len(weekdays) == 0 {
		return nil, badRequest("WEEKDAYS_REQUIRED")
	}
	first, err := parseDay(req.From, loc)
	if err != nil {
		return nil, err
	}
	last, err := parseDay(req.To, loc)
	if err != nil {
		return nil, err
	}
	if last.Before(first) {
		return nil, badRequest("TO_BEFORE_FROM")
	}
	if last.Sub(first) > maxReportDays*24*time.Hour {
		return nil, badRequest("PERIOD_TOO_LONG")
	}

	// the same checks as one class: title, capacity, price, currency, sport / court / coach fit
	sample := dto.ClassSessionRequest{
		SportID: req.SportID, CoachID: req.CoachID, ResourceID: req.ResourceID, Title: req.Title,
		StartsAt: first.Add(time.Duration(startMin) * time.Minute), EndsAt: first.Add(time.Duration(endMin) * time.Minute),
		Capacity: req.Capacity, Price: req.Price, Currency: req.Currency, Active: req.Active,
	}
	if err := s.validateClassSession(ctx, branchID, &sample); err != nil {
		return nil, err
	}

	closures, err := s.closures.Overlapping(ctx, branchID, first, last.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	closed := func(start, end time.Time) bool {
		for _, c := range closures {
			applies := c.OwnerType == domain.OwnerVenue ||
				(c.OwnerType == domain.OwnerResource && req.ResourceID != nil && c.OwnerID == *req.ResourceID) ||
				(c.OwnerType == domain.OwnerCoach && req.CoachID != nil && c.OwnerID == *req.CoachID)
			if applies && c.StartsAt.Before(end) && start.Before(c.EndsAt) {
				return true
			}
		}
		return false
	}

	out := &dto.RepeatClassResultDTO{Sessions: []dto.ClassSessionDTO{}, Skipped: []dto.SkippedDate{}}
	var sessions []domain.ClassSession
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		if !weekdays[day.Weekday()] {
			continue
		}
		start, end := day.Add(time.Duration(startMin)*time.Minute), day.Add(time.Duration(endMin)*time.Minute)
		if closed(start, end) {
			out.Skipped = append(out.Skipped, dto.SkippedDate{Date: day.Format("2006-01-02"), Reason: "closed"})
			continue
		}
		if err := s.refuseClash(ctx, branchID, req, start, end); err != nil {
			return nil, err
		}
		sessions = append(sessions, domain.ClassSession{
			BranchID: branchID, SportID: sample.SportID, CoachID: sample.CoachID, ResourceID: sample.ResourceID,
			Title: sample.Title, StartsAt: start, EndsAt: end, Capacity: sample.Capacity,
			Price: sample.Price, Currency: sample.Currency, Active: isActive(sample.Active),
		})
		if len(sessions) > maxRepeatSessions {
			return nil, badRequest("TOO_MANY_SESSIONS")
		}
	}
	if len(sessions) == 0 {
		return nil, badRequest("NO_SESSIONS_IN_RANGE")
	}
	// one transaction: a clash that slipped in meanwhile undoes every session (409)
	if err := s.classes.CreateMany(ctx, sessions); err != nil {
		return nil, err
	}
	out.Created = len(sessions)
	out.Sessions = mapSlice(sessions, toClassSessionDTO)
	return out, nil
}

// refuseClash names the first date whose court or coach is already taken
func (s *ScheduleServiceImpl) refuseClash(ctx context.Context, branchID uint, req dto.RepeatClassRequest, start, end time.Time) error {
	check := func(ownerType string, id *uint) error {
		if id == nil {
			return nil
		}
		items, err := s.bookings.ActiveItems(ctx, branchID, ownerType, *id, start, end)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			return conflict("SLOT_NOT_AVAILABLE_ON_" + start.Format("2006-01-02"))
		}
		return nil
	}
	if err := check(domain.OwnerResource, req.ResourceID); err != nil {
		return err
	}
	return check(domain.OwnerCoach, req.CoachID)
}

// classFilter turns list query parameters into a repository filter
func classFilter(q dto.ListQuery, loc *time.Location) (output.ClassFilter, error) {
	from, to, err := classRange(q, loc)
	if err != nil {
		return output.ClassFilter{}, err
	}
	return output.ClassFilter{From: from, To: to, SportID: q.SportID, Active: q.Active}, nil
}
