package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// NationalityLaos is the ISO 3166-1 code of Laos: Lao accounts need a phone, email is optional
const NationalityLaos = "LA"

// GeneratedEmailDomain marks a stand-in address for accounts without a real email. ".invalid" is
// reserved (RFC 2606), so nothing can ever be delivered there; no email is sent to it.
const GeneratedEmailDomain = "no-email.invalid"

var (
	nationalityPattern = regexp.MustCompile(`^[A-Z]{2}$`)
	// a Lao mobile number as people type and see it: 20 and 8 digits (no 0, no +856)
	laoMobilePattern = regexp.MustCompile(`^20\d{8}$`)
	nonDigits        = regexp.MustCompile(`\D`)
)

// ValidNationality accepts an ISO 3166-1 alpha-2 code such as "LA" or "TH"
func ValidNationality(code string) bool { return nationalityPattern.MatchString(code) }

// NormalizePhone returns an account phone in the one format used everywhere: "2055551234"
// (20 and 8 digits). Spaces, dashes and habits such as "020 5555 1234" or "+856 20 5555 1234"
// are accepted and turned into that format; anything else (030, landlines, other countries) is refused.
func NormalizePhone(raw string) (string, bool) {
	digits := nonDigits.ReplaceAllString(raw, "")
	digits = strings.TrimPrefix(digits, "856")
	digits = strings.TrimPrefix(digits, "0")
	if !laoMobilePattern.MatchString(digits) {
		return "", false
	}
	return digits, true
}

// IsLaoPhone reports whether a stored phone has the account format (2055551234)
func IsLaoPhone(phone string) bool { return laoMobilePattern.MatchString(phone) }

// GeneratedEmail is the stand-in address of an account that has no real email yet
func GeneratedEmail(phone string) string {
	return strings.TrimPrefix(phone, "+") + "@" + GeneratedEmailDomain
}

// IsGeneratedEmail reports a stand-in address: never shown to staff, never sent to
func IsGeneratedEmail(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), "@"+GeneratedEmailDomain)
}

// FormatCustomerNo is the short number a customer can tell staff, e.g. 42 -> "C0042"
func FormatCustomerNo(id uint) string { return fmt.Sprintf("C%04d", id) }

// ParseCustomerNo reads "C0042", "c42", "C-42" or "42" back to the account id
func ParseCustomerNo(s string) (uint, bool) {
	s = strings.TrimPrefix(strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "C"), "-")
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}
