package repository

import (
	"context"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersRepository_UpdateProfile_persists_and_clears(t *testing.T) {
	db := testJobDB(t)
	repo := NewUsersRepository(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, "clerk_"+uuid.NewString()[:8], "user@example.com", "Old Name", models.RoleUser)
	require.NoError(t, err)

	got, err := repo.UpdateProfile(ctx, created.ID, models.UserProfileInput{
		FullName:     strPtr("Jane Doe"),
		Headline:     strPtr("Backend Engineer"),
		Phone:        strPtr("+62 812"),
		PortfolioURL: strPtr("https://jane.dev"),
		LinkedInURL:  strPtr("https://linkedin.com/in/jane"),
		GitHubURL:    strPtr("https://github.com/jane"),
	})
	require.NoError(t, err)
	require.NotNil(t, got.FullName)
	assert.Equal(t, "Jane Doe", *got.FullName)
	require.NotNil(t, got.Headline)
	assert.Equal(t, "Backend Engineer", *got.Headline)
	require.NotNil(t, got.Phone)
	assert.Equal(t, "+62 812", *got.Phone)
	require.NotNil(t, got.PortfolioURL)
	assert.Equal(t, "https://jane.dev", *got.PortfolioURL)
	require.NotNil(t, got.LinkedInURL)
	assert.Equal(t, "https://linkedin.com/in/jane", *got.LinkedInURL)
	require.NotNil(t, got.GitHubURL)
	assert.Equal(t, "https://github.com/jane", *got.GitHubURL)
	require.NotNil(t, got.Email)
	assert.Equal(t, "user@example.com", *got.Email)

	got, err = repo.UpdateProfile(ctx, created.ID, models.UserProfileInput{})
	require.NoError(t, err)
	assert.Nil(t, got.FullName)
	assert.Nil(t, got.Headline)
	assert.Nil(t, got.Phone)
	assert.Nil(t, got.PortfolioURL)
	assert.Nil(t, got.LinkedInURL)
	assert.Nil(t, got.GitHubURL)
	require.NotNil(t, got.Email)
	assert.Equal(t, "user@example.com", *got.Email)
}
