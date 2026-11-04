package kit

import (
	"backend/internal/core/domain"
	"strconv"
	"strings"
	"time"
)

// LocationOf is the branch's time zone (UTC when it is not a known name)
func LocationOf(b *domain.Branch) *time.Location {
	loc, err := time.LoadLocation(b.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// StartOfDay is midnight of t's day in loc
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// ParseDay reads YYYY-MM-DD as midnight in loc
func ParseDay(date string, loc *time.Location) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, BadRequest("INVALID_DATE")
	}
	return day, nil
}

// MinTime is the earlier of two times
func MinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// ParseClock returns minutes since midnight for "HH:MM". "24:00" is accepted as a closing time.
func ParseClock(s string, allowMidnightEnd bool) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, BadRequest("INVALID_TIME_FORMAT")
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || m < 0 || m > 59 || h < 0 {
		return 0, BadRequest("INVALID_TIME_FORMAT")
	}
	if h == 24 && m == 0 && allowMidnightEnd {
		return 24 * 60, nil
	}
	if h > 23 {
		return 0, BadRequest("INVALID_TIME_FORMAT")
	}
	return h*60 + m, nil
}

// OpenIntervalsOn turns weekly hours into concrete intervals on day (midnight in the branch timezone)
func OpenIntervalsOn(rows []domain.OpeningHour, day time.Time) []domain.Interval {
	var out []domain.Interval
	for _, h := range rows {
		if h.Weekday != int(day.Weekday()) {
			continue
		}
		open, err1 := ParseClock(h.OpenTime, false)
		closeAt, err2 := ParseClock(h.CloseTime, true)
		if err1 != nil || err2 != nil {
			continue // stored hours were validated when saved
		}
		out = append(out, domain.Interval{
			Start: day.Add(time.Duration(open) * time.Minute),
			End:   day.Add(time.Duration(closeAt) * time.Minute),
		})
	}
	return out
}

// BookerName is who a booking is for, as staff see it
func BookerName(b domain.Booking) string {
	if b.Customer != nil {
		return b.Customer.Name
	}
	return b.GuestName
}
