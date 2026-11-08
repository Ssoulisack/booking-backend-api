package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	bookingport "backend/internal/modules/booking/port"
	catalogport "backend/internal/modules/catalog/port"
	"backend/internal/modules/restaurant/port"
	"backend/internal/platform/kit"
	"context"
	"time"
)

const (
	maxGuestName  = 120
	maxNotesChars = 1000
)

// Deps are what the service is built from
type Deps struct {
	Kernel     bookingport.Kernel // the booking row, the opening hours, who is booking
	Sports     output.SportRepository
	Resources  output.ResourceRepository
	Closures   output.ClosureRepository
	Bookings   output.BookingRepository
	FloorItems output.FloorItemRepository
	Rules      catalogport.RuleService
}

// Service books restaurant tables: the floor plans and free tables customers see, the table booking
// itself, the staff desk (arrived, move, walk-in) and the job that settles late groups. It
// implements port.TableService, port.DeskTables, bookingport.TableBooker and the events module's Booker.
type Service struct {
	kernel     bookingport.Kernel
	sports     output.SportRepository
	resources  output.ResourceRepository
	closures   output.ClosureRepository
	bookings   output.BookingRepository
	floorItems output.FloorItemRepository
	rules      catalogport.RuleService
	eventDays  output.EventDays    // set when the events module is on (SetEventDays)
	team       output.TeamNotifier // tells the team (Telegram) about a group that did not come in time; nil = nobody
	now        func() time.Time
}

var (
	_ port.TableService       = (*Service)(nil)
	_ port.DeskTables         = (*Service)(nil)
	_ bookingport.TableBooker = (*Service)(nil)
)

// New builds the service
func New(d Deps) *Service {
	return &Service{
		kernel: d.Kernel, sports: d.Sports, resources: d.Resources, closures: d.Closures, bookings: d.Bookings,
		floorItems: d.FloorItems, rules: d.Rules, now: time.Now,
	}
}

// CreateTableBooking is the booking module's Create for a restaurant branch (port.TableBooker)
func (s *Service) CreateTableBooking(ctx context.Context, b *domain.Branch, who *bookingport.Booker, req dto.CreateBookingRequest) (*domain.Booking, string, error) {
	return s.createTableBooking(ctx, b, who, req)
}

func conflict(code string) error   { return kit.Conflict(code) }
func badRequest(code string) error { return kit.BadRequest(code) }

func mapSlice[M, D any](items []M, convert func(M) D) []D { return kit.MapSlice(items, convert) }

func locationOf(b *domain.Branch) *time.Location { return kit.LocationOf(b) }
func startOfDay(t time.Time, loc *time.Location) time.Time {
	return kit.StartOfDay(t, loc)
}
func parseDay(date string, loc *time.Location) (time.Time, error) { return kit.ParseDay(date, loc) }
func parseClock(s string, allowMidnightEnd bool) (int, error) {
	return kit.ParseClock(s, allowMidnightEnd)
}
func minTime(a, b time.Time) time.Time { return kit.MinTime(a, b) }

func floorItemDTO(f domain.FloorItem) dto.FloorItemDTO {
	return dto.FloorItemDTO{ID: f.ID, Kind: f.Kind, Label: f.Label, Color: f.Color,
		PlanX: f.PlanX, PlanY: f.PlanY, PlanW: f.PlanW, PlanH: f.PlanH, Rotation: f.Rotation, TargetFloor: f.TargetFloor,
		Shape: f.Shape, CutW: f.CutW, CutH: f.CutH, CutCorner: f.CutCorner}
}

// SetTeamNotifier gives the service a way to tell the team when a group did not come by its time
func (s *Service) SetTeamNotifier(n output.TeamNotifier) { s.team = n }
