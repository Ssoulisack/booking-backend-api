package response

import (
	"backend/internal/core/dto"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// NewError creates a new AppError with the specified code and message
func NewError(code int, errMsg string) error {
	return AppError{
		Status:  code,
		Message: errMsg,
	}
}

// ErrorBadRequest creates a new bad request error
func ErrorBadRequest(errorMessage string) error {
	return AppError{
		Status:  http.StatusBadRequest,
		Message: errorMessage,
	}
}

// ErrorUnprocessableEntity creates a new unprocessable entity error
func ErrorUnprocessableEntity(errorMessage string) error {
	return AppError{
		Status:  http.StatusUnprocessableEntity,
		Message: errorMessage,
	}
}

// NewErrorResponses converts an error to an appropriate HTTP response
func NewErrorResponses(ctx *fiber.Ctx, err error) error {
	var code int
	var message string

	switch e := err.(type) {
	case AppError:
		code = e.Status
		message = e.Message
	case error:
		code = http.StatusUnprocessableEntity
		message = err.Error()
	}

	response := dto.BaseResponse{
		Success: false,
		Message: message,
	}

	return ctx.Status(code).JSON(response)
}

// NewAppErrorStatusMessage creates an error with the given status code and message
func NewAppErrorStatusMessage(statusCode int, err error) error {
	return AppError{
		Status:  statusCode,
		Message: err.Error(),
	}
}

// NewErrorMessageResponse returns an error response with the given message
func NewErrorMessageResponse(ctx *fiber.Ctx, message interface{}) error {
	response := dto.BaseResponse{
		Success: false,
		Message: message.(string),
	}

	return ctx.Status(http.StatusUnprocessableEntity).JSON(response)
}

// NewErrorErrMsgInternalServerError returns an internal server error response
func NewErrorErrMsgInternalServerError(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgInternalServerError,
	}

	return ctx.Status(http.StatusInternalServerError).JSON(response)
}

// NewErrorErrMsgUnauthorized returns an unauthorized error response
func NewErrorErrMsgUnauthorized(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgUnauthorized,
	}

	return ctx.Status(http.StatusUnauthorized).JSON(response)
}

// NewErrorErrMsgUnauthorizedErrMsgInvalidToken returns an invalid token error response
func NewErrorErrMsgUnauthorizedErrMsgInvalidToken(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgInvalidAccessToken,
	}

	return ctx.Status(http.StatusUnauthorized).JSON(response)
}

// NewErrorBadRequest returns a bad request error response
func NewErrorBadRequest(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgBadRequest,
	}

	return ctx.Status(http.StatusBadRequest).JSON(response)
}

// NewErrorIDISRequired returns an error response when an ID is required
func NewErrorIDISRequired(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgParamIdIsRequired,
	}

	return ctx.Status(http.StatusBadRequest).JSON(response)
}

// NewErrorUnAuthorizeRole returns an error response for unauthorized role
func NewErrorUnAuthorizeRole(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgYourRoleNotAllowedToAccessThisResource,
	}

	return ctx.Status(http.StatusForbidden).JSON(response)
}

// NewErrorUnAuthorizePermission returns an error response for unauthorized permission
func NewErrorUnAuthorizePermission(ctx *fiber.Ctx) error {
	response := dto.BaseResponse{
		Success: false,
		Message: ErrMsgYourPermissionNotAllowedToAccessThisResource,
	}

	return ctx.Status(http.StatusForbidden).JSON(response)
}
