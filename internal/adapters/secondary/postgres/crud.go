package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"net/http"

	"gorm.io/gorm"
)

// errNoScope is returned when a branch-owned query is attempted without a branch:
// failing closed is safer than returning every branch's rows
var errNoScope = response.NewError(http.StatusInternalServerError, "MISSING_BRANCH_SCOPE")

// scopedRepository is a generic gorm implementation of output.ScopedRepository for models
// with a branch_id column. searchColumn is matched (ILIKE) by ListQuery.Keyword.
type scopedRepository[T any] struct {
	db           *gorm.DB
	searchColumn string
	preloads     []string
}

// NewScopedRepository creates a PostgreSQL repository for a branch-owned gorm model
func NewScopedRepository[T any](db *gorm.DB, searchColumn string, preloads ...string) output.ScopedRepository[T] {
	return newScoped[T](db, searchColumn, preloads...)
}

func newScoped[T any](db *gorm.DB, searchColumn string, preloads ...string) *scopedRepository[T] {
	return &scopedRepository[T]{db: db, searchColumn: searchColumn, preloads: preloads}
}

// scope starts a query limited to one branch
func (r *scopedRepository[T]) scope(ctx context.Context, branchID uint) (*gorm.DB, error) {
	if branchID == 0 {
		return nil, errNoScope
	}
	return r.db.WithContext(ctx).Model(new(T)).Where("branch_id = ?", branchID), nil
}

func (r *scopedRepository[T]) withPreloads(tx *gorm.DB) *gorm.DB {
	for _, p := range r.preloads {
		tx = tx.Preload(p)
	}
	return tx
}

func (r *scopedRepository[T]) Create(ctx context.Context, m *T) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(m).Error)
}

func (r *scopedRepository[T]) GetByID(ctx context.Context, branchID, id uint) (*T, error) {
	tx, err := r.scope(ctx, branchID)
	if err != nil {
		return nil, err
	}
	var m T
	if err := r.withPreloads(tx).First(&m, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &m, nil
}

func applyFilters(tx *gorm.DB, filters map[string]any) *gorm.DB {
	for column, value := range filters {
		tx = tx.Where(column+" = ?", value)
	}
	return tx
}

func paginate(tx *gorm.DB, q output.ListQuery) *gorm.DB {
	if q.Limit > 0 {
		tx = tx.Offset((max(q.Page, 1) - 1) * q.Limit).Limit(q.Limit)
	}
	return tx
}

func (r *scopedRepository[T]) List(ctx context.Context, branchID uint, q output.ListQuery) ([]T, int64, error) {
	tx, err := r.scope(ctx, branchID)
	if err != nil {
		return nil, 0, err
	}
	if q.Keyword != "" && r.searchColumn != "" {
		tx = tx.Where(r.searchColumn+" ILIKE ?", "%"+q.Keyword+"%")
	}
	tx = applyFilters(tx, q.Filters)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var items []T
	if err := paginate(r.withPreloads(tx).Order("id ASC"), q).Find(&items).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	return items, total, nil
}

func (r *scopedRepository[T]) Count(ctx context.Context, branchID uint, filters map[string]any) (int64, error) {
	tx, err := r.scope(ctx, branchID)
	if err != nil {
		return 0, err
	}
	var total int64
	err = applyFilters(tx, filters).Count(&total).Error
	return total, exception.HandleGormError(err)
}

func (r *scopedRepository[T]) Update(ctx context.Context, m *T) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(m).Error)
}

func (r *scopedRepository[T]) Delete(ctx context.Context, branchID, id uint) error {
	tx, err := r.scope(ctx, branchID)
	if err != nil {
		return err
	}
	res := tx.Delete(new(T), id)
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected == 0 {
		return exception.HandleGormError(gorm.ErrRecordNotFound)
	}
	return nil
}
