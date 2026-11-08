package service

import (
	"backend/internal/core/domain"
	coredto "backend/internal/core/dto"
	"backend/internal/core/port/output"
	evdomain "backend/internal/modules/events/domain"
	"backend/internal/modules/events/dto"
	"backend/internal/modules/events/port"
	"backend/pkg/response"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"
)

func badRequest(code string) error { return response.ErrorBadRequest(code) }
func conflict(code string) error   { return response.NewError(http.StatusConflict, code) }
func notFound(code string) error   { return response.NewError(http.StatusNotFound, code) }

const (
	maxSpanDays   = 31
	maxNewTables  = 200
	maxPrice      = 100_000_000_000
	defaultHold   = 24 * 60
	defaultBigN   = 4
	defaultBigMin = 60
)

// Deps are what the service is built from
type Deps struct {
	Repo       port.Repository
	Branches   output.BranchRepository
	Sports     output.SportRepository
	Resources  output.ResourceRepository
	FloorItems output.FloorItemRepository
	Bookings   output.BookingRepository
	Media      output.MediaRepository
	Files      output.FileStorage
	Booker     port.Booker
	// Notifier emails the customer when their unpaid booking is released (nil: no email)
	Notifier port.Notifier
	Now      func() time.Time
}

// Service holds the events rules (it implements port.Service)
type Service struct {
	Deps
	// Slips: asks whether a branch can receive payment slips in Telegram (nil: no)
	Slips port.SlipChannel
}

var _ port.Service = (*Service)(nil)

// New builds the service
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{Deps: d}
}

// ---------- helpers ----------

func (s *Service) branch(ctx context.Context, id uint) (*domain.Branch, error) {
	b, err := s.Branches.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	if b.Type != domain.BranchTypeRestaurant {
		return nil, conflict("NOT_A_RESTAURANT")
	}
	return b, nil
}

// anyBranch and anyPublicBranch are for what both kinds of branch share (how customers pay outside the
// system): a sport branch uses it for packages, a restaurant for event tables
func (s *Service) anyBranch(ctx context.Context, id uint) (*domain.Branch, error) {
	b, err := s.Branches.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	return b, nil
}

func (s *Service) anyPublicBranch(ctx context.Context, slug string) (*domain.Branch, error) {
	b, err := s.Branches.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	if !b.IsPublic() {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	return b, nil
}

func (s *Service) publicBranch(ctx context.Context, slug string) (*domain.Branch, error) {
	b, err := s.Branches.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	if !b.IsPublic() || b.Type != domain.BranchTypeRestaurant {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	return b, nil
}

func mapNotFound(err error, code string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(strings.ToUpper(err.Error()), "NOT_FOUND") {
		return notFound(code)
	}
	return err
}

func locationOf(b *domain.Branch) *time.Location {
	if loc, err := time.LoadLocation(b.Timezone); err == nil {
		return loc
	}
	return time.UTC
}

func (s *Service) today(b *domain.Branch) string {
	return s.Now().In(locationOf(b)).Format("2006-01-02")
}

func validDate(d string) bool {
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

// fileKey accepts "" or a picture of this branch's library
func (s *Service) fileKey(ctx context.Context, branchID uint, key string) error {
	if key == "" {
		return nil
	}
	rec, err := s.Media.Get(ctx, branchID, key)
	if err != nil {
		return err
	}
	if rec == nil {
		return badRequest("INVALID_FILE_KEY")
	}
	return nil
}

func (s *Service) url(key string) string {
	if key == "" {
		return ""
	}
	return s.Files.URL(key)
}

// catalog is what can be sold: the active tables (with seats) of active areas, and the zones of those areas
func (s *Service) catalog(ctx context.Context, branchID uint) ([]domain.Resource, []domain.FloorItem, error) {
	areas, _, err := s.Sports.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"active": true}})
	if err != nil {
		return nil, nil, err
	}
	live := map[uint]bool{}
	for _, a := range areas {
		live[a.ID] = true
	}
	all, _, err := s.Resources.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"active": true}})
	if err != nil {
		return nil, nil, err
	}
	// (a table kept for walk-ins is not sold in events either)
	tables := slices.DeleteFunc(all, func(r domain.Resource) bool { return !live[r.SportID] || r.Seats < 1 || r.WalkInOnly })
	items, _, err := s.FloorItems.List(ctx, branchID, output.ListQuery{})
	if err != nil {
		return nil, nil, err
	}
	zones := slices.DeleteFunc(items, func(f domain.FloorItem) bool { return f.Kind != domain.FloorItemZone || !live[f.AreaID] })
	slices.SortFunc(zones, func(a, b domain.FloorItem) int { return int(a.ID) - int(b.ID) })
	return tables, zones, nil
}

// ---------- staff: events ----------

func (s *Service) toDTO(ctx context.Context, b *domain.Branch, e evdomain.Event) dto.EventDTO {
	d := dto.EventDTO{
		ID: e.ID, Name: e.Name, Description: e.Description, StartsOn: e.StartsOn, EndsOn: e.EndsOn,
		BackgroundKey: e.BackgroundKey, BackgroundURL: s.url(e.BackgroundKey), AdKey: e.AdKey, AdURL: s.url(e.AdKey),
		HoldMinutes: e.HoldMinutes, BigBookingTables: e.BigBookingTables, BigHoldMinutes: e.BigHoldMinutes,
		DefaultPrice: e.DefaultPrice, Published: e.Published, Currency: b.Currency,
	}
	d.StartTime, d.EndTime, d.AcceptGuests = e.StartTime, e.EndTime, !e.GuestsOff
	d.AreaIDs, d.Lineup = []uint{}, []dto.LineupItem{}
	if ids, err := s.Repo.AreaIDs(ctx, e.ID); err == nil && ids != nil {
		d.AreaIDs = ids
	}
	if rows, err := s.Repo.Lineup(ctx, e.ID); err == nil {
		d.Lineup = lineupItems(rows)
	}
	if counts, err := s.Repo.BookingCounts(ctx, e.ID); err == nil {
		d.Waiting, d.Paid, d.Cancelled = counts[domain.BookingPending], counts[domain.BookingConfirmed]+counts[domain.BookingCompleted], counts[domain.BookingCancelled]
	}
	return d
}

func (s *Service) List(ctx context.Context, branchID uint) ([]dto.EventDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	list, err := s.Repo.List(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EventDTO, 0, len(list))
	for _, e := range list {
		out = append(out, s.toDTO(ctx, b, e))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, branchID, id uint) (*dto.EventDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	e, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "EVENT_NOT_FOUND")
	}
	d := s.toDTO(ctx, b, *e)
	return &d, nil
}

// apply checks a request and copies it onto the event
func (s *Service) apply(ctx context.Context, b *domain.Branch, e *evdomain.Event, req dto.EventRequest, creating bool) (*extras, error) {
	name := strings.TrimSpace(req.Name)
	switch {
	case name == "" || utf8.RuneCountInString(name) > 120:
		return nil, badRequest("INVALID_NAME")
	case utf8.RuneCountInString(req.Description) > 2000:
		return nil, badRequest("DESCRIPTION_TOO_LONG")
	case !validDate(req.StartsOn):
		return nil, badRequest("INVALID_START_DATE")
	}
	if req.EndsOn == "" {
		req.EndsOn = req.StartsOn // one day unless the owner asks for more
	}
	from, _ := time.Parse("2006-01-02", req.StartsOn)
	to, err := time.Parse("2006-01-02", req.EndsOn)
	switch {
	case err != nil:
		return nil, badRequest("INVALID_END_DATE")
	case to.Before(from):
		return nil, badRequest("END_BEFORE_START")
	case to.Sub(from) > (maxSpanDays-1)*24*time.Hour:
		return nil, badRequest("EVENT_TOO_LONG")
	case creating && req.EndsOn < s.today(b):
		return nil, badRequest("EVENT_IN_THE_PAST")
	}
	if req.HoldMinutes == 0 {
		req.HoldMinutes = defaultHold
	}
	if req.BigBookingTables == 0 {
		req.BigBookingTables = defaultBigN
	}
	if req.BigHoldMinutes == 0 {
		req.BigHoldMinutes = defaultBigMin
	}
	switch {
	case req.HoldMinutes < 15 || req.HoldMinutes > 7*24*60:
		return nil, badRequest("INVALID_HOLD_MINUTES")
	case req.BigBookingTables < 1 || req.BigBookingTables > 50:
		return nil, badRequest("INVALID_BIG_BOOKING_TABLES")
	case req.BigHoldMinutes < 5 || req.BigHoldMinutes > 7*24*60:
		return nil, badRequest("INVALID_BIG_HOLD_MINUTES")
	case req.DefaultPrice < 0 || req.DefaultPrice > maxPrice:
		return nil, badRequest("INVALID_PRICE")
	}
	for _, key := range []string{req.BackgroundKey, req.AdKey} {
		if err := s.fileKey(ctx, b.ID, key); err != nil {
			return nil, err
		}
	}
	// the time of day
	for _, clock := range []string{req.StartTime, req.EndTime} {
		if clock != "" && !validClock(clock) {
			return nil, badRequest("INVALID_EVENT_TIME")
		}
	}
	if req.StartTime != "" && req.StartTime == req.EndTime {
		return nil, badRequest("INVALID_EVENT_TIME")
	}
	if req.StartTime == "" && req.EndTime != "" {
		return nil, badRequest("EVENT_START_TIME_REQUIRED")
	}
	ex := &extras{}
	// the areas: not sent = unchanged; none = every area
	if req.AreaIDs != nil {
		ids, err := s.checkAreas(ctx, b.ID, *req.AreaIDs)
		if err != nil {
			return nil, err
		}
		ex.areas, ex.setAreas = ids, true
	}
	if req.Lineup != nil {
		rows, err := checkLineup(*req.Lineup, req.StartsOn, req.EndsOn)
		if err != nil {
			return nil, err
		}
		ex.lineup, ex.setLineup = rows, true
	}
	if req.Published {
		clash, err := s.Repo.PublishedOverlapping(ctx, b.ID, req.StartsOn, req.EndsOn, e.ID)
		if err != nil {
			return nil, err
		}
		// events may share a day only when they are held in different areas
		mine := ex.areas
		if !ex.setAreas && e.ID != 0 {
			if mine, err = s.Repo.AreaIDs(ctx, e.ID); err != nil {
				return nil, err
			}
		}
		for _, other := range clash {
			theirs, err := s.Repo.AreaIDs(ctx, other.ID)
			if err != nil {
				return nil, err
			}
			if len(mine) == 0 || len(theirs) == 0 || slices.ContainsFunc(mine, func(id uint) bool { return slices.Contains(theirs, id) }) {
				return nil, conflict("EVENT_DATES_OVERLAP") // the same area cannot hold two events on a day
			}
		}
	}
	// a published event takes over its days: normal table bookings that already hold its tables would clash with the
	// event's own sale of them, so the owner moves or cancels those first
	if req.Published && b.Type == domain.BranchTypeRestaurant {
		areas := ex.areas
		if !ex.setAreas && e.ID != 0 {
			var err error
			if areas, err = s.Repo.AreaIDs(ctx, e.ID); err != nil {
				return nil, err
			}
		}
		loc := locationOf(b)
		first, _ := time.ParseInLocation("2006-01-02", req.StartsOn, loc)
		last, _ := time.ParseInLocation("2006-01-02", req.EndsOn, loc)
		codes, err := s.Bookings.NormalTableBookings(ctx, b.ID, first, last.AddDate(0, 0, 1), areas)
		if err != nil {
			return nil, err
		}
		if len(codes) > 0 {
			return nil, conflict("EVENT_HAS_TABLE_BOOKINGS_" + strings.Join(codes, ","))
		}
	}
	e.BranchID, e.Name, e.Description = b.ID, name, strings.TrimSpace(req.Description)
	e.StartsOn, e.EndsOn = req.StartsOn, req.EndsOn
	e.BackgroundKey, e.AdKey = req.BackgroundKey, req.AdKey
	e.HoldMinutes, e.BigBookingTables, e.BigHoldMinutes = req.HoldMinutes, req.BigBookingTables, req.BigHoldMinutes
	e.DefaultPrice, e.Published = req.DefaultPrice, req.Published
	e.StartTime, e.EndTime = req.StartTime, req.EndTime
	if req.AcceptGuests != nil {
		e.GuestsOff = !*req.AcceptGuests
	}
	return ex, nil
}

func (s *Service) Create(ctx context.Context, branchID uint, req dto.EventRequest) (*dto.EventDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	e := &evdomain.Event{}
	ex, err := s.apply(ctx, b, e, req, true)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Create(ctx, e); err != nil {
		return nil, err
	}
	if err := s.saveExtras(ctx, e.ID, ex); err != nil {
		return nil, err
	}
	d := s.toDTO(ctx, b, *e)
	return &d, nil
}

func (s *Service) Update(ctx context.Context, branchID, id uint, req dto.EventRequest) (*dto.EventDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	e, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "EVENT_NOT_FOUND")
	}
	ex, err := s.apply(ctx, b, e, req, false)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Update(ctx, e); err != nil {
		return nil, err
	}
	if err := s.saveExtras(ctx, e.ID, ex); err != nil {
		return nil, err
	}
	d := s.toDTO(ctx, b, *e)
	return &d, nil
}

// Delete removes an event nobody booked; an event with bookings is unpublished instead
func (s *Service) Delete(ctx context.Context, branchID, id uint) error {
	if _, err := s.branch(ctx, branchID); err != nil {
		return err
	}
	if _, err := s.Repo.Get(ctx, branchID, id); err != nil {
		return mapNotFound(err, "EVENT_NOT_FOUND")
	}
	counts, err := s.Repo.BookingCounts(ctx, id)
	if err != nil {
		return err
	}
	for _, n := range counts {
		if n > 0 {
			return conflict("EVENT_HAS_BOOKINGS")
		}
	}
	return s.Repo.Delete(ctx, branchID, id)
}

// Copy makes a new unpublished event from an old one, prices included
func (s *Service) Copy(ctx context.Context, branchID, id uint, req dto.CopyRequest) (*dto.EventDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	old, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "EVENT_NOT_FOUND")
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = old.Name + " (copy)"
	}
	if req.StartsOn == "" {
		req.StartsOn, req.EndsOn = old.StartsOn, old.EndsOn
	}
	e := &evdomain.Event{}
	oldAreas, err := s.Repo.AreaIDs(ctx, old.ID)
	if err != nil {
		return nil, err
	}
	oldLineup, err := s.Repo.Lineup(ctx, old.ID)
	if err != nil {
		return nil, err
	}
	items := lineupItems(oldLineup)
	if old.StartsOn != req.StartsOn { // dated rows belong to the old days: they become "every day" rows
		for i := range items {
			items[i].Day = ""
		}
	}
	ex, err := s.apply(ctx, b, e, dto.EventRequest{
		Name: req.Name, Description: old.Description, StartsOn: req.StartsOn, EndsOn: req.EndsOn,
		BackgroundKey: old.BackgroundKey, AdKey: old.AdKey, HoldMinutes: old.HoldMinutes,
		BigBookingTables: old.BigBookingTables, BigHoldMinutes: old.BigHoldMinutes, DefaultPrice: old.DefaultPrice,
		StartTime: old.StartTime, EndTime: old.EndTime, AreaIDs: &oldAreas, Lineup: &items, AcceptGuests: ptr(!old.GuestsOff),
	}, true)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Create(ctx, e); err != nil {
		return nil, err
	}
	if err := s.saveExtras(ctx, e.ID, ex); err != nil {
		return nil, err
	}
	prices, err := s.Repo.Prices(ctx, old.ID)
	if err != nil {
		return nil, err
	}
	for i := range prices {
		prices[i].ID, prices[i].EventID = 0, e.ID
	}
	if err := s.Repo.ReplacePrices(ctx, e.ID, prices); err != nil {
		return nil, err
	}
	d := s.toDTO(ctx, b, *e)
	return &d, nil
}

// ---------- staff: prices ----------

func (s *Service) Prices(ctx context.Context, branchID, id uint) (*dto.PricesDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	e, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "EVENT_NOT_FOUND")
	}
	rows, err := s.Repo.Prices(ctx, id)
	if err != nil {
		return nil, err
	}
	tables, zones, err := s.catalog(ctx, branchID)
	if err != nil {
		return nil, err
	}
	book := evdomain.NewPriceBook(*e, rows, zones)
	out := &dto.PricesDTO{DefaultPrice: e.DefaultPrice, Currency: b.Currency, Zones: []dto.ZonePriceDTO{}, Tables: []dto.TablePriceDTO{}}
	for _, z := range zones {
		d := dto.ZonePriceDTO{ItemID: z.ID, AreaID: z.AreaID, Label: z.Label, Color: z.Color}
		if v, ok := book.ZonePrice(z.ID); ok {
			d.Price = &v
		}
		out.Zones = append(out.Zones, d)
	}
	for _, t := range tables {
		d := dto.TablePriceDTO{TableID: t.ID, AreaID: t.SportID, Name: t.Name, Seats: t.Seats, Price: book.Of(t)}
		if v, ok := book.OwnPrice(t.ID); ok {
			d.Own = &v
		}
		for _, z := range zones {
			if z.AreaID == t.SportID && z.Holds(t) {
				id := z.ID
				d.ZoneID = &id
				break
			}
		}
		out.Tables = append(out.Tables, d)
	}
	return out, nil
}

// SetPrices replaces every price of the event: one per zone, one per table that differs, and the default
func (s *Service) SetPrices(ctx context.Context, branchID, id uint, req dto.PricesRequest) (*dto.PricesDTO, error) {
	if _, err := s.branch(ctx, branchID); err != nil {
		return nil, err
	}
	e, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "EVENT_NOT_FOUND")
	}
	tables, zones, err := s.catalog(ctx, branchID)
	if err != nil {
		return nil, err
	}
	okZone, okTable := map[uint]bool{}, map[uint]bool{}
	for _, z := range zones {
		okZone[z.ID] = true
	}
	for _, t := range tables {
		okTable[t.ID] = true
	}
	valid := func(p int64) bool { return p >= 0 && p <= maxPrice }
	if !valid(req.DefaultPrice) {
		return nil, badRequest("INVALID_PRICE")
	}
	var rows []evdomain.Price
	seen := map[string]bool{}
	for _, z := range req.Zones {
		key := fmt.Sprintf("%s:%d", evdomain.PriceZone, z.ItemID)
		if !okZone[z.ItemID] || !valid(z.Price) || seen[key] {
			return nil, badRequest("INVALID_ZONE_PRICE")
		}
		seen[key] = true
		rows = append(rows, evdomain.Price{EventID: id, Kind: evdomain.PriceZone, RefID: z.ItemID, Amount: z.Price})
	}
	for _, t := range req.Tables {
		key := fmt.Sprintf("%s:%d", evdomain.PriceTable, t.TableID)
		if !okTable[t.TableID] || !valid(t.Price) || seen[key] {
			return nil, badRequest("INVALID_TABLE_PRICE")
		}
		seen[key] = true
		rows = append(rows, evdomain.Price{EventID: id, Kind: evdomain.PriceTable, RefID: t.TableID, Amount: t.Price})
	}
	if err := s.Repo.ReplacePrices(ctx, id, rows); err != nil {
		return nil, err
	}
	e.DefaultPrice = req.DefaultPrice
	if err := s.Repo.Update(ctx, e); err != nil {
		return nil, err
	}
	return s.Prices(ctx, branchID, id)
}

// ---------- staff: how customers pay ----------

func (s *Service) toPayDTO(ctx context.Context, branchID uint, p *evdomain.PayInfo) *dto.PayInfoDTO {
	out := &dto.PayInfoDTO{TransferEnabled: true, SlipWhatsapp: true, TelegramReady: s.telegramReady(ctx, branchID)}
	if p == nil {
		return out
	}
	out.Whatsapp, out.QRKey, out.QRURL, out.Note = p.Whatsapp, p.QRKey, s.url(p.QRKey), p.Note
	out.AccountName, out.AccountNumber, out.TransferEnabled = p.AccountName, p.AccountNumber, !p.TransferOff
	out.SlipWhatsapp, out.SlipTelegram = !p.WhatsappOff, p.SlipTelegram
	return out
}

// SetSlipChannel tells the service how to ask whether a branch can receive slips in Telegram (nil: it cannot)
func (s *Service) SetSlipChannel(c port.SlipChannel) { s.Slips = c }

func (s *Service) telegramReady(ctx context.Context, branchID uint) bool {
	return s.Slips != nil && s.Slips.SlipAvailable(ctx, branchID)
}

// SlipWays says which ways a customer can send a payment slip to the branch right now: WhatsApp (a number is set
// and the way is on) and Telegram (on, and the bot and group are connected). Nothing is on when the branch turned
// its own way to pay off.
func (s *Service) SlipWays(ctx context.Context, branchID uint) (whatsapp, telegram bool, err error) {
	p, err := s.Repo.PayInfo(ctx, branchID)
	if err != nil || p == nil {
		return false, false, err
	}
	if p.TransferOff {
		return false, false, nil
	}
	return p.Whatsapp != "" && !p.WhatsappOff, p.SlipTelegram && s.telegramReady(ctx, branchID), nil
}

func (s *Service) Pay(ctx context.Context, branchID uint) (*dto.PayInfoDTO, error) {
	if _, err := s.anyBranch(ctx, branchID); err != nil {
		return nil, err
	}
	p, err := s.Repo.PayInfo(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return s.toPayDTO(ctx, branchID, p), nil
}

// cleanWhatsapp keeps digits only (the number as used in wa.me links, with the country code)
func cleanWhatsapp(raw string) (string, error) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return "", badRequest("INVALID_WHATSAPP")
		}
	}
	n := b.String()
	if n != "" && (len(n) < 8 || len(n) > 15) {
		return "", badRequest("INVALID_WHATSAPP")
	}
	return n, nil
}

func (s *Service) SavePay(ctx context.Context, branchID uint, req dto.PayInfoRequest) (*dto.PayInfoDTO, error) {
	if _, err := s.anyBranch(ctx, branchID); err != nil {
		return nil, err
	}
	number, err := cleanWhatsapp(req.Whatsapp)
	if err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(req.Note) > 2000 {
		return nil, badRequest("NOTE_TOO_LONG")
	}
	if err := s.fileKey(ctx, branchID, req.QRKey); err != nil {
		return nil, err
	}
	name, account := strings.TrimSpace(req.AccountName), strings.TrimSpace(req.AccountNumber)
	if utf8.RuneCountInString(name) > 120 || utf8.RuneCountInString(account) > 60 {
		return nil, badRequest("ACCOUNT_TOO_LONG")
	}
	p := &evdomain.PayInfo{
		BranchID: branchID, Whatsapp: number, QRKey: req.QRKey, Note: strings.TrimSpace(req.Note),
		AccountName: name, AccountNumber: account, TransferOff: req.TransferEnabled != nil && !*req.TransferEnabled,
		WhatsappOff:  req.SlipWhatsapp != nil && !*req.SlipWhatsapp,
		SlipTelegram: req.SlipTelegram != nil && *req.SlipTelegram,
	}
	if p.SlipTelegram && !s.telegramReady(ctx, branchID) {
		return nil, conflict("TELEGRAM_NOT_CONNECTED") // connect the bot and the group first (Setup > Telegram)
	}
	if err := s.Repo.SavePayInfo(ctx, p); err != nil {
		return nil, err
	}
	return s.toPayDTO(ctx, branchID, p), nil
}

// ---------- public ----------

// PublicPay tells a customer how to pay the branch outside the system; nothing secret: it is what they see
// after buying tables
func (s *Service) PublicPay(ctx context.Context, slug string) (*dto.PublicPayInfoDTO, error) {
	b, err := s.anyPublicBranch(ctx, slug)
	if err != nil {
		return nil, err
	}
	info, err := s.Repo.PayInfo(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	out := &dto.PublicPayInfoDTO{TransferEnabled: true}
	if info != nil {
		out.Whatsapp, out.QRURL, out.Note = info.Whatsapp, s.url(info.QRKey), info.Note
		out.AccountName, out.AccountNumber, out.TransferEnabled = info.AccountName, info.AccountNumber, !info.TransferOff
		out.SlipWhatsapp, out.SlipTelegram, _ = s.SlipWays(ctx, b.ID)
	}
	return out, nil
}

func (s *Service) publicCard(ctx context.Context, e evdomain.Event, b *domain.Branch, book evdomain.PriceBook, allTables []domain.Resource) dto.PublicEventDTO {
	areaIDs, _ := s.Repo.AreaIDs(ctx, e.ID)
	tables := tablesOf(allTables, areaIDs)
	var from int64
	for _, t := range tables {
		if p := book.Of(t); p > 0 && (from == 0 || p < from) {
			from = p
		}
	}
	ad := e.AdKey
	if ad == "" {
		ad = e.BackgroundKey
	}
	return dto.PublicEventDTO{
		ID: e.ID, Name: e.Name, Description: e.Description, StartsOn: e.StartsOn, EndsOn: e.EndsOn,
		BackgroundURL: s.url(e.BackgroundKey), AdURL: s.url(ad), FromPrice: from, Currency: b.Currency,
		StartTime: e.StartTime, EndTime: e.EndTime, AcceptGuests: !e.GuestsOff, Areas: s.areaRefs(ctx, b.ID, areaIDs), Lineup: s.lineupOf(ctx, e.ID),
	}
}

func (s *Service) lineupOf(ctx context.Context, eventID uint) []dto.LineupItem {
	rows, err := s.Repo.Lineup(ctx, eventID)
	if err != nil {
		return []dto.LineupItem{}
	}
	return lineupItems(rows)
}

// Upcoming lists the published events not over yet: the small ads on the home and branch pages
func (s *Service) Upcoming(ctx context.Context, slug string) ([]dto.PublicEventDTO, error) {
	b, err := s.publicBranch(ctx, slug)
	if err != nil {
		return nil, err
	}
	list, err := s.Repo.Upcoming(ctx, b.ID, s.today(b))
	if err != nil {
		return nil, err
	}
	out := []dto.PublicEventDTO{}
	if len(list) == 0 {
		return out, nil
	}
	tables, zones, err := s.catalog(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		rows, err := s.Repo.Prices(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, s.publicCard(ctx, e, b, evdomain.NewPriceBook(e, rows, zones), tables))
	}
	return out, nil
}

// publicEvent loads a published event of the branch and the day asked about (default: the first day to come)
func (s *Service) publicEvent(ctx context.Context, b *domain.Branch, id uint, date string) (*evdomain.Event, string, error) {
	e, err := s.Repo.Get(ctx, b.ID, id)
	if err != nil || !e.Published {
		return nil, "", notFound("EVENT_NOT_FOUND")
	}
	today := s.today(b)
	if e.EndsOn < today {
		return nil, "", conflict("EVENT_OVER")
	}
	if date == "" {
		date = max(e.StartsOn, today)
	}
	if !validDate(date) || !e.Covers(date) || date < today {
		return nil, "", badRequest("INVALID_EVENT_DAY")
	}
	return e, date, nil
}

// taken lists the tables already held or paid on the day
func (s *Service) taken(ctx context.Context, branchID uint, start, end time.Time) (map[uint]bool, error) {
	items, err := s.Bookings.ActiveResourceItems(ctx, branchID, start, end)
	if err != nil {
		return nil, err
	}
	out := map[uint]bool{}
	for _, it := range items {
		if it.ResourceID != nil {
			out[*it.ResourceID] = true
		}
	}
	return out, nil
}

// Detail is an event day for the customer: every table with its price, and which are already taken
func (s *Service) Detail(ctx context.Context, slug string, id uint, date string) (*dto.PublicEventDetailDTO, error) {
	b, err := s.publicBranch(ctx, slug)
	if err != nil {
		return nil, err
	}
	e, date, err := s.publicEvent(ctx, b, id, date)
	if err != nil {
		return nil, err
	}
	rows, err := s.Repo.Prices(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	tables, zones, err := s.catalog(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	book := evdomain.NewPriceBook(*e, rows, zones)
	out := &dto.PublicEventDetailDTO{
		PublicEventDTO: s.publicCard(ctx, *e, b, book, tables), Days: e.Days(), Date: date, Tables: []dto.SaleTableDTO{},
		HoldMinutes: e.HoldMinutes, BigBookingTables: e.BigBookingTables, BigHoldMinutes: e.BigHoldMinutes,
	}
	start, end, open, err := s.Booker.OpenOn(ctx, b.ID, date)
	if err != nil {
		return nil, err
	}
	var taken map[uint]bool
	if open {
		out.OpensAt, out.ClosesAt = &start, &end
		if taken, err = s.taken(ctx, b.ID, start, end); err != nil {
			return nil, err
		}
	}
	areaIDs, err := s.Repo.AreaIDs(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range tablesOf(tables, areaIDs) {
		// closed that day: nothing can be bought
		out.Tables = append(out.Tables, dto.SaleTableDTO{ID: t.ID, Price: book.Of(t), Sold: !open || taken[t.ID]})
	}
	return out, nil
}

// Book buys tables of an event day: the module checks what is for sale, prices it and decides how long
// the tables are held; the booking service stores the booking
func (s *Service) Book(ctx context.Context, slug string, id uint, actor *domain.User, req dto.BookRequest) (*dto.BookedDTO, error) {
	b, err := s.publicBranch(ctx, slug)
	if err != nil {
		return nil, err
	}
	e, date, err := s.publicEvent(ctx, b, id, req.Date)
	if err != nil {
		return nil, err
	}
	// the team must be able to call the customer: an account buys event tables only with a phone number saved
	// (a guest's phone is always required with their details)
	if actor != nil && strings.TrimSpace(actor.Phone) == "" {
		return nil, badRequest("PHONE_REQUIRED_FOR_EVENT")
	}
	if e.GuestsOff && actor == nil {
		return nil, response.NewError(http.StatusUnauthorized, "LOGIN_REQUIRED") // this event is for customers with an account
	}
	if len(req.TableIDs) == 0 {
		return nil, badRequest("TABLE_REQUIRED")
	}
	if len(req.TableIDs) > maxNewTables {
		return nil, badRequest("TOO_MANY_TABLES")
	}
	rows, err := s.Repo.Prices(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	tables, zones, err := s.catalog(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	book := evdomain.NewPriceBook(*e, rows, zones)
	areaIDs, err := s.Repo.AreaIDs(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	byID := map[uint]domain.Resource{}
	for _, t := range tablesOf(tables, areaIDs) { // only the event's areas are sold
		byID[t.ID] = t
	}
	start, end, open, err := s.Booker.OpenOn(ctx, b.ID, date)
	if err != nil {
		return nil, err
	}
	if !open {
		return nil, conflict("VENUE_CLOSED")
	}
	taken, err := s.taken(ctx, b.ID, start, end)
	if err != nil {
		return nil, err
	}
	var sales []coredto.EventTableSale
	var total, capacity int64
	for _, tid := range req.TableIDs {
		t, ok := byID[tid]
		if !ok {
			return nil, conflict("TABLE_NOT_AVAILABLE")
		}
		price := book.Of(t)
		switch {
		case price <= 0:
			return nil, conflict("TABLE_NOT_FOR_SALE")
		case taken[tid]:
			return nil, conflict("TABLE_NOT_AVAILABLE")
		}
		sales = append(sales, coredto.EventTableSale{ID: tid, Price: price})
		total += price
		capacity += int64(t.Seats + t.ExtraChairs)
	}
	// the door list cannot hold more people than the tables seat (extra chairs count)
	if int64(req.Guests) > capacity {
		return nil, badRequest("TOO_MANY_GUESTS_FOR_TABLES")
	}
	hold := e.HoldFor(len(sales))
	created, err := s.Booker.CreateEventBooking(ctx, coredto.EventBookingInput{
		Slug: slug, Actor: actor, Guest: req.Guest, Notes: req.Notes, EventID: e.ID, Date: date,
		Guests: req.Guests, HoldMinutes: int(hold / time.Minute), Tables: sales,
		EventName: e.Name, EventStartTime: e.StartTime, EventEndTime: e.EndTime,
	})
	if err != nil {
		return nil, err
	}
	info, err := s.Repo.PayInfo(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	pay := dto.PayDTO{
		Total: total, Currency: b.Currency, HoldUntil: created.PayBy, BigBooking: len(sales) > e.BigBookingTables,
	}
	pay.TransferEnabled = true
	if info != nil {
		pay.Whatsapp, pay.QRURL, pay.Note = info.Whatsapp, s.url(info.QRKey), info.Note
		pay.AccountName, pay.AccountNumber, pay.TransferEnabled = info.AccountName, info.AccountNumber, !info.TransferOff
	}
	return &dto.BookedDTO{CreatedBookingDTO: *created, Pay: pay}, nil
}

// ---------- the booking service asks, and the job ----------

// EventOn tells the booking service that an event takes over a day (normal table booking is off)
func (s *Service) EventOn(ctx context.Context, branchID uint, day string) (*domain.EventDay, error) {
	events, err := s.Repo.OnDay(ctx, branchID, day)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	// several events may share a day in different areas: the first one is named, the areas are all of them,
	// and one held everywhere covers the whole branch
	first := events[0]
	out := &domain.EventDay{ID: first.ID, Name: first.Name, StartTime: first.StartTime, EndTime: first.EndTime}
	whole := false
	for _, e := range events {
		ids, err := s.Repo.AreaIDs(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			whole = true
		}
		for _, id := range ids {
			if !slices.Contains(out.AreaIDs, id) {
				out.AreaIDs = append(out.AreaIDs, id)
			}
		}
	}
	if whole {
		out.AreaIDs = nil
	}
	for _, id := range out.AreaIDs {
		if a, err := s.Sports.GetByID(ctx, branchID, id); err == nil {
			out.AreaNames = append(out.AreaNames, a.Name)
		}
	}
	return out, nil
}

// ExpireUnpaid cancels event bookings that were not paid in time and have no slip sent: their
// tables go back on sale. Runs every few minutes.
func (s *Service) ExpireUnpaid(ctx context.Context) error {
	list, err := s.Bookings.EventHoldsExpired(ctx, s.Now())
	if err != nil {
		return err
	}
	for i := range list {
		// only if it is still unpaid and waiting when the row is locked: a payment that arrived meanwhile keeps its tables
		released, err := s.Bookings.ReleaseUnpaid(ctx, &list[i], "Not paid in time", s.Now())
		if err != nil {
			log.Printf("⚠️  event booking %d: %v", list[i].ID, err) // the others still go
			continue
		}
		if !released {
			continue // paid, confirmed or cancelled in the meantime: nothing to tell anybody
		}
		if s.Notifier != nil {
			s.Notifier.BookingReleased(ctx, list[i].BranchID, list[i].ID)
		}
	}
	return nil
}
