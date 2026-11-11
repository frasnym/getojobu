package repository

import (
	"context"
	"errors"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// WorkflowClient defines the client methods required to run workflows.
// client.Client implements this interface.
type WorkflowClient interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
}

// Workflows defines operations for triggering Temporal workflows.
type Workflows interface {
	StartJobAnalyze(ctx context.Context, workflowID string, input models.JobAnalyzeWorkflowInput) (string, error)
}

type workflowsRepository struct {
	client    WorkflowClient
	taskQueue string
}

// NewWorkflowsRepository creates a Workflows repository backed by the Temporal client.
func NewWorkflowsRepository(tc WorkflowClient, taskQueue string) Workflows {
	if taskQueue == "" {
		taskQueue = "job-evaluation"
	}
	return &workflowsRepository{
		client:    tc,
		taskQueue: taskQueue,
	}
}

// StartJobAnalyze starts the JobAnalyzeWorkflow in Temporal with deduplication handling.
func (r *workflowsRepository) StartJobAnalyze(ctx context.Context, workflowID string, input models.JobAnalyzeWorkflowInput) (string, error) {
	if r.client == nil {
		return "", errors.New("temporal client not configured")
	}

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: r.taskQueue,
	}

	_, err := r.client.ExecuteWorkflow(ctx, options, "JobAnalyzeWorkflow", input)
	if err != nil {
		if temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			return workflowID, nil
		}
		return "", fmt.Errorf("execute temporal workflow: %w", err)
	}

	return workflowID, nil
}
