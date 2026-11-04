package app

import (
	"backend/internal/modules/booking"
	"backend/internal/modules/catalog"
	"backend/internal/modules/company"
	"backend/internal/modules/events"
	"backend/internal/modules/identity"
	"backend/internal/modules/media"
	"backend/internal/modules/payments"
	"backend/internal/modules/payments/adapter/phajay"
	paymentsport "backend/internal/modules/payments/port"
	"backend/internal/modules/reports"
	"backend/internal/modules/restaurant"
	"context"
	"log"
	"time"
)

// AppServices holds all application services
type AppServices struct {
	filesDir   string // local storage folder served at /files
	identity   *identity.Module
	company    *company.Module
	catalog    *catalog.Module
	media      *media.Module
	booking    *booking.Module
	reports    *reports.Module
	restaurant *restaurant.Module
	events     *events.Module
	payments   *payments.Module
}

// SetupServices initializes all core services
func SetupServices(repos *AppRepositories) *AppServices {
	r := repos
	catalogModule := catalog.New(catalog.Deps{
		Sports: r.sports, Resources: r.resources, Coaches: r.coaches, Classes: r.classSessions, Bookings: r.bookings,
		Branches: r.branches, FloorItems: r.floorItems, Hours: r.openingHours, Closures: r.closures, Prices: r.prices,
		Timetables: r.timetables, Rules: r.rules, Storage: r.storage, Media: r.media,
	})
	bookingModule := booking.New(booking.Deps{
		Branches: r.branches, Sports: r.sports, Resources: r.resources, Coaches: r.coaches, Hours: r.openingHours,
		Closures: r.closures, Prices: r.prices, Classes: r.classSessions, Bookings: r.bookings, Users: r.user,
		Memberships: r.memberships, Rules: catalogModule.Rules, Mailer: r.mailer, FrontendURL: r.app.FrontendURL, FloorItems: r.floorItems,
	})
	// modules: each owns its tables, rules and routes; the booking service and the jobs talk to them
	bookingImpl := bookingModule.Service
	restaurantModule := restaurant.New(restaurant.Deps{
		Kernel: bookingImpl, Sports: r.sports, Resources: r.resources, Closures: r.closures, Bookings: r.bookings,
		FloorItems: r.floorItems, Rules: catalogModule.Rules,
	})
	bookingImpl.SetTables(restaurantModule.Service)
	eventsModule := events.New(events.Deps{
		DB: r.postgres.GetDB(), Branches: r.branches, Sports: r.sports,
		Resources: r.resources, FloorItems: r.floorItems, Bookings: r.bookings, Media: r.media,
		Files: r.storage, Booker: restaurantModule.Service,
	})
	restaurantModule.SetEventDays(eventsModule.Service)
	// online payment: a gateway without keys is off (the QR / transfer / WhatsApp way stays)
	var gateways []paymentsport.Provider
	if p := phajay.New(r.payments.PhaJay); p != nil {
		gateways = append(gateways, p)
		log.Printf("💳 PhaJay payments on (%s)", phajay.KeyInfo(r.payments.PhaJay))
	} else {
		log.Printf("💳 online payments off (no gateway keys in config.yaml)")
	}
	paymentsModule := payments.New(payments.Deps{
		DB: r.postgres.GetDB(), Providers: gateways,
		Branches: r.branches, Bookings: bookingImpl,
	})
	identityModule := identity.New(identity.Deps{
		Users: r.user, Cache: r.redis, Memberships: r.memberships, Branches: r.branches, Company: r.company,
		PasswordResets: r.passwordResets, Social: r.social, DB: r.postgres.GetDB(),
		Mailer: r.mailer, FrontendURL: r.app.FrontendURL,
	})
	companyModule := company.New(company.Deps{
		Company: r.company, Branches: r.branches, Directory: r.directory, Memberships: r.memberships,
		Users: r.user, Sports: r.sports, Resources: r.resources, Coaches: r.coaches, Classes: r.classSessions, Bookings: r.bookings,
		Prices: r.prices, OpeningHours: r.openingHours, Rules: r.rules, PasswordResets: r.passwordResets,
		Storage: r.storage, Media: r.media, Mailer: r.mailer, FrontendURL: r.app.FrontendURL,
		Install: company.InstallConfig{
			CompanyName: r.install.Name, OwnerName: r.install.OwnerName,
			OwnerEmail: r.install.OwnerEmail, OwnerPassword: r.install.OwnerPassword,
		},
		Customers: bookingModule.Desk(),
	})
	return &AppServices{
		restaurant: restaurantModule,
		events:     eventsModule,
		payments:   paymentsModule,
		filesDir:   r.filesDir,
		identity:   identityModule,
		company:    companyModule,
		catalog:    catalogModule,
		media:      media.New(media.Deps{Storage: r.storage, Media: r.media}),
		booking:    bookingModule,
		reports:    reports.New(reports.Deps{Branches: r.branches, Bookings: r.bookings, Resources: r.resources, Sports: r.sports, Hours: r.openingHours, Closures: r.closures}),
	}
}

// Install prepares a fresh install (company, its booking rule, the first owner); safe on every start
func Install(svc *AppServices) error {
	return svc.company.EnsureInstalled(context.Background())
}

// RollTimetables makes the missing timetable classes once (KeepTimetablesRolling runs it hourly)
func RollTimetables(svc *AppServices) error {
	return svc.catalog.RollTimetables(context.Background())
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

// KeepTablesSettled runs every 5 minutes: restaurant groups not there by their arrive-by time become
// no-shows (their tables free at once), evenings that are over get their bills closed, and event
// tables not paid in time (and without a slip sent) go back on sale
func KeepTablesSettled(svc *AppServices) {
	go func() {
		for range time.Tick(5 * time.Minute) {
			if err := svc.restaurant.Service.SettleTables(context.Background()); err != nil {
				log.Printf("⚠️  settling tables: %v", err)
			}
			if err := svc.events.Service.ExpireUnpaid(context.Background()); err != nil {
				log.Printf("⚠️  event holds: %v", err)
			}
			// a payment whose webhook never came is asked about at the gateway
			if err := svc.payments.Service.Poll(context.Background()); err != nil {
				log.Printf("⚠️  payment status check: %v", err)
			}
		}
	}()
}

// Models are the tables the modules own, for the migration (make migrate)
func Models() []any {
	return append(append(events.Models(), payments.Models()...), identity.Models()...)
}

// SettleEventHolds cancels event bookings not paid in time (KeepTablesSettled runs it every 5 minutes)
func SettleEventHolds(svc *AppServices) error {
	return svc.events.Service.ExpireUnpaid(context.Background())
}

// PollPayments asks the gateways about payments still waiting (KeepTablesSettled runs it every 5 minutes)
func PollPayments(svc *AppServices) error {
	return svc.payments.Service.Poll(context.Background())
}
