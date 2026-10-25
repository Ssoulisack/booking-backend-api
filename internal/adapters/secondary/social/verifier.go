// Package social checks Google and Facebook sign-in tokens with the providers themselves.
package social

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config holds the app credentials. A provider with empty credentials is switched off.
// The Facebook app secret stays on the server; the Google client id and Facebook app id are
// public values the browser needs.
type Config struct {
	GoogleClientID    string
	FacebookAppID     string
	FacebookAppSecret string
}

type verifier struct {
	config Config
	client *http.Client
	// the provider endpoints, replaceable in tests
	googleTokenInfo string
	facebookGraph   string
}

func NewVerifier(config Config) output.SocialVerifier {
	return &verifier{
		config:          config,
		client:          &http.Client{Timeout: 10 * time.Second},
		googleTokenInfo: "https://oauth2.googleapis.com/tokeninfo",
		facebookGraph:   "https://graph.facebook.com/v19.0",
	}
}

func (v *verifier) Public() output.SocialPublic {
	p := output.SocialPublic{}
	if v.enabled()[domain.ProviderGoogle] {
		p.GoogleClientID = v.config.GoogleClientID
	}
	if v.enabled()[domain.ProviderFacebook] {
		p.FacebookAppID = v.config.FacebookAppID
	}
	return p
}

func (v *verifier) enabled() map[string]bool {
	return map[string]bool{
		domain.ProviderGoogle:   v.config.GoogleClientID != "",
		domain.ProviderFacebook: v.config.FacebookAppID != "" && v.config.FacebookAppSecret != "",
	}
}

func invalid() error { return response.NewError(http.StatusUnauthorized, "INVALID_SOCIAL_TOKEN") }

func (v *verifier) Verify(ctx context.Context, provider, token string) (*output.SocialProfile, error) {
	if !v.enabled()[provider] {
		return nil, response.NewError(http.StatusNotFound, "SOCIAL_LOGIN_NOT_AVAILABLE")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, invalid()
	}
	if provider == domain.ProviderGoogle {
		return v.google(ctx, token)
	}
	return v.facebook(ctx, token)
}

func (v *verifier) get(ctx context.Context, u string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	res, err := v.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("social check failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return res.StatusCode, err
		}
	}
	return res.StatusCode, nil
}

// google asks Google to verify the ID token (signature and expiry), then checks it was issued
// for this app
func (v *verifier) google(ctx context.Context, idToken string) (*output.SocialProfile, error) {
	var info struct {
		Aud           string `json:"aud"`
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
	}
	status, err := v.get(ctx, v.googleTokenInfo+"?id_token="+url.QueryEscape(idToken), &info)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK || info.Sub == "" || info.Aud != v.config.GoogleClientID {
		return nil, invalid()
	}
	return &output.SocialProfile{
		UID:           info.Sub,
		Email:         strings.ToLower(strings.TrimSpace(info.Email)),
		EmailVerified: info.EmailVerified == "true",
		Name:          strings.TrimSpace(info.Name),
	}, nil
}

// facebook checks the access token belongs to this app (debug_token), then reads the profile.
// A Facebook email is never treated as verified.
func (v *verifier) facebook(ctx context.Context, accessToken string) (*output.SocialProfile, error) {
	appToken := v.config.FacebookAppID + "|" + v.config.FacebookAppSecret
	var debug struct {
		Data struct {
			AppID   string `json:"app_id"`
			IsValid bool   `json:"is_valid"`
			UserID  string `json:"user_id"`
		} `json:"data"`
	}
	status, err := v.get(ctx, v.facebookGraph+"/debug_token?input_token="+url.QueryEscape(accessToken)+
		"&access_token="+url.QueryEscape(appToken), &debug)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK || !debug.Data.IsValid || debug.Data.AppID != v.config.FacebookAppID || debug.Data.UserID == "" {
		return nil, invalid()
	}
	var me struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	status, err = v.get(ctx, v.facebookGraph+"/me?fields=id,name,email&access_token="+url.QueryEscape(accessToken), &me)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK || me.ID != debug.Data.UserID {
		return nil, invalid()
	}
	return &output.SocialProfile{
		UID:   me.ID,
		Email: strings.ToLower(strings.TrimSpace(me.Email)),
		Name:  strings.TrimSpace(me.Name),
	}, nil
}
