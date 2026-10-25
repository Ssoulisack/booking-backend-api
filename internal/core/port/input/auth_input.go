package input

import (
	"backend/internal/core/dto"
	"context"
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterDTO) (*dto.AuthResponseDTO, error)
	Login(ctx context.Context, req dto.LoginDTO) (*dto.AuthResponseDTO, error)
	// Refresh issues new tokens; issuedAt is the refresh token's issue time (Unix seconds)
	Refresh(ctx context.Context, userID uint, issuedAt int64) (*dto.AuthResponseDTO, error)
	Me(ctx context.Context, userID uint) (*dto.UserDTO, error)
	MyBranches(ctx context.Context, userID uint) ([]dto.MyBranchDTO, error)
	// UpdateProfile changes the person's own name, nationality, phone and email
	UpdateProfile(ctx context.Context, userID uint, req dto.ProfileRequest) (*dto.UserDTO, error)
	// SocialLogin signs a customer in with a Google or Facebook token, creating the account on the
	// first visit. A verified provider email joins an existing customer account; team members
	// (owner, admins, managers, staff) keep their password.
	SocialLogin(ctx context.Context, provider, token string) (*dto.AuthResponseDTO, error)
	// SocialConfig lists the providers switched on for this install (empty ids are off)
	SocialConfig() dto.SocialConfigDTO
	// ForgotPassword always succeeds from the caller's view, so it cannot reveal which emails exist
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error
}
