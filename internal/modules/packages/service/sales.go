package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"time"
)

var _ output.PackageSales = (*Service)(nil)

// SalesBetween lists the packages paid in [from, to): who bought, how it was paid, who took the money
func (s *Service) SalesBetween(ctx context.Context, branchID uint, from, to time.Time) ([]output.PackageSale, error) {
	list, err := s.Repo.PaidBetween(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	people := map[uint]*domain.User{}
	out := make([]output.PackageSale, 0, len(list))
	for _, cp := range list {
		sale := output.PackageSale{
			ID: cp.ID, Code: cp.Code(), Package: cp.Name, Kind: cp.Kind, Amount: cp.Amount, Price: cp.Price, Currency: cp.Currency,
			Method: cp.PaymentMethod, Cancelled: cp.Status == "cancelled",
		}
		if cp.PaidAt != nil {
			sale.PaidAt = *cp.PaidAt
		}
		if u := s.person(ctx, people, cp.CustomerID); u != nil {
			sale.CustomerName, sale.CustomerPhone = u.Name, u.Phone
		}
		if cp.PaidByID != nil {
			if u := s.person(ctx, people, *cp.PaidByID); u != nil {
				sale.PaidBy = u.Name
			}
		}
		out = append(out, sale)
	}
	return out, nil
}

// Refs names the packages with these ids
func (s *Service) Refs(ctx context.Context, branchID uint, ids []uint) (map[uint]output.PackageRef, error) {
	list, err := s.Repo.PurchasesByIDs(ctx, branchID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]output.PackageRef, len(list))
	for _, cp := range list {
		out[cp.ID] = output.PackageRef{Code: cp.Code(), Package: cp.Name, Kind: cp.Kind, Amount: cp.Amount}
	}
	return out, nil
}
