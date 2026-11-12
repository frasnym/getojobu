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
	SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
}

// Workflows defines operations for triggering Temporal workflows.
type Workflows interface {
	StartLimiterWorkItem(ctx context.Context, itemID string, itemType models.LimiterJobType, input interface{}) (string, error)
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

const (
	LimiterWorkflowID   = "job-concurrency-limiter"
	LimiterWorkflowType = "ConcurrencyLimiter"
	LimiterSignalName   = "enqueue"
	LimiterTaskQueue    = "job-limit"
)

// StartLimiterWorkItem enqueues a work item to the singleton ConcurrencyLimiter workflow
// via SignalWithStartWorkflow on the "job-limit" task queue.
func (r *workflowsRepository) StartLimiterWorkItem(ctx context.Context, itemID string, itemType models.LimiterJobType, input interface{}) (string, error) {
	if r.client == nil {
		return "", errors.New("temporal client not configured")
	}

	options := client.StartWorkflowOptions{
		ID:        LimiterWorkflowID,
		TaskQueue: LimiterTaskQueue,
	}

	signalArg := models.LimiterWorkItem{
		ItemID:  itemID,
		JobType: itemType,
		Params:  input,
	}

	_, err := r.client.SignalWithStartWorkflow(ctx, LimiterWorkflowID, LimiterSignalName, signalArg, options, LimiterWorkflowType)
	if err != nil {
		if temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			return itemID, nil
		}
		return "", fmt.Errorf("signal with start limiter workflow: %w", err)
	}

	return itemID, nil
}
