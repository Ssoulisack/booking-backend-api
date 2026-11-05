package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/platform/kit"
	"context"
	"sort"
	"strings"
	"time"
)

var reportStatuses = map[string]bool{
	domain.BookingPending: true, domain.BookingConfirmed: true, domain.BookingCancelled: true,
	domain.BookingCompleted: true, domain.BookingNoShow: true,
}

// dash is what the reports show where a booking has nothing to show
func dash(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return "-"
	}
	return s
}

// rowsFilter checks the query and turns it into a repository filter. The days are optional for the event report;
// the table report starts on today when none is given. A single day gives from = that day.
func (s *ReportServiceImpl) rowsFilter(ctx context.Context, branchID uint, q dto.BookingRowsQuery, event, daysRequired bool) (output.BookingReportFilter, *time.Location, time.Time, error) {
	f := output.BookingReportFilter{Event: event, EventID: q.EventID, Bank: strings.TrimSpace(q.Bank), Keyword: strings.TrimSpace(q.Keyword), PaymentStatus: q.PaymentStatus}
	var first time.Time
	if q.Status != "" && !reportStatuses[q.Status] {
		return f, nil, first, badRequest("INVALID_STATUS")
	}
	f.Status = q.Status
	if q.PaymentStatus != "" && q.PaymentStatus != domain.PaymentUnpaid && q.PaymentStatus != domain.PaymentPaid &&
		q.PaymentStatus != "refunded" && q.PaymentStatus != "partial" {
		return f, nil, first, badRequest("INVALID_PAYMENT_STATUS")
	}
	switch q.SortBy {
	case "", "created_at":
	default:
		return f, nil, first, badRequest("INVALID_SORT")
	}
	switch q.Order {
	case "", "desc":
	case "asc":
		f.Oldest = true
	default:
		return f, nil, first, badRequest("INVALID_SORT")
	}

	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return f, nil, first, err
	}
	loc := locationOf(b)
	from, to := q.From, q.To
	if daysRequired {
		today := s.now().In(loc).Format("2006-01-02")
		if from == "" {
			from = today
		}
		if to == "" {
			to = from
		}
	}
	if from != "" || to != "" {
		if from == "" {
			from = to
		}
		if to == "" {
			to = from
		}
		start, err := parseDay(from, loc)
		if err != nil {
			return f, nil, first, err
		}
		last, err := parseDay(to, loc)
		if err != nil {
			return f, nil, first, err
		}
		end := last.AddDate(0, 0, 1)
		if !end.After(start) {
			return f, nil, first, badRequest("TO_BEFORE_FROM")
		}
		if end.Sub(start) > maxReportDays*24*time.Hour {
			return f, nil, first, badRequest("PERIOD_TOO_LONG")
		}
		f.From, f.To, first = &start, &end, start
	}
	return f, loc, first, nil
}

func customerType(r output.BookingReportRow) string {
	switch r.Booking.CreatedByType {
	case domain.CreatedByStaff:
		if r.MadeByAdmin {
			return "admin"
		}
		return "staff"
	case domain.CreatedByCustomer:
		return "customer"
	}
	return "guest"
}

func tableNumbers(b domain.Booking) string {
	names := []string{}
	for _, it := range b.Items {
		if it.Resource != nil && it.Resource.Kind == domain.ResourceKindTable {
			names = append(names, it.Resource.Name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return naturalLess(names[i], names[j]) })
	return dash(strings.Join(names, ", "))
}

func tableRow(index int, r output.BookingReportRow) dto.TableReportRow {
	b := r.Booking
	name, phone, email := b.GuestName, b.GuestPhone, b.GuestEmail
	if b.Customer != nil {
		name, phone, email = b.Customer.Name, b.Customer.Phone, b.Customer.ContactEmail()
	}
	return dto.TableReportRow{
		Index: index, TableNumber: tableNumbers(b), BookCode: b.Code, Fullname: dash(name), Phone: dash(phone),
		Email: dash(email), CustomerType: customerType(r), BookTime: b.CreatedAt, Status: b.Status,
	}
}

func (s *ReportServiceImpl) TableBookings(ctx context.Context, branchID uint, q dto.BookingRowsQuery) (*dto.TableReportDTO, error) {
	f, loc, first, err := s.rowsFilter(ctx, branchID, q, false, true)
	if err != nil {
		return nil, err
	}
	page, limit := kit.NormalizePage(dto.ListQuery{Page: q.Page, Limit: q.Limit})
	rows, total, err := s.rows.Rows(ctx, branchID, f, page, limit)
	if err != nil {
		return nil, err
	}
	all, taken, err := s.rows.TablesOnDay(ctx, branchID, first, first.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	out := &dto.TableReportDTO{
		Cards: dto.TableCards{Date: first.In(loc).Format("2006-01-02"), TotalTables: all, TableTaken: min(taken, all), Available: max(all-taken, 0)},
		Items: make([]dto.TableReportRow, len(rows)), Meta: kit.NewMeta(total, page, limit),
	}
	for i, r := range rows {
		out.Items[i] = tableRow((page-1)*limit+i+1, r)
	}
	return out, nil
}

func (s *ReportServiceImpl) EventBookings(ctx context.Context, branchID uint, q dto.BookingRowsQuery) (*dto.EventReportDTO, error) {
	f, _, _, err := s.rowsFilter(ctx, branchID, q, true, false)
	if err != nil {
		return nil, err
	}
	page, limit := kit.NormalizePage(dto.ListQuery{Page: q.Page, Limit: q.Limit})
	rows, total, err := s.rows.Rows(ctx, branchID, f, page, limit)
	if err != nil {
		return nil, err
	}
	amounts, err := s.rows.Amounts(ctx, branchID, f)
	if err != nil {
		return nil, err
	}
	out := &dto.EventReportDTO{
		Totals: make([]dto.EventTotal, len(amounts)), Items: make([]dto.EventReportRow, len(rows)),
		Meta: kit.NewMeta(total, page, limit),
	}
	for i, a := range amounts {
		out.Totals[i] = dto.EventTotal{Currency: a.Currency, SuccessAmount: a.Success, CancelledAmount: a.Cancelled}
	}
	for i, r := range rows {
		out.Items[i] = dto.EventReportRow{
			TableReportRow: tableRow((page-1)*limit+i+1, r), PaymentStatus: dash(r.Booking.PaymentStatus),
			Bank: dash(r.Bank), Amount: r.Booking.TotalPrice, Currency: r.Booking.Currency,
		}
	}
	return out, nil
}

// naturalLess orders table names the way people do: "T2" before "T10"
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digitsLen(a), digitsLen(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digitsLen(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}
