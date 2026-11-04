package httpx

import (
	"backend/internal/core/dto"
	"backend/internal/platform/middleware"
	"backend/pkg/response"
	"context"

	"github.com/gofiber/fiber/v2"
)

// ListQuery reads ?page, ?limit and ?keyword
func ListQuery(c *fiber.Ctx) (dto.ListQuery, error) {
	var q dto.ListQuery
	if err := c.QueryParser(&q); err != nil {
		return q, response.ErrorBadRequest("INVALID_QUERY")
	}
	return q, nil
}

// Paged answers a page of items, or the error
func Paged[Res any](c *fiber.Ctx, res *dto.PagedResult[Res], err error) error {
	if err != nil {
		return Fail(c, err)
	}
	return response.NewPaginatedResponse(c, res.Items, res.Meta)
}

// The Scoped helpers are for branch-scoped routes: the branch comes from middleware.Branch.

func ScopedGet[Res any](c *fiber.Ctx, fn func(context.Context, uint, uint) (*Res, error)) error {
	id, err := ID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), id)
	return OK(c, res, err)
}

func ScopedList[Res any](c *fiber.Ctx, fn func(context.Context, uint, dto.ListQuery) (*dto.PagedResult[Res], error)) error {
	q, err := ListQuery(c)
	if err != nil {
		return Fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), q)
	return Paged(c, res, err)
}

func ScopedUpdate[Req, Res any](c *fiber.Ctx, fn func(context.Context, uint, uint, Req) (*Res, error)) error {
	id, err := ID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	var req Req
	if err := Bind(c, &req); err != nil {
		return Fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), id, req)
	return OK(c, res, err)
}

// ScopedCreate reads the body and creates one record in the branch
func ScopedCreate[Req, Res any](c *fiber.Ctx, fn func(context.Context, uint, Req) (*Res, error)) error {
	var req Req
	if err := Bind(c, &req); err != nil {
		return Fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), req)
	return Created(c, res, err)
}

// ScopedRemove deletes the record named by :id in the branch
func ScopedRemove(c *fiber.Ctx, fn func(context.Context, uint, uint) error) error {
	id, err := ID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	return Done(c, "DELETED", fn(c.UserContext(), middleware.BranchID(c), id))
}

// OwnerParams reads /:ownerType/:ownerId; the venue has no id, so 0 is allowed here
func OwnerParams(c *fiber.Ctx) (string, uint, error) {
	id, err := c.ParamsInt("ownerId")
	if err != nil || id < 0 {
		return "", 0, response.ErrorBadRequest("INVALID_ID")
	}
	return c.Params("ownerType"), uint(id), nil
}
