package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/platform/kit"
	"time"
)

// The shared helpers live in platform/kit (one implementation, also used by the modules); these keep
// the short names the services already use.

func conflict(code string) error   { return kit.Conflict(code) }
func badRequest(code string) error { return kit.BadRequest(code) }
func forbidden(code string) error  { return kit.Forbidden(code) }
func notFound(code string) error   { return kit.NotFound(code) }
func isNotFound(err error) bool    { return kit.IsNotFound(err) }

func orBadRequest(err error, code string) error { return kit.OrBadRequest(err, code) }

func mapSlice[M, D any](items []M, convert func(M) D) []D { return kit.MapSlice(items, convert) }

func newToken() (raw, hash string, err error) { return kit.NewToken() }
func hashToken(raw string) string             { return kit.HashToken(raw) }

func locationOf(b *domain.Branch) *time.Location { return kit.LocationOf(b) }
func startOfDay(t time.Time, loc *time.Location) time.Time {
	return kit.StartOfDay(t, loc)
}
func parseDay(date string, loc *time.Location) (time.Time, error) { return kit.ParseDay(date, loc) }
func parseClock(s string, allowMidnightEnd bool) (int, error) {
	return kit.ParseClock(s, allowMidnightEnd)
}

// endClock reads a closing or end time as minutes since midnight: "00:00" after a later start means midnight at the
// END of that day (24:00), which is what a time field gives for 12 am. "00:00" with a start at 00:00 stays 0, so it
// is refused like any end that is not after its start (a whole day is written "24:00").
func endClock(start, end int) int {
	if end == 0 && start > 0 {
		return 24 * 60
	}
	return end
}

func minTime(a, b time.Time) time.Time { return kit.MinTime(a, b) }
func openIntervalsOn(rows []domain.OpeningHour, day time.Time) []domain.Interval {
	return kit.OpenIntervalsOn(rows, day)
}
func bookerName(b domain.Booking) string { return kit.BookerName(b) }

// shared with the other modules: one implementation in platform/kit
func isActive(flag *bool) bool                     { return kit.IsActive(flag) }
func validCurrency(c string) bool                  { return kit.ValidCurrency(c) }
func slugify(name, fallback string) string         { return kit.Slugify(name, fallback) }
func sportRefs(s []domain.Sport) []dto.SportRefDTO { return kit.SportRefs(s) }
func teaches(c domain.Coach, sportID uint) bool    { return kit.Teaches(c, sportID) }
func toClassSessionDTO(m domain.ClassSession) dto.ClassSessionDTO {
	return kit.ClassSessionDTO(m)
}
func classFilter(q dto.ListQuery, loc *time.Location) (output.ClassFilter, error) {
	return kit.ClassFilter(q, loc)
}

// orNotFound turns a missing record into a 404 with a clear code
func orNotFound(err error, code string) error {
	if isNotFound(err) {
		return notFound(code)
	}
	return err
}

func newListQuery(q dto.ListQuery) (output.ListQuery, int, int) { return kit.NewListQuery(q) }
func pageOf[M, D any](items []M, total int64, page, limit int, toDTO func(M) D) *dto.PagedResult[D] {
	return kit.PageOf(items, total, page, limit, toDTO)
}
