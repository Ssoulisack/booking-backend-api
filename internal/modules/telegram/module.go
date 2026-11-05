// Package telegram: a branch's Telegram group gets a message when a booking, an event booking, a payment or a
// cancellation happens. The owner connects the bot on the website (bot token from @BotFather; the group is found
// by adding the bot to it); the token and the group id are stored encrypted. The service also implements
// core's TeamNotifier, which the booking service calls.
package telegram

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/telegram/adapter/botapi"
	"backend/internal/modules/telegram/adapter/http"
	"backend/internal/modules/telegram/adapter/postgres"
	"backend/internal/modules/telegram/domain"
	"backend/internal/modules/telegram/port"
	"backend/internal/modules/telegram/service"
	"backend/internal/platform/middleware"
	"backend/internal/platform/secretbox"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Deps are what the module is built from
type Deps struct {
	DB       *gorm.DB
	Bookings output.BookingRepository
	// Secret is the server secret the encryption key is made from; APIBase is Telegram's address ("" = the real one)
	Secret  string
	APIBase string
}

// Module is the Telegram module
type Module struct {
	Service *service.Service
	handler *http.Handler
}

// New builds the module. It fails when the server secret is too short to encrypt with.
func New(d Deps) (*Module, error) {
	box, err := secretbox.New(d.Secret, "telegram")
	if err != nil {
		return nil, err
	}
	svc := service.New(service.Deps{
		Repo: postgres.New(d.DB), Bot: botapi.New(d.APIBase), Box: box, Bookings: d.Bookings,
	})
	return &Module{Service: svc, handler: http.New(svc)}, nil
}

var _ port.Service = (*service.Service)(nil)

// Routes registers the settings routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }

// Models are the tables of this module, for the migration
func Models() []any { return []any{&domain.Setting{}} }
