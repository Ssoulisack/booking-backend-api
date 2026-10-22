package mappers

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/pkg/jwt"
)

// MapperFactory provides centralized access to all mappers
type MapperFactory interface {
	UserDTOToDomainMapper() *ClassicMapper[dto.UserDTO, domain.User]
	DomainToUserDTOMapper() *ClassicMapper[domain.User, dto.UserDTO]
	TokenPair() *ClassicMapper[jwt.TokenPair, dto.UserJWTDTO]
}

type mapperFactory struct {
	classicMapper *ClassicMapper[any, any]
}

// NewMapperFactory creates a new MapperFactory instance
func NewMapperFactory(classicMapper *ClassicMapper[any, any]) MapperFactory {
	return &mapperFactory{
		classicMapper: classicMapper,
	}
}

func (f *mapperFactory) UserDTOToDomainMapper() *ClassicMapper[dto.UserDTO, domain.User] {
	return (*ClassicMapper[dto.UserDTO, domain.User])(f.classicMapper)
}

func (f *mapperFactory) DomainToUserDTOMapper() *ClassicMapper[domain.User, dto.UserDTO] {
	return (*ClassicMapper[domain.User, dto.UserDTO])(f.classicMapper)
}

func (f *mapperFactory) TokenPair() *ClassicMapper[jwt.TokenPair, dto.UserJWTDTO] {
	return (*ClassicMapper[jwt.TokenPair, dto.UserJWTDTO])(f.classicMapper)
}
