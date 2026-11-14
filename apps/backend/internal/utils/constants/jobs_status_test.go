package constants

import (
	"strings"
	"testing"
)

func TestValidateJobStatusTransition(t *testing.T) {
	t.Parallel()

	t.Run("allowed_paths", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			current JobStatus
			next    JobStatus
		}{
			{"blank_to_analyze_pending", "", StatusAnalyzePending},
			{"new_to_analyze_pending", StatusNew, StatusAnalyzePending},
			{"new_retry_to_analyze_pending", StatusNewRetry, StatusAnalyzePending},
			{"analyze_pending_to_analyzed", StatusAnalyzePending, StatusAnalyzed},
			{"analyze_pending_to_failed", StatusAnalyzePending, StatusFailed},
			{"analyze_pending_to_new_retry", StatusAnalyzePending, StatusNewRetry},
			{"failed_to_new_retry", StatusFailed, StatusNewRetry},
			{"failed_to_analyze_pending_retry", StatusFailed, StatusAnalyzePending},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if err := ValidateJobStatusTransition(tc.current, tc.next); err != nil {
					t.Errorf("expected no error for %q → %q, got: %v", tc.current, tc.next, err)
				}
			})
		}
	})

	t.Run("blocked_paths", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			current     JobStatus
			next        JobStatus
			wantContain string
		}{
			{
				name:        "analyzed_to_analyze_pending",
				current:     StatusAnalyzed,
				next:        StatusAnalyzePending,
				wantContain: "already finalized",
			},
			{
				name:        "analyzed_to_failed",
				current:     StatusAnalyzed,
				next:        StatusFailed,
				wantContain: "already finalized",
			},
			{
				name:        "analyzed_to_new",
				current:     StatusAnalyzed,
				next:        StatusNew,
				wantContain: "already finalized",
			},
			{
				name:        "failed_to_analyzed_directly",
				current:     StatusFailed,
				next:        StatusAnalyzed,
				wantContain: "failed jobs may only be retried",
			},
			{
				name:        "failed_to_new",
				current:     StatusFailed,
				next:        StatusNew,
				wantContain: "failed jobs may only be retried",
			},
			{
				name:        "new_to_analyzed_skip_pending",
				current:     StatusNew,
				next:        StatusAnalyzed,
				wantContain: "analyze_pending",
			},
			{
				name:        "new_to_failed_skip_pending",
				current:     StatusNew,
				next:        StatusFailed,
				wantContain: "analyze_pending",
			},
			{
				name:        "unknown_current_status",
				current:     "bogus_status",
				next:        StatusAnalyzePending,
				wantContain: "unknown current status",
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateJobStatusTransition(tc.current, tc.next)
				if err == nil {
					t.Fatalf("expected an error for %q → %q, got nil", tc.current, tc.next)
				}
				if !strings.Contains(err.Error(), tc.wantContain) {
					t.Errorf(
						"error for %q → %q = %q; want it to contain %q",
						tc.current, tc.next, err.Error(), tc.wantContain,
					)
				}
			})
		}
	})
}
