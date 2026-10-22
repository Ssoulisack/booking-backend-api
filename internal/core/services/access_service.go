package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"net/http"
)

type AccessServiceImpl struct {
	users       output.UserRepository
	memberships output.MembershipRepository
	branches    output.BranchRepository
}

func NewAccessService(
	users output.UserRepository,
	memberships output.MembershipRepository,
	branches output.BranchRepository,
) input.AccessService {
	return &AccessServiceImpl{users: users, memberships: memberships, branches: branches}
}

func (s *AccessServiceImpl) Authenticate(ctx context.Context, userID uint, issuedAt int64) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if isNotFound(err) {
			return nil, response.NewError(http.StatusUnauthorized, response.ErrMsgUnauthorized)
		}
		return nil, err
	}
	if err := checkUsable(user, issuedAt); err != nil {
		return nil, err
	}
	return user, nil
}

// BranchRole is checked on every request, so a change of role or branches, switching someone
// off or closing a branch takes effect at once. The owner and admins act at every branch (also a
// closed one, to reopen it). Others need an active membership; unknown ids look like "not found".
func (s *AccessServiceImpl) BranchRole(ctx context.Context, user *domain.User, branchID uint, minRole string) (string, error) {
	hidden := notFound("BRANCH_NOT_FOUND")
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		if isNotFound(err) {
			return "", hidden
		}
		return "", err
	}
	if user.RunsCompany() {
		return user.CompanyRole, nil
	}
	m, err := s.memberships.Get(ctx, branchID, user.ID)
	if err != nil {
		return "", err
	}
	switch {
	case m == nil:
		return "", hidden
	case b.Closed:
		return "", forbidden("BRANCH_CLOSED")
	case !m.Active:
		return "", forbidden("MEMBERSHIP_DISABLED")
	case domain.MemberRank(m.Role) < domain.MemberRank(minRole):
		return "", forbidden("ROLE_NOT_ALLOWED")
	}
	return m.Role, nil
}
