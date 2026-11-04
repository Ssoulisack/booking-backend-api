package kit

import (
	"backend/internal/core/port/output"
	"context"
	"regexp"
	"strconv"
	"strings"
)

// the only shape of key the server ever generates: branches/{id}/{folder}/{uuid}.{ext}
var mediaKeyPattern = regexp.MustCompile(`^branches/([0-9]+)/(sports|coaches|classes|branding)/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)

// IsValidMediaKey checks the key's shape and that it belongs to branchID
func IsValidMediaKey(branchID uint, key string) bool {
	m := mediaKeyPattern.FindStringSubmatch(key)
	return m != nil && m[1] == strconv.FormatUint(uint64(branchID), 10)
}

// placeholderImages are outside demo pictures stored as a full link instead of an uploaded key
// (e.g. https://placehold.co/600x400.png): shown as they are, accepted on save, never deleted
const placeholderImages = "https://placehold.co/"

func IsPlaceholder(key string) bool { return strings.HasPrefix(key, placeholderImages) }

// MediaFiles is shared by every service that stores an image key on a record
type MediaFiles struct {
	Storage output.FileStorage
	Media   output.MediaRepository
}

// RequireOwned accepts "" (no image) or a key this branch uploaded
func (m MediaFiles) RequireOwned(ctx context.Context, branchID uint, key string) error {
	if key == "" || IsPlaceholder(key) {
		return nil
	}
	if !IsValidMediaKey(branchID, key) {
		return BadRequest("INVALID_FILE_KEY")
	}
	rec, err := m.Media.Get(ctx, branchID, key)
	if err != nil {
		return err
	}
	if rec == nil {
		return BadRequest("INVALID_FILE_KEY")
	}
	return nil
}

// URL is the address a picture is shown from
func (m MediaFiles) URL(key string) string {
	if IsPlaceholder(key) {
		return key
	}
	return m.Storage.URL(key)
}
