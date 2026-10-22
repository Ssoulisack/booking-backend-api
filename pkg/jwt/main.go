package jwt

import (
	"backend/internal/config"
	"backend/internal/core/dto"
	"backend/pkg/response"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kataras/jwt"
)

type ClaimsToken struct {
	ID        int   `json:"id,omitempty"`
	RoleID    int   `json:"role_id,omitempty"`
	IssuedAt  int64 `json:"iat,omitempty"`
	ExpiresAt int64 `json:"exp,omitempty"`
}

var (
	JWT_ACCESS_EXSPIRATION  = 7  // days
	JWT_REFRESH_EXSPIRATION = 30 // days
)

type TokenPair struct {
	AccessToken  json.RawMessage `json:"access_token,omitempty"`
	RefreshToken json.RawMessage `json:"refresh_token,omitempty"`
}

func GenerateJWTToken(id, roleID int) (*TokenPair, error) {
	standardClaims := ClaimsToken{
		ID:        id,
		RoleID:    roleID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * time.Duration(JWT_ACCESS_EXSPIRATION)).Unix(),
	}
	encrypt, _, err := jwt.GCM([]byte(config.JWTGlobal.Secret), nil)
	if err != nil {

		return nil, err
	}
	token, err := jwt.SignEncrypted(jwt.HS256, []byte(config.JWTGlobal.Secret), encrypt, standardClaims, jwt.MaxAge(time.Duration(JWT_ACCESS_EXSPIRATION)*24*time.Hour)) //7Days
	if err != nil {

		return nil, err
	}
	reEncrypt, _, _ := jwt.GCM([]byte(config.JWTGlobal.RefreshSecret), nil)
	refreshToken, err := jwt.SignEncrypted(jwt.HS256, []byte(config.JWTGlobal.RefreshSecret), reEncrypt, standardClaims, jwt.MaxAge(time.Duration(JWT_REFRESH_EXSPIRATION)*24*time.Hour)) //30Days
	if err != nil {
		return nil, err
	}
	tokenPairData := jwt.NewTokenPair(token, refreshToken)
	return &TokenPair{
		AccessToken:  BytesQuote(tokenPairData.AccessToken),
		RefreshToken: BytesQuote(tokenPairData.RefreshToken),
	}, nil
}

func AccessToken(ctx *fiber.Ctx) error {
	_, decrypt, _ := jwt.GCM([]byte(config.JWTGlobal.Secret), nil)
	auth := ctx.Get("Authorization")
	if auth == "" {
		return response.NewErrorErrMsgUnauthorized(ctx)
	}
	jwtFromHeader := bearerToken(auth)
	verifiedToken, err := jwt.VerifyEncrypted(jwt.HS256, []byte(config.JWTGlobal.Secret), decrypt, []byte(jwtFromHeader))
	if err != nil {
		if strings.Contains(err.Error(), "jwt: invalid token signature") {
			return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
				Success: false,
				Message: "ACCESS_TOKEN_INVALID_SIGNATURE",
			})
		}
		if strings.Contains(err.Error(), "jwt: unexpected token algorithm") {
			return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
				Success: false,
				Message: "ACCESS_TOKEN_INVALID_ALGORITHM",
			})
		}
		if strings.Contains(err.Error(), "jwt: token expired") {
			return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
				Success: false,
				Message: "ACCESS_TOKEN_EXPIRED",
			})
		}
		return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
			Success: false,
			Message: err.Error(),
		})
	}
	var claims ClaimsToken
	err = verifiedToken.Claims(&claims)
	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(
			dto.BaseResponse{
				Success: false,
				Message: err.Error(),
			},
		)
	}
	return ctx.Next()
}

func AccessRefreshToken(ctx *fiber.Ctx) error {
	_, decrypt, _ := jwt.GCM([]byte(config.JWTGlobal.RefreshSecret), nil)
	auth := ctx.Get("Authorization")
	if auth == "" {
		return response.NewErrorErrMsgUnauthorized(ctx)
	}
	jwtFromHeader := bearerToken(auth)
	verifiedToken, err := jwt.VerifyEncrypted(jwt.HS256, []byte(config.JWTGlobal.RefreshSecret), decrypt, []byte(jwtFromHeader))
	if err != nil {
		if strings.Contains(err.Error(), "jwt: invalid token signature") {
			return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
				Success: false,
				Message: "REFRESH_TOKEN_INVALID_SIGNATURE",
			})
		}
		if strings.Contains(err.Error(), "jwt: unexpected token algorithm") {
			return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
				Success: false,
				Message: "REFRESH_TOKEN_INVALID_ALGORITHM",
			})
		}
		if strings.Contains(err.Error(), "jwt: token expired") {
			return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
				Success: false,
				Message: "REFRESH_TOKEN_EXPIRED",
			})
		}
		return ctx.Status(fiber.StatusUnauthorized).JSON(
			dto.BaseResponse{
				Success: false,
				Message: err.Error(),
			},
		)
	}
	var claims ClaimsToken
	err = verifiedToken.Claims(&claims)
	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(dto.BaseResponse{
			Success: false,
			Message: err.Error(),
		})
	}
	return ctx.Next()
}

func GetOwnerAccessToken(c *fiber.Ctx) (*ClaimsToken, error) {
	_, decrypt, _ := jwt.GCM([]byte(config.JWTGlobal.Secret), nil)
	auth := c.Get("Authorization")
	jwtFromHeader := bearerToken(auth)
	verifiedToken, err := jwt.VerifyEncrypted(jwt.HS256, []byte(config.JWTGlobal.Secret), decrypt, []byte(jwtFromHeader))
	if err != nil {
		return nil, err
	}
	var claims ClaimsToken
	err = verifiedToken.Claims(&claims)
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

func GetOwnerRefresh(c *fiber.Ctx) (*ClaimsToken, error) {
	_, decrypt, _ := jwt.GCM([]byte(config.JWTGlobal.RefreshSecret), nil)
	auth := c.Get("Authorization")
	if auth == "" {
		return nil, response.NewErrorErrMsgUnauthorized(c)
	}
	jwtFromHeader := bearerToken(auth)

	verifiedToken, err := jwt.VerifyEncrypted(jwt.HS256, []byte(config.JWTGlobal.RefreshSecret), decrypt, []byte(jwtFromHeader))
	if err != nil {
		return nil, err
	}
	var claims ClaimsToken
	err = verifiedToken.Claims(&claims)
	if err != nil {
		return nil, err
	}
	resp := ClaimsToken{
		ID:        claims.ID,
		RoleID:    claims.RoleID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * 7).Unix(),
	}
	return &resp, nil
}

// TODO: Refresh token
func GenerateRefreshToken(ctx *fiber.Ctx) (*TokenPair, error) {
	auth := ctx.Get("Authorization")
	if auth == "" {
		return nil, response.NewErrorErrMsgUnauthorized(ctx)
	}
	claims, err := GetOwnerRefresh(ctx)
	if err != nil {
		return nil, err
	}
	standardClaims := ClaimsToken{
		ID:        claims.ID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * time.Duration(JWT_ACCESS_EXSPIRATION)).Unix(),
	}
	encrypt, _, _ := jwt.GCM([]byte(config.JWTGlobal.Secret), nil)
	token, err := jwt.SignEncrypted(jwt.HS256, []byte(config.JWTGlobal.Secret), encrypt, standardClaims, jwt.MaxAge(time.Duration(JWT_ACCESS_EXSPIRATION)*24*time.Hour))
	if err != nil {
		return nil, err
	}
	reEncrypt, _, _ := jwt.GCM([]byte(config.JWTGlobal.RefreshSecret), nil)
	refreshToken, err := jwt.SignEncrypted(jwt.HS256, []byte(config.JWTGlobal.RefreshSecret), reEncrypt, standardClaims, jwt.MaxAge(time.Duration(JWT_REFRESH_EXSPIRATION)*24*time.Hour))
	if err != nil {

		return nil, err
	}
	tokenPairData := jwt.NewTokenPair(token, refreshToken)
	return &TokenPair{
		AccessToken:  BytesQuote(tokenPairData.AccessToken),
		RefreshToken: BytesQuote(tokenPairData.RefreshToken),
	}, nil
}

func BytesQuote(b []byte) []byte {
	dst := make([]byte, len(b)+2)
	dst[0] = '"'
	copy(dst[1:], b)
	dst[len(dst)-1] = '"'
	return dst
}

// bearerToken strips the "Bearer " prefix; a malformed header yields "" and fails verification
func bearerToken(auth string) string {
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[7:])
}

// GetRefreshClaims returns the refresh token's claims as issued (GetOwnerRefresh replaces IssuedAt with now)
func GetRefreshClaims(c *fiber.Ctx) (*ClaimsToken, error) {
	_, decrypt, _ := jwt.GCM([]byte(config.JWTGlobal.RefreshSecret), nil)
	verifiedToken, err := jwt.VerifyEncrypted(jwt.HS256, []byte(config.JWTGlobal.RefreshSecret), decrypt, []byte(bearerToken(c.Get("Authorization"))))
	if err != nil {
		return nil, err
	}
	var claims ClaimsToken
	if err := verifiedToken.Claims(&claims); err != nil {
		return nil, err
	}
	return &claims, nil
}
