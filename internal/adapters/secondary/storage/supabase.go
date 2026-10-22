package storage

import (
	"backend/internal/core/port/output"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config holds the Supabase Storage settings. ServiceKey is the server-side
// service_role key; it must never reach the browser.
type Config struct {
	ProjectURL    string // https://<project-ref>.supabase.co
	Bucket        string
	ServiceKey    string
	PublicBaseURL string // optional override, e.g. a CDN or Cloudflare domain
}

type supabaseStorage struct {
	config Config
	client *http.Client
}

// NewSupabaseStorage creates a FileStorage backed by the Supabase Storage REST API
func NewSupabaseStorage(config Config) output.FileStorage {
	config.ProjectURL = strings.TrimRight(config.ProjectURL, "/")
	config.PublicBaseURL = strings.TrimRight(config.PublicBaseURL, "/")
	return &supabaseStorage{
		config: config,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *supabaseStorage) objectURL(key string) string {
	return fmt.Sprintf("%s/storage/v1/object/%s/%s", s.config.ProjectURL, s.config.Bucket, key)
}

// Upload stores the file under key; it fails if the key already exists
func (s *supabaseStorage) Upload(ctx context.Context, key, contentType string, body io.Reader) error {
	if err := s.ready(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.objectURL(key), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	return s.do(req, "upload")
}

func (s *supabaseStorage) Delete(ctx context.Context, key string) error {
	if err := s.ready(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(key), nil)
	if err != nil {
		return err
	}
	return s.do(req, "delete")
}

// URL returns the public URL of a key, or "" for an empty key
func (s *supabaseStorage) URL(key string) string {
	if key == "" {
		return ""
	}
	if s.config.PublicBaseURL != "" {
		return s.config.PublicBaseURL + "/" + key
	}
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.config.ProjectURL, s.config.Bucket, key)
}

func (s *supabaseStorage) ready() error {
	if s.config.ProjectURL == "" || s.config.Bucket == "" || s.config.ServiceKey == "" {
		return fmt.Errorf("file storage is not configured (storage.projectUrl, bucket, serviceKey)")
	}
	return nil
}

func (s *supabaseStorage) do(req *http.Request, action string) error {
	req.Header.Set("Authorization", "Bearer "+s.config.ServiceKey)
	req.Header.Set("apikey", s.config.ServiceKey)
	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("storage %s failed: %w", action, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("storage %s failed: status %d: %s", action, res.StatusCode, msg)
	}
	return nil
}
