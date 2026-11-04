package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	meddto "backend/internal/modules/media/dto"
	"backend/internal/modules/media/port"
	"backend/internal/platform/kit"
	"backend/pkg/response"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
)

const maxUploadBytes = 2 << 20 // 2 MB

var (
	// folders a file may be uploaded to
	mediaFolders = map[string]bool{"sports": true, "coaches": true, "classes": true, "branding": true}
	// content type (detected from the bytes, not the filename) -> extension
	imageExtensions = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
)

type MediaServiceImpl struct {
	files kit.MediaFiles
}

func NewMediaService(storage output.FileStorage, media output.MediaRepository) port.MediaService {
	return &MediaServiceImpl{files: kit.MediaFiles{Storage: storage, Media: media}}
}

// Upload accepts jpg / png / webp up to 2 MB. Branch files go under branches/{id}/, company files
// (branchID 0: logo, cover) under branches/0/. The key is generated here, so callers cannot choose paths.
func (s *MediaServiceImpl) Upload(ctx context.Context, branchID uint, folder string, file io.Reader) (*meddto.MediaDTO, error) {
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
	if err := s.files.Storage.Upload(ctx, key, contentType, bytes.NewReader(data)); err != nil {
		log.Printf("❌ file upload failed: %v", err)
		return nil, response.NewError(http.StatusBadGateway, "STORAGE_UPLOAD_FAILED")
	}
	if err := s.files.Media.Create(ctx, &domain.Media{BranchID: branchID, Key: key, SizeBytes: int64(len(data))}); err != nil {
		_ = s.files.Storage.Delete(ctx, key) // keep storage and records in step
		return nil, err
	}
	return &meddto.MediaDTO{Key: key, URL: s.files.URL(key)}, nil
}

// List returns every file of the branch (0: the company) newest first, with what uses each one
func (s *MediaServiceImpl) List(ctx context.Context, branchID uint) ([]meddto.MediaItemDTO, error) {
	files, err := s.files.Media.List(ctx, branchID)
	if err != nil {
		return nil, err
	}
	uses, err := s.files.Media.Uses(ctx, branchID)
	if err != nil {
		return nil, err
	}
	usedBy := map[string][]meddto.MediaUseDTO{}
	for _, u := range uses {
		usedBy[u.Key] = append(usedBy[u.Key], meddto.MediaUseDTO{Kind: u.Kind, Name: u.Name})
	}
	out := make([]meddto.MediaItemDTO, 0, len(files))
	for _, f := range files {
		by := usedBy[f.Key]
		if by == nil {
			by = []meddto.MediaUseDTO{}
		}
		out = append(out, meddto.MediaItemDTO{Key: f.Key, URL: s.files.URL(f.Key), SizeBytes: f.SizeBytes, CreatedAt: f.CreatedAt, UsedBy: by})
	}
	return out, nil
}

// Delete removes a file nothing uses; a picture still shown somewhere is refused (FILE_IN_USE)
func (s *MediaServiceImpl) Delete(ctx context.Context, branchID uint, key string) error {
	if !kit.IsValidMediaKey(branchID, key) {
		return badRequest("INVALID_FILE_KEY")
	}
	rec, err := s.files.Media.Get(ctx, branchID, key)
	if err != nil {
		return err
	}
	if rec == nil {
		return notFound("FILE_NOT_FOUND")
	}
	uses, err := s.files.Media.Uses(ctx, branchID)
	if err != nil {
		return err
	}
	for _, u := range uses {
		if u.Key == key {
			return conflict("FILE_IN_USE")
		}
	}
	if err := s.files.Storage.Delete(ctx, key); err != nil {
		log.Printf("❌ file delete failed: %v", err)
		return response.NewError(http.StatusBadGateway, "STORAGE_DELETE_FAILED")
	}
	return s.files.Media.Delete(ctx, branchID, key)
}
