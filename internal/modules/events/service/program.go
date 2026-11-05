package service

import (
	"backend/internal/core/domain"
	coredto "backend/internal/core/dto"
	"backend/internal/core/port/output"
	evdomain "backend/internal/modules/events/domain"
	"backend/internal/modules/events/dto"
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// The event's time of day, the areas it is held in and its lineup (program).

const maxLineupRows = 40

var clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func validClock(s string) bool { return clockRe.MatchString(s) }

// extras are the parts of an event that live in their own tables; set* tells whether the request sent them
type extras struct {
	areas     []uint
	setAreas  bool
	lineup    []evdomain.LineupRow
	setLineup bool
}

func (s *Service) saveExtras(ctx context.Context, eventID uint, ex *extras) error {
	if ex == nil {
		return nil
	}
	if ex.setAreas {
		if err := s.Repo.ReplaceAreas(ctx, eventID, ex.areas); err != nil {
			return err
		}
	}
	if ex.setLineup {
		for i := range ex.lineup {
			ex.lineup[i].EventID = eventID
		}
		return s.Repo.ReplaceLineup(ctx, eventID, ex.lineup)
	}
	return nil
}

// checkAreas keeps the ids of areas this branch really has (no duplicates), sorted
func (s *Service) checkAreas(ctx context.Context, branchID uint, ids []uint) ([]uint, error) {
	out := []uint{}
	for _, id := range ids {
		if slices.Contains(out, id) {
			continue
		}
		if _, err := s.Sports.GetByID(ctx, branchID, id); err != nil {
			return nil, badRequest("INVALID_EVENT_AREA")
		}
		out = append(out, id)
	}
	slices.Sort(out)
	return out, nil
}

// nightMinutes puts a lineup time on one clock for a night that runs past midnight: 00:00–05:59 comes after 23:59
func nightMinutes(clock string) int {
	h, m := int(clock[0]-'0')*10+int(clock[1]-'0'), int(clock[3]-'0')*10+int(clock[4]-'0')
	if h < 6 {
		h += 24
	}
	return h*60 + m
}

// checkLineup validates the program and puts it in the order of the night: every-day rows first, then by date,
// each by start time (after-midnight rows last)
func checkLineup(in []dto.LineupItem, startsOn, endsOn string) ([]evdomain.LineupRow, error) {
	if len(in) > maxLineupRows {
		return nil, badRequest("TOO_MANY_LINEUP_ROWS")
	}
	rows := make([]evdomain.LineupRow, 0, len(in))
	for _, it := range in {
		title := strings.TrimSpace(it.Title)
		switch {
		case title == "" || utf8.RuneCountInString(title) > 120:
			return nil, badRequest("INVALID_LINEUP_TITLE")
		case !validClock(it.StartTime) || (it.EndTime != "" && !validClock(it.EndTime)) || it.StartTime == it.EndTime:
			return nil, badRequest("INVALID_LINEUP_TIME")
		case it.Day != "" && (!validDate(it.Day) || it.Day < startsOn || it.Day > endsOn):
			return nil, badRequest("INVALID_LINEUP_DAY")
		}
		rows = append(rows, evdomain.LineupRow{Day: it.Day, StartTime: it.StartTime, EndTime: it.EndTime, Title: title})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.Day == "") != (b.Day == "") {
			return a.Day == ""
		}
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		return nightMinutes(a.StartTime) < nightMinutes(b.StartTime)
	})
	for i := range rows {
		rows[i].Position = i
	}
	return rows, nil
}

func lineupItems(rows []evdomain.LineupRow) []dto.LineupItem {
	out := make([]dto.LineupItem, len(rows))
	for i, r := range rows {
		out[i] = dto.LineupItem{Day: r.Day, StartTime: r.StartTime, EndTime: r.EndTime, Title: r.Title}
	}
	return out
}

// areaRefs names the areas an event is held in (none: the whole branch)
func (s *Service) areaRefs(ctx context.Context, branchID uint, ids []uint) []coredto.EventAreaRef {
	out := []coredto.EventAreaRef{}
	for _, id := range ids {
		if a, err := s.Sports.GetByID(ctx, branchID, id); err == nil {
			out = append(out, coredto.EventAreaRef{ID: a.ID, Name: a.Name})
		}
	}
	return out
}

// tablesOf keeps the tables of the areas the event is held in (every table when it is held everywhere)
func tablesOf(tables []domain.Resource, areaIDs []uint) []domain.Resource {
	if len(areaIDs) == 0 {
		return tables
	}
	return slices.DeleteFunc(slices.Clone(tables), func(t domain.Resource) bool { return !slices.Contains(areaIDs, t.SportID) })
}

var _ = output.ListQuery{}

func ptr[T any](v T) *T { return &v }
