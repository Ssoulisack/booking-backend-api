package utils

import (
	"errors"
	"strconv"
	"time"
)

// StringToUint converts a string ID to uint
// It uses strconv.ParseUint with base 10 and 32 bit size
func StringToUint(id string) (uint, error) {
	if id == "" {
		return 0, errors.New("empty ID")
	}

	uid, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return 0, errors.New("invalid ID format")
	}

	return uint(uid), nil
}

// UintToString converts a uint ID to string
// It uses strconv.FormatUint with base 10
func UintToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// StringToUintSafe converts a string ID to uint without returning an error
// If the conversion fails, it returns 0
// This is useful for optional IDs in model conversions
func StringToUintSafe(id string) uint {
	if id == "" {
		return 0
	}

	uid, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return 0
	}

	return uint(uid)
}

// FormatDateTime converts a time.Time to a formatted string for API responses
// It returns the date in a user-friendly format with timezone information
// and converts UTC time to local time (Asia/Bangkok, GMT+7)
func FormatDateTime(t time.Time) string {
	//log input t
	//log.Println("input time: ", t)
	// Check for zero time or invalid time
	if t.IsZero() || t.Year() < 1970 {
		return ""
	}

	// Define the Asia/Bangkok location
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		// Fallback to fixed +7 hours offset if location loading fails
		loc = time.FixedZone("GMT+7", 7*60*60)
	}

	// Convert the time to the local timezone
	localTime := t.In(loc)

	// Format with timezone information
	//log output localTime
	//log.Println("output localTime: ", localTime)
	//log.Println("output localTime format: ", localTime.Format("2006-01-02 15:04:05 -07:00"))

	return localTime.Format("2006-01-02 15:04:05 -07:00")

}

// TimeToString is a helper function to convert time.Time to string
func TimeToString(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return FormatDateTime(*t)
}

// ParseDateTime parses a string into a time.Time with proper timezone
func ParseDateTime(s string) (time.Time, error) {
	// Try parsing with various formats
	formats := []string{
		"2006-01-02 15:04:05.999999 -07:00",
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
	}

	var t time.Time
	var err error

	for _, format := range formats {
		t, err = time.Parse(format, s)
		if err == nil {
			break
		}
	}

	if err != nil {
		return time.Time{}, err
	}

	// Ensure timezone is set to Bangkok if not specified
	if t.Location().String() == "UTC" {
		loc, err := time.LoadLocation("Asia/Bangkok")
		if err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(),
				t.Nanosecond(), loc)
		}
	}

	return t, nil
}
