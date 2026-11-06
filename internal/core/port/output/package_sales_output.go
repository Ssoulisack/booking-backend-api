package output

import (
	"context"
	"fmt"
	"time"
)

// PackageSale is a package (course pass) a customer paid for: money received, for the reports and the cash-up
type PackageSale struct {
	ID            uint
	Code          string // P-0003
	Package       string
	Kind          string // times | months
	Amount        int    // visits or months
	CustomerName  string
	CustomerPhone string
	Price         int64
	Currency      string
	Method        string // cash | transfer | qr | other | online
	PaidAt        time.Time
	PaidBy        string // the team member who took the money; "" for an online payment
	Cancelled     bool   // cancelled afterwards (the money was received; giving it back is settled outside the app)
}

// PackageRef names a customer's package: for a booking paid with it
type PackageRef struct {
	Code    string // P-0003
	Package string // Package A
	Kind    string // times | months
	Amount  int    // visits or months
}

// Label writes the package for people: "Package A · 10 times"
func (r PackageRef) Label() string {
	unit := "times"
	if r.Kind == "months" {
		unit = "months"
	}
	return fmt.Sprintf("%s · %d %s", r.Package, r.Amount, unit)
}

// PackageSales lists the package sales paid in a period, and names packages (the packages module); nil in the
// reports: no packages
type PackageSales interface {
	SalesBetween(ctx context.Context, branchID uint, from, to time.Time) ([]PackageSale, error)
	// Refs names the customer packages with these ids (a booking paid with a package says which one)
	Refs(ctx context.Context, branchID uint, ids []uint) (map[uint]PackageRef, error)
}
