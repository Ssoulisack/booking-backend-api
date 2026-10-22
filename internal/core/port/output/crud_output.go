package output

import "context"

// ListQuery is a paginated list request. Filters are exact-match column filters
// chosen by the service, never taken straight from user input.
type ListQuery struct {
	Page    int
	Limit   int
	Keyword string
	Filters map[string]any
}

// ScopedRepository is the storage contract for entities that belong to one branch.
// Every read, list and delete is limited to branchID; branchID 0 is refused.
// Create and Update use the BranchID already set on the model.
type ScopedRepository[T any] interface {
	Create(ctx context.Context, m *T) error
	GetByID(ctx context.Context, branchID, id uint) (*T, error)
	List(ctx context.Context, branchID uint, q ListQuery) ([]T, int64, error)
	Count(ctx context.Context, branchID uint, filters map[string]any) (int64, error)
	Update(ctx context.Context, m *T) error
	Delete(ctx context.Context, branchID, id uint) error
}
