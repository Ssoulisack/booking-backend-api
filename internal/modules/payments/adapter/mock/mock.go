// Package mock: a stand-in for the PhaJay gateway that lives inside the backend, used when server.env is
// "dev". No network call is ever made and no key is needed. It speaks the same language as the real
// adapter (same provider name, same banks), so the pages, the database and the API behave the same; a
// person (or a test) settles a payment by pressing Pay on the dev page (see dev.go), which reaches the
// service the way a webhook does.
//
// State is kept in memory: after a restart the mock no longer knows the QR codes it made.
package mock

import (
	"backend/internal/modules/payments/adapter/phajay"
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/port"
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"
)

// Config is the mock's setup
type Config struct {
	// BaseURL is where this backend can be reached from a browser: the hosted payment page lives there
	BaseURL string
	// DailyLimit refuses more than this many payments a day with the gateway's "limit reached" answer, to
	// try that message (the real test account allows 20); 0 = no limit
	DailyLimit int
}

// Tx is one transaction the mock made
type Tx struct {
	ID      string       `json:"id"`
	Order   string       `json:"order_no"`
	Method  string       `json:"method"`
	Amount  int64        `json:"amount"`
	State   domain.State `json:"state"`
	Created time.Time    `json:"created_at"`
}

// Mock is the dev gateway
type Mock struct {
	cfg  Config
	now  func() time.Time
	mu   sync.Mutex
	next int
	refs int
	txs  map[string]*Tx
	day  string
	made int
}

var _ port.Provider = (*Mock)(nil)

// New builds the mock
func New(cfg Config) *Mock { return &Mock{cfg: cfg, now: time.Now, txs: map[string]*Tx{}} }

func (m *Mock) Name() string  { return "phajay" }
func (m *Mock) Label() string { return "PhaJay (mock)" }

func (m *Mock) Methods(currency string) []domain.Method { return phajay.Methods(currency) }

func (m *Mock) Charge(_ context.Context, r domain.ChargeRequest) (*domain.Issued, error) {
	if r.Currency != "LAK" {
		return nil, domain.ErrUnsupported
	}
	if r.Method != "link" && !slices.ContainsFunc(m.Methods("LAK"), func(x domain.Method) bool { return x.Key == r.Method }) {
		return nil, domain.ErrUnsupported
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if today := m.now().Format("2006-01-02"); today != m.day {
		m.day, m.made = today, 0
	}
	if m.cfg.DailyLimit > 0 && m.made >= m.cfg.DailyLimit {
		return nil, fmt.Errorf("%w: mock: max %d transactions/day", domain.ErrRateLimited, m.cfg.DailyLimit)
	}
	m.made++
	m.next++
	t := &Tx{ID: fmt.Sprintf("MOCK%04d", m.next), Order: r.OrderNo, Method: r.Method, Amount: r.Amount, State: domain.Waiting, Created: m.now()}
	m.txs[t.ID] = t
	if r.Method == "link" {
		// like the real payment page: its transaction id is told to the backend only when it is paid
		return &domain.Issued{RedirectURL: m.cfg.BaseURL + "/api/v1/dev/payments/page/" + r.OrderNo}, nil
	}
	return &domain.Issued{ProviderRef: t.ID, QRCode: "MOCK-QR-" + t.ID, Deeplink: "mockbank://pay/" + t.ID}, nil
}

// VerifyWebhook: nobody sends the mock a webhook; every one is refused
func (m *Mock) VerifyWebhook(func(string) string, []byte) (*domain.Notice, error) {
	return nil, domain.ErrBadSignature
}

func (m *Mock) Status(_ context.Context, ref string) (*domain.Notice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.txs[ref]
	if t == nil {
		return nil, domain.ErrNotFound
	}
	return notice(t, m.now()), nil
}

func (m *Mock) Refund(_ context.Context, ref string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.txs[ref]
	if t == nil || t.State != domain.Paid {
		return "", fmt.Errorf("mock: %s cannot be refunded", ref)
	}
	t.State = domain.Refunded
	m.refs++
	return fmt.Sprint(9000 + m.refs), nil
}

func notice(t *Tx, at time.Time) *domain.Notice {
	return &domain.Notice{ProviderRef: t.ID, OrderNo: t.Order, State: t.State, Amount: t.Amount, Method: t.Method, PaidAt: at}
}

// Settle moves a waiting transaction to paid or expired and gives back the notice the backend is told
// (what a webhook would carry); an unknown or already settled one is an error
func (m *Mock) Settle(id string, to domain.State) (*domain.Notice, error) {
	if to != domain.Paid && to != domain.Expired {
		return nil, fmt.Errorf("mock: a payment can only be paid or expired")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.txs[id]
	if t == nil {
		return nil, domain.ErrNotFound
	}
	if t.State != domain.Waiting {
		return nil, fmt.Errorf("mock: %s is %s already", id, t.State)
	}
	t.State = to
	return notice(t, m.now()), nil
}

// List gives every transaction, newest first
func (m *Mock) List() []Tx {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Tx, 0, len(m.txs))
	for _, t := range m.txs {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// IDOfOrder finds the transaction made for an order number
func (m *Mock) IDOfOrder(order string) string {
	for _, t := range m.List() {
		if t.Order == order {
			return t.ID
		}
	}
	return ""
}
