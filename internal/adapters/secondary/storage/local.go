package storage

import (
	"backend/internal/core/port/output"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type localStorage struct {
	dir     string
	baseURL string
}

// NewLocalStorage creates a FileStorage that keeps files in a folder on this machine.
// Links are PublicBaseURL/<key>, or /files/<key> on the backend itself when it is empty.
func NewLocalStorage(config LocalConfig) output.FileStorage {
	base := strings.TrimRight(config.PublicBaseURL, "/")
	if base == "" {
		base = FilesRoute
	}
	if config.Path != "" {
		_ = os.MkdirAll(config.Path, 0o755) // so /files serves an empty folder, not an error, before the first upload
	}
	return &localStorage{dir: config.Path, baseURL: base}
}

// path turns a key into a file path inside the folder, refusing anything that would leave it
func (s *localStorage) path(key string) (string, error) {
	if s.dir == "" {
		return "", fmt.Errorf("file storage is not configured (storage.local.path)")
	}
	clean := filepath.Clean("/" + key)
	if key == "" || clean == "/" || clean != "/"+key {
		return "", fmt.Errorf("invalid file key %q", key)
	}
	return filepath.Join(s.dir, filepath.FromSlash(clean)), nil
}

// Upload stores the file under key; it fails if the key already exists
func (s *localStorage) Upload(_ context.Context, key, _ string, body io.Reader) error {
	target, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	// write a temp file next to the target, then link it in place so readers never see half a file
	tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return fmt.Errorf("storage upload failed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	if err := os.Link(tmp.Name(), target); err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	return nil
}

// Delete removes the file; a file that is already gone counts as deleted
func (s *localStorage) Delete(_ context.Context, key string) error {
	target, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storage delete failed: %w", err)
	}
	return nil
}

// URL returns the public link of a key, or "" for an empty key
func (s *localStorage) URL(key string) string {
	if key == "" {
		return ""
	}
	return s.baseURL + "/" + key
}
