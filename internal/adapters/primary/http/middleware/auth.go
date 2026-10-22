package middleware

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/input"
	"backend/pkg/jwt"
	"backend/pkg/response"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	localUser   = "user"
	localBranch = "branchID"
	localRole   = "memberRole"
)

// Auth checks who is calling and what they may do. The token only proves identity:
// account status and branch roles are read from the database on every request.
type Auth struct {
	access input.AccessService
}

func NewAuth(access input.AccessService) *Auth {
	return &Auth{access: access}
}

// authenticate returns (nil, nil) when no bearer token is sent
func (a *Auth) authenticate(c *fiber.Ctx) (*domain.User, error) {
	if !strings.HasPrefix(c.Get("Authorization"), "Bearer ") {
		return nil, nil
	}
	claims, err := jwt.GetOwnerAccessToken(c)
	if err != nil {
		return nil, response.NewError(http.StatusUnauthorized, response.ErrMsgInvalidAccessToken)
	}
	return a.access.Authenticate(c.UserContext(), uint(claims.ID), claims.IssuedAt)
}

// RequireUser lets only logged-in, enabled users through
func (a *Auth) RequireUser(c *fiber.Ctx) error {
	user, err := a.authenticate(c)
	if err != nil {
		return response.NewErrorResponses(c, err)
	}
	if user == nil {
		return response.NewErrorErrMsgUnauthorized(c)
	}
	c.Locals(localUser, user)
	return c.Next()
}

// OptionalUser loads the user when a token is sent; a bad token is still rejected
func (a *Auth) OptionalUser(c *fiber.Ctx) error {
	user, err := a.authenticate(c)
	if err != nil {
		return response.NewErrorResponses(c, err)
	}
	if user != nil {
		c.Locals(localUser, user)
	}
	return c.Next()
}

// RequireCompany admits the company owner and admins (company settings, branches, team, users).
// Must run after RequireUser.
func (a *Auth) RequireCompany(c *fiber.Ctx) error {
	user := CurrentUser(c)
	if user == nil || !user.RunsCompany() {
		return response.NewErrorResponses(c, response.NewError(fiber.StatusForbidden, "ROLE_NOT_ALLOWED"))
	}
	return c.Next()
}

// Branch admits the people who work at the :branchId branch with a role ranking at least
// minRole; the owner and admins act at every branch. Must run after RequireUser.
func (a *Auth) Branch(minRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("branchId")
		if err != nil || id < 1 {
			return response.NewErrorResponses(c, response.ErrorBadRequest("INVALID_ID"))
		}
		role, err := a.access.BranchRole(c.UserContext(), CurrentUser(c), uint(id), minRole)
		if err != nil {
			return response.NewErrorResponses(c, err)
		}
		c.Locals(localBranch, uint(id))
		c.Locals(localRole, role)
		return c.Next()
	}
}

// CurrentUser is the user loaded by RequireUser / OptionalUser, or nil
func CurrentUser(c *fiber.Ctx) *domain.User {
	user, _ := c.Locals(localUser).(*domain.User)
	return user
}

// BranchID is the branch admitted by Branch(), or 0
func BranchID(c *fiber.Ctx) uint {
	id, _ := c.Locals(localBranch).(uint)
	return id
}
