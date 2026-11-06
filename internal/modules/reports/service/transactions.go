package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"context"
	"fmt"
	"sort"
)

// Transactions lists what was paid in a period, newest first: money for bookings, money for packages, and the
// package visits used (no money). Counted by the day the money came in, not the day of the booking.
func (s *ReportServiceImpl) Transactions(ctx context.Context, branchID uint, q dto.ReportQuery) (*dto.TransactionsDTO, error) {
	loc, from, to, err := s.period(ctx, branchID, q)
	if err != nil {
		return nil, err
	}
	paid, err := s.bookings.PaidBetween(ctx, branchID, from, to)
	if err != nil {
		return nil, err
	}
	sports, _, err := s.sports.List(ctx, branchID, output.ListQuery{Limit: 500})
	if err != nil {
		return nil, err
	}
	sportName := map[uint]string{}
	for _, sp := range sports {
		sportName[sp.ID] = sp.Name
	}
	refs := s.packageRefs(ctx, branchID, paid)
	out := &dto.TransactionsDTO{From: q.From, To: q.To, Rows: []dto.TransactionRow{}}
	for _, b := range paid {
		kind := dto.TransactionBooking
		if b.PaymentMethod == domain.MethodPackage {
			kind = dto.TransactionVisit
		}
		who := ""
		if b.PaidBy != nil {
			who = b.PaidBy.Name
		}
		out.Rows = append(out.Rows, dto.TransactionRow{
			PaidAt: b.PaidAt.In(loc), Kind: kind, Code: b.Code, Name: bookerName(b), Phone: b.GuestPhone,
			What: sportName[b.SportID], Method: b.PaymentMethod, Amount: b.TotalPrice, Currency: b.Currency, PaidBy: who,
			Cancelled: b.Status == domain.BookingCancelled,
		})
		last := &out.Rows[len(out.Rows)-1]
		if b.Customer != nil {
			last.Phone = b.Customer.Phone
		}
		if b.CustomerPackageID != nil {
			if ref, ok := refs[*b.CustomerPackageID]; ok {
				last.Package, last.PackageCode = ref.Label(), ref.Code
			}
		}
	}
	if s.packages != nil {
		sales, err := s.packages.SalesBetween(ctx, branchID, from, to)
		if err != nil {
			return nil, err
		}
		for _, p := range sales {
			unit := "times"
			if p.Kind == "months" {
				unit = "months"
			}
			out.Rows = append(out.Rows, dto.TransactionRow{
				PaidAt: p.PaidAt.In(loc), Kind: dto.TransactionPackage, Code: p.Code, Name: p.CustomerName, Phone: p.CustomerPhone,
				What: fmt.Sprintf("%s · %d %s", p.Package, p.Amount, unit), Method: p.Method, Amount: p.Price, Currency: p.Currency,
				PaidBy: p.PaidBy, Cancelled: p.Cancelled,
			})
		}
	}
	sort.SliceStable(out.Rows, func(i, j int) bool { return out.Rows[i].PaidAt.After(out.Rows[j].PaidAt) })

	totals := map[string]int64{}
	kinds := map[string]*dto.TransactionKindTotal{}
	methods := map[string]*dto.CashUpMethod{}
	for _, r := range out.Rows {
		totals[r.Currency] += r.Amount
		kk := r.Kind + "|" + r.Currency
		k, ok := kinds[kk]
		if !ok {
			k = &dto.TransactionKindTotal{Kind: r.Kind, Currency: r.Currency}
			kinds[kk] = k
		}
		k.Count++
		k.Amount += r.Amount
		mk := r.Method + "|" + r.Currency
		m, ok := methods[mk]
		if !ok {
			m = &dto.CashUpMethod{Method: r.Method, Currency: r.Currency}
			methods[mk] = m
		}
		m.Count++
		m.Amount += r.Amount
	}
	out.Totals = moneyList(totals)
	out.ByKind = make([]dto.TransactionKindTotal, 0, len(kinds))
	for _, k := range kinds {
		out.ByKind = append(out.ByKind, *k)
	}
	sort.Slice(out.ByKind, func(i, j int) bool {
		return out.ByKind[i].Kind+out.ByKind[i].Currency < out.ByKind[j].Kind+out.ByKind[j].Currency
	})
	out.ByMethod = make([]dto.CashUpMethod, 0, len(methods))
	for _, m := range methods {
		out.ByMethod = append(out.ByMethod, *m)
	}
	sort.Slice(out.ByMethod, func(i, j int) bool {
		return out.ByMethod[i].Method+out.ByMethod[i].Currency < out.ByMethod[j].Method+out.ByMethod[j].Currency
	})
	return out, nil
}
