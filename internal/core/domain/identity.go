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

// Every account has one number people can tell staff, stored in users.member_no. The prefix
// follows the role: customers CNO, the team (owner, admins, managers, staff) ENO. Each prefix has
// its own counter; when the role changes the account gets the next number of the other kind.
const (
	CustomerNoPrefix = "CNO"
	EmployeeNoPrefix = "ENO"
)

// FormatMemberNo gives the stored form: ("CNO", 42) -> "CNO000042"; past 999999 it simply grows
func FormatMemberNo(prefix string, n int64) string { return fmt.Sprintf("%s%06d", prefix, n) }

var memberNoPattern = regexp.MustCompile(`^(CNO|ENO)[\s-]*0*([1-9]\d{0,8})$`)

// ParseMemberNo reads what people type ("CNO000042", "cno42", "ENO-7", "eno 000007") into the
// stored form; false for anything else
func ParseMemberNo(s string) (string, bool) {
	m := memberNoPattern.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(s)))
	if m == nil {
		return "", false
	}
	n, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return "", false
	}
	return FormatMemberNo(m[1], n), true
}

// MemberNoOf is the stored number, or "" before one is given
func MemberNoOf(no *string) string {
	if no == nil {
		return ""
	}
	return *no
}
