package postgres

import (
	"backend/internal/adapters/secondary/postgres/exception"
	"backend/internal/adapters/secondary/postgres/numbering"
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"context"
	"time"

	"gorm.io/gorm"
)

// userRepository implements the UserRepository interface using PostgreSQL
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new PostgreSQL user repository
func NewUserRepository(db *gorm.DB) output.UserRepository {
	return &userRepository{db: db}
}

var _ output.UserRepository = (*userRepository)(nil)

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Create(user).Error)
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &user, nil
}

func (r *userRepository) List(ctx context.Context, q output.ListQuery) ([]domain.User, int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.User{})
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		match := r.db.Where("name ILIKE ? OR email ILIKE ?", like, like)
		if phone, ok := domain.NormalizePhone(q.Keyword); ok {
			match = match.Or("phone = ?", phone)
		}
		if no, ok := domain.ParseMemberNo(q.Keyword); ok {
			match = match.Or("member_no = ?", no)
		}
		tx = tx.Where(match)
	}
	for column, value := range q.Filters {
		tx = tx.Where(column+" = ?", value)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	var users []domain.User
	tx = tx.Order("id DESC")
	if q.Limit > 0 {
		tx = tx.Offset((max(q.Page, 1) - 1) * q.Limit).Limit(q.Limit)
	}
	if err := tx.Find(&users).Error; err != nil {
		return nil, 0, exception.HandleGormError(err)
	}
	return users, total, nil
}

func (r *userRepository) SetStatus(ctx context.Context, id uint, status bool) error {
	res := r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return exception.HandleGormError(res.Error)
	}
	if res.RowsAffected == 0 {
		return exception.HandleGormError(gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *userRepository) SetPassword(ctx context.Context, id uint, hash []byte, changedAt time.Time) error {
	return requireAffected(r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).
		Updates(map[string]any{"password_hash": hash, "password_changed_at": changedAt}))
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, exception.HandleGormError(err)
}

func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("phone = ? AND phone <> ''", phone).First(&user).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &user, nil
}

// FindByMemberNo looks up a stored member number (CNO000042 / ENO000007)
func (r *userRepository) FindByMemberNo(ctx context.Context, no string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("member_no = ?", no).First(&user).Error; err != nil {
		return nil, exception.HandleGormError(err)
	}
	return &user, nil
}

// SyncMemberNo gives the account the number for its current role (see numbering.Sync)
func (r *userRepository) SyncMemberNo(ctx context.Context, id uint) (string, error) {
	no, err := numbering.Sync(r.db.WithContext(ctx), id)
	return no, exception.HandleGormError(err)
}

func (r *userRepository) PhoneTaken(ctx context.Context, phone string, exceptID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("phone = ? AND id <> ?", phone, exceptID).Count(&count).Error
	return count > 0, exception.HandleGormError(err)
}

func (r *userRepository) EmailTaken(ctx context.Context, email string, exceptID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("email = ? AND id <> ?", email, exceptID).Count(&count).Error
	return count > 0, exception.HandleGormError(err)
}

func (r *userRepository) UpdateProfile(ctx context.Context, u *domain.User) error {
	return requireAffected(r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"name": u.Name, "email": u.Email, "username": u.Username, "phone": u.Phone,
		"nationality": u.Nationality, "email_generated": u.EmailGenerated,
	}))
}

func (r *userRepository) SetCompanyRole(ctx context.Context, id uint, role string) error {
	return requireAffected(r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Update("company_role", role))
}

func (r *userRepository) TransferOwnership(ctx context.Context, fromID, toID uint) error {
	return exception.HandleGormError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.User{}).Where("id = ? AND company_role = ?", fromID, domain.CompanyOwner).
			Update("company_role", domain.CompanyAdmin)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		res = tx.Model(&domain.User{}).Where("id = ?", toID).Update("company_role", domain.CompanyOwner)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}))
}
