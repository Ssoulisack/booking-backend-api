package storage

import (
	"backend/internal/core/port/output"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type s3Storage struct {
	client  *minio.Client
	bucket  string
	baseURL string
}

// NewS3Storage creates a FileStorage for any S3-compatible service: Cloudflare R2, AWS S3,
// Backblaze B2, DigitalOcean Spaces... Files are viewed through PublicBaseURL (e.g. an R2 custom
// domain); the keys stay on the server and are only used to upload and delete.
func NewS3Storage(config S3Config) (output.FileStorage, error) {
	if config.Endpoint == "" || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("storage.r2 needs endpoint, bucket, accessKey and secretKey")
	}
	if config.PublicBaseURL == "" {
		return nil, fmt.Errorf("storage.r2 needs publicBaseUrl (the bucket's public address)")
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" {
		return nil, fmt.Errorf("storage.r2.endpoint must look like https://<host>, got %q", config.Endpoint)
	}
	region := config.Region
	if region == "" {
		region = "auto" // what Cloudflare R2 expects
	}
	client, err := minio.New(endpoint.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure:       endpoint.Scheme != "http",
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("storage r2 setup failed: %w", err)
	}
	return &s3Storage{
		client:  client,
		bucket:  config.Bucket,
		baseURL: strings.TrimRight(config.PublicBaseURL, "/"),
	}, nil
}

// Upload stores the file under key. Keys are unique, so browsers may cache the file for a year.
func (s *s3Storage) Upload(ctx context.Context, key, contentType string, body io.Reader) error {
	size := int64(-1)
	if sized, ok := body.(interface{ Len() int }); ok {
		size = int64(sized.Len())
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	return nil
}

// Delete removes the file; a file that is already gone counts as deleted
func (s *s3Storage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("storage delete failed: %w", err)
	}
	return nil
}

// URL returns the public link of a key, or "" for an empty key
func (s *s3Storage) URL(key string) string {
	if key == "" {
		return ""
	}
	return s.baseURL + "/" + key
}
