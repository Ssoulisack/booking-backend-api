package response

import (
	"backend/internal/core/dto"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// NewSuccessResponse sends a successful response with data
func NewSuccessResponse(ctx *fiber.Ctx, data interface{}) error {
	response := dto.BaseResponse{
		Success: true,
		Data:    data,
	}
	return ctx.Status(http.StatusOK).JSON(response)
}

// NewSuccessMessagesResponse sends a successful response with a message
func NewSuccessMessagesResponse(ctx *fiber.Ctx, message string) error {
	response := dto.BaseResponse{
		Success: true,
		Message: message,
	}
	return ctx.Status(http.StatusOK).JSON(response)
}

// NewCreatedResponse sends a 201 Created response with data
func NewCreatedResponse(ctx *fiber.Ctx, data interface{}) error {
	response := dto.BaseResponse{
		Success: true,
		Data:    data,
	}
	return ctx.Status(http.StatusCreated).JSON(response)
}

// NewPaginatedResponse sends a paginated response
func NewPaginatedResponse(ctx *fiber.Ctx, data interface{}, meta dto.Meta) error {
	response := dto.BaseResponse{
		Success: true,
		Data:    data,
		Meta: &dto.Meta{
			Total:     meta.Total,
			Page:      meta.Page,
			Limit:     meta.Limit,
			TotalPage: meta.TotalPage,
		},
	}
	return ctx.Status(http.StatusOK).JSON(response)
}
