package services

import (
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	slugCleaner     = regexp.MustCompile(`[^a-z0-9]+`)
	slugPattern     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
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
		Page:    page,
		Limit:   limit,
		Keyword: strings.TrimSpace(q.Keyword),
		Filters: map[string]any{},
	}, page, limit
}

// mapSlice converts each item; the result is never nil so it encodes as [] in JSON
func mapSlice[M, D any](items []M, convert func(M) D) []D {
	out := make([]D, 0, len(items))
	for _, item := range items {
		out = append(out, convert(item))
	}
	return out
}

// pageOf maps a repository page to a PagedResult
func pageOf[M, D any](items []M, total int64, page, limit int, toDTO func(M) D) *dto.PagedResult[D] {
	return &dto.PagedResult[D]{Items: mapSlice(items, toDTO), Meta: newMeta(total, page, limit)}
}

// orBadRequest turns a not-found error from a referenced entity into a 400 with code
// orNotFound turns a missing record into a 404 with a clear code
func orNotFound(err error, code string) error {
	if isNotFound(err) {
		return notFound(code)
	}
	return err
}

// validEmail accepts a plain address such as "info@club.la"
func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func orBadRequest(err error, code string) error {
	if err != nil && isNotFound(err) {
		return response.ErrorBadRequest(code)
	}
	return err
}

func isActive(flag *bool) bool { return flag == nil || *flag }

func validCurrency(c string) bool { return currencyPattern.MatchString(c) }

// slugify makes a URL-safe slug; fallback is used for names with no ASCII letters (Lao, Thai)
func slugify(name, fallback string) string {
	slug := strings.Trim(slugCleaner.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = fallback + "-" + uuid.NewString()[:8]
	}
	return slug
}

// reservedSlugs would clash with other public addresses (/public/branches is the list of places)
var reservedSlugs = map[string]bool{"branches": true}

func validSlug(slug string) bool {
	return len(slug) >= 3 && len(slug) <= 60 && slugPattern.MatchString(slug) && !reservedSlugs[slug]
}

func conflict(code string) error { return response.NewError(http.StatusConflict, code) }

func badRequest(code string) error { return response.ErrorBadRequest(code) }

func forbidden(code string) error { return response.NewError(http.StatusForbidden, code) }

func notFound(code string) error { return response.NewError(http.StatusNotFound, code) }

// newToken returns a random URL-safe token and the hash to store. Only the hash is kept,
// so a database leak does not expose working invite or reset links.
func newToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
