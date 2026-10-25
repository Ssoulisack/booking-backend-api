package services

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/pkg/response"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const maxUploadBytes = 2 << 20 // 2 MB

var (
	// folders a file may be uploaded to
	mediaFolders = map[string]bool{"sports": true, "coaches": true, "classes": true, "branding": true}
	// content type (detected from the bytes, not the filename) -> extension
	imageExtensions = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
	// the only shape of key the server ever generates: branches/{id}/{folder}/{uuid}.{ext}
	mediaKeyPattern = regexp.MustCompile(`^branches/([0-9]+)/(sports|coaches|classes|branding)/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)
)

// isValidMediaKey checks the key's shape and that it belongs to branchID
func isValidMediaKey(branchID uint, key string) bool {
	m := mediaKeyPattern.FindStringSubmatch(key)
	return m != nil && m[1] == strconv.FormatUint(uint64(branchID), 10)
}

// placeholderImages are outside demo pictures stored as a full link instead of an uploaded key
// (e.g. https://placehold.co/600x400.png): shown as they are, accepted on save, never deleted
const placeholderImages = "https://placehold.co/"

func isPlaceholder(key string) bool { return strings.HasPrefix(key, placeholderImages) }

// mediaFiles is shared by every service that stores an image key on a record
type mediaFiles struct {
	storage output.FileStorage
	media   output.MediaRepository
}

// requireOwned accepts "" (no image) or a key this branch uploaded
func (m mediaFiles) requireOwned(ctx context.Context, branchID uint, key string) error {
	if key == "" || isPlaceholder(key) {
		return nil
	}
	if !isValidMediaKey(branchID, key) {
		return badRequest("INVALID_FILE_KEY")
	}
	rec, err := m.media.Get(ctx, branchID, key)
	if err != nil {
		return err
	}
	if rec == nil {
		return badRequest("INVALID_FILE_KEY")
	}
	return nil
}

func (m mediaFiles) url(key string) string {
	if isPlaceholder(key) {
		return key
	}
	return m.storage.URL(key)
}

type MediaServiceImpl struct {
	files mediaFiles
}

func NewMediaService(storage output.FileStorage, media output.MediaRepository) input.MediaService {
	return &MediaServiceImpl{files: mediaFiles{storage: storage, media: media}}
}

// Upload accepts jpg / png / webp up to 2 MB. Branch files go under branches/{id}/, company files
// (branchID 0: logo, cover) under branches/0/. The key is generated here, so callers cannot choose paths.
func (s *MediaServiceImpl) Upload(ctx context.Context, branchID uint, folder string, file io.Reader) (*dto.MediaDTO, error) {
	if !mediaFolders[folder] {
		return nil, badRequest("INVALID_FOLDER")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return nil, badRequest("INVALID_FILE")
	}
	if len(data) == 0 {
		return nil, badRequest("FILE_IS_EMPTY")
	}
	if len(data) > maxUploadBytes {
		return nil, response.NewError(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE")
	}
	contentType := http.DetectContentType(data)
	ext, ok := imageExtensions[contentType]
	if !ok {
		return nil, response.NewError(http.StatusUnsupportedMediaType, "UNSUPPORTED_FILE_TYPE")
	}

	key := fmt.Sprintf("branches/%d/%s/%s.%s", branchID, folder, uuid.NewString(), ext)
	if err := s.files.storage.Upload(ctx, key, contentType, bytes.NewReader(data)); err != nil {
		log.Printf("❌ file upload failed: %v", err)
		return nil, response.NewError(http.StatusBadGateway, "STORAGE_UPLOAD_FAILED")
	}
	if err := s.files.media.Create(ctx, &domain.Media{BranchID: branchID, Key: key, SizeBytes: int64(len(data))}); err != nil {
		_ = s.files.storage.Delete(ctx, key) // keep storage and records in step
		return nil, err
	}
	return &dto.MediaDTO{Key: key, URL: s.files.url(key)}, nil
}

// List returns every file of the branch (0: the company) newest first, with what uses each one
func (s *MediaServiceImpl) List(ctx context.Context, branchID uint) ([]dto.MediaItemDTO, error) {
	files, err := s.files.media.List(ctx, branchID)
	if err != nil {
		return nil, err
	}
	uses, err := s.files.media.Uses(ctx, branchID)
	if err != nil {
		return nil, err
	}
	usedBy := map[string][]dto.MediaUseDTO{}
	for _, u := range uses {
		usedBy[u.Key] = append(usedBy[u.Key], dto.MediaUseDTO{Kind: u.Kind, Name: u.Name})
	}
	out := make([]dto.MediaItemDTO, 0, len(files))
	for _, f := range files {
		by := usedBy[f.Key]
		if by == nil {
			by = []dto.MediaUseDTO{}
		}
		out = append(out, dto.MediaItemDTO{Key: f.Key, URL: s.files.url(f.Key), SizeBytes: f.SizeBytes, CreatedAt: f.CreatedAt, UsedBy: by})
	}
	return out, nil
}

// Delete removes a file nothing uses; a picture still shown somewhere is refused (FILE_IN_USE)
func (s *MediaServiceImpl) Delete(ctx context.Context, branchID uint, key string) error {
	if !isValidMediaKey(branchID, key) {
		return badRequest("INVALID_FILE_KEY")
	}
	rec, err := s.files.media.Get(ctx, branchID, key)
	if err != nil {
		return err
	}
	if rec == nil {
		return notFound("FILE_NOT_FOUND")
	}
	uses, err := s.files.media.Uses(ctx, branchID)
	if err != nil {
		return err
	}
	for _, u := range uses {
		if u.Key == key {
			return conflict("FILE_IN_USE")
		}
	}
	if err := s.files.storage.Delete(ctx, key); err != nil {
		log.Printf("❌ file delete failed: %v", err)
		return response.NewError(http.StatusBadGateway, "STORAGE_DELETE_FAILED")
	}
	return s.files.media.Delete(ctx, branchID, key)
}
