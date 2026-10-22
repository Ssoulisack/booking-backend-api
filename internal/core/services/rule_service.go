package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"net/http"
)

type RuleServiceImpl struct {
	rules     output.RuleRepository
	sports    output.SportRepository
	resources output.ResourceRepository
}

func NewRuleService(
	rules output.RuleRepository,
	sports output.SportRepository,
	resources output.ResourceRepository,
) input.RuleService {
	return &RuleServiceImpl{rules: rules, sports: sports, resources: resources}
}

func toRuleDTO(r domain.BookingRule) dto.RuleDTO {
	return dto.RuleDTO{
		ScopeType: r.ScopeType,
		ScopeID:   r.ScopeID,
		RuleRequest: dto.RuleRequest{
			MinDurationMin: r.MinDurationMin, MaxDurationMin: r.MaxDurationMin,
			DurationStepMin: r.DurationStepMin, StartStepMin: r.StartStepMin,
			CancelBeforeHours: r.CancelBeforeHours, MaxDaysAhead: r.MaxDaysAhead,
			MinNoticeMin: r.MinNoticeMin, Capacity: r.Capacity,
			MaxActiveBookingsPerCustomer: r.MaxActiveBookingsPerCustomer,
		},
	}
}

// override replaces dst when the narrower scope sets a value
func override(dst *int, src *int) {
	if src != nil {
		*dst = *src
	}
}

// mergeRules applies rules from the widest scope to the most specific one
func mergeRules(chain ...*domain.BookingRule) domain.EffectiveRule {
	var e domain.EffectiveRule
	for _, r := range chain {
		if r == nil {
			continue
		}
		override(&e.MinDurationMin, r.MinDurationMin)
		override(&e.MaxDurationMin, r.MaxDurationMin)
		override(&e.DurationStepMin, r.DurationStepMin)
		override(&e.StartStepMin, r.StartStepMin)
		override(&e.CancelBeforeHours, r.CancelBeforeHours)
		override(&e.MaxDaysAhead, r.MaxDaysAhead)
		override(&e.MinNoticeMin, r.MinNoticeMin)
		override(&e.Capacity, r.Capacity)
		override(&e.MaxActiveBookingsPerCustomer, r.MaxActiveBookingsPerCustomer)
	}
	return e
}

// validateEffective checks that a merged rule is usable: durations run from min to max in whole steps
func validateEffective(e domain.EffectiveRule) error {
	switch {
	case e.MinDurationMin < 1 || e.MaxDurationMin < 1 || e.DurationStepMin < 1 || e.StartStepMin < 1:
		return badRequest("DURATIONS_AND_STEPS_MUST_BE_POSITIVE")
	case e.MaxDurationMin < e.MinDurationMin:
		return badRequest("MAX_DURATION_MUST_NOT_BE_BELOW_MIN")
	case (e.MaxDurationMin-e.MinDurationMin)%e.DurationStepMin != 0:
		return badRequest("MAX_DURATION_MUST_BE_MIN_PLUS_WHOLE_STEPS")
	case e.CancelBeforeHours < 0 || e.MinNoticeMin < 0:
		return badRequest("CANCEL_AND_NOTICE_MUST_NOT_BE_NEGATIVE")
	case e.MaxDaysAhead < 1:
		return badRequest("MAX_DAYS_AHEAD_MUST_BE_POSITIVE")
	case e.Capacity < 1:
		return badRequest("CAPACITY_MUST_BE_AT_LEAST_1")
	case e.MaxActiveBookingsPerCustomer < 1:
		return badRequest("MAX_ACTIVE_BOOKINGS_MUST_BE_AT_LEAST_1")
	}
	return nil
}

// companyRule is the company default every rule starts from (created at install)
func (s *RuleServiceImpl) companyRule(ctx context.Context) (*domain.BookingRule, error) {
	r, err := s.rules.GetByScope(ctx, 0, domain.ScopeCompany, 0)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, response.NewError(http.StatusInternalServerError, "COMPANY_RULE_MISSING")
	}
	return r, nil
}

// checkScope validates the scope and returns the sport that applies (0 for company and branch rules).
// The company rule is reached with branchID 0, every other scope within a branch.
func (s *RuleServiceImpl) checkScope(ctx context.Context, branchID uint, scopeType string, scopeID uint) (sportID uint, err error) {
	if (scopeType == domain.ScopeCompany) != (branchID == 0) {
		return 0, badRequest("INVALID_SCOPE")
	}
	switch scopeType {
	case domain.ScopeCompany, domain.ScopeBranch:
		if scopeID != 0 {
			return 0, badRequest("RULE_HAS_NO_ID")
		}
	case domain.ScopeSport:
		if _, err := s.sports.GetByID(ctx, branchID, scopeID); err != nil {
			return 0, orBadRequest(err, "SPORT_NOT_FOUND")
		}
		return scopeID, nil
	case domain.ScopeResource:
		res, err := s.resources.GetByID(ctx, branchID, scopeID)
		if err != nil {
			return 0, orBadRequest(err, "RESOURCE_NOT_FOUND")
		}
		return res.SportID, nil
	default:
		return 0, badRequest("INVALID_SCOPE")
	}
	return 0, nil
}

func (s *RuleServiceImpl) GetRule(ctx context.Context, branchID uint, scopeType string, scopeID uint) (*dto.RuleDTO, error) {
	if _, err := s.checkScope(ctx, branchID, scopeType, scopeID); err != nil {
		return nil, err
	}
	r, err := s.rules.GetByScope(ctx, branchID, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, notFound("RULE_NOT_FOUND")
	}
	d := toRuleDTO(*r)
	return &d, nil
}

func (s *RuleServiceImpl) ListRules(ctx context.Context, branchID uint) ([]dto.RuleDTO, error) {
	rules, err := s.rules.List(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rules, toRuleDTO), nil
}

// SetRule replaces the rule of a scope. The merged result for that scope must stay valid.
// Rules of narrower scopes are not re-checked here; Effective refuses them (409) when they are used.
func (s *RuleServiceImpl) SetRule(ctx context.Context, branchID uint, scopeType string, scopeID uint, req dto.RuleRequest) (*dto.RuleDTO, error) {
	sportID, err := s.checkScope(ctx, branchID, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	candidate := domain.BookingRule{
		BranchID: branchID, ScopeType: scopeType, ScopeID: scopeID,
		MinDurationMin: req.MinDurationMin, MaxDurationMin: req.MaxDurationMin,
		DurationStepMin: req.DurationStepMin, StartStepMin: req.StartStepMin,
		CancelBeforeHours: req.CancelBeforeHours, MaxDaysAhead: req.MaxDaysAhead,
		MinNoticeMin: req.MinNoticeMin, Capacity: req.Capacity,
		MaxActiveBookingsPerCustomer: req.MaxActiveBookingsPerCustomer,
	}

	var chain []*domain.BookingRule
	if scopeType == domain.ScopeCompany {
		if !allSet(req) {
			return nil, badRequest("COMPANY_RULE_NEEDS_EVERY_FIELD")
		}
	} else {
		company, err := s.companyRule(ctx)
		if err != nil {
			return nil, err
		}
		chain = append(chain, company)
		if scopeType != domain.ScopeBranch {
			branchRule, err := s.rules.GetByScope(ctx, branchID, domain.ScopeBranch, 0)
			if err != nil {
				return nil, err
			}
			chain = append(chain, branchRule)
		}
		if scopeType == domain.ScopeResource {
			sportRule, err := s.rules.GetByScope(ctx, branchID, domain.ScopeSport, sportID)
			if err != nil {
				return nil, err
			}
			chain = append(chain, sportRule)
		}
	}
	if err := validateEffective(mergeRules(append(chain, &candidate)...)); err != nil {
		return nil, err
	}

	if err := s.rules.Upsert(ctx, &candidate); err != nil {
		return nil, err
	}
	d := toRuleDTO(candidate)
	return &d, nil
}

func allSet(r dto.RuleRequest) bool {
	return r.MinDurationMin != nil && r.MaxDurationMin != nil && r.DurationStepMin != nil &&
		r.StartStepMin != nil && r.CancelBeforeHours != nil && r.MaxDaysAhead != nil &&
		r.MinNoticeMin != nil && r.Capacity != nil && r.MaxActiveBookingsPerCustomer != nil
}

func (s *RuleServiceImpl) DeleteRule(ctx context.Context, branchID uint, scopeType string, scopeID uint) error {
	if scopeType == domain.ScopeCompany {
		return badRequest("CANNOT_DELETE_COMPANY_RULE")
	}
	if _, err := s.checkScope(ctx, branchID, scopeType, scopeID); err != nil {
		return err
	}
	return s.rules.DeleteByScope(ctx, branchID, scopeType, scopeID)
}

// Effective returns the rule that applies to a resource (or to a sport when resourceID is 0, or to
// the branch when both are 0): company -> branch -> sport -> resource
func (s *RuleServiceImpl) Effective(ctx context.Context, branchID, sportID, resourceID uint) (*domain.EffectiveRule, error) {
	company, err := s.companyRule(ctx)
	if err != nil {
		return nil, err
	}
	branchRule, err := s.rules.GetByScope(ctx, branchID, domain.ScopeBranch, 0)
	if err != nil {
		return nil, err
	}
	chain := []*domain.BookingRule{company, branchRule}

	if resourceID != 0 {
		res, err := s.resources.GetByID(ctx, branchID, resourceID)
		if err != nil {
			return nil, orBadRequest(err, "RESOURCE_NOT_FOUND")
		}
		sportID = res.SportID
	} else if sportID != 0 {
		if _, err := s.sports.GetByID(ctx, branchID, sportID); err != nil {
			return nil, orBadRequest(err, "SPORT_NOT_FOUND")
		}
	}
	if sportID != 0 {
		r, err := s.rules.GetByScope(ctx, branchID, domain.ScopeSport, sportID)
		if err != nil {
			return nil, err
		}
		chain = append(chain, r)
	}
	if resourceID != 0 {
		r, err := s.rules.GetByScope(ctx, branchID, domain.ScopeResource, resourceID)
		if err != nil {
			return nil, err
		}
		chain = append(chain, r)
	}

	e := mergeRules(chain...)
	if err := validateEffective(e); err != nil {
		return nil, conflict("RULES_ARE_INCONSISTENT")
	}
	return &e, nil
}
