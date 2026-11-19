package matching

import (
	"context"
	"fmt"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
)

// requireStaleNotRunning guards a forced re-queue of a row left in an
// in-progress state: it must be at least 30 minutes old and its workflow must
// have finished.
func (s *matchingService) requireStaleNotRunning(ctx context.Context, updatedAt time.Time, workflowID, subject string) error {
	if time.Since(updatedAt) <= 30*time.Minute {
		return fmt.Errorf("%w: %s has been queued for less than 30 minutes", constants.ErrInvalidRequest, subject)
	}
	running, err := s.workflowsRepo.IsWorkflowRunning(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("check workflow status: %w", err)
	}
	if running {
		return fmt.Errorf("%w: %s workflow is currently running", constants.ErrInvalidRequest, subject)
	}
	return nil
}
