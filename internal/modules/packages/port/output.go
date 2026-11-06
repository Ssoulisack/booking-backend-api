// Package port: the module's two edges. Input is what the module offers; output is what it needs.
package port

import (
	"backend/internal/modules/packages/domain"
	"context"
	"time"
)

// SlipWays tells which ways a customer can send a payment slip to a branch now (the events module keeps the
// branch's "how to pay" setting)
type SlipWays interface {
	SlipWays(ctx context.Context, branchID uint) (whatsapp, telegram bool, err error)
}

// SlipSender puts a customer's payment slip (a picture) in the branch's Telegram group (the Telegram module); the
// title and lines are Lao text written here, the customer's name and phone are added by the sender
type SlipSender interface {
	SendSlip(ctx context.Context, branchID uint, title string, lines []string, name, phone string, photo []byte, filename string) error
}

// Repository stores the module's tables
type Repository interface {
	// Save creates (ID 0) or updates the package and replaces its options in one step: options with an ID
	// are updated, options with ID 0 are added, the branch's other options of the package are removed.
	Save(ctx context.Context, p *domain.Package, options []domain.Option) error
	// Get returns the package of the branch, or a not-found error
	Get(ctx context.Context, branchID, id uint) (*domain.Package, error)
	// List: the branch's packages, newest first; activeOnly keeps active ones
	List(ctx context.Context, branchID uint, activeOnly bool) ([]domain.Package, error)
	// Options: the options of these packages (activeOnly keeps active ones), by package then amount
	Options(ctx context.Context, packageIDs []uint, activeOnly bool) ([]domain.Option, error)
	// Delete removes the package and its options; customer packages already sold keep their copied facts
	Delete(ctx context.Context, branchID, id uint) error
	// Option returns one option of a package, or a not-found error
	Option(ctx context.Context, packageID, id uint) (*domain.Option, error)

	// ---- packages customers own ----
	CreatePurchase(ctx context.Context, cp *domain.CustomerPackage) error
	SavePurchase(ctx context.Context, cp *domain.CustomerPackage) error
	// AdjustPurchase changes one package in a single transaction with its row locked, so two people adjusting at the
	// same moment both count. change edits the package it is given and returns the log line to write (or an error to
	// stop: nothing is saved then)
	AdjustPurchase(ctx context.Context, branchID, id uint, change func(cp *domain.CustomerPackage) (*domain.Visit, error)) (*domain.CustomerPackage, error)
	// Purchase / PurchaseForCustomer return one, or a not-found error
	Purchase(ctx context.Context, branchID, id uint) (*domain.CustomerPackage, error)
	PurchaseForCustomer(ctx context.Context, userID, id uint) (*domain.CustomerPackage, error)
	// Purchases: a branch's customer packages, newest first; customerID 0 = any. q finds by code ("P-12" or 12), by
	// the customer's phone, name, number or email, or by the package name ("" = no search)
	Purchases(ctx context.Context, branchID, customerID uint, q string, limit int) ([]domain.CustomerPackage, error)
	// PurchasesOf: everything one customer bought, at every branch, newest first
	PurchasesOf(ctx context.Context, userID uint) ([]domain.CustomerPackage, error)
	// TakeVisit takes one visit of an active times package that has some left (false: none left); ReturnVisit gives
	// one back (only called for a visit that was taken)
	TakeVisit(ctx context.Context, id uint) (bool, error)
	ReturnVisit(ctx context.Context, id uint) error
	// AddVisit writes a line of the log; Visits reads a package's log, newest first; VisitSum adds up what one
	// booking did to a package (-1 = a visit taken and not given back)
	AddVisit(ctx context.Context, v *domain.Visit) error
	Visits(ctx context.Context, customerPackageID uint) ([]domain.Visit, error)
	VisitSum(ctx context.Context, customerPackageID, bookingID uint) (int, error)
	// PurchasesByIDs: the branch's customer packages with these ids
	PurchasesByIDs(ctx context.Context, branchID uint, ids []uint) ([]domain.CustomerPackage, error)
	// PaidBetween: the branch's packages paid in [from, to), oldest first (the reports and the cash-up)
	PaidBetween(ctx context.Context, branchID uint, from, to time.Time) ([]domain.CustomerPackage, error)
	// PendingCount: the customer's packages at the branch still waiting for payment
	PendingCount(ctx context.Context, branchID, customerID uint) (int64, error)
}
