package port

import (
	"backend/internal/core/domain"
	coredto "backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/modules/packages/dto"
	"context"
	"time"
)

// Service is what the module offers
type Service interface {
	// the booking service pays bookings with a customer's package through this
	output.PackageUse
	// and the package sales the reports and the cash-up read
	output.PackageSales

	// staff
	List(ctx context.Context, branchID uint) ([]dto.PackageDTO, error)
	Get(ctx context.Context, branchID, id uint) (*dto.PackageDTO, error)
	Create(ctx context.Context, branchID uint, req dto.PackageRequest) (*dto.PackageDTO, error)
	Update(ctx context.Context, branchID, id uint, req dto.PackageRequest) (*dto.PackageDTO, error)
	Delete(ctx context.Context, branchID, id uint) error

	// customers: the active packages with their active options (a package with none is left out)
	Public(ctx context.Context, slug string) ([]dto.PackageDTO, error)

	// customers: buy a package (it waits for payment), see theirs, drop one that is not paid yet
	Buy(ctx context.Context, slug string, user *domain.User, packageID uint, req dto.BuyRequest) (*dto.CustomerPackageDTO, error)
	Mine(ctx context.Context, userID uint) ([]dto.CustomerPackageDTO, error)
	CancelMine(ctx context.Context, userID, id uint) (*dto.CustomerPackageDTO, error)
	// SlipSent: "I paid and sent the slip": the package keeps waiting, the team checks it and marks it paid
	SlipSent(ctx context.Context, userID, id uint, slip dto.SlipUpload) (*dto.CustomerPackageDTO, error)

	// staff: packages sold at a branch; sell at the desk; mark paid (cash, transfer, QR); cancel
	Sold(ctx context.Context, branchID, customerID uint, status, q string) ([]dto.CustomerPackageDTO, error)
	Sell(ctx context.Context, branchID uint, by *domain.User, req dto.SellRequest) (*dto.CustomerPackageDTO, error)
	// Members: the customers who bought a package here (the Membership tab), a page at a time
	Members(ctx context.Context, branchID uint, keyword string, page, limit int) (*coredto.PagedResult[dto.MemberDTO], error)
	MarkPaid(ctx context.Context, branchID, id uint, by *domain.User, req dto.MarkPaidRequest) (*dto.CustomerPackageDTO, error)
	Cancel(ctx context.Context, branchID, id uint, by *domain.User) (*dto.CustomerPackageDTO, error)

	// the log of visits: the team reads any package's, a customer their own; the team can adjust (add or remove a
	// visit, extend the last day) with a note
	Visits(ctx context.Context, branchID, id uint) ([]dto.VisitDTO, error)
	MyVisits(ctx context.Context, userID, id uint) ([]dto.VisitDTO, error)
	Adjust(ctx context.Context, branchID, id uint, by *domain.User, req dto.AdjustRequest) (*dto.CustomerPackageDTO, error)

	// what the payments module needs to take money for a package
	PayFactsByCustomer(ctx context.Context, userID, id uint) (*domain.PayFacts, error)
	SettlePaid(ctx context.Context, branchID, id uint, ref string) error
	// CancelUnpaid cancels a package that is still waiting for its payment because the online payment failed or ran
	// out; one that is paid, active or already cancelled is left alone (nil)
	CancelUnpaid(ctx context.Context, branchID, id uint, why string) error
	// SetHold moves the pay-by time to the end of the customer's online payment
	SetHold(ctx context.Context, id uint, until time.Time) error
	// ExpireUnpaid cancels the packages not paid by their pay-by time (a job, every minute)
	ExpireUnpaid(ctx context.Context) error
}
