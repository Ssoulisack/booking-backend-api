package dto

type UserDTO struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	Phone       string `json:"phone"`
	CompanyRole string `json:"company_role"` // "owner", "admin" or empty
	Nationality string `json:"nationality"`
	// the email is a stand-in: the person should add a real one in My account
	EmailGenerated bool `json:"email_generated"`
	// the team set the password: the person is asked to choose their own
	MustChangePassword bool `json:"must_change_password"`
	// the person proved the email is theirs with a code (false for a stand-in email)
	EmailVerified bool `json:"email_verified"`
	// false for an account made with Google / Facebook that never set a password
	HasPassword bool `json:"has_password"`
	// the number this person tells staff: CNO000042 (customer) or ENO000007 (the team)
	MemberNo  string `json:"member_no"`
	Status    bool   `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type UserJWTDTO struct {
	Access  string `json:"access,omitempty"`
	Refresh string `json:"refresh,omitempty"`
}
