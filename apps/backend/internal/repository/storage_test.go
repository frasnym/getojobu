package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/repository"
)

func TestNoopStorageRepository(t *testing.T) {
	storage := repository.NewR2StorageRepository(config.R2Config{})
	require.NotNil(t, storage)

	url, err := storage.GetPresignedDownloadURL(context.Background(), "some/key.pdf", "resume.pdf", 15*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage is not configured")
	assert.Empty(t, url)
}

func TestR2StorageRepository_PresignValidation(t *testing.T) {
	storage := repository.NewR2StorageRepository(config.R2Config{
		AccountID:       "test-account",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		BucketName:      "getojobu",
		EndpointURL:     "https://test-account.r2.cloudflarestorage.com",
	})
	require.NotNil(t, storage)

	// Empty key
	url, err := storage.GetPresignedDownloadURL(context.Background(), "", "resume.pdf", 15*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key is required")
	assert.Empty(t, url)

	// Valid key generates presigned URL
	url, err = storage.GetPresignedDownloadURL(context.Background(), "123/456/resume.pdf", "Jane_Doe_Resume.pdf", 15*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, "https://test-account.r2.cloudflarestorage.com/getojobu/123/456/resume.pdf")
	assert.Contains(t, url, "X-Amz-Signature=")
	assert.Contains(t, url, "response-content-disposition=")
}
