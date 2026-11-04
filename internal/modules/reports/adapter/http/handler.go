// Package http: the reports module's driving adapter. Reports are JSON, or a CSV download with
// ?format=csv where a table makes sense.
package http

import (
	"backend/internal/core/dto"
	"backend/internal/modules/reports/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/response"
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the report routes
type Handler struct{ svc port.ReportService }

// New builds the handler
func New(svc port.ReportService) *Handler { return &Handler{svc: svc} }

func reportQuery(c *fiber.Ctx) dto.ReportQuery {
	return dto.ReportQuery{From: c.Query("from"), To: c.Query("to")}
}

func wantsCSV(c *fiber.Ctx) bool { return c.Query("format") == "csv" }

// sendCSV writes rows as a downloadable CSV file
func sendCSV(c *fiber.Ctx, filename string, rows [][]string) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		return httpx.Fail(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func amount(n int64) string { return strconv.FormatInt(n, 10) }

func (h *Handler) Bookings(c *fiber.Ctx) error {
	res, err := h.svc.Bookings(c.UserContext(), middleware.BranchID(c), reportQuery(c))
	return httpx.OK(c, res, err)
}

func (h *Handler) Utilisation(c *fiber.Ctx) error {
	res, err := h.svc.Utilisation(c.UserContext(), middleware.BranchID(c), reportQuery(c))
	if err != nil || !wantsCSV(c) {
		return httpx.OK(c, res, err)
	}
	rows := [][]string{{"resource_id", "name", "open_minutes", "booked_minutes", "percent"}}
	for _, r := range res.Resources {
		rows = append(rows, []string{strconv.Itoa(int(r.ResourceID)), r.Name, strconv.Itoa(r.OpenMinutes),
			strconv.Itoa(r.BookedMinutes), strconv.FormatFloat(r.Percent, 'f', 1, 64)})
	}
	return sendCSV(c, "utilisation_"+res.From+"_"+res.To+".csv", rows)
}

func (h *Handler) Finance(c *fiber.Ctx) error {
	res, err := h.svc.Finance(c.UserContext(), middleware.BranchID(c), reportQuery(c))
	if err != nil || !wantsCSV(c) {
		return httpx.OK(c, res, err)
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

func (h *Handler) Unpaid(c *fiber.Ctx) error {
	q := reportQuery(c)
	res, err := h.svc.Unpaid(c.UserContext(), middleware.BranchID(c), q)
	if err != nil || !wantsCSV(c) {
		return httpx.OK(c, res, err)
	}
	rows := [][]string{{"code", "starts_at", "status", "name", "phone", "email", "amount", "currency"}}
	for _, r := range res {
		rows = append(rows, []string{r.Code, r.StartsAt.Format(time.RFC3339), r.Status, r.Name, r.Phone, r.Email,
			amount(r.Amount), r.Currency})
	}
	return sendCSV(c, "unpaid_"+q.From+"_"+q.To+".csv", rows)
}

// CashUp expects ?date=YYYY-MM-DD
func (h *Handler) CashUp(c *fiber.Ctx) error {
	res, err := h.svc.CashUp(c.UserContext(), middleware.BranchID(c), c.Query("date"))
	if err != nil || !wantsCSV(c) {
		return httpx.OK(c, res, err)
	}
	rows := [][]string{{"paid_at", "code", "name", "method", "amount", "currency", "paid_by"}}
	for _, p := range res.Payments {
		rows = append(rows, []string{p.PaidAt.Format(time.RFC3339), p.Code, p.Name, p.Method, amount(p.Amount), p.Currency, p.PaidBy})
	}
	return sendCSV(c, "cash-up_"+res.Date+".csv", rows)
}

// Company adds up every branch: ?from=&to= (YYYY-MM-DD)
func (h *Handler) Company(c *fiber.Ctx) error {
	var q dto.ReportQuery
	if err := c.QueryParser(&q); err != nil {
		return httpx.Fail(c, response.ErrorBadRequest("INVALID_QUERY"))
	}
	res, err := h.svc.Company(c.UserContext(), q)
	return httpx.OK(c, res, err)
}
