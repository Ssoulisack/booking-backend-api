package input

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
	"io"
)

// Every branch-scoped method takes the branch it acts on as branchID.
// The HTTP layer has already checked that the caller may act on that branch.

// CatalogService manages sports, resources and coaches of one branch
type CatalogService interface {
	CreateSport(ctx context.Context, branchID uint, req dto.SportRequest) (*dto.SportDTO, error)
	GetSport(ctx context.Context, branchID, id uint) (*dto.SportDTO, error)
	ListSports(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.SportDTO], error)
	UpdateSport(ctx context.Context, branchID, id uint, req dto.SportRequest) (*dto.SportDTO, error)
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

// MediaService validates and stores uploaded images; branchID 0 stores company files (logo, cover)
type MediaService interface {
	Upload(ctx context.Context, branchID uint, folder string, file io.Reader) (*dto.MediaDTO, error)
	Delete(ctx context.Context, branchID uint, key string) error
}

// BranchService is one branch's own settings and publishing
type BranchService interface {
	GetSettings(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
	UpdateSettings(ctx context.Context, branchID uint, req dto.SettingsRequest) (*dto.SettingsDTO, error)
	Publish(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
	Unpublish(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
}

// CompanyService is what the owner and admins run: the company, its branches, the team and users
type CompanyService interface {
	GetCompany(ctx context.Context) (*dto.CompanyDTO, error)
	UpdateCompany(ctx context.Context, req dto.CompanyRequest) (*dto.CompanyDTO, error)

	ListBranches(ctx context.Context) ([]dto.BranchDTO, error)
	CreateBranch(ctx context.Context, req dto.CreateBranchRequest) (*dto.BranchDTO, error)
	// SetClosed closes a branch (unpublished, hidden from customers and its team) or reopens it
	SetClosed(ctx context.Context, branchID uint, closed bool) (*dto.BranchDTO, error)

	ListTeam(ctx context.Context) ([]dto.TeamMemberDTO, error)
	UpdateTeamMember(ctx context.Context, actor *domain.User, userID uint, req dto.TeamMemberRequest) (*dto.TeamMemberDTO, error)
	// RemoveTeamMember takes away every company and branch role; the account stays a customer
	RemoveTeamMember(ctx context.Context, actor *domain.User, userID uint) error
	// TransferOwnership makes an admin the owner; the old owner becomes an admin
	TransferOwnership(ctx context.Context, actor *domain.User, toUserID uint) error

	ListInvitations(ctx context.Context) ([]dto.InvitationDTO, error)
	Invite(ctx context.Context, actor *domain.User, req dto.InviteRequest) (*dto.InvitationDTO, error)
	CancelInvitation(ctx context.Context, id uint) error
	ResendInvitation(ctx context.Context, actor *domain.User, id uint) (*dto.InvitationDTO, error)

	ListUsers(ctx context.Context, q dto.ListQuery) (*dto.PagedResult[dto.UserDTO], error)
	SetUserStatus(ctx context.Context, actor *domain.User, targetID uint, status bool) error
	// SetUserPassword gives someone a new password when they cannot reset it by email; their logins end
	SetUserPassword(ctx context.Context, actor *domain.User, targetID uint, password string) error
}

// InstallService prepares a fresh install from config.yaml: the company, its default booking
// rule and the first owner (who gets an email to choose a password). Safe to run on every start.
type InstallService interface {
	EnsureInstalled(ctx context.Context) error
}

// InvitationService serves the invite link: look at an invite, then accept it
type InvitationService interface {
	GetInvitation(ctx context.Context, token string) (*dto.InvitationInfoDTO, error)
	// Accept joins the team. actor is the logged-in user (nil if none). A new user is created
	// from req; an existing account must be logged in as the invited email.
	Accept(ctx context.Context, token string, actor *domain.User, req dto.AcceptInvitationRequest) (*dto.AuthResponseDTO, error)
}

// AccessService answers "who is calling and what may they do", for the HTTP middleware
type AccessService interface {
	// Authenticate loads the token's user and refuses disabled users and tokens issued before a password reset
	Authenticate(ctx context.Context, userID uint, issuedAt int64) (*domain.User, error)
	// BranchRole returns the caller's role at the branch if it ranks at least minRole
	// (the owner and admins: their company role at every branch)
	BranchRole(ctx context.Context, user *domain.User, branchID uint, minRole string) (string, error)
}

// PublicService serves the public site: the company's brand, its live branches and their booking pages
type PublicService interface {
	Company(ctx context.Context) (*dto.PublicCompanyDTO, error)
	// Directory lists the live branches for the home page
	Directory(ctx context.Context, q dto.DirectoryQuery) (*dto.PagedResult[dto.DirectoryBranchDTO], error)
	Branch(ctx context.Context, slug string) (*dto.PublicBranchDTO, error)
	Sports(ctx context.Context, slug string) ([]dto.SportDTO, error)
	Resources(ctx context.Context, slug string, sportID uint) ([]dto.PublicResourceDTO, error)
	Coaches(ctx context.Context, slug string, sportID uint) ([]dto.PublicCoachDTO, error)
	ClassSessions(ctx context.Context, slug string, q dto.ListQuery) (*dto.PagedResult[dto.ClassSessionDTO], error)
}
