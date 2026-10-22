package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// The branch team's booking desk. These methods live on BookingServiceImpl so staff bookings go
// through exactly the same slot check as customer bookings (see slotsFor).

// BookingServiceImpl serves both the public booking page and the desk
var _ input.DeskService = (*BookingServiceImpl)(nil)

const maxCalendarDays = 7

var paymentMethods = map[string]bool{
	domain.MethodCash: true, domain.MethodTransfer: true, domain.MethodQR: true, domain.MethodOther: true,
}

// parseDay reads YYYY-MM-DD as midnight in loc
func parseDay(date string, loc *time.Location) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, badRequest("INVALID_DATE")
	}
	return day, nil
}

func (s *BookingServiceImpl) branch(ctx context.Context, branchID uint) (*domain.Branch, *time.Location, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, nil, err
	}
	return b, locationOf(b), nil
}

func (s *BookingServiceImpl) ListBookings(ctx context.Context, branchID uint, q dto.BookingQuery) (*dto.PagedResult[dto.BookingDTO], error) {
	_, loc, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	f := output.BookingFilter{
		SportID: q.SportID, ResourceID: q.ResourceID, CoachID: q.CoachID, CustomerID: q.CustomerID,
		Status: q.Status, PaymentStatus: q.PaymentStatus, Keyword: strings.TrimSpace(q.Keyword),
	}
	if f.Status != "" && !bookingStatuses[f.Status] {
		return nil, badRequest("INVALID_STATUS")
	}
	if f.PaymentStatus != "" && f.PaymentStatus != domain.PaymentPaid && f.PaymentStatus != domain.PaymentUnpaid {
		return nil, badRequest("INVALID_PAYMENT_STATUS")
	}
	if q.From != "" {
		from, err := parseDay(q.From, loc)
		if err != nil {
			return nil, err
		}
		f.From = &from
	}
	if q.To != "" {
		to, err := parseDay(q.To, loc)
		if err != nil {
			return nil, err
		}
		end := to.AddDate(0, 0, 1) // the "to" day is included
		f.To = &end
	}
	lq, page, limit := newListQuery(dto.ListQuery{Page: q.Page, Limit: q.Limit})
	items, total, err := s.bookings.List(ctx, branchID, f, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, toBookingDTO), nil
}

func (s *BookingServiceImpl) GetBooking(ctx context.Context, branchID, id uint) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	d := toBookingDTO(*bk)
	return &d, nil
}

// CreateForCustomer books for an existing customer (exact email) or a walk-in (name + phone)
func (s *BookingServiceImpl) CreateForCustomer(ctx context.Context, branchID uint, actor *domain.User, req dto.StaffBookingRequest) (*dto.BookingDTO, error) {
	b, _, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if len([]rune(req.Notes)) > maxNotesChars {
		return nil, badRequest("NOTES_TOO_LONG")
	}
	who := &booker{staff: actor}
	find := strings.TrimSpace(req.Customer)
	if find == "" {
		find = req.CustomerEmail
	}
	if strings.TrimSpace(find) != "" {
		customer, err := s.findAccount(ctx, find)
		if err != nil {
			return nil, err
		}
		who.customer = customer
	} else {
		if req.Guest == nil {
			return nil, badRequest("CUSTOMER_OR_WALK_IN_REQUIRED")
		}
		if who.guest, err = cleanGuest(*req.Guest); err != nil {
			return nil, err
		}
	}

	var bk *domain.Booking
	if req.ClassSessionID != nil {
		bk, _, err = s.createClassBooking(ctx, b, who, req.CreateBookingRequest)
	} else {
		bk, _, err = s.createRental(ctx, b, who, req.CreateBookingRequest)
	}
	if err != nil {
		return nil, err
	}
	return s.GetBooking(ctx, branchID, bk.ID)
}

// ChangeStatus moves a booking one way: pending → confirmed / cancelled; confirmed → cancelled /
// completed / no-show. Completed and no-show need the booking to have started. Staff may cancel
// at any time (the customer window does not apply); the reason is kept.
func (s *BookingServiceImpl) ChangeStatus(ctx context.Context, branchID, id uint, req dto.StatusChangeRequest) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if !domain.CanChangeStatus(bk.Status, req.Status) {
		return nil, badRequest("INVALID_STATUS_CHANGE")
	}
	if len([]rune(req.Reason)) > 255 {
		return nil, badRequest("REASON_TOO_LONG")
	}
	now := s.now()
	switch req.Status {
	case domain.BookingCompleted, domain.BookingNoShow:
		if now.Before(bk.StartsAt) {
			return nil, badRequest("BOOKING_NOT_STARTED")
		}
		err = s.bookings.SetStatus(ctx, bk, req.Status)
	case domain.BookingCancelled:
		err = s.bookings.Cancel(ctx, bk, strings.TrimSpace(req.Reason), now)
	default:
		err = s.bookings.SetStatus(ctx, bk, req.Status)
	}
	if err != nil {
		return nil, err
	}
	if req.Status == domain.BookingCancelled || req.Status == domain.BookingConfirmed {
		s.notifyStatusChange(bk, req.Status, strings.TrimSpace(req.Reason))
	}
	return s.GetBooking(ctx, branchID, id)
}

// SetPayment records money taken at the venue: paid with a method (time and staff member saved),
// or back to unpaid if it was marked by mistake
func (s *BookingServiceImpl) SetPayment(ctx context.Context, branchID uint, actor *domain.User, id uint, req dto.PaymentRequest) (*dto.BookingDTO, error) {
	bk, err := s.bookings.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	switch req.Status {
	case domain.PaymentPaid:
		if !paymentMethods[req.Method] {
			return nil, badRequest("INVALID_PAYMENT_METHOD")
		}
		if bk.Status == domain.BookingCancelled {
			return nil, conflict("BOOKING_CANCELLED")
		}
		now, actorID := s.now(), actor.ID
		bk.PaymentStatus, bk.PaymentMethod, bk.PaidAt, bk.PaidByID = domain.PaymentPaid, req.Method, &now, &actorID
	case domain.PaymentUnpaid:
		bk.PaymentStatus, bk.PaymentMethod, bk.PaidAt, bk.PaidByID = domain.PaymentUnpaid, "", nil, nil
	default:
		return nil, badRequest("INVALID_PAYMENT_STATUS")
	}
	if err := s.bookings.SetPayment(ctx, bk); err != nil {
		return nil, err
	}
	return s.GetBooking(ctx, branchID, id)
}

// bookerName is who a booking is for, as staff see it
func bookerName(b domain.Booking) string {
	if b.Customer != nil {
		return b.Customer.Name
	}
	return b.GuestName
}

// Calendar lays out bookings, classes and closures on one line per court and per coach
func (s *BookingServiceImpl) Calendar(ctx context.Context, branchID uint, date string, days int) (*dto.CalendarDTO, error) {
	_, loc, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	from, err := parseDay(date, loc)
	if err != nil {
		return nil, err
	}
	if days == 0 {
		days = 1
	}
	if days < 1 || days > maxCalendarDays {
		return nil, badRequest("DAYS_MUST_BE_1_TO_7")
	}
	to := from.AddDate(0, 0, days)

	all := output.ListQuery{} // a branch has few courts and coaches
	resources, _, err := s.resources.List(ctx, branchID, all)
	if err != nil {
		return nil, err
	}
	coaches, _, err := s.coaches.List(ctx, branchID, all)
	if err != nil {
		return nil, err
	}
	lanes := map[string]*dto.CalendarLane{}
	var order []string
	addLane := func(ownerType string, id uint, name string) {
		lanes[laneKey(ownerType, id)] = &dto.CalendarLane{OwnerType: ownerType, OwnerID: id, Name: name, Entries: []dto.CalendarEntry{}}
		order = append(order, laneKey(ownerType, id))
	}
	for _, r := range resources {
		addLane(domain.OwnerResource, r.ID, r.Name)
	}
	for _, c := range coaches {
		addLane(domain.OwnerCoach, c.ID, c.Name)
	}
	put := func(ownerType string, id *uint, e dto.CalendarEntry) {
		if id == nil {
			return
		}
		if lane, ok := lanes[laneKey(ownerType, *id)]; ok {
			lane.Entries = append(lane.Entries, e)
		}
	}

	bookings, err := s.bookings.InRange(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, b := range bookings {
		if b.Status == domain.BookingCancelled {
			continue
		}
		for _, item := range b.Items {
			e := dto.CalendarEntry{
				Kind: "booking", StartsAt: item.StartsAt.In(loc), EndsAt: item.EndsAt.In(loc), BookingID: b.ID,
				Code: b.Code, Status: b.Status, Name: bookerName(b), Paid: b.PaymentStatus == domain.PaymentPaid,
			}
			put(domain.OwnerResource, item.ResourceID, e)
			put(domain.OwnerCoach, item.CoachID, e)
		}
	}

	classes, err := s.classes.ListBetween(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	ids := mapSlice(classes, func(c domain.ClassSession) uint { return c.ID })
	taken, err := s.bookings.SeatsTaken(ctx, branchID, ids)
	if err != nil {
		return nil, err
	}
	for _, c := range classes {
		if !c.Active {
			continue
		}
		e := dto.CalendarEntry{
			Kind: "class", StartsAt: c.StartsAt.In(loc), EndsAt: c.EndsAt.In(loc), ClassID: c.ID,
			Title: c.Title, SeatsTaken: taken[c.ID], Capacity: c.Capacity,
		}
		put(domain.OwnerResource, c.ResourceID, e)
		put(domain.OwnerCoach, c.CoachID, e)
	}

	closures, err := s.closures.Overlapping(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	for _, c := range closures {
		e := dto.CalendarEntry{Kind: "closure", StartsAt: c.StartsAt.In(loc), EndsAt: c.EndsAt.In(loc), Reason: c.Reason}
		if c.OwnerType == domain.OwnerVenue {
			for _, lane := range lanes {
				lane.Entries = append(lane.Entries, e)
			}
			continue
		}
		id := c.OwnerID
		put(c.OwnerType, &id, e)
	}

	out := &dto.CalendarDTO{From: from, To: to, Timezone: loc.String(), Lanes: make([]dto.CalendarLane, 0, len(order))}
	for _, key := range order {
		lane := lanes[key]
		sort.SliceStable(lane.Entries, func(i, j int) bool { return lane.Entries[i].StartsAt.Before(lane.Entries[j].StartsAt) })
		out.Lanes = append(out.Lanes, *lane)
	}
	return out, nil
}

func laneKey(ownerType string, id uint) string { return fmt.Sprintf("%s:%d", ownerType, id) }

// Customers lists accounts that booked with this branch (never customers of other branches)
func (s *BookingServiceImpl) Customers(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.CustomerDTO], error) {
	lq, page, limit := newListQuery(q)
	rows, total, err := s.bookings.Customers(ctx, branchID, lq.Keyword, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(rows, total, page, limit, func(r output.CustomerRow) dto.CustomerDTO {
		return dto.CustomerDTO{
			UserID: r.UserID, CustomerNo: domain.FormatCustomerNo(r.UserID), Name: r.Name, Phone: r.Phone,
			Email:    contactEmail(r.Email),
			Bookings: r.Bookings, LastBookingAt: r.LastBookingAt,
		}
	}), nil
}

// notifyStatusChange emails the customer (or a guest who gave an email) after staff confirm or cancel.
// A failed email is logged and never undoes the change.
func (s *BookingServiceImpl) notifyStatusChange(bk *domain.Booking, status, reason string) {
	to, name := bk.GuestEmail, bk.GuestName
	if bk.Customer != nil {
		to, name = bk.Customer.Email, bk.Customer.Name
	}
	if to == "" {
		return // walk-ins without an email are told in person
	}
	link := s.frontendURL + "/b/" + bk.Branch.Slug
	if err := s.mailer.SendBookingUpdate(to, name, bk.Branch.Name, bk.Code, status, reason, link); err != nil {
		log.Printf("⚠️  status email for booking %s failed: %v", bk.Code, err)
	}
}

// findAccount finds a customer's account by what they can tell staff: phone number, customer number
// (C0042) or email
func (s *BookingServiceImpl) findAccount(ctx context.Context, q string) (*domain.User, error) {
	q = strings.TrimSpace(q)
	var u *domain.User
	var err error
	switch {
	case strings.Contains(q, "@"):
		u, err = s.users.FindByEmail(ctx, normalizeEmail(q))
	case strings.HasPrefix(strings.ToUpper(q), "C"):
		id, ok := domain.ParseCustomerNo(q)
		if !ok {
			return nil, badRequest("CUSTOMER_NOT_FOUND")
		}
		u, err = s.users.FindByID(ctx, id)
	default:
		phone, ok := domain.NormalizePhone(q)
		if !ok {
			return nil, badRequest("CUSTOMER_NOT_FOUND")
		}
		u, err = s.users.FindByPhone(ctx, phone)
	}
	if err != nil {
		return nil, orBadRequest(err, "CUSTOMER_NOT_FOUND")
	}
	if !u.Status {
		return nil, badRequest("CUSTOMER_NOT_FOUND")
	}
	return u, nil
}

// FindCustomer shows staff whose account it is before they book for them
func (s *BookingServiceImpl) FindCustomer(ctx context.Context, branchID uint, q string) (*dto.CustomerMatchDTO, error) {
	if _, _, err := s.branch(ctx, branchID); err != nil {
		return nil, err
	}
	u, err := s.findAccount(ctx, q)
	if err != nil {
		return nil, err
	}
	return &dto.CustomerMatchDTO{
		UserID: u.ID, CustomerNo: domain.FormatCustomerNo(u.ID), Name: u.Name, Phone: u.Phone, Email: u.ContactEmail(),
	}, nil
}

// contactEmail hides stand-in addresses from staff
func contactEmail(email string) string {
	if domain.IsGeneratedEmail(email) {
		return ""
	}
	return email
}
