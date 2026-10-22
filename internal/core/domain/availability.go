package domain

import (
	"sort"
	"time"
)

// Interval is a half-open time range [Start, End)
type Interval struct {
	Start time.Time
	End   time.Time
}

func (i Interval) overlaps(o Interval) bool { return i.Start.Before(o.End) && o.Start.Before(i.End) }

// Slot is a start time that can be booked, with every duration (minutes) allowed from it
type Slot struct {
	StartsAt  time.Time
	Durations []int
}

// SlotWindow limits which starts may be offered: not before Earliest (minimum notice)
// and not on or after Latest (end of the last bookable day)
type SlotWindow struct {
	Earliest time.Time
	Latest   time.Time
}

// FreeSlots lists bookable starts inside the open intervals. A start is offered every
// StartStepMin minutes from the beginning of its open interval; from it, durations run from
// MinDurationMin in DurationStepMin steps up to MaxDurationMin, as long as they end before the
// interval closes and before the next busy interval. This is the single definition of "free":
// availability pages and booking checks both use it.
func FreeSlots(open, busy []Interval, rule EffectiveRule, window SlotWindow) []Slot {
	if rule.MinDurationMin < 1 || rule.DurationStepMin < 1 || rule.StartStepMin < 1 {
		return nil
	}
	open = append([]Interval(nil), open...) // sort copies, not the caller's slices
	busy = append([]Interval(nil), busy...)
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })
	sort.Slice(open, func(i, j int) bool { return open[i].Start.Before(open[j].Start) })

	minDur := time.Duration(rule.MinDurationMin) * time.Minute
	stepDur := time.Duration(rule.DurationStepMin) * time.Minute
	startStep := time.Duration(rule.StartStepMin) * time.Minute

	var slots []Slot
	for _, o := range open {
		for start := o.Start; !start.Add(minDur).After(o.End); start = start.Add(startStep) {
			if start.Before(window.Earliest) || !start.Before(window.Latest) {
				continue
			}
			limit := o.End // the slot may run until the interval closes or the next busy time starts
			free := true
			for _, b := range busy {
				if !b.End.After(start) {
					continue
				}
				if !b.Start.After(start) { // busy covers the start itself
					free = false
				} else if b.Start.Before(limit) {
					limit = b.Start
				}
				break
			}
			if !free {
				continue
			}
			var durations []int
			for d := minDur; d <= time.Duration(rule.MaxDurationMin)*time.Minute && !start.Add(d).After(limit); d += stepDur {
				durations = append(durations, int(d/time.Minute))
			}
			if len(durations) > 0 {
				slots = append(slots, Slot{StartsAt: start, Durations: durations})
			}
		}
	}
	return slots
}

// SlotAllows tells whether slots offer the given start with the given duration
func SlotAllows(slots []Slot, start time.Time, durationMin int) bool {
	for _, s := range slots {
		if !s.StartsAt.Equal(start) {
			continue
		}
		for _, d := range s.Durations {
			if d == durationMin {
				return true
			}
		}
	}
	return false
}

// Intersect keeps only the times present in both lists of open intervals (a coach and a court
// booked together must both be open)
func Intersect(a, b []Interval) []Interval {
	var out []Interval
	for _, x := range a {
		for _, y := range b {
			start, end := x.Start, x.End
			if y.Start.After(start) {
				start = y.Start
			}
			if y.End.Before(end) {
				end = y.End
			}
			if start.Before(end) {
				out = append(out, Interval{Start: start, End: end})
			}
		}
	}
	return out
}

// AnyOverlap tells whether interval i overlaps any of list
func AnyOverlap(i Interval, list []Interval) bool {
	for _, o := range list {
		if i.overlaps(o) {
			return true
		}
	}
	return false
}

// PriceFor returns the price of minutes at an hourly price, rounded half up to a whole unit
// (e.g. 1h30 at 250/h = 375; 50 minutes at 100/h = 83)
func PriceFor(pricePerHour int64, minutes int) int64 {
	return (pricePerHour*int64(minutes) + 30) / 60
}
