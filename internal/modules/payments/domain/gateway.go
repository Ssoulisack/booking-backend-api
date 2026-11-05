package domain

import (
	"errors"
	"time"
)

// State is what a gateway says about a payment, in the module's own words
type State string

const (
	Waiting  State = "waiting"
	Paid     State = "paid"
	Expired  State = "expired"
	Refunded State = "refunded"
	Disputed State = "disputed"
)

// Method is one way to pay through a provider (a bank's QR, a hosted page…)
type Method struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ChargeRequest asks a provider to create a payment
type ChargeRequest struct {
	OrderNo     string // unique per payment attempt
	Amount      int64
	Currency    string
	Description string // short, plain ASCII: some banks refuse anything else
	Method      string
	Tags        [3]string // our own references, returned in the webhook
}

// Charge is what the customer needs to pay: a QR text, a bank-app link, or a page to open
type Charge struct {
	ProviderRef string // the gateway's id of the payment ("" until known, e.g. a payment page)
	QRCode      string
	Deeplink    string
	RedirectURL string
}

// Notice is what a webhook or a status check says about one payment
type Notice struct {
	ProviderRef string
	OrderNo     string
	State       State
	Amount      int64
	Method      string
	PaidAt      time.Time
}

var (
	// ErrBadSignature: the webhook is not from the gateway (or the key is wrong)
	ErrBadSignature = errors.New("payments: bad webhook signature")
	// ErrNotFound: the gateway does not know that payment
	ErrNotFound = errors.New("payments: payment not found at the gateway")
	// ErrRateLimited: the gateway refuses more payments for now (a daily limit on a test account)
	ErrRateLimited = errors.New("payments: the gateway's limit is reached")
	// ErrUnsupported: this provider cannot do that
	ErrUnsupported = errors.New("payments: not supported by this provider")
)
