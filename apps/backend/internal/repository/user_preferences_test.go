package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

func TestUserPreferences_GetActivePreferences(t *testing.T) {
	db := testJobDB(t)
	repo := NewUserPreferencesRepository(database.NewPG(db))
	ctx := context.Background()

	userA := "user_a_" + uuid.NewString()[:8]
	userB := "user_b_" + uuid.NewString()[:8]
	userC := "user_c_" + uuid.NewString()[:8]

	// Insert active preference for userA
	_, err := db.Exec(ctx, `
		INSERT INTO user_preferences (user_id, keywords, locations, boards, is_active)
		VALUES ($1, ARRAY['golang', 'php'], ARRAY['indonesia'], ARRAY['glassdoor'], TRUE)
	`, userA)
	require.NoError(t, err)

	// Insert active preference for userB
	_, err = db.Exec(ctx, `
		INSERT INTO user_preferences (user_id, keywords, locations, boards, is_active)
		VALUES ($1, ARRAY['golang'], ARRAY['malaysia', 'indonesia'], ARRAY['linkedin', 'glassdoor'], TRUE)
	`, userB)
	require.NoError(t, err)

	// Insert inactive preference for userC
	_, err = db.Exec(ctx, `
		INSERT INTO user_preferences (user_id, keywords, locations, boards, is_active)
		VALUES ($1, ARRAY['rust'], ARRAY['singapore'], ARRAY['glassdoor'], FALSE)
	`, userC)
	require.NoError(t, err)

	t.Run("query by specific user IDs", func(t *testing.T) {
		prefs, err := repo.GetActivePreferences(ctx, []string{userA})
		require.NoError(t, err)
		require.Len(t, prefs, 1)
		assert.Equal(t, userA, prefs[0].UserID)
		assert.Equal(t, []string{"golang", "php"}, prefs[0].Keywords)
		assert.Equal(t, []string{"indonesia"}, prefs[0].Locations)
		assert.Equal(t, []string{"glassdoor"}, prefs[0].Boards)
	})

	t.Run("query inactive user returns empty", func(t *testing.T) {
		prefs, err := repo.GetActivePreferences(ctx, []string{userC})
		require.NoError(t, err)
		assert.Empty(t, prefs)
	})

	t.Run("query all active preferences", func(t *testing.T) {
		prefs, err := repo.GetActivePreferences(ctx, nil)
		require.NoError(t, err)

		// Both userA and userB should be among active preferences
		foundA := false
		foundB := false
		foundC := false
		for _, p := range prefs {
			if p.UserID == userA {
				foundA = true
			}
			if p.UserID == userB {
				foundB = true
			}
			if p.UserID == userC {
				foundC = true
			}
		}
		assert.True(t, foundA)
		assert.True(t, foundB)
		assert.False(t, foundC)
	})

	t.Run("query all active preferences with empty slice", func(t *testing.T) {
		prefs, err := repo.GetActivePreferences(ctx, []string{})
		require.NoError(t, err)

		foundA := false
		foundB := false
		for _, p := range prefs {
			if p.UserID == userA {
				foundA = true
			}
			if p.UserID == userB {
				foundB = true
			}
		}
		assert.True(t, foundA)
		assert.True(t, foundB)
	})
}
