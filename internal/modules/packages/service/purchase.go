package service

import (
	"backend/internal/core/domain"
	coredto "backend/internal/core/dto"
	"backend/internal/core/port/output"
	pkgdomain "backend/internal/modules/packages/domain"
	"backend/internal/modules/packages/dto"
	"backend/internal/platform/kit"
	"backend/pkg/response"
	"context"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

const maxPending = 3 // packages one customer may leave waiting for payment at a branch

// the ways staff can record a payment (never "online": that comes from the gateway)
var deskMethods = []string{domain.MethodCash, domain.MethodTransfer, domain.MethodQR, domain.MethodOther}

// PayWindow is how long a customer has to pay a package they just bought online, before it cancels itself
const PayWindow = 30 * time.Minute

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func locationOf(b *domain.Branch) *time.Location {
	if loc, err := time.LoadLocation(b.Timezone); err == nil {
		return loc
	}
	return time.UTC
}

func (s *Service) today(b *domain.Branch) string {
	return s.now().In(locationOf(b)).Format("2006-01-02")
}

// ---------- customers ----------

func (s *Service) Buy(ctx context.Context, slug string, user *domain.User, packageID uint, req dto.BuyRequest) (*dto.CustomerPackageDTO, error) {
	b, err := s.Branches.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	if !b.IsPublic() || b.Type != domain.BranchTypeSport {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	n, err := s.Repo.PendingCount(ctx, b.ID, user.ID)
	if err != nil {
		return nil, err
	}
	if n >= maxPending {
		return nil, conflict("TOO_MANY_UNPAID_PACKAGES")
	}
	cp, err := s.newPurchase(ctx, b, user.ID, packageID, req.OptionID, true)
	if err != nil {
		return nil, err
	}
	cp.SoldByType = domain.CreatedByCustomer
	// the customer has this long to pay (the same as an online QR lives); the pay page counts it down
	by := s.now().Add(PayWindow)
	cp.PayBy = &by
	if err := s.Repo.CreatePurchase(ctx, cp); err != nil {
		return nil, err
	}
	s.tell(ctx, b, cp, "🛒 ລູກຄ້າຊື້ແພັກເກັດ (ລໍຖ້າຊຳລະ)")
	return s.purchaseDTO(ctx, b, cp, nil)
}

func (s *Service) Mine(ctx context.Context, userID uint) ([]dto.CustomerPackageDTO, error) {
	list, err := s.Repo.PurchasesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	branches := map[uint]*domain.Branch{}
	out := make([]dto.CustomerPackageDTO, 0, len(list))
	for i := range list {
		cp := &list[i]
		b, ok := branches[cp.BranchID]
		if !ok {
			if b, err = s.Branches.GetByID(ctx, cp.BranchID); err != nil {
				continue // the branch is gone
			}
			branches[cp.BranchID] = b
		}
		d, err := s.purchaseDTO(ctx, b, cp, nil)
		if err != nil {
			return nil, err
		}
		d.BranchName, d.BranchSlug = b.Name, b.Slug
		out = append(out, *d)
	}
	return out, nil
}

func (s *Service) CancelMine(ctx context.Context, userID, id uint) (*dto.CustomerPackageDTO, error) {
	cp, err := s.Repo.PurchaseForCustomer(ctx, userID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	// only one that is not paid: a paid package is the team's to deal with
	// (and not one whose slip the team is checking)
	if cp.Status != pkgdomain.StatusPendingPayment || cp.PaymentStatus == domain.PaymentPaid || cp.SlipSentAt != nil {
		return nil, conflict("PACKAGE_NOT_CANCELLABLE")
	}
	b, err := s.Branches.GetByID(ctx, cp.BranchID)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	cp.Status = pkgdomain.StatusCancelled
	if err := s.Repo.SavePurchase(ctx, cp); err != nil {
		return nil, err
	}
	s.tellAs(ctx, output.NewsCancel, b, cp, "↩️ ລູກຄ້າຍົກເລີກແພັກເກັດ")
	return s.purchaseDTO(ctx, b, cp, nil)
}

func (s *Service) SlipSent(ctx context.Context, userID, id uint, slip dto.SlipUpload) (*dto.CustomerPackageDTO, error) {
	cp, err := s.Repo.PurchaseForCustomer(ctx, userID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	if cp.Status != pkgdomain.StatusPendingPayment || cp.PaymentStatus != domain.PaymentUnpaid {
		return nil, conflict("NOT_WAITING_FOR_PAYMENT")
	}
	b, err := s.Branches.GetByID(ctx, cp.BranchID)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	via := slip.Via
	if via == "" {
		via = dto.SlipWhatsapp
	}
	if via != dto.SlipWhatsapp && via != dto.SlipTelegram {
		return nil, badRequest("INVALID_SLIP_WAY")
	}
	var wa, tg bool
	if s.Ways != nil {
		if wa, tg, err = s.Ways.SlipWays(ctx, b.ID); err != nil {
			return nil, err
		}
	}
	if (via == dto.SlipWhatsapp && !wa) || (via == dto.SlipTelegram && (!tg || s.Sender == nil)) {
		return nil, conflict("SLIP_WAY_OFF") // the branch does not take slips that way (any more)
	}

	if via == dto.SlipTelegram {
		ext, err := kit.CheckSlipImage(slip.Data)
		if err != nil {
			return nil, err
		}
		cust, _ := s.Users.FindByID(ctx, userID)
		var name, phone string
		if cust != nil {
			name, phone = cust.Name, cust.Phone
		}
		lines := []string{"ສາຂາ: " + b.Name, "ລະຫັດ: " + cp.Code(), "ແພັກເກັດ: " + cp.Name, "ລວມ: " + moneyText(cp.Price, cp.Currency)}
		err = s.Sender.SendSlip(ctx, b.ID, "🧾 ລູກຄ້າສົ່ງສະລິບແພັກເກັດ ກະລຸນາກວດສອບ", lines, name, phone, slip.Data, "slip-"+cp.Code()+"."+ext)
		if err != nil {
			log.Printf("⚠️  slip of package %s was not delivered to Telegram: %v", cp.Code(), err)
			return nil, response.NewError(http.StatusBadGateway, "SLIP_NOT_DELIVERED")
		}
	}
	// the team has it: the package keeps waiting until someone marks it paid. A WhatsApp slip is said once; a
	// picture may be sent again (the first was the wrong one) while the package waits.
	if cp.SlipSentAt == nil || via == dto.SlipTelegram {
		first := cp.SlipSentAt == nil
		now := s.now()
		cp.SlipSentAt, cp.SlipVia = &now, via
		if err := s.Repo.SavePurchase(ctx, cp); err != nil {
			return nil, err
		}
		// a WhatsApp slip is only a claim: tell the team to look (a picture in the group already says it)
		if via == dto.SlipWhatsapp && first {
			s.tell(ctx, b, cp, "🧾 ລູກຄ້າສົ່ງສະລິບແລ້ວ ກະລຸນາກວດສອບ (WhatsApp)")
		}
	}
	return s.purchaseDTO(ctx, b, cp, nil)
}

// moneyText writes an amount with thousands separators and its currency: 900,000 LAK
func moneyText(amount int64, currency string) string {
	digits := strconv.FormatInt(amount, 10)
	var sb strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return strings.TrimSpace(sb.String() + " " + currency)
}

// ---------- staff ----------

func (s *Service) Sold(ctx context.Context, branchID, customerID uint, status, q string) ([]dto.CustomerPackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	limit := 200
	if strings.TrimSpace(q) != "" {
		limit = 50 // a search: the best few, not the whole list
	}
	list, err := s.Repo.Purchases(ctx, branchID, customerID, q, limit)
	if err != nil {
		return nil, err
	}
	names, err := s.names(ctx, branchID)
	if err != nil {
		return nil, err
	}
	today := s.today(b)
	people := map[uint]*domain.User{}
	out := make([]dto.CustomerPackageDTO, 0, len(list))
	for i := range list {
		cp := &list[i]
		if status != "" && cp.StatusOn(today) != status {
			continue // the status filter is on what people see, so "expired" finds the ones that ran out by date too
		}
		d := purchaseDTO(b, cp, names)
		if u := s.person(ctx, people, cp.CustomerID); u != nil {
			d.CustomerName, d.CustomerPhone = u.Name, u.Phone
		}
		out = append(out, d)
	}
	return out, nil
}

// Members lists the customers who bought a package here (paid, not cancelled) with those packages: the Membership tab
func (s *Service) Members(ctx context.Context, branchID uint, keyword string, page, limit int) (*coredto.PagedResult[dto.MemberDTO], error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	ids, total, err := s.Repo.MemberIDs(ctx, branchID, keyword, (page-1)*limit, limit)
	if err != nil {
		return nil, err
	}
	names, err := s.names(ctx, branchID)
	if err != nil {
		return nil, err
	}
	today := s.today(b)
	out := make([]dto.MemberDTO, 0, len(ids))
	for _, id := range ids {
		list, err := s.Repo.Purchases(ctx, branchID, id, "", 100)
		if err != nil {
			return nil, err
		}
		m := dto.MemberDTO{UserID: id, Packages: []dto.CustomerPackageDTO{}}
		if u, err := s.Users.FindByID(ctx, id); err == nil && u != nil {
			m.Name, m.Phone, m.Email, m.MemberNo = u.Name, u.Phone, u.ContactEmail(), domain.MemberNoOf(u.MemberNo)
		}
		for i := range list {
			cp := &list[i]
			if cp.PaymentStatus != domain.PaymentPaid || cp.Status == pkgdomain.StatusCancelled {
				continue // only what the customer really has
			}
			d := purchaseDTO(b, cp, names)
			d.CustomerName, d.CustomerPhone = m.Name, m.Phone
			if cp.StatusOn(today) == pkgdomain.StatusActive {
				m.Active++
			}
			m.Packages = append(m.Packages, d)
		}
		out = append(out, m)
	}
	pages := int((total + int64(limit) - 1) / int64(limit))
	return &coredto.PagedResult[dto.MemberDTO]{Items: out, Meta: coredto.Meta{Total: int(total), Page: page, Limit: limit, TotalPage: pages}}, nil
}

func (s *Service) person(ctx context.Context, cache map[uint]*domain.User, id uint) *domain.User {
	if u, ok := cache[id]; ok {
		return u
	}
	u, err := s.Users.FindByID(ctx, id)
	if err != nil {
		u = nil
	}
	cache[id] = u
	return u
}

func (s *Service) Sell(ctx context.Context, branchID uint, by *domain.User, req dto.SellRequest) (*dto.CustomerPackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	method := strings.TrimSpace(req.PaymentMethod)
	if method != "" && !slices.Contains(deskMethods, method) {
		return nil, badRequest("INVALID_PAYMENT_METHOD")
	}
	cust, err := s.Users.FindByID(ctx, req.CustomerID)
	if err != nil || cust == nil {
		return nil, notFound("CUSTOMER_NOT_FOUND")
	}
	// staff may sell inactive packages and options too (a private deal): only the branch's own are allowed
	cp, err := s.newPurchase(ctx, b, cust.ID, req.PackageID, req.OptionID, false)
	if err != nil {
		return nil, err
	}
	cp.SoldByType, cp.SoldByID, cp.Notes = domain.CreatedByStaff, &by.ID, strings.TrimSpace(req.Notes)
	if method != "" {
		s.activate(b, cp, method, &by.ID)
	}
	if err := s.Repo.CreatePurchase(ctx, cp); err != nil {
		return nil, err
	}
	return s.purchaseDTO(ctx, b, cp, cust)
}

func (s *Service) MarkPaid(ctx context.Context, branchID, id uint, by *domain.User, req dto.MarkPaidRequest) (*dto.CustomerPackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(deskMethods, req.Method) {
		return nil, badRequest("INVALID_PAYMENT_METHOD")
	}
	cp, err := s.Repo.Purchase(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	switch {
	case cp.Status == pkgdomain.StatusCancelled:
		return nil, conflict("PACKAGE_CANCELLED")
	case cp.PaymentStatus == domain.PaymentPaid:
		return nil, conflict("PACKAGE_ALREADY_PAID")
	}
	s.activate(b, cp, req.Method, &by.ID)
	if err := s.Repo.SavePurchase(ctx, cp); err != nil {
		return nil, err
	}
	s.tell(ctx, b, cp, "✅ ທີມງານຮັບເງິນແລ້ວ (ແພັກເກັດ)", "ຊຳລະຜ່ານ: "+method(req.Method), "ໂດຍ: "+by.Name)
	return s.purchaseDTO(ctx, b, cp, nil)
}

func (s *Service) Cancel(ctx context.Context, branchID, id uint, by *domain.User) (*dto.CustomerPackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	cp, err := s.Repo.Purchase(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	if cp.Status == pkgdomain.StatusCancelled {
		return nil, conflict("PACKAGE_CANCELLED")
	}
	cp.Status = pkgdomain.StatusCancelled
	if err := s.Repo.SavePurchase(ctx, cp); err != nil {
		return nil, err
	}
	paid := "ຍັງບໍ່ໄດ້ຊຳລະ"
	if cp.PaymentStatus == domain.PaymentPaid {
		paid = "ຊຳລະແລ້ວ (ຄືນເງິນຕ້ອງຈັດການເອງ)"
	}
	s.tellAs(ctx, output.NewsStaffCancel, b, cp, "🚫 ທີມງານຍົກເລີກແພັກເກັດ", "ສະຖານະ: "+paid, "ໂດຍ: "+by.Name)
	return s.purchaseDTO(ctx, b, cp, nil)
}

// ---------- paying (the payments module) ----------

func (s *Service) PayFactsByCustomer(ctx context.Context, userID, id uint) (*domain.PayFacts, error) {
	cp, err := s.Repo.PurchaseForCustomer(ctx, userID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	b, err := s.Branches.GetByID(ctx, cp.BranchID)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	// the payments module speaks in booking words: waiting = pending, cancelled = cancelled
	status := domain.BookingConfirmed
	switch cp.Status {
	case pkgdomain.StatusPendingPayment:
		status = domain.BookingPending
	case pkgdomain.StatusCancelled:
		status = domain.BookingCancelled
	}
	return &domain.PayFacts{
		ID: cp.ID, BranchID: cp.BranchID, BranchSlug: b.Slug, Code: cp.Code(), OrderRef: cp.OrderRef(), Total: cp.Price, Currency: cp.Currency,
		Status: status, PaymentStatus: cp.PaymentStatus, PayBy: cp.PayBy, SlipSent: cp.SlipSentAt != nil, Purchase: true,
	}, nil
}

// SettlePaid: the gateway says the package was paid. It happens once; a cancelled or paid package refuses the
// money and the payments module marks it to be given back.
func (s *Service) SettlePaid(ctx context.Context, branchID, id uint, _ string) error {
	b, err := s.Branches.GetByID(ctx, branchID)
	if err != nil {
		return err
	}
	cp, err := s.Repo.Purchase(ctx, branchID, id)
	if err != nil {
		return err
	}
	switch {
	case cp.Status == pkgdomain.StatusCancelled:
		return domain.ErrNotPayable
	case cp.PaymentStatus == domain.PaymentPaid:
		return domain.ErrAlreadyPaid
	}
	s.activate(b, cp, domain.MethodOnline, nil)
	if err := s.Repo.SavePurchase(ctx, cp); err != nil {
		return err
	}
	s.tell(ctx, b, cp, "💰 ໄດ້ຮັບເງິນແລ້ວ (ແພັກເກັດ)", "ຊຳລະຜ່ານ: "+method(domain.MethodOnline))
	return nil
}

// CancelUnpaid: the online payment of a package failed or ran out, so the purchase is cancelled (it was never paid and
// gives the customer nothing). Only a package still waiting for money is touched; the team is told.
func (s *Service) CancelUnpaid(ctx context.Context, branchID, id uint, why string) error {
	b, err := s.Branches.GetByID(ctx, branchID)
	if err != nil {
		return err
	}
	cp, err := s.Repo.Purchase(ctx, branchID, id)
	if err != nil {
		return err
	}
	// one step in the database: a payment that arrives at this moment keeps the package
	done, err := s.Repo.CancelUnpaid(ctx, cp.ID)
	if err != nil || !done {
		return err
	}
	s.tell(ctx, b, cp, "❌ ຍົກເລີກແພັກເກັດ (ບໍ່ໄດ້ຊຳລະ)", "ເຫດຜົນ: "+why)
	return nil
}

// SetHold moves the pay-by time of a package still waiting for money to the end of the customer's online payment
func (s *Service) SetHold(ctx context.Context, id uint, until time.Time) error {
	return s.Repo.SetPayBy(ctx, id, until)
}

// ExpireUnpaid cancels the packages customers bought online and did not pay by their pay-by time (and sent no slip for:
// then the team decides). Runs every minute; a minute of grace lets a late webhook arrive first.
func (s *Service) ExpireUnpaid(ctx context.Context) error {
	list, err := s.Repo.PayByExpired(ctx, s.now().Add(-time.Minute))
	if err != nil {
		return err
	}
	for _, cp := range list {
		if err := s.CancelUnpaid(ctx, cp.BranchID, cp.ID, "ໝົດເວລາຊຳລະ"); err != nil {
			log.Printf("⚠️  package %d: %v", cp.ID, err) // the others still go
		}
	}
	return nil
}

// ---------- helpers ----------

// newPurchase makes a waiting customer package from a package and one of its options (the facts are copied);
// activeOnly: the package and the option must be on sale (customers), staff may sell any of the branch's
func (s *Service) newPurchase(ctx context.Context, b *domain.Branch, customerID, packageID, optionID uint, activeOnly bool) (*pkgdomain.CustomerPackage, error) {
	p, err := s.Repo.Get(ctx, b.ID, packageID)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	o, err := s.Repo.Option(ctx, p.ID, optionID)
	if err != nil {
		return nil, mapNotFound(err, "OPTION_NOT_FOUND")
	}
	// a customer can only buy what is on sale (also when buying the same one again): said plainly, not "not found"
	if activeOnly && (!p.Active || !o.Active) {
		return nil, conflict("PACKAGE_NOT_ON_SALE")
	}
	return &pkgdomain.CustomerPackage{
		BranchID: b.ID, CustomerID: customerID, PackageID: p.ID, OptionID: o.ID, SportID: p.SportID,
		Name: p.Name, Covers: p.Covers, ResourceID: p.ResourceID, ClassIDs: p.ClassIDs, CoachIDs: p.CoachIDs, Kind: o.Kind, Amount: o.Amount, ValidDays: o.ValidDays, MinutesPerVisit: o.MinutesPerVisit,
		Price: o.Price, Currency: b.Currency, Status: pkgdomain.StatusPendingPayment, PaymentStatus: domain.PaymentUnpaid,
	}, nil
}

// activate records the payment and starts the package today (branch time zone): a times package gets its
// visits and its last day, a months pass its end date
func (s *Service) activate(b *domain.Branch, cp *pkgdomain.CustomerPackage, method string, by *uint) {
	now := s.now()
	start := now.In(locationOf(b))
	cp.PaymentStatus, cp.PaymentMethod, cp.PaidAt, cp.PaidByID = domain.PaymentPaid, method, &now, by
	cp.Status = pkgdomain.StatusActive
	cp.StartsOn = start.Format("2006-01-02")
	switch cp.Kind {
	case pkgdomain.OptionMonths:
		cp.EndsOn = start.AddDate(0, cp.Amount, 0).Format("2006-01-02")
	default:
		cp.TimesLeft = cp.Amount
		if cp.ValidDays > 0 {
			cp.ExpiresOn = start.AddDate(0, 0, cp.ValidDays).Format("2006-01-02")
		}
	}
}

// purchaseDTO shows one customer package; the customer (when known) gives the staff's name and phone
func (s *Service) purchaseDTO(ctx context.Context, b *domain.Branch, cp *pkgdomain.CustomerPackage, cust *domain.User) (*dto.CustomerPackageDTO, error) {
	names, err := s.names(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	d := purchaseDTO(b, cp, names)
	if cust != nil {
		d.CustomerName, d.CustomerPhone = cust.Name, cust.Phone
	}
	return &d, nil
}

func purchaseDTO(b *domain.Branch, cp *pkgdomain.CustomerPackage, n nameBook) dto.CustomerPackageDTO {
	today := time.Now().In(locationOf(b)).Format("2006-01-02")
	d := dto.CustomerPackageDTO{
		ID: cp.ID, Code: cp.Code(), BranchID: cp.BranchID, CustomerID: cp.CustomerID, PackageID: cp.PackageID, OptionID: cp.OptionID, SportID: cp.SportID,
		Name: cp.Name, Covers: cp.Covers, ResourceID: cp.ResourceID, Kind: cp.Kind, Amount: cp.Amount, Price: cp.Price,
		Currency: cp.Currency, TimesLeft: cp.TimesLeft, MinutesPerVisit: cp.MinutesPerVisit, StartsOn: cp.StartsOn, EndsOn: cp.EndsOn, ExpiresOn: cp.ExpiresOn,
		Status: cp.StatusOn(today), PaymentStatus: cp.PaymentStatus, PaymentMethod: cp.PaymentMethod, PaidAt: cp.PaidAt, SlipSentAt: cp.SlipSentAt, SlipVia: cp.SlipVia, PayBy: cp.PayBy,
		SoldBy: cp.SoldByType, Notes: cp.Notes, CreatedAt: cp.CreatedAt,
	}
	if cp.ResourceID != nil {
		d.ResourceName = n.spaces[*cp.ResourceID]
	}
	d.ClassIDs, d.ClassNames = n.classList(cp.ClassIDs)
	d.CoachIDs, d.CoachNames = n.coachList(cp.CoachIDs)
	return d
}
