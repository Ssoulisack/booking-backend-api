package services

import (
	"backend/internal/adapters/secondary/minio"
	"backend/internal/adapters/secondary/redis"
	"backend/internal/core/port/output"
)

type UserService interface {
}

type UserServiceImpl struct {
	userRepo  output.UserRepository
	cacheRepo redis.CacheRepository
	minioRepo minio.MinIORepository
}

// NewUserService creates a new UserService
func NewUserService(
	userRepo output.UserRepository,
	cacheRepo redis.CacheRepository,
	minioRepo minio.MinIORepository,
) UserService {
	return &UserServiceImpl{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		minioRepo: minioRepo,
	}
}
