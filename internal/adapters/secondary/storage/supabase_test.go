package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupabaseUploadDeleteURL(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth, gotType = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSupabaseStorage(Config{ProjectURL: srv.URL + "/", Bucket: "media", ServiceKey: "secret"})

	if err := s.Upload(context.Background(), "sports/a.png", "image/png", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/storage/v1/object/media/sports/a.png" ||
		gotAuth != "Bearer secret" || gotType != "image/png" || gotBody != "data" {
		t.Fatalf("unexpected upload request: %s %s %q %q %q", gotMethod, gotPath, gotAuth, gotType, gotBody)
	}

	if err := s.Delete(context.Background(), "sports/a.png"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" || gotPath != "/storage/v1/object/media/sports/a.png" {
		t.Fatalf("unexpected delete request: %s %s", gotMethod, gotPath)
	}

	if got, want := s.URL("sports/a.png"), srv.URL+"/storage/v1/object/public/media/sports/a.png"; got != want {
		t.Fatalf("URL = %s, want %s", got, want)
	}
	if s.URL("") != "" {
		t.Fatal("empty key must give empty URL")
	}
}

func TestSupabasePublicBaseURLOverride(t *testing.T) {
	s := NewSupabaseStorage(Config{ProjectURL: "https://x.supabase.co", Bucket: "media", ServiceKey: "k", PublicBaseURL: "https://cdn.example.com/"})
	if got := s.URL("a/b.jpg"); got != "https://cdn.example.com/a/b.jpg" {
		t.Fatalf("got %s", got)
	}
}

func TestSupabaseErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "duplicate", http.StatusBadRequest)
	}))
	defer srv.Close()

	s := NewSupabaseStorage(Config{ProjectURL: srv.URL, Bucket: "media", ServiceKey: "k"})
	if err := s.Upload(context.Background(), "a.png", "image/png", strings.NewReader("x")); err == nil {
		t.Fatal("expected error on non-2xx response")
	}

	unconfigured := NewSupabaseStorage(Config{})
	if err := unconfigured.Upload(context.Background(), "a.png", "image/png", strings.NewReader("x")); err == nil {
		t.Fatal("expected error when not configured")
	}
}
