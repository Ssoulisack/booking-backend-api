package domain

import "time"

// Social sign-in providers
const (
	ProviderGoogle   = "google"
	ProviderFacebook = "facebook"
)

// ValidProvider reports a provider the app can sign people in with
func ValidProvider(p string) bool { return p == ProviderGoogle || p == ProviderFacebook }

// SocialIdentity links a Google or Facebook account to a user: one provider account belongs to one user
type SocialIdentity struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UserID      uint   `gorm:"not null;index"`
	Provider    string `gorm:"type:varchar(20);not null;uniqueIndex:idx_social_provider_uid,priority:1"`
	ProviderUID string `gorm:"type:varchar(100);not null;uniqueIndex:idx_social_provider_uid,priority:2"`
	Email       string `gorm:"type:varchar(255)"`
}

// SocialEmail is the stand-in address of an account that came from a provider without sharing an email
func SocialEmail(provider, uid string) string {
	return provider + "." + uid + "@" + GeneratedEmailDomain
}
