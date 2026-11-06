// Package port: the module's two edges. Input is what the module offers (the handlers, and the booking service
// through core's TeamNotifier); output is what it needs (storage, and Telegram itself).
package port

import (
	"backend/internal/modules/telegram/domain"
	"backend/internal/modules/telegram/dto"
	"context"
	"errors"
	"fmt"
)

// Repository stores a branch's setting
type Repository interface {
	Get(ctx context.Context, branchID uint) (*domain.Setting, error) // nil when never set
	Save(ctx context.Context, s *domain.Setting) error
	Delete(ctx context.Context, branchID uint) error
	// Result records how the last message went ("" = fine); a failure never changes the other fields
	Result(ctx context.Context, branchID uint, lastError string) error
}

// Bot is Telegram's Bot API, as far as this module uses it
type Bot interface {
	// Me checks the token and returns the bot's @username
	Me(ctx context.Context, token string) (username string, err error)
	// Chat checks the bot can see the chat and returns its title
	Chat(ctx context.Context, token, chatID string) (title string, err error)
	// FindGroups lists the groups the bot recently had activity in, latest first; ErrNoGroup when none
	FindGroups(ctx context.Context, token string) ([]Group, error)
	Send(ctx context.Context, token, chatID, text string) error
	// SendPhoto sends a picture with a caption (at most 1024 characters)
	SendPhoto(ctx context.Context, token, chatID, caption string, photo []byte, filename string) error
}

// Service is what the handlers call
type Service interface {
	Get(ctx context.Context, branchID uint) (*dto.SettingDTO, error)
	Save(ctx context.Context, branchID uint, req dto.SaveRequest) (*dto.SettingDTO, error)
	// FindGroups lists the groups the bot recently saw (nothing is saved: the owner picks one with Save)
	FindGroups(ctx context.Context, branchID uint) (*dto.GroupsDTO, error)
	Test(ctx context.Context, branchID uint) (*dto.SettingDTO, error)
	Disconnect(ctx context.Context, branchID uint) error
}

// ErrNoGroup: the bot has not been added to, or heard in, any group
var ErrNoGroup = errors.New("telegram: no group found")

// APIError is what Telegram answered when it refused
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string { return fmt.Sprintf("telegram %d: %s", e.Code, e.Description) }

// Group is a group (or channel) the bot recently had activity in
type Group struct {
	ID    string
	Title string
}
