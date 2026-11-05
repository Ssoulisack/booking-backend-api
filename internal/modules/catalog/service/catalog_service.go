package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/output"
	"backend/internal/modules/catalog/port"
	"backend/internal/platform/kit"
	"context"
	"log"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

type CatalogServiceImpl struct {
	sports    output.SportRepository
	resources output.ResourceRepository
	coaches   output.CoachRepository
	classes   output.ClassSessionRepository
	bookings  output.BookingRepository
	branches  output.BranchRepository
	items     output.FloorItemRepository
	files     kit.MediaFiles
}

func NewCatalogService(
	sports output.SportRepository,
	resources output.ResourceRepository,
	coaches output.CoachRepository,
	classes output.ClassSessionRepository,
	bookings output.BookingRepository,
	storage output.FileStorage,
	media output.MediaRepository,
	branches output.BranchRepository,
	items output.FloorItemRepository,
) port.CatalogService {
	return &CatalogServiceImpl{
		sports: sports, resources: resources, coaches: coaches, classes: classes, bookings: bookings,
		branches: branches, items: items, files: kit.MediaFiles{Storage: storage, Media: media},
	}
}

// restaurant reports whether the branch takes table bookings (its sports are areas, its resources tables)
func (s *CatalogServiceImpl) restaurant(ctx context.Context, branchID uint) (bool, error) {
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return false, err
	}
	return b.Type == domain.BranchTypeRestaurant, nil
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
		ImageKey: m.ImageKey, ImageURL: s.files.URL(m.ImageKey), Active: m.Active, Type: m.Type,
		PlanWidth: m.PlanWidth, PlanHeight: m.PlanHeight, Floor: m.Floor,
	}
}

// validPlanSize fills in the default floor plan size and checks the limits
func validPlanSize(w, h *int) error {
	if *w == 0 {
		*w = domain.DefaultPlanWidth
	}
	if *h == 0 {
		*h = domain.DefaultPlanHeight
	}
	if *w < domain.MinPlanSize || *w > domain.MaxPlanSize || *h < domain.MinPlanSize || *h > domain.MaxPlanSize {
		return badRequest("INVALID_PLAN_SIZE")
	}
	return nil
}

func (s *CatalogServiceImpl) validateSport(ctx context.Context, branchID uint, req *dto.SportRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("NAME_IS_REQUIRED")
	}
	restaurant, err := s.restaurant(ctx, branchID)
	if err != nil {
		return err
	}
	if restaurant {
		// an area: no sport type, a floor plan instead
		req.Type = domain.SportTypeOther
		if err := validPlanSize(&req.PlanWidth, &req.PlanHeight); err != nil {
			return err
		}
		if req.Floor < domain.MinFloor || req.Floor > domain.MaxFloor {
			return badRequest("INVALID_FLOOR")
		}
	} else {
		req.PlanWidth, req.PlanHeight, req.Floor = 0, 0, 0
	}
	if req.Type == "" {
		req.Type = domain.SportTypeOther
	}
	if !domain.ValidSportType(req.Type) {
		return badRequest("INVALID_SPORT_TYPE")
	}
	return s.files.RequireOwned(ctx, branchID, req.ImageKey)
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
		PlanWidth: req.PlanWidth, PlanHeight: req.PlanHeight, Floor: req.Floor,
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
	m.Name, m.Description, m.ImageKey, m.Active = req.Name, req.Description, req.ImageKey, isActive(req.Active)
	m.Type = req.Type
	if m.PlanWidth != req.PlanWidth || m.PlanHeight != req.PlanHeight {
		if err := s.tablesFit(ctx, branchID, id, req.PlanWidth, req.PlanHeight); err != nil {
			return nil, err
		}
	}
	m.PlanWidth, m.PlanHeight, m.Floor = req.PlanWidth, req.PlanHeight, req.Floor
	if err := s.sports.Update(ctx, m); err != nil {
		return nil, err
	}
	d := s.toSportDTO(*m)
	return &d, nil
}

func (s *CatalogServiceImpl) DeleteSport(ctx context.Context, branchID, id uint) error {
	if _, err := s.sports.GetByID(ctx, branchID, id); err != nil {
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
	if s.items != nil {
		if err := s.replaceItems(ctx, branchID, id, nil); err != nil {
			return err
		}
	}
	return s.sports.Delete(ctx, branchID, id)
}

// ---------- resources ----------

func toResourceDTO(m domain.Resource) dto.ResourceDTO {
	return dto.ResourceDTO{
		ID: m.ID, SportID: m.SportID, Name: m.Name, Kind: m.Kind, Active: m.Active,
		TableFields: dto.TableFields{
			Seats: m.Seats, ExtraChairs: m.ExtraChairs, Shape: m.Shape,
			PlanX: m.PlanX, PlanY: m.PlanY, PlanW: m.PlanW, PlanH: m.PlanH, Rotation: m.Rotation,
		},
	}
}

// tableOf is the table a request describes (only the floor-plan fields)
func tableOf(id, areaID uint, t dto.TableFields) domain.Resource {
	r := domain.Resource{SportID: areaID, Seats: t.Seats, ExtraChairs: t.ExtraChairs, Shape: t.Shape,
		PlanX: t.PlanX, PlanY: t.PlanY, PlanW: t.PlanW, PlanH: t.PlanH, Rotation: t.Rotation}
	r.ID = id
	return r
}

// defaultTableSize suits the seats: 2 × 2 up to 4 people, then longer (two people per square)
func defaultTableSize(seats int) (int, int) {
	if seats <= 4 {
		return 2, 2
	}
	return min((seats+1)/2, domain.MaxTableSize), 2
}

func validRotation(r int) bool { return r == 0 || r == 90 || r == 180 || r == 270 }

// areaTables lists every table of an area
func (s *CatalogServiceImpl) areaTables(ctx context.Context, branchID, areaID uint) ([]domain.Resource, error) {
	items, _, err := s.resources.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"sport_id": areaID}})
	return items, err
}

// tablesFit refuses a plan size that would leave a table outside it
func (s *CatalogServiceImpl) tablesFit(ctx context.Context, branchID, areaID uint, w, h int) error {
	tables, err := s.areaTables(ctx, branchID, areaID)
	if err != nil {
		return err
	}
	for _, t := range tables {
		if !t.FitsIn(w, h) {
			return conflict("TABLES_OUTSIDE_PLAN")
		}
	}
	items, err := s.areaItems(ctx, branchID, areaID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if !it.FitsIn(w, h) {
			return conflict("TABLES_OUTSIDE_PLAN")
		}
	}
	return nil
}

// areaItems lists the objects drawn on an area's plan (none when the repository is not set up)
func (s *CatalogServiceImpl) areaItems(ctx context.Context, branchID, areaID uint) ([]domain.FloorItem, error) {
	if s.items == nil {
		return nil, nil
	}
	items, _, err := s.items.List(ctx, branchID, output.ListQuery{Filters: map[string]any{"area_id": areaID}})
	return items, err
}

// placeTable checks a table's seats and size, finds it a free spot on a new table without one,
// and refuses a place outside the plan or on top of another table
func (s *CatalogServiceImpl) placeTable(ctx context.Context, branchID, id uint, req *dto.ResourceRequest) error {
	t := &req.TableFields
	switch {
	case t.Seats < 1 || t.Seats > domain.MaxTableSeats:
		return badRequest("INVALID_SEATS")
	case t.ExtraChairs < 0 || t.ExtraChairs > domain.MaxExtraChairs:
		return badRequest("INVALID_EXTRA_CHAIRS")
	case !validRotation(t.Rotation):
		return badRequest("INVALID_ROTATION")
	}
	if t.Shape == "" {
		t.Shape = domain.TableShapeSquare
	}
	if !domain.ValidTableShape(t.Shape) {
		return badRequest("INVALID_TABLE_SHAPE")
	}
	if t.PlanW == 0 && t.PlanH == 0 {
		t.PlanW, t.PlanH = defaultTableSize(t.Seats)
	}
	if t.PlanW < 1 || t.PlanH < 1 || t.PlanW > domain.MaxTableSize || t.PlanH > domain.MaxTableSize {
		return badRequest("INVALID_TABLE_SIZE")
	}
	area, err := s.sports.GetByID(ctx, branchID, req.SportID)
	if err != nil {
		return orBadRequest(err, "SPORT_NOT_FOUND")
	}
	others, err := s.areaTables(ctx, branchID, req.SportID)
	if err != nil {
		return err
	}
	others = slices.DeleteFunc(others, func(o domain.Resource) bool { return o.ID == id })
	items, err := s.areaItems(ctx, branchID, req.SportID)
	if err != nil {
		return err
	}
	onItem := func(c domain.Resource) bool {
		return slices.ContainsFunc(items, func(it domain.FloorItem) bool { return it.Covers(c) })
	}
	free := func(c domain.Resource) bool {
		if !c.FitsIn(area.PlanWidth, area.PlanHeight) || onItem(c) {
			return false
		}
		return !slices.ContainsFunc(others, c.Overlaps)
	}
	table := tableOf(id, req.SportID, *t)
	if id == 0 && t.PlanX == 0 && t.PlanY == 0 && !free(table) {
		// a new table without a place: the first free spot, row by row
		for y := 0; y < area.PlanHeight; y++ {
			for x := 0; x < area.PlanWidth; x++ {
				table.PlanX, table.PlanY = x, y
				if free(table) {
					t.PlanX, t.PlanY = x, y
					return nil
				}
			}
		}
		return conflict("PLAN_IS_FULL")
	}
	if !table.FitsIn(area.PlanWidth, area.PlanHeight) {
		return badRequest("TABLE_OUTSIDE_PLAN")
	}
	if slices.ContainsFunc(others, table.Overlaps) {
		return conflict("TABLES_OVERLAP")
	}
	if onItem(table) {
		return conflict("TABLE_ON_OBJECT")
	}
	return nil
}

func (s *CatalogServiceImpl) validateResource(ctx context.Context, branchID, id uint, req *dto.ResourceRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("NAME_IS_REQUIRED")
	}
	restaurant, err := s.restaurant(ctx, branchID)
	if err != nil {
		return err
	}
	if restaurant {
		req.Kind = domain.ResourceKindTable
		return s.placeTable(ctx, branchID, id, req)
	}
	req.TableFields = dto.TableFields{} // courts have no seats or floor plan
	if !resourceKinds[req.Kind] {
		return badRequest("INVALID_RESOURCE_KIND")
	}
	_, err = s.sports.GetByID(ctx, branchID, req.SportID)
	return orBadRequest(err, "SPORT_NOT_FOUND")
}

// applyTable copies a table's seats and place onto the stored resource
func applyTable(m *domain.Resource, t dto.TableFields) {
	m.Seats, m.ExtraChairs, m.Shape = t.Seats, t.ExtraChairs, t.Shape
	m.PlanX, m.PlanY, m.PlanW, m.PlanH, m.Rotation = t.PlanX, t.PlanY, t.PlanW, t.PlanH, t.Rotation
}

func (s *CatalogServiceImpl) CreateResource(ctx context.Context, branchID uint, req dto.ResourceRequest) (*dto.ResourceDTO, error) {
	if err := s.validateResource(ctx, branchID, 0, &req); err != nil {
		return nil, err
	}
	m := domain.Resource{BranchID: branchID, SportID: req.SportID, Name: req.Name, Kind: req.Kind, Active: isActive(req.Active)}
	applyTable(&m, req.TableFields)
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
	m, err := s.resources.GetByID(ctx, branchID, id)
	if err != nil {
		return nil, err
	}
	if err := s.validateResource(ctx, branchID, id, &req); err != nil {
		return nil, err
	}
	m.SportID, m.Name, m.Kind, m.Active = req.SportID, req.Name, req.Kind, isActive(req.Active)
	applyTable(m, req.TableFields)
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

// ---------- restaurant floor plans ----------

// GetFloorPlan returns an area with every one of its tables, for the floor plan editor
func (s *CatalogServiceImpl) GetFloorPlan(ctx context.Context, branchID, areaID uint) (*dto.FloorPlanDTO, error) {
	area, err := s.sports.GetByID(ctx, branchID, areaID)
	if err != nil {
		return nil, err
	}
	tables, err := s.areaTables(ctx, branchID, areaID)
	if err != nil {
		return nil, err
	}
	items, err := s.areaItems(ctx, branchID, areaID)
	if err != nil {
		return nil, err
	}
	return &dto.FloorPlanDTO{
		Area: s.toSportDTO(*area), Tables: mapSlice(tables, toResourceDTO), Items: mapSlice(items, toFloorItemDTO),
	}, nil
}

func toFloorItemDTO(f domain.FloorItem) dto.FloorItemDTO {
	return dto.FloorItemDTO{ID: f.ID, Kind: f.Kind, Label: f.Label, Color: f.Color,
		PlanX: f.PlanX, PlanY: f.PlanY, PlanW: f.PlanW, PlanH: f.PlanH, Rotation: f.Rotation, TargetFloor: f.TargetFloor,
		Shape: f.Shape, CutW: f.CutW, CutH: f.CutH, CutCorner: f.CutCorner}
}

const maxFloorItems = 100

// floorItemsOf checks the objects of a plan save and turns them into rows of the area
func floorItemsOf(branchID, areaID uint, in []dto.FloorItemDTO) ([]domain.FloorItem, error) {
	if len(in) > maxFloorItems {
		return nil, badRequest("TOO_MANY_FLOOR_ITEMS")
	}
	out := make([]domain.FloorItem, 0, len(in))
	for _, it := range in {
		label := strings.TrimSpace(it.Label)
		switch {
		case !domain.ValidFloorItemKind(it.Kind):
			return nil, badRequest("INVALID_FLOOR_ITEM")
		case utf8.RuneCountInString(label) > 40:
			return nil, badRequest("FLOOR_ITEM_LABEL_TOO_LONG")
		case !validRotation(it.Rotation):
			return nil, badRequest("INVALID_ROTATION")
		case it.PlanW < 1 || it.PlanH < 1 || it.PlanW > domain.MaxPlanSize || it.PlanH > domain.MaxPlanSize:
			return nil, badRequest("INVALID_FLOOR_ITEM_SIZE")
		}
		color := ""
		if it.Kind == domain.FloorItemZone {
			color = it.Color
			if color == "" {
				color = domain.ZoneColors[0]
			}
			if !slices.Contains(domain.ZoneColors, color) {
				return nil, badRequest("INVALID_ZONE_COLOR")
			}
		}
		shape, cutW, cutH, corner, err := zoneShapeOf(it)
		if err != nil {
			return nil, err
		}
		var target *int
		if it.Kind == domain.FloorItemStairs && it.TargetFloor != nil {
			if *it.TargetFloor < domain.MinFloor || *it.TargetFloor > domain.MaxFloor {
				return nil, badRequest("INVALID_FLOOR")
			}
			target = it.TargetFloor
		}
		out = append(out, domain.FloorItem{Model: gorm.Model{ID: it.ID}, BranchID: branchID, AreaID: areaID, Kind: it.Kind, Label: label, Color: color,
			PlanX: it.PlanX, PlanY: it.PlanY, PlanW: it.PlanW, PlanH: it.PlanH, Rotation: it.Rotation, TargetFloor: target,
			Shape: shape, CutW: cutW, CutH: cutH, CutCorner: corner})
	}
	return out, nil
}

// zoneShapeOf checks the shape of a zone: square, round or long (like a table), or an L (its box with one corner
// cut out, a cut smaller than the box on both sides, so a real L is left). A zone that says nothing is a square;
// everything that is not a zone is a plain square.
func zoneShapeOf(it dto.FloorItemDTO) (shape string, cutW, cutH int, corner string, err error) {
	if it.Kind != domain.FloorItemZone || it.Shape == "" {
		return domain.ShapeSquare, 0, 0, "tr", nil
	}
	if !slices.Contains(domain.ZoneShapes, it.Shape) {
		return "", 0, 0, "", badRequest("INVALID_ZONE_SHAPE")
	}
	if it.Shape != domain.ShapeL {
		return it.Shape, 0, 0, "tr", nil
	}
	if it.Rotation != 0 || !slices.Contains(domain.CutCorners, it.CutCorner) ||
		it.CutW < 1 || it.CutH < 1 || it.CutW >= it.PlanW || it.CutH >= it.PlanH {
		return "", 0, 0, "", badRequest("INVALID_ZONE_SHAPE")
	}
	return domain.ShapeL, it.CutW, it.CutH, it.CutCorner, nil
}

const maxNewTables = 200 // made by one plan save (copies)

// SaveFloorPlan sets the plan size and moves, turns or resizes the area's tables at once; the
// whole plan is checked first (inside the plan, no two tables on the same square), so a refused
// save changes nothing
func (s *CatalogServiceImpl) SaveFloorPlan(ctx context.Context, branchID, areaID uint, req dto.FloorPlanRequest) (*dto.FloorPlanDTO, error) {
	restaurant, err := s.restaurant(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if !restaurant {
		return nil, conflict("NOT_A_RESTAURANT")
	}
	area, err := s.sports.GetByID(ctx, branchID, areaID)
	if err != nil {
		return nil, err
	}
	if err := validPlanSize(&req.PlanWidth, &req.PlanHeight); err != nil {
		return nil, err
	}
	tables, err := s.areaTables(ctx, branchID, areaID)
	if err != nil {
		return nil, err
	}
	byID := map[uint]int{}
	for i, t := range tables {
		byID[t.ID] = i
	}
	changed := map[int]bool{}
	for _, p := range req.Tables {
		i, ok := byID[p.ID]
		if !ok {
			return nil, badRequest("TABLE_NOT_IN_AREA")
		}
		if !validRotation(p.Rotation) {
			return nil, badRequest("INVALID_ROTATION")
		}
		if p.Shape == "" {
			p.Shape = tables[i].Shape
		}
		if !domain.ValidTableShape(p.Shape) {
			return nil, badRequest("INVALID_TABLE_SHAPE")
		}
		if p.PlanW < 1 || p.PlanH < 1 || p.PlanW > domain.MaxTableSize || p.PlanH > domain.MaxTableSize {
			return nil, badRequest("INVALID_TABLE_SIZE")
		}
		t := &tables[i]
		t.PlanX, t.PlanY, t.PlanW, t.PlanH, t.Rotation, t.Shape = p.PlanX, p.PlanY, p.PlanW, p.PlanH, p.Rotation, p.Shape
		changed[i] = true
	}
	// tables made with this save take part in every check below, and are stored last
	if len(req.NewTables) > maxNewTables {
		return nil, badRequest("TOO_MANY_NEW_TABLES")
	}
	var fresh []domain.Resource
	for _, n := range req.NewTables {
		name := strings.TrimSpace(n.Name)
		switch {
		case name == "":
			return nil, badRequest("NAME_IS_REQUIRED")
		case n.Seats < 1 || n.Seats > domain.MaxTableSeats:
			return nil, badRequest("INVALID_SEATS")
		case n.ExtraChairs < 0 || n.ExtraChairs > domain.MaxExtraChairs:
			return nil, badRequest("INVALID_EXTRA_CHAIRS")
		case !validRotation(n.Rotation):
			return nil, badRequest("INVALID_ROTATION")
		case n.PlanW < 1 || n.PlanH < 1 || n.PlanW > domain.MaxTableSize || n.PlanH > domain.MaxTableSize:
			return nil, badRequest("INVALID_TABLE_SIZE")
		}
		shape := n.Shape
		if shape == "" {
			shape = domain.TableShapeSquare
		}
		if !domain.ValidTableShape(shape) {
			return nil, badRequest("INVALID_TABLE_SHAPE")
		}
		fresh = append(fresh, domain.Resource{
			BranchID: branchID, SportID: areaID, Name: name, Kind: domain.ResourceKindTable, Active: isActive(n.Active),
			Seats: n.Seats, ExtraChairs: n.ExtraChairs, Shape: shape,
			PlanX: n.PlanX, PlanY: n.PlanY, PlanW: n.PlanW, PlanH: n.PlanH, Rotation: n.Rotation,
		})
	}
	existing := len(tables)
	tables = append(tables, fresh...)
	// the objects: the new list, or the ones already there
	var items []domain.FloorItem
	if req.Items != nil {
		if items, err = floorItemsOf(branchID, areaID, *req.Items); err != nil {
			return nil, err
		}
	} else if items, err = s.areaItems(ctx, branchID, areaID); err != nil {
		return nil, err
	}
	for _, it := range items {
		if !it.FitsIn(req.PlanWidth, req.PlanHeight) {
			return nil, badRequest("FLOOR_ITEM_OUTSIDE_PLAN")
		}
	}
	for i, t := range tables {
		if !t.FitsIn(req.PlanWidth, req.PlanHeight) {
			return nil, badRequest("TABLE_OUTSIDE_PLAN")
		}
		for _, o := range tables[i+1:] {
			if t.Overlaps(o) {
				return nil, conflict("TABLES_OVERLAP")
			}
		}
		for _, it := range items {
			if it.Covers(t) {
				return nil, conflict("TABLE_ON_OBJECT")
			}
		}
	}
	if req.Items != nil {
		if err := s.replaceItems(ctx, branchID, areaID, items); err != nil {
			return nil, err
		}
	}
	area.PlanWidth, area.PlanHeight = req.PlanWidth, req.PlanHeight
	if err := s.sports.Update(ctx, area); err != nil {
		return nil, err
	}
	for i := range tables {
		switch {
		case i >= existing:
			if err := s.resources.Create(ctx, &tables[i]); err != nil {
				return nil, err
			}
		case changed[i]:
			if err := s.resources.Update(ctx, &tables[i]); err != nil {
				return nil, err
			}
		}
	}
	return s.GetFloorPlan(ctx, branchID, areaID)
}

// replaceItems swaps the area's objects for the new list
func (s *CatalogServiceImpl) replaceItems(ctx context.Context, branchID, areaID uint, items []domain.FloorItem) error {
	old, err := s.areaItems(ctx, branchID, areaID)
	if err != nil {
		return err
	}
	// an object that is already on the plan keeps its id (event prices point at zone ids); the rest are new,
	// and the ones left out are deleted
	byID := map[uint]domain.FloorItem{}
	for _, o := range old {
		byID[o.ID] = o
	}
	kept := map[uint]bool{}
	for i := range items {
		if o, ok := byID[items[i].ID]; ok && items[i].ID != 0 && !kept[items[i].ID] {
			kept[o.ID] = true
			items[i].CreatedAt = o.CreatedAt
			if err := s.items.Update(ctx, &items[i]); err != nil {
				return err
			}
			continue
		}
		items[i].ID = 0
		if err := s.items.Create(ctx, &items[i]); err != nil {
			return err
		}
	}
	for _, o := range old {
		if !kept[o.ID] {
			if err := s.items.Delete(ctx, branchID, o.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------- coaches ----------

func (s *CatalogServiceImpl) toCoachDTO(m domain.Coach) dto.CoachDTO {
	return dto.CoachDTO{
		ID: m.ID, Name: m.Name, Bio: m.Bio, Phone: m.Phone, ImageKey: m.ImageKey,
		ImageURL: s.files.URL(m.ImageKey), Active: m.Active, Sports: sportRefs(m.Sports),
	}
}

func (s *CatalogServiceImpl) validateCoach(ctx context.Context, branchID uint, req *dto.CoachRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return badRequest("NAME_IS_REQUIRED")
	}
	return s.files.RequireOwned(ctx, branchID, req.ImageKey)
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
	m.Name, m.Bio, m.Phone = req.Name, req.Bio, strings.TrimSpace(req.Phone)
	m.ImageKey, m.Active = req.ImageKey, isActive(req.Active)
	m.Sports = nil // SetSports owns the association
	if err := s.coaches.Update(ctx, m); err != nil {
		return nil, err
	}
	return s.GetCoach(ctx, branchID, id)
}

// DeleteCoach refuses a coach who teaches classes or has upcoming bookings
func (s *CatalogServiceImpl) DeleteCoach(ctx context.Context, branchID, id uint) error {
	if err := s.refuseIfInUse(ctx, branchID, domain.OwnerCoach, id, "coach_id"); err != nil {
		return err
	}
	if _, err := s.coaches.GetByID(ctx, branchID, id); err != nil {
		return err
	}
	return s.coaches.Delete(ctx, branchID, id)
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
