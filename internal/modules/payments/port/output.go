// Package port: the module's two edges. Input is what the module offers; output is what it needs.
package port

import (
	coredomain "backend/internal/core/domain"
	"backend/internal/modules/payments/domain"
	"context"
	"time"
)

// Repository stores the payments
type Repository interface {
	Create(ctx context.Context, p *domain.Payment) error
	Update(ctx context.Context, p *domain.Payment) error
	Get(ctx context.Context, branchID, id uint) (*domain.Payment, error)
	// ByRef finds a payment by the gateway's id; ByOrderNo by the order number we sent
	ByRef(ctx context.Context, provider, ref string) (*domain.Payment, error)
	ByOrderNo(ctx context.Context, provider, orderNo string) (*domain.Payment, error)
	ByBooking(ctx context.Context, branchID, bookingID uint) ([]domain.Payment, error)
	CountByBooking(ctx context.Context, bookingID uint) (int64, error)
	// the same for a customer package's payments
	ByPurchase(ctx context.Context, branchID, purchaseID uint) ([]domain.Payment, error)
	CountByPurchase(ctx context.Context, purchaseID uint) (int64, error)
	// Pending: payments to ask the gateway about (created before `before`): those still waiting whose time ran
	// out after `since`, and refunds in progress
	Pending(ctx context.Context, before, since time.Time) ([]domain.Payment, error)
	// Attention: the branch's payments in these statuses, newest first
	Attention(ctx context.Context, branchID uint, statuses []string) ([]domain.Payment, error)
	// Setting is the branch's choice (nil: never set, online payment on)
	Setting(ctx context.Context, branchID uint) (*domain.Setting, error)
	SaveSetting(ctx context.Context, s *domain.Setting) error
}

// Provider is a payment gateway
type Provider interface {
	// Name is the short id used in addresses and the database ("phajay")
	Name() string
	// Label is shown to people ("PhaJay")
	Label() string
	// Methods lists the ways to pay in this currency (none: the provider cannot take it)
	Methods(currency string) []domain.Method
	Charge(ctx context.Context, r domain.ChargeRequest) (*domain.Issued, error)
	// VerifyWebhook checks the signature over the raw body and reads the event; header gives a request header
	VerifyWebhook(header func(string) string, body []byte) (*domain.Notice, error)
	// Status asks the gateway where a payment stands (the fallback when a webhook never arrives)
	Status(ctx context.Context, ref string) (*domain.Notice, error)
	// Refund asks for the money back; it returns the gateway's id of the refund
	Refund(ctx context.Context, ref string) (string, error)
}

// Purchases is what the module needs from the packages module: a customer package is paid like a booking
type Purchases interface {
	// PayFactsByCustomer: the facts of the customer's own package (ErrNotFound-style error for someone else's)
	PayFactsByCustomer(ctx context.Context, userID, id uint) (*coredomain.PayFacts, error)
	// SettlePaid activates the package once; domain.ErrNotPayable / ErrAlreadyPaid when it cannot take the money
	SettlePaid(ctx context.Context, branchID, id uint, ref string) error
}

// Bookings is what the module needs from the booking service
type Bookings interface {
	PayFactsByToken(ctx context.Context, slug, token string) (*coredomain.PayFacts, error)
	PayFactsByCustomer(ctx context.Context, userID, id uint) (*coredomain.PayFacts, error)
	PayFactsByID(ctx context.Context, branchID, id uint) (*coredomain.PayFacts, error)
	// SetHold: an unpaid event booking is held until the customer's online payment runs out, then released
	SetHold(ctx context.Context, bookingID uint, until time.Time) error
	// SettlePaid records the booking as paid online once; domain.ErrNotPayable / ErrAlreadyPaid when it cannot
	SettlePaid(ctx context.Context, branchID, bookingID uint, ref string) error
	// NotifyRefund emails the booking's customer that a refund was asked (done false) or confirmed (done true)
	NotifyRefund(ctx context.Context, branchID, bookingID uint, amount int64, currency string, done bool)
}
