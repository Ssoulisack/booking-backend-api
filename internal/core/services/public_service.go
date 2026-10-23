package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"context"
	"strings"
	"time"
)

type PublicServiceImpl struct {
	company   output.CompanyRepository
	branches  output.BranchRepository
	directory output.DirectoryRepository
	sports    output.SportRepository
	resources output.ResourceRepository
	coaches   output.CoachRepository
	classes   output.ClassSessionRepository
	bookings  output.BookingRepository
	prices    output.PriceRepository
	files     mediaFiles
	now       func() time.Time
}

func NewPublicService(
	company output.CompanyRepository,
	branches output.BranchRepository,
	directory output.DirectoryRepository,
	sports output.SportRepository,
	resources output.ResourceRepository,
	coaches output.CoachRepository,
	classes output.ClassSessionRepository,
	bookings output.BookingRepository,
	prices output.PriceRepository,
	storage output.FileStorage,
) input.PublicService {
	return &PublicServiceImpl{
		company: company, branches: branches, directory: directory, sports: sports, resources: resources, coaches: coaches, classes: classes, bookings: bookings,
		prices: prices, files: mediaFiles{storage: storage}, now: time.Now,
	}
}

// published loads a branch that may be shown publicly. Drafts, closed and unknown
// slugs give the same 404, so the page does not reveal that a branch exists.
func (s *PublicServiceImpl) published(ctx context.Context, slug string) (*domain.Branch, error) {
	return loadPublished(ctx, s.branches, slug)
}

func loadPublished(ctx context.Context, branches output.BranchRepository, slug string) (*domain.Branch, error) {
	b, err := branches.GetBySlug(ctx, slug)
	if err != nil {
		if isNotFound(err) {
			return nil, notFound("BRANCH_NOT_FOUND")
		}
		return nil, err
	}
	if !b.IsPublic() {
		return nil, notFound("BRANCH_NOT_FOUND")
	}
	return b, nil
}

// activeOnly lists every active item; a branch has few of each
var activeOnly = output.ListQuery{Filters: map[string]any{"active": true}}

func (s *PublicServiceImpl) Branch(ctx context.Context, slug string) (*dto.PublicBranchDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &dto.PublicBranchDTO{
		Name: b.Name, Slug: b.Slug, CompanyName: b.CompanyName, Timezone: b.Timezone, Currency: b.Currency, Language: b.Language,
		LogoURL: s.files.url(b.LogoKey), CoverURL: s.files.url(b.ShownCoverKey), BrandColor: b.BrandColor,
		RingColors: ringColorsOrEmpty(b.RingColors), HeadingFont: b.HeadingFont, BodyFont: b.BodyFont,
		Description: b.Description, Address: b.Address, Province: b.Province, Phone: b.Phone, MapURL: b.MapURL,
		SocialLinks: b.SocialLinks, BookingPolicy: b.BookingPolicy, BookingMode: b.BookingMode,
		RequireApproval: b.RequireApproval,
	}, nil
}

func (s *PublicServiceImpl) Sports(ctx context.Context, slug string) ([]dto.SportDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	items, _, err := s.sports.List(ctx, b.ID, activeOnly)
	if err != nil {
		return nil, err
	}
	return mapSlice(items, func(m domain.Sport) dto.SportDTO {
		return dto.SportDTO{
			ID: m.ID, Name: m.Name, Slug: m.Slug, Description: m.Description,
			ImageKey: m.ImageKey, ImageURL: s.files.url(m.ImageKey), Active: m.Active, Type: m.Type,
		}
	}), nil
}

// hourly is the owner's price per hour, or nil when none is set yet
func (s *PublicServiceImpl) hourly(ctx context.Context, branchID uint, ownerType string, ownerID uint) (*int64, string, error) {
	p, err := s.prices.GetByOwner(ctx, branchID, ownerType, ownerID)
	if err != nil {
		if isNotFound(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	return &p.PricePerHour, p.Currency, nil
}

func (s *PublicServiceImpl) Resources(ctx context.Context, slug string, sportID uint) ([]dto.PublicResourceDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	q := output.ListQuery{Filters: map[string]any{"active": true}}
	if sportID != 0 {
		q.Filters["sport_id"] = sportID
	}
	items, _, err := s.resources.List(ctx, b.ID, q)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PublicResourceDTO, 0, len(items))
	for _, r := range items {
		price, currency, err := s.hourly(ctx, b.ID, domain.OwnerResource, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.PublicResourceDTO{
			ID: r.ID, SportID: r.SportID, Name: r.Name, Kind: r.Kind, PricePerHour: price, Currency: currency,
		})
	}
	return out, nil
}

func (s *PublicServiceImpl) Coaches(ctx context.Context, slug string, sportID uint) ([]dto.PublicCoachDTO, error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	items, _, err := s.coaches.List(ctx, b.ID, activeOnly)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PublicCoachDTO, 0, len(items))
	for _, c := range items {
		if sportID != 0 && !teaches(c, sportID) {
			continue
		}
		price, currency, err := s.hourly(ctx, b.ID, domain.OwnerCoach, c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.PublicCoachDTO{
			ID: c.ID, Name: c.Name, Bio: c.Bio, ImageURL: s.files.url(c.ImageKey), Sports: sportRefs(c.Sports),
			PricePerHour: price, Currency: currency,
		})
	}
	return out, nil
}

func teaches(c domain.Coach, sportID uint) bool {
	for _, sp := range c.Sports {
		if sp.ID == sportID {
			return true
		}
	}
	return false
}

// ClassSessions lists upcoming active sessions, soonest first, with the seats still free.
// ?date= or ?from=&to= (branch timezone) limit the days; sessions that already started are never listed.
func (s *PublicServiceImpl) ClassSessions(ctx context.Context, slug string, q dto.ListQuery) (*dto.PagedResult[dto.ClassSessionDTO], error) {
	b, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	f, err := classFilter(q, locationOf(b))
	if err != nil {
		return nil, err
	}
	now := s.now()
	if f.From == nil || f.From.Before(now) {
		f.From = &now
	}
	f.Active, f.ActiveOnly = nil, true
	lq, page, limit := newListQuery(q)
	lq.Keyword = ""
	items, total, err := s.classes.ListFiltered(ctx, b.ID, f, lq)
	if err != nil {
		return nil, err
	}
	ids := mapSlice(items, func(c domain.ClassSession) uint { return c.ID })
	taken, err := s.bookings.SeatsTaken(ctx, b.ID, ids)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, func(c domain.ClassSession) dto.ClassSessionDTO {
		d := toClassSessionDTO(c)
		left := max(c.Capacity-taken[c.ID], 0)
		d.SeatsLeft = &left
		return d
	}), nil
}

// Company is the brand shown on the public home
func (s *PublicServiceImpl) Company(ctx context.Context) (*dto.PublicCompanyDTO, error) {
	c, err := s.company.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.PublicCompanyDTO{
		Name: c.Name, LogoURL: s.files.url(c.LogoKey), CoverURL: s.files.url(c.CoverKey), BrandColor: c.BrandColor,
		RingColors: ringColorsOrEmpty(c.RingColors), HeadingFont: c.HeadingFont, BodyFont: c.BodyFont,
		Description: c.Description, Phone: c.Phone, Email: c.Email, SocialLinks: c.SocialLinks,
		Language: c.Language, Currency: c.Currency,
	}, nil
}

// Directory lists the live branches for the public home, filtered by name or address, sport
// type and province, each with its active sports and lowest court price
func (s *PublicServiceImpl) Directory(ctx context.Context, q dto.DirectoryQuery) (*dto.PagedResult[dto.DirectoryBranchDTO], error) {
	if q.SportType != "" && !domain.ValidSportType(q.SportType) {
		return nil, badRequest("INVALID_SPORT_TYPE")
	}
	if !domain.ValidProvince(q.Province) {
		return nil, badRequest("INVALID_PROVINCE")
	}
	page, limit := normalizePage(dto.ListQuery{Page: q.Page, Limit: q.Limit})
	items, total, err := s.directory.Branches(ctx, output.DirectoryQuery{
		Page: page, Limit: limit, Keyword: strings.TrimSpace(q.Keyword), SportType: q.SportType, Province: q.Province,
	})
	if err != nil {
		return nil, err
	}
	ids := mapSlice(items, func(b domain.Branch) uint { return b.ID })
	sports, err := s.directory.ActiveSports(ctx, ids)
	if err != nil {
		return nil, err
	}
	prices, err := s.directory.LowestCourtPrices(ctx, ids)
	if err != nil {
		return nil, err
	}
	company, err := s.company.Get(ctx)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, func(b domain.Branch) dto.DirectoryBranchDTO {
		b.WithCompany(*company)
		d := dto.DirectoryBranchDTO{
			Name: b.Name, Slug: b.Slug, CoverURL: s.files.url(b.ShownCoverKey),
			Address: b.Address, Province: b.Province, Currency: b.Currency,
			Sports: mapSlice(sports[b.ID], func(sp domain.Sport) dto.DirectorySport {
				return dto.DirectorySport{Name: sp.Name, Type: sp.Type}
			}),
		}
		if p, ok := prices[b.ID]; ok {
			d.FromPrice, d.Currency = &p.PricePerHour, p.Currency
		}
		return d
	}), nil
}
