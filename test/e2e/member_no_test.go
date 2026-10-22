package e2e

import (
	"net/url"
	"testing"
)

// Member numbers: customers CNO000001…, the team ENO000001…, each with its own counter. A role
// change takes the next number of the other kind; the old number is retired.
func TestMemberNumbers(t *testing.T) {
	a := api{t: t, app: newApp(t)}
	owner := ownerToken(a)
	branch, _, _ := setupClub(t, a, owner, "Numbers Club", "")

	// the owner from the install is the first of the team
	if me := a.expect(200, "", "GET", "/auth/me", owner, nil); me.str("data.member_no") != "ENO000001" {
		t.Fatalf("owner number: %v", me.body)
	}

	// customers count separately, from 1
	cara := register(a, "Cara Customer", "cara@numbers.test")
	dan := register(a, "Dan Customer", "dan@numbers.test")
	if me := a.expect(200, "", "GET", "/auth/me", cara, nil); me.str("data.member_no") != "CNO000001" {
		t.Fatalf("first customer: %v", me.body)
	}
	if me := a.expect(200, "", "GET", "/auth/me", dan, nil); me.str("data.member_no") != "CNO000002" {
		t.Fatalf("second customer: %v", me.body)
	}

	// a new account invited as staff starts as ENO, without using a CNO number
	teamMember(a, owner, "sam@numbers.test", "staff", branch)
	team := a.expect(200, "", "GET", "/company/team", owner, nil).body["data"].([]any)
	numbers := map[string]string{}
	for _, m := range team {
		row := m.(map[string]any)
		numbers[row["email"].(string)] = row["member_no"].(string)
	}
	if numbers["sam@numbers.test"] != "ENO000002" {
		t.Fatalf("team numbers: %v", numbers)
	}

	// Cara (a customer) accepts an invitation while logged in: she joins the team with the next ENO
	a.expect(201, "", "POST", "/company/invitations", owner, map[string]any{"email": "cara@numbers.test", "role": "manager", "branch_ids": []int{num(branch)}})
	joined := a.expect(200, "", "POST", "/invitations/"+linkFor(t, "cara@numbers.test", "/invite/")+"/accept", cara, map[string]any{})
	if joined.str("data.user.member_no") != "ENO000003" {
		t.Fatalf("customer joins the team: %v", joined.body)
	}
	caraID := joined.str("data.user.id")

	// her old number is retired; the new one finds her
	if found := a.expect(200, "", "GET", "/company/users?keyword=CNO000001", owner, nil); len(found.body["data"].([]any)) != 0 {
		t.Fatalf("retired number still finds someone: %v", found.body)
	}
	if found := a.expect(200, "", "GET", "/company/users?keyword="+url.QueryEscape("eno 3"), owner, nil); found.str("data.0.id") != caraID {
		t.Fatalf("search by ENO: %v", found.body)
	}

	// manager -> admin -> staff stays on the team: same number
	a.expect(200, "", "PUT", "/company/team/"+caraID, owner, map[string]any{"role": "admin"})
	if r := a.expect(200, "", "PUT", "/company/team/"+caraID, owner, map[string]any{"role": "staff", "branch_ids": []int{num(branch)}}); r.str("data.member_no") != "ENO000003" {
		t.Fatalf("role change inside the team: %v", r.body)
	}

	// removed from the team: a customer again, with the next CNO number
	a.expect(200, "", "DELETE", "/company/team/"+caraID, owner, nil)
	if u := a.expect(200, "", "GET", "/company/users?keyword=cara%40numbers.test", owner, nil); u.str("data.0.member_no") != "CNO000003" {
		t.Fatalf("back to customer: %v", u.body)
	}

	// the desk finds a customer by number (any spelling); the old C0042 style is gone
	m := "/branches/" + branch
	if found := a.expect(200, "", "GET", m+"/customers/find?q=cno-2", owner, nil); found.str("data.member_no") != "CNO000002" {
		t.Fatalf("desk lookup: %v", found.body)
	}
	a.expect(400, "CUSTOMER_NOT_FOUND", "GET", m+"/customers/find?q=C0002", owner, nil)
}
