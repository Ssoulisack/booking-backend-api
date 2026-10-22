package application

import (
	"backend/internal/core/port/input"
	"backend/internal/core/services"
	"context"
	"log"
	"time"
)

// AppServices holds all application services
type AppServices struct {
	user        input.UserService
	auth        input.AuthService
	access      input.AccessService
	company     input.CompanyService
	install     input.InstallService
	branch      input.BranchService
	invitations input.InvitationService
	public      input.PublicService
	catalog     input.CatalogService
	schedule    input.ScheduleService
	rules       input.RuleService
	media       input.MediaService
	booking     input.BookingService
	desk        input.DeskService
	reports     input.ReportService
}

// SetupServices initializes all core services
func SetupServices(repos *AppRepositories) *AppServices {
	r := repos
	rules := services.NewRuleService(r.rules, r.sports, r.resources)
	booking := services.NewBookingService(
		r.branches, r.sports, r.resources, r.coaches, r.openingHours, r.closures, r.prices, r.classSessions,
		r.bookings, r.user, rules, r.mailer, r.app.FrontendURL,
	)
	return &AppServices{
		user:   services.NewUserService(r.user, r.redis, r.minio),
		auth:   services.NewAuthService(r.user, r.redis, r.memberships, r.branches, r.passwordResets, r.mailer, r.app.FrontendURL),
		access: services.NewAccessService(r.user, r.memberships, r.branches),
		company: services.NewCompanyService(
			r.company, r.branches, r.memberships, r.invitations, r.user, r.storage, r.media, r.mailer, r.app.FrontendURL,
		),
		install: services.NewInstallService(r.company, r.user, r.rules, r.passwordResets, r.mailer, r.app.FrontendURL,
			services.InstallConfig{
				CompanyName: r.install.Name, OwnerName: r.install.OwnerName,
				OwnerEmail: r.install.OwnerEmail, OwnerPassword: r.install.OwnerPassword,
			}),
		invitations: services.NewInvitationService(r.invitations, r.memberships, r.branches, r.company, r.user),
		public: services.NewPublicService(r.company, r.branches, r.directory, r.sports, r.resources, r.coaches,
			r.classSessions, r.bookings, r.prices, r.storage),
		branch:  services.NewBranchService(r.branches, r.sports, r.resources, r.openingHours, r.prices, r.storage, r.media),
		catalog: services.NewCatalogService(r.sports, r.resources, r.coaches, r.classSessions, r.bookings, r.storage, r.media),
		schedule: services.NewScheduleService(
			r.openingHours, r.closures, r.prices, r.classSessions, r.sports, r.resources, r.coaches, r.bookings, r.branches,
			r.timetables,
		),
		rules:   rules,
		media:   services.NewMediaService(r.storage, r.media),
		booking: booking,
		desk:    booking.(input.DeskService), // the same service: staff bookings share the customer checks
		reports: services.NewReportService(r.branches, r.bookings, r.resources, r.sports, r.openingHours, r.closures),
	}
}

// Install prepares a fresh install (company, its booking rule, the first owner); safe on every start
func Install(svc *AppServices) error {
	return svc.install.EnsureInstalled(context.Background())
}

// RollTimetables makes the missing timetable classes once (KeepTimetablesRolling runs it hourly)
func RollTimetables(svc *AppServices) error {
	return svc.schedule.RollTimetables(context.Background())
}

// KeepTimetablesRolling makes the classes of every weekly timetable ClassWindowDays ahead: once now
// and then every hour, so "Mon–Sat 08:00" never runs out. Closed branches are skipped.
func KeepTimetablesRolling(svc *AppServices) {
	roll := func() {
		if err := RollTimetables(svc); err != nil {
			log.Printf("⚠️  class timetables: %v", err)
		}
	}
	roll()
	go func() {
		for range time.Tick(time.Hour) {
			roll()
		}
	}()
}
