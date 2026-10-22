package response

// AppError represents a structured API error
type AppError struct {
	Status  int
	Message string
}

func (a AppError) Error() string {
	return a.Message
}

// Common error messages
const (
	ErrMsgDataNotFound                           = "DATA_NOT_FOUND"
	ErrMsgUnprocessableEntity                    = "UNPROCESSABLE_ENTITY"
	ErrMsgUnsupportedMediaType                   = "UNSUPPORTED_MEDIA_TYPE"
	ErrMsgMethodNotAllowed                       = "METHOD_NOT_ALLOWED"
	ErrMsgNotImplemented                         = "NOT_IMPLEMENTED"
	ErrMsgBadGateway                             = "BAD_GATEWAY"
	ErrMsgServiceUnavailable                     = "SERVICE_UNAVAILABLE"
	ErrMsgGatewayTimeout                         = "GATEWAY_TIMEOUT"
	ErrMsgForbidden                              = "FORBIDDEN"
	ErrMsgInternalServerError                    = "INTERNAL_SERVER_ERROR"
	ErrMsgBadRequest                             = "BAD_REQUEST"
	ErrMsgUnauthorized                           = "UNAUTHORIZED"
	ErrMsgInvalidAccessToken                     = "INVALID_ACCESS_TOKEN"
	ErrMsgParamIdIsRequired                      = "ID_PARAMETER_IS_REQUIRED"
	ErrMsgTooManyRequests                        = "TOO_MANY_REQUESTS"
	ErrMsgRateLimitExceeded                      = "RATE_LIMIT_EXCEEDED"
	ErrMsgInvalidRefreshToken                    = "INVALID_REFRESH_TOKEN"
	ErrMsgYourRoleNotAllowedToAccessThisResource = "YOUR_ROLE_NOT_ALLOWED_TO_ACCESS_THIS_RESOURCE"
	ErrMsgYourPermissionNotAllowedToAccessThisResource = "YOUR_PERMISSION_NOT_ALLOWED_TO_ACCESS_THIS_RESOURCE"
)
