package dto

// PasswordRequest is a new password set by the owner or an admin for someone who cannot reset it by email
type PasswordRequest struct {
	Password string `json:"password"`
}
