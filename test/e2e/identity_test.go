package e2e

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Phase 6d.2: nationality, phone login, stand-in emails, profile, desk lookup by phone or customer number (CNO)
func TestCustomerIdentity(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	day := tomorrow()
	admin := ownerToken(a) // the company owner (from the test config) runs the branches
	bizID, _, owner := setupClub(t, a, admin, "Identity Club", "owner@identity.test")
	m := "/branches/" + bizID
	reg := func(body map[string]any) reply { return a.call("POST", "/auth/register", "", body) }

	// --- registering: nationality required; Lao needs a phone, gets a stand-in email; others need an email ---
	if r := reg(map[string]any{"name": "No Nation", "email": "n@id.test", "password": "password123"}); r.msg() != "NATIONALITY_REQUIRED" {
		t.Fatalf("no nationality: %d %v", r.status, r.body)
	}
	if r := reg(map[string]any{"name": "Lao NoPhone", "password": "password123", "nationality": "LA"}); r.msg() != "PHONE_REQUIRED" {
		t.Fatalf("Lao without phone: %d %v", r.status, r.body)
	}
	if r := reg(map[string]any{"name": "Thai NoEmail", "password": "password123", "nationality": "TH"}); r.msg() != "EMAIL_REQUIRED" {
		t.Fatalf("foreigner without email: %d %v", r.status, r.body)
	}
	noy := reg(map[string]any{"name": "Noy Phone", "phone": "020 5555 7001", "password": "password123", "nationality": "la"})
	if noy.status != 201 || noy.str("data.user.phone") != "2055557001" || noy.str("data.user.nationality") != "LA" ||
		noy.str("data.user.email_generated") != "true" || !strings.HasPrefix(noy.str("data.user.member_no"), "CNO") {
		t.Fatalf("Lao register: %d %v", noy.status, noy.body)
	}
	if r := reg(map[string]any{"name": "Copy", "phone": "+856 20 5555 7001", "password": "password123", "nationality": "LA"}); r.msg() != "PHONE_ALREADY_REGISTERED" {
		t.Fatalf("same phone twice: %d %v", r.status, r.body)
	}
	if r := reg(map[string]any{"name": "Old 030", "phone": "030 512 3456", "password": "password123", "nationality": "LA"}); r.msg() != "INVALID_PHONE" {
		t.Fatalf("only 20xxxxxxxx: %d %v", r.status, r.body)
	}
	if r := reg(map[string]any{"name": "Bad", "phone": "0812345678", "email": "bad@id.test", "password": "password123", "nationality": "TH"}); r.msg() != "INVALID_PHONE" {
		t.Fatalf("foreign phone without country code: %d %v", r.status, r.body)
	}
	// account phones are Lao numbers only, whatever the nationality; a foreigner without one leaves it empty
	if r := reg(map[string]any{"name": "Thai Phone", "phone": "+66 81 234 5678", "email": "thai@id.test", "password": "password123", "nationality": "TH"}); r.msg() != "INVALID_PHONE" {
		t.Fatalf("foreign number on an account: %d %v", r.status, r.body)
	}
	if r := reg(map[string]any{"name": "Thai LaoPhone", "phone": "020 5555 7009", "email": "thai2@id.test", "password": "password123", "nationality": "TH"}); r.status != 201 || r.str("data.user.phone") != "2055557009" {
		t.Fatalf("foreigner with a Lao number: %d %v", r.status, r.body)
	}

	// --- logging in with the phone, in any spelling; the stand-in email can never be reset by email ---
	for _, login := range []string{"2055557001", "02055557001", "+856 20 5555 7001", "20 5555 7001"} {
		a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": login, "password": "password123"})
	}
	resetMailpit(t)
	a.expect(200, "", "POST", "/auth/forgot-password", "", map[string]any{"email": noy.str("data.user.email")})

	// --- My account: add a real email; staff then see it ---
	token := noy.str("data.access_token")
	me := a.expect(200, "", "PUT", "/auth/me", token, map[string]any{"name": "Noy Phone", "nationality": "LA", "phone": "020 5555 7001", "email": "Noy@ID.test"})
	if me.str("data.email") != "noy@id.test" || me.str("data.email_generated") != "false" {
		t.Fatalf("profile: %v", me.body)
	}
	a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": "noy@id.test", "password": "password123"})

	// --- the desk finds the account by phone or customer number, and books for it ---
	sport := num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": "Tennis"}).str("data.id"))
	court := num(a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": sport, "name": "Court 1", "kind": "court"}).str("data.id"))
	a.expect(200, "", "PUT", m+"/opening-hours/resource/"+strconv.Itoa(court), owner, map[string]any{"hours": []map[string]any{{"weekday": int(day.date.Weekday()), "open": "08:00", "close": "22:00"}}})
	a.expect(200, "", "PUT", m+"/prices/resource/"+strconv.Itoa(court), owner, map[string]any{"price_per_hour": 100, "currency": "LAK"})
	found := a.expect(200, "", "GET", m+"/customers/find?q=020%205555%207001", owner, nil)
	if found.str("data.name") != "Noy Phone" || found.str("data.member_no") != noy.str("data.user.member_no") {
		t.Fatalf("find by phone: %v", found.body)
	}
	a.expect(200, "", "GET", m+"/customers/find?q="+noy.str("data.user.member_no"), owner, nil)
	a.expect(400, "CUSTOMER_NOT_FOUND", "GET", m+"/customers/find?q=020%209999%209999", owner, nil)
	bk := a.expect(201, "", "POST", m+"/bookings", owner, map[string]any{"resource_id": court, "starts_at": day.at(9, 0), "duration_min": 60, "customer": "2055557001"})
	if bk.str("data.customer_name") != "Noy Phone" || bk.str("data.customer_phone") != "2055557001" {
		t.Fatalf("desk booking by phone: %v", bk.body)
	}

	// --- a Lao customer without email: staff never see the stand-in address ---
	keo := reg(map[string]any{"name": "Keo NoMail", "phone": "20 5512 7002", "password": "password123", "nationality": "LA"})
	bk2 := a.expect(201, "", "POST", m+"/bookings", owner, map[string]any{"resource_id": court, "starts_at": day.at(11, 0), "duration_min": 60, "customer": keo.str("data.user.member_no")})
	if bk2.str("data.customer_email") != "" || strings.Contains(a.raw("GET", m+"/customers", owner), "no-email.invalid") {
		t.Fatalf("stand-in email shown to staff: %v", bk2.body)
	}

	// --- the owner finds them by phone (any spelling) or customer number, newest first ---
	for _, q := range []string{"020 5512 7002", keo.str("data.user.member_no")} {
		found := a.expect(200, "", "GET", "/company/users?keyword="+url.QueryEscape(q), admin, nil)
		if found.str("data.0.id") != keo.str("data.user.id") {
			t.Fatalf("user search %q: %v", q, found.body)
		}
	}

	// --- the owner sets a new password for someone without a real email ---
	time.Sleep(1100 * time.Millisecond) // tokens carry whole seconds; make the old token clearly older
	a.expect(200, "PASSWORD_UPDATED", "POST", "/company/users/"+keo.str("data.user.id")+"/password", admin, map[string]any{"password": "brand-new-pass"})
	a.expect(401, "TOKEN_REVOKED", "GET", "/auth/me", keo.str("data.access_token"), nil)
	a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": "2055127002", "password": "brand-new-pass"})
}
