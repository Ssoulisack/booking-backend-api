// Package dto: the identity module's own request and answer shapes
package dto

import (
	"time"

	coredto "backend/internal/core/dto"
)

// RegisterDTO creates a customer account. Nationality "LA": phone required, email optional (a stand-in
// is made until the customer adds one). Any other nationality: email required, phone optional.
// The phone is always a Lao mobile number in the format 20xxxxxxxx (020… / +856 20… are accepted and
// converted), for every nationality.
type RegisterDTO struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Phone       string `json:"phone"`
	Nationality string `json:"nationality"`
}

// LoginDTO: login is an email or a phone number; email is still accepted for older clients
type LoginDTO struct {
	Login    string `json:"login"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ProfileRequest changes the logged-in person's own details (same rules as registering)
type ProfileRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Nationality string `json:"nationality"`
}

// SocialLoginRequest carries the token the browser got from Google (an ID token) or Facebook (an
// access token)
type SocialLoginRequest struct {
	Token string `json:"token"`
}

// SocialConfigDTO tells the login page which providers to offer and the public ids it needs
type SocialConfigDTO struct {
	GoogleClientID string `json:"google_client_id"`
	FacebookAppID  string `json:"facebook_app_id"`
}

type AuthResponseDTO struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	User         coredto.UserDTO `json:"user"`
}

// InviteRequest asks someone to join: role "admin" (every branch) or "manager" / "staff" with branch_ids
type InviteRequest struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	BranchIDs []uint `json:"branch_ids"`
}

type InvitationDTO struct {
	ID        uint                `json:"id"`
	Email     string              `json:"email"`
	Role      string              `json:"role"`
	Branches  []coredto.BranchRef `json:"branches"`
	ExpiresAt time.Time           `json:"expires_at"`
}

// InvitationInfoDTO is what the accept page shows before the user accepts
type InvitationInfoDTO struct {
	CompanyName   string   `json:"company_name"`
	Email         string   `json:"email"`
	Role          string   `json:"role"`
	Branches      []string `json:"branches"`       // empty for an admin (every branch)
	AccountExists bool     `json:"account_exists"` // true: log in first; false: choose a name and password
}

type AcceptInvitationRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}
