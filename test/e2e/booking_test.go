package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// bookingDay is tomorrow in Vientiane: far enough for the 60-minute notice, inside 30 days ahead
type bookingDay struct {
	loc  *time.Location
	date time.Time
}

func tomorrow() bookingDay {
	loc, _ := time.LoadLocation("Asia/Vientiane")
	d := time.Now().In(loc).AddDate(0, 0, 1)
	return bookingDay{loc: loc, date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)}
}

func (d bookingDay) ymd() string { return d.date.Format("2006-01-02") }

// at returns the time hh:mm on that day in RFC 3339, as clients send it
func (d bookingDay) at(h, m int) string {
	return d.date.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Format(time.RFC3339)
}

// setupClub opens a branch (draft) as the company owner and returns its id, slug and the owner's
// token, which has every right at every branch. The last argument is kept for the call sites.
func setupClub(t *testing.T, a api, owner, name, _ string) (id, slug, token string) {
	t.Helper()
	br := a.expect(201, "", "POST", "/company/branches", owner, map[string]any{"name": name})
	return br.str("data.id"), br.str("data.slug"), owner
}

// settings reads the branch settings, applies changes and saves them (PUT replaces all fields)
func updateSettings(a api, m, owner string, changes map[string]any) {
	s := a.expect(200, "", "GET", m+"/settings", owner, nil).body["data"].(map[string]any)
	body := map[string]any{}
	for _, k := range []string{"name", "slug", "cover_key", "description", "address", "province", "phone", "map_url",
		"timezone", "booking_mode", "booking_policy", "require_approval"} {
		body[k] = s[k]
	}
	for k, v := range changes {
		body[k] = v
	}
	a.expect(200, "", "PUT", m+"/settings", owner, body)
}

func register(a api, name, email string) string {
	return a.expect(201, "", "POST", "/auth/register", "", map[string]any{"name": name, "email": email, "password": "password123", "nationality": "TH"}).str("data.access_token")
}

func guest(n int) map[string]any {
	return map[string]any{"name": fmt.Sprintf("Guest %d", n), "phone": fmt.Sprintf("+856 20 5555 %04d", n)}
}

// firstStart returns the first free start (HH:MM, Vientiane) of an availability reply
func firstStart(r reply, loc *time.Location) string {
	slots := r.body["data"].(map[string]any)["slots"].([]any)
	if len(slots) == 0 {
		return ""
	}
	ts, _ := time.Parse(time.RFC3339, slots[0].(map[string]any)["starts_at"].(string))
	return ts.In(loc).Format("15:04")
}

func hasStart(r reply, loc *time.Location, hhmm string) bool {
	for _, s := range r.body["data"].(map[string]any)["slots"].([]any) {
		ts, _ := time.Parse(time.RFC3339, s.(map[string]any)["starts_at"].(string))
		if ts.In(loc).Format("15:04") == hhmm {
			return true
		}
	}
	return false
}

// race sends n requests at the same moment and counts the status codes
func race(t *testing.T, a api, n int, body func(i int) map[string]any, path string) map[int]int {
	t.Helper()
	var mu sync.Mutex
	var wg sync.WaitGroup
	counts := map[int]int{}
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := a.call("POST", path, "", body(i))
			mu.Lock()
			counts[r.status]++
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()
	return counts
}

func TestBookingFlow(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	day := tomorrow()
	admin := ownerToken(a) // the company owner (from the test config) runs the branches
	bizID, slug, owner := setupClub(t, a, admin, "Vientiane Sports Club", "owner@vsc.test")
	m, p := "/branches/"+bizID, "/public/branches/"+slug

	// --- set up: 3 sports, 4 places, a tennis coach, hours, prices, rules ---
	sport := func(name string) int {
		return num(a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": name}).str("data.id"))
	}
	tennis, soccer, boxing := sport("Tennis"), sport("Soccer"), sport("Boxing")
	place := func(sportID int, name, kind string) int {
		return num(a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": sportID, "name": name, "kind": kind}).str("data.id"))
	}
	court1, court2 := place(tennis, "Court 1", "court"), place(tennis, "Court 2", "court")
	pitch, ring := place(soccer, "Pitch", "pitch"), place(boxing, "Ring", "ring")
	coach := num(a.expect(201, "", "POST", m+"/coaches", owner, map[string]any{"name": "Somchai", "sport_ids": []int{tennis}}).str("data.id"))

	week := []map[string]any{}
	for d := range 7 {
		week = append(week, map[string]any{"weekday": d, "open": "08:00", "close": "22:00"})
	}
	for _, o := range []struct {
		kind  string
		id    int
		price int
	}{{"resource", court1, 200}, {"resource", court2, 200}, {"resource", pitch, 300}, {"resource", ring, 100}, {"coach", coach, 150}} {
		a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/%s/%d", m, o.kind, o.id), owner, map[string]any{"hours": week})
		a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/%s/%d", m, o.kind, o.id), owner, map[string]any{"price_per_hour": o.price, "currency": "LAK"})
	}
	// tennis up to 4 h, cancel 48 h ahead (so tomorrow's booking can no longer be cancelled);
	// soccer up to 90 min, boxing a fixed hour; both cancellable until they start
	a.expect(200, "", "PUT", fmt.Sprintf("%s/rules/sport/%d", m, tennis), owner, map[string]any{"max_duration_min": 240, "cancel_before_hours": 48})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/rules/sport/%d", m, soccer), owner, map[string]any{"max_duration_min": 90, "cancel_before_hours": 0})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/rules/sport/%d", m, boxing), owner, map[string]any{"max_duration_min": 60, "cancel_before_hours": 0})
	updateSettings(a, m, owner, map[string]any{"booking_mode": "both"})
	a.expect(200, "", "POST", m+"/publish", owner, nil)

	book := func(token string, body map[string]any) reply { return a.call("POST", p+"/bookings", token, body) }
	rental := func(resource any, start string, minutes int) map[string]any {
		return map[string]any{"resource_id": resource, "starts_at": start, "duration_min": minutes}
	}
	avail := func(query string) reply {
		return a.expect(200, "", "GET", p+"/availability?date="+day.ymd()+query, "", nil)
	}

	alice := register(a, "Alice", "alice@test.la")
	bob := register(a, "Bob", "bob@test.la")

	// --- 08:00-09:30 booked: 09:30 becomes the next free start; overlapping booking refused ---
	if s := firstStart(avail(fmt.Sprintf("&resource_id=%d", court1)), day.loc); s != "08:00" {
		t.Fatalf("first free start before booking = %s", s)
	}
	b1 := book(alice, rental(court1, day.at(8, 0), 90))
	if b1.status != 201 || b1.str("data.status") != "confirmed" || b1.str("data.total_price") != "300" || len(b1.str("data.code")) != 8 {
		t.Fatalf("booking: %d %v", b1.status, b1.body)
	}
	if s := firstStart(avail(fmt.Sprintf("&resource_id=%d", court1)), day.loc); s != "09:30" {
		t.Fatalf("first free start after 08:00-09:30 = %s, want 09:30", s)
	}
	if r := book(bob, rental(court1, day.at(8, 30), 60)); r.status != 409 || r.msg() != "SLOT_NOT_AVAILABLE" {
		t.Fatalf("overlap: %d %v", r.status, r.body)
	}
	if r := book(bob, rental(court1, day.at(9, 45), 60)); r.status != 409 { // off the 30-minute start grid
		t.Fatalf("off-grid start: %d %v", r.status, r.body)
	}

	// --- per-sport duration rules ---
	if r := book(bob, rental(ring, day.at(8, 0), 90)); r.status != 400 || r.msg() != "INVALID_DURATION" {
		t.Fatalf("boxing 90 min: %d %v", r.status, r.body)
	}
	if r := book(bob, rental(pitch, day.at(8, 0), 120)); r.status != 400 || r.msg() != "INVALID_DURATION" {
		t.Fatalf("soccer 120 min: %d %v", r.status, r.body)
	}
	if r := book(bob, rental(court2, day.at(8, 0), 240)); r.status != 201 || r.str("data.total_price") != "800" {
		t.Fatalf("tennis 240 min: %d %v", r.status, r.body)
	}

	// --- coach + court together: both reserved, prices added; the coach cannot be double-booked ---
	both := map[string]any{"resource_id": court2, "coach_id": coach, "starts_at": day.at(14, 0), "duration_min": 60}
	if r := book(bob, both); r.status != 201 || r.str("data.total_price") != "350" || len(r.body["data"].(map[string]any)["items"].([]any)) != 2 {
		t.Fatalf("coach + court: %d %v", r.status, r.body)
	}
	// --- prices before booking, names after: public courts and coaches show the hourly price, free times the
	// exact price of each length (same as the booking total); bookings name the sport, court and coach ---
	courts := a.expect(200, "", "GET", fmt.Sprintf("%s/resources?sport_id=%d", p, tennis), "", nil)
	if courts.str("data.0.price_per_hour") != "200" || courts.str("data.0.currency") != "LAK" {
		t.Fatalf("public court price: %v", courts.body)
	}
	if c := a.expect(200, "", "GET", p+"/coaches", "", nil); c.str("data.0.price_per_hour") != "150" {
		t.Fatalf("public coach price: %v", c.body)
	}
	lengths := avail(fmt.Sprintf("&resource_id=%d&coach_id=%d", court1, coach))
	if lengths.str("data.currency") != "LAK" || lengths.str("data.prices.0.duration_min") != "60" ||
		lengths.str("data.prices.0.price") != "350" || lengths.str("data.prices.1.price") != "525" {
		t.Fatalf("length prices (court 200 + coach 150 per hour): %v", lengths.body["data"])
	}
	bobs := a.expect(200, "", "GET", "/me/bookings", bob, nil)
	var named bool
	for _, b := range bobs.body["data"].([]any) {
		bk := b.(map[string]any)
		if bk["total_price"] != float64(350) {
			continue
		}
		items := bk["items"].([]any)
		names := []any{items[0].(map[string]any)["resource_name"], items[0].(map[string]any)["coach_name"],
			items[1].(map[string]any)["resource_name"], items[1].(map[string]any)["coach_name"]}
		named = bk["sport_name"] == "Tennis" && fmt.Sprint(names) == fmt.Sprint([]any{"Court 2", nil, nil, "Somchai"})
	}
	if !named {
		t.Fatalf("my bookings should name sport, court and coach: %v", bobs.body["data"])
	}

	coachOnly := map[string]any{"coach_id": coach, "sport_id": tennis, "starts_at": day.at(14, 30), "duration_min": 60}
	if r := book(alice, coachOnly); r.status != 409 || r.msg() != "SLOT_NOT_AVAILABLE" {
		t.Fatalf("busy coach: %d %v", r.status, r.body)
	}
	if r := book(alice, map[string]any{"resource_id": pitch, "coach_id": coach, "starts_at": day.at(16, 0), "duration_min": 60}); r.msg() != "COACH_DOES_NOT_TEACH_SPORT" {
		t.Fatalf("coach for the wrong sport: %d %v", r.status, r.body)
	}

	// --- cancellation window ---
	if r := a.call("POST", "/me/bookings/"+b1.str("data.id")+"/cancel", alice, map[string]any{}); r.status != 409 || r.msg() != "CANCELLATION_WINDOW_PASSED" {
		t.Fatalf("tennis cancel inside 48 h: %d %v", r.status, r.body)
	}
	soccerBooking := book(alice, rental(pitch, day.at(10, 0), 60))
	if soccerBooking.status != 201 || hasStart(avail(fmt.Sprintf("&resource_id=%d", pitch)), day.loc, "10:00") {
		t.Fatalf("soccer booking: %d %v", soccerBooking.status, soccerBooking.body)
	}
	a.expect(200, "", "POST", "/me/bookings/"+soccerBooking.str("data.id")+"/cancel", alice, map[string]any{"reason": "rain"})
	if !hasStart(avail(fmt.Sprintf("&resource_id=%d", pitch)), day.loc, "10:00") {
		t.Fatal("a cancelled booking must free its time")
	}
	a.expect(409, "BOOKING_NOT_ACTIVE", "POST", "/me/bookings/"+soccerBooking.str("data.id")+"/cancel", alice, map[string]any{})
	a.expect(404, "", "GET", "/me/bookings/"+soccerBooking.str("data.id"), bob, nil) // not Bob's booking

	// --- per-customer limit: 3 upcoming bookings (Alice has 1 active) ---
	for _, h := range []int{8, 9} {
		if r := book(alice, rental(ring, day.at(h, 0), 60)); r.status != 201 {
			t.Fatalf("ring %d:00: %d %v", h, r.status, r.body)
		}
	}
	if r := book(alice, rental(ring, day.at(10, 0), 60)); r.status != 409 || r.msg() != "BOOKING_LIMIT_REACHED" {
		t.Fatalf("4th booking: %d %v", r.status, r.body)
	}
	mine := a.expect(200, "", "GET", "/me/bookings", alice, nil)
	if mine.str("meta.total") != "4" || !strings.Contains(fmt.Sprint(mine.body["data"]), "Vientiane Sports Club") {
		t.Fatalf("my bookings: %v", mine.body)
	}

	// --- guests: private link, email, view and cancel ---
	g := book("", map[string]any{"resource_id": ring, "starts_at": day.at(12, 0), "duration_min": 60,
		"guest": map[string]any{"name": "Noy", "phone": "020 5555 1234", "email": "noy@guest.test"}})
	if g.status != 201 || g.str("data.guest_token") == "" || !strings.Contains(g.str("data.guest_link"), "/b/"+slug+"/booking/") {
		t.Fatalf("guest booking: %d %v", g.status, g.body)
	}
	tok := g.str("data.guest_token")
	if linkFor(t, "noy@guest.test", "/booking/") != tok {
		t.Fatal("the emailed link must carry the same token")
	}
	a.expect(200, "", "GET", p+"/bookings/"+tok, "", nil)
	a.expect(404, "BOOKING_NOT_FOUND", "GET", p+"/bookings/not-the-token", "", nil)
	a.expect(200, "", "POST", p+"/bookings/"+tok+"/cancel", "", map[string]any{})
	if r := book("", map[string]any{"resource_id": ring, "starts_at": day.at(13, 0), "duration_min": 60, "guest": map[string]any{"name": "X", "phone": "call me"}}); r.msg() != "INVALID_PHONE" {
		t.Fatalf("bad phone: %d %v", r.status, r.body)
	}
	if r := book("", rental(ring, day.at(13, 0), 60)); r.status != 400 || r.msg() != "GUEST_DETAILS_REQUIRED" {
		t.Fatalf("no login, no guest details: %d %v", r.status, r.body)
	}
	updateSettings(a, m, owner, map[string]any{"booking_mode": "account"})
	if r := book("", map[string]any{"resource_id": ring, "starts_at": day.at(13, 0), "duration_min": 60, "guest": guest(1)}); r.status != 401 || r.msg() != "LOGIN_REQUIRED" {
		t.Fatalf("guest when only accounts allowed: %d %v", r.status, r.body)
	}
	updateSettings(a, m, owner, map[string]any{"booking_mode": "both"})

	// --- 10 guests race for the same hour: exactly one wins ---
	counts := race(t, a, 10, func(i int) map[string]any {
		return map[string]any{"resource_id": court2, "starts_at": day.at(18, 0), "duration_min": 60, "guest": guest(100 + i)}
	}, p+"/bookings")
	if counts[201] != 1 || counts[409] != 9 {
		t.Fatalf("race for one slot: %v", counts)
	}

	// --- group class: holds its court and coach, seats are limited, booked classes are protected ---
	class := a.expect(201, "", "POST", m+"/class-sessions", owner, map[string]any{
		"sport_id": tennis, "coach_id": coach, "resource_id": court1, "title": "Evening group",
		"starts_at": day.at(19, 0), "ends_at": day.at(20, 0), "capacity": 3, "price": 50, "currency": "LAK"})
	classID := num(class.str("data.id"))
	if r := book(bob, rental(court1, day.at(19, 0), 60)); r.status != 409 {
		t.Fatalf("court during the class: %d %v", r.status, r.body)
	}
	two := book("", map[string]any{"class_session_id": classID, "seats": 2, "guest": guest(200)})
	if two.status != 201 || two.str("data.total_price") != "100" {
		t.Fatalf("2 seats: %d %v", two.status, two.body)
	}
	if r := book("", map[string]any{"class_session_id": classID, "seats": 2, "guest": guest(201)}); r.msg() != "CLASS_FULL" {
		t.Fatalf("over capacity: %d %v", r.status, r.body)
	}
	if left := a.expect(200, "", "GET", p+"/class-sessions", "", nil).body["data"].([]any)[0].(map[string]any)["seats_left"]; left != float64(1) {
		t.Fatalf("seats_left = %v, want 1", left)
	}
	counts = race(t, a, 5, func(i int) map[string]any {
		return map[string]any{"class_session_id": classID, "seats": 1, "guest": guest(300 + i)}
	}, p+"/bookings")
	if counts[201] != 1 || counts[409] != 4 {
		t.Fatalf("race for the last seat: %v", counts)
	}
	a.expect(409, "CLASS_HAS_BOOKINGS", "DELETE", fmt.Sprintf("%s/class-sessions/%d", m, classID), owner, nil)
	a.expect(409, "CLASS_HAS_BOOKINGS", "PUT", fmt.Sprintf("%s/class-sessions/%d", m, classID), owner, map[string]any{
		"sport_id": tennis, "coach_id": coach, "resource_id": court1, "title": "Evening group",
		"starts_at": day.at(20, 0), "ends_at": day.at(21, 0), "capacity": 3, "price": 50, "currency": "LAK"})

	// --- deleting things with bookings or classes is refused ---
	a.expect(409, "USED_BY_CLASS_SESSIONS", "DELETE", fmt.Sprintf("%s/resources/%d", m, court1), owner, nil)
	a.expect(409, "HAS_UPCOMING_BOOKINGS", "DELETE", fmt.Sprintf("%s/resources/%d", m, court2), owner, nil)

	// --- a venue closure removes those hours ---
	a.expect(201, "", "POST", m+"/closures", owner, map[string]any{"owner_type": "venue", "owner_id": 0,
		"starts_at": day.at(20, 0), "ends_at": day.at(22, 0), "reason": "Event"})
	for _, hhmm := range []string{"20:00", "20:30", "21:00"} {
		if hasStart(avail(fmt.Sprintf("&resource_id=%d", pitch)), day.loc, hhmm) {
			t.Fatalf("%s offered during a closure", hhmm)
		}
	}

	// --- branches that approve bookings get pending ones ---
	updateSettings(a, m, owner, map[string]any{"require_approval": true})
	if r := book(bob, rental(pitch, day.at(15, 0), 60)); r.status != 201 || r.str("data.status") != "pending" {
		t.Fatalf("approval: %d %v", r.status, r.body)
	}

	// --- another branch's page cannot reach this club's courts ---
	otherID, otherSlug, otherOwner := setupClub(t, a, admin, "Other Club", "owner@other.test")
	om := "/branches/" + otherID
	otherSport := num(a.expect(201, "", "POST", om+"/sports", otherOwner, map[string]any{"name": "Tennis"}).str("data.id"))
	otherCourt := num(a.expect(201, "", "POST", om+"/resources", otherOwner, map[string]any{"sport_id": otherSport, "name": "Court A", "kind": "court"}).str("data.id"))
	a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%d", om, otherCourt), otherOwner, map[string]any{"hours": week})
	a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%d", om, otherCourt), otherOwner, map[string]any{"price_per_hour": 100, "currency": "LAK"})
	a.expect(200, "", "POST", om+"/publish", otherOwner, nil)
	op := "/public/branches/" + otherSlug
	a.expect(400, "RESOURCE_NOT_FOUND", "GET", op+"/availability?date="+day.ymd()+fmt.Sprintf("&resource_id=%d", court2), "", nil)
	a.expect(400, "RESOURCE_NOT_FOUND", "POST", op+"/bookings", bob, rental(court2, day.at(20, 0), 60))
	a.expect(400, "COACH_NOT_FOUND", "POST", op+"/bookings", bob, map[string]any{"coach_id": coach, "sport_id": otherSport, "starts_at": day.at(8, 0), "duration_min": 60})
	a.expect(400, "CLASS_NOT_FOUND", "POST", op+"/bookings", bob, map[string]any{"class_session_id": classID})
	a.expect(201, "", "POST", op+"/bookings", bob, rental(otherCourt, day.at(8, 0), 60)) // its own court works
}
