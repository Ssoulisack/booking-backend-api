package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	pkgdomain "backend/internal/modules/packages/domain"
	"backend/internal/modules/packages/dto"
	"context"
	"log"
	"strings"
	"time"
	"unicode/utf8"
)

var _ output.PackageUse = (*Service)(nil)

// Take checks that a customer's package can pay a booking and takes the visit (see output.PackageUse). The
// package must be the customer's own, at this branch, paid, not out of time, with visits left, and cover what is
// booked: a group class of its sport, or a rental of its space with no coach.
func (s *Service) Take(ctx context.Context, f output.PackageFit) (int, output.PackageRef, error) {
	cp, err := s.Repo.Purchase(ctx, f.BranchID, f.PackageID)
	if err != nil || cp.CustomerID != f.CustomerID {
		return 0, output.PackageRef{}, notFound("PACKAGE_NOT_FOUND")
	}
	switch cp.Status {
	case pkgdomain.StatusPendingPayment:
		return 0, output.PackageRef{}, conflict("PACKAGE_NOT_PAID")
	case pkgdomain.StatusActive:
	default:
		return 0, output.PackageRef{}, conflict("PACKAGE_NOT_ACTIVE")
	}
	// the status on the day of the booking: a package that runs out before then cannot pay for it
	switch cp.StatusOn(f.Day) {
	case pkgdomain.StatusUsedUp:
		return 0, output.PackageRef{}, conflict("PACKAGE_USED_UP")
	case pkgdomain.StatusExpired:
		return 0, output.PackageRef{}, conflict("PACKAGE_EXPIRED")
	}
	ref := output.PackageRef{Code: cp.Code(), Package: cp.Name, Kind: cp.Kind, Amount: cp.Amount}
	covers := false
	switch {
	case f.Class:
		covers = cp.Covers == pkgdomain.CoversClass && cp.SportID == f.SportID
	default:
		covers = cp.Covers == pkgdomain.CoversSpace && cp.ResourceID != nil && f.ResourceID != nil &&
			*cp.ResourceID == *f.ResourceID && !f.WithCoach
	}
	if !covers {
		return 0, output.PackageRef{}, conflict("PACKAGE_DOES_NOT_COVER")
	}
	if cp.Kind == pkgdomain.OptionMonths {
		return 0, ref, nil
	}
	ok, err := s.Repo.TakeVisit(ctx, cp.ID)
	if err != nil {
		return 0, output.PackageRef{}, err
	}
	if !ok {
		return 0, output.PackageRef{}, conflict("PACKAGE_USED_UP")
	}
	return -1, ref, nil
}

// Record writes the booking into the package's log
func (s *Service) Record(ctx context.Context, packageID, bookingID uint, change int) {
	id := bookingID
	if err := s.Repo.AddVisit(ctx, &pkgdomain.Visit{CustomerPackageID: packageID, BookingID: &id, Change: change, CreatedAt: s.now()}); err != nil {
		log.Printf("⚠️  package %d: visit of booking %d was not logged: %v", packageID, bookingID, err)
	}
}

// GiveBack puts a visit back: the booking was not saved (bookingID 0), or it was cancelled. A cancelled booking
// gives its visit back once.
func (s *Service) GiveBack(ctx context.Context, packageID, bookingID uint) {
	if bookingID != 0 {
		sum, err := s.Repo.VisitSum(ctx, packageID, bookingID)
		if err != nil || sum >= 0 {
			return // no visit was taken for it, or it was given back already
		}
	}
	if err := s.Repo.ReturnVisit(ctx, packageID); err != nil {
		log.Printf("⚠️  package %d: visit not given back: %v", packageID, err)
		return
	}
	if bookingID != 0 {
		id := bookingID
		_ = s.Repo.AddVisit(ctx, &pkgdomain.Visit{CustomerPackageID: packageID, BookingID: &id, Change: 1, Note: "cancelled", CreatedAt: s.now()})
	}
}

// ---------- the log, and the team's adjustments ----------

func (s *Service) Visits(ctx context.Context, branchID, id uint) ([]dto.VisitDTO, error) {
	if _, err := s.branch(ctx, branchID); err != nil {
		return nil, err
	}
	if _, err := s.Repo.Purchase(ctx, branchID, id); err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	return s.visitList(ctx, id)
}

func (s *Service) MyVisits(ctx context.Context, userID, id uint) ([]dto.VisitDTO, error) {
	if _, err := s.Repo.PurchaseForCustomer(ctx, userID, id); err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	return s.visitList(ctx, id)
}

func (s *Service) visitList(ctx context.Context, id uint) ([]dto.VisitDTO, error) {
	list, err := s.Repo.Visits(ctx, id)
	if err != nil {
		return nil, err
	}
	people := map[uint]*domain.User{}
	out := make([]dto.VisitDTO, 0, len(list))
	for _, v := range list {
		d := dto.VisitDTO{ID: v.ID, Change: v.Change, BookingID: v.BookingID, Note: v.Note, CreatedAt: v.CreatedAt}
		if v.ByUserID != nil {
			if u := s.person(ctx, people, *v.ByUserID); u != nil {
				d.ByName = u.Name
			}
		}
		out = append(out, d)
	}
	return out, nil
}

const maxAdjust = 1000

// Adjust lets the team add or remove visits and extend the last day of a paid package, with a reason that is kept
// in the log. Times packages take visits; the last day is a months pass's end, or a times package's expiry.
func (s *Service) Adjust(ctx context.Context, branchID, id uint, by *domain.User, req dto.AdjustRequest) (*dto.CustomerPackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	note := strings.TrimSpace(req.Note)
	if note == "" || utf8.RuneCountInString(note) > 200 {
		return nil, badRequest("NOTE_REQUIRED")
	}
	if (req.Visits == 0 && req.ExtendDays == 0) || req.Visits < -maxAdjust || req.Visits > maxAdjust || req.ExtendDays < 0 || req.ExtendDays > maxDays {
		return nil, badRequest("INVALID_ADJUSTMENT")
	}
	// everything that reads the package runs inside the locked transaction, so a second adjust waits and then
	// sees the first one's result (no lost update)
	cp, err := s.Repo.AdjustPurchase(ctx, branchID, id, func(cp *pkgdomain.CustomerPackage) (*pkgdomain.Visit, error) {
		if cp.Status != pkgdomain.StatusActive {
			return nil, conflict("PACKAGE_NOT_ACTIVE")
		}
		if req.Visits != 0 && cp.Kind != pkgdomain.OptionTimes {
			return nil, badRequest("INVALID_ADJUSTMENT") // a months pass has no visits to count
		}
		if req.Visits != 0 && cp.TimesLeft+req.Visits < 0 {
			return nil, conflict("NOT_ENOUGH_VISITS")
		}
		if req.ExtendDays != 0 {
			end := &cp.ExpiresOn
			if cp.Kind == pkgdomain.OptionMonths {
				end = &cp.EndsOn
			}
			if *end == "" {
				return nil, badRequest("INVALID_ADJUSTMENT") // it never runs out: nothing to extend
			}
			from := s.today(b)
			if *end > from {
				from = *end
			}
			day, err := time.Parse("2006-01-02", from)
			if err != nil {
				return nil, err
			}
			*end = day.AddDate(0, 0, req.ExtendDays).Format("2006-01-02")
		}
		cp.TimesLeft += req.Visits
		byID := by.ID
		return &pkgdomain.Visit{CustomerPackageID: cp.ID, Change: req.Visits, Note: note, ByUserID: &byID, CreatedAt: s.now()}, nil
	})
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	return s.purchaseDTO(ctx, b, cp, nil)
}
