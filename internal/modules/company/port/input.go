// Package port: what the company module offers to the rest of the app (input ports)
package port

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
)

// Every branch-scoped method takes the branch it acts on as branchID.
// The HTTP layer has already checked that the caller may act on that branch.

// BranchService is one branch's own settings and publishing
type BranchService interface {
	GetSettings(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
	UpdateSettings(ctx context.Context, branchID uint, req dto.SettingsRequest) (*dto.SettingsDTO, error)
	Publish(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
	Unpublish(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
	// restaurant branches: how tables are booked
	GetTableSettings(ctx context.Context, branchID uint) (*dto.TableSettingsDTO, error)
	UpdateTableSettings(ctx context.Context, branchID uint, req dto.TableSettingsDTO) (*dto.TableSettingsDTO, error)
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
