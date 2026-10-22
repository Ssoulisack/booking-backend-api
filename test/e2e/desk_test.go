package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// daylightZone picks a timezone where it is now between 04:00 and 20:00, so bookings at
// 00:00-03:00 "today" have always ended and "tomorrow" is always in the future
func daylightZone(t *testing.T) *time.Location {
	for _, name := range []string{"Asia/Vientiane", "Europe/London", "America/New_York", "Pacific/Auckland"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			continue
		}
		if h := time.Now().In(loc).Hour(); h >= 4 && h <= 20 {
			return loc
		}
	}
	t.Fatal("no daylight timezone found")
	return nil
}

func dayIn(loc *time.Location, offset int) bookingDay {
	n := time.Now().In(loc)
	return bookingDay{loc: loc, date: time.Date(n.Year(), n.Month(), n.Day()+offset, 0, 0, 0, 0, loc)}
}

func TestDeskAndReports(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	loc := daylightZone(t)
	today, tmr, later := dayIn(loc, 0), dayIn(loc, 1), dayIn(loc, 2)
	admin := ownerToken(a) // the company owner (from the test config) runs the branches
	bizID, slug, owner := setupClub(t, a, admin, "Desk Club", "owner@desk.test")
	m, p := "/branches/"+bizID, "/public/branches/"+slug

	// --- set up: open all day, two courts at 200 LAK/h, a staff member ---
	updateSettings(a, m, owner, map[string]any{"timezone": loc.String(), "booking_mode": "both"})
	tennis := num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": "Tennis"}).str("data.id"))
	court := func(name string) int {
		return num(a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": tennis, "name": name, "kind": "court"}).str("data.id"))
	}
	court1, court2 := court("Court 1"), court("Court 2")
	allDay := []map[string]any{}
	for d := range 7 {
		allDay = append(allDay, map[string]any{"weekday": d, "open": "00:00", "close": "24:00"})
	}
	for _, c := range []int{court1, court2} {
		a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%d", m, c), owner, map[string]any{"hours": allDay})
		a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", m, c), owner, map[string]any{"price_per_hour": 200, "currency": "LAK"})
	}
	a.expect(200, "", "POST", m+"/publish", owner, nil)
	staff := teamMember(a, owner, "staff@desk.test", "staff", bizID)
	alice := register(a, "Alice", "alice@desk.test")
	bob := register(a, "Bob", "bob@desk.test")

	walkIn := map[string]any{"name": "Somsak", "phone": "020 7777 1111"}
	staffBook := func(body map[string]any) reply { return a.call("POST", m+"/bookings", staff, body) }
	rentalFor := func(resource int, start string, minutes int, extra map[string]any) map[string]any {
		body := map[string]any{"resource_id": resource, "starts_at": start, "duration_min": minutes}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	mustBook := func(r reply) string {
		t.Helper()
		if r.status != 201 {
			t.Fatalf("booking: %d %v", r.status, r.body)
		}
		return r.str("data.id")
	}
	status := func(id, to, reason string) reply {
		return a.call("POST", m+"/bookings/"+id+"/status", staff, map[string]any{"status": to, "reason": reason})
	}
	pay := func(id, method string) reply {
		return a.call("POST", m+"/bookings/"+id+"/payment", staff, map[string]any{"status": "paid", "method": method})
	}

	// --- staff free times: earlier today is offered to staff (not to customers), with prices ---
	free := func(base, token string) reply {
		return a.expect(200, "", "GET", fmt.Sprintf("%s/availability?date=%s&resource_id=%d", base, today.ymd(), court2), token, nil)
	}
	if staffFree := free(m, staff); !hasStart(staffFree, loc, "00:00") || staffFree.str("data.prices.0.price") != "200" {
		t.Fatalf("staff availability: %v", staffFree.body["data"])
	}
	if hasStart(free(p, ""), loc, "00:00") {
		t.Fatalf("customers must not be offered times that have passed")
	}
	if r := a.call("GET", fmt.Sprintf("%s/availability?date=%s&resource_id=%d", m, today.ymd(), court2), alice, nil); r.status != 404 {
		t.Fatalf("a customer must not use the desk availability: %d %v", r.status, r.body)
	}

	// --- staff bookings: past hours of today are fine for staff, not for customers ---
	w1 := staffBook(rentalFor(court1, today.at(0, 0), 60, map[string]any{"guest": walkIn}))
	if w1.status != 201 || w1.str("data.status") != "confirmed" || w1.str("data.created_by") != "staff" || w1.str("data.guest_name") != "Somsak" {
		t.Fatalf("walk-in: %d %v", w1.status, w1.body)
	}
	if r := a.call("POST", p+"/bookings", alice, rentalFor(court2, today.at(0, 0), 60, nil)); r.status != 409 {
		t.Fatalf("customer booking in the past: %d %v", r.status, r.body)
	}
	a1 := staffBook(rentalFor(court1, today.at(1, 0), 60, map[string]any{"customer_email": "Alice@Desk.test"}))
	if a1.status != 201 || a1.str("data.customer_email") != "alice@desk.test" {
		t.Fatalf("booking for a customer: %d %v", a1.status, a1.body)
	}
	if r := staffBook(rentalFor(court1, today.at(5, 0), 60, map[string]any{"customer_email": "nobody@desk.test"})); r.msg() != "CUSTOMER_NOT_FOUND" {
		t.Fatalf("unknown customer: %d %v", r.status, r.body)
	}
	if r := staffBook(rentalFor(court1, today.at(5, 0), 60, nil)); r.msg() != "CUSTOMER_OR_WALK_IN_REQUIRED" {
		t.Fatalf("nobody: %d %v", r.status, r.body)
	}
	if r := staffBook(rentalFor(court1, today.at(5, 15), 60, map[string]any{"guest": walkIn})); r.status != 409 {
		t.Fatalf("staff off the start grid: %d %v", r.status, r.body)
	}
	if r := staffBook(rentalFor(court1, today.at(5, 0), 240, map[string]any{"guest": walkIn})); r.msg() != "INVALID_DURATION" {
		t.Fatalf("staff over the max length: %d %v", r.status, r.body)
	}
	if r := staffBook(rentalFor(court1, today.at(0, 30), 60, map[string]any{"guest": walkIn})); r.status != 409 {
		t.Fatalf("staff overlap: %d %v", r.status, r.body)
	}
	w2 := mustBook(staffBook(rentalFor(court2, today.at(0, 0), 120, map[string]any{"guest": walkIn})))
	w3 := mustBook(staffBook(rentalFor(court2, today.at(2, 0), 60, map[string]any{"guest": walkIn})))
	c1 := mustBook(staffBook(rentalFor(court1, today.at(2, 0), 60, map[string]any{"guest": walkIn})))
	a2 := mustBook(a.call("POST", p+"/bookings", alice, rentalFor(court1, tmr.at(10, 0), 60, nil)))
	mustBook(a.call("POST", p+"/bookings", bob, rentalFor(court2, tmr.at(10, 0), 60, nil)))

	// --- calendar shows the day per court, with names ---
	cal := a.expect(200, "", "GET", m+"/calendar?date="+today.ymd(), staff, nil)
	lanes := cal.body["data"].(map[string]any)["lanes"].([]any)
	court1Lane := fmt.Sprint(lanes[0])
	if !strings.Contains(court1Lane, "Somsak") || !strings.Contains(court1Lane, "Alice") || len(lanes) != 2 {
		t.Fatalf("calendar: %v", lanes)
	}
	a.expect(400, "DAYS_MUST_BE_1_TO_7", "GET", m+"/calendar?date="+today.ymd()+"&days=8", staff, nil)

	// --- status changes: one way only ---
	if r := status(w1.str("data.id"), "completed", ""); r.status != 200 || r.str("data.status") != "completed" {
		t.Fatalf("complete: %d %v", r.status, r.body)
	}
	for _, to := range []string{"pending", "cancelled", "confirmed"} {
		if r := status(w1.str("data.id"), to, ""); r.msg() != "INVALID_STATUS_CHANGE" {
			t.Fatalf("completed -> %s: %d %v", to, r.status, r.body)
		}
	}
	a.expect(200, "", "POST", m+"/bookings/"+a1.str("data.id")+"/status", staff, map[string]any{"status": "no_show"})
	if r := status(a2, "completed", ""); r.msg() != "BOOKING_NOT_STARTED" {
		t.Fatalf("complete a future booking: %d %v", r.status, r.body)
	}
	// staff cancel ignores the customer window (tomorrow is inside the default 24 h) and keeps the reason
	if r := status(a2, "cancelled", "court repair"); r.str("data.status") != "cancelled" || r.str("data.cancel_reason") != "court repair" {
		t.Fatalf("staff cancel: %d %v", r.status, r.body)
	}
	expectEmail(t, "alice@desk.test", "cancelled your booking", "Reason: court repair", "Desk Club")
	a.expect(200, "", "POST", m+"/bookings/"+c1+"/status", staff, map[string]any{"status": "cancelled", "reason": "double entry"})
	if r := status(c1, "nonsense", ""); r.msg() != "INVALID_STATUS_CHANGE" {
		t.Fatalf("unknown status: %d %v", r.status, r.body)
	}

	// --- payments at the venue ---
	if r := pay(w1.str("data.id"), "cash"); r.str("data.payment_status") != "paid" || r.str("data.payment_method") != "cash" || r.str("data.paid_at") == "" {
		t.Fatalf("pay: %d %v", r.status, r.body)
	}
	if r := pay(w3, "cheque"); r.msg() != "INVALID_PAYMENT_METHOD" {
		t.Fatalf("bad method: %d %v", r.status, r.body)
	}
	if r := pay(c1, "cash"); r.msg() != "BOOKING_CANCELLED" {
		t.Fatalf("pay a cancelled booking: %d %v", r.status, r.body)
	}
	pay(w3, "qr")
	pay(w2, "cash") // marked by mistake...
	if r := a.call("POST", m+"/bookings/"+w2+"/payment", staff, map[string]any{"status": "unpaid"}); r.str("data.payment_status") != "unpaid" || r.str("data.paid_at") != "" {
		t.Fatalf("undo payment: %d %v", r.status, r.body) // ...and undone
	}

	// --- list and filters ---
	if r := a.expect(200, "", "GET", m+"/bookings?from="+today.ymd()+"&to="+today.ymd(), staff, nil); r.str("meta.total") != "5" {
		t.Fatalf("today's bookings: %v", r.body["meta"])
	}
	if r := a.expect(200, "", "GET", m+"/bookings?keyword=somsak&status=confirmed", staff, nil); r.str("meta.total") != "2" {
		t.Fatalf("keyword + status: %v", r.body["meta"])
	}
	if r := a.expect(200, "", "GET", fmt.Sprintf("%s/bookings?resource_id=%d&payment_status=paid", m, court2), staff, nil); r.str("meta.total") != "1" {
		t.Fatalf("resource + paid: %v", r.body["meta"])
	}

	// --- finance: booked = paid + outstanding + upcoming unpaid + lost to no-shows ---
	// W1 200 paid · A1 200 no-show · W2 400 outstanding · W3 200 paid · Bob tomorrow 200 upcoming
	// cancelled: C1 200 + Alice tomorrow 200
	period := "?from=" + today.ymd() + "&to=" + tmr.ymd()
	fin := a.expect(200, "", "GET", m+"/reports/finance"+period, owner, nil)
	totals := fin.body["data"].(map[string]any)["totals"].([]any)[0].(map[string]any)
	want := map[string]float64{"booked_value": 1200, "paid": 400, "outstanding": 400, "upcoming_unpaid": 200, "lost_no_show": 200, "lost_cancelled": 400}
	for k, v := range want {
		if totals[k] != v {
			t.Fatalf("finance %s = %v, want %v (all: %v)", k, totals[k], v, totals)
		}
	}
	unpaid := a.expect(200, "", "GET", m+"/reports/unpaid"+period, owner, nil).body["data"].([]any)
	if len(unpaid) != 1 || unpaid[0].(map[string]any)["booking_id"] != float64(num(w2)) {
		t.Fatalf("unpaid list: %v", unpaid)
	}

	// --- cash-up: money marked paid today, per method and staff member ---
	cash := a.expect(200, "", "GET", m+"/reports/cash-up?date="+today.ymd(), staff, nil)
	cd := cash.body["data"].(map[string]any)
	if fmt.Sprint(cd["totals"]) != "[map[amount:400 currency:LAK]]" || len(cd["by_method"].([]any)) != 2 ||
		!strings.Contains(fmt.Sprint(cd["by_staff"]), "name:staff") || len(cd["payments"].([]any)) != 2 {
		t.Fatalf("cash-up: %v", cd)
	}

	// --- bookings report, utilisation, CSV ---
	rep := a.expect(200, "", "GET", m+"/reports/bookings"+period, owner, nil).body["data"].(map[string]any)
	byStatus := fmt.Sprint(rep["by_status"])
	if rep["total"] != float64(7) || byStatus != "map[cancelled:2 completed:1 confirmed:3 no_show:1]" ||
		!strings.Contains(fmt.Sprint(rep["top_customers"].([]any)[0]), "Somsak") {
		t.Fatalf("bookings report: %v", rep)
	}
	use := a.expect(200, "", "GET", m+"/reports/utilisation"+period, owner, nil).body["data"].(map[string]any)["resources"].([]any)
	u1, u2 := use[0].(map[string]any), use[1].(map[string]any)
	if u1["open_minutes"] != float64(2880) || u1["booked_minutes"] != float64(120) || u2["booked_minutes"] != float64(240) || u2["percent"] != 8.3 {
		t.Fatalf("utilisation: %v", use)
	}
	csv := a.call("GET", m+"/reports/finance"+period+"&format=csv", owner, nil)
	if csv.status != 200 {
		t.Fatalf("csv: %d", csv.status)
	}
	raw := a.raw("GET", m+"/reports/unpaid"+period+"&format=csv", owner)
	if !strings.HasPrefix(raw, "code,starts_at,status,name") || !strings.Contains(raw, "Somsak") || !strings.Contains(raw, ",400,LAK") {
		t.Fatalf("unpaid csv: %q", raw)
	}

	// --- permissions and isolation ---
	a.expect(403, "ROLE_NOT_ALLOWED", "GET", m+"/reports/finance"+period, staff, nil)
	a.expect(403, "ROLE_NOT_ALLOWED", "GET", m+"/reports/bookings"+period, staff, nil)
	otherID, otherSlug, otherOwner := setupClub(t, a, admin, "Other Desk", "owner@otherdesk.test")
	otherManager := teamMember(a, owner, "manager@otherdesk.test", "manager", otherID)
	a.expect(404, "BRANCH_NOT_FOUND", "GET", m+"/bookings", otherManager, nil)
	a.expect(404, "BRANCH_NOT_FOUND", "GET", m+"/reports/cash-up?date="+today.ymd(), otherManager, nil)
	a.expect(404, "BRANCH_NOT_FOUND", "GET", m+"/bookings", alice, nil)

	// --- customers: only those who booked here ---
	// Carol books only at the other club
	om := "/branches/" + otherID
	os := num(a.expect(201, "", "POST", om+"/sports", otherOwner, map[string]any{"name": "Tennis"}).str("data.id"))
	oc := num(a.expect(201, "", "POST", om+"/resources", otherOwner, map[string]any{"sport_id": os, "name": "Court A", "kind": "court"}).str("data.id"))
	a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%d", om, oc), otherOwner, map[string]any{"hours": allDay})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", om, oc), otherOwner, map[string]any{"price_per_hour": 100, "currency": "LAK"})
	a.expect(200, "", "POST", om+"/publish", otherOwner, nil)
	carol := register(a, "Carol", "carol@desk.test")
	mustBook(a.call("POST", "/public/branches/"+otherSlug+"/bookings", carol, rentalFor(oc, tmr.at(9, 0), 60, nil)))
	customers := a.expect(200, "", "GET", m+"/customers", staff, nil)
	list := fmt.Sprint(customers.body["data"])
	if customers.str("meta.total") != "2" || !strings.Contains(list, "alice@desk.test") || !strings.Contains(list, "bob@desk.test") || strings.Contains(list, "carol") {
		t.Fatalf("customers: %v", customers.body)
	}

	// --- approval: pending until staff confirm ---
	updateSettings(a, m, owner, map[string]any{"require_approval": true})
	pending := a.call("POST", p+"/bookings", bob, rentalFor(court1, later.at(9, 0), 60, nil))
	if pending.str("data.status") != "pending" {
		t.Fatalf("approval: %v", pending.body)
	}
	if r := status(pending.str("data.id"), "confirmed", ""); r.str("data.status") != "confirmed" {
		t.Fatalf("confirm: %d %v", r.status, r.body)
	}
	expectEmail(t, "bob@desk.test", "confirmed your booking", pending.str("data.code"))
	if r := staffBook(rentalFor(court1, later.at(11, 0), 60, map[string]any{"guest": walkIn})); r.str("data.status") != "confirmed" {
		t.Fatalf("staff bookings skip approval: %v", r.body)
	}
}
