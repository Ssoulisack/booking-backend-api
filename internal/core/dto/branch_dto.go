package dto

import "time"

// ---------- company (owner / admins) ----------

// CompanyRequest replaces every company setting at once (like PUT elsewhere)
type CompanyRequest struct {
	Name            string            `json:"name"`
	LogoKey         string            `json:"logo_key"`
	CoverKey        string            `json:"cover_key"`
	BrandColor      string            `json:"brand_color"`
	Description     string            `json:"description"`
	Phone           string            `json:"phone"`
	Email           string            `json:"email"`
	SocialLinks     map[string]string `json:"social_links"`
	Language        string            `json:"language"`
	Currency        string            `json:"currency"`
	Timezone        string            `json:"timezone"`
	BookingMode     string            `json:"booking_mode"`
	BookingPolicy   string            `json:"booking_policy"`
	RequireApproval bool              `json:"require_approval"`
}

type CompanyDTO struct {
	CompanyRequest
	LogoURL  string `json:"logo_url"`
	CoverURL string `json:"cover_url"`
}

// CreateBranchRequest opens a new branch (draft); slug is generated from the name when empty
type CreateBranchRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// BranchDTO is a branch in the company's list
type BranchDTO struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Status      string     `json:"status"` // draft, published, closed
	Address     string     `json:"address"`
	Province    string     `json:"province"`
	PublicPath  string     `json:"public_path"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ---------- branch settings ----------

// SettingsRequest replaces a branch's own settings. The four overrides are optional: null
// (or absent) means "use the company's".
type SettingsRequest struct {
	Name            string            `json:"name"`
	Slug            string            `json:"slug"` // can change only until the first publish
	CoverKey        string            `json:"cover_key"`
	Description     string            `json:"description"`
	Address         string            `json:"address"`
	Province        string            `json:"province"`
	Phone           string            `json:"phone"`
	MapURL          string            `json:"map_url"`
	SocialLinks     map[string]string `json:"social_links"`
	Timezone        *string           `json:"timezone"`
	BookingMode     *string           `json:"booking_mode"`
	BookingPolicy   *string           `json:"booking_policy"`
	RequireApproval *bool             `json:"require_approval"`
}

// InForce is what applies at the branch after its overrides
type InForce struct {
	Timezone        string `json:"timezone"`
	Currency        string `json:"currency"`
	Language        string `json:"language"`
	BookingMode     string `json:"booking_mode"`
	BookingPolicy   string `json:"booking_policy"`
	RequireApproval bool   `json:"require_approval"`
}

type SettingsDTO struct {
	SettingsRequest
	ID       uint   `json:"id"`
	Status   string `json:"status"`
	CoverURL string `json:"cover_url"`
	// InForce is the result of the branch's overrides on the company settings
	InForce     InForce `json:"in_force"`
	CompanyName string  `json:"company_name"`
	// PublicPath is the shareable booking page path, e.g. /b/riverside
	PublicPath  string     `json:"public_path"`
	PublishedAt *time.Time `json:"published_at"`
}

// ---------- team (owner / admins) ----------

// TeamMemberDTO is one person who works for the company: an admin, or a manager / staff member
// with the branches they work at
type TeamMemberDTO struct {
	UserID   uint        `json:"user_id"`
	MemberNo string      `json:"member_no"` // ENO000007
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Phone    string      `json:"phone"`
	Role     string      `json:"role"` // owner, admin, manager, staff
	Branches []BranchRef `json:"branches"`
	Active   bool        `json:"active"` // false: dashboard access switched off (manager / staff)
}

type BranchRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// TeamMemberRequest changes what someone does: role (admin, manager, staff), their branches
// (manager / staff) and whether their access is on
type TeamMemberRequest struct {
	Role      string `json:"role"`
	BranchIDs []uint `json:"branch_ids"`
	Active    *bool  `json:"active"`
}

type TransferOwnershipRequest struct {
	UserID uint `json:"user_id"`
}

// InviteRequest asks someone to join: role "admin" (every branch) or "manager" / "staff" with branch_ids
type InviteRequest struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	BranchIDs []uint `json:"branch_ids"`
}

type InvitationDTO struct {
	ID        uint        `json:"id"`
	Email     string      `json:"email"`
	Role      string      `json:"role"`
	Branches  []BranchRef `json:"branches"`
	ExpiresAt time.Time   `json:"expires_at"`
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

// MyBranchDTO feeds the dashboard's branch switcher: every branch for the owner and admins
type MyBranchDTO struct {
	BranchID uint   `json:"branch_id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Status   string `json:"status"`
	Role     string `json:"role"` // owner / admin (company) or manager / staff (this branch)
	Active   bool   `json:"active"`
}

// ---------- password reset ----------

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ---------- public ----------

// PublicCompanyDTO is the brand on the public home
type PublicCompanyDTO struct {
	Name        string            `json:"name"`
	LogoURL     string            `json:"logo_url"`
	CoverURL    string            `json:"cover_url"`
	BrandColor  string            `json:"brand_color"`
	Description string            `json:"description"`
	Phone       string            `json:"phone"`
	Email       string            `json:"email"`
	SocialLinks map[string]string `json:"social_links"`
	Language    string            `json:"language"`
	Currency    string            `json:"currency"`
}

// PublicBranchDTO is one branch's booking page: its own details and the settings in force
type PublicBranchDTO struct {
	Name            string            `json:"name"`
	Slug            string            `json:"slug"`
	CompanyName     string            `json:"company_name"`
	Timezone        string            `json:"timezone"`
	Currency        string            `json:"currency"`
	Language        string            `json:"language"`
	LogoURL         string            `json:"logo_url"`
	CoverURL        string            `json:"cover_url"`
	BrandColor      string            `json:"brand_color"`
	Description     string            `json:"description"`
	Address         string            `json:"address"`
	Province        string            `json:"province"`
	Phone           string            `json:"phone"`
	MapURL          string            `json:"map_url"`
	SocialLinks     map[string]string `json:"social_links"`
	BookingPolicy   string            `json:"booking_policy"`
	BookingMode     string            `json:"booking_mode"`
	RequireApproval bool              `json:"require_approval"`
}

// PublicCoachDTO leaves out the coach's private phone number
type PublicCoachDTO struct {
	ID           uint          `json:"id"`
	Name         string        `json:"name"`
	Bio          string        `json:"bio"`
	ImageURL     string        `json:"image_url"`
	Sports       []SportRefDTO `json:"sports"`
	PricePerHour *int64        `json:"price_per_hour"` // nil: no price set yet
	Currency     string        `json:"currency,omitempty"`
}

// PublicResourceDTO is a bookable court / pitch / ring as customers see it, with its hourly price
type PublicResourceDTO struct {
	ID           uint   `json:"id"`
	SportID      uint   `json:"sport_id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	PricePerHour *int64 `json:"price_per_hour"` // nil: no price set yet
	Currency     string `json:"currency,omitempty"`
}

// DirectoryBranchDTO is one branch on the public home
type DirectoryBranchDTO struct {
	Name     string           `json:"name"`
	Slug     string           `json:"slug"`
	CoverURL string           `json:"cover_url"`
	Address  string           `json:"address"`
	Province string           `json:"province"`
	Sports   []DirectorySport `json:"sports"` // active sports only
	// FromPrice is the lowest hourly court price (nil: no court price set yet)
	FromPrice *int64 `json:"from_price"`
	Currency  string `json:"currency"`
}

type DirectorySport struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DirectoryQuery filters the public list of branches; every field is optional
type DirectoryQuery struct {
	Page      int    `query:"page"`
	Limit     int    `query:"limit"`
	Keyword   string `query:"keyword"` // name or address
	SportType string `query:"sport_type"`
	Province  string `query:"province"`
}
