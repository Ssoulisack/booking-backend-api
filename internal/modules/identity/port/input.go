// Package port: what the identity module offers to the rest of the app (input ports)
package port

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	iddto "backend/internal/modules/identity/dto"
	"context"
)

type AuthService interface {
	Register(ctx context.Context, req iddto.RegisterDTO) (*iddto.AuthResponseDTO, error)
	Login(ctx context.Context, req iddto.LoginDTO) (*iddto.AuthResponseDTO, error)
	// Refresh issues new tokens; issuedAt is the refresh token's issue time (Unix seconds)
	Refresh(ctx context.Context, userID uint, issuedAt int64) (*iddto.AuthResponseDTO, error)
	Me(ctx context.Context, userID uint) (*dto.UserDTO, error)
	MyBranches(ctx context.Context, userID uint) ([]dto.MyBranchDTO, error)
	// UpdateProfile changes the person's own name, nationality, phone and email
	UpdateProfile(ctx context.Context, userID uint, req iddto.ProfileRequest) (*dto.UserDTO, error)
	// SocialLogin signs a customer in with a Google or Facebook token, creating the account on the
	// first visit. A verified provider email joins an existing customer account; team members
	// (owner, admins, managers, staff) keep their password.
	SocialLogin(ctx context.Context, provider, token string) (*iddto.AuthResponseDTO, error)
	// SocialConfig lists the providers switched on for this install (empty ids are off)
	SocialConfig() iddto.SocialConfigDTO
	// ForgotPassword always succeeds from the caller's view, so it cannot reveal which emails exist
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req iddto.ResetPasswordRequest) error
}

// InvitationService serves the invite link: look at an invite, then accept it
type InvitationService interface {
	GetInvitation(ctx context.Context, token string) (*iddto.InvitationInfoDTO, error)
	// Accept joins the team. actor is the logged-in user (nil if none). A new user is created
	// from req; an existing account must be logged in as the invited email.
	Accept(ctx context.Context, token string, actor *domain.User, req iddto.AcceptInvitationRequest) (*iddto.AuthResponseDTO, error)

	// the company owner's and admins' side: who is invited, invite, cancel, send again
	ListInvitations(ctx context.Context) ([]iddto.InvitationDTO, error)
	Invite(ctx context.Context, actor *domain.User, req iddto.InviteRequest) (*iddto.InvitationDTO, error)
	CancelInvitation(ctx context.Context, id uint) error
	ResendInvitation(ctx context.Context, actor *domain.User, id uint) (*iddto.InvitationDTO, error)
}

// AccessService answers "who is calling and what may they do", for the HTTP middleware
type AccessService interface {
	// Authenticate loads the token's user and refuses disabled users and tokens issued before a password reset
	Authenticate(ctx context.Context, userID uint, issuedAt int64) (*domain.User, error)
	// BranchRole returns the caller's role at the branch if it ranks at least minRole
	// (the owner and admins: their company role at every branch)
	BranchRole(ctx context.Context, user *domain.User, branchID uint, minRole string) (string, error)
}
