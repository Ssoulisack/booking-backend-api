package dto

import (
	"backend/internal/modules/payments/domain"
	"time"
)

// StartRequest: which gateway and which way to pay
type StartRequest struct {
	Provider string `json:"provider"`
	Method   string `json:"method"`
}

// OptionDTO: a gateway the customer can pay through, with its ways to pay
type OptionDTO struct {
	Provider string          `json:"provider"`
	Label    string          `json:"label"`
	Methods  []domain.Method `json:"methods"`
}

// PaymentDTO is a payment as the customer sees it: what to scan or open, and where it stands
type PaymentDTO struct {
	ID          uint      `json:"id"`
	Provider    string    `json:"provider"`
	Method      string    `json:"method"`
	Status      string    `json:"status"`
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	QRCode      string    `json:"qr_code,omitempty"`
	Deeplink    string    `json:"deeplink,omitempty"`
	RedirectURL string    `json:"redirect_url,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// StaffPaymentDTO adds what the team needs to follow the money: every try is its own payment
type StaffPaymentDTO struct {
	PaymentDTO
	BookingID   uint       `json:"booking_id"`
	OrderNo     string     `json:"order_no"`
	ProviderRef string     `json:"provider_ref"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	RefundRef   string     `json:"refund_ref,omitempty"`
	Note        string     `json:"note,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// SettingsDTO: the branch's online payment switch, and whether the server has a gateway to switch on
type SettingsDTO struct {
	OnlineEnabled    bool `json:"online_enabled"`
	GatewayAvailable bool `json:"gateway_available"`
	// ChargeMinutes is how long a customer has to pay once they chose a bank
	ChargeMinutes int `json:"charge_minutes"`
}

// SettingsRequest turns online payment on or off for the branch
type SettingsRequest struct {
	OnlineEnabled bool `json:"online_enabled"`
	// ChargeMinutes: 5 to 30; not sent (0) = the default, 30
	ChargeMinutes int `json:"charge_minutes"`
}
