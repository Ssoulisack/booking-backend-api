package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// firstOrNil maps "not found" to (nil, nil) for lookups where absence is a normal answer
func firstOrNil[T any](tx *gorm.DB) (*T, error) {
	var m T
	err := tx.First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &m, nil
}

// ---------- company ----------

type companyRepository struct{ db *gorm.DB }

func NewCompanyRepository(db *gorm.DB) output.CompanyRepository { return &companyRepository{db: db} }

func (r *companyRepository) Get(ctx context.Context) (*domain.Company, error) {
	var c domain.Company
	if err := r.db.WithContext(ctx).Order("id ASC").First(&c).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &c, nil
}

func (r *companyRepository) Create(ctx context.Context, c *domain.Company) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(c).Error)
}

func (r *companyRepository) Update(ctx context.Context, c *domain.Company) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(c).Error)
}

// ---------- branches ----------

type branchRepository struct{ db *gorm.DB }

func NewBranchRepository(db *gorm.DB) output.BranchRepository { return &branchRepository{db: db} }

// withCompany fills the settings in force of loaded branches from the one company row
func (r *branchRepository) withCompany(ctx context.Context, branches ...*domain.Branch) error {
	var c domain.Company
	if err := r.db.WithContext(ctx).Order("id ASC").First(&c).Error; err != nil {
		return exception.HandleGormError(err)
	}
	for _, b := range branches {
		b.WithCompany(c)
	}
	return nil
}

func (r *branchRepository) Create(ctx context.Context, b *domain.Branch) error {
	if err := r.db.WithContext(ctx).Create(b).Error; err != nil {
		return exception.HandleGormError(err)
	}
	return r.withCompany(ctx, b)
}

// keptImage keeps only outside demo pictures: an uploaded file belongs to its branch
func keptImage(key string) string {
	if strings.HasPrefix(key, "https://placehold.co/") {
		return key
	}
	return ""
}

func (r *branchRepository) Clone(ctx context.Context, sourceID uint, b *domain.Branch) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(b).Error; err != nil {
			return err
		}
		return copySetup(tx, sourceID, b.ID)
	})
	if err != nil {
		return exception.HandleGormError(err)
	}
	return r.withCompany(ctx, b)
}

// copySetup copies what a branch offers (not what happened there) from one branch to another,
// pointing the copies at each other through old id -> new id maps
func copySetup(tx *gorm.DB, from, to uint) error {
	sports := map[uint]uint{}
	var oldSports []domain.Sport
	if err := tx.Where("branch_id = ?", from).Order("id").Find(&oldSports).Error; err != nil {
		return err
	}
	for _, o := range oldSports {
		n := domain.Sport{BranchID: to, Name: o.Name, Slug: o.Slug, Description: o.Description,
			ImageKey: keptImage(o.ImageKey), Active: o.Active, Type: o.Type}
		if err := tx.Create(&n).Error; err != nil {
			return err
		}
		sports[o.ID] = n.ID
	}

	resources := map[uint]uint{}
	var oldResources []domain.Resource
	if err := tx.Where("branch_id = ?", from).Order("id").Find(&oldResources).Error; err != nil {
		return err
	}
	for _, o := range oldResources {
		n := domain.Resource{BranchID: to, SportID: sports[o.SportID], Name: o.Name, Kind: o.Kind, Active: o.Active}
		if err := tx.Create(&n).Error; err != nil {
			return err
		}
		resources[o.ID] = n.ID
	}

	coaches := map[uint]uint{}
	var oldCoaches []domain.Coach
	if err := tx.Preload("Sports").Where("branch_id = ?", from).Order("id").Find(&oldCoaches).Error; err != nil {
		return err
	}
	for _, o := range oldCoaches {
		n := domain.Coach{BranchID: to, Name: o.Name, Bio: o.Bio, Phone: o.Phone, ImageKey: keptImage(o.ImageKey), Active: o.Active}
		if err := tx.Omit("Sports").Create(&n).Error; err != nil {
			return err
		}
		coaches[o.ID] = n.ID
		var own []domain.Sport
		for _, sp := range o.Sports {
			if id, ok := sports[sp.ID]; ok {
				own = append(own, domain.Sport{Model: gorm.Model{ID: id}})
			}
		}
		if len(own) > 0 {
			if err := tx.Model(&n).Association("Sports").Append(own); err != nil {
				return err
			}
		}
	}

	// the owner of an hour, price or closure: a space, a coach, or the whole branch
	owner := func(kind string, id uint) (uint, bool) {
		switch kind {
		case domain.OwnerResource:
			n, ok := resources[id]
			return n, ok
		case domain.OwnerCoach:
			n, ok := coaches[id]
			return n, ok
		}
		return 0, kind == domain.OwnerVenue
	}

	var hours []domain.OpeningHour
	if err := tx.Where("branch_id = ?", from).Find(&hours).Error; err != nil {
		return err
	}
	for _, o := range hours {
		if id, ok := owner(o.OwnerType, o.OwnerID); ok {
			if err := tx.Create(&domain.OpeningHour{BranchID: to, OwnerType: o.OwnerType, OwnerID: id,
				Weekday: o.Weekday, OpenTime: o.OpenTime, CloseTime: o.CloseTime}).Error; err != nil {
				return err
			}
		}
	}

	var prices []domain.Price
	if err := tx.Where("branch_id = ?", from).Find(&prices).Error; err != nil {
		return err
	}
	for _, o := range prices {
		if id, ok := owner(o.OwnerType, o.OwnerID); ok {
			if err := tx.Create(&domain.Price{BranchID: to, OwnerType: o.OwnerType, OwnerID: id,
				PricePerHour: o.PricePerHour, Currency: o.Currency}).Error; err != nil {
				return err
			}
		}
	}

	// closed days that are still to come (a past holiday means nothing to the copy)
	var closures []domain.Closure
	if err := tx.Where("branch_id = ? AND ends_at > ?", from, time.Now()).Find(&closures).Error; err != nil {
		return err
	}
	for _, o := range closures {
		if id, ok := owner(o.OwnerType, o.OwnerID); ok {
			if err := tx.Create(&domain.Closure{BranchID: to, OwnerType: o.OwnerType, OwnerID: id,
				StartsAt: o.StartsAt, EndsAt: o.EndsAt, Reason: o.Reason}).Error; err != nil {
				return err
			}
		}
	}

	// booking rules: the branch's own changes, then per sport and per space
	var rules []domain.BookingRule
	if err := tx.Where("branch_id = ? AND scope_type <> ?", from, domain.ScopeCompany).Find(&rules).Error; err != nil {
		return err
	}
	for _, o := range rules {
		scope := o.ScopeID
		switch o.ScopeType {
		case domain.ScopeSport:
			scope = sports[o.ScopeID]
		case domain.ScopeResource:
			scope = resources[o.ScopeID]
		}
		if o.ScopeType != domain.ScopeBranch && scope == 0 {
			continue
		}
		o.Model, o.BranchID, o.ScopeID = gorm.Model{}, to, scope
		if err := tx.Create(&o).Error; err != nil {
			return err
		}
	}

	// weekly classes: the timetable only; its classes are made again for the new branch
	var tables []domain.ClassTimetable
	if err := tx.Where("branch_id = ?", from).Order("id").Find(&tables).Error; err != nil {
		return err
	}
	for _, o := range tables {
		sport, ok := sports[o.SportID]
		if !ok {
			continue
		}
		o.Model, o.BranchID, o.SportID, o.FilledUntil = gorm.Model{}, to, sport, ""
		if o.CoachID != nil {
			id := coaches[*o.CoachID]
			o.CoachID = &id
		}
		if o.ResourceID != nil {
			id := resources[*o.ResourceID]
			o.ResourceID = &id
		}
		if err := tx.Create(&o).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *branchRepository) get(ctx context.Context, tx *gorm.DB) (*domain.Branch, error) {
	var b domain.Branch
	if err := tx.First(&b).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	if err := r.withCompany(ctx, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *branchRepository) GetByID(ctx context.Context, id uint) (*domain.Branch, error) {
	return r.get(ctx, r.db.WithContext(ctx).Where("id = ?", id))
}

func (r *branchRepository) GetBySlug(ctx context.Context, slug string) (*domain.Branch, error) {
	return r.get(ctx, r.db.WithContext(ctx).Where("slug = ?", slug))
}

func (r *branchRepository) List(ctx context.Context, q output.ListQuery) ([]domain.Branch, int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.Branch{})
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("(name ILIKE ? OR slug ILIKE ?)", like, like)
	}
	tx = applyFilters(tx, q.Filters)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var items []domain.Branch
	if err := paginate(tx.Order("id ASC"), q).Find(&items).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	ptrs := make([]*domain.Branch, len(items))
	for i := range items {
		ptrs[i] = &items[i]
	}
	if len(items) > 0 {
		if err := r.withCompany(ctx, ptrs...); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func (r *branchRepository) Update(ctx context.Context, b *domain.Branch) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(b).Error)
}

func (r *branchRepository) SlugTaken(ctx context.Context, slug string, exceptID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Branch{}).Where("slug = ? AND id <> ?", slug, exceptID).Count(&count).Error
	return count > 0, exception.HandleGormError(err)
}

// ---------- memberships ----------

type membershipRepository struct{ db *gorm.DB }

func NewMembershipRepository(db *gorm.DB) output.MembershipRepository {
	return &membershipRepository{db: db}
}

func (r *membershipRepository) Create(ctx context.Context, m *domain.Membership) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Omit("User", "Branch").Create(m).Error)
}

func (r *membershipRepository) Get(ctx context.Context, branchID, userID uint) (*domain.Membership, error) {
	return firstOrNil[domain.Membership](r.db.WithContext(ctx).Where("branch_id = ? AND user_id = ?", branchID, userID))
}

func (r *membershipRepository) ListByBranch(ctx context.Context, branchID uint) ([]domain.Membership, error) {
	var items []domain.Membership
	err := r.db.WithContext(ctx).Preload("User").Where("branch_id = ?", branchID).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *membershipRepository) ListByUser(ctx context.Context, userID uint) ([]domain.Membership, error) {
	var items []domain.Membership
	err := r.db.WithContext(ctx).Preload("Branch").Where("user_id = ?", userID).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *membershipRepository) ListAll(ctx context.Context) ([]domain.Membership, error) {
	var items []domain.Membership
	err := r.db.WithContext(ctx).Preload("User").Preload("Branch").Order("user_id ASC, branch_id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *membershipRepository) Replace(ctx context.Context, userID uint, role string, branchIDs []uint, active bool) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&domain.Membership{}).Error; err != nil {
			return err
		}
		for _, id := range branchIDs {
			m := domain.Membership{UserID: userID, BranchID: id, Role: role, Active: active}
			if err := tx.Omit("User", "Branch").Create(&m).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

// ---------- invitations ----------

type invitationRepository struct{ db *gorm.DB }

func NewInvitationRepository(db *gorm.DB) output.InvitationRepository {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) pending(ctx context.Context, now time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&domain.Invitation{}).Where("accepted_at IS NULL AND expires_at > ?", now)
}

func (r *invitationRepository) Create(ctx context.Context, inv *domain.Invitation) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(inv).Error)
}

func (r *invitationRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.Invitation, error) {
	return firstOrNil[domain.Invitation](r.db.WithContext(ctx).Where("token_hash = ?", hash))
}

func (r *invitationRepository) GetByID(ctx context.Context, id uint) (*domain.Invitation, error) {
	var inv domain.Invitation
	if err := r.db.WithContext(ctx).First(&inv, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &inv, nil
}

func (r *invitationRepository) ListPending(ctx context.Context, now time.Time) ([]domain.Invitation, error) {
	var items []domain.Invitation
	err := r.pending(ctx, now).Order("id ASC").Find(&items).Error
	return items, exception.HandleGormError(err)
}

func (r *invitationRepository) PendingForEmail(ctx context.Context, email string, now time.Time) (*domain.Invitation, error) {
	return firstOrNil[domain.Invitation](r.pending(ctx, now).Where("email = ?", email))
}

func (r *invitationRepository) Update(ctx context.Context, inv *domain.Invitation) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Save(inv).Error)
}

func (r *invitationRepository) Delete(ctx context.Context, id uint) error {
	return requireAffected(r.db.WithContext(ctx).Delete(&domain.Invitation{}, id))
}

// ---------- password resets ----------

type passwordResetRepository struct{ db *gorm.DB }

func NewPasswordResetRepository(db *gorm.DB) output.PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

func (r *passwordResetRepository) Create(ctx context.Context, reset *domain.PasswordReset) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(reset).Error)
}

func (r *passwordResetRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordReset, error) {
	return firstOrNil[domain.PasswordReset](r.db.WithContext(ctx).Where("token_hash = ?", hash))
}

func (r *passwordResetRepository) UseAll(ctx context.Context, userID uint, at time.Time) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Model(&domain.PasswordReset{}).
		Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", at).Error)
}
