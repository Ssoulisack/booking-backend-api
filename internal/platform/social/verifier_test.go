package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTest(t *testing.T, handler http.HandlerFunc) *verifier {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	v := NewVerifier(Config{GoogleClientID: "g-client", FacebookAppID: "fb-app", FacebookAppSecret: "fb-secret"}).(*verifier)
	v.googleTokenInfo, v.facebookGraph = srv.URL+"/tokeninfo", srv.URL
	return v
}

func TestGoogleChecksAudience(t *testing.T) {
	v := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		aud := "g-client"
		if r.URL.Query().Get("id_token") == "foreign" {
			aud = "someone-elses-app"
		}
		_, _ = w.Write([]byte(`{"aud":"` + aud + `","sub":"123","email":"A@Mail.com","email_verified":"true","name":"Ann"}`))
	})
	p, err := v.Verify(context.Background(), "google", "good")
	if err != nil || p.UID != "123" || p.Email != "a@mail.com" || !p.EmailVerified {
		t.Fatalf("got %+v, %v", p, err)
	}
	if _, err := v.Verify(context.Background(), "google", "foreign"); err == nil || err.Error() != "INVALID_SOCIAL_TOKEN" {
		t.Fatalf("a token for another app must be refused, got %v", err)
	}
}

func TestFacebookChecksAppAndNeverVerifiesEmail(t *testing.T) {
	v := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/debug_token":
			app := "fb-app"
			if r.URL.Query().Get("input_token") == "foreign" {
				app = "other-app"
			}
			_, _ = w.Write([]byte(`{"data":{"app_id":"` + app + `","is_valid":true,"user_id":"77"}}`))
		case "/me":
			_, _ = w.Write([]byte(`{"id":"77","name":"Bo","email":"bo@mail.com"}`))
		}
	})
	p, err := v.Verify(context.Background(), "facebook", "good")
	if err != nil || p.UID != "77" || p.Email != "bo@mail.com" || p.EmailVerified {
		t.Fatalf("got %+v, %v", p, err)
	}
	if _, err := v.Verify(context.Background(), "facebook", "foreign"); err == nil || err.Error() != "INVALID_SOCIAL_TOKEN" {
		t.Fatalf("a token for another app must be refused, got %v", err)
	}
}

func TestProviderNeedsCredentials(t *testing.T) {
	v := NewVerifier(Config{GoogleClientID: "g"})
	if p := v.Public(); p.GoogleClientID != "g" || p.FacebookAppID != "" {
		t.Fatalf("public: %+v", p)
	}
	if _, err := v.Verify(context.Background(), "facebook", "x"); err == nil || err.Error() != "SOCIAL_LOGIN_NOT_AVAILABLE" {
		t.Fatalf("got %v", err)
	}
}
