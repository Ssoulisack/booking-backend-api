package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"context"
	"log"
	"strings"
	"time"
)

type CatalogServiceImpl struct {
	sports    output.SportRepository
	resources output.ResourceRepository
	coaches   output.CoachRepository
	classes   output.ClassSessionRepository
	bookings  output.BookingRepository
	files     mediaFiles
}

func NewCatalogService(
	sports output.SportRepository,
	resources output.ResourceRepository,
	coaches output.CoachRepository,
	classes output.ClassSessionRepository,
	bookings output.BookingRepository,
	storage output.FileStorage,
	media output.MediaRepository,
) input.CatalogService {
	return &CatalogServiceImpl{
		sports: sports, resources: resources, coaches: coaches, classes: classes, bookings: bookings,
		files: mediaFiles{storage: storage, media: media},
	}
}

var resourceKinds = map[string]bool{
	domain.ResourceKindCourt: true,
	domain.ResourceKindPitch: true,
	domain.ResourceKindRing:  true,
	domain.ResourceKindLane:  true,
	domain.ResourceKindBay:   true,
	domain.ResourceKindTable: true,
	domain.ResourceKindRoom:  true,
	domain.ResourceKindOther: true,
}

// ---------- sports ----------

func (s *CatalogServiceImpl) toSportDTO(m domain.Sport) dto.SportDTO {
	return dto.SportDTO{
		ID: m.ID, Name: m.Name, Slug: m.Slug, Description: m.Description,
		ImageKey: m.ImageKey, ImageURL: s.files.url(m.ImageKey), Active: m.Active, Type: m.Type,
	}
}

func (s *CatalogServiceImpl) validateSport(ctx context.Context, branchID uint, req *dto.SportRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("NAME_IS_REQUIRED")
	}
	if req.Type == "" {
		req.Type = domain.SportTypeOther
	}
	if !domain.ValidSportType(req.Type) {
		return badRequest("INVALID_SPORT_TYPE")
	}
	return s.files.requireOwned(ctx, branchID, req.ImageKey)
}

func (s *CatalogServiceImpl) CreateSport(ctx context.Context, branchID uint, req dto.SportRequest) (*dto.SportDTO, error) {
	if err := s.validateSport(ctx, branchID, &req); err != nil {
		return nil, err
	}
	slug := slugify(req.Name, "sport")
	taken, err := s.sports.Count(ctx, branchID, map[string]any{"slug": slug})
	if err != nil {
		return nil, err
	}
	if taken > 0 {
		return nil, conflict("SPORT_ALREADY_EXISTS")
	}
	m := domain.Sport{
		BranchID: branchID, Name: req.Name, Slug: slug, Description: req.Description,
		ImageKey: req.ImageKey, Active: isActive(req.Active), Type: req.Type,
	}
	if err := s.sports.Create(ctx, &m); err != nil {
		return nil, err
	}
	d := s.toSportDTO(m)
	return &d, nil
}

func (s *CatalogServiceImpl) GetSport(ctx context.Context, branchID, id uint) (*dto.SportDTO, error) {
	m, err := s.sports.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	d := s.toSportDTO(*m)
	return &d, nil
}

func (s *CatalogServiceImpl) ListSports(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.SportDTO], error) {
	lq, page, limit := newListQuery(q)
	if q.Active != nil {
		lq.Filters["active"] = *q.Active
	}
	items, total, err := s.sports.List(ctx, branchID, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, s.toSportDTO), nil
}

// UpdateSport keeps the slug stable so public URLs do not change when a sport is renamed
func (s *CatalogServiceImpl) UpdateSport(ctx context.Context, branchID, id uint, req dto.SportRequest) (*dto.SportDTO, error) {
	if err := s.validateSport(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m, err := s.sports.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	oldKey := m.ImageKey
	m.Name, m.Description, m.ImageKey, m.Active = req.Name, req.Description, req.ImageKey, isActive(req.Active)
	m.Type = req.Type
	if err := s.sports.Update(ctx, m); err != nil {
		return nil, err
	}
	s.files.replaced(ctx, branchID, oldKey, m.ImageKey)
	d := s.toSportDTO(*m)
	return &d, nil
}

func (s *CatalogServiceImpl) DeleteSport(ctx context.Context, branchID, id uint) error {
	m, err := s.sports.GetByID(ctx, branchID, id)
	if err != nil {
		return err
	}
	inUse, err := s.resources.Count(ctx, branchID, map[string]any{"sport_id": id})
	if err != nil {
		return err
	}
	if inUse > 0 {
		return conflict("SPORT_HAS_RESOURCES")
	}
	if err := s.refuseIfUsedByClasses(ctx, branchID, "sport_id", id, "SPORT_HAS_CLASS_SESSIONS"); err != nil {
		return err
	}
	if err := s.sports.Delete(ctx, branchID, id); err != nil {
		return err
	}
	s.files.replaced(ctx, branchID, m.ImageKey, "")
	return nil
}

// ---------- resources ----------

func toResourceDTO(m domain.Resource) dto.ResourceDTO {
	return dto.ResourceDTO{ID: m.ID, SportID: m.SportID, Name: m.Name, Kind: m.Kind, Active: m.Active}
}

func (s *CatalogServiceImpl) validateResource(ctx context.Context, branchID uint, req *dto.ResourceRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("NAME_IS_REQUIRED")
	}
	if !resourceKinds[req.Kind] {
		return badRequest("INVALID_RESOURCE_KIND")
	}
	_, err := s.sports.GetByID(ctx, branchID, req.SportID)
	return orBadRequest(err, "SPORT_NOT_FOUND")
}

func (s *CatalogServiceImpl) CreateResource(ctx context.Context, branchID uint, req dto.ResourceRequest) (*dto.ResourceDTO, error) {
	if err := s.validateResource(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m := domain.Resource{BranchID: branchID, SportID: req.SportID, Name: req.Name, Kind: req.Kind, Active: isActive(req.Active)}
	if err := s.resources.Create(ctx, &m); err != nil {
		return nil, err
	}
	d := toResourceDTO(m)
	return &d, nil
}

func (s *CatalogServiceImpl) GetResource(ctx context.Context, branchID, id uint) (*dto.ResourceDTO, error) {
	m, err := s.resources.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	d := toResourceDTO(*m)
	return &d, nil
}

func (s *CatalogServiceImpl) ListResources(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.ResourceDTO], error) {
	lq, page, limit := newListQuery(q)
	if q.SportID != 0 {
		lq.Filters["sport_id"] = q.SportID
	}
	if q.Active != nil {
		lq.Filters["active"] = *q.Active
	}
	items, total, err := s.resources.List(ctx, branchID, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, toResourceDTO), nil
}

func (s *CatalogServiceImpl) UpdateResource(ctx context.Context, branchID, id uint, req dto.ResourceRequest) (*dto.ResourceDTO, error) {
	if err := s.validateResource(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m, err := s.resources.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	m.SportID, m.Name, m.Kind, m.Active = req.SportID, req.Name, req.Kind, isActive(req.Active)
	if err := s.resources.Update(ctx, m); err != nil {
		return nil, err
	}
	d := toResourceDTO(*m)
	return &d, nil
}

// DeleteResource refuses a resource that classes use or that has upcoming bookings
func (s *CatalogServiceImpl) DeleteResource(ctx context.Context, branchID, id uint) error {
	if err := s.refuseIfInUse(ctx, branchID, domain.OwnerResource, id, "resource_id"); err != nil {
		return err
	}
	return s.resources.Delete(ctx, branchID, id)
}

// ---------- coaches ----------

func (s *CatalogServiceImpl) toCoachDTO(m domain.Coach) dto.CoachDTO {
	return dto.CoachDTO{
		ID: m.ID, Name: m.Name, Bio: m.Bio, Phone: m.Phone, ImageKey: m.ImageKey,
		ImageURL: s.files.url(m.ImageKey), Active: m.Active, Sports: sportRefs(m.Sports),
	}
}

func sportRefs(sports []domain.Sport) []dto.SportRefDTO {
	return mapSlice(sports, func(sp domain.Sport) dto.SportRefDTO { return dto.SportRefDTO{ID: sp.ID, Name: sp.Name} })
}

func (s *CatalogServiceImpl) validateCoach(ctx context.Context, branchID uint, req *dto.CoachRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("NAME_IS_REQUIRED")
	}
	return s.files.requireOwned(ctx, branchID, req.ImageKey)
}

func (s *CatalogServiceImpl) CreateCoach(ctx context.Context, branchID uint, req dto.CoachRequest) (*dto.CoachDTO, error) {
	if err := s.validateCoach(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m := domain.Coach{
		BranchID: branchID, Name: req.Name, Bio: req.Bio, Phone: strings.TrimSpace(req.Phone),
		ImageKey: req.ImageKey, Active: isActive(req.Active),
	}
	if err := s.coaches.Create(ctx, &m); err != nil {
		return nil, err
	}
	if err := s.coaches.SetSports(ctx, branchID, m.ID, req.SportIDs); err != nil {
		// do not leave a coach whose sports could not be saved
		if delErr := s.coaches.Delete(ctx, branchID, m.ID); delErr != nil {
			log.Printf("❌ could not roll back coach %d: %v", m.ID, delErr)
		}
		return nil, err
	}
	return s.GetCoach(ctx, branchID, m.ID)
}

func (s *CatalogServiceImpl) GetCoach(ctx context.Context, branchID, id uint) (*dto.CoachDTO, error) {
	m, err := s.coaches.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	d := s.toCoachDTO(*m)
	return &d, nil
}

func (s *CatalogServiceImpl) ListCoaches(ctx context.Context, branchID uint, q dto.ListQuery) (*dto.PagedResult[dto.CoachDTO], error) {
	lq, page, limit := newListQuery(q)
	if q.Active != nil {
		lq.Filters["active"] = *q.Active
	}
	items, total, err := s.coaches.List(ctx, branchID, lq)
	if err != nil {
		return nil, err
	}
	return pageOf(items, total, page, limit, s.toCoachDTO), nil
}

func (s *CatalogServiceImpl) UpdateCoach(ctx context.Context, branchID, id uint, req dto.CoachRequest) (*dto.CoachDTO, error) {
	if err := s.validateCoach(ctx, branchID, &req); err != nil {
		return nil, err
	}
	m, err := s.coaches.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	// check the sports first so a bad id does not leave a half-updated coach
	if err := s.coaches.SetSports(ctx, branchID, id, req.SportIDs); err != nil {
		return nil, err
	}
	oldKey := m.ImageKey
	m.Name, m.Bio, m.Phone = req.Name, req.Bio, strings.TrimSpace(req.Phone)
	m.ImageKey, m.Active = req.ImageKey, isActive(req.Active)
	m.Sports = nil // SetSports owns the association
	if err := s.coaches.Update(ctx, m); err != nil {
		return nil, err
	}
	s.files.replaced(ctx, branchID, oldKey, m.ImageKey)
	return s.GetCoach(ctx, branchID, id)
}

// DeleteCoach refuses a coach who teaches classes or has upcoming bookings
func (s *CatalogServiceImpl) DeleteCoach(ctx context.Context, branchID, id uint) error {
	if err := s.refuseIfInUse(ctx, branchID, domain.OwnerCoach, id, "coach_id"); err != nil {
		return err
	}
	m, err := s.coaches.GetByID(ctx, branchID, id)
	if err != nil {
		return err
	}
	if err := s.coaches.Delete(ctx, branchID, id); err != nil {
		return err
	}
	s.files.replaced(ctx, branchID, m.ImageKey, "")
	return nil
}

// refuseIfUsedByClasses stops deleting something class sessions still point to
func (s *CatalogServiceImpl) refuseIfUsedByClasses(ctx context.Context, branchID uint, column string, id uint, code string) error {
	n, err := s.classes.Count(ctx, branchID, map[string]any{column: id})
	if err != nil {
		return err
	}
	if n > 0 {
		return conflict(code)
	}
	return nil
}

// refuseIfInUse stops deleting a resource or coach used by classes or by upcoming bookings
func (s *CatalogServiceImpl) refuseIfInUse(ctx context.Context, branchID uint, ownerType string, id uint, column string) error {
	if err := s.refuseIfUsedByClasses(ctx, branchID, column, id, "USED_BY_CLASS_SESSIONS"); err != nil {
		return err
	}
	n, err := s.bookings.CountFutureForOwner(ctx, branchID, ownerType, id, time.Now())
	if err != nil {
		return err
	}
	if n > 0 {
		return conflict("HAS_UPCOMING_BOOKINGS")
	}
	return nil
}
