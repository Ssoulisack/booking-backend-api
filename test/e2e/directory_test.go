package e2e

import (
	"fmt"
	"net/url"
	"testing"
)

// 6f: the public list of places to play — only published branches, filtered by sport type,
// province and name or address, each with its sports and lowest court price
func TestDirectory(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	admin := ownerToken(a) // the company owner (from the test config) runs the branches

	week := []map[string]any{}
	for d := range 7 {
		week = append(week, map[string]any{"weekday": d, "open": "08:00", "close": "22:00"})
	}
	// club makes a branch with one sport and one priced court; publish puts it in the list
	club := func(name, email, sportName, sportType, province, address string, price int, publish bool) (string, string) {
		id, slug, owner := setupClub(t, a, admin, name, email)
		m := "/branches/" + id
		sport := a.expect(201, "", "POST", m+"/sports", owner, map[string]any{"name": sportName, "type": sportType})
		if sport.str("data.type") != sportType {
			t.Fatalf("sport type not kept: %v", sport.body)
		}
		court := a.expect(201, "", "POST", m+"/resources", owner, map[string]any{"sport_id": num(sport.str("data.id")), "name": "Court A", "kind": "court"}).str("data.id")
		a.expect(200, "", "PUT", fmt.Sprintf("%s/opening-hours/resource/%s", m, court), owner, map[string]any{"hours": week})
		a.expect(200, "", "PUT", fmt.Sprintf("%s/prices/resource/%s", m, court), owner, map[string]any{"price_per_hour": price, "currency": "LAK"})
		updateSettings(a, m, owner, map[string]any{"province": province, "address": address})
		if publish {
			a.expect(200, "", "POST", m+"/publish", owner, nil)
		}
		return id, slug
	}
	tennisID, tennis := club("Riverside Tennis", "owner@riverside.test", "Court tennis VIP", "tennis", "luang_prabang", "Ban Xieng Mouane", 200, true)
	_, football := club("Lao Kickers", "owner@kickers.test", "ບານເຕະ", "football", "vientiane_capital", "Sisattanak", 300, true)
	club("Hidden Draft", "owner@draft.test", "Tennis", "tennis", "luang_prabang", "Ban Xieng Mouane", 100, false)

	list := func(query string) []string {
		r := a.expect(200, "", "GET", "/public/branches?"+query, "", nil)
		slugs := []string{}
		for _, b := range r.body["data"].([]any) {
			slugs = append(slugs, b.(map[string]any)["slug"].(string))
		}
		return slugs
	}
	same := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	if got := list("sport_type=tennis"); !same(got, tennis) {
		t.Fatalf("tennis places (draft hidden): %v", got)
	}
	if got := list("province=vientiane_capital"); !same(got, football) {
		t.Fatalf("Vientiane Capital places: %v", got)
	}
	if got := list("keyword=" + url.QueryEscape("xieng mouane")); !same(got, tennis) {
		t.Fatalf("search by address: %v", got)
	}
	if got := list("sport_type=football&province=luang_prabang"); !same(got) {
		t.Fatalf("both filters: %v", got)
	}

	// each place shows its sports (name and type) and the lowest court price
	r := a.expect(200, "", "GET", "/public/branches?sport_type=tennis", "", nil)
	if r.str("data.0.sports.0.type") != "tennis" || r.str("data.0.sports.0.name") != "Court tennis VIP" ||
		r.str("data.0.from_price") != "200" || r.str("data.0.province") != "luang_prabang" {
		t.Fatalf("place details: %v", r.body)
	}

	// demo pictures: a placehold.co link is accepted as an image and shown as it is; other links are not
	logo := "https://placehold.co/400x400/0E7C66/FFFFFF.png?text=PC"
	a.expect(200, "", "PUT", "/company/", admin, map[string]any{
		"name": "E2E Sports Co", "currency": "LAK", "language": "en", "timezone": "Asia/Vientiane", "booking_mode": "account", "logo_key": logo,
	})
	if got := a.expect(200, "", "GET", "/public/company", "", nil).str("data.logo_url"); got != logo {
		t.Fatalf("placeholder logo url: %s", got)
	}
	phID, _, ph := setupClub(t, a, admin, "Placeholder Club", "owner@placeholder.test")
	a.expect(400, "INVALID_FILE_KEY", "POST", "/branches/"+phID+"/sports", ph, map[string]any{"name": "Golf", "type": "golf", "image_key": "https://example.com/x.png"})

	// wrong values are refused
	a.expect(400, "INVALID_SPORT_TYPE", "GET", "/public/branches?sport_type=quidditch", "", nil)
	a.expect(400, "INVALID_PROVINCE", "GET", "/public/branches?province=narnia", "", nil)

	// a closed branch leaves the list
	a.expect(200, "", "POST", "/company/branches/"+tennisID+"/close", admin, nil)
	if got := list("sport_type=tennis"); !same(got) {
		t.Fatalf("closed still listed: %v", got)
	}
}
