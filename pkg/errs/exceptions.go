package errs

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

func NewBusinessError(err error) error {
	return NewAppErrorStatusMessage(http.StatusBadRequest, err)
}
func NewDBError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NewAppErrorStatusMessage(http.StatusNotFound, errors.New(ErrMsgDataNotFound))
	case errors.Is(err, gorm.ErrInvalidTransaction):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrInvalidTransaction))
	case errors.Is(err, gorm.ErrNotImplemented):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrNotImplemented))
	case errors.Is(err, gorm.ErrMissingWhereClause):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrMissingWhereClause))
	case errors.Is(err, gorm.ErrUnsupportedRelation):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrUnsupportedRelation))
	case errors.Is(err, gorm.ErrPrimaryKeyRequired):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrPrimaryKeyRequired))
	case errors.Is(err, gorm.ErrModelValueRequired):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrModelValueRequired))
	case errors.Is(err, gorm.ErrModelAccessibleFieldsRequired):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrModelAccessibleFieldsRequired))
	case errors.Is(err, gorm.ErrSubQueryRequired):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrSubQueryRequired))
	case errors.Is(err, gorm.ErrInvalidData):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrInvalidData))
	case errors.Is(err, gorm.ErrUnsupportedDriver):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrUnsupportedDriver))
	case errors.Is(err, gorm.ErrRegistered):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrRegistered))
	case errors.Is(err, gorm.ErrInvalidDB):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrInvalidDB))
	case errors.Is(err, gorm.ErrInvalidValue):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrInvalidValue))
	case errors.Is(err, gorm.ErrInvalidValueOfLength):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrInvalidValueOfLength))
	case errors.Is(err, gorm.ErrPreloadNotAllowed):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrPreloadNotAllowed))
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrDuplicatedKey))
	default:
		return NewAppErrorStatusMessage(http.StatusInternalServerError, err)
	}
}
