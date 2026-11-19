package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMatchingStatusTransition(t *testing.T) {
	tests := []struct {
		name      string
		current   MatchingEvaluationStatus
		next      MatchingEvaluationStatus
		expectErr bool
	}{
		{
			name:      "queued to pending is allowed",
			current:   MatchingStatusQueued,
			next:      MatchingStatusPending,
			expectErr: false,
		},
		{
			name:      "queued_retry to pending is allowed",
			current:   MatchingStatusQueuedRetry,
			next:      MatchingStatusPending,
			expectErr: false,
		},
		{
			name:      "new to pending is allowed",
			current:   MatchingStatusNew,
			next:      MatchingStatusPending,
			expectErr: false,
		},
		{
			name:      "new to queued is allowed",
			current:   MatchingStatusNew,
			next:      MatchingStatusQueued,
			expectErr: false,
		},
		{
			name:      "failed to queued_retry is allowed",
			current:   MatchingStatusFailed,
			next:      MatchingStatusQueuedRetry,
			expectErr: false,
		},
		{
			name:      "success to new is rejected",
			current:   MatchingStatusSuccess,
			next:      MatchingStatusNew,
			expectErr: true,
		},
		{
			name:      "new-retry to pending is rejected",
			current:   MatchingEvaluationStatus("new-retry"),
			next:      MatchingStatusPending,
			expectErr: true,
		},
		{
			name:      "pending to pending is rejected",
			current:   MatchingStatusPending,
			next:      MatchingStatusPending,
			expectErr: true,
		},
		{
			name:      "failed to pending is rejected",
			current:   MatchingStatusFailed,
			next:      MatchingStatusPending,
			expectErr: true,
		},
		{
			name:      "success to pending is rejected",
			current:   MatchingStatusSuccess,
			next:      MatchingStatusPending,
			expectErr: true,
		},
		{
			name:      "pending to success is allowed",
			current:   MatchingStatusPending,
			next:      MatchingStatusSuccess,
			expectErr: false,
		},
		{
			name:      "pending to failed is allowed",
			current:   MatchingStatusPending,
			next:      MatchingStatusFailed,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMatchingStatusTransition(tt.current, tt.next)
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "transition from")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCoverLetterValidationHelpers(t *testing.T) {
	for _, status := range []string{
		string(CoverLetterStatusQueued),
		string(CoverLetterStatusQueuedRetry),
		string(CoverLetterStatusPending),
		string(CoverLetterStatusSuccess),
		string(CoverLetterStatusFailed),
	} {
		assert.True(t, IsValidCoverLetterStatus(status), status)
	}
	assert.False(t, IsValidCoverLetterStatus("new"))
	assert.False(t, IsValidCoverLetterStatus(""))

	assert.True(t, IsValidCoverLetterVariant(string(CoverLetterVariantShort)))
	assert.True(t, IsValidCoverLetterVariant(string(CoverLetterVariantFull)))
	assert.False(t, IsValidCoverLetterVariant("medium"))
}
