package domain

import (
	"time"

	"gorm.io/gorm"
)

// User is the gorm model for the users table
type User struct {
	gorm.Model
	Email        string `gorm:"type:varchar(255);uniqueIndex"`
	Username     string `gorm:"type:varchar(255);uniqueIndex"`
	PasswordHash []byte
	Name         string `gorm:"type:varchar(255);not null"`
	Avatar       string `gorm:"type:varchar(255)"`
	Phone        string `gorm:"type:varchar(20)"` // 20xxxxxxxx (Lao mobile); one account per number, see migration
	// ISO 3166-1 code chosen at registration ("LA", "TH"…); empty for older accounts until they choose
	Nationality string `gorm:"type:varchar(2);not null;default:''"`
	// the email is a stand-in (no real address yet): never shown to staff, never sent to
	EmailGenerated bool `gorm:"not null;default:false"`
	// CompanyRole is "owner" or "admin" for the people who run every branch; empty for everyone else
	CompanyRole string `gorm:"type:varchar(10);not null;default:''"`
	Verified    bool   `gorm:"type:boolean;not null;default:false"`
	Status      bool   `gorm:"type:boolean;not null;default:true"`
	LastActive  time.Time
	// tokens issued before this moment are rejected (set on password reset)
	PasswordChangedAt *time.Time
}

// RunsCompany is true for the company owner and admins: every branch, the company settings, the team
func (u User) RunsCompany() bool {
	return u.CompanyRole == CompanyOwner || u.CompanyRole == CompanyAdmin
}

func (u User) IsOwner() bool { return u.CompanyRole == CompanyOwner }

// ContactEmail is the address to show or write to: empty when the account has only a stand-in
func (u User) ContactEmail() string {
	if u.EmailGenerated || IsGeneratedEmail(u.Email) {
		return ""
	}
	return u.Email
}
