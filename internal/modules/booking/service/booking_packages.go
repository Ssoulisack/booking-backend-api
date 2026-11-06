package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"net/http"
	"time"
)

// Paying a booking with a customer's package (course pass). The packages module decides whether the package fits
// and counts the visits; this file only asks it, and makes the booking free.

// packageUse is a visit taken for a booking that is about to be saved
type packageUse struct {
	packageID uint
	change    int
	ref       output.PackageRef
}

// takePackage checks the request may use a package and takes the visit. nil when no package is asked for.
// Only a logged-in customer (or staff booking for one) has packages; tables are never paid this way.
func (s *Service) takePackage(ctx context.Context, b *domain.Branch, who *booker, req dto.CreateBookingRequest, start time.Time, sportID uint, resourceID *uint, withCoach bool) (*packageUse, error) {
	if req.PackageID == nil {
		return nil, nil
	}
	if s.packages == nil || b.Type != domain.BranchTypeSport {
		return nil, response.NewError(http.StatusConflict, "PACKAGES_OFF")
	}
	if who.Customer == nil {
		return nil, badRequest("PACKAGE_NEEDS_ACCOUNT")
	}
	change, ref, err := s.packages.Take(ctx, output.PackageFit{
		BranchID: b.ID, CustomerID: who.Customer.ID, PackageID: *req.PackageID, Day: start.Format("2006-01-02"),
		SportID: sportID, ResourceID: resourceID, Class: req.ClassSessionID != nil, WithCoach: withCoach,
	})
	if err != nil {
		return nil, err
	}
	return &packageUse{packageID: *req.PackageID, change: change, ref: ref}, nil
}

// apply makes the booking free and paid by the package
func (u *packageUse) apply(bk *domain.Booking, now time.Time) {
	if u == nil {
		return
	}
	id := u.packageID
	bk.CustomerPackageID = &id
	bk.PackageLabel, bk.PackageCode = u.ref.Label(), u.ref.Code
	bk.TotalPrice = 0
	for i := range bk.Items {
		bk.Items[i].Price = 0
	}
	bk.PaymentStatus, bk.PaymentMethod, bk.PaidAt = domain.PaymentPaid, domain.MethodPackage, &now
}

// saved writes the booking into the package's log; failed gives the visit back when the booking was not saved
func (s *Service) packageSaved(ctx context.Context, u *packageUse, bk *domain.Booking) {
	if u != nil {
		s.packages.Record(ctx, u.packageID, bk.ID, u.change)
	}
}

func (s *Service) packageFailed(ctx context.Context, u *packageUse) {
	if u != nil && u.change < 0 {
		s.packages.GiveBack(ctx, u.packageID, 0) // no booking: the visit is simply put back
	}
}

// packageCancelled gives a cancelled booking's visit back
func (s *Service) packageCancelled(ctx context.Context, bk *domain.Booking) {
	if s.packages != nil && bk.CustomerPackageID != nil {
		s.packages.GiveBack(ctx, *bk.CustomerPackageID, bk.ID)
	}
}
