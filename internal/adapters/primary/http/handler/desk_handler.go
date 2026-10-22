package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/pkg/response"
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

type (
	DeskHandler interface {
		ListBookings(c *fiber.Ctx) error
		GetBooking(c *fiber.Ctx) error
		Availability(c *fiber.Ctx) error
		CreateBooking(c *fiber.Ctx) error
		ChangeStatus(c *fiber.Ctx) error
		SetPayment(c *fiber.Ctx) error
		Calendar(c *fiber.Ctx) error
		Customers(c *fiber.Ctx) error
		FindCustomer(c *fiber.Ctx) error
	}
	deskHandler struct {
		svc input.DeskService
	}

	// ReportHandler serves reports as JSON, or as a CSV download with ?format=csv where a table makes sense
	ReportHandler interface {
		Bookings(c *fiber.Ctx) error
		Utilisation(c *fiber.Ctx) error
		Finance(c *fiber.Ctx) error
		Unpaid(c *fiber.Ctx) error
		CashUp(c *fiber.Ctx) error
	}
	reportHandler struct {
		svc input.ReportService
	}
)

func NewDeskHandler(svc input.DeskService) DeskHandler { return &deskHandler{svc: svc} }

func (h *deskHandler) ListBookings(c *fiber.Ctx) error {
	var q dto.BookingQuery
	if err := c.QueryParser(&q); err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.svc.ListBookings(c.UserContext(), middleware.BranchID(c), q)
	return paged(c, res, err)
}

func (h *deskHandler) GetBooking(c *fiber.Ctx) error { return scopedGet(c, h.svc.GetBooking) }

// Availability is the staff version of the public free times (same query: date + resource / coach)
func (h *deskHandler) Availability(c *fiber.Ctx) error {
	var q dto.AvailabilityQuery
	if err := c.QueryParser(&q); err != nil {
		return fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.svc.DeskAvailability(c.UserContext(), middleware.BranchID(c), q)
	return ok(c, res, err)
}

func (h *deskHandler) CreateBooking(c *fiber.Ctx) error {
	var req dto.StaffBookingRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.CreateForCustomer(c.UserContext(), middleware.BranchID(c), middleware.CurrentUser(c), req)
	return created(c, res, err)
}

func (h *deskHandler) ChangeStatus(c *fiber.Ctx) error { return scopedUpdate(c, h.svc.ChangeStatus) }

func (h *deskHandler) SetPayment(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return fail(c, err)
	}
	var req dto.PaymentRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.svc.SetPayment(c.UserContext(), middleware.BranchID(c), middleware.CurrentUser(c), id, req)
	return ok(c, res, err)
}

// Calendar expects ?date=YYYY-MM-DD and optionally &days=1..7
func (h *deskHandler) Calendar(c *fiber.Ctx) error {
	res, err := h.svc.Calendar(c.UserContext(), middleware.BranchID(c), c.Query("date"), c.QueryInt("days", 1))
	return ok(c, res, err)
}

func (h *deskHandler) Customers(c *fiber.Ctx) error { return scopedList(c, h.svc.Customers) }

// FindCustomer expects ?q= a phone number, customer number (C0042) or email
func (h *deskHandler) FindCustomer(c *fiber.Ctx) error {
	res, err := h.svc.FindCustomer(c.UserContext(), middleware.BranchID(c), c.Query("q"))
	return ok(c, res, err)
}

func NewReportHandler(svc input.ReportService) ReportHandler { return &reportHandler{svc: svc} }

func reportQuery(c *fiber.Ctx) dto.ReportQuery {
	return dto.ReportQuery{From: c.Query("from"), To: c.Query("to")}
}

func wantsCSV(c *fiber.Ctx) bool { return c.Query("format") == "csv" }

// sendCSV writes rows as a downloadable CSV file
func sendCSV(c *fiber.Ctx, filename string, rows [][]string) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		return fail(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func amount(n int64) string { return strconv.FormatInt(n, 10) }

func (h *reportHandler) Bookings(c *fiber.Ctx) error {
	res, err := h.svc.Bookings(c.UserContext(), middleware.BranchID(c), reportQuery(c))
	return ok(c, res, err)
}

func (h *reportHandler) Utilisation(c *fiber.Ctx) error {
	res, err := h.svc.Utilisation(c.UserContext(), middleware.BranchID(c), reportQuery(c))
	if err != nil || !wantsCSV(c) {
		return ok(c, res, err)
	}
	rows := [][]string{{"resource_id", "name", "open_minutes", "booked_minutes", "percent"}}
	for _, r := range res.Resources {
		rows = append(rows, []string{strconv.Itoa(int(r.ResourceID)), r.Name, strconv.Itoa(r.OpenMinutes),
			strconv.Itoa(r.BookedMinutes), strconv.FormatFloat(r.Percent, 'f', 1, 64)})
	}
	return sendCSV(c, "utilisation_"+res.From+"_"+res.To+".csv", rows)
}

func (h *reportHandler) Finance(c *fiber.Ctx) error {
	res, err := h.svc.Finance(c.UserContext(), middleware.BranchID(c), reportQuery(c))
	if err != nil || !wantsCSV(c) {
		return ok(c, res, err)
	}
	rows := [][]string{{"date", "currency", "booked_value", "paid"}}
	for _, d := range res.PerDay {
		rows = append(rows, []string{d.Date, d.Currency, amount(d.BookedValue), amount(d.Paid)})
	}
	rows = append(rows, []string{}, []string{"currency", "booked_value", "paid", "outstanding", "upcoming_unpaid", "lost_cancelled", "lost_no_show"})
	for _, t := range res.Totals {
		rows = append(rows, []string{t.Currency, amount(t.BookedValue), amount(t.Paid), amount(t.Outstanding),
			amount(t.UpcomingUnpaid), amount(t.LostCancelled), amount(t.LostNoShow)})
	}
	return sendCSV(c, "finance_"+res.From+"_"+res.To+".csv", rows)
}

func (h *reportHandler) Unpaid(c *fiber.Ctx) error {
	q := reportQuery(c)
	res, err := h.svc.Unpaid(c.UserContext(), middleware.BranchID(c), q)
	if err != nil || !wantsCSV(c) {
		return ok(c, res, err)
	}
	rows := [][]string{{"code", "starts_at", "status", "name", "phone", "email", "amount", "currency"}}
	for _, r := range res {
		rows = append(rows, []string{r.Code, r.StartsAt.Format(time.RFC3339), r.Status, r.Name, r.Phone, r.Email,
			amount(r.Amount), r.Currency})
	}
	return sendCSV(c, "unpaid_"+q.From+"_"+q.To+".csv", rows)
}

// CashUp expects ?date=YYYY-MM-DD
func (h *reportHandler) CashUp(c *fiber.Ctx) error {
	res, err := h.svc.CashUp(c.UserContext(), middleware.BranchID(c), c.Query("date"))
	if err != nil || !wantsCSV(c) {
		return ok(c, res, err)
	}
	rows := [][]string{{"paid_at", "code", "name", "method", "amount", "currency", "paid_by"}}
	for _, p := range res.Payments {
		rows = append(rows, []string{p.PaidAt.Format(time.RFC3339), p.Code, p.Name, p.Method, amount(p.Amount), p.Currency, p.PaidBy})
	}
	return sendCSV(c, "cash-up_"+res.Date+".csv", rows)
}
