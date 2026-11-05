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
