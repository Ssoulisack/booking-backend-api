package port

import (
	"backend/internal/modules/payments/dto"
	"context"
)

// Service is what the module offers: the handlers call it, and the job calls Poll
type Service interface {
	// customers
	Options(ctx context.Context, slug string) ([]dto.OptionDTO, error)
	StartByToken(ctx context.Context, slug, token string, req dto.StartRequest) (*dto.PaymentDTO, error)
	StartByCustomer(ctx context.Context, userID, id uint, req dto.StartRequest) (*dto.PaymentDTO, error)
	// the payment a customer can carry on with after leaving the page (nil: none)
	OpenByToken(ctx context.Context, slug, token string) (*dto.PaymentDTO, error)
	OpenByCustomer(ctx context.Context, userID, id uint) (*dto.PaymentDTO, error)
	CheckByToken(ctx context.Context, slug, token string, id uint) (*dto.PaymentDTO, error)
	CheckByCustomer(ctx context.Context, userID, bookingID, id uint) (*dto.PaymentDTO, error)
	// the gateway's callback: ErrBadSignature when it is not from the gateway
	HandleWebhook(ctx context.Context, provider string, header func(string) string, body []byte) error
	// payments still waiting are asked about at the gateway (the fallback for a webhook that never came)
	Poll(ctx context.Context) error
	// the team
	ForBooking(ctx context.Context, branchID, bookingID uint) ([]dto.StaffPaymentDTO, error)
	Attention(ctx context.Context, branchID uint) ([]dto.StaffPaymentDTO, error)
	Refund(ctx context.Context, branchID, id uint) (*dto.StaffPaymentDTO, error)
	// whether the branch takes online payment (and whether the server has a gateway at all)
	Settings(ctx context.Context, branchID uint) (*dto.SettingsDTO, error)
	SaveSettings(ctx context.Context, branchID uint, req dto.SettingsRequest) (*dto.SettingsDTO, error)
}
