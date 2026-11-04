// Package phajay: the PhaJay gateway (https://payment-doc.phajay.co/v1, see docs/phajay-guide.md) as an
// adapter of the payments module's provider port.
package phajay

import (
	"backend/internal/modules/payments/domain"
	"backend/internal/modules/payments/port"
	"backend/internal/platform/config"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// PhaJay talks to https://payment-doc.phajay.co/v1 (see docs/phajay-guide.md). Calls happen on the
// server only: the key never reaches a browser. Note the gateway's two auth styles: QR and refund
// take the raw key in a "secretKey" header; the payment link and status take Basic auth.
// PhaJay is the gateway adapter (it implements port.Provider)
type PhaJay struct {
	baseURL     string
	callKey     string   // calls: the production key, else the test key (before KYC)
	webhookKeys []string // signatures: the dev key (before KYC) and the production key
	http        *http.Client
	loc         *time.Location
	banks       []domain.Method
	banksByKey  map[string]bool
	// divisor > 1: a test account (before KYC) takes 1 to 999 kip, so amounts are sent divided and read back multiplied
	divisor float64
}

// New returns nil when no key is set: the gateway is off
func New(cfg config.PhaJayConfig) *PhaJay {
	call := cfg.SecretKey
	if call == "" {
		call = cfg.TestKey
	}
	if call == "" {
		return nil
	}
	var keys []string
	for _, k := range []string{cfg.DevSecretKey, cfg.SecretKey} {
		if k != "" {
			keys = append(keys, k)
		}
	}
	loc, err := time.LoadLocation("Asia/Vientiane")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	p := &PhaJay{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), callKey: call, webhookKeys: keys,
		http: &http.Client{Timeout: 12 * time.Second}, loc: loc, banksByKey: map[string]bool{},
		banks: []domain.Method{
			{Key: "bcel", Label: "BCEL One"}, {Key: "jdb", Label: "JDB"}, {Key: "ldb", Label: "LDB"}, {Key: "ib", Label: "Indochina Bank"},
			{Key: "stb", Label: "STB"}, {Key: "m-money", Label: "M MoneyX"}, {Key: "apb", Label: "APB"},
		},
	}
	if p.baseURL == "" {
		p.baseURL = "https://payment-gateway.phajay.co"
	}
	for _, b := range p.banks {
		p.banksByKey[b.Key] = true
	}
	// never with the production key: real money is always the real amount
	if cfg.SecretKey == "" && cfg.TestAmountDivisor > 1 {
		p.divisor = float64(cfg.TestAmountDivisor)
	}
	return p
}

// TestScale reports the test amount divisor in force (0: real amounts), for the start-up log
func (p *PhaJay) TestScale() int { return int(p.divisor) }

// toGateway is the amount PhaJay is asked for: the real amount, or (test mode) the amount divided,
// at least 1 and with at most two decimals
func (p *PhaJay) toGateway(amount int64) any {
	if p.divisor == 0 {
		return amount
	}
	v := math.Round(float64(amount)/p.divisor*100) / 100
	return math.Max(v, 1)
}

// fromGateway is the real amount behind an amount PhaJay reports
func (p *PhaJay) fromGateway(f float64) int64 {
	if p.divisor == 0 {
		return kip(f)
	}
	return kip(f * p.divisor)
}

// KeyInfo describes the loaded keys for the start-up log: lengths only, never the keys. A key that
// contains "$" may be cut short by a shell or an .env file; a wrong length shows it at once.
func KeyInfo(cfg config.PhaJayConfig) string {
	return fmt.Sprintf("test key %d chars, dev secret %d chars, secret %d chars",
		len(cfg.TestKey), len(cfg.DevSecretKey), len(cfg.SecretKey))
}

func (p *PhaJay) Name() string  { return "phajay" }
func (p *PhaJay) Label() string { return "PhaJay" }

// Methods: every bank's QR and the hosted payment page; the gateway takes Lao kip
func (p *PhaJay) Methods(currency string) []domain.Method {
	if currency != "LAK" {
		return nil
	}
	return append(append([]domain.Method{}, p.banks...), domain.Method{Key: "link", Label: "Choose a bank on the payment page"})
}

// plain keeps the description short and ASCII: BCEL refuses Lao and Thai letters, APB allows 24 characters
func plain(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if len(out) > 24 {
		out = strings.TrimSpace(out[:24])
	}
	if out == "" {
		out = "Booking"
	}
	return out
}

// do sends a request and reads the answer (at most 1 MB) into out
func (p *PhaJay) do(ctx context.Context, method, path string, basic bool, body any, out any) error {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if basic {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.callKey)))
	} else {
		req.Header.Set("secretKey", p.callKey)
	}
	res, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil && res.StatusCode < 300 {
		return fmt.Errorf("phajay: unreadable answer: %w", err)
	}
	if res.StatusCode >= 300 {
		var msg struct{ Message string }
		_ = json.Unmarshal(raw, &msg)
		if msg.Message == "PAYMENT_NOT_FOUND" {
			return domain.ErrNotFound
		}
		// the gateway explains a refusal in its body (never the key): keep a short copy for the log
		return fmt.Errorf("phajay: %s %s: %d %s %s", method, path, res.StatusCode, msg.Message, bodyHint(raw))
	}
	return nil
}

func (p *PhaJay) Charge(ctx context.Context, r domain.ChargeRequest) (*domain.Charge, error) {
	if r.Currency != "LAK" {
		return nil, domain.ErrUnsupported
	}
	body := map[string]any{
		"amount": p.toGateway(r.Amount), "description": plain(r.Description), "orderNo": r.OrderNo,
		"tag1": r.Tags[0], "tag2": r.Tags[1], "tag3": r.Tags[2],
	}
	if r.Method == "link" {
		var res struct {
			Message     string `json:"message"`
			RedirectURL string `json:"redirectURL"`
		}
		if err := p.do(ctx, http.MethodPost, "/v1/api/link/payment-link", true, body, &res); err != nil {
			return nil, err
		}
		if res.RedirectURL == "" {
			return nil, fmt.Errorf("phajay: no payment page (%s)", res.Message)
		}
		// the page's own transaction id arrives with the webhook; until then the order number identifies it
		return &domain.Charge{RedirectURL: res.RedirectURL}, nil
	}
	if !p.banksByKey[r.Method] {
		return nil, domain.ErrUnsupported
	}
	var res struct {
		Message       string `json:"message"`
		TransactionID string `json:"transactionId"`
		QRCode        string `json:"qrCode"`
		Link          string `json:"link"`
	}
	if err := p.do(ctx, http.MethodPost, "/v1/api/payment/generate-"+r.Method+"-qr", false, body, &res); err != nil {
		return nil, err
	}
	if res.TransactionID == "" || res.QRCode == "" {
		return nil, fmt.Errorf("phajay: no QR (%s)", res.Message)
	}
	c := &domain.Charge{ProviderRef: res.TransactionID, QRCode: res.QRCode}
	if r.Method != "ib" { // Indochina Bank's link does not open the app: show the QR
		c.Deeplink = res.Link
	}
	return c, nil
}

// stateOf maps the gateway's status words
func stateOf(status string, isPaid *bool) domain.State {
	switch strings.ToUpper(status) {
	case "PAYMENT_COMPLETED":
		if isPaid == nil || *isPaid { // the status call also says isPaid: both must hold
			return domain.Paid
		}
		return domain.Waiting
	case "EXPIRED":
		return domain.Expired
	case "REFUNDED":
		return domain.Refunded
	case "DISPUTED":
		return domain.Disputed
	default:
		return domain.Waiting
	}
}

// kip turns the gateway's amount (a number, sometimes with decimals) into whole kip
func kip(f float64) int64 { return int64(math.Round(f)) }

func (p *PhaJay) VerifyWebhook(header func(string) string, body []byte) (*domain.Notice, error) {
	got := header("X-Signature")
	ok := false
	for _, key := range p.webhookKeys {
		m := hmac.New(sha256.New, []byte(key))
		m.Write(body) // the raw bytes, exactly as received
		want := base64.StdEncoding.EncodeToString(m.Sum(nil))
		if hmac.Equal([]byte(want), []byte(got)) {
			ok = true
		}
	}
	if !ok {
		return nil, domain.ErrBadSignature
	}
	var e struct {
		Message       string  `json:"message"`
		Status        string  `json:"status"`
		TransactionID string  `json:"transactionId"`
		OrderNo       string  `json:"orderNo"`
		TxnAmount     float64 `json:"txnAmount"`
		PaymentMethod string  `json:"paymentMethod"`
		TxnDateTime   string  `json:"txnDateTime"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, fmt.Errorf("phajay: unreadable webhook: %w", err)
	}
	status := e.Status
	if status == "" {
		status = e.Message
	}
	n := &domain.Notice{ProviderRef: e.TransactionID, OrderNo: e.OrderNo, State: stateOf(status, nil), Amount: p.fromGateway(e.TxnAmount), Method: e.PaymentMethod}
	if t, err := time.ParseInLocation("02/01/2006 15:04:05", e.TxnDateTime, p.loc); err == nil {
		n.PaidAt = t
	}
	return n, nil
}

func (p *PhaJay) Status(ctx context.Context, ref string) (*domain.Notice, error) {
	var res struct {
		Data struct {
			TransactionID string  `json:"transactionId"`
			OrderNo       string  `json:"orderNo"`
			Status        string  `json:"status"`
			IsPaid        bool    `json:"isPaid"`
			Amount        float64 `json:"amount"`
			PaymentMethod string  `json:"paymentMethod"`
		} `json:"data"`
	}
	if err := p.do(ctx, http.MethodGet, "/v1/api/payment/check-transaction/status/"+ref, true, nil, &res); err != nil {
		return nil, err
	}
	d := res.Data
	paid := d.IsPaid
	return &domain.Notice{
		ProviderRef: d.TransactionID, OrderNo: d.OrderNo, State: stateOf(d.Status, &paid),
		Amount: p.fromGateway(d.Amount), Method: d.PaymentMethod,
	}, nil
}

func (p *PhaJay) Refund(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("phajay: no transaction to refund")
	}
	var res struct {
		Data struct {
			ID     any    `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := p.do(ctx, http.MethodPost, "/v1/api/refund", false, map[string]string{"transactionId": ref}, &res); err != nil {
		return "", err
	}
	return fmt.Sprint(res.Data.ID), nil
}

var _ port.Provider = (*PhaJay)(nil)

// bodyHint is the start of the gateway's answer, one line, for an error message
func bodyHint(raw []byte) string {
	s := strings.Join(strings.Fields(string(raw)), " ")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
