// Package domain: a branch's Telegram connection. The bot token and the group id are secrets: they are kept
// encrypted (see platform/secretbox) and never leave the server in an answer.
package domain

import "time"

// Setting is how one branch tells its team about bookings in a Telegram group: the bot, the group, and what to tell.
// Booleans have no gorm defaults: every field is written explicitly.
type Setting struct {
	BranchID uint `gorm:"primaryKey"`
	// TokenEnc and ChatIDEnc are sealed with secretbox: never the plain values
	TokenEnc  string `gorm:"type:text;not null"`
	ChatIDEnc string `gorm:"type:text;not null"`
	// BotName ("@my_bot") and ChatTitle ("Staff group") are public names, shown on the page
	BotName   string `gorm:"type:varchar(80);not null"`
	ChatTitle string `gorm:"type:varchar(200);not null"`

	Enabled   bool // the master switch
	OnBooking bool // a new table or sport booking
	OnEvent   bool // a new event booking
	OnPayment bool // an online payment received, or a customer says the slip was sent
	OnCancel  bool // a booking cancelled by the customer, or released because it was not paid in time
	// the team's own actions, announced too (so the whole team sees them). Existing branches get them ON: the column
	// default is true, and Save writes them explicitly (see the repository)
	OnStaffCancel bool `gorm:"not null;default:true"` // a booking cancelled by the team
	OnNoShow      bool `gorm:"not null;default:true"` // a group that did not come (the team marked it, or it was not there by its time)
	HidePhone     bool // leave the customer's phone number out of the messages

	LastError string `gorm:"type:varchar(300);not null"` // what Telegram last answered when sending failed; "" = fine
	LastOKAt  *time.Time
	UpdatedAt time.Time
	CreatedAt time.Time
}

func (Setting) TableName() string { return "telegram_settings" }
