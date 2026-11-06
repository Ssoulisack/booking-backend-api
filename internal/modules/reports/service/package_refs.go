package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
)

// packageRefs names the packages that paid these bookings (the ones with a customer package)
func (s *ReportServiceImpl) packageRefs(ctx context.Context, branchID uint, bookings []domain.Booking) map[uint]output.PackageRef {
	if s.packages == nil {
		return nil
	}
	var ids []uint
	seen := map[uint]bool{}
	for _, b := range bookings {
		if b.CustomerPackageID != nil && !seen[*b.CustomerPackageID] {
			seen[*b.CustomerPackageID] = true
			ids = append(ids, *b.CustomerPackageID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	refs, err := s.packages.Refs(ctx, branchID, ids)
	if err != nil {
		return nil // the names are a help: a report without them is still right
	}
	return refs
}
