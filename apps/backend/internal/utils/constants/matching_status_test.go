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
			name:      "new to pending is allowed",
			current:   MatchingStatusNew,
			next:      MatchingStatusPending,
			expectErr: false,
		},
		{
			name:      "new-retry to pending is allowed",
			current:   MatchingStatusNewRetry,
			next:      MatchingStatusPending,
			expectErr: false,
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
