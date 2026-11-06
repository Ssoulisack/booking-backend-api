package app

import (
	"backend/internal/modules/booking"
	"backend/internal/modules/catalog"
	"backend/internal/modules/company"
	"backend/internal/modules/events"
	"backend/internal/modules/identity"
	"backend/internal/modules/media"
	"backend/internal/modules/packages"
	"backend/internal/modules/payments"
	"backend/internal/modules/payments/adapter/mock"
	"backend/internal/modules/payments/adapter/phajay"
	paymentsport "backend/internal/modules/payments/port"
	"backend/internal/modules/reports"
	"backend/internal/modules/restaurant"
	"backend/internal/modules/telegram"
	"backend/internal/platform/config"
	"context"
	"log"
	"strings"
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
	packages   *packages.Module
	telegram   *telegram.Module // nil when the server secret is too short to encrypt with
}

// publicLimits are the abuse limits of the public booking page: what config.yaml says, or when it says 0 the default of
// the environment (10 per IP per hour and 3 waiting per phone; dev has no real limit, because every test comes from one address)
func publicLimits(srv config.ServerConfig) (l struct{ bookingsPerHour, waitingPerPhone int }) {
	l.bookingsPerHour, l.waitingPerPhone = srv.PublicBookingsPerHour, srv.OpenWaitingPerPhone
	if srv.Env == "" || srv.Env == "dev" {
		if l.bookingsPerHour <= 0 {
			l.bookingsPerHour = 100000
		}
		if l.waitingPerPhone <= 0 {
			l.waitingPerPhone = 1000
		}
	}
	return l
}

// SetupServices initializes all core services
func SetupServices(repos *AppRepositories) *AppServices {
	r := repos
	catalogModule := catalog.New(catalog.Deps{
		Sports: r.sports, Resources: r.resources, Coaches: r.coaches, Classes: r.classSessions, Bookings: r.bookings,
		Branches: r.branches, FloorItems: r.floorItems, Hours: r.openingHours, Closures: r.closures, Prices: r.prices,
		Timetables: r.timetables, Rules: r.rules, Storage: r.storage, Media: r.media,
	})
	limits := publicLimits(r.server)
	bookingModule := booking.New(booking.Deps{
		Branches: r.branches, Sports: r.sports, Resources: r.resources, Coaches: r.coaches, Hours: r.openingHours,
		Closures: r.closures, Prices: r.prices, Classes: r.classSessions, Bookings: r.bookings, Users: r.user,
		Memberships: r.memberships, Rules: catalogModule.Rules, Mailer: r.mailer, FrontendURL: r.app.FrontendURL, FloorItems: r.floorItems,
		OpenWaitingPerPhone: limits.waitingPerPhone, PublicBookingsPerHour: limits.bookingsPerHour,
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
		Files: r.storage, Booker: restaurantModule.Service, Notifier: bookingImpl, Cache: r.cache,
	})
	restaurantModule.SetEventDays(eventsModule.Service)
	packagesModule := packages.New(packages.Deps{DB: r.postgres.GetDB(), Branches: r.branches, Sports: r.sports, Resources: r.resources, Timetables: r.timetables, Users: r.user})
	// online payment: server.env picks the gateway (dev: the built-in mock, uat: PhaJay's test account,
	// prod: PhaJay for real money); a PhaJay without its keys is off and the QR / transfer / WhatsApp way stays
	gateways, devGateway := ChooseGateway(r.server, r.payments)
	paymentsModule := payments.New(payments.Deps{
		DB: r.postgres.GetDB(), Providers: gateways,
		Branches: r.branches, Bookings: bookingImpl, Purchases: packagesModule.Service, Dev: devGateway, Cache: r.cache,
	})
	// Telegram: bots and groups are connected on the website; what is stored is encrypted with a key made from the
	// server's secret. Too short a secret switches the feature off (the booking flows then tell nobody).
	telegramModule, err := telegram.New(telegram.Deps{
		DB: r.postgres.GetDB(), Bookings: r.bookings, APIBase: r.telegram.APIBase,
		Secret: firstNonEmpty(r.telegram.EncryptionKey, r.jwtSecret),
	})
	if err != nil {
		log.Printf("⚠️  Telegram notifications are off: %v", err)
		telegramModule = nil
	} else {
		bookingImpl.SetTeamNotifier(telegramModule.Service)
	}
	// a booking can be paid with a customer's package (course pass)
	bookingImpl.SetPackages(packagesModule.Service)
	// payment slips: how a customer may send one (the branch's "how to pay" setting) and, by Telegram, where it goes
	if telegramModule != nil {
		eventsModule.SetSlipChannel(telegramModule.Service)
		packagesModule.SetSlips(eventsModule.Service, telegramModule.Service)
		packagesModule.SetNews(telegramModule.Service)
		bookingImpl.SetSlips(eventsModule.Service, telegramModule.Service)
	} else {
		packagesModule.SetSlips(eventsModule.Service, nil)
		bookingImpl.SetSlips(eventsModule.Service, nil)
	}
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
		packages:   packagesModule,
		telegram:   telegramModule,
		filesDir:   r.filesDir,
		identity:   identityModule,
		company:    companyModule,
		catalog:    catalogModule,
		media:      media.New(media.Deps{Storage: r.storage, Media: r.media}),
		booking:    bookingModule,
		reports:    reports.New(reports.Deps{Branches: r.branches, Bookings: r.bookings, Resources: r.resources, Sports: r.sports, Hours: r.openingHours, Closures: r.closures, Rows: r.bookingReports, Packages: packagesModule.Service}),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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

// KeepTablesSettled runs the background jobs. Every minute: event tables not paid in time (and without a
// slip sent) are cancelled and go back on sale, so a booking is released within a minute of its countdown.
// Every 5 minutes: restaurant groups not there by their arrive-by time become no-shows (their tables free at
// once), evenings that are over get their bills closed, and a payment whose webhook never came is asked about
// at the gateway.
func KeepTablesSettled(svc *AppServices) {
	go func() {
		holds := time.NewTicker(time.Minute)
		settle := time.NewTicker(5 * time.Minute)
		for {
			select {
			case <-holds.C:
				if err := svc.events.Service.ExpireUnpaid(context.Background()); err != nil {
					log.Printf("⚠️  event holds: %v", err)
				}
			case <-settle.C:
				if err := svc.restaurant.Service.SettleTables(context.Background()); err != nil {
					log.Printf("⚠️  settling tables: %v", err)
				}
				if err := svc.payments.Service.Poll(context.Background()); err != nil {
					log.Printf("⚠️  payment status check: %v", err)
				}
			}
		}
	}()
}

// Models are the tables the modules own, for the migration (make migrate)
func Models() []any {
	return append(append(append(append(events.Models(), payments.Models()...), identity.Models()...), telegram.Models()...), packages.Models()...)
}

// SettleEventHolds cancels event bookings not paid in time (KeepTablesSettled runs it every minute)
func SettleEventHolds(svc *AppServices) error {
	return svc.events.Service.ExpireUnpaid(context.Background())
}

// PollPayments asks the gateways about payments still waiting (KeepTablesSettled runs it every 5 minutes)
func PollPayments(svc *AppServices) error {
	return svc.payments.Service.Poll(context.Background())
}

// ChooseGateway turns server.env into the payment gateways of this run. Each environment uses only its own
// keys, so a key left in config.yaml can never be used by the wrong one: dev never calls PhaJay.
func ChooseGateway(srv config.ServerConfig, pay config.PaymentsConfig) ([]paymentsport.Provider, *mock.Mock) {
	phajayCfg := pay.PhaJay
	switch env := srv.Mode(); env {
	case config.EnvDev:
		m := mock.New(mock.Config{BaseURL: "http://localhost:" + srv.Port, DailyLimit: pay.Mock.DailyLimit})
		log.Printf("💳 payments: dev, built-in mock gateway (PhaJay is never called); dev page at /api/v1/dev/payments")
		return []paymentsport.Provider{m}, m
	case config.EnvUAT:
		phajayCfg.SecretKey = "" // the production key is not for the test account
		if phajayCfg.TestKey == "" {
			log.Printf("⚠️  payments: uat needs payments.phajay.testKey: online payment is off")
			return nil, nil
		}
		log.Printf("💳 payments: uat, PhaJay test account (%s)", phajay.KeyInfo(phajayCfg))
	default: // prod
		phajayCfg.TestKey, phajayCfg.DevSecretKey, phajayCfg.TestAmountDivisor = "", "", 0 // real money is always the real amount
		if phajayCfg.SecretKey == "" {
			log.Printf("⚠️  payments: prod has no payments.phajay.secretKey (needs KYC): online payment is off")
			return nil, nil
		}
		log.Printf("💳 payments: prod, PhaJay with real money (%s)", phajay.KeyInfo(phajayCfg))
	}
	p := phajay.New(phajayCfg)
	if n := p.TestScale(); n > 0 {
		log.Printf("⚠️  PhaJay TEST AMOUNTS: the gateway is asked for the amount divided by %d (test account, before KYC). Never use this with real money.", n)
	}
	return []paymentsport.Provider{p}, nil
}
