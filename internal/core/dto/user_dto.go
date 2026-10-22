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
