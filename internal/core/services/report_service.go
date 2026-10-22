package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"context"
	"math"
	"sort"
	"time"
)

const (
	maxReportDays = 366
	topCustomers  = 10
)

type ReportServiceImpl struct {
	branches  output.BranchRepository
	bookings  output.BookingRepository
	resources output.ResourceRepository
	sports    output.SportRepository
	hours     output.OpeningHourRepository
	closures  output.ClosureRepository
	now       func() time.Time
}

func NewReportService(
	branches output.BranchRepository,
	bookings output.BookingRepository,
	resources output.ResourceRepository,
	sports output.SportRepository,
	hours output.OpeningHourRepository,
	closures output.ClosureRepository,
) input.ReportService {
	return &ReportServiceImpl{
		branches: branches, bookings: bookings, resources: resources, sports: sports,
		hours: hours, closures: closures, now: time.Now,
	}
}

// period reads from / to (both days included) in the branch timezone
func (s *ReportServiceImpl) period(ctx context.Context, branchID uint, q dto.ReportQuery) (*time.Location, time.Time, time.Time, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	loc := locationOf(b)
	from, err := parseDay(q.From, loc)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	last, err := parseDay(q.To, loc)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	to := last.AddDate(0, 0, 1)
	if !to.After(from) {
		return nil, time.Time{}, time.Time{}, badRequest("TO_BEFORE_FROM")
	}
	if to.Sub(from) > maxReportDays*24*time.Hour {
		return nil, time.Time{}, time.Time{}, badRequest("PERIOD_TOO_LONG")
	}
	return loc, from, to, nil
}

// startingIn returns the bookings that start in [from, to): reports count a booking on its start day
func (s *ReportServiceImpl) startingIn(ctx context.Context, branchID uint, from, to time.Time) ([]domain.Booking, error) {
	all, err := s.bookings.InRange(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, b := range all {
		if !b.StartsAt.Before(from) && b.StartsAt.Before(to) {
			out = append(out, b)
		}
	}
	return out, nil
}

// days lists every date of the period as YYYY-MM-DD
func days(from, to time.Time) []string {
	var out []string
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

// moneyList turns per-currency sums into a list sorted by currency
func moneyList(m map[string]int64) []dto.Money {
	out := make([]dto.Money, 0, len(m))
	for c, a := range m {
		out = append(out, dto.Money{Currency: c, Amount: a})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}

func contactOf(b domain.Booking) string {
	if b.Customer != nil {
		if email := b.Customer.ContactEmail(); email != "" {
			return email
		}
		return b.Customer.Phone
	}
	return b.GuestPhone
}

// Bookings counts bookings starting in the period. Per sport, booked value and top customers leave out cancelled bookings.
func (s *ReportServiceImpl) Bookings(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.BookingsReportDTO, error) {
	loc, from, to, err := s.period(ctx, branchID, q)
	if err != nil {
		return nil, err
	}
	bookings, err := s.startingIn(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	sports, _, err := s.sports.List(ctx, branchID, output.ListQuery{})
	if err != nil {
		return nil, err
	}
	sportName := map[uint]string{}
	for _, sp := range sports {
		sportName[sp.ID] = sp.Name
	}

	out := &dto.BookingsReportDTO{From: q.From, To: q.To, Total: len(bookings), ByStatus: map[string]int{}}
	perDay := map[string]*dto.DayCount{}
	for _, d := range days(from, to) {
		perDay[d] = &dto.DayCount{Date: d, ByStatus: map[string]int{}}
	}
	perSport := map[uint]int{}
	booked := map[string]int64{}
	type customerTotals struct {
		top   dto.TopCustomer
		value map[string]int64
	}
	customers := map[string]*customerTotals{}

	for _, b := range bookings {
		out.ByStatus[b.Status]++
		day := perDay[b.StartsAt.In(loc).Format("2006-01-02")]
		day.Total++
		day.ByStatus[b.Status]++
		if b.Status == domain.BookingCancelled {
			continue
		}
		perSport[b.SportID]++
		booked[b.Currency] += b.TotalPrice
		key := "guest:" + b.GuestPhone
		if b.CustomerID != nil {
			key = "customer:" + contactOf(b)
		}
		c, ok := customers[key]
		if !ok {
			c = &customerTotals{top: dto.TopCustomer{Name: bookerName(b), Contact: contactOf(b)}, value: map[string]int64{}}
			customers[key] = c
		}
		c.top.Bookings++
		c.value[b.Currency] += b.TotalPrice
	}

	for _, d := range days(from, to) {
		out.PerDay = append(out.PerDay, *perDay[d])
	}
	for id, n := range perSport {
		out.PerSport = append(out.PerSport, dto.SportCount{SportID: id, Name: sportName[id], Total: n})
	}
	sort.Slice(out.PerSport, func(i, j int) bool {
		if out.PerSport[i].Total != out.PerSport[j].Total {
			return out.PerSport[i].Total > out.PerSport[j].Total
		}
		return out.PerSport[i].Name < out.PerSport[j].Name
	})
	out.BookedValue = moneyList(booked)

	tops := make([]dto.TopCustomer, 0, len(customers))
	for _, c := range customers {
		c.top.Value = moneyList(c.value)
		tops = append(tops, c.top)
	}
	sort.Slice(tops, func(i, j int) bool {
		if tops[i].Bookings != tops[j].Bookings {
			return tops[i].Bookings > tops[j].Bookings
		}
		return tops[i].Name < tops[j].Name
	})
	if len(tops) > topCustomers {
		tops = tops[:topCustomers]
	}
	out.TopCustomers = tops
	return out, nil
}

// openMinutes is the length of the open intervals minus the closures inside them
func openMinutes(open, closed []domain.Interval) int {
	total := time.Duration(0)
	for _, o := range open {
		var cut []domain.Interval
		for _, c := range closed {
			start, end := maxTime(o.Start, c.Start), minTime(o.End, c.End)
			if start.Before(end) {
				cut = append(cut, domain.Interval{Start: start, End: end})
			}
		}
		total += o.End.Sub(o.Start) - unionLength(cut)
	}
	return int(total / time.Minute)
}

// unionLength measures overlapping intervals once
func unionLength(list []domain.Interval) time.Duration {
	sort.Slice(list, func(i, j int) bool { return list[i].Start.Before(list[j].Start) })
	var total time.Duration
	var cur *domain.Interval
	for i := range list {
		iv := list[i]
		if cur == nil || iv.Start.After(cur.End) {
			if cur != nil {
				total += cur.End.Sub(cur.Start)
			}
			cur = &iv
			continue
		}
		if iv.End.After(cur.End) {
			cur.End = iv.End
		}
	}
	if cur != nil {
		total += cur.End.Sub(cur.Start)
	}
	return total
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Utilisation compares booked time with open time per court / pitch / ring.
// Classes count as booked; time closed for holidays or maintenance does not count as open.
func (s *ReportServiceImpl) Utilisation(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.UtilisationDTO, error) {
	_, from, to, err := s.period(ctx, branchID, q)
	if err != nil {
		return nil, err
	}
	resources, _, err := s.resources.List(ctx, branchID, output.ListQuery{})
	if err != nil {
		return nil, err
	}
	closures, err := s.closures.Overlapping(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	out := &dto.UtilisationDTO{From: q.From, To: q.To, Resources: []dto.ResourceUse{}}
	for _, r := range resources {
		rows, err := s.hours.ListByOwner(ctx, branchID, domain.OwnerResource, r.ID)
		if err != nil {
			return nil, err
		}
		var open []domain.Interval
		for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
			open = append(open, openIntervalsOn(rows, d)...)
		}
		var closed []domain.Interval
		for _, c := range closures {
			if c.OwnerType == domain.OwnerVenue || (c.OwnerType == domain.OwnerResource && c.OwnerID == r.ID) {
				closed = append(closed, domain.Interval{Start: c.StartsAt, End: c.EndsAt})
			}
		}
		items, err := s.bookings.ActiveItems(ctx, branchID, domain.OwnerResource, r.ID, from, to)
		if err != nil {
			return nil, err
		}
		booked := time.Duration(0)
		for _, it := range items {
			booked += minTime(it.EndsAt, to).Sub(maxTime(it.StartsAt, from))
		}
		use := dto.ResourceUse{ResourceID: r.ID, Name: r.Name, OpenMinutes: openMinutes(open, closed), BookedMinutes: int(booked / time.Minute)}
		if use.OpenMinutes > 0 {
			use.Percent = math.Round(float64(use.BookedMinutes)/float64(use.OpenMinutes)*1000) / 10
		}
		out.Resources = append(out.Resources, use)
	}
	return out, nil
}

// Finance sums money of bookings starting in the period, per currency. For every currency:
// booked value = paid + outstanding + upcoming unpaid + lost to no-shows.
func (s *ReportServiceImpl) Finance(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.FinanceDTO, error) {
	loc, from, to, err := s.period(ctx, branchID, q)
	if err != nil {
		return nil, err
	}
	bookings, err := s.startingIn(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	now := s.now()
	totals := map[string]*dto.FinanceTotals{}
	perDay := map[string]*dto.FinanceDay{}
	for _, b := range bookings {
		t, ok := totals[b.Currency]
		if !ok {
			t = &dto.FinanceTotals{Currency: b.Currency}
			totals[b.Currency] = t
		}
		if b.Status == domain.BookingCancelled {
			t.LostCancelled += b.TotalPrice
			continue
		}
		t.BookedValue += b.TotalPrice
		paid := b.PaymentStatus == domain.PaymentPaid
		switch {
		case paid:
			t.Paid += b.TotalPrice
		case b.Status == domain.BookingNoShow:
			t.LostNoShow += b.TotalPrice
		case b.EndsAt.Before(now):
			t.Outstanding += b.TotalPrice
		default:
			t.UpcomingUnpaid += b.TotalPrice
		}
		key := b.StartsAt.In(loc).Format("2006-01-02") + "|" + b.Currency
		d, ok := perDay[key]
		if !ok {
			d = &dto.FinanceDay{Date: b.StartsAt.In(loc).Format("2006-01-02"), Currency: b.Currency}
			perDay[key] = d
		}
		d.BookedValue += b.TotalPrice
		if paid {
			d.Paid += b.TotalPrice
		}
	}
	out := &dto.FinanceDTO{From: q.From, To: q.To, Totals: []dto.FinanceTotals{}, PerDay: []dto.FinanceDay{}}
	for _, t := range totals {
		out.Totals = append(out.Totals, *t)
	}
	sort.Slice(out.Totals, func(i, j int) bool { return out.Totals[i].Currency < out.Totals[j].Currency })
	for _, d := range perDay {
		out.PerDay = append(out.PerDay, *d)
	}
	sort.Slice(out.PerDay, func(i, j int) bool {
		if out.PerDay[i].Date != out.PerDay[j].Date {
			return out.PerDay[i].Date < out.PerDay[j].Date
		}
		return out.PerDay[i].Currency < out.PerDay[j].Currency
	})
	return out, nil
}

// Unpaid lists bookings in the period that have ended without payment (no-shows are counted as lost, not owed)
func (s *ReportServiceImpl) Unpaid(ctx context.Context, branchID uint, q dto.ReportQuery) ([]dto.UnpaidRow, error) {
	_, from, to, err := s.period(ctx, branchID, q)
	if err != nil {
		return nil, err
	}
	bookings, err := s.startingIn(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := []dto.UnpaidRow{}
	for _, b := range bookings {
		if b.PaymentStatus == domain.PaymentPaid || b.Status == domain.BookingCancelled ||
			b.Status == domain.BookingNoShow || !b.EndsAt.Before(now) {
			continue
		}
		row := dto.UnpaidRow{
			BookingID: b.ID, Code: b.Code, StartsAt: b.StartsAt, Status: b.Status, Name: bookerName(b),
			Phone: b.GuestPhone, Email: b.GuestEmail, Amount: b.TotalPrice, Currency: b.Currency,
		}
		if b.Customer != nil {
			row.Phone, row.Email = b.Customer.Phone, b.Customer.ContactEmail()
		}
		out = append(out, row)
	}
	return out, nil
}

// CashUp sums the money marked paid on one day, per method and per staff member
func (s *ReportServiceImpl) CashUp(ctx context.Context, branchID uint, date string) (*dto.CashUpDTO, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	loc := locationOf(b)
	day, err := parseDay(date, loc)
	if err != nil {
		return nil, err
	}
	paid, err := s.bookings.PaidBetween(ctx, branchID, day, day.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	totals := map[string]int64{}
	byMethod := map[string]*dto.CashUpMethod{}
	byStaff := map[string]*dto.CashUpStaff{}
	out := &dto.CashUpDTO{Date: date, Payments: []dto.CashUpPayment{}}
	for _, p := range paid {
		totals[p.Currency] += p.TotalPrice
		mk := p.PaymentMethod + "|" + p.Currency
		m, ok := byMethod[mk]
		if !ok {
			m = &dto.CashUpMethod{Method: p.PaymentMethod, Currency: p.Currency}
			byMethod[mk] = m
		}
		m.Count++
		m.Amount += p.TotalPrice

		staffName, staffID := "", uint(0)
		if p.PaidBy != nil {
			staffName, staffID = p.PaidBy.Name, p.PaidBy.ID
		}
		sk := staffName + "|" + p.Currency
		st, ok := byStaff[sk]
		if !ok {
			st = &dto.CashUpStaff{UserID: staffID, Name: staffName, Currency: p.Currency}
			byStaff[sk] = st
		}
		st.Count++
		st.Amount += p.TotalPrice

		out.Payments = append(out.Payments, dto.CashUpPayment{
			Code: p.Code, Name: bookerName(p), Method: p.PaymentMethod, Amount: p.TotalPrice,
			Currency: p.Currency, PaidAt: p.PaidAt.In(loc), PaidBy: staffName,
		})
	}
	out.Totals = moneyList(totals)
	out.ByMethod = make([]dto.CashUpMethod, 0, len(byMethod))
	for _, m := range byMethod {
		out.ByMethod = append(out.ByMethod, *m)
	}
	sort.Slice(out.ByMethod, func(i, j int) bool {
		return out.ByMethod[i].Method+out.ByMethod[i].Currency < out.ByMethod[j].Method+out.ByMethod[j].Currency
	})
	out.ByStaff = make([]dto.CashUpStaff, 0, len(byStaff))
	for _, st := range byStaff {
		out.ByStaff = append(out.ByStaff, *st)
	}
	sort.Slice(out.ByStaff, func(i, j int) bool {
		return out.ByStaff[i].Name+out.ByStaff[i].Currency < out.ByStaff[j].Name+out.ByStaff[j].Currency
	})
	return out, nil
}

// Company adds up the branch reports, so each branch's numbers are exactly those of its own page
func (s *ReportServiceImpl) Company(ctx context.Context, q dto.ReportQuery) (*dto.CompanyReportDTO, error) {
	branches, _, err := s.branches.List(ctx, output.ListQuery{Filters: map[string]any{}})
	if err != nil {
		return nil, err
	}
	out := &dto.CompanyReportDTO{From: q.From, To: q.To, Totals: []dto.FinanceTotals{}, Branches: []dto.BranchReportRow{}}
	sums := map[string]*dto.FinanceTotals{}
	for _, b := range branches {
		finance, err := s.Finance(ctx, b.ID, q)
		if err != nil {
			return nil, err
		}
		counts, err := s.Bookings(ctx, b.ID, q)
		if err != nil {
			return nil, err
		}
		out.Bookings += counts.Total
		out.Branches = append(out.Branches, dto.BranchReportRow{
			BranchID: b.ID, Name: b.Name, Status: b.Status(), Bookings: counts.Total, Totals: finance.Totals,
		})
		for _, t := range finance.Totals {
			sum, ok := sums[t.Currency]
			if !ok {
				sum = &dto.FinanceTotals{Currency: t.Currency}
				sums[t.Currency] = sum
			}
			sum.BookedValue += t.BookedValue
			sum.Paid += t.Paid
			sum.Outstanding += t.Outstanding
			sum.UpcomingUnpaid += t.UpcomingUnpaid
			sum.LostCancelled += t.LostCancelled
			sum.LostNoShow += t.LostNoShow
		}
	}
	for _, t := range sums {
		out.Totals = append(out.Totals, *t)
	}
	sort.Slice(out.Totals, func(i, j int) bool { return out.Totals[i].Currency < out.Totals[j].Currency })
	return out, nil
}
