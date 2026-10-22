package servicetest

import (
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/services"
	"context"
	"testing"
)

const court = 1 // resource 1 belongs to branch 7

func newScheduleService(hours *fakeHours) input.ScheduleService {
	resources := fakeResources{owner: map[uint]uint{court: branch}, sportOf: map[uint]uint{court: 1}}
	return services.NewScheduleService(hours, nil, nil, nil, nil, resources, nil, nil, nil, nil)
}

func hours(weekday int, open, close string) dto.OpeningHourDTO {
	return dto.OpeningHourDTO{Weekday: weekday, Open: open, Close: close}
}

func setHours(svc input.ScheduleService, in []dto.OpeningHourDTO) ([]dto.OpeningHourDTO, error) {
	return svc.SetOpeningHours(context.Background(), branch, "resource", court, dto.OpeningHoursRequest{Hours: in})
}

func TestOpeningHoursValid(t *testing.T) {
	cases := map[string][]dto.OpeningHourDTO{
		"full week":                  {hours(1, "08:00", "22:00"), hours(2, "08:00", "22:00")},
		"split day":                  {hours(1, "08:00", "12:00"), hours(1, "13:00", "22:00")},
		"touching intervals":         {hours(1, "08:00", "12:00"), hours(1, "12:00", "22:00")},
		"closes at midnight":         {hours(5, "10:00", "24:00")},
		"unsorted input":             {hours(1, "13:00", "22:00"), hours(1, "08:00", "12:00")},
		"never open (empty)":         {},
		"sunday is 0, saturday is 6": {hours(0, "09:00", "18:00"), hours(6, "09:00", "18:00")},
		"same hours on two days":     {hours(1, "08:00", "22:00"), hours(2, "08:00", "22:00")},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := setHours(newScheduleService(newFakeHours()), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(in) {
				t.Fatalf("saved %d intervals, want %d", len(got), len(in))
			}
		})
	}
}

func TestOpeningHoursInvalid(t *testing.T) {
	cases := map[string]struct {
		in   []dto.OpeningHourDTO
		code string
	}{
		"close before open":    {[]dto.OpeningHourDTO{hours(1, "22:00", "08:00")}, "CLOSE_MUST_BE_AFTER_OPEN"},
		"close equals open":    {[]dto.OpeningHourDTO{hours(1, "08:00", "08:00")}, "CLOSE_MUST_BE_AFTER_OPEN"},
		"overlap":              {[]dto.OpeningHourDTO{hours(1, "08:00", "14:00"), hours(1, "13:00", "22:00")}, "OPENING_HOURS_OVERLAP"},
		"weekday too big":      {[]dto.OpeningHourDTO{hours(7, "08:00", "22:00")}, "INVALID_WEEKDAY"},
		"negative weekday":     {[]dto.OpeningHourDTO{hours(-1, "08:00", "22:00")}, "INVALID_WEEKDAY"},
		"bad format":           {[]dto.OpeningHourDTO{hours(1, "8:00", "22:00")}, "INVALID_TIME_FORMAT"},
		"minutes out of range": {[]dto.OpeningHourDTO{hours(1, "08:60", "22:00")}, "INVALID_TIME_FORMAT"},
		"hour out of range":    {[]dto.OpeningHourDTO{hours(1, "08:00", "25:00")}, "INVALID_TIME_FORMAT"},
		"24:00 cannot open":    {[]dto.OpeningHourDTO{hours(1, "24:00", "24:00")}, "INVALID_TIME_FORMAT"},
		"text":                 {[]dto.OpeningHourDTO{hours(1, "morning", "evening")}, "INVALID_TIME_FORMAT"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := setHours(newScheduleService(newFakeHours()), tc.in)
			if err == nil || err.Error() != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
		})
	}
}

func TestRejectedScheduleKeepsTheOldOne(t *testing.T) {
	store := newFakeHours()
	svc := newScheduleService(store)
	if _, err := setHours(svc, []dto.OpeningHourDTO{hours(1, "08:00", "22:00")}); err != nil {
		t.Fatal(err)
	}
	if _, err := setHours(svc, []dto.OpeningHourDTO{hours(1, "22:00", "08:00")}); err == nil {
		t.Fatal("expected rejection")
	}
	got, err := svc.GetOpeningHours(context.Background(), branch, "resource", court)
	if err != nil || len(got) != 1 || got[0].Open != "08:00" {
		t.Fatalf("old schedule lost: %v %v", got, err)
	}
}

func TestOpeningHoursOfAnotherBranch(t *testing.T) {
	svc := newScheduleService(newFakeHours())
	_, err := svc.SetOpeningHours(context.Background(), 8, "resource", court, dto.OpeningHoursRequest{})
	if err == nil || err.Error() != "OWNER_NOT_FOUND" {
		t.Fatalf("got %v, want OWNER_NOT_FOUND", err)
	}
}

func TestOpeningHoursMidnightClose(t *testing.T) {
	got, err := setHours(newScheduleService(newFakeHours()), []dto.OpeningHourDTO{hours(1, "08:00", "24:00")})
	if err != nil || len(got) != 1 || got[0].Close != "24:00" {
		t.Fatalf("unexpected: %v %v", got, err)
	}
}
