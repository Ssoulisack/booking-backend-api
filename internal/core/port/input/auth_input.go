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
	// ForgotPassword always succeeds from the caller's view, so it cannot reveal which emails exist
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) error
}
