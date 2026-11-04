package handler

import (
	"backend/internal/core/mappers"
	"backend/internal/core/port/input"
)

type (
	UserHandler interface {
	}
	userHandler struct {
		userService   input.UserService
		mapperFactory mappers.MapperFactory
	}
)

// NewUserHandler creates a new UserHandler
func NewUserHandler(
	userService input.UserService,
	mapperFactory mappers.MapperFactory,
) UserHandler {
	return &userHandler{
		userService:   userService,
		mapperFactory: mapperFactory,
	}
}
