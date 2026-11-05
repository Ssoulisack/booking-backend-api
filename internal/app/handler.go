package app

import (
	"backend/internal/modules/booking"
	"backend/internal/modules/catalog"
	"backend/internal/modules/company"
	"backend/internal/modules/events"
	"backend/internal/modules/identity"
	"backend/internal/modules/media"
	"backend/internal/modules/payments"
	"backend/internal/modules/reports"
	"backend/internal/modules/restaurant"
	"backend/internal/modules/telegram"
	"backend/internal/platform/middleware"
)

type AppHandler struct {
	authMiddleware *middleware.Auth
	identity       *identity.Module
	company        *company.Module
	catalog        *catalog.Module
	media          *media.Module
	booking        *booking.Module
	reports        *reports.Module
	restaurant     *restaurant.Module
	events         *events.Module
	payments       *payments.Module
	telegram       *telegram.Module
	filesDir       string // local storage folder served at /files
}

func SetupHandlers(svc *AppServices) *AppHandler {
	return &AppHandler{
		filesDir:       svc.filesDir,
		authMiddleware: svc.identity.Middleware(),
		identity:       svc.identity,
		company:        svc.company,
		catalog:        svc.catalog,
		media:          svc.media,
		booking:        svc.booking,
		reports:        svc.reports,
		restaurant:     svc.restaurant,
		events:         svc.events,
		payments:       svc.payments,
		telegram:       svc.telegram,
	}
}
