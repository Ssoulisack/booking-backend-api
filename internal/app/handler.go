package app

import (
	"backend/internal/core/mappers"
	"backend/internal/http/handler"
	"backend/internal/platform/middleware"
)

// handler.AuthHandler

type AppHandler struct {
	authMiddleware *middleware.Auth
	auth           handler.AuthHandler
	user           handler.UserHandler
	company        handler.CompanyHandler
	branch         handler.BranchHandler
	invitations    handler.InvitationHandler
	public         handler.PublicHandler
	catalog        handler.CatalogHandler
	schedule       handler.ScheduleHandler
	rules          handler.RuleHandler
	media          handler.MediaHandler
	booking        handler.BookingHandler
	desk           handler.DeskHandler
	reports        handler.ReportHandler
	filesDir       string // local storage folder served at /files
}

func SetupHandlers(svc *AppServices) *AppHandler {
	classicMapper := mappers.NewClassicMapper[any, any]()
	mapperFactory := mappers.NewMapperFactory(classicMapper)
	return &AppHandler{
		filesDir:       svc.filesDir,
		authMiddleware: middleware.NewAuth(svc.access),
		auth:           handler.NewAuthHandler(svc.auth, mapperFactory),
		user:           handler.NewUserHandler(svc.user, mapperFactory),
		company:        handler.NewCompanyHandler(svc.company, svc.rules, svc.media, svc.desk, svc.reports),
		branch:         handler.NewBranchHandler(svc.branch),
		invitations:    handler.NewInvitationHandler(svc.invitations),
		public:         handler.NewPublicHandler(svc.public),
		catalog:        handler.NewCatalogHandler(svc.catalog),
		schedule:       handler.NewScheduleHandler(svc.schedule),
		rules:          handler.NewRuleHandler(svc.rules),
		media:          handler.NewMediaHandler(svc.media),
		booking:        handler.NewBookingHandler(svc.booking),
		desk:           handler.NewDeskHandler(svc.desk),
		reports:        handler.NewReportHandler(svc.reports),
	}
}
