// Package cache puts the platform cache in front of the events repository. What is cached is the fixed part of an
// event: its row, the event running on a day, the upcoming ones, its prices, areas and lineup, and the branch's
// payment info. Never cached: the booking counts, and which tables are sold (those come from bookings).
package cache

import (
	"backend/internal/modules/events/domain"
	"backend/internal/modules/events/port"
	platform "backend/internal/platform/cache"
	"context"
)

type repo struct {
	port.Repository
	c *platform.Cache
}

// Wrap caches the events repository; with the cache off it is returned as it is
func Wrap(r port.Repository, c *platform.Cache) port.Repository {
	if !c.On() {
		return r
	}
	return &repo{r, c}
}

func (w *repo) branchScope(id uint) []string { return []string{platform.Branch(id)} }

func (w *repo) Get(ctx context.Context, branchID, id uint) (*domain.Event, error) {
	return platform.Remember(ctx, w.c, w.branchScope(branchID), platform.Key("event", branchID, id), func() (*domain.Event, error) { return w.Repository.Get(ctx, branchID, id) })
}

func (w *repo) On(ctx context.Context, branchID uint, day string) (*domain.Event, error) {
	return platform.Remember(ctx, w.c, w.branchScope(branchID), platform.Key("eventon", branchID, day), func() (*domain.Event, error) { return w.Repository.On(ctx, branchID, day) })
}

func (w *repo) OnDay(ctx context.Context, branchID uint, day string) ([]domain.Event, error) {
	return platform.Remember(ctx, w.c, w.branchScope(branchID), platform.Key("eventsonday", branchID, day), func() ([]domain.Event, error) { return w.Repository.OnDay(ctx, branchID, day) })
}

func (w *repo) Upcoming(ctx context.Context, branchID uint, today string) ([]domain.Event, error) {
	return platform.Remember(ctx, w.c, w.branchScope(branchID), platform.Key("eventsupcoming", branchID, today), func() ([]domain.Event, error) { return w.Repository.Upcoming(ctx, branchID, today) })
}

func (w *repo) PayInfo(ctx context.Context, branchID uint) (*domain.PayInfo, error) {
	return platform.Remember(ctx, w.c, w.branchScope(branchID), platform.Key("eventpay", branchID), func() (*domain.PayInfo, error) { return w.Repository.PayInfo(ctx, branchID) })
}

func (w *repo) Prices(ctx context.Context, eventID uint) ([]domain.Price, error) {
	return platform.Remember(ctx, w.c, []string{platform.Event(eventID)}, platform.Key("eventprices", eventID), func() ([]domain.Price, error) { return w.Repository.Prices(ctx, eventID) })
}

func (w *repo) AreaIDs(ctx context.Context, eventID uint) ([]uint, error) {
	return platform.Remember(ctx, w.c, []string{platform.Event(eventID)}, platform.Key("eventareas", eventID), func() ([]uint, error) { return w.Repository.AreaIDs(ctx, eventID) })
}

func (w *repo) Lineup(ctx context.Context, eventID uint) ([]domain.LineupRow, error) {
	return platform.Remember(ctx, w.c, []string{platform.Event(eventID)}, platform.Key("eventlineup", eventID), func() ([]domain.LineupRow, error) { return w.Repository.Lineup(ctx, eventID) })
}

// ---- writes: whatever they change is made unreadable ----

func (w *repo) Create(ctx context.Context, e *domain.Event) error {
	err := w.Repository.Create(ctx, e)
	w.c.Bump(ctx, platform.Branch(e.BranchID))
	return err
}

func (w *repo) Update(ctx context.Context, e *domain.Event) error {
	err := w.Repository.Update(ctx, e)
	w.c.Bump(ctx, platform.Branch(e.BranchID), platform.Event(e.ID))
	return err
}

func (w *repo) Delete(ctx context.Context, branchID, id uint) error {
	err := w.Repository.Delete(ctx, branchID, id)
	w.c.Bump(ctx, platform.Branch(branchID), platform.Event(id))
	return err
}

func (w *repo) ReplacePrices(ctx context.Context, eventID uint, prices []domain.Price) error {
	err := w.Repository.ReplacePrices(ctx, eventID, prices)
	w.c.Bump(ctx, platform.Event(eventID))
	return err
}

func (w *repo) ReplaceAreas(ctx context.Context, eventID uint, ids []uint) error {
	err := w.Repository.ReplaceAreas(ctx, eventID, ids)
	w.c.Bump(ctx, platform.Event(eventID))
	return err
}

func (w *repo) ReplaceLineup(ctx context.Context, eventID uint, rows []domain.LineupRow) error {
	err := w.Repository.ReplaceLineup(ctx, eventID, rows)
	w.c.Bump(ctx, platform.Event(eventID))
	return err
}

func (w *repo) SavePayInfo(ctx context.Context, p *domain.PayInfo) error {
	err := w.Repository.SavePayInfo(ctx, p)
	w.c.Bump(ctx, platform.Branch(p.BranchID))
	return err
}
