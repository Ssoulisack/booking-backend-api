// Package domain: the identity module's own records — invitations and the links between a person and
// their Google / Facebook account. Accounts (User) stay in core/domain: every module needs them.
package domain

import (
	"time"

	coredomain "backend/internal/core/domain"
	"backend/internal/platform/social"
)

// Social sign-in providers
const (
	ProviderGoogle   = social.Google
	ProviderFacebook = social.Facebook
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
	return provider + "." + uid + "@" + coredomain.GeneratedEmailDomain
}

// Invitation is a pending email invite to join the team: as an admin (every branch) or as a
// manager / staff member at the chosen branches. Only the token's hash is stored.
type Invitation struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Email       string    `gorm:"type:varchar(255);not null;index"`
	Role        string    `gorm:"type:varchar(10);not null"`
	BranchIDs   []uint    `gorm:"type:jsonb;serializer:json"` // empty for an admin
	TokenHash   string    `gorm:"type:varchar(64);not null;uniqueIndex"`
	ExpiresAt   time.Time `gorm:"not null"`
	AcceptedAt  *time.Time
	InvitedByID uint `gorm:"not null"`
}

// EmailCode is the 6-digit code sent to someone's email to prove it is theirs: one live code per person, for the
// email the account had when it was sent
type EmailCode struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UserID    uint      `gorm:"not null;uniqueIndex"`
	Email     string    `gorm:"type:varchar(255);not null"`
	CodeHash  string    `gorm:"type:varchar(64);not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Attempts  int       `gorm:"not null;default:0"`
}
