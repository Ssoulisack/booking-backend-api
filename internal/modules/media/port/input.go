// Package port: what the media module offers to the rest of the app (input ports)
package port

import (
	meddto "backend/internal/modules/media/dto"
	"context"
	"io"
)

// MediaService validates and stores uploaded images; branchID 0 stores company files (logo, cover)
type MediaService interface {
	Upload(ctx context.Context, branchID uint, folder string, file io.Reader) (*meddto.MediaDTO, error)
	Delete(ctx context.Context, branchID uint, key string) error
	List(ctx context.Context, branchID uint) ([]meddto.MediaItemDTO, error)
}
