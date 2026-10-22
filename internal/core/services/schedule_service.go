package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ScheduleServiceImpl struct {
	hours     output.OpeningHourRepository
	closures  output.ClosureRepository
	prices    output.PriceRepository
	classes   output.ClassSessionRepository
	sports    output.SportRepository
	resources output.ResourceRepository
	coaches   output.CoachRepository
	bookings  output.BookingRepository
	branches  output.BranchRepository
}

func NewScheduleService(
	hours output.OpeningHourRepository,
	closures output.ClosureRepository,
	prices output.PriceRepository,
	classes output.ClassSessionRepository,
	sports output.SportRepository,
	resources output.ResourceRepository,
	coaches output.CoachRepository,
	bookings output.BookingRepository,
	branches output.BranchRepository,
) input.ScheduleService {
	return &ScheduleServiceImpl{
		hours: hours, closures: closures, prices: prices, classes: classes,
		sports: sports, resources: resources, coaches: coaches, bookings: bookings, branches: branches,
	}
}

// location is the time zone in force at the branch (its own, else the company's)
func (s *ScheduleServiceImpl) location(ctx context.Context, branchID uint) (*time.Location, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return time.LoadLocation(b.Timezone)
}

// requireOwner checks that the resource or coach exists in this branch.
// allowVenue also accepts the whole venue (id 0).
func (s *ScheduleServiceImpl) requireOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint, allowVenue bool) error {
	switch ownerType {
	case domain.OwnerResource:
		_, err := s.resources.GetByID(ctx, branchID, ownerID)
		return orBadRequest(err, "OWNER_NOT_FOUND")
	case domain.OwnerCoach:
		_, err := s.coaches.GetByID(ctx, branchID, ownerID)
		return orBadRequest(err, "OWNER_NOT_FOUND")
	case domain.OwnerVenue:
		if allowVenue && ownerID == 0 {
			return nil
		}
	}
	return badRequest("INVALID_OWNER")
}

// ---------- opening hours ----------

// parseClock returns minutes since midnight for "HH:MM". "24:00" is accepted as a closing time.
func parseClock(s string, allowMidnightEnd bool) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, badRequest("INVALID_TIME_FORMAT")
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || m < 0 || m > 59 || h < 0 {
		return 0, badRequest("INVALID_TIME_FORMAT")
	}
	if h == 24 && m == 0 && allowMidnightEnd {
		return 24 * 60, nil
	}
	if h > 23 {
		return 0, badRequest("INVALID_TIME_FORMAT")
	}
	return h*60 + m, nil
}

// buildOpeningHours validates a weekly schedule and converts it to domain rows
func buildOpeningHours(branchID uint, ownerType string, ownerID uint, in []dto.OpeningHourDTO) ([]domain.OpeningHour, error) {
	type span struct{ open, close int }
	byDay := map[int][]span{}
	rows := make([]domain.OpeningHour, 0, len(in))

	for _, h := range in {
		if h.Weekday < 0 || h.Weekday > 6 {
			return nil, badRequest("INVALID_WEEKDAY")
		}
		open, err := parseClock(h.Open, false)
		if err != nil {
			return nil, err
		}
		closeAt, err := parseClock(h.Close, true)
		if err != nil {
			return nil, err
		}
		if closeAt <= open {
			return nil, badRequest("CLOSE_MUST_BE_AFTER_OPEN")
		}
		byDay[h.Weekday] = append(byDay[h.Weekday], span{open, closeAt})
		rows = append(rows, domain.OpeningHour{
			BranchID: branchID, OwnerType: ownerType, OwnerID: ownerID, Weekday: h.Weekday,
			OpenTime: clockString(open), CloseTime: clockString(closeAt),
		})
	}
	for _, spans := range byDay {
		sort.Slice(spans, func(i, j int) bool { return spans[i].open < spans[j].open })
		for i := 1; i < len(spans); i++ {
			if spans[i].open < spans[i-1].close {
				return nil, badRequest("OPENING_HOURS_OVERLAP")
			}
		}
	}
	return rows, nil
}

func clockString(minutes int) string { return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60) }

func toOpeningHourDTO(h domain.OpeningHour) dto.OpeningHourDTO {
	return dto.OpeningHourDTO{Weekday: h.Weekday, Open: h.OpenTime, Close: h.CloseTime}
}

func (s *ScheduleServiceImpl) GetOpeningHours(ctx context.Context, branchID uint, ownerType string, ownerID uint) ([]dto.OpeningHourDTO, error) {
	if err := s.requireOwner(ctx, branchID, ownerType, ownerID, false); err != nil {
		return nil, err
	}
	rows, err := s.hours.ListByOwner(ctx, branchID, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, toOpeningHourDTO), nil
}

// SetOpeningHours replaces the whole weekly schedule; an empty list means "never open"
func (s *ScheduleServiceImpl) SetOpeningHours(ctx context.Context, branchID uint, ownerType string, ownerID uint, req dto.OpeningHoursRequest) ([]dto.OpeningHourDTO, error) {
	if err := s.requireOwner(ctx, branchID, ownerType, ownerID, false); err != nil {
		return nil, err
	}
	rows, err := buildOpeningHours(branchID, ownerType, ownerID, req.Hours)
	if err != nil {
		return nil, err
	}
	if err := s.hours.ReplaceForOwner(ctx, branchID, ownerType, ownerID, rows); err != nil {
		return nil, err
	}
	return s.GetOpeningHours(ctx, branchID, ownerType, ownerID)
}

// ---------- closures ----------

func toClosureDTO(m domain.Closure) dto.ClosureDTO {
	return dto.ClosureDTO{
		ID: m.ID, OwnerType: m.OwnerType, OwnerID: m.OwnerID,
		StartsAt: m.StartsAt, EndsAt: m.EndsAt, Reason: m.Reason,
	}
}

func (s *ScheduleServiceImpl) validateClosure(ctx context.Context, branchID uint, req *dto.ClosureRequest) error {
	if req.StartsAt.IsZero() || req.EndsAt.IsZero() {
		return badRequest("STARTS_AT_AND_ENDS_AT_ARE_REQUIRED")
	}
	if !req.EndsAt.After(req.StartsAt) {
		return badRequest("ENDS_AT_MUST_BE_AFTER_STARTS_AT")
	}
	return s.requireOwner(ctx, branchID, req.OwnerType, req.OwnerID, true)
}

func (s *ScheduleServiceImpl) CreateClosure(ctx context.Context, branchID uint, req dto.ClosureRequest) (*dto.ClosureDTO, error) {
	if err := s.validateClosure(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m := domain.Closure{
		BranchID: branchID, OwnerType: req.OwnerType, OwnerID: req.OwnerID,
		StartsAt: req.StartsAt, EndsAt: req.EndsAt, Reason: req.Reason,
	}
	if err := s.closures.Create(ctx, &m); err != nil {
		return nil, err
	}
	d := toClosureDTO(m)
	return &d, nil
}

func (s *ScheduleServiceImpl) GetClosure(ctx context.Context, branchID, id uint) (*dto.ClosureDTO, error) {
	m, err := s.closures.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	d := toClosureDTO(*m)
	return &d, nil
}

func (s *ScheduleServiceImpl) ListClosures(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.ClosureDTO], error) {
	lq, page, limit := newListQuery(q)
	if q.OwnerType != "" {
		lq.Filters["owner_type"] = q.OwnerType
		lq.Filters["owner_id"] = q.OwnerID
	}
	items, total, err := s.closures.List(ctx, branchID, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, toClosureDTO), nil
}

func (s *ScheduleServiceImpl) UpdateClosure(ctx context.Context, branchID, id uint, req dto.ClosureRequest) (*dto.ClosureDTO, error) {
	if err := s.validateClosure(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m, err := s.closures.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	m.OwnerType, m.OwnerID, m.StartsAt, m.EndsAt, m.Reason = req.OwnerType, req.OwnerID, req.StartsAt, req.EndsAt, req.Reason
	if err := s.closures.Update(ctx, m); err != nil {
		return nil, err
	}
	d := toClosureDTO(*m)
	return &d, nil
}

func (s *ScheduleServiceImpl) DeleteClosure(ctx context.Context, branchID, id uint) error {
	return s.closures.Delete(ctx, branchID, id)
}

// ---------- prices ----------

func toPriceDTO(m domain.Price) dto.PriceDTO {
	return dto.PriceDTO{OwnerType: m.OwnerType, OwnerID: m.OwnerID, PricePerHour: m.PricePerHour, Currency: m.Currency}
}

func (s *ScheduleServiceImpl) GetPrice(ctx context.Context, branchID uint, ownerType string, ownerID uint) (*dto.PriceDTO, error) {
	if err := s.requireOwner(ctx, branchID, ownerType, ownerID, false); err != nil {
		return nil, err
	}
	m, err := s.prices.GetByOwner(ctx, branchID, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	d := toPriceDTO(*m)
	return &d, nil
}

func (s *ScheduleServiceImpl) SetPrice(ctx context.Context, branchID uint, ownerType string, ownerID uint, req dto.PriceRequest) (*dto.PriceDTO, error) {
	if err := s.requireOwner(ctx, branchID, ownerType, ownerID, false); err != nil {
		return nil, err
	}
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	if req.PricePerHour < 0 {
		return nil, badRequest("PRICE_MUST_NOT_BE_NEGATIVE")
	}
	if !validCurrency(req.Currency) {
		return nil, badRequest("INVALID_CURRENCY")
	}
	m := domain.Price{
		BranchID: branchID, OwnerType: ownerType, OwnerID: ownerID,
		PricePerHour: req.PricePerHour, Currency: req.Currency,
	}
	if err := s.prices.Upsert(ctx, &m); err != nil {
		return nil, err
	}
	d := toPriceDTO(m)
	return &d, nil
}

func (s *ScheduleServiceImpl) DeletePrice(ctx context.Context, branchID uint, ownerType string, ownerID uint) error {
	if err := s.requireOwner(ctx, branchID, ownerType, ownerID, false); err != nil {
		return err
	}
	return s.prices.DeleteByOwner(ctx, branchID, ownerType, ownerID)
}

// ---------- class sessions ----------

func toClassSessionDTO(m domain.ClassSession) dto.ClassSessionDTO {
	return dto.ClassSessionDTO{
		ID: m.ID, SportID: m.SportID, CoachID: m.CoachID, ResourceID: m.ResourceID,
		Title: m.Title, StartsAt: m.StartsAt, EndsAt: m.EndsAt, Capacity: m.Capacity,
		Price: m.Price, Currency: m.Currency, Active: m.Active,
	}
}

// validateClassSession checks the fields and that the coach and resource fit the sport.
// Clashes with other bookings on the same resource or coach are checked by the booking engine.
func (s *ScheduleServiceImpl) validateClassSession(ctx context.Context, branchID uint, req *dto.ClassSessionRequest) error {
	req.Title = strings.TrimSpace(req.Title)
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	switch {
	case req.Title == "":
		return badRequest("TITLE_IS_REQUIRED")
	case req.StartsAt.IsZero() || req.EndsAt.IsZero():
		return badRequest("STARTS_AT_AND_ENDS_AT_ARE_REQUIRED")
	case !req.EndsAt.After(req.StartsAt):
		return badRequest("ENDS_AT_MUST_BE_AFTER_STARTS_AT")
	case req.Capacity < 1:
		return badRequest("CAPACITY_MUST_BE_AT_LEAST_1")
	case req.Price < 0:
		return badRequest("PRICE_MUST_NOT_BE_NEGATIVE")
	case !validCurrency(req.Currency):
		return badRequest("INVALID_CURRENCY")
	}
	if _, err := s.sports.GetByID(ctx, branchID, req.SportID); err != nil {
		return orBadRequest(err, "SPORT_NOT_FOUND")
	}
	if req.ResourceID != nil {
		res, err := s.resources.GetByID(ctx, branchID, *req.ResourceID)
		if err != nil {
			return orBadRequest(err, "RESOURCE_NOT_FOUND")
		}
		if res.SportID != req.SportID {
			return badRequest("RESOURCE_SPORT_MISMATCH")
		}
	}
	if req.CoachID != nil {
		coach, err := s.coaches.GetByID(ctx, branchID, *req.CoachID)
		if err != nil {
			return orBadRequest(err, "COACH_NOT_FOUND")
		}
		teaches := false
		for _, sp := range coach.Sports {
			teaches = teaches || sp.ID == req.SportID
		}
		if !teaches {
			return badRequest("COACH_DOES_NOT_TEACH_SPORT")
		}
	}
	return nil
}

func (s *ScheduleServiceImpl) CreateClassSession(ctx context.Context, branchID uint, req dto.ClassSessionRequest) (*dto.ClassSessionDTO, error) {
	if err := s.validateClassSession(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m := domain.ClassSession{
		BranchID: branchID, SportID: req.SportID, CoachID: req.CoachID, ResourceID: req.ResourceID,
		Title: req.Title, StartsAt: req.StartsAt, EndsAt: req.EndsAt, Capacity: req.Capacity,
		Price: req.Price, Currency: req.Currency, Active: isActive(req.Active),
	}
	if err := s.classes.Create(ctx, &m); err != nil {
		return nil, err
	}
	d := toClassSessionDTO(m)
	return &d, nil
}

func (s *ScheduleServiceImpl) GetClassSession(ctx context.Context, branchID, id uint) (*dto.ClassSessionDTO, error) {
	m, err := s.classes.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	d := toClassSessionDTO(*m)
	return &d, nil
}

// ListClassSessions lists classes soonest first; ?date= or ?from=&to= limit the days, ?keyword= searches titles
func (s *ScheduleServiceImpl) ListClassSessions(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.ClassSessionDTO], error) {
	loc, err := s.location(ctx, branchID)
	if err != nil {
		return nil, err
	}
	f, err := classFilter(q, loc)
	if err != nil {
		return nil, err
	}
	lq, page, limit := newListQuery(q)
	items, total, err := s.classes.ListFiltered(ctx, branchID, f, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, toClassSessionDTO), nil
}

func (s *ScheduleServiceImpl) UpdateClassSession(ctx context.Context, branchID, id uint, req dto.ClassSessionRequest) (*dto.ClassSessionDTO, error) {
	if err := s.validateClassSession(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m, err := s.classes.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if err := s.protectBookedClass(ctx, branchID, m, req); err != nil {
		return nil, err
	}
	m.SportID, m.CoachID, m.ResourceID, m.Title = req.SportID, req.CoachID, req.ResourceID, req.Title
	m.StartsAt, m.EndsAt, m.Capacity = req.StartsAt, req.EndsAt, req.Capacity
	m.Price, m.Currency, m.Active = req.Price, req.Currency, isActive(req.Active)
	if err := s.classes.Update(ctx, m); err != nil {
		return nil, err
	}
	d := toClassSessionDTO(*m)
	return &d, nil
}

// DeleteClassSession refuses a class that customers have booked seats in
func (s *ScheduleServiceImpl) DeleteClassSession(ctx context.Context, branchID, id uint) error {
	taken, err := s.seatsTaken(ctx, branchID, id)
	if err != nil {
		return err
	}
	if taken > 0 {
		return conflict("CLASS_HAS_BOOKINGS")
	}
	return s.classes.Delete(ctx, branchID, id)
}

func (s *ScheduleServiceImpl) seatsTaken(ctx context.Context, branchID, classID uint) (int, error) {
	taken, err := s.bookings.SeatsTaken(ctx, branchID, []uint{classID})
	if err != nil {
		return 0, err
	}
	return taken[classID], nil
}

// protectBookedClass keeps a class that has bookings where its customers expect it: same time,
// same court and coach, still active, and with room for every seat already booked
func (s *ScheduleServiceImpl) protectBookedClass(ctx context.Context, branchID uint, m *domain.ClassSession, req dto.ClassSessionRequest) error {
	taken, err := s.seatsTaken(ctx, branchID, m.ID)
	if err != nil || taken == 0 {
		return err
	}
	samePlace := equalID(m.ResourceID, req.ResourceID) && equalID(m.CoachID, req.CoachID)
	sameTime := m.StartsAt.Equal(req.StartsAt) && m.EndsAt.Equal(req.EndsAt)
	if !samePlace || !sameTime || !isActive(req.Active) || req.SportID != m.SportID {
		return conflict("CLASS_HAS_BOOKINGS")
	}
	if req.Capacity < taken {
		return conflict("CAPACITY_BELOW_BOOKED_SEATS")
	}
	return nil
}

func equalID(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
