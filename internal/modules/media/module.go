// Package media: the pictures people upload (logos, covers, courts, coaches, classes) and the media
// library that shows where each one is used. Other modules keep only a picture key and ask the shared
// helpers in platform/kit to check and show it.
package media

import (
	"backend/internal/core/port/output"
	"backend/internal/modules/media/adapter/http"
	"backend/internal/modules/media/port"
	"backend/internal/modules/media/service"
	"backend/internal/platform/middleware"

	"github.com/gofiber/fiber/v2"
)

// Deps are what the module is built from
type Deps struct {
	Storage output.FileStorage
	Media   output.MediaRepository
}

// Module is the media module
type Module struct {
	Service port.MediaService
	handler *http.Handler
}

// New builds the module
func New(d Deps) *Module {
	svc := service.NewMediaService(d.Storage, d.Media)
	return &Module{Service: svc, handler: http.New(svc)}
}

// Routes registers the module's routes
func (m *Module) Routes(api fiber.Router, auth *middleware.Auth) { m.handler.Routes(api, auth) }
