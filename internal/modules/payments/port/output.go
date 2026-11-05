// Package port: the module's two edges. Input is what the module offers; output is what it needs.
package port

import (
	coredomain "backend/internal/core/domain"
	"backend/internal/modules/payments/domain"
	"context"
	"time"
)

// Repository stores the payments and their charges. A payment comes back with its charges (newest first).
type Repository interface {
	CreatePayment(ctx context.Context, p *domain.Payment) error
	// SavePayment saves the payment itself, not its charges
	SavePayment(ctx context.Context, p *domain.Payment) error
	CreateCharge(ctx context.Context, c *domain.Charge) error
	SaveCharge(ctx context.Context, c *domain.Charge) error
	// SplitOff gives the charge a payment of its own (money that arrived a second time), all or nothing
	SplitOff(ctx context.Context, c *domain.Charge, p *domain.Payment) error
	Get(ctx context.Context, branchID, id uint) (*domain.Payment, error)
	// Find is Get for the module's own work (a webhook knows no branch)
	Find(ctx context.Context, id uint) (*domain.Payment, error)
	// ChargeByRef finds a charge by the gateway's id; ChargeByOrderNo by the order number we sent
	ChargeByRef(ctx context.Context, provider, ref string) (*domain.Charge, error)
	ChargeByOrderNo(ctx context.Context, provider, orderNo string) (*domain.Charge, error)
	ByBooking(ctx context.Context, branchID, bookingID uint) ([]domain.Payment, error)
	CountByBooking(ctx context.Context, bookingID uint) (int64, error)
	// Pending: charges to ask the gateway about (created before `before`): those still waiting whose time
	// ran out after `since`, and the paid charge of a refund in progress
	Pending(ctx context.Context, before, since time.Time) ([]domain.Charge, error)
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

// Bookings is what the module needs from the booking service
type Bookings interface {
	PayFactsByToken(ctx context.Context, slug, token string) (*coredomain.PayFacts, error)
	PayFactsByCustomer(ctx context.Context, userID, id uint) (*coredomain.PayFacts, error)
	PayFactsByID(ctx context.Context, branchID, id uint) (*coredomain.PayFacts, error)
	// SetHold: an unpaid event booking is held until the customer's online payment runs out, then released
	SetHold(ctx context.Context, bookingID uint, until time.Time) error
	// SettlePaid records the booking as paid online once; domain.ErrNotPayable / ErrAlreadyPaid when it cannot
	SettlePaid(ctx context.Context, branchID, bookingID uint, ref string) error
}
