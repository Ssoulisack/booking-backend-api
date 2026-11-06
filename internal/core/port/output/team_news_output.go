package output

import "context"

// News topics: which of the branch's Telegram switches a piece of news follows
const NewsPayment = "payment" // money, slips: follows the "payments" switch

// TeamNews tells a branch's team something that is not about one booking (a package sold): a Lao title and a few
// lines, with the customer's name and phone added (the phone unless the branch hides it). It returns at once and
// works in the background; a failure never changes what happened. Staff-made changes are not announced.
type TeamNews interface {
	Announce(ctx context.Context, branchID uint, topic, title string, lines []string, customerName, customerPhone string)
}
