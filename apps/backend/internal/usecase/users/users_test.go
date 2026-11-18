package users

import (
	"context"
	"errors"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repoMocks "github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func strPtr(s string) *string {
	return &s
}

func TestUserService_UpdateProfile_delegates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := repoMocks.NewMockUsers(ctrl)
	s := NewUserService(repo)

	input := models.UserProfileInput{
		FullName:     strPtr("Jane Doe"),
		Headline:     strPtr("Backend Engineer"),
		Phone:        strPtr("+62 812"),
		PortfolioURL: strPtr("https://jane.dev"),
		LinkedInURL:  strPtr("https://linkedin.com/in/jane"),
		GitHubURL:    strPtr("https://github.com/jane"),
	}
	expected := &models.User{ID: "usr-1", FullName: strPtr("Jane Doe")}

	repo.EXPECT().
		UpdateProfile(gomock.Any(), "usr-1", input).
		Return(expected, nil)

	got, err := s.UpdateProfile(context.Background(), "usr-1", input)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestUserService_UpdateProfile_propagates_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := repoMocks.NewMockUsers(ctrl)
	s := NewUserService(repo)

	input := models.UserProfileInput{FullName: strPtr("Jane Doe")}

	repo.EXPECT().
		UpdateProfile(gomock.Any(), "usr-1", input).
		Return(nil, errors.New("db down"))

	got, err := s.UpdateProfile(context.Background(), "usr-1", input)

	require.Error(t, err)
	assert.Nil(t, got)
	assert.EqualError(t, err, "db down")
}
