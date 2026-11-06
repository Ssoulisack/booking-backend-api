package output

import "context"

// TeamNotifier tells a branch's team about what happens to bookings (today through a Telegram group). Every
// method returns at once and does its work in the background; a failure is the notifier's to record and never
// changes the booking. Staff-made changes are not announced: the team already knows.
type TeamNotifier interface {
	NewBooking(ctx context.Context, branchID, bookingID uint)
	// SlipSent: a customer says they sent the payment slip; the team should check it
	SlipSent(ctx context.Context, branchID, bookingID uint)
	// PaymentReceived: an online payment settled the booking
	PaymentReceived(ctx context.Context, branchID, bookingID uint)
	// Cancelled: the customer cancelled ("customer"), or the unpaid event booking was released ("unpaid")
	Cancelled(ctx context.Context, branchID, bookingID uint, why string)
}

// SlipWays tells which ways a customer can send a payment slip to a branch now (WhatsApp, Telegram): the branch's
// "how customers pay" setting (events module)
type SlipWays interface {
	SlipWays(ctx context.Context, branchID uint) (whatsapp, telegram bool, err error)
}

// SlipDesk puts a customer's payment slip (a picture) for a booking in the branch's Telegram group, with the
// booking's facts as the caption (the Telegram module)
type SlipDesk interface {
	SendBookingSlip(ctx context.Context, branchID, bookingID uint, photo []byte, filename string) error
}

// PackageFit is what a booking is, for checking that a customer's package (course pass) can pay it
type PackageFit struct {
	BranchID    uint
	CustomerID  uint
	PackageID   uint   // the customer's package (not the package that is sold)
	Day         string // the day of the booking, YYYY-MM-DD in the branch time zone
	SportID     uint
	ResourceID  *uint // the space of a rental
	Class       bool  // a seat in a group class (else a rental of a space)
	TimetableID *uint // the weekly class the seat belongs to (nil: a one-off class)
	WithCoach   bool  // a coach is part of the booking: packages do not cover coaches
}

// PackageUse lets a booking be paid with a customer's package (the packages module). It works in three steps so a
// booking that fails to save does not cost a visit: Take, then Record when the booking exists (or GiveBack when it
// did not); a cancelled booking calls GiveBack too.
type PackageUse interface {
	// Take checks the package fits and takes one visit (a months pass takes none); change is -1 or 0; ref names it
	Take(ctx context.Context, f PackageFit) (change int, ref PackageRef, err error)
	// Record writes the booking into the package's log
	Record(ctx context.Context, packageID, bookingID uint, change int)
	// GiveBack gives the visit of a booking back, once (nothing when none was taken, or already given back)
	GiveBack(ctx context.Context, packageID, bookingID uint)
}
