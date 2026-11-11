package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/modules/company/port"
	"backend/internal/platform/kit"
	"context"
	"sort"
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
	hours     output.OpeningHourRepository
	files     kit.MediaFiles
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
	hours output.OpeningHourRepository,
	storage output.FileStorage,
) port.PublicService {
	return &PublicServiceImpl{
		company: company, branches: branches, directory: directory, sports: sports, resources: resources, coaches: coaches, classes: classes, bookings: bookings,
		prices: prices, hours: hours, files: kit.MediaFiles{Storage: storage}, now: time.Now,
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
		Type: b.Type, Sister: s.sister(ctx, b), Name: b.Name, Slug: b.Slug, CompanyName: b.CompanyName, Timezone: b.Timezone, Currency: b.Currency, Language: b.Language,
		LogoURL: s.files.URL(b.LogoKey), CoverURL: s.files.URL(b.ShownCoverKey), BrandColor: b.BrandColor,
		RingColors: ringColorsOrEmpty(b.RingColors), HeadingFont: b.HeadingFont, BodyFont: b.BodyFont,
		Description: b.Description, Address: b.Address, Province: b.Province, Phone: b.Phone, MapURL: b.MapURL,
		SocialLinks: b.SocialLinks, BookingPolicy: b.BookingPolicy, BookingMode: b.BookingMode,
		RequireApproval: b.RequireApproval,
	}, nil
}

// sister is the "Also at this place" branch, when it is live too
func (s *PublicServiceImpl) sister(ctx context.Context, b *domain.Branch) *dto.SisterBranchDTO {
	if b.SisterBranchID == nil {
		return nil
	}
	o, err := s.branches.GetByID(ctx, *b.SisterBranchID)
	if err != nil || !o.IsPublic() {
		return nil
	}
	return &dto.SisterBranchDTO{Type: o.Type, Name: o.Name, Slug: o.Slug, CoverURL: s.files.URL(o.ShownCoverKey)}
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
			ImageKey: m.ImageKey, ImageURL: s.files.URL(m.ImageKey), Active: m.Active, Type: m.Type,
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
			CoachIDs: idList(r.CoachIDs),
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
		// a coach closed every day (no opening hours) cannot be booked by the hour: still listed (their name shows on the
		// classes they lead), but not offered as a coach to book
		hours, err := s.hours.ListByOwner(ctx, b.ID, domain.OwnerCoach, c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.PublicCoachDTO{
			ID: c.ID, Name: c.Name, Bio: c.Bio, ImageURL: s.files.URL(c.ImageKey), Sports: sportRefs(c.Sports),
			PricePerHour: price, Currency: currency, Bookable: len(hours) > 0, Hours: weekHours(hours),
		})
	}
	return out, nil
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
		Name: c.Name, LogoURL: s.files.URL(c.LogoKey), CoverURL: s.files.URL(c.CoverKey), BrandColor: c.BrandColor,
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
	page, limit := kit.NormalizePage(dto.ListQuery{Page: q.Page, Limit: q.Limit})
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
			Type: b.Type, Name: b.Name, Slug: b.Slug, CoverURL: s.files.URL(b.ShownCoverKey),
			Address: b.Address, Province: b.Province, Currency: b.Currency,
			Sports: mapSlice(sports[b.ID], func(sp domain.Sport) dto.DirectorySport {
				return dto.DirectorySport{Name: sp.Name, Type: sp.Type}
			}),
		}
		if p, ok := prices[b.ID]; ok {
			d.FromPrice, d.Currency = &p.PricePerHour, p.Currency
		}
		if b.Type == domain.BranchTypeRestaurant { // its "sports" are areas (Indoor, Terrace), not sports
			d.Sports = []dto.DirectorySport{}
			d.FromPrice = nil
		}
		return d
	}), nil
}

// idList reads an id list kept as text, never nil
func idList(text string) []uint {
	ids := domain.IDList(text)
	if ids == nil {
		return []uint{}
	}
	return ids
}

// weekHours lists a coach's weekly hours for customers, by day then time
func weekHours(rows []domain.OpeningHour) []dto.OpeningHourDTO {
	out := make([]dto.OpeningHourDTO, 0, len(rows))
	for _, h := range rows {
		out = append(out, dto.OpeningHourDTO{Weekday: h.Weekday, Open: h.OpenTime, Close: h.CloseTime})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Weekday != out[j].Weekday {
			return out[i].Weekday < out[j].Weekday
		}
		return out[i].Open < out[j].Open
	})
	return out
}
