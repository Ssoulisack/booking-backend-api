package e2e

import "testing"

// 2g: the look of the booking pages: up to 4 ring colours and two fonts, set for the company and
// overridden by a branch ("same as company" = null); invalid colours and fonts are refused
func TestLookCompanyAndBranch(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	admin := ownerToken(a)
	id, _, owner := setupClub(t, a, admin, "Look Club", "")
	m := "/branches/" + id

	company := map[string]any{
		"name": "Look Company", "currency": "LAK", "language": "en", "timezone": "Asia/Vientiane",
		"booking_mode": "both", "brand_color": "#0e7c66",
		"ring_colors": []string{"#34d399", "#60a5fa", "#fbbf24"}, "heading_font": "kanit", "body_font": "inter",
	}
	a.expect(200, "", "PUT", "/company/", admin, company)

	set := func(changes map[string]any, want int, msg string) {
		body := map[string]any{}
		for k, v := range company {
			body[k] = v
		}
		for k, v := range changes {
			body[k] = v
		}
		a.expect(want, msg, "PUT", "/company/", admin, body)
	}
	set(map[string]any{"ring_colors": []string{"#111111", "#222222", "#333333", "#444444", "#555555"}}, 400, "INVALID_LOOK")
	set(map[string]any{"ring_colors": []string{"red"}}, 400, "INVALID_LOOK")
	set(map[string]any{"heading_font": "comic-sans"}, 400, "INVALID_LOOK")

	// --- a branch without its own look follows the company ---
	s := a.expect(200, "", "GET", m+"/settings", owner, nil)
	if s.str("data.in_force.heading_font") != "kanit" || s.str("data.in_force.ring_colors.2") != "#fbbf24" || s.str("data.ring_colors") != "" {
		t.Fatalf("follows the company: %v", s.body["data"])
	}

	// --- its own look wins; null goes back to the company's ---
	body := map[string]any{}
	for _, k := range []string{"name", "slug", "cover_key", "description", "address", "province", "phone", "map_url",
		"timezone", "booking_mode", "booking_policy", "require_approval"} {
		body[k] = s.body["data"].(map[string]any)[k]
	}
	own := map[string]any{}
	for k, v := range body {
		own[k] = v
	}
	own["ring_colors"], own["body_font"] = []string{"#ff0000", "#00ff00"}, "poppins"
	a.expect(200, "", "PUT", m+"/settings", owner, own)
	s = a.expect(200, "", "GET", m+"/settings", owner, nil)
	if s.str("data.in_force.ring_colors.1") != "#00ff00" || s.str("data.in_force.body_font") != "poppins" || s.str("data.in_force.heading_font") != "kanit" {
		t.Fatalf("own look: %v", s.body["data"])
	}
	bad := map[string]any{}
	for k, v := range own {
		bad[k] = v
	}
	bad["body_font"] = "papyrus"
	a.expect(400, "INVALID_LOOK", "PUT", m+"/settings", owner, bad)
	a.expect(200, "", "PUT", m+"/settings", owner, body)
	if s = a.expect(200, "", "GET", m+"/settings", owner, nil); s.str("data.in_force.body_font") != "inter" {
		t.Fatalf("back to the company's: %v", s.body["data"])
	}

	// --- the public page and home carry what is in force ---
	if pc := a.expect(200, "", "GET", "/public/company", "", nil); pc.str("data.heading_font") != "kanit" || pc.str("data.ring_colors.0") != "#34d399" {
		t.Fatalf("public company: %v", pc.body)
	}
}
