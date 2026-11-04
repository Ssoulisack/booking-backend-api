// Package httpx: the small helpers every module's handlers use (read an id or a body, answer with
// the standard success / error shapes).
package httpx

import (
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ID reads a positive numeric path parameter
func ID(c *fiber.Ctx, name string) (uint, error) {
	id, err := c.ParamsInt(name)
	if err != nil || id < 1 {
		return 0, response.ErrorBadRequest("INVALID_ID")
	}
	return uint(id), nil
}

// Bind reads the JSON body into req
func Bind(c *fiber.Ctx, req any) error {
	if err := c.BodyParser(req); err != nil {
		return response.ErrorBadRequest("INVALID_BODY")
	}
	return nil
}

// Fail answers with the error's status and code
func Fail(c *fiber.Ctx, err error) error { return response.NewErrorResponses(c, err) }

// OK answers 200 with data, or the error
func OK(c *fiber.Ctx, data any, err error) error {
	if err != nil {
		return Fail(c, err)
	}
	return response.NewSuccessResponse(c, data)
}

// Created answers 201 with data, or the error
func Created(c *fiber.Ctx, data any, err error) error {
	if err != nil {
		return Fail(c, err)
	}
	return response.NewCreatedResponse(c, data)
}

// Done answers 200 with a message, or the error
func Done(c *fiber.Ctx, message string, err error) error {
	if err != nil {
		return Fail(c, err)
	}
	return response.NewSuccessMessagesResponse(c, message)
}
