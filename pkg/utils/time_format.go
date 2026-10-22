package utils

import (
	"backend/pkg/constants"
	"backend/pkg/errs"
	"backend/pkg/response"
	"errors"

	"net/http"
	"time"
)

func NewDateTimeFormatToString(datetime time.Time) string {

	return datetime.Format(constants.APP_DATE_TIME_FORMAT)
}

// NewTimestampToDateTime convert timestamp to date time
func NewTimestampToDateTime(timestamp int64) time.Time {
	unixTime := time.Unix(timestamp, 0)
	return unixTime
}

func NewStringFormatToDate(datetime string) (time.Time, error) {
	//convert string to datetime
	loc, err := time.LoadLocation(constants.APP_TIME_ZONE)
	if err != nil {
		return time.Time{}, errs.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New("["+datetime+"]"+errs.InvalidDatetimeFormat))
	}
	t, err := time.ParseInLocation(constants.APP_DATE_FORMAT, datetime, loc)
	if err != nil {
		return time.Time{}, errs.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New("["+datetime+"]"+errs.InvalidDatetimeFormat))
	}

	// format the time value
	s := t.Format(constants.APP_DATE_TIME_LAYOUT_FORMAT)
	//TODO: format layout := "2006-01-02 15:04:05.000000 -0700 -07 MST m=+0.000000000"

	//TODO: input 2023-04-05 output 2023-04-05 00:00:00+07
	parse, err := time.Parse(constants.APP_DATE_TIME_LAYOUT_FORMAT, s)
	if err != nil {
		return time.Time{}, errs.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New("["+datetime+"]"+errs.InvalidDatetimeFormat))
	}
	return parse, nil

}

func NewStringFormatToDateTime(datetime string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return time.Time{}, response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New("["+datetime+"]"+"INVALID_DATETIME_FORMAT_EX:(yyyy-mm-dd hh:mm:ss)"))
	}
	t, err := time.ParseInLocation(constants.APP_DATE_TIME_FORMAT, datetime, loc)
	if err != nil {

		return time.Time{}, response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New("["+datetime+"]"+"INVALID_DATETIME_FORMAT_EX:(yyyy-mm-dd hh:mm:ss)"))
	}

	// format the time value
	s := t.Format(constants.APP_DATE_TIME_LAYOUT_FORMAT)
	//TODO: format layout := "2006-01-02 15:04:05.000000 -0700 -07 MST m=+0.000000000"
	//TODO: input 2023-04-05 17:28:04 output 2023-04-05 17:28:04 +0700 +07
	parse, err := time.Parse(constants.APP_DATE_TIME_LAYOUT_FORMAT, s)
	if err != nil {
		return time.Time{}, response.NewAppErrorStatusMessage(http.StatusBadRequest, errors.New("["+datetime+"]"+"INVALID_DATETIME_FORMAT_EX:(yyyy-mm-dd hh:mm:ss)"))
	}
	return parse, nil
}
