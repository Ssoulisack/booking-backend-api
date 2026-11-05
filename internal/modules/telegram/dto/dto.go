// Package dto: what the Telegram settings page sends and receives. There is no token or group id in any answer.
package dto

import "time"

// SaveRequest changes a branch's connection. Fields not sent stay as they are.
type SaveRequest struct {
	// BotToken from @BotFather; sent only when it is new (it is checked with Telegram, then kept encrypted)
	BotToken *string `json:"bot_token"`
	// ChatID typed by hand ("-1001234567890" or "@channel"); usually found with "Find my group" instead
	ChatID    *string `json:"chat_id"`
	Enabled   *bool   `json:"enabled"`
	OnBooking *bool   `json:"on_booking"`
	OnEvent   *bool   `json:"on_event"`
	OnPayment *bool   `json:"on_payment"`
	OnCancel  *bool   `json:"on_cancel"`
	HidePhone *bool   `json:"hide_phone"`
}

// GroupDTO is a group the bot recently saw
type GroupDTO struct {
	ID      string `json:"id"` // the group id: not a secret to the admins who see it in Telegram, and never stored in plain
	Title   string `json:"title"`
	Current bool   `json:"current"` // the group this branch sends to now
}

// GroupsDTO: the groups found, the one with the latest activity first
type GroupsDTO struct {
	Groups []GroupDTO `json:"groups"`
}

// SettingDTO is the page: whether a bot and a group are connected (never their secrets), and the switches
type SettingDTO struct {
	HasBot    bool       `json:"has_bot"`
	BotName   string     `json:"bot_name"`
	HasGroup  bool       `json:"has_group"`
	ChatTitle string     `json:"chat_title"`
	Enabled   bool       `json:"enabled"`
	OnBooking bool       `json:"on_booking"`
	OnEvent   bool       `json:"on_event"`
	OnPayment bool       `json:"on_payment"`
	OnCancel  bool       `json:"on_cancel"`
	HidePhone bool       `json:"hide_phone"`
	LastError string     `json:"last_error"`
	LastOKAt  *time.Time `json:"last_ok_at,omitempty"`
}
