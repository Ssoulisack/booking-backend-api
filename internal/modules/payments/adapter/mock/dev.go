package mock

import (
	"backend/internal/modules/payments/domain"
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Notifier is how a settled payment reaches the payments service, as a webhook would
type Notifier interface {
	Notify(ctx context.Context, provider string, n *domain.Notice) error
}

// Routes are the dev tools; they exist only while the mock does (server.env: dev):
//
//	GET  /dev/payments               every transaction, with Pay / Expire (JSON with ?json=1)
//	POST /dev/payments/:id/pay       the customer paid it   (?json=1 answers JSON instead of going back to the list)
//	POST /dev/payments/:id/expire    its time ran out
//	GET  /dev/payments/page/:order   the "payment page" a customer gets for the "choose a bank" way
func (m *Mock) Routes(api fiber.Router, svc Notifier) {
	g := api.Group("/dev/payments")
	g.Get("/", m.list)
	g.Post("/:id/pay", func(c *fiber.Ctx) error { return m.settle(c, svc, domain.Paid) })
	g.Post("/:id/expire", func(c *fiber.Ctx) error { return m.settle(c, svc, domain.Expired) })
	g.Get("/page/:order", m.page)
}

// listPath is this page's own address (the api prefix is the app's choice)
func listPath(c *fiber.Ctx) string {
	p := c.Path()
	if i := strings.Index(p, "/dev/payments"); i >= 0 {
		return p[:i] + "/dev/payments"
	}
	return p
}

func (m *Mock) settle(c *fiber.Ctx, svc Notifier, to domain.State) error {
	id := c.Params("id")
	n, err := m.Settle(id, to)
	if err != nil {
		return fiber.NewError(http.StatusNotFound, err.Error())
	}
	if err := svc.Notify(c.UserContext(), m.Name(), n); err != nil {
		return fiber.NewError(http.StatusInternalServerError, err.Error())
	}
	if c.Query("json") != "" {
		return c.JSON(fiber.Map{"id": id, "state": to})
	}
	return c.Redirect(listPath(c), http.StatusSeeOther)
}

func (m *Mock) list(c *fiber.Ctx) error {
	txs := m.List()
	if c.Query("json") != "" {
		return c.JSON(fiber.Map{"payments": txs})
	}
	base := listPath(c)
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><meta charset=utf-8><meta name=viewport content="width=device-width,initial-scale=1"><title>Mock PhaJay</title>
<style>body{font:15px system-ui;margin:24px;max-width:900px}table{border-collapse:collapse;width:100%%}td,th{padding:6px 10px;border-bottom:1px solid #ddd;text-align:left}
button{padding:4px 12px;margin-right:4px}.waiting{color:#b45309}.paid{color:#15803d}.expired{color:#6b7280}.refunded{color:#1d4ed8}</style>
<h1>Mock PhaJay <small>(server.env: dev, PhaJay is never called)</small></h1><p><a href="%s">Refresh</a></p>
<table><tr><th>Id<th>Order<th>Way<th>Amount<th>State<th></tr>`, base)
	for _, t := range txs {
		fmt.Fprintf(&b, `<tr><td>%s<td>%s<td>%s<td>%d<td class=%s>%s<td>`, t.ID, html.EscapeString(t.Order), html.EscapeString(t.Method), t.Amount, t.State, t.State)
		if t.State == domain.Waiting {
			fmt.Fprintf(&b, `<form method=post action="%s/%s/pay" style=display:inline><button>Pay</button></form>
<form method=post action="%s/%s/expire" style=display:inline><button>Expire</button></form>`, base, t.ID, base, t.ID)
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</table>")
	c.Type("html", "utf-8")
	return c.SendString(b.String())
}

// page is what the customer sees after "Choose a bank on the payment page": one Pay button
func (m *Mock) page(c *fiber.Ctx) error {
	order := c.Params("order")
	id := m.IDOfOrder(order)
	base := listPath(c)
	c.Type("html", "utf-8")
	return c.SendString(fmt.Sprintf(`<!doctype html><meta charset=utf-8><meta name=viewport content="width=device-width,initial-scale=1"><title>Mock payment page</title>
<body style="font:16px system-ui;margin:40px"><h2>Mock PhaJay payment page</h2><p>Order %s</p>
<form method=post action="%s/%s/pay"><button style="padding:10px 24px">Pay</button></form>`,
		html.EscapeString(order), base, html.EscapeString(id)))
}
