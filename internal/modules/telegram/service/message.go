package service

import (
	"backend/internal/core/domain"
	"backend/internal/platform/kit"
	"fmt"
	"strconv"
	"strings"
)

// compose writes one message in Lao: a title, then one line per fact. Plain text: nothing the customer typed can be
// read as formatting. Dates are numbers (day/month/year, 24-hour time); names the team or the customer typed are
// shown as they were typed.
func compose(b domain.Booking, k kind, why string, hidePhone bool) string {
	loc := kit.LocationOf(&b.Branch)
	title := ""
	switch k {
	case kindNew:
		title = "🆕 ການຈອງໃໝ່"
		if b.EventID != nil {
			title = "🎉 ການຈອງ Event ໃໝ່"
		} else if b.Status == domain.BookingPending {
			title = "🆕 ການຈອງໃໝ່ (ລໍຖ້າອະນຸມັດ)"
		}
	case kindSlip:
		title = "🧾 ລູກຄ້າສົ່ງສະລິບແລ້ວ ກະລຸນາກວດສອບ"
	case kindPaid:
		title = "💰 ໄດ້ຮັບເງິນແລ້ວ"
	case kindCancelled:
		title = "❌ ລູກຄ້າຍົກເລີກການຈອງ"
		if why == "unpaid" {
			title = "⏱ ປ່ອຍການຈອງ ເພາະບໍ່ຊຳລະທັນເວລາ"
		}
	}

	// a restaurant booking lists the table(s), the zone and the number of people on their own lines; any other
	// booking keeps one "items" line (sport, court, coach, class)
	restaurant := b.Branch.Type == domain.BranchTypeRestaurant
	var what, tables []string
	if b.Event != nil && b.Event.Name != "" {
		what = append(what, "Event "+b.Event.Name)
	}
	if b.Sport != nil && b.Sport.Name != "" {
		what = append(what, b.Sport.Name) // for a restaurant this is the area ("Main hall")
	}
	for _, it := range b.Items {
		if name := resourceName(it); name != "" {
			if restaurant {
				tables = append(tables, name)
			} else {
				what = append(what, name)
			}
		}
		for _, name := range []string{coachName(it), classTitle(it)} {
			if name != "" {
				what = append(what, name)
			}
		}
	}
	if !restaurant {
		if b.Zones != "" {
			what = append(what, b.Zones)
		}
		if b.Guests > 0 {
			what = append(what, fmt.Sprintf("%d ຄົນ", b.Guests))
		}
	}

	lines := []string{title, "", "ສາຂາ: " + b.Branch.Name, "ລະຫັດ: " + b.Code}
	if len(what) > 0 {
		lines = append(lines, "ລາຍການ: "+strings.Join(what, " · "))
	}
	if restaurant {
		if len(tables) > 0 {
			lines = append(lines, "ໂຕະ: "+strings.Join(tables, ", "))
		}
		if b.Zones != "" { // only when the tables stand in a named zone
			lines = append(lines, "ໂຊນ: "+b.Zones)
		}
		if b.Guests > 0 {
			lines = append(lines, fmt.Sprintf("ຈຳນວນ: %d ຄົນ", b.Guests))
		}
	}
	lines = append(lines, "ເວລາ: "+whenText(b))
	if b.TotalPrice > 0 {
		lines = append(lines, "ລວມ: "+money(b.TotalPrice, b.Currency))
	}
	if k == kindNew && b.CustomerPackageID != nil {
		lines = append(lines, "ຊຳລະດ້ວຍ: ແພັກເກັດ")
	}
	if k == kindPaid && b.PaymentMethod != "" {
		lines = append(lines, "ຊຳລະຜ່ານ: "+methodText(b.PaymentMethod))
	}
	if k == kindCancelled && strings.TrimSpace(b.CancelReason) != "" {
		lines = append(lines, "ເຫດຜົນ: "+truncate(b.CancelReason, 200))
	}
	name, phone, email := contactOf(b)
	if hidePhone {
		phone, email = "", ""
	}
	if who := strings.TrimSpace(name + " " + phone); who != "" {
		lines = append(lines, "ລູກຄ້າ: "+who)
	}
	if email != "" {
		lines = append(lines, "ອີເມວ: "+email)
	}
	if k == kindNew && b.EventID != nil && b.PaymentStatus != domain.PaymentPaid && b.PayBy != nil {
		lines = append(lines, "ຊຳລະກ່ອນ: "+b.PayBy.In(loc).Format("02/01/2006 15:04"))
	}
	return strings.Join(lines, "\n")
}

// methodText: how a booking was paid
func methodText(m string) string {
	switch m {
	case domain.MethodOnline:
		return "ອອນລາຍ"
	case domain.MethodCash:
		return "ເງິນສົດ"
	case domain.MethodTransfer:
		return "ໂອນເງິນ"
	case domain.MethodPackage:
		return "ແພັກເກັດ"
	}
	return "ອື່ນໆ"
}

func resourceName(it domain.BookingItem) string {
	if it.Resource != nil {
		return it.Resource.Name
	}
	return ""
}

func coachName(it domain.BookingItem) string {
	if it.Coach != nil {
		return it.Coach.Name
	}
	return ""
}

func classTitle(it domain.BookingItem) string {
	if it.ClassSession != nil {
		return it.ClassSession.Title
	}
	return ""
}

// whenText: a court or class has a start and end; a table has a start (a bar only the day); an event day shows the
// event's own time, not the hours its tables are held for
func whenText(b domain.Booking) string {
	l := kit.LocationOf(&b.Branch)
	day := b.StartsAt.In(l).Format("02/01/2006")
	switch {
	case b.Event != nil:
		if b.Event.StartTime == "" {
			return day
		}
		if b.Event.EndTime == "" {
			return day + ", " + b.Event.StartTime
		}
		return day + ", " + b.Event.StartTime + "–" + b.Event.EndTime
	case b.Guests > 0:
		return day + ", " + b.StartsAt.In(l).Format("15:04")
	}
	return day + ", " + b.StartsAt.In(l).Format("15:04") + "–" + b.EndsAt.In(l).Format("15:04")
}

// contactOf: who booked and how to reach them. An account's email counts only when it is a real address (the
// placeholder the system makes for phone-only accounts is left out).
func contactOf(b domain.Booking) (name, phone, email string) {
	name, phone, email = b.GuestName, b.GuestPhone, b.GuestEmail
	if b.Customer != nil {
		name, phone, email = b.Customer.Name, b.Customer.Phone, b.Customer.ContactEmail()
	}
	return name, phone, email
}

func money(amount int64, currency string) string {
	digits := strconv.FormatInt(amount, 10)
	var sb strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 && d != '-' && digits[i-1] != '-' {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return strings.TrimSpace(sb.String() + " " + currency)
}
