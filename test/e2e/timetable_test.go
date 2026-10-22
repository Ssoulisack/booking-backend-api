package e2e

import (
	"backend/cmd/application"
	"fmt"
	"testing"
	"time"
)

// 2d: a weekly timetable keeps making its classes 60 days ahead; closed days skip and clear free
// classes; editing and stopping never touch classes that already have bookings
func TestClassTimetables(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	mon := nextMonday()
	tue, wed := mon.plus(1), mon.plus(2)
	admin := ownerToken(a)
	bizID, slug, owner := setupClub(t, a, admin, "Timetable Club", "")
	m, p := "/branches/"+bizID, "/public/branches/"+slug

	boxing := num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": "Boxing"}).str("data.id"))
	ring := num(a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": boxing, "name": "Ring", "kind": "ring"}).str("data.id"))
	week := []map[string]any{}
	for d := range 7 {
		week = append(week, map[string]any{"weekday": d, "open": "06:00", "close": "22:00"})
	}
	a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%d", m, ring), owner, map[string]any{"hours": week})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", m, ring), owner, map[string]any{"price_per_hour": 100000, "currency": "LAK"})
	a.expect(200, "", "POST", m+"/publish", owner, nil)
	customer := register(a, "Tia Timetable", "tia@timetable.test")

	body := func(changes map[string]any) map[string]any {
		b := map[string]any{
			"sport_id": boxing, "resource_id": ring, "title": "Morning boxing",
			"start_time": "08:00", "end_time": "09:00", "weekdays": []int{1, 2, 3, 4, 5, 6},
			"starts_on": mon.ymd(), "ends_on": "", "capacity": 6, "price": 50000, "currency": "LAK",
		}
		for k, v := range changes {
			b[k] = v
		}
		return b
	}
	// Mon–Sat from next Monday up to today + 60 days
	today := tomorrow().plus(-1)
	until := today.plus(60)
	expected := 0
	for d := mon.date; !d.After(until.date); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Sunday {
			expected++
		}
	}
	classOn := func(d bookingDay) reply {
		return a.expect(200, "", "GET", m+"/class-sessions?date="+d.ymd(), owner, nil)
	}

	// --- invalid ---
	a.expect(400, "WEEKDAYS_REQUIRED", "POST", m+"/class-timetables", owner, body(map[string]any{"weekdays": []int{}}))
	a.expect(400, "TO_BEFORE_FROM", "POST", m+"/class-timetables", owner, body(map[string]any{"ends_on": today.ymd()}))
	a.expect(400, "END_TIME_MUST_BE_AFTER_START_TIME", "POST", m+"/class-timetables", owner, body(map[string]any{"end_time": "07:00"}))

	// --- Mon–Sat with no end: 60 days of classes at once ---
	made := a.expect(201, "", "POST", m+"/class-timetables", owner, body(nil))
	id := made.str("data.timetable.id")
	if made.str("data.created") != fmt.Sprint(expected) || made.str("data.timetable.filled_until") != until.ymd() ||
		made.str("data.timetable.ends_on") != "" {
		t.Fatalf("timetable create (want %d up to %s): %v", expected, until.ymd(), made.body)
	}
	if l := a.expect(200, "", "GET", m+"/class-timetables", owner, nil); l.str("data.0.id") != id || l.str("data.0.weekdays.5") != "6" {
		t.Fatalf("timetable list: %v", l.body)
	}
	if c := classOn(mon); c.str("data.0.timetable_id") != id || c.str("data.0.title") != "Morning boxing" {
		t.Fatalf("class made from the timetable: %v", c.body)
	}
	if c := classOn(mon.plus(6)); c.str("meta.total") != "0" {
		t.Fatalf("no class on Sunday: %v", c.body)
	}

	// --- the hourly roll forward: as if the window had only been made two weeks ahead ---
	cut := mon.plus(13)
	rewind := func() {
		dbExec(t, `DELETE FROM booking_items WHERE class_session_id IN
			(SELECT id FROM class_sessions WHERE timetable_id = ? AND starts_at >= ?)`, num(id), cut.plus(1).at(0, 0))
		dbExec(t, "DELETE FROM class_sessions WHERE timetable_id = ? AND starts_at >= ?", num(id), cut.plus(1).at(0, 0))
		dbExec(t, "UPDATE class_timetables SET filled_until = ? WHERE id = ?", cut.ymd(), num(id))
	}
	classCount := func() string {
		return a.expect(200, "", "GET", m+"/class-sessions?limit=1", owner, nil).str("meta.total")
	}
	rewind()
	if classCount() != "12" { // two weeks of Mon–Sat
		t.Fatalf("rewound to two weeks: %s", classCount())
	}
	// a closed branch is left alone
	a.expect(200, "", "POST", "/company/branches/"+bizID+"/close", admin, nil)
	if err := application.RollTimetables(current.services); err != nil || classCount() != "12" {
		t.Fatalf("closed branch rolled: %v %s", err, classCount())
	}
	// reopened: the next roll makes the missing weeks again
	a.expect(200, "", "POST", "/company/branches/"+bizID+"/reopen", admin, nil)
	if err := application.RollTimetables(current.services); err != nil || classCount() != fmt.Sprint(expected) {
		t.Fatalf("roll forward (want %d): %v %s", expected, err, classCount())
	}
	a.expect(200, "", "POST", m+"/publish", owner, nil)

	// --- a closed Tuesday and Wednesday: the free Wednesday class goes, the booked Tuesday stays ---
	tueClass := classOn(tue).str("data.0.id")
	a.expect(201, "", "POST", p+"/bookings", customer, map[string]any{"class_session_id": num(tueClass), "seats": 1})
	closed := a.expect(201, "", "POST", m+"/closures", owner, map[string]any{"owner_type": "venue", "owner_id": 0,
		"starts_at": tue.at(0, 0), "ends_at": wed.plus(1).at(0, 0), "reason": "Festival"})
	if closed.str("data.classes_removed") != "1" || closed.str("data.classes_kept.0.id") != tueClass {
		t.Fatalf("closed days: %v", closed.body)
	}
	if c := classOn(wed); c.str("meta.total") != "0" {
		t.Fatalf("free class on a closed day should be gone: %v", c.body)
	}

	// --- a new time: free classes are made again at 18:00, the booked Tuesday keeps 08:00 ---
	moved := a.expect(200, "", "PUT", m+"/class-timetables/"+id, owner, body(map[string]any{"start_time": "18:00", "end_time": "19:00"}))
	if moved.str("data.kept_booked") != "1" || moved.str("data.created") != fmt.Sprint(expected-2) {
		t.Fatalf("timetable edit (want %d made, 1 kept): %v", expected-2, moved.body)
	}
	if c := classOn(mon); c.str("data.0.starts_at") != mon.at(18, 0) {
		t.Fatalf("Monday moved to 18:00: %v", c.body)
	}
	if c := classOn(tue); c.str("data.0.id") != tueClass || c.str("data.0.starts_at") != tue.at(8, 0) {
		t.Fatalf("booked Tuesday unchanged: %v", c.body)
	}

	// --- stop: free upcoming classes are removed, the booked one stays as an ordinary class ---
	stopped := a.expect(200, "", "DELETE", m+"/class-timetables/"+id, owner, nil)
	if stopped.str("data.removed") != fmt.Sprint(expected-2) || stopped.str("data.kept_booked") != "1" {
		t.Fatalf("timetable stop: %v", stopped.body)
	}
	if l := a.expect(200, "", "GET", m+"/class-timetables", owner, nil); len(l.body["data"].([]any)) != 0 {
		t.Fatalf("timetable gone: %v", l.body)
	}
	if c := classOn(tue); c.str("data.0.id") != tueClass {
		t.Fatalf("booked class kept after stop: %v", c.body)
	}
	a.expect(404, "TIMETABLE_NOT_FOUND", "DELETE", m+"/class-timetables/"+id, owner, nil)

	// --- an end date: one week only; Tuesday and Wednesday are closed ---
	week1 := a.expect(201, "", "POST", m+"/class-timetables", owner, body(map[string]any{"start_time": "12:00", "end_time": "13:00", "ends_on": mon.plus(6).ymd()}))
	if week1.str("data.created") != "4" || week1.str("data.skipped.0.reason") != "closed" || week1.str("data.timetable.filled_until") != mon.plus(6).ymd() {
		t.Fatalf("one week with an end: %v", week1.body)
	}
	// the same time on the same ring: every day is taken (or closed), nothing is double-booked
	clash := a.expect(201, "", "POST", m+"/class-timetables", owner, body(map[string]any{"start_time": "12:00", "end_time": "13:00", "ends_on": mon.plus(6).ymd(), "title": "Clash"}))
	if clash.str("data.created") != "0" || clash.str("data.skipped.0.reason") != "taken" {
		t.Fatalf("taken days are skipped: %v", clash.body)
	}
}
