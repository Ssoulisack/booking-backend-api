package output

import (
	"backend/internal/core/domain"
	"context"
)

// DirectoryQuery filters the public list of places; empty fields do not filter
type DirectoryQuery struct {
	Page      int
	Limit     int
	Keyword   string // name or address
	SportType string // has an active sport of this type
	Province  string
}

// DirectoryRepository reads the public list of places to play ("Find a place")
type DirectoryRepository interface {
	// Branches lists published, open branches, newest published first
	Branches(ctx context.Context, q DirectoryQuery) ([]domain.Branch, int64, error)
	// ActiveSports returns the active sports of each branch, by branch id
	ActiveSports(ctx context.Context, branchIDs []uint) (map[uint][]domain.Sport, error)
	// LowestCourtPrices returns the lowest hourly price of an active court per branch (none: absent)
	LowestCourtPrices(ctx context.Context, branchIDs []uint) (map[uint]domain.Price, error)
}
