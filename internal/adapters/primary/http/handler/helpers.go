package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/pkg/response"
	"context"

	"github.com/gofiber/fiber/v2"
)

// idParam reads a positive numeric path parameter
func idParam(c *fiber.Ctx, name string) (uint, error) {
	id, err := c.ParamsInt(name)
	if err != nil || id < 1 {
		return 0, response.ErrorBadRequest("INVALID_ID")
	}
	return uint(id), nil
}

// ownerParams reads /:ownerType/:ownerId; the venue has no id, so 0 is allowed here
func ownerParams(c *fiber.Ctx) (string, uint, error) {
	id, err := c.ParamsInt("ownerId")
	if err != nil || id < 0 {
		return "", 0, response.ErrorBadRequest("INVALID_ID")
	}
	return c.Params("ownerType"), uint(id), nil
}

func fail(c *fiber.Ctx, err error) error { return response.NewErrorResponses(c, err) }

func bind(c *fiber.Ctx, req any) error {
	if err := c.BodyParser(req); err != nil {
		return response.ErrorBadRequest("INVALID_BODY")
	}
	return nil
}

func ok(c *fiber.Ctx, data any, err error) error {
	if err != nil {
		return fail(c, err)
	}
	return response.NewSuccessResponse(c, data)
}

func created(c *fiber.Ctx, data any, err error) error {
	if err != nil {
		return fail(c, err)
	}
	return response.NewCreatedResponse(c, data)
}

func done(c *fiber.Ctx, message string, err error) error {
	if err != nil {
		return fail(c, err)
	}
	return response.NewSuccessMessagesResponse(c, message)
}

func paged[Res any](c *fiber.Ctx, res *dto.PagedResult[Res], err error) error {
	if err != nil {
		return fail(c, err)
	}
	return response.NewPaginatedResponse(c, res.Items, res.Meta)
}

func listQuery(c *fiber.Ctx) (dto.ListQuery, error) {
	var q dto.ListQuery
	if err := c.QueryParser(&q); err != nil {
		return q, response.ErrorBadRequest("INVALID_QUERY")
	}
	return q, nil
}

// ---------- branch-scoped CRUD helpers: the branch comes from middleware.Branch ----------

func scopedCreate[Req, Res any](c *fiber.Ctx, fn func(context.Context, uint, Req) (*Res, error)) error {
	var req Req
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), req)
	return created(c, res, err)
}

func scopedGet[Res any](c *fiber.Ctx, fn func(context.Context, uint, uint) (*Res, error)) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), id)
	return ok(c, res, err)
}

func scopedList[Res any](c *fiber.Ctx, fn func(context.Context, uint, dto.ListQuery) (*dto.PagedResult[Res], error)) error {
	q, err := listQuery(c)
	if err != nil {
		return fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), q)
	return paged(c, res, err)
}

func scopedUpdate[Req, Res any](c *fiber.Ctx, fn func(context.Context, uint, uint, Req) (*Res, error)) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req Req
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := fn(c.UserContext(), middleware.BranchID(c), id, req)
	return ok(c, res, err)
}

func scopedRemove(c *fiber.Ctx, fn func(context.Context, uint, uint) error) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	return done(c, "DELETED", fn(c.UserContext(), middleware.BranchID(c), id))
}
