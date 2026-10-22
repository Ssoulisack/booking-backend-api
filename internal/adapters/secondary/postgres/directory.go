package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"

	"gorm.io/gorm"
)

type directoryRepository struct{ db *gorm.DB }

func NewDirectoryRepository(db *gorm.DB) output.DirectoryRepository {
	return &directoryRepository{db: db}
}

func (r *directoryRepository) Branches(ctx context.Context, q output.DirectoryQuery) ([]domain.Branch, int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.Branch{}).Where("published AND NOT closed")
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("(name ILIKE ? OR address ILIKE ?)", like, like)
	}
	if q.Province != "" {
		tx = tx.Where("province = ?", q.Province)
	}
	if q.SportType != "" {
		tx = tx.Where(`EXISTS (SELECT 1 FROM sports s WHERE s.branch_id = branches.id
			AND s.active AND s.deleted_at IS NULL AND s.type = ?)`, q.SportType)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var items []domain.Branch
	err := paginate(tx.Order("published_at DESC NULLS LAST, id DESC"), output.ListQuery{Page: q.Page, Limit: q.Limit}).Find(&items).Error
	return items, total, exception.HandleGormError(err)
}

func (r *directoryRepository) ActiveSports(ctx context.Context, branchIDs []uint) (map[uint][]domain.Sport, error) {
	out := map[uint][]domain.Sport{}
	if len(branchIDs) == 0 {
		return out, nil
	}
	var sports []domain.Sport
	err := r.db.WithContext(ctx).Where("branch_id IN ? AND active", branchIDs).Order("name ASC").Find(&sports).Error
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	for _, s := range sports {
		out[s.BranchID] = append(out[s.BranchID], s)
	}
	return out, nil
}

func (r *directoryRepository) LowestCourtPrices(ctx context.Context, branchIDs []uint) (map[uint]domain.Price, error) {
	out := map[uint]domain.Price{}
	if len(branchIDs) == 0 {
		return out, nil
	}
	var rows []domain.Price
	err := r.db.WithContext(ctx).
		Table("prices p").
		Select("DISTINCT ON (p.branch_id) p.branch_id, p.price_per_hour, p.currency").
		Joins("JOIN resources r ON r.id = p.owner_id AND r.deleted_at IS NULL AND r.active").
		Where("p.owner_type = ? AND p.deleted_at IS NULL AND p.branch_id IN ?", domain.OwnerResource, branchIDs).
		Order("p.branch_id, p.price_per_hour ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	for _, p := range rows {
		out[p.BranchID] = p
	}
	return out, nil
}
