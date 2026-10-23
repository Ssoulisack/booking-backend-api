package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Phase 1 "Done when": a fresh install creates the company and its owner; the owner creates two
// branches and a manager for one; the manager cannot see the other branch; the company report adds
// both branches; a customer books at both with one account. Also the team rules, company defaults
// with branch overrides, closing a branch and handing over ownership.
func TestCompanyWorkflow(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	const pw = "password123"
	day := tomorrow()

	// --- the install created the company, its booking rule and the owner from the config ---
	owner := ownerToken(a)
	if me := a.expect(200, "", "GET", "/auth/me", owner, nil); me.str("data.company_role") != "owner" {
		t.Fatalf("owner role: %v", me.body)
	}
	if c := a.expect(200, "", "GET", "/company/", owner, nil); c.str("data.name") != "E2E Sports Co" || c.str("data.currency") != "LAK" {
		t.Fatalf("company: %v", c.body)
	}
	if r := a.expect(200, "", "GET", "/company/rules", owner, nil); r.str("data.max_duration_min") != "180" {
		t.Fatalf("company rule: %v", r.body)
	}
	customer := register(a, "Cara Customer", "cara@test.la")
	a.expect(403, "ROLE_NOT_ALLOWED", "GET", "/company/", customer, nil)
	a.expect(401, "", "GET", "/company/", "", nil)

	// --- company settings: the brand every branch shows ---
	company := map[string]any{
		"name": "Riverside Sports", "currency": "LAK", "language": "en", "timezone": "Asia/Vientiane",
		"booking_mode": "both", "brand_color": "#0e7c66", "booking_policy": "Free cancellation up to 24 hours before.",
	}
	a.expect(200, "", "PUT", "/company/", owner, company)
	bad := map[string]any{}
	for k, v := range company {
		bad[k] = v
	}
	bad["currency"] = "lakk"
	a.expect(400, "INVALID_CURRENCY", "PUT", "/company/", owner, bad)
	if pc := a.expect(200, "", "GET", "/public/company", "", nil); pc.str("data.name") != "Riverside Sports" || pc.str("data.brand_color") != "#0e7c66" {
		t.Fatalf("public company: %v", pc.body)
	}

	// --- two branches ---
	brA := a.expect(201, "", "POST", "/company/branches", owner, map[string]any{"name": "Riverside Sisattanak"})
	if brA.str("data.slug") != "riverside-sisattanak" || brA.str("data.status") != "draft" {
		t.Fatalf("branch: %v", brA.body)
	}
	idA := brA.str("data.id")
	idB := a.expect(201, "", "POST", "/company/branches", owner, map[string]any{"name": "Riverside Xaythany"}).str("data.id")
	a.expect(409, "SLUG_TAKEN", "POST", "/company/branches", owner, map[string]any{"name": "X", "slug": "riverside-sisattanak"})
	if l := a.expect(200, "", "GET", "/company/branches", owner, nil); len(l.body["data"].([]any)) != 2 {
		t.Fatalf("branches: %v", l.body)
	}
	mA, mB := "/branches/"+idA, "/branches/"+idB

	// --- team: a manager of A, staff at both, an admin for every branch ---
	a.expect(400, "INVALID_ROLE", "POST", "/company/invitations", owner, map[string]any{"email": "x@co.test", "role": "owner"})
	a.expect(400, "CHOOSE_AT_LEAST_ONE_BRANCH", "POST", "/company/invitations", owner, map[string]any{"email": "x@co.test", "role": "manager"})
	a.expect(201, "", "POST", "/company/invitations", owner, map[string]any{"email": "manager@co.test", "role": "manager", "branch_ids": []int{num(idA)}})
	a.expect(409, "INVITATION_ALREADY_PENDING", "POST", "/company/invitations", owner, map[string]any{"email": "manager@co.test", "role": "staff", "branch_ids": []int{num(idB)}})
	tok := linkFor(t, "manager@co.test", "/invite/")
	info := a.expect(200, "", "GET", "/invitations/"+tok, "", nil)
	if info.str("data.role") != "manager" || info.str("data.company_name") != "Riverside Sports" || info.str("data.branches.0") != "Riverside Sisattanak" {
		t.Fatalf("invitation info: %v", info.body)
	}
	manager := a.expect(200, "", "POST", "/invitations/"+tok+"/accept", "", map[string]any{"name": "Max Manager", "password": pw}).str("data.access_token")
	a.expect(404, "INVITATION_NOT_FOUND_OR_EXPIRED", "GET", "/invitations/"+tok, "", nil) // single use
	a.expect(409, "ALREADY_IN_THE_TEAM", "POST", "/company/invitations", owner, map[string]any{"email": "manager@co.test", "role": "staff", "branch_ids": []int{num(idB)}})
	staff := teamMember(a, owner, "staff@co.test", "staff", idA, idB)
	admin := teamMember(a, owner, "admin@co.test", "admin")

	switcher := func(token string) string {
		return fmt.Sprint(a.expect(200, "", "GET", "/me/branches", token, nil).body["data"])
	}
	if s := switcher(manager); strings.Contains(s, "Xaythany") || !strings.Contains(s, "Sisattanak") || !strings.Contains(s, "manager") {
		t.Fatalf("manager switcher: %s", s)
	}
	if s := switcher(owner); !strings.Contains(s, "Xaythany") || !strings.Contains(s, "Sisattanak") {
		t.Fatalf("owner switcher: %s", s)
	}
	if s := switcher(staff); !strings.Contains(s, "Xaythany") || !strings.Contains(s, "Sisattanak") {
		t.Fatalf("staff switcher: %s", s)
	}

	// --- the manager sets up branch A and cannot see branch B ---
	week := []map[string]any{}
	for d := range 7 {
		week = append(week, map[string]any{"weekday": d, "open": "08:00", "close": "22:00"})
	}
	setUp := func(m, token string, price int) (court int) {
		sport := a.expect(201, "", "POST", m+"/sports", token, map[string]any{"name": "Tennis", "type": "tennis"}).str("data.id")
		c := a.expect(201, "", "POST", m+"/resources", token, map[string]any{"sport_id": num(sport), "name": "Court 1", "kind": "court"}).str("data.id")
		a.expect(200, "", "PUT", m+"/opening-hours/resource/"+c, token, map[string]any{"hours": week})
		a.expect(200, "", "PUT", m+"/prices/resource/"+c, token, map[string]any{"price_per_hour": price, "currency": "LAK"})
		return num(c)
	}
	a.expect(400, "PUBLISH_NEEDS_ACTIVE_RESOURCE_WITH_HOURS_AND_PRICE", "POST", mA+"/publish", owner, nil)
	courtA := setUp(mA, manager, 100)
	a.expect(200, "", "GET", mA+"/sports", staff, nil)
	a.expect(403, "ROLE_NOT_ALLOWED", "POST", mA+"/sports", staff, map[string]any{"name": "Boxing", "type": "boxing"})
	// something created hidden stays hidden
	if h := a.expect(201, "", "POST", mA+"/sports", manager, map[string]any{"name": "Golf", "type": "golf", "active": false}); h.str("data.active") != "false" ||
		a.expect(200, "", "GET", mA+"/sports/"+h.str("data.id"), manager, nil).str("data.active") != "false" {
		t.Fatalf("hidden sport: %v", h.body)
	}
	for _, probe := range []struct{ method, path string }{
		{"GET", "/settings"}, {"GET", "/sports"}, {"POST", "/sports"}, {"GET", "/bookings"}, {"GET", "/reports/finance"},
	} {
		a.expect(404, "BRANCH_NOT_FOUND", probe.method, mB+probe.path, manager, map[string]any{})
	}
	a.expect(403, "ROLE_NOT_ALLOWED", "POST", mA+"/publish", manager, nil) // publishing is for the owner and admins
	a.expect(403, "ROLE_NOT_ALLOWED", "GET", "/company/", manager, nil)
	a.expect(403, "ROLE_NOT_ALLOWED", "POST", "/company/branches", manager, map[string]any{"name": "Rogue"})

	// --- publishing: the page shows the company brand; drafts stay hidden ---
	a.expect(404, "BRANCH_NOT_FOUND", "GET", "/public/branches/riverside-sisattanak/", "", nil)
	if p := a.expect(200, "", "POST", mA+"/publish", admin, nil); p.str("data.status") != "published" || p.str("data.public_path") != "/b/riverside-sisattanak" {
		t.Fatalf("publish: %v", p.body)
	}
	pub := a.expect(200, "", "GET", "/public/branches/riverside-sisattanak/", "", nil)
	if pub.str("data.company_name") != "Riverside Sports" || pub.str("data.brand_color") != "#0e7c66" || pub.str("data.booking_mode") != "both" {
		t.Fatalf("public branch: %v", pub.body)
	}
	if l := a.expect(200, "", "GET", "/public/branches", "", nil); l.str("meta.total") != "1" || l.str("data.0.slug") != "riverside-sisattanak" {
		t.Fatalf("public list: %v", l.body)
	}
	a.expect(404, "BRANCH_NOT_FOUND", "GET", "/public/branches/riverside-xaythany/", "", nil)

	// --- company defaults and branch overrides ---
	effective := func(m, token string) reply { return a.expect(200, "", "GET", m+"/rules/effective", token, nil) }
	if c := a.expect(200, "", "GET", mA+"/rules/company", manager, nil); c.str("data.max_duration_min") != "180" {
		t.Fatalf("company rule for the branch team: %v", c.body)
	}
	a.expect(200, "", "PUT", mA+"/rules/branch/0", manager, map[string]any{"max_duration_min": 120})
	if e := effective(mA, manager); e.str("data.max_duration_min") != "120" || e.str("data.cancel_before_hours") != "24" {
		t.Fatalf("A with its change: %v", e.body)
	}
	a.expect(200, "", "PUT", "/company/rules", owner, map[string]any{
		"min_duration_min": 60, "max_duration_min": 240, "duration_step_min": 30, "start_step_min": 30,
		"cancel_before_hours": 48, "max_days_ahead": 30, "min_notice_min": 0, "capacity": 10, "max_active_bookings_per_customer": 3,
	})
	if e := effective(mB, owner); e.str("data.max_duration_min") != "240" || e.str("data.cancel_before_hours") != "48" {
		t.Fatalf("B follows the company: %v", e.body)
	}
	if e := effective(mA, manager); e.str("data.max_duration_min") != "120" || e.str("data.cancel_before_hours") != "48" {
		t.Fatalf("A keeps its own change only: %v", e.body)
	}
	settings := a.expect(200, "", "GET", mA+"/settings", manager, nil)
	if settings.str("data.in_force.booking_mode") != "both" || settings.str("data.in_force.currency") != "LAK" {
		t.Fatalf("settings in force: %v", settings.body)
	}
	a.expect(403, "ROLE_NOT_ALLOWED", "PUT", mA+"/settings", manager, map[string]any{"name": "Renamed"})
	a.expect(200, "", "PUT", mA+"/settings", owner, map[string]any{
		"name": "Riverside Sisattanak", "slug": "riverside-sisattanak", "address": "Ban Phonxay", "province": "vientiane_capital",
		"booking_mode": "account",
	})
	if p := a.expect(200, "", "GET", "/public/branches/riverside-sisattanak/", "", nil); p.str("data.booking_mode") != "account" || p.str("data.province") != "vientiane_capital" {
		t.Fatalf("branch override: %v", p.body)
	}
	a.expect(409, "SLUG_LOCKED_AFTER_PUBLISH", "PUT", mA+"/settings", owner, map[string]any{"name": "Riverside Sisattanak", "slug": "new-slug"})

	// --- a customer books at both branches with one account; the company report adds them up ---
	courtB := setUp(mB, owner, 150)
	a.expect(200, "", "POST", mB+"/publish", owner, nil)
	for _, b := range []struct {
		slug  string
		court int
	}{{"riverside-sisattanak", courtA}, {"riverside-xaythany", courtB}} {
		r := a.call("POST", "/public/branches/"+b.slug+"/bookings", customer, map[string]any{"resource_id": b.court, "starts_at": day.at(10, 0), "duration_min": 60})
		if r.status != 201 {
			t.Fatalf("booking at %s: %d %v", b.slug, r.status, r.body)
		}
	}
	mine := fmt.Sprint(a.expect(200, "", "GET", "/me/bookings/", customer, nil).body["data"])
	if !strings.Contains(mine, "Riverside Sisattanak") || !strings.Contains(mine, "Riverside Xaythany") {
		t.Fatalf("my bookings at both branches: %s", mine)
	}
	report := a.expect(200, "", "GET", "/company/report?from="+day.ymd()+"&to="+day.ymd(), owner, nil)
	if report.str("data.bookings") != "2" || report.str("data.totals.0.booked_value") != "250" || len(report.body["data"].(map[string]any)["branches"].([]any)) != 2 {
		t.Fatalf("company report: %v", report.body)
	}
	a.expect(403, "ROLE_NOT_ALLOWED", "GET", "/company/report?from="+day.ymd()+"&to="+day.ymd(), manager, nil)
	if c := a.expect(200, "", "GET", "/company/customers", admin, nil); c.str("meta.total") != "1" {
		t.Fatalf("company customers: %v", c.body)
	}
	if f := a.expect(200, "", "GET", mA+"/reports/finance?from="+day.ymd()+"&to="+day.ymd(), manager, nil); f.str("data.totals.0.booked_value") != "100" {
		t.Fatalf("branch A finance: %v", f.body)
	}

	// --- team changes take effect on the next request ---
	ids := map[string]string{}
	for _, raw := range a.expect(200, "", "GET", "/company/team", owner, nil).body["data"].([]any) {
		m := raw.(map[string]any)
		ids[fmt.Sprint(m["role"])+":"+fmt.Sprint(m["name"])] = fmt.Sprintf("%.0f", m["user_id"])
	}
	staffID, adminID, ownerID := ids["staff:staff"], ids["admin:admin"], ids["owner:Olga Owner"]
	if staffID == "" || adminID == "" || ownerID == "" {
		t.Fatalf("team: %v", ids)
	}
	a.expect(200, "", "PUT", "/company/team/"+staffID, admin, map[string]any{"role": "staff", "branch_ids": []int{num(idA)}, "active": false})
	a.expect(403, "MEMBERSHIP_DISABLED", "GET", mA+"/sports", staff, nil)
	a.expect(404, "BRANCH_NOT_FOUND", "GET", mB+"/sports", staff, nil)
	a.expect(200, "", "PUT", "/company/team/"+staffID, admin, map[string]any{"role": "staff", "branch_ids": []int{num(idA), num(idB)}, "active": true})
	a.expect(200, "", "GET", mB+"/sports", staff, nil)
	a.expect(400, "USE_TRANSFER_OWNERSHIP", "PUT", "/company/team/"+ownerID, admin, map[string]any{"role": "staff", "branch_ids": []int{num(idA)}})
	a.expect(403, "ONLY_THE_OWNER_MANAGES_ADMINS", "POST", "/company/invitations", admin, map[string]any{"email": "admin2@co.test", "role": "admin"})
	a.expect(400, "CANNOT_CHANGE_YOURSELF", "PUT", "/company/team/"+adminID, admin, map[string]any{"role": "staff", "branch_ids": []int{num(idA)}})
	a.expect(200, "", "PATCH", "/company/users/"+staffID+"/status", admin, map[string]any{"status": false})
	a.expect(403, "ACCOUNT_DISABLED", "GET", mA+"/sports", staff, nil)
	a.expect(200, "", "PATCH", "/company/users/"+staffID+"/status", admin, map[string]any{"status": true})
	a.expect(403, "NOT_ALLOWED_FOR_THE_OWNER", "PATCH", "/company/users/"+ownerID+"/status", admin, map[string]any{"status": false})
	a.expect(200, "PASSWORD_UPDATED", "POST", "/company/users/"+staffID+"/password", admin, map[string]any{"password": "brand-new-pass"})
	a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": "staff@co.test", "password": "brand-new-pass"})

	// --- closing a branch: offline for customers, locked for its team, kept for the owner ---
	a.expect(200, "", "POST", "/company/branches/"+idA+"/close", admin, nil)
	a.expect(404, "BRANCH_NOT_FOUND", "GET", "/public/branches/riverside-sisattanak/", "", nil)
	a.expect(403, "BRANCH_CLOSED", "GET", mA+"/sports", manager, nil)
	a.expect(200, "", "GET", mA+"/settings", owner, nil)
	a.expect(409, "BRANCH_CLOSED", "POST", mA+"/publish", owner, nil)
	if r := a.expect(200, "", "POST", "/company/branches/"+idA+"/reopen", owner, nil); r.str("data.status") != "draft" {
		t.Fatalf("reopen: %v", r.body)
	}
	a.expect(200, "", "GET", mA+"/sports", manager, nil)

	// --- handing over ownership: an admin becomes the owner, the old owner an admin ---
	a.expect(400, "NEW_OWNER_MUST_BE_AN_ADMIN", "POST", "/company/transfer-ownership", owner, map[string]any{"user_id": num(staffID)})
	a.expect(403, "ONLY_THE_OWNER", "POST", "/company/transfer-ownership", admin, map[string]any{"user_id": num(adminID)})
	a.expect(200, "OWNERSHIP_TRANSFERRED", "POST", "/company/transfer-ownership", owner, map[string]any{"user_id": num(adminID)})
	if me := a.expect(200, "", "GET", "/auth/me", admin, nil); me.str("data.company_role") != "owner" {
		t.Fatalf("new owner: %v", me.body)
	}
	a.expect(403, "ONLY_THE_OWNER_MANAGES_ADMINS", "POST", "/company/invitations", owner, map[string]any{"email": "admin3@co.test", "role": "admin"})

	// --- forgot / reset password ---
	a.expect(200, "IF_THE_ACCOUNT_EXISTS_AN_EMAIL_WAS_SENT", "POST", "/auth/forgot-password", "", map[string]any{"email": "nobody@test.la"})
	a.expect(200, "IF_THE_ACCOUNT_EXISTS_AN_EMAIL_WAS_SENT", "POST", "/auth/forgot-password", "", map[string]any{"email": "Cara@Test.la"})
	resetTok := linkFor(t, "cara@test.la", "/reset-password/")
	time.Sleep(1100 * time.Millisecond) // tokens carry whole seconds; make the old token clearly older
	a.expect(400, "PASSWORD_TOO_SHORT", "POST", "/auth/reset-password", "", map[string]any{"token": resetTok, "password": "short"})
	a.expect(200, "PASSWORD_UPDATED", "POST", "/auth/reset-password", "", map[string]any{"token": resetTok, "password": "new-password-456"})
	a.expect(400, "INVALID_OR_EXPIRED_TOKEN", "POST", "/auth/reset-password", "", map[string]any{"token": resetTok, "password": "another-pass-789"})
	a.expect(401, "TOKEN_REVOKED", "GET", "/auth/me", customer, nil)
	a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": "cara@test.la", "password": "new-password-456"})

	// --- rate limit: forgot-password allows 1000 per 15 minutes per client (raised for testing; 2 used above) ---
	for range 998 {
		a.expect(200, "", "POST", "/auth/forgot-password", "", map[string]any{"email": "nobody@test.la"})
	}
	a.expect(429, "RATE_LIMIT_EXCEEDED", "POST", "/auth/forgot-password", "", map[string]any{"email": "nobody@test.la"})
}

// A fresh install without a configured password emails the owner a "choose your password" link
func TestInstallWelcomesTheOwner(t *testing.T) {
	a := api{t: t, app: newAppWith(t, "")}
	tok := linkFor(t, ownerEmail, "/reset-password/")
	a.expect(401, "INVALID_CREDENTIALS", "POST", "/auth/login", "", map[string]any{"login": ownerEmail, "password": ownerPassword})
	a.expect(200, "PASSWORD_UPDATED", "POST", "/auth/reset-password", "", map[string]any{"token": tok, "password": "chosen-by-owner"})
	if me := a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": ownerEmail, "password": "chosen-by-owner"}); me.str("data.user.company_role") != "owner" {
		t.Fatalf("owner after welcome: %v", me.body)
	}
}
