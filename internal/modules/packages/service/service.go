package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	pkgdomain "backend/internal/modules/packages/domain"
	"backend/internal/modules/packages/dto"
	"backend/internal/modules/packages/port"
	"backend/pkg/response"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

func badRequest(code string) error { return response.ErrorBadRequest(code) }
func conflict(code string) error   { return response.NewError(http.StatusConflict, code) }
func notFound(code string) error   { return response.NewError(http.StatusNotFound, code) }

const (
	maxPrice   = 100_000_000_000
	maxOptions = 12
	maxTimes   = 1000
	maxMonths  = 36
	maxDays    = 3650
)

// Deps are what the service is built from
type Deps struct {
	Repo      port.Repository
	Branches  output.BranchRepository
	Sports    output.SportRepository
	Resources output.ResourceRepository
	Users     output.UserRepository
	// Now: the clock (nil: the real one)
	Now func() time.Time
}

// Service holds the packages rules (it implements port.Service)
type Service struct {
	Deps
	// Ways and Sender: how a customer may send a payment slip, and the Telegram group that receives it (nil: neither)
	Ways   port.SlipWays
	Sender port.SlipSender
	// News tells the team (Telegram) when a customer buys a package or pays it online (nil: nobody)
	News output.TeamNews
}

// SetNews gives the service a way to tell the branch's team about package sales
func (s *Service) SetNews(n output.TeamNews) { s.News = n }

// SetSlips tells the service how to read the branch's slip ways and where uploaded slips go
func (s *Service) SetSlips(w port.SlipWays, sender port.SlipSender) { s.Ways, s.Sender = w, sender }

var _ port.Service = (*Service)(nil)

// New builds the service
func New(d Deps) *Service { return &Service{Deps: d} }

func mapNotFound(err error, code string) error {
	if err == nil {
		return nil
	}
	up := strings.ToUpper(err.Error())
	if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(up, "NOT_FOUND") || strings.Contains(up, "DATA NOT FOUND") {
		return notFound(code)
	}
	return err
}

// branch: packages are sold by sport branches only
func (s *Service) branch(ctx context.Context, id uint) (*domain.Branch, error) {
	b, err := s.Branches.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	if b.Type != domain.BranchTypeSport {
		return nil, conflict("NOT_A_SPORT_BRANCH")
	}
	return b, nil
}

// ---------- staff ----------

func (s *Service) List(ctx context.Context, branchID uint) ([]dto.PackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return s.list(ctx, b, false)
}

func (s *Service) Get(ctx context.Context, branchID, id uint) (*dto.PackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	p, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	return s.one(ctx, b, p)
}

func (s *Service) Create(ctx context.Context, branchID uint, req dto.PackageRequest) (*dto.PackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	p := &pkgdomain.Package{BranchID: branchID, Active: true}
	return s.save(ctx, b, p, req)
}

func (s *Service) Update(ctx context.Context, branchID, id uint, req dto.PackageRequest) (*dto.PackageDTO, error) {
	b, err := s.branch(ctx, branchID)
	if err != nil {
		return nil, err
	}
	p, err := s.Repo.Get(ctx, branchID, id)
	if err != nil {
		return nil, mapNotFound(err, "PACKAGE_NOT_FOUND")
	}
	return s.save(ctx, b, p, req)
}

func (s *Service) Delete(ctx context.Context, branchID, id uint) error {
	if _, err := s.branch(ctx, branchID); err != nil {
		return err
	}
	return mapNotFound(s.Repo.Delete(ctx, branchID, id), "PACKAGE_NOT_FOUND")
}

// save checks the request, then stores the package with its options
func (s *Service) save(ctx context.Context, b *domain.Branch, p *pkgdomain.Package, req dto.PackageRequest) (*dto.PackageDTO, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return nil, badRequest("INVALID_NAME")
	}
	if req.Covers != pkgdomain.CoversClass && req.Covers != pkgdomain.CoversSpace {
		return nil, badRequest("INVALID_COVERS")
	}
	if _, err := s.Sports.GetByID(ctx, b.ID, req.SportID); err != nil {
		return nil, mapNotFound(err, "SPORT_NOT_FOUND")
	}
	var resourceID *uint
	if req.Covers == pkgdomain.CoversSpace {
		if req.ResourceID == nil {
			return nil, badRequest("SPACE_REQUIRED")
		}
		res, err := s.Resources.GetByID(ctx, b.ID, *req.ResourceID)
		if err != nil {
			return nil, mapNotFound(err, "RESOURCE_NOT_FOUND")
		}
		if res.SportID != req.SportID {
			return nil, badRequest("SPACE_NOT_IN_SPORT")
		}
		resourceID = req.ResourceID
	}
	options, err := s.options(req.Options)
	if err != nil {
		return nil, err
	}
	p.SportID, p.Name, p.Description = req.SportID, name, strings.TrimSpace(req.Description)
	p.Covers, p.ResourceID = req.Covers, resourceID
	if req.Active != nil {
		p.Active = *req.Active
	}
	if err := s.Repo.Save(ctx, p, options); err != nil {
		return nil, mapNotFound(err, "OPTION_NOT_FOUND")
	}
	return s.one(ctx, b, p)
}

// options checks the options of a request: a kind, a sensible amount and price, no two the same
func (s *Service) options(in []dto.OptionRequest) ([]pkgdomain.Option, error) {
	if len(in) == 0 || len(in) > maxOptions {
		return nil, badRequest("INVALID_OPTIONS")
	}
	seen := map[[2]any]bool{}
	out := make([]pkgdomain.Option, 0, len(in))
	for _, o := range in {
		limit := maxTimes
		switch o.Kind {
		case pkgdomain.OptionTimes:
		case pkgdomain.OptionMonths:
			limit = maxMonths
			if o.ValidDays != 0 {
				return nil, badRequest("INVALID_VALID_DAYS")
			}
		default:
			return nil, badRequest("INVALID_OPTION_KIND")
		}
		if o.Amount < 1 || o.Amount > limit {
			return nil, badRequest("INVALID_OPTION_AMOUNT")
		}
		if o.Price < 1 || o.Price > maxPrice {
			return nil, badRequest("INVALID_PRICE")
		}
		if o.ValidDays < 0 || o.ValidDays > maxDays {
			return nil, badRequest("INVALID_VALID_DAYS")
		}
		key := [2]any{o.Kind, o.Amount}
		if seen[key] {
			return nil, badRequest("DUPLICATE_OPTION")
		}
		seen[key] = true
		active := true
		if o.Active != nil {
			active = *o.Active
		}
		out = append(out, pkgdomain.Option{
			Model: gorm.Model{ID: o.ID}, Kind: o.Kind, Amount: o.Amount, Price: o.Price, ValidDays: o.ValidDays, Active: active,
		})
	}
	return out, nil
}

// ---------- customers ----------

func (s *Service) Public(ctx context.Context, slug string) ([]dto.PackageDTO, error) {
	b, err := s.Branches.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapNotFound(err, "BRANCH_NOT_FOUND")
	}
	if !b.IsPublic() || b.Type != domain.BranchTypeSport {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	all, err := s.list(ctx, b, true)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PackageDTO, 0, len(all))
	for _, p := range all {
		if len(p.Options) > 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

// ---------- answers ----------

func (s *Service) list(ctx context.Context, b *domain.Branch, activeOnly bool) ([]dto.PackageDTO, error) {
	ps, err := s.Repo.List(ctx, b.ID, activeOnly)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	opts, err := s.Repo.Options(ctx, ids, activeOnly)
	if err != nil {
		return nil, err
	}
	byPackage := map[uint][]dto.OptionDTO{}
	for _, o := range opts {
		byPackage[o.PackageID] = append(byPackage[o.PackageID], optionDTO(o))
	}
	names, err := s.names(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PackageDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, packageDTO(p, b.Currency, names, byPackage[p.ID]))
	}
	return out, nil
}

func (s *Service) one(ctx context.Context, b *domain.Branch, p *pkgdomain.Package) (*dto.PackageDTO, error) {
	opts, err := s.Repo.Options(ctx, []uint{p.ID}, false)
	if err != nil {
		return nil, err
	}
	names, err := s.names(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	od := make([]dto.OptionDTO, 0, len(opts))
	for _, o := range opts {
		od = append(od, optionDTO(o))
	}
	d := packageDTO(*p, b.Currency, names, od)
	return &d, nil
}

// nameBook gives the sport and space names of a branch, for answers
type nameBook struct{ sports, spaces map[uint]string }

func (s *Service) names(ctx context.Context, branchID uint) (nameBook, error) {
	n := nameBook{sports: map[uint]string{}, spaces: map[uint]string{}}
	sports, _, err := s.Sports.List(ctx, branchID, output.ListQuery{Limit: 500})
	if err != nil {
		return n, err
	}
	for _, sp := range sports {
		n.sports[sp.ID] = sp.Name
	}
	res, _, err := s.Resources.List(ctx, branchID, output.ListQuery{Limit: 1000})
	if err != nil {
		return n, err
	}
	for _, r := range res {
		n.spaces[r.ID] = r.Name
	}
	return n, nil
}

func optionDTO(o pkgdomain.Option) dto.OptionDTO {
	return dto.OptionDTO{ID: o.ID, Kind: o.Kind, Amount: o.Amount, Price: o.Price, ValidDays: o.ValidDays, Active: o.Active}
}

func packageDTO(p pkgdomain.Package, currency string, n nameBook, opts []dto.OptionDTO) dto.PackageDTO {
	if opts == nil {
		opts = []dto.OptionDTO{}
	}
	d := dto.PackageDTO{
		ID: p.ID, SportID: p.SportID, SportName: n.sports[p.SportID], Name: p.Name, Description: p.Description,
		Covers: p.Covers, ResourceID: p.ResourceID, Active: p.Active, Currency: currency, Options: opts,
	}
	if p.ResourceID != nil {
		d.ResourceName = n.spaces[*p.ResourceID]
	}
	return d
}
