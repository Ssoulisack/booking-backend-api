// Package http: the identity module's driving adapter: sign up, login, social login, the person's own
// profile and branches, password reset, and invite links.
package http

import (
	iddto "backend/internal/modules/identity/dto"
	"backend/internal/modules/identity/port"
	"backend/internal/platform/httpx"
	"backend/internal/platform/middleware"
	"backend/pkg/jwt"
	"backend/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Handler answers the identity routes (it only knows the input ports)
type Handler struct {
	auth    port.AuthService
	invites port.InvitationService
}

// New builds the handler
func New(auth port.AuthService, invites port.InvitationService) *Handler {
	return &Handler{auth: auth, invites: invites}
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req iddto.RegisterDTO
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.auth.Register(c.UserContext(), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req iddto.LoginDTO
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.auth.Login(c.UserContext(), req)
	return httpx.OK(c, res, err)
}

// SocialLogin expects {"token": ...} from Google (ID token) or Facebook (access token) in /auth/social/:provider
func (h *Handler) SocialLogin(c *fiber.Ctx) error {
	var req iddto.SocialLoginRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.auth.SocialLogin(c.UserContext(), c.Params("provider"), req.Token)
	return httpx.OK(c, res, err)
}

// SocialConfig tells the login page which providers are switched on
func (h *Handler) SocialConfig(c *fiber.Ctx) error {
	return httpx.OK(c, h.auth.SocialConfig(), nil)
}

// Refresh expects the refresh token as the bearer token (see jwt.AccessRefreshToken)
func (h *Handler) Refresh(c *fiber.Ctx) error {
	claims, err := jwt.GetRefreshClaims(c)
	if err != nil {
		return response.NewErrorErrMsgUnauthorizedErrMsgInvalidToken(c)
	}
	res, err := h.auth.Refresh(c.UserContext(), uint(claims.ID), claims.IssuedAt)
	return httpx.OK(c, res, err)
}

func (h *Handler) Me(c *fiber.Ctx) error {
	res, err := h.auth.Me(c.UserContext(), middleware.CurrentUser(c).ID)
	return httpx.OK(c, res, err)
}

// MyBranches lists the branches the user belongs to, for the dashboard's branch switcher
func (h *Handler) MyBranches(c *fiber.Ctx) error {
	res, err := h.auth.MyBranches(c.UserContext(), middleware.CurrentUser(c).ID)
	return httpx.OK(c, res, err)
}

// ForgotPassword answers the same whether or not the email has an account
func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req iddto.ForgotPasswordRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "IF_THE_ACCOUNT_EXISTS_AN_EMAIL_WAS_SENT", h.auth.ForgotPassword(c.UserContext(), req.Email))
}

func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req iddto.ResetPasswordRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "PASSWORD_UPDATED", h.auth.ResetPassword(c.UserContext(), req))
}

// UpdateMe changes the logged-in person's name, nationality, phone and email
func (h *Handler) UpdateMe(c *fiber.Ctx) error {
	var req iddto.ProfileRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.auth.UpdateProfile(c.UserContext(), middleware.CurrentUser(c).ID, req)
	return httpx.OK(c, res, err)
}

// ChangePassword: the logged-in person sets a new password and gets new tokens
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	var req iddto.ChangePasswordRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.auth.ChangePassword(c.UserContext(), middleware.CurrentUser(c).ID, req)
	return httpx.OK(c, res, err)
}

// SendEmailCode emails the logged-in person a code for their email
func (h *Handler) SendEmailCode(c *fiber.Ctx) error {
	return httpx.Done(c, "CODE_SENT", h.auth.SendEmailCode(c.UserContext(), middleware.CurrentUser(c).ID))
}

// VerifyEmail checks the code and marks the email verified
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.auth.VerifyEmail(c.UserContext(), middleware.CurrentUser(c).ID, req.Code)
	return httpx.OK(c, res, err)
}

// Invitation serves an invite link (no branch membership needed)
func (h *Handler) Invitation(c *fiber.Ctx) error {
	res, err := h.invites.GetInvitation(c.UserContext(), c.Params("token"))
	return httpx.OK(c, res, err)
}

// AcceptInvitation needs a login only when the invited email already has an account
func (h *Handler) AcceptInvitation(c *fiber.Ctx) error {
	var req iddto.AcceptInvitationRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.invites.Accept(c.UserContext(), c.Params("token"), middleware.CurrentUser(c), req)
	return httpx.OK(c, res, err)
}

// ---------- the team's invitations (company owner and admins) ----------

func (h *Handler) ListInvitations(c *fiber.Ctx) error {
	res, err := h.invites.ListInvitations(c.UserContext())
	return httpx.OK(c, res, err)
}

func (h *Handler) Invite(c *fiber.Ctx) error {
	var req iddto.InviteRequest
	if err := httpx.Bind(c, &req); err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.invites.Invite(c.UserContext(), middleware.CurrentUser(c), req)
	return httpx.Created(c, res, err)
}

func (h *Handler) CancelInvitation(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	return httpx.Done(c, "DELETED", h.invites.CancelInvitation(c.UserContext(), id))
}

func (h *Handler) ResendInvitation(c *fiber.Ctx) error {
	id, err := httpx.ID(c, "id")
	if err != nil {
		return httpx.Fail(c, err)
	}
	res, err := h.invites.ResendInvitation(c.UserContext(), middleware.CurrentUser(c), id)
	return httpx.OK(c, res, err)
}
