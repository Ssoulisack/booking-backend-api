package e2e

import (
	"fmt"
	"testing"
)

// 2f: a branch can be started as a copy of another one: its setup comes along (details, sports,
// spaces, coaches, hours, prices, rules, weekly classes, closed days to come) and nothing that
// happened there (bookings, made classes); the copy is a draft with its own ids
func TestCloneBranch(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	mon := nextMonday()
	admin := ownerToken(a)
	srcID, _, owner := setupClub(t, a, admin, "Clone Source", "")
	m := "/branches/" + srcID

	updateSettings(a, m, owner, map[string]any{"address": "Ban Nongbone", "province": "vientiane_capital", "phone": "+856 20 1111 2222"})
	boxing := num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": "Boxing"}).str("data.id"))
	ring := num(a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": boxing, "name": "Ring", "kind": "ring"}).str("data.id"))
	week := []map[string]any{}
	for d := range 7 {
		week = append(week, map[string]any{"weekday": d, "open": "06:00", "close": "22:00"})
	}
	a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%d", m, ring), owner, map[string]any{"hours": week})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", m, ring), owner, map[string]any{"price_per_hour": 100000, "currency": "LAK"})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/rules/sport/%d", m, boxing), owner, map[string]any{"max_duration_min": 90})
	coach := num(a.expect(201, "", "POST", m+"/coaches", owner, map[string]any{"name": "Coach Kham", "sport_ids": []int{boxing}}).str("data.id"))
	a.expect(201, "", "POST", m+"/class-timetables", owner, map[string]any{
		"sport_id": boxing, "resource_id": ring, "coach_id": coach, "title": "Morning boxing",
		"start_time": "08:00", "end_time": "09:00", "weekdays": []int{1, 2, 3, 4, 5, 6},
		"starts_on": mon.ymd(), "ends_on": "", "capacity": 6, "price": 50000, "currency": "LAK"})
	a.expect(201, "", "POST", m+"/closures", owner, map[string]any{"owner_type": "venue", "owner_id": 0,
		"starts_at": mon.plus(20).at(0, 0), "ends_at": mon.plus(21).at(0, 0), "reason": "Festival"})
	a.expect(200, "", "POST", m+"/publish", owner, nil)

	// --- the copy ---
	a.expect(404, "BRANCH_NOT_FOUND", "POST", "/company/branches", admin, map[string]any{"name": "Nope", "clone_from": 999999})
	cp := a.expect(201, "", "POST", "/company/branches", admin, map[string]any{"name": "Clone Source-COPY", "clone_from": num(srcID)})
	if cp.str("data.status") != "draft" || cp.str("data.slug") != "clone-source-copy" || cp.str("data.province") != "vientiane_capital" {
		t.Fatalf("the copy is a draft with the same area: %v", cp.body)
	}
	c := "/branches/" + cp.str("data.id")

	if got := a.expect(200, "", "GET", c+"/settings", owner, nil); got.str("data.address") != "Ban Nongbone" || got.str("data.phone") != "+856 20 1111 2222" {
		t.Fatalf("details copied: %v", got.body)
	}
	sports := a.expect(200, "", "GET", c+"/sports", owner, nil)
	newSport := num(sports.str("data.0.id"))
	if sports.str("meta.total") != "1" || sports.str("data.0.name") != "Boxing" || newSport == boxing {
		t.Fatalf("sports copied with new ids: %v", sports.body)
	}
	spaces := a.expect(200, "", "GET", c+"/resources", owner, nil)
	newRing := num(spaces.str("data.0.id"))
	if spaces.str("meta.total") != "1" || spaces.str("data.0.name") != "Ring" || spaces.str("data.0.sport_id") != fmt.Sprint(newSport) {
		t.Fatalf("space points at the copied sport: %v", spaces.body)
	}
	if got := a.expect(200, "", "GET", fmt.Sprintf("%s/prices/resource/%d", c, newRing), owner, nil); got.str("data.price_per_hour") != "100000" {
		t.Fatalf("price copied: %v", got.body)
	}
	if got := a.expect(200, "", "GET", fmt.Sprintf("%s/opening-hours/resource/%d", c, newRing), owner, nil); got.str("data.6.close") != "22:00" {
		t.Fatalf("hours copied: %v", got.body)
	}
	if got := a.expect(200, "", "GET", fmt.Sprintf("%s/rules/sport/%d", c, newSport), owner, nil); got.str("data.max_duration_min") != "90" {
		t.Fatalf("rule copied to the copied sport: %v", got.body)
	}
	if got := a.expect(200, "", "GET", c+"/coaches", owner, nil); got.str("data.0.name") != "Coach Kham" || got.str("data.0.sports.0.id") != fmt.Sprint(newSport) {
		t.Fatalf("coach copied with the copied sport: %v", got.body)
	}
	tt := a.expect(200, "", "GET", c+"/class-timetables", owner, nil)
	if tt.str("data.1.id") != "" || tt.str("data.0.sport_id") != fmt.Sprint(newSport) || tt.str("data.0.resource_id") != fmt.Sprint(newRing) {
		t.Fatalf("weekly class copied: %v", tt.body)
	}
	if got := a.expect(200, "", "GET", c+"/closures", owner, nil); got.str("data.0.reason") != "Festival" || got.str("data.1.id") != "" {
		t.Fatalf("closed day copied: %v", got.body)
	}

	// --- the source is untouched, and a change in the copy does not reach it ---
	a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", c, newRing), owner, map[string]any{"price_per_hour": 1, "currency": "LAK"})
	if got := a.expect(200, "", "GET", fmt.Sprintf("%s/prices/resource/%d", m, ring), owner, nil); got.str("data.price_per_hour") != "100000" {
		t.Fatalf("the source price: %v", got.body)
	}
	if got := a.expect(200, "", "GET", m+"/sports", owner, nil); got.str("meta.total") != "1" {
		t.Fatalf("the source sports: %v", got.body)
	}
}
