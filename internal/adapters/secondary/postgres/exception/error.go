package exception

import (
	"backend/pkg/response"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Error messages
const (
	ErrMsgDataNotFound               = "data not found"
	ErrInvalidTransaction            = "invalid transaction"
	ErrNotImplemented                = "not implemented"
	ErrMissingWhereClause            = "missing WHERE clause"
	ErrUnsupportedRelation           = "unsupported relation"
	ErrPrimaryKeyRequired            = "primary key required"
	ErrModelValueRequired            = "model value required"
	ErrInvalidData                   = "invalid data"
	ErrUnsupportedDriver             = "unsupported database driver"
	ErrRegistered                    = "already registered"
	ErrInvalidField                  = "invalid field"
	ErrEmptySlice                    = "empty slice found"
	ErrDryRunModeUnsupported         = "dry run mode not supported"
	ErrInvalidDB                     = "invalid database"
	ErrDatabaseError                 = "database error"
	ErrDuplicateEntry                = "duplicate entry"
	ErrForeignKeyConstraintViolation = "foreign key constraint violation"
	ErrGenericDatabaseError          = "generic database error"
)

func HandleGormError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return response.NewAppErrorStatusMessage(http.StatusNotFound, errors.New(ErrMsgDataNotFound))
	case errors.Is(err, gorm.ErrInvalidTransaction):
		return response.NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrInvalidTransaction))
	case errors.Is(err, gorm.ErrNotImplemented):
		return response.NewAppErrorStatusMessage(http.StatusConflict, errors.New(ErrNotImplemented))
	case errors.Is(err, gorm.ErrMissingWhereClause):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrMissingWhereClause))
	case errors.Is(err, gorm.ErrUnsupportedRelation):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrUnsupportedRelation))
	case errors.Is(err, gorm.ErrPrimaryKeyRequired):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrPrimaryKeyRequired))
	case errors.Is(err, gorm.ErrModelValueRequired):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrModelValueRequired))
	case errors.Is(err, gorm.ErrInvalidData):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrInvalidData))
	case errors.Is(err, gorm.ErrUnsupportedDriver):
		return response.NewAppErrorStatusMessage(http.StatusInternalServerError, errors.New(ErrUnsupportedDriver))
	case errors.Is(err, gorm.ErrRegistered):
		return response.NewAppErrorStatusMessage(http.StatusInternalServerError, errors.New(ErrRegistered))
	case errors.Is(err, gorm.ErrInvalidField):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrInvalidField))
	case errors.Is(err, gorm.ErrEmptySlice):
		return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrEmptySlice))
	case errors.Is(err, gorm.ErrDryRunModeUnsupported):
		return response.NewAppErrorStatusMessage(http.StatusInternalServerError, errors.New(ErrDryRunModeUnsupported))
	case errors.Is(err, gorm.ErrInvalidDB):
		return response.NewAppErrorStatusMessage(http.StatusInternalServerError, errors.New(ErrInvalidDB))
	default:
		if err != nil {
			// PostgreSQL-specific error handling
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				switch pgErr.Code {
				case "23505": // unique_violation
					return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrDuplicateEntry))
				case "23503": // foreign_key_violation
					return response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New(ErrForeignKeyConstraintViolation))
				case "23P01": // exclusion_violation: two active bookings of one resource or coach would overlap
					return response.NewAppErrorStatusMessage(http.StatusConflict, errors.New("SLOT_NOT_AVAILABLE"))
				}
			}

			// Fallback for any other database error
			return response.NewAppErrorStatusMessage(http.StatusInternalServerError, errors.New("Database error: "+err.Error()))
		}
		return nil
	}
}
