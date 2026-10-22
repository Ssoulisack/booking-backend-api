package e2e

import (
	"fmt"
	"testing"
	"time"
)

// nextMonday is the Monday of next week in Vientiane (always at least 1 day ahead)
func nextMonday() bookingDay {
	loc, _ := time.LoadLocation("Asia/Vientiane")
	now := time.Now().In(loc)
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	for d.Weekday() != time.Monday {
		d = d.AddDate(0, 0, 1)
	}
	return bookingDay{loc: loc, date: d}
}

func (d bookingDay) plus(days int) bookingDay {
	return bookingDay{loc: d.loc, date: d.date.AddDate(0, 0, days)}
}

func TestRepeatingClasses(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	mon := nextMonday()
	wed, thu2, fri2 := mon.plus(2), mon.plus(10), mon.plus(11)
	admin := ownerToken(a) // the company owner (from the test config) runs the branches
	bizID, slug, owner := setupClub(t, a, admin, "Fight Club", "owner@fight.test")
	m, p := "/branches/"+bizID, "/public/branches/"+slug

	boxing := num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": "Boxing"}).str("data.id"))
	tennis := num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": "Tennis"}).str("data.id"))
	ring := num(a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": boxing, "name": "Ring", "kind": "ring"}).str("data.id"))
	// every kind of space can be added; anything else is refused
	for _, kind := range []string{"lane", "bay", "table", "room"} {
		if r := a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": boxing, "name": "Space " + kind, "kind": kind}); r.str("data.kind") != kind {
			t.Fatalf("kind %s: %v", kind, r.body)
		}
	}
	a.expect(400, "INVALID_RESOURCE_KIND", "POST", m+"/resources", owner, map[string]any{"sport_id": boxing, "name": "Stage", "kind": "stage"})
	coach := num(a.expect(201, "", "POST", m+"/coaches", owner, map[string]any{"name": "Kru Lek", "sport_ids": []int{boxing}}).str("data.id"))
	week := []map[string]any{}
	for d := range 7 {
		week = append(week, map[string]any{"weekday": d, "open": "06:00", "close": "22:00"})
	}
	a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%d", m, ring), owner, map[string]any{"hours": week})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", m, ring), owner, map[string]any{"price_per_hour": 100000, "currency": "LAK"})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/rules/sport/%d", m, boxing), owner, map[string]any{"max_duration_min": 60})
	a.expect(200, "", "POST", m+"/publish", owner, nil)

	// a holiday on the first Wednesday
	a.expect(201, "", "POST", m+"/closures", owner, map[string]any{"owner_type": "venue", "owner_id": 0,
		"starts_at": wed.at(0, 0), "ends_at": wed.plus(1).at(0, 0), "reason": "Holiday"})

	series := func(changes map[string]any) map[string]any {
		body := map[string]any{
			"sport_id": boxing, "coach_id": coach, "resource_id": ring, "title": "Evening boxing",
			"start_time": "18:00", "end_time": "20:00", "weekdays": []int{1, 2, 3, 4, 5},
			"from": mon.ymd(), "to": fri2.ymd(), "capacity": 12, "price": 50000, "currency": "LAK",
		}
		for k, v := range changes {
			body[k] = v
		}
		return body
	}
	repeat := func(body map[string]any) reply { return a.call("POST", m+"/class-sessions/repeat", owner, body) }
	classCount := func() string { return a.expect(200, "", "GET", m+"/class-sessions", owner, nil).str("meta.total") }

	// --- invalid requests ---
	for _, c := range []struct {
		changes map[string]any
		code    string
	}{
		{map[string]any{"weekdays": []int{1, 7}}, "INVALID_WEEKDAY"},
		{map[string]any{"weekdays": []int{}}, "WEEKDAYS_REQUIRED"},
		{map[string]any{"end_time": "17:00"}, "END_TIME_MUST_BE_AFTER_START_TIME"},
		{map[string]any{"start_time": "6pm"}, "INVALID_TIME_FORMAT"},
		{map[string]any{"from": fri2.ymd(), "to": mon.ymd()}, "TO_BEFORE_FROM"},
		{map[string]any{"from": mon.ymd(), "to": mon.ymd(), "weekdays": []int{0}}, "NO_SESSIONS_IN_RANGE"},
		{map[string]any{"sport_id": tennis}, "RESOURCE_SPORT_MISMATCH"},
		{map[string]any{"capacity": 0}, "CAPACITY_MUST_BE_AT_LEAST_1"},
	} {
		if r := repeat(series(c.changes)); r.status != 400 || r.msg() != c.code {
			t.Fatalf("%v: got %d %q, want %s", c.changes, r.status, r.msg(), c.code)
		}
	}

	// --- a booking on the second Thursday blocks the whole series: nothing is created ---
	walkIn := a.expect(201, "", "POST", m+"/bookings", owner, map[string]any{"resource_id": ring, "starts_at": thu2.at(18, 0),
		"duration_min": 60, "guest": map[string]any{"name": "Somsak", "phone": "020 7777 1111"}}).str("data.id")
	if r := repeat(series(nil)); r.status != 409 || r.msg() != "SLOT_NOT_AVAILABLE_ON_"+thu2.ymd() {
		t.Fatalf("clash: %d %v", r.status, r.body)
	}
	if n := classCount(); n != "0" {
		t.Fatalf("a refused series must create nothing, found %s classes", n)
	}

	// --- without the clash: 10 weekdays minus the holiday = 9 classes ---
	a.expect(200, "", "POST", m+"/bookings/"+walkIn+"/status", owner, map[string]any{"status": "cancelled", "reason": "moved"})
	res := a.expect(201, "", "POST", m+"/class-sessions/repeat", owner, series(nil)).body["data"].(map[string]any)
	skipped := res["skipped"].([]any)
	if res["created"] != float64(9) || len(skipped) != 1 || skipped[0].(map[string]any)["date"] != wed.ymd() {
		t.Fatalf("repeat: created %v, skipped %v", res["created"], skipped)
	}
	first := res["sessions"].([]any)[0].(map[string]any)
	if first["starts_at"] != mon.at(18, 0) || first["ends_at"] != mon.at(20, 0) {
		t.Fatalf("first session: %v", first)
	}

	// --- date filters ---
	count := func(path string) string { return a.expect(200, "", "GET", path, owner, nil).str("meta.total") }
	if n := count(p + "/class-sessions?date=" + mon.ymd()); n != "1" {
		t.Fatalf("public, one day: %s", n)
	}
	if n := count(p + "/class-sessions?from=" + mon.ymd() + "&to=" + mon.plus(4).ymd()); n != "4" { // Mon-Fri, holiday skipped
		t.Fatalf("public, first week: %s", n)
	}
	if n := count(m + "/class-sessions?date=" + wed.ymd()); n != "0" {
		t.Fatalf("manage, holiday: %s", n)
	}
	if n := count(m + "/class-sessions?from=" + mon.ymd() + "&to=" + fri2.ymd() + "&keyword=boxing"); n != "9" {
		t.Fatalf("manage, range + keyword: %s", n)
	}
	a.expect(400, "INVALID_DATE", "GET", p+"/class-sessions?date=next-monday", "", nil)
	a.expect(400, "TO_BEFORE_FROM", "GET", p+"/class-sessions?from="+fri2.ymd()+"&to="+mon.ymd(), "", nil)

	// --- each class still holds the ring: no rental at 18:00 or 19:00 on the first Monday ---
	slots := a.expect(200, "", "GET", fmt.Sprintf("%s/availability?date=%s&resource_id=%d", p, mon.ymd(), ring), "", nil)
	for _, hhmm := range []string{"18:00", "18:30", "19:00", "19:30"} {
		if hasStart(slots, mon.loc, hhmm) {
			t.Fatalf("%s is offered during the class", hhmm)
		}
	}
	if !hasStart(slots, mon.loc, "20:00") || !hasStart(slots, mon.loc, "17:00") {
		t.Fatal("the hours around the class must stay bookable")
	}

	// --- each session is independent: book seats in one, cancel another date ---
	ids := res["sessions"].([]any)
	customer := register(a, "Noy", "noy@fight.test")
	a.expect(201, "", "POST", p+"/bookings", customer, map[string]any{"class_session_id": ids[0].(map[string]any)["id"], "seats": 2})
	a.expect(200, "DELETED", "DELETE", fmt.Sprintf("%s/class-sessions/%.0f", m, ids[1].(map[string]any)["id"]), owner, nil)
}
