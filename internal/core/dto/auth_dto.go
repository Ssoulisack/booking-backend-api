package dto

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

// PasswordRequest is a new password set by the owner or an admin for someone who cannot reset it by email
type PasswordRequest struct {
	Password string `json:"password"`
}

type AuthResponseDTO struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	User         UserDTO `json:"user"`
}
