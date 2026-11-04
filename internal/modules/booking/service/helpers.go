package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/platform/kit"
	"context"
	"strings"
	"time"
)

// The shared helpers live in platform/kit; these keep the short names this service already uses.

func conflict(code string) error   { return kit.Conflict(code) }
func badRequest(code string) error { return kit.BadRequest(code) }
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
func minTime(a, b time.Time) time.Time { return kit.MinTime(a, b) }
func openIntervalsOn(rows []domain.OpeningHour, day time.Time) []domain.Interval {
	return kit.OpenIntervalsOn(rows, day)
}
func bookerName(b domain.Booking) string { return kit.BookerName(b) }

// loadPublished finds a branch by its address and refuses one customers cannot see
func loadPublished(ctx context.Context, branches output.BranchRepository, slug string) (*domain.Branch, error) {
	b, err := branches.GetBySlug(ctx, slug)
	if err != nil {
		if isNotFound(err) {
			return nil, notFound("BRANCH_NOT_FOUND")
		}
		return nil, err
	}
	if !b.IsPublic() {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	return b, nil
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// normalizePage applies the default and maximum page size
func normalizePage(q dto.ListQuery) (page, limit int) {
	page, limit = q.Page, q.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	return page, limit
}

func newMeta(total int64, page, limit int) dto.Meta {
	totalPage := int((total + int64(limit) - 1) / int64(limit))
	return dto.Meta{Total: int(total), Page: page, Limit: limit, TotalPage: totalPage}
}

// newListQuery builds a repository query with the page already normalized
func newListQuery(q dto.ListQuery) (output.ListQuery, int, int) {
	page, limit := normalizePage(q)
	return output.ListQuery{
		Page: page, Limit: limit, Keyword: strings.TrimSpace(q.Keyword), Filters: map[string]any{},
	}, page, limit
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// pageOf maps a repository page to a PagedResult
func pageOf[M, D any](items []M, total int64, page, limit int, toDTO func(M) D) *dto.PagedResult[D] {
	return &dto.PagedResult[D]{Items: mapSlice(items, toDTO), Meta: newMeta(total, page, limit)}
}

func teaches(c domain.Coach, sportID uint) bool {
	for _, sp := range c.Sports {
		if sp.ID == sportID {
			return true
		}
	}
	return false
}
