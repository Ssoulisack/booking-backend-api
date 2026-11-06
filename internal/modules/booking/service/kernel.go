package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/modules/booking/port"
	"context"
	"time"
)

// What the restaurant module borrows (port.Kernel): the booking module's own helpers, exported.

var _ port.Kernel = (*Service)(nil)

// SetPackages gives the service the packages module, so a booking can be paid with a course pass
func (s *Service) SetPackages(p output.PackageUse) { s.packages = p }

// SetSlips tells the service how to read a branch's slip ways and where an uploaded slip goes (desk nil: no Telegram)
func (s *Service) SetSlips(w output.SlipWays, d output.SlipDesk) { s.slipWays, s.slipDesk = w, d }

// SetTeamNotifier gives the service a way to tell the branch's team about new bookings, payments and cancellations
func (s *Service) SetTeamNotifier(t output.TeamNotifier) { s.team = t }

// SetTables gives the service the module that books restaurant tables
func (s *Service) SetTables(t port.TableBooker) { s.tables = t }

func (s *Service) tableBooker() port.TableBooker {
	if s.tables == nil {
		return noTables{}
	}
	return s.tables
}

// noTables answers when the restaurant module is not wired in
type noTables struct{}

func (noTables) CreateTableBooking(context.Context, *domain.Branch, *port.Booker, dto.CreateBookingRequest) (*domain.Booking, string, error) {
	return nil, "", conflict("NOT_A_RESTAURANT")
}

func (s *Service) Published(ctx context.Context, slug string) (*domain.Branch, error) {
	return s.published(ctx, slug)
}

func (s *Service) Branch(ctx context.Context, branchID uint) (*domain.Branch, *time.Location, error) {
	return s.branch(ctx, branchID)
}

func (s *Service) OpenIntervals(ctx context.Context, branchID uint, ownerType string, ownerID uint, day time.Time) ([]domain.Interval, error) {
	return s.openIntervals(ctx, branchID, ownerType, ownerID, day)
}

func (s *Service) BusyIntervals(ctx context.Context, branchID uint, ownerType string, ownerID uint, from, to time.Time) ([]domain.Interval, error) {
	return s.busyIntervals(ctx, branchID, ownerType, ownerID, from, to)
}

func (s *Service) WhoBooks(b *domain.Branch, actor *domain.User, guest *dto.GuestInfo) (*port.Booker, error) {
	return whoBooks(b, actor, guest)
}

func (s *Service) CleanGuest(g dto.GuestInfo) (*dto.GuestInfo, error) { return cleanGuest(g) }

func (s *Service) NewBooking(b *domain.Branch, who *port.Booker, sportID uint, start, end time.Time, rule *domain.EffectiveRule, notes string) (*domain.Booking, string, error) {
	return s.newBooking(b, who, sportID, start, end, rule, notes)
}

func (s *Service) CheckBookingLimit(ctx context.Context, branchID uint, who *port.Booker, rule *domain.EffectiveRule) error {
	return s.checkBookingLimit(ctx, branchID, who, rule)
}

func (s *Service) Created(ctx context.Context, b *domain.Branch, bk *domain.Booking, rawToken string, actor *domain.User, who *port.Booker) *dto.CreatedBookingDTO {
	return s.created(ctx, b, bk, rawToken, actor, who)
}
