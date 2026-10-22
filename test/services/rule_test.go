package servicetest

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/services"
	"context"
	"testing"
)

const (
	bizA, bizB             = 1, 2
	tennis, soccer, boxing = 1, 2, 3 // sports of branch A
	otherSport             = 4       // sport of branch B
	tennisCourt1           = 1
	soccerPitch1           = 2
	otherCourt             = 3 // resource of branch B
)

// newRuleService starts from the company rule every branch inherits: min 60, max 180, step 30
func newRuleService() (input.RuleService, *fakeRuleRepo) {
	repo := &fakeRuleRepo{rules: map[string]*domain.BookingRule{}}
	r := domain.DefaultCompanyRule()
	repo.rules[ruleKey(0, domain.ScopeCompany, 0)] = &r
	sports := &fakeSports{owner: map[uint]uint{tennis: bizA, soccer: bizA, boxing: bizA, otherSport: bizB}}
	resources := fakeResources{
		sportOf: map[uint]uint{tennisCourt1: tennis, soccerPitch1: soccer, otherCourt: otherSport},
		owner:   map[uint]uint{tennisCourt1: bizA, soccerPitch1: bizA, otherCourt: bizB},
	}
	return services.NewRuleService(repo, sports, resources), repo
}

func mustSet(t *testing.T, svc input.RuleService, branch uint, scope string, id uint, req dto.RuleRequest) {
	t.Helper()
	if _, err := svc.SetRule(context.Background(), branch, scope, id, req); err != nil {
		t.Fatalf("SetRule(%d,%s,%d): %v", branch, scope, id, err)
	}
}

func TestEffectiveSportRules(t *testing.T) {
	svc, _ := newRuleService()
	ctx := context.Background()

	// tennis: branch-chosen maximum 240; soccer: max 90; boxing: fixed 60-minute session
	mustSet(t, svc, bizA, domain.ScopeSport, tennis, dto.RuleRequest{MaxDurationMin: ip(240)})
	mustSet(t, svc, bizA, domain.ScopeSport, soccer, dto.RuleRequest{MaxDurationMin: ip(90)})
	mustSet(t, svc, bizA, domain.ScopeSport, boxing, dto.RuleRequest{MaxDurationMin: ip(60)})

	cases := []struct {
		name               string
		sportID, resource  uint
		wantMin, wantMax   int
		wantStep, wantCanc int
	}{
		{"tennis court inherits the tennis max", 0, tennisCourt1, 60, 240, 30, 24},
		{"tennis by sport id", tennis, 0, 60, 240, 30, 24},
		{"soccer max 90", soccer, 0, 60, 90, 30, 24},
		{"soccer pitch via resource", 0, soccerPitch1, 60, 90, 30, 24},
		{"boxing is a single 60 minute session", boxing, 0, 60, 60, 30, 24},
		{"no sport or resource gives the company rule", 0, 0, 60, 180, 30, 24},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := svc.Effective(ctx, bizA, c.sportID, c.resource)
			if err != nil {
				t.Fatal(err)
			}
			if e.MinDurationMin != c.wantMin || e.MaxDurationMin != c.wantMax ||
				e.DurationStepMin != c.wantStep || e.CancelBeforeHours != c.wantCanc {
				t.Fatalf("got %+v", *e)
			}
		})
	}
}

func TestBranchChangesOnlyItsOwnRule(t *testing.T) {
	svc, _ := newRuleService()
	ctx := context.Background()
	// branch A changes two fields of the company rule; the rest still comes from the company
	mustSet(t, svc, bizA, domain.ScopeBranch, 0, dto.RuleRequest{MaxDurationMin: ip(120), CancelBeforeHours: ip(2)})
	a, err := svc.Effective(ctx, bizA, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.MaxDurationMin != 120 || a.CancelBeforeHours != 2 || a.MinDurationMin != 60 || a.MaxDaysAhead != 30 {
		t.Fatalf("branch A: got %+v", *a)
	}
	e, err := svc.Effective(ctx, bizB, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if e.MaxDurationMin != 180 || e.CancelBeforeHours != 24 {
		t.Fatalf("branch B must keep the company rule, got %+v", *e)
	}
	// branch A cannot touch branch B's sports or resources
	if _, err := svc.SetRule(ctx, bizA, domain.ScopeSport, otherSport, dto.RuleRequest{MaxDurationMin: ip(90)}); err == nil || err.Error() != "SPORT_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.Effective(ctx, bizA, 0, otherCourt); err == nil || err.Error() != "RESOURCE_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
}

func TestResourceRuleOverridesSport(t *testing.T) {
	svc, _ := newRuleService()
	mustSet(t, svc, bizA, domain.ScopeSport, tennis, dto.RuleRequest{MaxDurationMin: ip(240)})
	mustSet(t, svc, bizA, domain.ScopeResource, tennisCourt1, dto.RuleRequest{MaxDurationMin: ip(120), CancelBeforeHours: ip(2)})

	e, err := svc.Effective(context.Background(), bizA, 0, tennisCourt1)
	if err != nil {
		t.Fatal(err)
	}
	if e.MaxDurationMin != 120 || e.CancelBeforeHours != 2 || e.MinDurationMin != 60 {
		t.Fatalf("got %+v", *e)
	}
}

func TestSetRuleRejectsInvalid(t *testing.T) {
	svc, _ := newRuleService()
	ctx := context.Background()
	cases := map[string]struct {
		scope string
		id    uint
		req   dto.RuleRequest
		code  string
	}{
		"max below min":                   {domain.ScopeSport, tennis, dto.RuleRequest{MaxDurationMin: ip(30)}, "MAX_DURATION_MUST_NOT_BE_BELOW_MIN"},
		"max is not min plus whole steps": {domain.ScopeSport, tennis, dto.RuleRequest{MaxDurationMin: ip(100)}, "MAX_DURATION_MUST_BE_MIN_PLUS_WHOLE_STEPS"},
		"zero step":                       {domain.ScopeSport, tennis, dto.RuleRequest{DurationStepMin: ip(0)}, "DURATIONS_AND_STEPS_MUST_BE_POSITIVE"},
		"negative cancel window":          {domain.ScopeSport, tennis, dto.RuleRequest{CancelBeforeHours: ip(-1)}, "CANCEL_AND_NOTICE_MUST_NOT_BE_NEGATIVE"},
		"unknown sport":                   {domain.ScopeSport, 9, dto.RuleRequest{MaxDurationMin: ip(90)}, "SPORT_NOT_FOUND"},
		"unknown resource":                {domain.ScopeResource, 9, dto.RuleRequest{MaxDurationMin: ip(90)}, "RESOURCE_NOT_FOUND"},
		"bad scope":                       {"venue", 0, dto.RuleRequest{}, "INVALID_SCOPE"},
		"company rule through a branch":   {domain.ScopeCompany, 0, dto.RuleRequest{}, "INVALID_SCOPE"},
		"branch rule takes no id":         {domain.ScopeBranch, 1, dto.RuleRequest{}, "RULE_HAS_NO_ID"},
		"branch rule breaks the chain":    {domain.ScopeBranch, 0, dto.RuleRequest{MaxDurationMin: ip(100)}, "MAX_DURATION_MUST_BE_MIN_PLUS_WHOLE_STEPS"},
		"resource rule breaks the chain":  {domain.ScopeResource, tennisCourt1, dto.RuleRequest{MinDurationMin: ip(100)}, "MAX_DURATION_MUST_BE_MIN_PLUS_WHOLE_STEPS"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.SetRule(ctx, bizA, tc.scope, tc.id, tc.req)
			if err == nil || err.Error() != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
		})
	}
}

func TestInvalidRuleIsNotSaved(t *testing.T) {
	svc, repo := newRuleService()
	_, _ = svc.SetRule(context.Background(), bizA, domain.ScopeSport, tennis, dto.RuleRequest{MaxDurationMin: ip(30)})
	if repo.rules[ruleKey(bizA, domain.ScopeSport, tennis)] != nil {
		t.Fatal("a rejected rule must not be stored")
	}
}

func TestCompanyRule(t *testing.T) {
	svc, _ := newRuleService()
	ctx := context.Background()
	// the company rule is reached with branch 0 and needs every field
	if _, err := svc.SetRule(ctx, 0, domain.ScopeCompany, 0, dto.RuleRequest{MaxDurationMin: ip(90)}); err == nil || err.Error() != "COMPANY_RULE_NEEDS_EVERY_FIELD" {
		t.Fatalf("got %v", err)
	}
	if _, err := svc.SetRule(ctx, 0, domain.ScopeSport, tennis, dto.RuleRequest{}); err == nil || err.Error() != "INVALID_SCOPE" {
		t.Fatalf("a sport rule needs a branch: got %v", err)
	}
	if err := svc.DeleteRule(ctx, 0, domain.ScopeCompany, 0); err == nil {
		t.Fatal("the company rule must not be deletable")
	}
	// a change reaches every branch that did not change that field
	mustSet(t, svc, bizA, domain.ScopeBranch, 0, dto.RuleRequest{CancelBeforeHours: ip(2)})
	mustSet(t, svc, 0, domain.ScopeCompany, 0, dto.RuleRequest{
		MinDurationMin: ip(60), MaxDurationMin: ip(240), DurationStepMin: ip(30), StartStepMin: ip(30),
		CancelBeforeHours: ip(48), MaxDaysAhead: ip(60), MinNoticeMin: ip(0), Capacity: ip(8), MaxActiveBookingsPerCustomer: ip(5),
	})
	a, _ := svc.Effective(ctx, bizA, 0, 0)
	b, _ := svc.Effective(ctx, bizB, 0, 0)
	if a.MaxDurationMin != 240 || a.CancelBeforeHours != 2 || b.MaxDurationMin != 240 || b.CancelBeforeHours != 48 {
		t.Fatalf("A %+v, B %+v", *a, *b)
	}
	// deleting the branch rule falls back to the company
	if err := svc.DeleteRule(ctx, bizA, domain.ScopeBranch, 0); err != nil {
		t.Fatal(err)
	}
	if a, _ = svc.Effective(ctx, bizA, 0, 0); a.CancelBeforeHours != 48 {
		t.Fatalf("after delete: %+v", *a)
	}
}

func TestSportRuleMakesNarrowerRuleInconsistent(t *testing.T) {
	svc, _ := newRuleService()
	ctx := context.Background()
	mustSet(t, svc, bizA, domain.ScopeResource, tennisCourt1, dto.RuleRequest{MaxDurationMin: ip(120)})
	// raising the sport minimum later breaks the court rule; Effective must refuse to serve it
	mustSet(t, svc, bizA, domain.ScopeSport, tennis, dto.RuleRequest{MinDurationMin: ip(150), MaxDurationMin: ip(240)})
	if _, err := svc.Effective(ctx, bizA, 0, tennisCourt1); err == nil || err.Error() != "RULES_ARE_INCONSISTENT" {
		t.Fatalf("got %v", err)
	}
}
