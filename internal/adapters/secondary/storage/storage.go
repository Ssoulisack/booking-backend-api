package storage

import (
	"backend/internal/core/port/output"
	"fmt"
)

// FilesRoute is where the backend serves a local storage folder
const FilesRoute = "/files"

// Config holds the file storage settings (config.yaml "storage"). Driver picks which section is used;
// the other sections are ignored, so each place keeps its own settings. Keys and secrets stay on the
// server; they must never reach the browser.
type Config struct {
	Driver   string         // local | r2 | supabase
	Local    LocalConfig    // storage.local
	R2       S3Config       // storage.r2
	Supabase SupabaseConfig // storage.supabase
}

// LocalConfig: files in a folder on this machine, served by the backend at /files
type LocalConfig struct {
	Path          string // folder for the files, e.g. ./data/uploads
	PublicBaseURL string // start of picture links; empty = /files (site and API on one domain)
}

// S3Config: Cloudflare R2, or any S3-compatible service (AWS S3, Backblaze B2, DigitalOcean Spaces)
type S3Config struct {
	Endpoint      string // https://<account-id>.r2.cloudflarestorage.com
	Region        string // "auto" for R2 (default)
	Bucket        string
	AccessKey     string
	SecretKey     string
	PublicBaseURL string // the bucket's public address (custom domain), required
}

// SupabaseConfig: Supabase Storage. ServiceKey is the server-side service_role key.
type SupabaseConfig struct {
	ProjectURL    string // https://<project-ref>.supabase.co
	Bucket        string
	ServiceKey    string
	PublicBaseURL string // optional CDN / custom domain; empty = the Supabase public URL
}

// New returns the FileStorage the config asks for
func New(config Config) (output.FileStorage, error) {
	switch config.Driver {
	case "local":
		return NewLocalStorage(config.Local), nil
	case "r2":
		return NewS3Storage(config.R2)
	case "supabase":
		return NewSupabaseStorage(config.Supabase), nil
	default:
		return nil, fmt.Errorf("unknown storage.driver %q (use local, r2 or supabase)", config.Driver)
	}
}

// LocalDir is the folder the backend must serve at FilesRoute, or "" when files live elsewhere
func LocalDir(config Config) string {
	if config.Driver == "local" {
		return config.Local.Path
	}
	return ""
}
