package service

import (
	"backend/internal/core/domain"
	"backend/internal/platform/kit"
	"time"
)

func badRequest(code string) error { return kit.BadRequest(code) }

func locationOf(b *domain.Branch) *time.Location                  { return kit.LocationOf(b) }
func parseDay(date string, loc *time.Location) (time.Time, error) { return kit.ParseDay(date, loc) }
func minTime(a, b time.Time) time.Time                            { return kit.MinTime(a, b) }
func bookerName(b domain.Booking) string                          { return kit.BookerName(b) }

func openIntervalsOn(rows []domain.OpeningHour, day time.Time) []domain.Interval {
	return kit.OpenIntervalsOn(rows, day)
}
