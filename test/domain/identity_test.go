package domaintest

import (
	"backend/internal/core/domain"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"2055551234", "2055551234", true},
		{"20 5555 1234", "2055551234", true},
		{"020 5555 1234", "2055551234", true}, // habits are accepted and converted
		{"+856 20 5555 1234", "2055551234", true},
		{"856-20-5555-1234", "2055551234", true},
		{"030 512 3456", "", false}, // only 20 + 8 digits
		{"+66 81 234 5678", "", false},
		{"20555512", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := domain.NormalizePhone(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
	if !domain.IsLaoPhone("2055551234") || domain.IsLaoPhone("+8562055551234") {
		t.Error("IsLaoPhone")
	}
}

func TestMemberNumbers(t *testing.T) {
	if got := domain.FormatMemberNo(domain.CustomerNoPrefix, 42); got != "CNO000042" {
		t.Errorf("FormatMemberNo(CNO, 42) = %s", got)
	}
	if got := domain.FormatMemberNo(domain.EmployeeNoPrefix, 1234567); got != "ENO1234567" {
		t.Errorf("past six digits the number grows: %s", got)
	}
	for in, want := range map[string]string{
		"CNO000042": "CNO000042", "cno42": "CNO000042", "CNO-42": "CNO000042", " eno 000007 ": "ENO000007",
	} {
		if got, ok := domain.ParseMemberNo(in); !ok || got != want {
			t.Errorf("ParseMemberNo(%q) = %q, %v", in, got, ok)
		}
	}
	// the old customer number (C0042), a bare number and zero are not member numbers
	for _, bad := range []string{"", "CNO", "CNO0", "C0042", "42", "XNO42", "CNOabc"} {
		if _, ok := domain.ParseMemberNo(bad); ok {
			t.Errorf("ParseMemberNo(%q) should fail", bad)
		}
	}
}

func TestStandInEmail(t *testing.T) {
	email := domain.GeneratedEmail("2055551234")
	if email != "2055551234@no-email.invalid" || !domain.IsGeneratedEmail(email) || domain.IsGeneratedEmail("a@b.la") {
		t.Errorf("stand-in email: %s", email)
	}
	u := domain.User{Email: email, EmailGenerated: true}
	if u.ContactEmail() != "" {
		t.Error("a stand-in must never be shown")
	}
	if (domain.User{Email: "noy@mail.la"}).ContactEmail() != "noy@mail.la" {
		t.Error("a real email is shown")
	}
	if !domain.ValidNationality("LA") || domain.ValidNationality("lao") || domain.ValidNationality("") {
		t.Error("ValidNationality")
	}
}

func TestSportTypesAndProvinces(t *testing.T) {
	for name, want := range map[string]string{
		"Tennis": "tennis", "Table tennis": "table_tennis", "Court tennis VIP": "tennis", "Futsal 5v5": "futsal",
		"Soccer": "football", "ບານເຕະ": "football", "ມວຍໄທ": "muay_thai", "Boxing": "boxing", "ແບດມິນຕັນ": "badminton",
		"Swimming pool": "swimming", "Archery": "other",
	} {
		if got := domain.GuessSportType(name); got != want {
			t.Errorf("GuessSportType(%q) = %s, want %s", name, got, want)
		}
	}
	if !domain.ValidSportType("muay_thai") || domain.ValidSportType("quidditch") || domain.ValidSportType("") {
		t.Error("ValidSportType")
	}
	if !domain.ValidProvince("") || !domain.ValidProvince("luang_prabang") || domain.ValidProvince("bangkok") || len(domain.Provinces) != 18 {
		t.Error("provinces")
	}
}
