// Package port: what the catalog module offers to the rest of the app (input ports)
package port

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
)

// CatalogService manages sports, resources and coaches of one branch
type CatalogService interface {
	CreateSport(ctx context.Context, branchID uint, req dto.SportRequest) (*dto.SportDTO, error)
	GetSport(ctx context.Context, branchID, id uint) (*dto.SportDTO, error)
	ListSports(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.SportDTO], error)
	UpdateSport(ctx context.Context, branchID, id uint, req dto.SportRequest) (*dto.SportDTO, error)
	// restaurant areas: the floor plan with its tables
	GetFloorPlan(ctx context.Context, branchID, areaID uint) (*dto.FloorPlanDTO, error)
	SaveFloorPlan(ctx context.Context, branchID, areaID uint, req dto.FloorPlanRequest) (*dto.FloorPlanDTO, error)
	DeleteSport(ctx context.Context, branchID, id uint) error

	CreateResource(ctx context.Context, branchID uint, req dto.ResourceRequest) (*dto.ResourceDTO, error)
	GetResource(ctx context.Context, branchID, id uint) (*dto.ResourceDTO, error)
	ListResources(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.ResourceDTO], error)
	UpdateResource(ctx context.Context, branchID, id uint, req dto.ResourceRequest) (*dto.ResourceDTO, error)
	DeleteResource(ctx context.Context, branchID, id uint) error

	CreateCoach(ctx context.Context, branchID uint, req dto.CoachRequest) (*dto.CoachDTO, error)
	GetCoach(ctx context.Context, branchID, id uint) (*dto.CoachDTO, error)
	ListCoaches(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.CoachDTO], error)
	UpdateCoach(ctx context.Context, branchID, id uint, req dto.CoachRequest) (*dto.CoachDTO, error)
	DeleteCoach(ctx context.Context, branchID, id uint) error
}

// ScheduleService manages opening hours, closures, prices and class sessions of one branch
type ScheduleService interface {
	GetOpeningHours(ctx context.Context, branchID uint, ownerType string, ownerID uint) ([]dto.OpeningHourDTO, error)
	SetOpeningHours(ctx context.Context, branchID uint, ownerType string, ownerID uint, req dto.OpeningHoursRequest) ([]dto.OpeningHourDTO, error)

	CreateClosure(ctx context.Context, branchID uint, req dto.ClosureRequest) (*dto.ClosureDTO, error)
	GetClosure(ctx context.Context, branchID, id uint) (*dto.ClosureDTO, error)
	ListClosures(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.ClosureDTO], error)
	UpdateClosure(ctx context.Context, branchID, id uint, req dto.ClosureRequest) (*dto.ClosureDTO, error)
	DeleteClosure(ctx context.Context, branchID, id uint) error

	GetPrice(ctx context.Context, branchID uint, ownerType string, ownerID uint) (*dto.PriceDTO, error)
	SetPrice(ctx context.Context, branchID uint, ownerType string, ownerID uint, req dto.PriceRequest) (*dto.PriceDTO, error)
	DeletePrice(ctx context.Context, branchID uint, ownerType string, ownerID uint) error

	CreateClassSession(ctx context.Context, branchID uint, req dto.ClassSessionRequest) (*dto.ClassSessionDTO, error)
	GetClassSession(ctx context.Context, branchID, id uint) (*dto.ClassSessionDTO, error)
	ListClassSessions(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.ClassSessionDTO], error)
	UpdateClassSession(ctx context.Context, branchID, id uint, req dto.ClassSessionRequest) (*dto.ClassSessionDTO, error)
	DeleteClassSession(ctx context.Context, branchID, id uint) error
	// RepeatClassSessions creates a session on every chosen weekday in a date range, all or nothing
	RepeatClassSessions(ctx context.Context, branchID uint, req dto.RepeatClassRequest) (*dto.RepeatClassResultDTO, error)
	// weekly timetables: classes are made automatically ClassWindowDays ahead
	ListTimetables(ctx context.Context, branchID uint) ([]dto.TimetableDTO, error)
	CreateTimetable(ctx context.Context, branchID uint, req dto.TimetableRequest) (*dto.TimetableResultDTO, error)
	UpdateTimetable(ctx context.Context, branchID, id uint, req dto.TimetableRequest) (*dto.TimetableResultDTO, error)
	DeleteTimetable(ctx context.Context, branchID, id uint) (*dto.TimetableStopDTO, error)
	// RollTimetables keeps every running timetable's classes made ahead (start and every hour)
	RollTimetables(ctx context.Context) error
}

// RuleService manages booking rules and resolves the rule that applies to a sport or resource
type RuleService interface {
	GetRule(ctx context.Context, branchID uint, scopeType string, scopeID uint) (*dto.RuleDTO, error)
	ListRules(ctx context.Context, branchID uint) ([]dto.RuleDTO, error)
	SetRule(ctx context.Context, branchID uint, scopeType string, scopeID uint, req dto.RuleRequest) (*dto.RuleDTO, error)
	DeleteRule(ctx context.Context, branchID uint, scopeType string, scopeID uint) error
	// Effective merges company -> branch -> sport -> resource. resourceID implies its sport.
	Effective(ctx context.Context, branchID, sportID, resourceID uint) (*domain.EffectiveRule, error)
}
