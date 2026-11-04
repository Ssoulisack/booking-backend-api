package domain

import (
	"time"

	"gorm.io/gorm"
)

// Company roles: the people who run every branch. Stored on the user (users.company_role).
const (
	CompanyOwner = "owner" // one per install; hands over with TransferOwnership
	CompanyAdmin = "admin" // same rights over every branch, except managing admins and ownership
)

// Branch roles, given per branch through a membership
const (
	MemberManager = "manager"
	MemberStaff   = "staff"
)

// MemberRank orders roles for branch access: the company owner and admins can do everything at
// every branch; a manager everything a staff member can
func MemberRank(role string) int {
	switch role {
	case CompanyOwner, CompanyAdmin:
		return 3
	case MemberManager:
		return 2
	case MemberStaff:
		return 1
	}
	return 0
}

// Branch status as shown to users; stored as the Published and Closed flags
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusClosed    = "closed"
)

// How customers may book at a branch
const (
	BookingModeAccount = "account"
	BookingModeGuest   = "guest"
	BookingModeBoth    = "both"
)

// Company is the one client this install belongs to: its brand and the settings every branch
// starts from. There is exactly one row, created at the first start (see migration seed).
type Company struct {
	gorm.Model
	Name       string `gorm:"type:varchar(120);not null"`
	LogoKey    string `gorm:"type:varchar(255)"`
	CoverKey   string `gorm:"type:varchar(255)"`
	BrandColor string `gorm:"type:varchar(7)"`
	// RingColors (up to MaxRingColors, #rrggbb) colour the rings behind the page's top; empty = made from the brand colour
	RingColors    []string          `gorm:"type:jsonb;serializer:json"`
	HeadingFont   string            `gorm:"type:varchar(20)"` // a key of Fonts; "" = the default
	BodyFont      string            `gorm:"type:varchar(20)"`
	Description   string            `gorm:"type:text"`
	Phone         string            `gorm:"type:varchar(30)"`
	Email         string            `gorm:"type:varchar(255)"`
	SocialLinks   map[string]string `gorm:"type:jsonb;serializer:json"`
	Language      string            `gorm:"type:varchar(5);not null;default:en"`
	Currency      string            `gorm:"type:varchar(3);not null;default:LAK"`
	Timezone      string            `gorm:"type:varchar(64);not null;default:Asia/Vientiane"`
	BookingMode   string            `gorm:"type:varchar(10);not null;default:account"`
	BookingPolicy string            `gorm:"type:text"`
	// RequireApproval makes new customer bookings pending until staff confirm them
	RequireApproval bool `gorm:"not null;default:false"`
}

// Branch types
const (
	BranchTypeSport      = "sport"
	BranchTypeRestaurant = "restaurant"
)

// ValidBranchType reports whether t is a known branch type
func ValidBranchType(t string) bool { return t == BranchTypeSport || t == BranchTypeRestaurant }

// Branch is one place of the company with its own booking page. Settings the company sets for
// everyone (time zone, booking mode, policy, approval) can be overridden per branch (nil = the
// company's). Brand and currency always come from the company.
type Branch struct {
	gorm.Model
	// Type is what the branch takes bookings for: BranchTypeSport (courts) or BranchTypeRestaurant
	// (tables). It can change only until the first publish.
	Type        string     `gorm:"type:varchar(12);not null;default:sport;index"`
	Name        string     `gorm:"type:varchar(120);not null"`
	Slug        string     `gorm:"type:varchar(60);not null;uniqueIndex:idx_branches_slug,where:deleted_at IS NULL"`
	Published   bool       `gorm:"not null;default:false"`
	PublishedAt *time.Time // first publish; the slug cannot change after it
	// Closed hides the branch from customers and its team; the owner and admins can reopen it
	Closed bool `gorm:"not null;default:false"`

	// the branch's own details
	CoverKey    string            `gorm:"type:varchar(255)"` // empty: the company cover is shown
	Description string            `gorm:"type:text"`
	Address     string            `gorm:"type:varchar(255)"`
	Province    string            `gorm:"type:varchar(40);index"` // one of Provinces, "" until chosen
	Phone       string            `gorm:"type:varchar(30)"`
	MapURL      string            `gorm:"type:varchar(500)"`
	SocialLinks map[string]string `gorm:"type:jsonb;serializer:json"`
	// SisterBranchID is another branch at the same place, shown as "Also at this place" (nil: none)
	SisterBranchID *uint

	// table settings (restaurant branches only)
	TableMode           string `gorm:"type:varchar(15);not null;default:one_at_a_time"`
	HoldMinutes         int    `gorm:"not null;default:120"` // TableModeHold: how long a booking holds its table
	LateMinutes         int    `gorm:"not null;default:15"`  // not arrived this long after the time: no-show
	WalkinBeforeBooking bool   `gorm:"not null;default:false"`
	MaxGuestsOnline     int    `gorm:"not null;default:20"`
	ApprovalAboveGuests *int   // bigger groups wait for staff approval; nil = never
	LastCallMinutes     int    `gorm:"not null;default:60"` // last online booking this long before closing
	// how customers book: BookingStyleTime (pick a start time) or BookingStyleArriveBy (a bar:
	// only the evening; arrive by ArriveByTime, or within LateBookingMinutes when booking later)
	BookingStyle       string `gorm:"type:varchar(10);not null;default:time"`
	ArriveByTime       string `gorm:"type:varchar(5);not null;default:'19:30'"`
	LateBookingMinutes int    `gorm:"not null;default:15"`

	// overrides of the company settings; nil = use the company's
	OwnTimezone        *string `gorm:"column:timezone;type:varchar(64)"`
	OwnBookingMode     *string `gorm:"column:booking_mode;type:varchar(10)"`
	OwnBookingPolicy   *string `gorm:"column:booking_policy;type:text"`
	OwnRequireApproval *bool   `gorm:"column:require_approval"`
	// the look: nil = the company's
	OwnRingColors  *[]string `gorm:"column:ring_colors;type:jsonb;serializer:json"`
	OwnHeadingFont *string   `gorm:"column:heading_font;type:varchar(20)"`
	OwnBodyFont    *string   `gorm:"column:body_font;type:varchar(20)"`

	// in force: filled from the company by WithCompany (repositories do it on every load)
	CompanyName     string   `gorm:"-"`
	Timezone        string   `gorm:"-"`
	Currency        string   `gorm:"-"`
	Language        string   `gorm:"-"`
	LogoKey         string   `gorm:"-"`
	BrandColor      string   `gorm:"-"`
	BookingMode     string   `gorm:"-"`
	BookingPolicy   string   `gorm:"-"`
	RequireApproval bool     `gorm:"-"`
	RingColors      []string `gorm:"-"`
	HeadingFont     string   `gorm:"-"`
	BodyFont        string   `gorm:"-"`
	// the picture shown for the branch: its own cover, else the company's
	ShownCoverKey string `gorm:"-"`
}

// WithCompany fills the settings in force: the branch's own where it has one, else the company's
func (b *Branch) WithCompany(c Company) {
	b.CompanyName, b.Currency, b.Language = c.Name, c.Currency, c.Language
	b.LogoKey, b.BrandColor = c.LogoKey, c.BrandColor
	b.Timezone, b.BookingMode, b.BookingPolicy, b.RequireApproval = c.Timezone, c.BookingMode, c.BookingPolicy, c.RequireApproval
	if b.OwnTimezone != nil {
		b.Timezone = *b.OwnTimezone
	}
	if b.OwnBookingMode != nil {
		b.BookingMode = *b.OwnBookingMode
	}
	if b.OwnBookingPolicy != nil {
		b.BookingPolicy = *b.OwnBookingPolicy
	}
	if b.OwnRequireApproval != nil {
		b.RequireApproval = *b.OwnRequireApproval
	}
	b.RingColors, b.HeadingFont, b.BodyFont = c.RingColors, c.HeadingFont, c.BodyFont
	if b.OwnRingColors != nil {
		b.RingColors = *b.OwnRingColors
	}
	if b.OwnHeadingFont != nil {
		b.HeadingFont = *b.OwnHeadingFont
	}
	if b.OwnBodyFont != nil {
		b.BodyFont = *b.OwnBodyFont
	}
	b.ShownCoverKey = b.CoverKey
	if b.ShownCoverKey == "" {
		b.ShownCoverKey = c.CoverKey
	}
}

func (b Branch) Status() string {
	switch {
	case b.Closed:
		return StatusClosed
	case b.Published:
		return StatusPublished
	}
	return StatusDraft
}

// IsPublic tells whether the public booking page may be shown
func (b Branch) IsPublic() bool { return b.Published && !b.Closed }

// Membership gives a user a role (manager or staff) at one branch. Removed members are deleted.
type Membership struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uint   `gorm:"not null;uniqueIndex:idx_memberships_user_branch,priority:1"`
	BranchID  uint   `gorm:"not null;uniqueIndex:idx_memberships_user_branch,priority:2;index"`
	Role      string `gorm:"type:varchar(10);not null"`
	Active    bool   `gorm:"not null"` // always set explicitly: gorm would write the default for false
	User      User   `gorm:"foreignKey:UserID"`
	Branch    Branch `gorm:"foreignKey:BranchID"`
}

// PasswordReset is a single-use reset link. Only the token's hash is stored.
type PasswordReset struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UserID    uint      `gorm:"not null;index"`
	TokenHash string    `gorm:"type:varchar(64);not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
}

// Media records an uploaded file; BranchID 0 is a company file (logo, cover)
type Media struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	BranchID  uint   `gorm:"not null;index"`
	Key       string `gorm:"type:varchar(255);not null;uniqueIndex"`
	SizeBytes int64  `gorm:"not null"`
}

// MaxRingColors is how many colours the rings can blend
const MaxRingColors = 4

// Fonts the booking pages can use (bundled with the frontend); "" means the default
var Fonts = map[string]bool{
	"noto-sans": true, "noto-condensed": true, "inter": true, "poppins": true,
	"kanit": true, "prompt": true, "sarabun": true,
}
