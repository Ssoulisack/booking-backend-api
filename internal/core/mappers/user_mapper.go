package mappers

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/pkg/constants"
	"backend/pkg/jwt"
	"backend/pkg/utils"
	"strings"

	"gorm.io/gorm"
)

// UserMapper provides methods to convert between domain User and DTO objects
type UserMapper struct {
}

// NewUserMapper creates a new UserMapper
func NewUserMapper() *UserMapper {
	return &UserMapper{}
}

func (m *UserMapper) UserDTOToDomain(user *dto.UserDTO) (*domain.User, error) {
	password, err := utils.Encrypt(user.Password)
	if err != nil {
		return nil, err
	}
	return &domain.User{
		Model:        gorm.Model{ID: user.ID},
		Avatar:       user.Avatar,
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: password,
		Name:         user.Name,
	}, nil
}

func (m *UserMapper) UserDomainToDTO(user *domain.User) *dto.UserDTO {
	return &dto.UserDTO{
		ID:        user.ID,
		Avatar:    constants.GetMinioURL(user.Avatar),
		Username:  user.Username,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: utils.NewDateTimeFormatToString(user.CreatedAt),
		UpdatedAt: utils.NewDateTimeFormatToString(user.UpdatedAt),
		Status:    user.Status,
	}
}

func (m *UserMapper) DomainToDTOs(users []domain.User) []*dto.UserDTO {
	var dtos []*dto.UserDTO
	for _, user := range users {
		dtos = append(dtos, m.UserDomainToDTO(&user))
	}
	return dtos
}

func (m *UserMapper) TokenPairToJWTDTO(jwt *jwt.TokenPair) *dto.UserJWTDTO {
	return &dto.UserJWTDTO{
		Access:  strings.Replace(string(jwt.AccessToken), "\"", "", -1),
		Refresh: strings.Replace(string(jwt.RefreshToken), "\"", "", -1),
	}
}
