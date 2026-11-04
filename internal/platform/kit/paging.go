package kit

import (
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"strings"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// NormalizePage applies the default and maximum page size
func NormalizePage(q dto.ListQuery) (page, limit int) {
	page, limit = q.Page, q.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	return page, limit
}

// NewMeta is the paging block of a list answer
func NewMeta(total int64, page, limit int) dto.Meta {
	totalPage := int((total + int64(limit) - 1) / int64(limit))
	return dto.Meta{Total: int(total), Page: page, Limit: limit, TotalPage: totalPage}
}

// NewListQuery builds a repository query with the page already normalized
func NewListQuery(q dto.ListQuery) (output.ListQuery, int, int) {
	page, limit := NormalizePage(q)
	return output.ListQuery{
		Page:    page,
		Limit:   limit,
		Keyword: strings.TrimSpace(q.Keyword),
		Filters: map[string]any{},
	}, page, limit
}

// PageOf maps a repository page to a PagedResult
func PageOf[M, D any](items []M, total int64, page, limit int, toDTO func(M) D) *dto.PagedResult[D] {
	return &dto.PagedResult[D]{Items: MapSlice(items, toDTO), Meta: NewMeta(total, page, limit)}
}
