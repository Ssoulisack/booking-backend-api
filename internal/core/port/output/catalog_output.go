package output

import (
	"backend/internal/core/domain"
	"context"
	"io"
	"time"
)

type SportRepository = ScopedRepository[domain.Sport]
type ResourceRepository = ScopedRepository[domain.Resource]

// ClosureRepository also finds the closures (of any owner, including the venue) overlapping a period
type ClosureRepository interface {
	ScopedRepository[domain.Closure]
	Overlapping(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Closure, error)
}

// CoachRepository loads coaches with their sports
type CoachRepository interface {
	ScopedRepository[domain.Coach]
	// SetSports replaces the coach's sports; every sport must belong to the same branch
	SetSports(ctx context.Context, branchID, coachID uint, sportIDs []uint) error
}

type ClassSessionRepository interface {
	ScopedRepository[domain.ClassSession]
	CountStartingBetween(ctx context.Context, branchID uint, from, to time.Time) (int64, error)
	// ListFiltered lists sessions starting in the filter's range, soonest first
	ListFiltered(ctx context.Context, branchID uint, f ClassFilter, q ListQuery) ([]domain.ClassSession, int64, error)
	// CreateMany stores every session (and its court / coach hold) in one transaction: all or nothing
	CreateMany(ctx context.Context, sessions []domain.ClassSession) error
	// ListBetween lists sessions (active or not) overlapping [from, to)
	ListBetween(ctx context.Context, branchID uint, from, to time.Time) ([]domain.ClassSession, error)
	// ListOfTimetable lists a timetable's sessions starting at or after from, soonest first
	ListOfTimetable(ctx context.Context, branchID, timetableID uint, from time.Time) ([]domain.ClassSession, error)
}

// ClassTimetableRepository stores weekly class timetables
type ClassTimetableRepository interface {
	ScopedRepository[domain.ClassTimetable]
	// ListRunning lists every timetable of an open (not closed) branch, for the daily roll forward
	ListRunning(ctx context.Context) ([]domain.ClassTimetable, error)
}

// ClassFilter narrows a class list; zero values do not filter
type ClassFilter struct {
	From, To   *time.Time // sessions starting in [From, To)
	SportID    uint
	ActiveOnly bool
	Active     *bool
}

// OpeningHourRepository replaces the whole weekly schedule of an owner in one transaction
type OpeningHourRepository interface {
	ListByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) ([]domain.OpeningHour, error)
	ReplaceForOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint, hours []domain.OpeningHour) error
}

type PriceRepository interface {
	GetByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) (*domain.Price, error)
	Upsert(ctx context.Context, p *domain.Price) error
	DeleteByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) error
}

type RuleRepository interface {
	// GetByScope returns (nil, nil) when no rule exists for the scope
	GetByScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) (*domain.BookingRule, error)
	Upsert(ctx context.Context, r *domain.BookingRule) error
	List(ctx context.Context, branchID uint) ([]domain.BookingRule, error)
	DeleteByScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) error
}

// FileStorage stores uploaded files. The database keeps only the object key.
type FileStorage interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader) error
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

// MediaRepository tracks uploaded files per branch
type MediaRepository interface {
	Create(ctx context.Context, m *domain.Media) error
	// Get returns (nil, nil) when the key does not belong to the branch (0: the company)
	Get(ctx context.Context, branchID uint, key string) (*domain.Media, error)
	Delete(ctx context.Context, branchID uint, key string) error
	// List returns the files of the branch (0: the company), newest first
	List(ctx context.Context, branchID uint) ([]domain.Media, error)
	// Uses returns every record that shows one of the scope's files (a sport, a coach, the cover, the logo)
	Uses(ctx context.Context, branchID uint) ([]MediaUse, error)
}

// MediaUse says which record shows a file: Kind is sport, coach, cover or logo; Name the record's name
type MediaUse struct{ Key, Kind, Name string }
