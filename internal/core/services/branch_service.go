package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	brandColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	languages         = map[string]bool{"en": true, "lo": true, "th": true}
	bookingModes      = map[string]bool{domain.BookingModeAccount: true, domain.BookingModeGuest: true, domain.BookingModeBoth: true}
)

const (
	maxSocialLinks       = 10
	maxDescriptionChars  = 2000
	maxBookingPolicyChar = 5000
)

type BranchServiceImpl struct {
	branches  output.BranchRepository
	sports    output.SportRepository
	resources output.ResourceRepository
	hours     output.OpeningHourRepository
	prices    output.PriceRepository
	files     mediaFiles
	now       func() time.Time
}

func NewBranchService(
	branches output.BranchRepository,
	sports output.SportRepository,
	resources output.ResourceRepository,
	hours output.OpeningHourRepository,
	prices output.PriceRepository,
	storage output.FileStorage,
	media output.MediaRepository,
) input.BranchService {
	return &BranchServiceImpl{
		branches: branches, sports: sports, resources: resources, hours: hours, prices: prices,
		files: mediaFiles{storage: storage, media: media}, now: time.Now,
	}
}

// ---------- settings ----------

func (s *BranchServiceImpl) toSettingsDTO(b *domain.Branch) *dto.SettingsDTO {
	return &dto.SettingsDTO{
		SettingsRequest: dto.SettingsRequest{
			Name: b.Name, Slug: b.Slug, CoverKey: b.CoverKey, Description: b.Description,
			Address: b.Address, Province: b.Province, Phone: b.Phone, MapURL: b.MapURL, SocialLinks: b.SocialLinks,
			Timezone: b.OwnTimezone, BookingMode: b.OwnBookingMode, BookingPolicy: b.OwnBookingPolicy,
			RequireApproval: b.OwnRequireApproval,
			RingColors:      b.OwnRingColors, HeadingFont: b.OwnHeadingFont, BodyFont: b.OwnBodyFont,
		},
		ID: b.ID, Status: b.Status(), CoverURL: s.files.url(b.ShownCoverKey), CompanyName: b.CompanyName,
		InForce: dto.InForce{
			Timezone: b.Timezone, Currency: b.Currency, Language: b.Language,
			BookingMode: b.BookingMode, BookingPolicy: b.BookingPolicy, RequireApproval: b.RequireApproval,
			RingColors: ringColorsOrEmpty(b.RingColors), HeadingFont: b.HeadingFont, BodyFont: b.BodyFont,
		},
		PublicPath: "/b/" + b.Slug, PublishedAt: b.PublishedAt,
	}
}

func (s *BranchServiceImpl) GetSettings(ctx context.Context, branchID uint) (*dto.SettingsDTO, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	return s.toSettingsDTO(b), nil
}

// validLink accepts empty or an absolute http(s) URL
func validLink(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func validSocialLinks(links map[string]string) error {
	if len(links) > maxSocialLinks {
		return badRequest("TOO_MANY_SOCIAL_LINKS")
	}
	for name, link := range links {
		if name == "" || len(name) > 30 || !validLink(link) || link == "" {
			return badRequest("INVALID_SOCIAL_LINK")
		}
	}
	return nil
}

func validTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil && tz != ""
}

func (s *BranchServiceImpl) validateSettings(ctx context.Context, b *domain.Branch, req *dto.SettingsRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	switch {
	case req.Name == "":
		return badRequest("NAME_IS_REQUIRED")
	case !domain.ValidProvince(req.Province):
		return badRequest("INVALID_PROVINCE")
	case !validLink(req.MapURL):
		return badRequest("INVALID_MAP_URL")
	case utf8.RuneCountInString(req.Description) > maxDescriptionChars:
		return badRequest("DESCRIPTION_TOO_LONG")
	case req.Timezone != nil && !validTimezone(*req.Timezone):
		return badRequest("INVALID_TIMEZONE")
	case req.BookingMode != nil && !bookingModes[*req.BookingMode]:
		return badRequest("INVALID_BOOKING_MODE")
	case req.BookingPolicy != nil && utf8.RuneCountInString(*req.BookingPolicy) > maxBookingPolicyChar:
		return badRequest("BOOKING_POLICY_TOO_LONG")
	case !validBranchLook(req):
		return badRequest("INVALID_LOOK")
	}
	if err := validSocialLinks(req.SocialLinks); err != nil {
		return err
	}
	if req.Slug != b.Slug {
		if b.PublishedAt != nil {
			return conflict("SLUG_LOCKED_AFTER_PUBLISH")
		}
		if !validSlug(req.Slug) {
			return badRequest("INVALID_SLUG")
		}
		taken, err := s.branches.SlugTaken(ctx, req.Slug, b.ID)
		if err != nil {
			return err
		}
		if taken {
			return conflict("SLUG_TAKEN")
		}
	}
	return s.files.requireOwned(ctx, b.ID, req.CoverKey)
}

// UpdateSettings replaces the branch's own settings at once (like PUT elsewhere)
func (s *BranchServiceImpl) UpdateSettings(ctx context.Context, branchID uint, req dto.SettingsRequest) (*dto.SettingsDTO, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSettings(ctx, b, &req); err != nil {
		return nil, err
	}
	b.Name, b.Slug, b.CoverKey, b.Description = req.Name, req.Slug, req.CoverKey, req.Description
	b.Address, b.Province, b.Phone, b.MapURL = strings.TrimSpace(req.Address), req.Province, strings.TrimSpace(req.Phone), req.MapURL
	b.SocialLinks = req.SocialLinks
	b.OwnTimezone, b.OwnBookingMode, b.OwnBookingPolicy, b.OwnRequireApproval = req.Timezone, req.BookingMode, req.BookingPolicy, req.RequireApproval
	b.OwnRingColors, b.OwnHeadingFont, b.OwnBodyFont = req.RingColors, req.HeadingFont, req.BodyFont
	if err := s.branches.Update(ctx, b); err != nil {
		return nil, err
	}
	return s.GetSettings(ctx, branchID)
}

// bookable reports whether at least one active resource of an active sport has hours and a price
func (s *BranchServiceImpl) bookable(ctx context.Context, branchID uint) (bool, error) {
	resources, _, err := s.resources.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"active": true}})
	if err != nil {
		return false, err
	}
	for _, r := range resources {
		sport, err := s.sports.GetByID(ctx, branchID, r.SportID)
		if err != nil || !sport.Active {
			continue
		}
		hours, err := s.hours.ListByOwner(ctx, branchID, domain.OwnerResource, r.ID)
		if err != nil {
			return false, err
		}
		if len(hours) == 0 {
			continue
		}
		if _, err := s.prices.GetByOwner(ctx, branchID, domain.OwnerResource, r.ID); err == nil {
			return true, nil
		} else if !isNotFound(err) {
			return false, err
		}
	}
	return false, nil
}

// Publish makes the public page live once something can actually be booked
func (s *BranchServiceImpl) Publish(ctx context.Context, branchID uint) (*dto.SettingsDTO, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if b.Closed {
		return nil, conflict("BRANCH_CLOSED")
	}
	ok, err := s.bookable(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, badRequest("PUBLISH_NEEDS_ACTIVE_RESOURCE_WITH_HOURS_AND_PRICE")
	}
	b.Published = true
	if b.PublishedAt == nil {
		now := s.now()
		b.PublishedAt = &now
	}
	if err := s.branches.Update(ctx, b); err != nil {
		return nil, err
	}
	return s.toSettingsDTO(b), nil
}

func (s *BranchServiceImpl) Unpublish(ctx context.Context, branchID uint) (*dto.SettingsDTO, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, err
	}
	b.Published = false
	if err := s.branches.Update(ctx, b); err != nil {
		return nil, err
	}
	return s.toSettingsDTO(b), nil
}

// ringColorsOrEmpty gives [] instead of null in the JSON
func ringColorsOrEmpty(colors []string) []string {
	if colors == nil {
		return []string{}
	}
	return colors
}

// validLook checks ring colours (up to MaxRingColors, #rrggbb) and the two font keys ("" = default)
func validLook(rings []string, heading, body string) bool {
	if len(rings) > domain.MaxRingColors {
		return false
	}
	for _, c := range rings {
		if !brandColorPattern.MatchString(c) {
			return false
		}
	}
	return (heading == "" || domain.Fonts[heading]) && (body == "" || domain.Fonts[body])
}

func validBranchLook(req *dto.SettingsRequest) bool {
	var rings []string
	if req.RingColors != nil {
		rings = *req.RingColors
	}
	val := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return validLook(rings, val(req.HeadingFont), val(req.BodyFont))
}
