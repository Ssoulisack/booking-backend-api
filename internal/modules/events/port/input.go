package port

import (
	"backend/internal/core/domain"
	"backend/internal/modules/events/dto"
	"context"
)

// Service is what the module offers: the handlers call it, and so do the booking service (EventOn)
// and the job (ExpireUnpaid)
type Service interface {
	// staff
	List(ctx context.Context, branchID uint) ([]dto.EventDTO, error)
	Get(ctx context.Context, branchID, id uint) (*dto.EventDTO, error)
	Create(ctx context.Context, branchID uint, req dto.EventRequest) (*dto.EventDTO, error)
	Update(ctx context.Context, branchID, id uint, req dto.EventRequest) (*dto.EventDTO, error)
	Delete(ctx context.Context, branchID, id uint) error
	Copy(ctx context.Context, branchID, id uint, req dto.CopyRequest) (*dto.EventDTO, error)
	Prices(ctx context.Context, branchID, id uint) (*dto.PricesDTO, error)
	SetPrices(ctx context.Context, branchID, id uint, req dto.PricesRequest) (*dto.PricesDTO, error)
	Pay(ctx context.Context, branchID uint) (*dto.PayInfoDTO, error)
	SavePay(ctx context.Context, branchID uint, req dto.PayInfoRequest) (*dto.PayInfoDTO, error)

	// customers
	Upcoming(ctx context.Context, slug string) ([]dto.PublicEventDTO, error)
	// PublicPay is the branch's own way to pay (QR, account, WhatsApp) for customers
	PublicPay(ctx context.Context, slug string) (*dto.PublicPayInfoDTO, error)
	Detail(ctx context.Context, slug string, id uint, date string) (*dto.PublicEventDetailDTO, error)
	Book(ctx context.Context, slug string, id uint, actor *domain.User, req dto.BookRequest) (*dto.BookedDTO, error)

	// the booking service asks whether an event takes a day over, and the job releases unpaid holds
	EventOn(ctx context.Context, branchID uint, day string) (*domain.EventDay, error)
	ExpireUnpaid(ctx context.Context) error
}
