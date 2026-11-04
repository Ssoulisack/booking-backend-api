package services

import (
	"backend/internal/core/port/output"
	"backend/internal/platform/redis"
)

type UserService interface {
}

type UserServiceImpl struct {
	userRepo  output.UserRepository
	cacheRepo redis.CacheRepository
}

// NewUserService creates a new UserService
func NewUserService(
	userRepo output.UserRepository,
	cacheRepo redis.CacheRepository,
) UserService {
	return &UserServiceImpl{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
	}
}
