// Package cache puts the platform cache in front of the one payments read that never changes by itself: a branch's
// payment settings. Payments themselves (their state) are never cached.
package cache

import (
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/port"
	platform "backend/internal/platform/cache"
	"context"
)

type repo struct {
	port.Repository
	c *platform.Cache
}

// Wrap caches the payment settings; with the cache off the repository is returned as it is
func Wrap(r port.Repository, c *platform.Cache) port.Repository {
	if !c.On() {
		return r
	}
	return &repo{r, c}
}

func (w *repo) Setting(ctx context.Context, branchID uint) (*domain.Setting, error) {
	return platform.Remember(ctx, w.c, []string{platform.Branch(branchID)}, platform.Key("paysetting", branchID), func() (*domain.Setting, error) { return w.Repository.Setting(ctx, branchID) })
}

func (w *repo) SaveSetting(ctx context.Context, st *domain.Setting) error {
	err := w.Repository.SaveSetting(ctx, st)
	w.c.Bump(ctx, platform.Branch(st.BranchID))
	return err
}
