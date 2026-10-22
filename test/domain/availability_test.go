// Package domaintest tests the pure booking math (free slots, prices) from the outside.
package domaintest

import (
	"backend/internal/core/domain"
	"fmt"
	"slices"
	"testing"
	"time"
)

var vientiane, _ = time.LoadLocation("Asia/Vientiane")

// day 2026-11-02 in Vientiane; at(9,30) is 09:30 that day
func at(h, m int) time.Time { return time.Date(2026, 11, 2, h, m, 0, 0, vientiane) }

func iv(h1, m1, h2, m2 int) domain.Interval {
	return domain.Interval{Start: at(h1, m1), End: at(h2, m2)}
}

// the default rule: 60..180 minutes in 30-minute steps, a start every 30 minutes
var rule = domain.EffectiveRule{MinDurationMin: 60, MaxDurationMin: 180, DurationStepMin: 30, StartStepMin: 30}

// wide window: no notice, no far limit
var anyTime = domain.SlotWindow{Earliest: at(0, 0), Latest: at(0, 0).AddDate(0, 0, 1)}

func starts(slots []domain.Slot) []string {
	var out []string
	for _, s := range slots {
		out = append(out, s.StartsAt.Format("15:04"))
	}
	return out
}

func durationsAt(slots []domain.Slot, hhmm string) []int {
	for _, s := range slots {
		if s.StartsAt.Format("15:04") == hhmm {
			return s.Durations
		}
	}
	return nil
}

func TestFreeSlotsOpenDay(t *testing.T) {
	slots := domain.FreeSlots([]domain.Interval{iv(8, 0, 12, 0)}, nil, rule, anyTime)
	want := []string{"08:00", "08:30", "09:00", "09:30", "10:00", "10:30", "11:00"}
	if got := starts(slots); !slices.Equal(got, want) {
		t.Fatalf("starts = %v, want %v", got, want)
	}
	if d := durationsAt(slots, "08:00"); !slices.Equal(d, []int{60, 90, 120, 150, 180}) {
		t.Fatalf("08:00 durations = %v", d)
	}
	if d := durationsAt(slots, "10:30"); !slices.Equal(d, []int{60, 90}) { // must end by 12:00
		t.Fatalf("10:30 durations = %v", d)
	}
}

// The phase 3 "done when" case: after 08:00-09:30 is booked, 09:30 is the next free start
func TestBookingMakesNextStartAfterIt(t *testing.T) {
	busy := []domain.Interval{iv(8, 0, 9, 30)}
	slots := domain.FreeSlots([]domain.Interval{iv(8, 0, 22, 0)}, busy, rule, anyTime)
	if got := starts(slots); got[0] != "09:30" {
		t.Fatalf("first free start = %s, want 09:30 (all: %v)", got[0], got)
	}
}

func TestBusyTimeLimitsDurations(t *testing.T) {
	busy := []domain.Interval{iv(10, 0, 11, 0)}
	slots := domain.FreeSlots([]domain.Interval{iv(8, 0, 12, 0)}, busy, rule, anyTime)
	if d := durationsAt(slots, "08:00"); !slices.Equal(d, []int{60, 90, 120}) { // stops at 10:00
		t.Fatalf("08:00 durations = %v", d)
	}
	if d := durationsAt(slots, "09:30"); d != nil { // only 30 minutes before the busy hour
		t.Fatalf("09:30 must not be offered, got %v", d)
	}
	for _, s := range []string{"10:00", "10:30"} {
		if d := durationsAt(slots, s); d != nil {
			t.Fatalf("%s is busy, got %v", s, d)
		}
	}
	if d := durationsAt(slots, "11:00"); !slices.Equal(d, []int{60}) {
		t.Fatalf("11:00 durations = %v", d)
	}
}

func TestSportRulesShapeDurations(t *testing.T) {
	open := []domain.Interval{iv(8, 0, 22, 0)}
	cases := map[string]struct {
		rule domain.EffectiveRule
		want []int
	}{
		"boxing: fixed 60":       {domain.EffectiveRule{MinDurationMin: 60, MaxDurationMin: 60, DurationStepMin: 30, StartStepMin: 30}, []int{60}},
		"soccer: up to 90":       {domain.EffectiveRule{MinDurationMin: 60, MaxDurationMin: 90, DurationStepMin: 30, StartStepMin: 30}, []int{60, 90}},
		"tennis: branch max 240": {domain.EffectiveRule{MinDurationMin: 60, MaxDurationMin: 240, DurationStepMin: 30, StartStepMin: 30}, []int{60, 90, 120, 150, 180, 210, 240}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if d := durationsAt(domain.FreeSlots(open, nil, c.rule, anyTime), "08:00"); !slices.Equal(d, c.want) {
				t.Fatalf("durations = %v, want %v", d, c.want)
			}
		})
	}
}

func TestStartsFollowStartStepFromOpening(t *testing.T) {
	hourly := rule
	hourly.StartStepMin = 60
	slots := domain.FreeSlots([]domain.Interval{iv(8, 30, 12, 30)}, nil, hourly, anyTime)
	if got := starts(slots); !slices.Equal(got, []string{"08:30", "09:30", "10:30", "11:30"}) {
		t.Fatalf("starts = %v", got)
	}
}

func TestNoticeAndDaysAheadWindow(t *testing.T) {
	open := []domain.Interval{iv(8, 0, 12, 0)}
	window := domain.SlotWindow{Earliest: at(9, 15), Latest: at(0, 0).AddDate(0, 0, 1)}
	if got := starts(domain.FreeSlots(open, nil, rule, window)); got[0] != "09:30" {
		t.Fatalf("first start before the notice: %v", got)
	}
	tooFar := domain.SlotWindow{Earliest: at(0, 0), Latest: at(0, 0)} // the day is beyond the allowed range
	if got := domain.FreeSlots(open, nil, rule, tooFar); len(got) != 0 {
		t.Fatalf("no start may be offered past the days-ahead limit, got %v", starts(got))
	}
}

func TestMidnightCloseAndSplitHours(t *testing.T) {
	open := []domain.Interval{iv(8, 0, 10, 0), {Start: at(22, 0), End: at(0, 0).AddDate(0, 0, 1)}}
	got := starts(domain.FreeSlots(open, nil, rule, anyTime))
	want := []string{"08:00", "08:30", "09:00", "22:00", "22:30", "23:00"}
	if !slices.Equal(got, want) {
		t.Fatalf("starts = %v, want %v", got, want)
	}
}

func TestCoachAndCourtMustBothBeOpen(t *testing.T) {
	court := []domain.Interval{iv(8, 0, 22, 0)}
	coach := []domain.Interval{iv(9, 0, 12, 0), iv(14, 0, 17, 0)}
	both := domain.Intersect(court, coach)
	if len(both) != 2 || !both[0].Start.Equal(at(9, 0)) || !both[1].End.Equal(at(17, 0)) {
		t.Fatalf("intersection = %v", both)
	}
}

func TestSlotAllows(t *testing.T) {
	slots := domain.FreeSlots([]domain.Interval{iv(8, 0, 12, 0)}, nil, rule, anyTime)
	cases := []struct {
		start time.Time
		dur   int
		want  bool
	}{
		{at(8, 0), 90, true},
		{at(8, 0), 100, false}, // not a whole step
		{at(8, 15), 60, false}, // not on the start grid
		{at(11, 0), 90, false}, // would run past closing
		{at(8, 0), 240, false}, // longer than the rule allows
	}
	for _, c := range cases {
		if got := domain.SlotAllows(slots, c.start, c.dur); got != c.want {
			t.Errorf("SlotAllows(%s, %d) = %v, want %v", c.start.Format("15:04"), c.dur, got, c.want)
		}
	}
}

func TestPriceFor(t *testing.T) {
	cases := []struct {
		perHour int64
		minutes int
		want    int64
	}{
		{250, 60, 250},
		{250, 90, 375},       // 1h30 = 1.5 x hourly
		{200, 240, 800},      // 4 hours
		{100, 50, 83},        // 83.33 rounds down
		{100, 45, 75},        // exact
		{1, 30, 1},           // 0.5 rounds half up
		{150000, 90, 225000}, // LAK-sized prices
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d per hour, %d min", c.perHour, c.minutes), func(t *testing.T) {
			if got := domain.PriceFor(c.perHour, c.minutes); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}
