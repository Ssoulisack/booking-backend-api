package kit

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	slugCleaner     = regexp.MustCompile(`[^a-z0-9]+`)
)

// IsActive reads an optional "active" flag: not sent means on
func IsActive(flag *bool) bool { return flag == nil || *flag }

// ValidCurrency accepts a 3-letter upper-case code such as LAK
func ValidCurrency(c string) bool { return currencyPattern.MatchString(c) }

// Slugify makes a URL-safe slug; fallback is used for names with no ASCII letters (Lao, Thai)
func Slugify(name, fallback string) string {
	slug := strings.Trim(slugCleaner.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = fallback + "-" + uuid.NewString()[:8]
	}
	return slug
}

func SportRefs(sports []domain.Sport) []dto.SportRefDTO {
	return MapSlice(sports, func(sp domain.Sport) dto.SportRefDTO { return dto.SportRefDTO{ID: sp.ID, Name: sp.Name} })
}

// Teaches says whether the coach teaches the sport
func Teaches(c domain.Coach, sportID uint) bool {
	for _, sp := range c.Sports {
		if sp.ID == sportID {
			return true
		}
	}
	return false
}

func ClassSessionDTO(m domain.ClassSession) dto.ClassSessionDTO {
	return dto.ClassSessionDTO{
		ID: m.ID, SportID: m.SportID, CoachID: m.CoachID, ResourceID: m.ResourceID,
		Title: m.Title, StartsAt: m.StartsAt, EndsAt: m.EndsAt, Capacity: m.Capacity,
		Price: m.Price, Currency: m.Currency, Active: m.Active, TimetableID: m.TimetableID,
	}
}

// ClassRange reads ?date= (one day) or ?from=&to= (both days included) in the branch timezone.
// Nothing given means no limit.
func ClassRange(q dto.ListQuery, loc *time.Location) (from, to *time.Time, err error) {
	if q.Date != "" {
		day, err := ParseDay(q.Date, loc)
		if err != nil {
			return nil, nil, err
		}
		end := day.AddDate(0, 0, 1)
		return &day, &end, nil
	}
	if q.From != "" {
		f, err := ParseDay(q.From, loc)
		if err != nil {
			return nil, nil, err
		}
		from = &f
	}
	if q.To != "" {
		t, err := ParseDay(q.To, loc)
		if err != nil {
			return nil, nil, err
		}
		end := t.AddDate(0, 0, 1)
		to = &end
	}
	if from != nil && to != nil && !to.After(*from) {
		return nil, nil, BadRequest("TO_BEFORE_FROM")
	}
	return from, to, nil
}

// ClassFilter turns list query parameters into a repository filter
func ClassFilter(q dto.ListQuery, loc *time.Location) (output.ClassFilter, error) {
	from, to, err := ClassRange(q, loc)
	if err != nil {
		return output.ClassFilter{}, err
	}
	return output.ClassFilter{From: from, To: to, SportID: q.SportID, Active: q.Active}, nil
}
