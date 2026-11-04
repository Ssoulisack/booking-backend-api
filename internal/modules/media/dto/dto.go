// Package dto: the media module's own answer shapes
package dto

import "time"

type MediaDTO struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// MediaItemDTO is a file in the media library with where it is used (empty: safe to delete)
type MediaItemDTO struct {
	Key       string        `json:"key"`
	URL       string        `json:"url"`
	SizeBytes int64         `json:"size_bytes"`
	CreatedAt time.Time     `json:"created_at"`
	UsedBy    []MediaUseDTO `json:"used_by"`
}

// MediaUseDTO is one place a file is shown: kind is sport, coach, cover or logo
type MediaUseDTO struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}
