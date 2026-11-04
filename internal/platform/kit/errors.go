// Package kit: small helpers every part of the backend shares — the standard errors, one-time tokens,
// and the branch's time zone and opening-hours arithmetic. One implementation, used by the old
// services and by the modules.
package kit

import (
	"backend/pkg/response"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
)

func Conflict(code string) error   { return response.NewError(http.StatusConflict, code) }
func BadRequest(code string) error { return response.ErrorBadRequest(code) }
func Forbidden(code string) error  { return response.NewError(http.StatusForbidden, code) }
func NotFound(code string) error   { return response.NewError(http.StatusNotFound, code) }

// IsNotFound reports whether err is a 404 answer
func IsNotFound(err error) bool {
	var appErr response.AppError
	return errors.As(err, &appErr) && appErr.Status == http.StatusNotFound
}

// OrBadRequest turns a not-found error into a 400 with this code (the id came from the caller's own input)
func OrBadRequest(err error, code string) error {
	if err != nil && IsNotFound(err) {
		return response.ErrorBadRequest(code)
	}
	return err
}

// MapSlice converts each item; the result is never nil so it encodes as [] in JSON
func MapSlice[M, D any](items []M, convert func(M) D) []D {
	out := make([]D, 0, len(items))
	for _, it := range items {
		out = append(out, convert(it))
	}
	return out
}

// NewToken returns a random URL-safe token and the hash to store. Only the hash is kept, so a
// database leak does not expose working invite, reset or booking links.
func NewToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken is what is stored for a token
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
