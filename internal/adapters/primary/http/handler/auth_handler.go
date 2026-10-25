package handler

import (
	"backend/internal/adapters/primary/http/middleware"
	"backend/internal/core/dto"
	"backend/internal/core/mappers"
	"backend/internal/core/port/input"
	"backend/pkg/jwt"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type (
	AuthHandler interface {
		Register(c *fiber.Ctx) error
		Login(c *fiber.Ctx) error
		SocialLogin(c *fiber.Ctx) error
		SocialConfig(c *fiber.Ctx) error
		Refresh(c *fiber.Ctx) error
		Me(c *fiber.Ctx) error
		UpdateMe(c *fiber.Ctx) error
		MyBranches(c *fiber.Ctx) error
		ForgotPassword(c *fiber.Ctx) error
		ResetPassword(c *fiber.Ctx) error
	}
	authHandler struct {
		authService   input.AuthService
		mapperFactory mappers.MapperFactory
	}
)

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(
	authService input.AuthService,
	mapperFactory mappers.MapperFactory,
) AuthHandler {
	return &authHandler{
		authService:   authService,
		mapperFactory: mapperFactory,
	}
}

func (h *authHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterDTO
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.authService.Register(c.UserContext(), req)
	return created(c, res, err)
}

func (h *authHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginDTO
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.authService.Login(c.UserContext(), req)
	return ok(c, res, err)
}

// SocialLogin expects {"token": ...} from Google (ID token) or Facebook (access token) in /auth/social/:provider
func (h *authHandler) SocialLogin(c *fiber.Ctx) error {
	var req dto.SocialLoginRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.authService.SocialLogin(c.UserContext(), c.Params("provider"), req.Token)
	return ok(c, res, err)
}

// SocialConfig tells the login page which providers are switched on
func (h *authHandler) SocialConfig(c *fiber.Ctx) error {
	return ok(c, h.authService.SocialConfig(), nil)
}

// Refresh expects the refresh token as the bearer token (see jwt.AccessRefreshToken)
func (h *authHandler) Refresh(c *fiber.Ctx) error {
	claims, err := jwt.GetRefreshClaims(c)
	if err != nil {
		return response.NewErrorErrMsgUnauthorizedErrMsgInvalidToken(c)
	}
	res, err := h.authService.Refresh(c.UserContext(), uint(claims.ID), claims.IssuedAt)
	return ok(c, res, err)
}

func (h *authHandler) Me(c *fiber.Ctx) error {
	res, err := h.authService.Me(c.UserContext(), middleware.CurrentUser(c).ID)
	return ok(c, res, err)
}

// MyBranches lists the branches the user belongs to, for the dashboard's branch switcher
func (h *authHandler) MyBranches(c *fiber.Ctx) error {
	res, err := h.authService.MyBranches(c.UserContext(), middleware.CurrentUser(c).ID)
	return ok(c, res, err)
}

// ForgotPassword answers the same whether or not the email has an account
func (h *authHandler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.ForgotPasswordRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	return done(c, "IF_THE_ACCOUNT_EXISTS_AN_EMAIL_WAS_SENT", h.authService.ForgotPassword(c.UserContext(), req.Email))
}

func (h *authHandler) ResetPassword(c *fiber.Ctx) error {
	var req dto.ResetPasswordRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	return done(c, "PASSWORD_UPDATED", h.authService.ResetPassword(c.UserContext(), req))
}

// UpdateMe changes the logged-in person's name, nationality, phone and email
func (h *authHandler) UpdateMe(c *fiber.Ctx) error {
	var req dto.ProfileRequest
	if err := bind(c, &req); err != nil {
		return fail(c, err)
	}
	res, err := h.authService.UpdateProfile(c.UserContext(), middleware.CurrentUser(c).ID, req)
	return ok(c, res, err)
}
