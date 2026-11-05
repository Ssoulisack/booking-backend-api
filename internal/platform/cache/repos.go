package cache

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"time"
)

// The wrappers below put the cache in front of the repositories the booking pages read all the time. A wrapper
// answers a read from Redis when it can, and after every write it bumps the version of what the write can change.
// A cache that is off wraps nothing: the repository is returned as it is.

// ---------- the company and its branches ----------

type company struct {
	output.CompanyRepository
	c *Cache
}

// WrapCompany caches the company row (brand, defaults); saving it changes every branch's settings in force
func WrapCompany(r output.CompanyRepository, c *Cache) output.CompanyRepository {
	if !c.On() {
		return r
	}
	return &company{r, c}
}

func (w *company) Get(ctx context.Context) (*domain.Company, error) {
	return Remember(ctx, w.c, []string{ScopeCompany}, "company", func() (*domain.Company, error) { return w.CompanyRepository.Get(ctx) })
}

func (w *company) Update(ctx context.Context, m *domain.Company) error {
	err := w.CompanyRepository.Update(ctx, m)
	w.c.Bump(ctx, ScopeCompany, ScopeDirectory)
	return err
}

func (w *company) Create(ctx context.Context, m *domain.Company) error {
	err := w.CompanyRepository.Create(ctx, m)
	w.c.Bump(ctx, ScopeCompany, ScopeDirectory)
	return err
}

type branches struct {
	output.BranchRepository
	c *Cache
}

// WrapBranches caches a branch by id or slug (read at the start of nearly every public request)
func WrapBranches(r output.BranchRepository, c *Cache) output.BranchRepository {
	if !c.On() {
		return r
	}
	return &branches{r, c}
}

func (w *branches) GetByID(ctx context.Context, id uint) (*domain.Branch, error) {
	return Remember(ctx, w.c, []string{ScopeCompany, ScopeBranches}, Key("byid", id), func() (*domain.Branch, error) { return w.BranchRepository.GetByID(ctx, id) })
}

func (w *branches) GetBySlug(ctx context.Context, slug string) (*domain.Branch, error) {
	return Remember(ctx, w.c, []string{ScopeCompany, ScopeBranches}, Key("byslug", slug), func() (*domain.Branch, error) { return w.BranchRepository.GetBySlug(ctx, slug) })
}

func (w *branches) bump(ctx context.Context) { w.c.Bump(ctx, ScopeBranches, ScopeDirectory) }

func (w *branches) Create(ctx context.Context, b *domain.Branch) error {
	err := w.BranchRepository.Create(ctx, b)
	w.bump(ctx)
	return err
}

func (w *branches) Clone(ctx context.Context, sourceID uint, b *domain.Branch) error {
	err := w.BranchRepository.Clone(ctx, sourceID, b)
	w.bump(ctx)
	return err
}

func (w *branches) Update(ctx context.Context, b *domain.Branch) error {
	err := w.BranchRepository.Update(ctx, b)
	w.bump(ctx)
	return err
}

type directory struct {
	output.DirectoryRepository
	c *Cache
}

// WrapDirectory caches the public list of places and what it shows about each
func WrapDirectory(r output.DirectoryRepository, c *Cache) output.DirectoryRepository {
	if !c.On() {
		return r
	}
	return &directory{r, c}
}

var directoryScopes = []string{ScopeCompany, ScopeBranches, ScopeDirectory}

func (w *directory) Branches(ctx context.Context, q output.DirectoryQuery) ([]domain.Branch, int64, error) {
	type result struct {
		Items []domain.Branch
		Total int64
	}
	out, err := Remember(ctx, w.c, directoryScopes, Key("dirbranches", q), func() (result, error) {
		items, total, err := w.DirectoryRepository.Branches(ctx, q)
		return result{items, total}, err
	})
	return out.Items, out.Total, err
}

func (w *directory) ActiveSports(ctx context.Context, ids []uint) (map[uint][]domain.Sport, error) {
	return Remember(ctx, w.c, directoryScopes, Key("dirsports", ids), func() (map[uint][]domain.Sport, error) { return w.DirectoryRepository.ActiveSports(ctx, ids) })
}

func (w *directory) LowestCourtPrices(ctx context.Context, ids []uint) (map[uint]domain.Price, error) {
	return Remember(ctx, w.c, directoryScopes, Key("dirprices", ids), func() (map[uint]domain.Price, error) { return w.DirectoryRepository.LowestCourtPrices(ctx, ids) })
}

// ---------- what a branch owns ----------

type rules struct {
	output.RuleRepository
	c *Cache
}

// WrapRules caches booking rules (read on every booking and availability check)
func WrapRules(r output.RuleRepository, c *Cache) output.RuleRepository {
	if !c.On() {
		return r
	}
	return &rules{r, c}
}

func (w *rules) GetByScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) (*domain.BookingRule, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key("rule", branchID, scopeType, scopeID), func() (*domain.BookingRule, error) {
		return w.RuleRepository.GetByScope(ctx, branchID, scopeType, scopeID)
	})
}

func (w *rules) List(ctx context.Context, branchID uint) ([]domain.BookingRule, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key("rules", branchID), func() ([]domain.BookingRule, error) { return w.RuleRepository.List(ctx, branchID) })
}

func (w *rules) Upsert(ctx context.Context, r *domain.BookingRule) error {
	err := w.RuleRepository.Upsert(ctx, r)
	w.c.Bump(ctx, Branch(r.BranchID))
	return err
}

func (w *rules) DeleteByScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) error {
	err := w.RuleRepository.DeleteByScope(ctx, branchID, scopeType, scopeID)
	w.c.Bump(ctx, Branch(branchID))
	return err
}

type hours struct {
	output.OpeningHourRepository
	c *Cache
}

// WrapHours caches weekly opening hours
func WrapHours(r output.OpeningHourRepository, c *Cache) output.OpeningHourRepository {
	if !c.On() {
		return r
	}
	return &hours{r, c}
}

func (w *hours) ListByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) ([]domain.OpeningHour, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key("hours", branchID, ownerType, ownerID), func() ([]domain.OpeningHour, error) {
		return w.OpeningHourRepository.ListByOwner(ctx, branchID, ownerType, ownerID)
	})
}

func (w *hours) ReplaceForOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint, list []domain.OpeningHour) error {
	err := w.OpeningHourRepository.ReplaceForOwner(ctx, branchID, ownerType, ownerID, list)
	w.c.Bump(ctx, Branch(branchID))
	return err
}

type prices struct {
	output.PriceRepository
	c *Cache
}

// WrapPrices caches the price of a space or coach (a change also changes the public "from" prices)
func WrapPrices(r output.PriceRepository, c *Cache) output.PriceRepository {
	if !c.On() {
		return r
	}
	return &prices{r, c}
}

func (w *prices) GetByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) (*domain.Price, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key("price", branchID, ownerType, ownerID), func() (*domain.Price, error) {
		return w.PriceRepository.GetByOwner(ctx, branchID, ownerType, ownerID)
	})
}

func (w *prices) Upsert(ctx context.Context, p *domain.Price) error {
	err := w.PriceRepository.Upsert(ctx, p)
	w.c.Bump(ctx, Branch(p.BranchID), ScopeDirectory)
	return err
}

func (w *prices) DeleteByOwner(ctx context.Context, branchID uint, ownerType string, ownerID uint) error {
	err := w.PriceRepository.DeleteByOwner(ctx, branchID, ownerType, ownerID)
	w.c.Bump(ctx, Branch(branchID), ScopeDirectory)
	return err
}

// ---------- the generic branch-scoped lists (sports, spaces, floor objects, coaches, closures) ----------

// Scoped caches GetByID, List and Count of a branch-scoped repository; any write to it bumps its branch. extra are
// scopes a write also changes (the public list of places shows each branch's sports).
type Scoped[T any] struct {
	output.ScopedRepository[T]
	c        *Cache
	name     string
	branchOf func(*T) uint
	extra    []string
}

// WrapScoped puts the cache in front of a ScopedRepository. name tells the cached values of one kind from another.
func WrapScoped[T any](r output.ScopedRepository[T], c *Cache, name string, branchOf func(*T) uint, extra ...string) output.ScopedRepository[T] {
	if !c.On() {
		return r
	}
	return newScoped(r, c, name, branchOf, extra...)
}

func newScoped[T any](r output.ScopedRepository[T], c *Cache, name string, branchOf func(*T) uint, extra ...string) *Scoped[T] {
	return &Scoped[T]{r, c, name, branchOf, extra}
}

func (w *Scoped[T]) bump(ctx context.Context, branchID uint) {
	w.c.Bump(ctx, append([]string{Branch(branchID)}, w.extra...)...)
}

func (w *Scoped[T]) GetByID(ctx context.Context, branchID, id uint) (*T, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key(w.name+":get", branchID, id), func() (*T, error) { return w.ScopedRepository.GetByID(ctx, branchID, id) })
}

func (w *Scoped[T]) List(ctx context.Context, branchID uint, q output.ListQuery) ([]T, int64, error) {
	type result struct {
		Items []T
		Total int64
	}
	out, err := Remember(ctx, w.c, []string{Branch(branchID)}, Key(w.name+":list", branchID, q), func() (result, error) {
		items, total, err := w.ScopedRepository.List(ctx, branchID, q)
		return result{items, total}, err
	})
	return out.Items, out.Total, err
}

func (w *Scoped[T]) Count(ctx context.Context, branchID uint, filters map[string]any) (int64, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key(w.name+":count", branchID, filters), func() (int64, error) { return w.ScopedRepository.Count(ctx, branchID, filters) })
}

func (w *Scoped[T]) Create(ctx context.Context, m *T) error {
	err := w.ScopedRepository.Create(ctx, m)
	w.bump(ctx, w.branchOf(m))
	return err
}

func (w *Scoped[T]) Update(ctx context.Context, m *T) error {
	err := w.ScopedRepository.Update(ctx, m)
	w.bump(ctx, w.branchOf(m))
	return err
}

func (w *Scoped[T]) Delete(ctx context.Context, branchID, id uint) error {
	err := w.ScopedRepository.Delete(ctx, branchID, id)
	w.bump(ctx, branchID)
	return err
}

type coaches struct {
	*Scoped[domain.Coach]
	inner output.CoachRepository
}

// WrapCoaches caches coaches (with their sports); changing a coach's sports clears them
func WrapCoaches(r output.CoachRepository, c *Cache) output.CoachRepository {
	if !c.On() {
		return r
	}
	return &coaches{newScoped[domain.Coach](r, c, "coaches", func(m *domain.Coach) uint { return m.BranchID }), r}
}

func (w *coaches) SetSports(ctx context.Context, branchID, coachID uint, sportIDs []uint) error {
	err := w.inner.SetSports(ctx, branchID, coachID, sportIDs)
	w.bump(ctx, branchID)
	return err
}

type closures struct {
	*Scoped[domain.Closure]
	inner output.ClosureRepository
}

// WrapClosures caches the closed days; a closure overlapping a period is looked up by that period
func WrapClosures(r output.ClosureRepository, c *Cache) output.ClosureRepository {
	if !c.On() {
		return r
	}
	return &closures{newScoped[domain.Closure](r, c, "closures", func(m *domain.Closure) uint { return m.BranchID }), r}
}

func (w *closures) Overlapping(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Closure, error) {
	return Remember(ctx, w.c, []string{Branch(branchID)}, Key("closures:over", branchID, from.UnixNano(), to.UnixNano()), func() ([]domain.Closure, error) {
		return w.inner.Overlapping(ctx, branchID, from, to)
	})
}
