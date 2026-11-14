package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
)

// Storage defines operations for object storage.
type Storage interface {
	GetPresignedDownloadURL(ctx context.Context, key string, filename string, expiry time.Duration) (string, error)
}

type r2StorageRepository struct {
	presignClient *s3.PresignClient
	bucketName    string
}

// NewR2StorageRepository creates a Storage repository backed by Cloudflare R2 (S3-compatible).
func NewR2StorageRepository(cfg config.R2Config) Storage {
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.EndpointURL == "" {
		return &noopStorageRepository{}
	}

	s3Client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(cfg.EndpointURL),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		UsePathStyle: true,
	})

	presignClient := s3.NewPresignClient(s3Client)

	bucket := cfg.BucketName
	if bucket == "" {
		bucket = "getojobu"
	}

	return &r2StorageRepository{
		presignClient: presignClient,
		bucketName:    bucket,
	}
}

func (r *r2StorageRepository) GetPresignedDownloadURL(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	if r.presignClient == nil {
		return "", errors.New("storage client is not configured")
	}
	if key == "" {
		return "", errors.New("key is required")
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	}
	if filename != "" {
		disposition := fmt.Sprintf(`attachment; filename="%s"`, filename)
		input.ResponseContentDisposition = aws.String(disposition)
	}

	req, err := r.presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}

	return req.URL, nil
}

type noopStorageRepository struct{}

func (n *noopStorageRepository) GetPresignedDownloadURL(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	return "", errors.New("storage is not configured (missing R2 credentials)")
}
