package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/modules/booking/port"
	catalogport "backend/internal/modules/catalog/port"
	"backend/pkg/response"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	maxGuestName  = 120
	maxNotesChars = 1000
)

// Deps are what the service is built from: the repositories (driven ports of the core), the rules
// service and the mailer
type Deps struct {
	Branches    output.BranchRepository
	Sports      output.SportRepository
	Resources   output.ResourceRepository
	Coaches     output.CoachRepository
	Hours       output.OpeningHourRepository
	Closures    output.ClosureRepository
	Prices      output.PriceRepository
	Classes     output.ClassSessionRepository
	Bookings    output.BookingRepository
	Users       output.UserRepository
	Memberships output.MembershipRepository
	Rules       catalogport.RuleService
	Mailer      output.Mailer
	FrontendURL string
	FloorItems  output.FloorItemRepository
	// OpenWaitingPerPhone: unpaid bookings waiting for approval one phone may hold (0: 3). PublicBookingsPerHour is
	// for the HTTP layer (new public bookings per IP per hour, 0: 10)
	OpenWaitingPerPhone   int
	PublicBookingsPerHour int
}

// Service serves both the public booking page and the staff desk (staff bookings go through exactly
// the same slot check as customer bookings, see slotsFor). It implements port.BookingService and
// port.DeskService.
type Service struct {
	openWaitingPerPhone int
	branches            output.BranchRepository
	sports              output.SportRepository
	resources           output.ResourceRepository
	coaches             output.CoachRepository
	hours               output.OpeningHourRepository
	closures            output.ClosureRepository
	prices              output.PriceRepository
	classes             output.ClassSessionRepository
	bookings            output.BookingRepository
	users               output.UserRepository
	memberships         output.MembershipRepository
	rules               catalogport.RuleService
	mailer              output.Mailer
	frontendURL         string
	floorItems          output.FloorItemRepository
	tables              port.TableBooker    // restaurant branches book tables through the restaurant module (SetTables)
	packages            output.PackageUse   // pays a booking with a customer's package (SetPackages); nil = off
	slipWays            output.SlipWays     // which ways a slip can be sent (SetSlips); nil = WhatsApp only
	slipDesk            output.SlipDesk     // where an uploaded slip goes; nil = no Telegram
	team                output.TeamNotifier // tells the branch's team (Telegram) what happens; nil = nobody (SetTeamNotifier)
	now                 func() time.Time
}

var (
	_ port.BookingService = (*Service)(nil)
	_ port.DeskService    = (*Service)(nil)
)

// New builds the service
func New(d Deps) *Service {
	return &Service{
		branches: d.Branches, sports: d.Sports, resources: d.Resources, coaches: d.Coaches, hours: d.Hours,
		closures: d.Closures, prices: d.Prices, classes: d.Classes, bookings: d.Bookings, users: d.Users,
		memberships: d.Memberships, rules: d.Rules, mailer: d.Mailer,
		frontendURL: strings.TrimRight(d.FrontendURL, "/"), floorItems: d.FloorItems, now: time.Now,
		openWaitingPerPhone: orDefault(d.OpenWaitingPerPhone, 3),
	}
}

func orDefault(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}

// ---------- what is being booked ----------

// owner is something that can be booked and has its own hours: a resource or a coach
type owner struct {
	ownerType string
	ownerID   uint
}

// target is a resource, a coach, or both, for one sport
type target struct {
	resource *domain.Resource
	coach    *domain.Coach
	sportID  uint
}

func (t target) resourceID() uint {
	if t.resource == nil {
		return 0
	}
	return t.resource.ID
}

// resolveTarget loads and checks what the customer asked for; everything must be active
// and belong to the branch
func (s *Service) resolveTarget(ctx context.Context, branchID, resourceID, coachID, sportID uint) (*target, error) {
	if resourceID == 0 && coachID == 0 {
		return nil, badRequest("RESOURCE_OR_COACH_REQUIRED")
	}
	t := &target{sportID: sportID}
	if resourceID != 0 {
		r, err := s.resources.GetByID(ctx, branchID, resourceID)
		if err != nil {
			return nil, orBadRequest(err, "RESOURCE_NOT_FOUND")
		}
		if !r.Active {
			return nil, badRequest("RESOURCE_NOT_AVAILABLE")
		}
		if t.sportID != 0 && t.sportID != r.SportID {
			return nil, badRequest("RESOURCE_SPORT_MISMATCH")
		}
		t.resource, t.sportID = r, r.SportID
	}
	if t.sportID == 0 {
		return nil, badRequest("SPORT_ID_REQUIRED")
	}
	sport, err := s.sports.GetByID(ctx, branchID, t.sportID)
	if err != nil {
		return nil, orBadRequest(err, "SPORT_NOT_FOUND")
	}
	if !sport.Active {
		return nil, badRequest("SPORT_NOT_AVAILABLE")
	}
	if coachID != 0 {
		c, err := s.coaches.GetByID(ctx, branchID, coachID)
		if err != nil {
			return nil, orBadRequest(err, "COACH_NOT_FOUND")
		}
		if !c.Active {
			return nil, badRequest("COACH_NOT_AVAILABLE")
		}
		if !teaches(*c, t.sportID) {
			return nil, badRequest("COACH_DOES_NOT_TEACH_SPORT")
		}
		t.coach = c
	}
	return t, nil
}

// openIntervals turns an owner's weekly hours into concrete intervals on day
func (s *Service) openIntervals(ctx context.Context, branchID uint, ownerType string, ownerID uint, day time.Time) ([]domain.Interval, error) {
	rows, err := s.hours.ListByOwner(ctx, branchID, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	return openIntervalsOn(rows, day), nil
}

// busyIntervals collects the owner's active bookings and class holds on [from, to)
func (s *Service) busyIntervals(ctx context.Context, branchID uint, ownerType string, ownerID uint, from, to time.Time) ([]domain.Interval, error) {
	items, err := s.bookings.ActiveItems(ctx, branchID, ownerType, ownerID, from, to)
	if err != nil {
		return nil, err
	}
	return mapSlice(items, func(i domain.BookingItem) domain.Interval {
		return domain.Interval{Start: i.StartsAt, End: i.EndsAt}
	}), nil
}

// slotsFor computes every bookable start on day for the target. It is the single definition
// of "free", used both to show availability and to accept a booking.
func (s *Service) slotsFor(ctx context.Context, b *domain.Branch, t *target, day time.Time, staff bool) ([]domain.Slot, *domain.EffectiveRule, error) {
	rule, err := s.rules.Effective(ctx, b.ID, t.sportID, t.resourceID())
	if err != nil {
		return nil, nil, err
	}
	from, to := day, day.AddDate(0, 0, 1)

	var open []domain.Interval
	var busy []domain.Interval
	var owners []owner
	if t.resource != nil {
		owners = append(owners, owner{domain.OwnerResource, t.resource.ID})
	}
	if t.coach != nil {
		owners = append(owners, owner{domain.OwnerCoach, t.coach.ID})
	}
	for i, o := range owners {
		ownerOpen, err := s.openIntervals(ctx, b.ID, o.ownerType, o.ownerID, day)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			open = ownerOpen
		} else {
			open = domain.Intersect(open, ownerOpen) // coach + court: both must be open
		}
		ownerBusy, err := s.busyIntervals(ctx, b.ID, o.ownerType, o.ownerID, from, to)
		if err != nil {
			return nil, nil, err
		}
		busy = append(busy, ownerBusy...)
	}

	closures, err := s.closures.Overlapping(ctx, b.ID, from, to)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range closures {
		applies := c.OwnerType == domain.OwnerVenue
		for _, o := range owners {
			applies = applies || (c.OwnerType == o.ownerType && c.OwnerID == o.ownerID)
		}
		if applies {
			busy = append(busy, domain.Interval{Start: c.StartsAt, End: c.EndsAt})
		}
	}

	now := s.now()
	window := domain.SlotWindow{
		Earliest: now.Add(time.Duration(rule.MinNoticeMin) * time.Minute),
		Latest:   startOfDay(now, day.Location()).AddDate(0, 0, rule.MaxDaysAhead+1),
	}
	if staff { // staff skip notice and days ahead, but never book a day that is over
		window = domain.SlotWindow{Earliest: startOfDay(now, day.Location()), Latest: now.AddDate(1, 0, 1)}
	}
	return domain.FreeSlots(open, busy, *rule, window), rule, nil
}

// ---------- public branch ----------

func (s *Service) published(ctx context.Context, slug string) (*domain.Branch, error) {
	return loadPublished(ctx, s.branches, slug)
}

func (s *Service) Availability(ctx context.Context, slug string, q dto.AvailabilityQuery) (*dto.AvailabilityDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.availability(ctx, b, q, false)
}

// CoachAvailability lists, for a space on a day, every active coach who teaches its sport with the times the space and
// the coach are both free (a coach with none that day is left out), so customers who picked a time see the coaches
// free then. The same rules as Availability: one call instead of one per coach.
func (s *Service) CoachAvailability(ctx context.Context, slug string, q dto.AvailabilityQuery) ([]dto.CoachAvailabilityDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	if q.ResourceID == 0 {
		return nil, badRequest("RESOURCE_REQUIRED")
	}
	space, err := s.resolveTarget(ctx, b.ID, q.ResourceID, 0, q.SportID)
	if err != nil {
		return nil, err
	}
	coaches, _, err := s.coaches.List(ctx, b.ID, output.ListQuery{Limit: 500})
	if err != nil {
		return nil, err
	}
	out := []dto.CoachAvailabilityDTO{}
	for _, c := range coaches {
		teaches := false
		for _, sp := range c.Sports {
			teaches = teaches || sp.ID == space.sportID
		}
		if !c.Active || !teaches {
			continue
		}
		cq := q
		cq.CoachID, cq.SportID = c.ID, space.sportID
		a, err := s.availability(ctx, b, cq, false)
		if err != nil {
			return nil, err
		}
		if len(a.Slots) == 0 {
			continue
		}
		out = append(out, dto.CoachAvailabilityDTO{CoachID: c.ID, Slots: a.Slots, Currency: a.Currency, Prices: a.Prices})
	}
	return out, nil
}

// DeskAvailability is what staff may book: no minimum notice or days-ahead limit, and the
// branch need not be published yet (the same rules CreateForCustomer applies)
func (s *Service) DeskAvailability(ctx context.Context, branchID uint, q dto.AvailabilityQuery) (*dto.AvailabilityDTO, error) {
	b, _, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return s.availability(ctx, b, q, true)
}

func (s *Service) availability(ctx context.Context, b *domain.Branch, q dto.AvailabilityQuery, staff bool) (*dto.AvailabilityDTO, error) {
	loc := locationOf(b)
	day, err := time.ParseInLocation("2006-01-02", q.Date, loc)
	if err != nil {
		return nil, badRequest("INVALID_DATE")
	}
	t, err := s.resolveTarget(ctx, b.ID, q.ResourceID, q.CoachID, q.SportID)
	if err != nil {
		return nil, err
	}
	slots, rule, err := s.slotsFor(ctx, b, t, day, staff)
	if err != nil {
		return nil, err
	}
	currency, prices, err := s.lengthPrices(ctx, b.ID, t, rule)
	if err != nil {
		return nil, err
	}
	return &dto.AvailabilityDTO{
		Date: q.Date, Timezone: loc.String(), Rule: *rule,
		Slots: mapSlice(slots, func(sl domain.Slot) dto.SlotDTO {
			return dto.SlotDTO{StartsAt: sl.StartsAt.In(loc), Durations: sl.Durations}
		}),
		Currency: currency, Prices: prices,
	}, nil
}

// lengthPrices is what each allowed length costs, worked out exactly as a booking's total
// (court and coach each rounded, then added). Empty when a price is missing or the currencies differ:
// booking then answers PRICE_NOT_SET / PRICE_CURRENCY_MISMATCH.
func (s *Service) lengthPrices(ctx context.Context, branchID uint, t *target, rule *domain.EffectiveRule) (string, []dto.LengthPriceDTO, error) {
	prices := []dto.LengthPriceDTO{}
	var rates []int64
	currency := ""
	type owner struct {
		kind string
		id   uint
	}
	var owners []owner
	if t.resource != nil {
		owners = append(owners, owner{domain.OwnerResource, t.resource.ID})
	}
	// a coach included free with the space adds nothing to the price
	if t.coach != nil && (t.resource == nil || !t.resource.IncludesCoach(t.coach.ID)) {
		owners = append(owners, owner{domain.OwnerCoach, t.coach.ID})
	}
	for _, o := range owners {
		p, err := s.prices.GetByOwner(ctx, branchID, o.kind, o.id)
		if err != nil {
			if isNotFound(err) {
				return "", prices, nil
			}
			return "", nil, err
		}
		if currency != "" && currency != p.Currency {
			return "", prices, nil
		}
		currency = p.Currency
		rates = append(rates, p.PricePerHour)
	}
	if rule.DurationStepMin <= 0 {
		return currency, prices, nil
	}
	for d := rule.MinDurationMin; d <= rule.MaxDurationMin; d += rule.DurationStepMin {
		var total int64
		for _, r := range rates {
			total += domain.PriceFor(r, d)
		}
		prices = append(prices, dto.LengthPriceDTO{DurationMin: d, Price: total})
	}
	return currency, prices, nil
}

// ---------- creating ----------

// booker is who the booking is for (see port.Booker)
type booker = port.Booker

// whoBooks applies the branch's booking mode
func whoBooks(b *domain.Branch, actor *domain.User, guest *dto.GuestInfo) (*booker, error) {
	switch {
	case b.BookingMode == domain.BookingModeAccount || (b.BookingMode == domain.BookingModeBoth && actor != nil):
		if actor == nil {
			return nil, response.NewError(http.StatusUnauthorized, "LOGIN_REQUIRED")
		}
		return &booker{Customer: actor}, nil
	default: // guest mode, or both without a login
		if guest == nil {
			return nil, badRequest("GUEST_DETAILS_REQUIRED")
		}
		g, err := cleanGuest(*guest)
		if err != nil {
			return nil, err
		}
		return &booker{Guest: g}, nil
	}
}

// cleanGuest validates a guest's details; the phone keeps only digits and a leading +
func cleanGuest(g dto.GuestInfo) (*dto.GuestInfo, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" || len([]rune(g.Name)) > maxGuestName {
		return nil, badRequest("GUEST_NAME_REQUIRED")
	}
	// a guest is reached by phone like an account: a Lao mobile, written 20xxxxxxxx (020 5555 1234 and +856 are accepted)
	phone, ok := domain.NormalizePhone(g.Phone)
	if !ok {
		return nil, badRequest("INVALID_PHONE")
	}
	g.Phone = phone
	g.Email = normalizeEmail(g.Email)
	if g.Email != "" {
		if addr, err := mail.ParseAddress(g.Email); err != nil || addr.Address != g.Email {
			return nil, badRequest("INVALID_EMAIL")
		}
	}
	return &g, nil
}

// checkBookingLimit applies the per-customer rule (by account, or by phone for guests); staff bookings skip it
func (s *Service) checkBookingLimit(ctx context.Context, branchID uint, who *booker, rule *domain.EffectiveRule) error {
	if who.Staff != nil {
		return nil
	}
	var active int64
	var err error
	if who.Customer != nil {
		active, err = s.bookings.CountActiveForCustomer(ctx, branchID, who.Customer.ID, s.now())
	} else {
		active, err = s.bookings.CountActiveForGuestPhone(ctx, branchID, who.Guest.Phone, s.now())
	}
	if err != nil {
		return err
	}
	if active >= int64(rule.MaxActiveBookingsPerCustomer) {
		return conflict("BOOKING_LIMIT_REACHED")
	}
	return nil
}

// checkWaitingLimit stops one phone number holding many unpaid bookings that wait for approval (a bot or a prank
// blocking the calendar): the phone of the account, or the one the guest typed, across all branches. Staff are
// never limited.
func (s *Service) checkWaitingLimit(ctx context.Context, who *booker) error {
	if who.Staff != nil {
		return nil
	}
	phone := ""
	switch {
	case who.Customer != nil:
		phone = who.Customer.Phone
	case who.Guest != nil:
		phone = who.Guest.Phone
	}
	if phone == "" {
		return nil
	}
	open, err := s.bookings.CountOpenWaitingForPhone(ctx, phone, s.now())
	if err != nil {
		return err
	}
	if open >= int64(s.openWaitingPerPhone) {
		return conflict("TOO_MANY_WAITING_BOOKINGS")
	}
	return nil
}

// newBooking fills in what every booking has: who, status, code, cancel deadline, guest token
// Staff bookings are confirmed at once and walk-ins get no private link (staff handle them).
func (s *Service) newBooking(b *domain.Branch, who *booker, sportID uint, start, end time.Time, rule *domain.EffectiveRule, notes string) (*domain.Booking, string, error) {
	status := domain.BookingConfirmed
	if b.RequireApproval && who.Staff == nil {
		status = domain.BookingPending
	}
	bk := &domain.Booking{
		BranchID: b.ID, SportID: sportID, Status: status, StartsAt: start, EndsAt: end,
		PaymentStatus: domain.PaymentUnpaid, Notes: strings.TrimSpace(notes),
		CancellableUntil: start.Add(-time.Duration(rule.CancelBeforeHours) * time.Hour),
	}
	var rawToken string
	switch {
	case who.Customer != nil:
		id := who.Customer.ID
		bk.CustomerID, bk.CreatedByType = &id, domain.CreatedByCustomer
	case who.Staff != nil: // walk-in
		bk.GuestName, bk.GuestPhone, bk.GuestEmail = who.Guest.Name, who.Guest.Phone, who.Guest.Email
	default:
		raw, hash, err := newToken()
		if err != nil {
			return nil, "", err
		}
		rawToken = raw
		bk.GuestName, bk.GuestPhone, bk.GuestEmail = who.Guest.Name, who.Guest.Phone, who.Guest.Email
		bk.GuestTokenHash, bk.CreatedByType = hash, domain.CreatedByGuest
	}
	if who.Staff != nil {
		staffID := who.Staff.ID
		bk.CreatedByType, bk.CreatedByID = domain.CreatedByStaff, &staffID
	}
	return bk, rawToken, nil
}

func (s *Service) Create(ctx context.Context, slug string, actor *domain.User, req dto.CreateBookingRequest) (*dto.CreatedBookingDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	if len([]rune(req.Notes)) > maxNotesChars {
		return nil, badRequest("NOTES_TOO_LONG")
	}
	who, err := whoBooks(b, actor, req.Guest)
	if err != nil {
		return nil, err
	}

	if err := s.checkWaitingLimit(ctx, who); err != nil {
		return nil, err
	}

	var bk *domain.Booking
	var rawToken string
	switch {
	case b.Type == domain.BranchTypeRestaurant:
		if req.PackageID != nil {
			return nil, badRequest("PACKAGE_NOT_FOR_TABLES")
		}
		bk, rawToken, err = s.tableBooker().CreateTableBooking(ctx, b, who, req)
	case req.ClassSessionID != nil:
		bk, rawToken, err = s.createClassBooking(ctx, b, who, req)
	default:
		bk, rawToken, err = s.createRental(ctx, b, who, req)
	}
	if err != nil {
		return nil, err
	}

	return s.created(ctx, b, bk, rawToken, actor, who), nil
}

// created reads the new booking back (so the answer names the court, coach, class or table), sends the
// guest their private link, and sends the other emails after the answer is ready
func (s *Service) created(ctx context.Context, b *domain.Branch, bk *domain.Booking, rawToken string, actor *domain.User, who *booker) *dto.CreatedBookingDTO {
	if full, err := s.bookings.GetByID(ctx, b.ID, bk.ID); err == nil {
		bk = full
	}
	bk.Branch = *b
	out := &dto.CreatedBookingDTO{BookingDTO: toBookingDTO(*bk)}
	if rawToken != "" {
		out.GuestToken = rawToken
		out.GuestLink = s.frontendURL + "/b/" + b.Slug + "/booking/" + rawToken
		if bk.GuestEmail != "" {
			if err := s.mailer.SendBookingLink(bk.GuestEmail, b.Name, bk.Code, out.GuestLink); err != nil {
				log.Printf("⚠️  booking link email for %s failed: %v", bk.Code, err) // the link is also in the response
			}
		}
	}
	// after the answer is ready: sending takes seconds and must not hold the customer up
	go s.notifyNewBooking(context.WithoutCancel(ctx), b, out, actor, who.Staff != nil, rawToken != "")
	return out
}

// notifyNewBooking sends the emails of a new booking: a receipt to the customer (a guest already got
// their private link) and, for a booking the customer made themselves, ONE email to the branch's
// active managers and staff. A failed email is logged and never undoes the booking.
func (s *Service) notifyNewBooking(ctx context.Context, b *domain.Branch, out *dto.CreatedBookingDTO, actor *domain.User, byStaff, guest bool) {
	if s.team != nil && !byStaff { // the team already knows about a booking one of them made
		s.team.NewBooking(ctx, b.ID, out.ID)
	}
	loc := locationOf(b)
	var what []string
	if out.SportName != "" {
		what = append(what, out.SportName)
	}
	for _, i := range out.Items {
		for _, name := range []string{i.ResourceName, i.CoachName, i.ClassTitle} {
			if name != "" {
				what = append(what, name)
			}
		}
	}
	info := output.BookingMail{
		BranchName: b.Name,
		Code:       out.Code,
		What:       strings.Join(what, " · "),
		When:       out.StartsAt.In(loc).Format("Mon 2 Jan 2006, 15:04") + "–" + out.EndsAt.In(loc).Format("15:04"),
		Price:      moneyText(out.TotalPrice, out.Currency),
		Pending:    out.Status == domain.BookingPending,
	}
	if out.Guests > 0 { // a table: no end time or price, the group size and when to arrive instead
		if out.Zones != "" {
			info.What += " · " + out.Zones
		}
		info.What = fmt.Sprintf("%s · %d guests", info.What, out.Guests)
		info.When = out.StartsAt.In(loc).Format("Mon 2 Jan 2006, 15:04")
		if out.BookingStyle == domain.BookingStyleArriveBy { // a bar: the evening, not a start time
			info.When = out.StartsAt.In(loc).Format("Mon 2 Jan 2006")
		}
		if out.ArriveBy != nil {
			info.When += ", please arrive by " + out.ArriveBy.In(loc).Format("15:04")
		}
		info.Price = "free to book"
	}
	if out.EventName != "" { // an event day: its name and its time of day (the booking itself runs from opening to closing)
		info.What = "Event " + out.EventName + " · " + info.What
		info.When = out.StartsAt.In(loc).Format("Mon 2 Jan 2006")
		if out.EventStartTime != "" {
			info.When += ", " + out.EventStartTime
			if out.EventEndTime != "" {
				info.When += "–" + out.EventEndTime
			}
		}
		info.Price = moneyText(out.TotalPrice, out.Currency)
	}

	// who booked, as the team should read it
	name, phone, customerEmail := out.GuestName, out.GuestPhone, ""
	if actor != nil && !byStaff {
		name, phone, customerEmail = actor.Name, actor.Phone, actor.ContactEmail()
	}
	info.Customer = strings.TrimSpace(name + " " + phone)

	if !guest { // a guest has their private link email; a customer or a walk-in gets a receipt
		to, link := customerEmail, s.frontendURL+"/account"
		if byStaff {
			to, link = "", s.frontendURL+"/b/"+b.Slug
			if out.CustomerEmail != "" {
				to = out.CustomerEmail
			}
		}
		if to != "" {
			info.Link = link
			if err := s.mailer.SendBookingReceived(to, info); err != nil {
				log.Printf("⚠️  booking receipt for %s failed: %v", out.Code, err)
			}
		}
	}

	if byStaff { // the team made it or knows about it
		return
	}
	members, err := s.memberships.ListByBranch(ctx, b.ID)
	if err != nil {
		log.Printf("⚠️  team email for booking %s: cannot list the team: %v", out.Code, err)
		return
	}
	var team []string
	seen := map[string]bool{strings.ToLower(customerEmail): true}
	for _, m := range members {
		email := m.User.ContactEmail()
		if !m.Active || (m.Role != domain.MemberManager && m.Role != domain.MemberStaff) || email == "" || seen[strings.ToLower(email)] {
			continue
		}
		seen[strings.ToLower(email)] = true
		team = append(team, email)
	}
	if len(team) == 0 {
		return
	}
	info.Link = fmt.Sprintf("%s/dashboard/%d/bookings", s.frontendURL, b.ID)
	if err := s.mailer.SendNewBooking(team, info); err != nil {
		log.Printf("⚠️  team email for booking %s failed: %v", out.Code, err)
	}
}

// moneyText writes an amount with thousands separators, e.g. 150,000 LAK
func moneyText(amount int64, currency string) string {
	digits := strconv.FormatInt(amount, 10)
	var sb strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 && d != '-' && digits[i-1] != '-' {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return strings.TrimSpace(sb.String() + " " + currency)
}

// createRental books a resource, a coach, or both for starts_at + duration_min
func (s *Service) createRental(ctx context.Context, b *domain.Branch, who *booker, req dto.CreateBookingRequest) (*domain.Booking, string, error) {
	var resourceID, coachID uint
	if req.ResourceID != nil {
		resourceID = *req.ResourceID
	}
	if req.CoachID != nil {
		coachID = *req.CoachID
	}
	t, err := s.resolveTarget(ctx, b.ID, resourceID, coachID, req.SportID)
	if err != nil {
		return nil, "", err
	}
	if req.StartsAt.IsZero() {
		return nil, "", badRequest("STARTS_AT_REQUIRED")
	}
	loc := locationOf(b)
	slots, rule, err := s.slotsFor(ctx, b, t, startOfDay(req.StartsAt, loc), who.Staff != nil)
	if err != nil {
		return nil, "", err
	}
	d := req.DurationMin
	if d < rule.MinDurationMin || d > rule.MaxDurationMin || (d-rule.MinDurationMin)%rule.DurationStepMin != 0 {
		return nil, "", badRequest("INVALID_DURATION")
	}
	if !domain.SlotAllows(slots, req.StartsAt, d) {
		return nil, "", conflict("SLOT_NOT_AVAILABLE")
	}
	if err := s.checkBookingLimit(ctx, b.ID, who, rule); err != nil {
		return nil, "", err
	}

	start := req.StartsAt.In(loc)
	end := start.Add(time.Duration(d) * time.Minute)
	var items []domain.BookingItem
	var total int64
	currency := ""
	add := func(ownerType string, ownerID uint) error {
		// a coach included free with the space: the coach item costs nothing (no price needed)
		if ownerType == domain.OwnerCoach && t.resource != nil && t.resource.IncludesCoach(ownerID) {
			id := ownerID
			items = append(items, domain.BookingItem{BranchID: b.ID, Kind: ownerType, StartsAt: start, EndsAt: end, Price: 0, Active: true, CoachID: &id})
			return nil
		}
		price, err := s.prices.GetByOwner(ctx, b.ID, ownerType, ownerID)
		if err != nil {
			if isNotFound(err) {
				return conflict("PRICE_NOT_SET")
			}
			return err
		}
		if currency != "" && currency != price.Currency {
			return conflict("PRICE_CURRENCY_MISMATCH")
		}
		currency = price.Currency
		item := domain.BookingItem{BranchID: b.ID, Kind: ownerType, StartsAt: start, EndsAt: end, Price: domain.PriceFor(price.PricePerHour, d), Active: true}
		id := ownerID
		if ownerType == domain.OwnerResource {
			item.ResourceID = &id
		} else {
			item.CoachID = &id
		}
		total += item.Price
		items = append(items, item)
		return nil
	}
	if t.resource != nil {
		if err := add(domain.OwnerResource, t.resource.ID); err != nil {
			return nil, "", err
		}
	}
	if t.coach != nil {
		if err := add(domain.OwnerCoach, t.coach.ID); err != nil {
			return nil, "", err
		}
	}

	bk, rawToken, err := s.newBooking(b, who, t.sportID, start, end, rule, req.Notes)
	if err != nil {
		return nil, "", err
	}
	bk.TotalPrice, bk.Currency, bk.Items = total, currency, items
	var withCoach *uint
	if t.coach != nil {
		id := t.coach.ID
		withCoach = &id
	}
	use, err := s.takePackage(ctx, b, who, req, start, int(end.Sub(start)/time.Minute), t.sportID, resourceRef(t), withCoach, nil)
	if err != nil {
		return nil, "", err
	}
	use.apply(bk, s.now())
	// the database refuses the insert if another booking took the time meanwhile (409)
	if err := s.bookings.Create(ctx, bk); err != nil {
		s.packageFailed(ctx, use)
		return nil, "", err
	}
	s.packageSaved(ctx, use, bk)
	return bk, rawToken, nil
}

// resourceRef is the id of the rented space (nil when the booking is a coach alone)
func resourceRef(t *target) *uint {
	if t.resource == nil {
		return nil
	}
	id := t.resource.ID
	return &id
}

// createClassBooking books seats in a group class
func (s *Service) createClassBooking(ctx context.Context, b *domain.Branch, who *booker, req dto.CreateBookingRequest) (*domain.Booking, string, error) {
	class, err := s.classes.GetByID(ctx, b.ID, *req.ClassSessionID)
	if err != nil {
		return nil, "", orBadRequest(err, "CLASS_NOT_FOUND")
	}
	if !class.Active {
		return nil, "", badRequest("CLASS_NOT_AVAILABLE")
	}
	seats := req.Seats
	if seats == 0 {
		seats = 1
	}
	if seats < 1 {
		return nil, "", badRequest("INVALID_SEATS")
	}
	var resourceID uint
	if class.ResourceID != nil {
		resourceID = *class.ResourceID
	}
	rule, err := s.rules.Effective(ctx, b.ID, class.SportID, resourceID)
	if err != nil {
		return nil, "", err
	}
	switch {
	case who.Staff != nil && !class.EndsAt.After(s.now()): // staff may add people until the class ends
		return nil, "", conflict("CLASS_ALREADY_ENDED")
	case who.Staff == nil && class.StartsAt.Before(s.now().Add(time.Duration(rule.MinNoticeMin)*time.Minute)):
		return nil, "", conflict("CLASS_STARTED_OR_TOO_SOON")
	}
	if err := s.checkBookingLimit(ctx, b.ID, who, rule); err != nil {
		return nil, "", err
	}

	loc := locationOf(b)
	start, end := class.StartsAt.In(loc), class.EndsAt.In(loc)
	bk, rawToken, err := s.newBooking(b, who, class.SportID, start, end, rule, req.Notes)
	if err != nil {
		return nil, "", err
	}
	classID := class.ID
	price := class.Price * int64(seats)
	bk.TotalPrice, bk.Currency = price, class.Currency
	bk.Items = []domain.BookingItem{{
		BranchID: b.ID, Kind: domain.ItemClassSession, ClassSessionID: &classID, Seats: seats,
		StartsAt: start, EndsAt: end, Price: price, Active: true,
	}}
	// a package pays one visit however many seats (a group class booking through a package is for one person)
	if req.PackageID != nil && seats != 1 {
		return nil, "", badRequest("PACKAGE_ONE_SEAT")
	}
	use, err := s.takePackage(ctx, b, who, req, start, 0, class.SportID, nil, nil, class.TimetableID)
	if err != nil {
		return nil, "", err
	}
	use.apply(bk, s.now())
	// the class row is locked while seats are counted, so the last seat goes to one customer only
	if err := s.bookings.CreateClassBooking(ctx, bk, class.ID, seats); err != nil {
		s.packageFailed(ctx, use)
		return nil, "", err
	}
	s.packageSaved(ctx, use, bk)
	return bk, rawToken, nil
}

// ---------- viewing and cancelling ----------

func toBookingDTO(b domain.Booking) dto.BookingDTO {
	d := dto.BookingDTO{
		ID: b.ID, Code: b.Code, SportID: b.SportID, BranchName: b.Branch.Name, BranchSlug: b.Branch.Slug,
		Status: b.Status, StartsAt: b.StartsAt, EndsAt: b.EndsAt, TotalPrice: b.TotalPrice, Currency: b.Currency,
		PaymentStatus: b.PaymentStatus, CancellableUntil: b.CancellableUntil, GuestName: b.GuestName,
		GuestPhone: b.GuestPhone, CustomerID: b.CustomerID, PaymentMethod: b.PaymentMethod, PaidAt: b.PaidAt,
		CreatedByType: b.CreatedByType,
		Notes:         b.Notes, CancelledAt: b.CancelledAt, CancelReason: b.CancelReason, CreatedAt: b.CreatedAt,
		Items: mapSlice(b.Items, func(i domain.BookingItem) dto.BookingItemDTO {
			item := dto.BookingItemDTO{
				Kind: i.Kind, ResourceID: i.ResourceID, CoachID: i.CoachID, ClassSessionID: i.ClassSessionID,
				Seats: i.Seats, StartsAt: i.StartsAt, EndsAt: i.EndsAt, Price: i.Price, Active: i.Active,
			}
			if i.Resource != nil {
				item.ResourceName = i.Resource.Name
			}
			if i.Coach != nil {
				item.CoachName = i.Coach.Name
			}
			if i.ClassSession != nil {
				item.ClassTitle = i.ClassSession.Title
			}
			return item
		}),
	}
	if b.Sport != nil {
		d.SportName = b.Sport.Name
	}
	d.EventID, d.PayBy, d.SlipSentAt, d.SlipVia, d.CustomerPackageID = b.EventID, b.PayBy, b.SlipSentAt, b.SlipVia, b.CustomerPackageID
	d.Package, d.PackageCode = b.PackageLabel, b.PackageCode
	if b.Event != nil {
		d.EventName, d.EventStartTime, d.EventEndTime = b.Event.Name, b.Event.StartTime, b.Event.EndTime
	}
	if b.Guests > 0 {
		d.Guests, d.ArrivedAt, d.ArriveBy, d.BookingStyle = b.Guests, b.ArrivedAt, b.ArriveBy, b.Branch.BookingStyle
		if b.Sport != nil {
			floor := b.Sport.Floor
			d.AreaFloor = &floor
		}
		d.Zones = b.Zones
	}
	if b.Customer != nil {
		d.CustomerName, d.CustomerEmail = b.Customer.Name, b.Customer.ContactEmail()
		d.CustomerPhone, d.MemberNo = b.Customer.Phone, domain.MemberNoOf(b.Customer.MemberNo)
	}
	return d
}

// cancel applies the customer rules: only active bookings, and only until the cancel deadline (an unpaid event
// booking can always be cancelled by its customer, see Booking.CanStartOver)
func (s *Service) cancel(ctx context.Context, b *domain.Booking, reason string) (*dto.BookingDTO, error) {
	if !b.IsActive() {
		return nil, conflict("BOOKING_NOT_ACTIVE")
	}
	now := s.now()
	if now.After(b.CancellableUntil) && !b.CanStartOver() {
		return nil, conflict("CANCELLATION_WINDOW_PASSED")
	}
	if len([]rune(reason)) > 255 {
		return nil, badRequest("REASON_TOO_LONG")
	}
	if err := s.bookings.Cancel(ctx, b, strings.TrimSpace(reason), now); err != nil {
		return nil, err
	}
	s.packageCancelled(ctx, b)
	if s.team != nil {
		s.team.Cancelled(ctx, b.BranchID, b.ID, "customer")
	}
	d := toBookingDTO(*b)
	return &d, nil
}

// guestBooking finds a booking by its private link; any branch status, so guests can still
// see and cancel after a branch unpublishes
func (s *Service) guestBooking(ctx context.Context, slug, token string) (*domain.Booking, error) {
	gone := notFound("BOOKING_NOT_FOUND")
	b, err := s.branches.GetBySlug(ctx, slug)
	if err != nil {
		if isNotFound(err) {
			return nil, gone
		}
		return nil, err
	}
	bk, err := s.bookings.GetByGuestToken(ctx, b.ID, hashToken(token))
	if err != nil {
		return nil, err
	}
	if bk == nil {
		return nil, gone
	}
	return bk, nil
}

// samePhone compares two phone numbers by their last 8 digits, so "020 5555 1234",
// "+856 20 5555 1234" and "2055551234" are the same number
func samePhone(a, b string) bool {
	digits := func(s string) string {
		var out strings.Builder
		for _, r := range s {
			if unicode.IsDigit(r) {
				out.WriteRune(r)
			}
		}
		return out.String()
	}
	x, y := digits(a), digits(b)
	if len(x) < 8 || len(y) < 8 {
		return false
	}
	return x[len(x)-8:] == y[len(y)-8:]
}

// foundBooking finds a booking by its code and the phone it was made with (a guest's phone or
// the customer's account phone). A wrong code and a wrong phone get the same answer.
func (s *Service) foundBooking(ctx context.Context, slug string, req dto.FindBookingRequest) (*domain.Booking, error) {
	gone := notFound("NO_BOOKING_WITH_CODE_AND_PHONE")
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" || strings.TrimSpace(req.Phone) == "" {
		return nil, badRequest("CODE_AND_PHONE_REQUIRED")
	}
	b, err := s.branches.GetBySlug(ctx, slug)
	if err != nil {
		if isNotFound(err) {
			return nil, gone
		}
		return nil, err
	}
	bk, err := s.bookings.GetByCode(ctx, b.ID, code)
	if err != nil {
		return nil, err
	}
	if bk == nil {
		return nil, gone
	}
	phone := bk.GuestPhone
	if bk.Customer != nil && bk.Customer.Phone != "" {
		phone = bk.Customer.Phone
	}
	if !samePhone(phone, req.Phone) {
		return nil, gone
	}
	return bk, nil
}

// FindBooking shows a booking to whoever knows its code and phone
func (s *Service) FindBooking(ctx context.Context, slug string, req dto.FindBookingRequest) (*dto.BookingDTO, error) {
	bk, err := s.foundBooking(ctx, slug, req)
	if err != nil {
		return nil, err
	}
	d := toBookingDTO(*bk)
	return &d, nil
}

// FindCancel cancels a booking found by code and phone, with the same rules as the guest link
func (s *Service) FindCancel(ctx context.Context, slug string, req dto.FindBookingRequest) (*dto.BookingDTO, error) {
	bk, err := s.foundBooking(ctx, slug, req)
	if err != nil {
		return nil, err
	}
	return s.cancel(ctx, bk, req.Reason)
}

func (s *Service) GuestView(ctx context.Context, slug, token string) (*dto.BookingDTO, error) {
	bk, err := s.guestBooking(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	d := toBookingDTO(*bk)
	return &d, nil
}

func (s *Service) GuestCancel(ctx context.Context, slug, token string, req dto.CancelBookingRequest) (*dto.BookingDTO, error) {
	bk, err := s.guestBooking(ctx, slug, token)
	if err != nil {
		return nil, err
	}
	return s.cancel(ctx, bk, req.Reason)
}

var bookingStatuses = map[string]bool{
	domain.BookingPending: true, domain.BookingConfirmed: true, domain.BookingCancelled: true,
	domain.BookingCompleted: true, domain.BookingNoShow: true,
}

func (s *Service) MyBookings(ctx context.Context, userID uint, q dto.ListQuery) (*dto.PagedResult[dto.BookingDTO], error) {
	lq, page, limit := newListQuery(q)
	if q.Status != "" {
		if !bookingStatuses[q.Status] {
			return nil, badRequest("INVALID_STATUS")
		}
		lq.Filters["status"] = q.Status
	}
	items, total, err := s.bookings.ListForCustomer(ctx, userID, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, toBookingDTO), nil
}

func (s *Service) MyBooking(ctx context.Context, userID, id uint) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetForCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	d := toBookingDTO(*bk)
	return &d, nil
}

// ClaimMine adds a booking made without an account to the customer's account. The booking must have been made
// with the phone number that is on the account (the same last 8 digits); a wrong code, another phone and a booking
// that belongs to someone else all get the same answer, so nobody can learn which codes exist.
func (s *Service) ClaimMine(ctx context.Context, userID uint, req dto.ClaimBookingRequest) (*dto.BookingDTO, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return nil, badRequest("CODE_REQUIRED")
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(u.Phone) == "" {
		return nil, badRequest("ADD_PHONE_TO_ADD_BOOKING")
	}
	gone := notFound("NO_BOOKING_WITH_CODE_AND_YOUR_PHONE")
	bk, err := s.bookings.GetByCodeAnywhere(ctx, code)
	if err != nil {
		return nil, err
	}
	if bk == nil || !samePhone(bk.GuestPhone, u.Phone) {
		return nil, gone
	}
	if bk.CustomerID != nil {
		if *bk.CustomerID != userID {
			return nil, gone
		}
	} else if err := s.bookings.SetCustomer(ctx, bk, userID); err != nil {
		return nil, err
	}
	d := toBookingDTO(*bk)
	return &d, nil
}

func (s *Service) CancelMine(ctx context.Context, userID, id uint, req dto.CancelBookingRequest) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetForCustomer(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.cancel(ctx, bk, req.Reason)
}
