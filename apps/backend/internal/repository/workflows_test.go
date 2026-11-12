package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

type mockTemporalWorkflowClient struct {
	executeFunc         func(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	signalWithStartFunc func(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
}

func (m *mockTemporalWorkflowClient) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, options, workflow, args...)
	}
	return nil, nil
}

func (m *mockTemporalWorkflowClient) SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error) {
	if m.signalWithStartFunc != nil {
		return m.signalWithStartFunc(ctx, workflowID, signalName, signalArg, options, workflow, workflowArgs...)
	}
	return nil, nil
}

func TestWorkflowsRepository_StartLimiterWorkItem_nil_client(t *testing.T) {
	repo := NewWorkflowsRepository(nil, "test-queue")
	_, err := repo.StartLimiterWorkItem(context.Background(), "wf-1", models.LimiterJobTypeScrape, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temporal client not configured")
}

func TestWorkflowsRepository_StartLimiterWorkItem_enqueues_on_job_limit_queue(t *testing.T) {
	var capturedWorkflowID string
	var capturedSignalName string
	var capturedSignalArg interface{}
	var capturedOptions client.StartWorkflowOptions
	var capturedWorkflow string

	mockClient := &mockTemporalWorkflowClient{
		signalWithStartFunc: func(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error) {
			capturedWorkflowID = workflowID
			capturedSignalName = signalName
			capturedSignalArg = signalArg
			capturedOptions = options
			capturedWorkflow = workflow.(string)
			return nil, nil
		},
	}

	repo := NewWorkflowsRepository(mockClient, "custom-queue")
	input := models.JobScrapeWorkflowInput{
		Board:   "glassdoor",
		Keyword: "golang",
	}

	itemID, err := repo.StartLimiterWorkItem(context.Background(), "job-scrape-123", models.LimiterJobTypeScrape, input)
	require.NoError(t, err)
	assert.Equal(t, "job-scrape-123", itemID)

	assert.Equal(t, "job-concurrency-limiter", capturedWorkflowID)
	assert.Equal(t, "job-concurrency-limiter", capturedOptions.ID)
	assert.Equal(t, "job-limit", capturedOptions.TaskQueue)
	assert.Equal(t, "ConcurrencyLimiter", capturedWorkflow)
	assert.Equal(t, "enqueue", capturedSignalName)

	workItem, ok := capturedSignalArg.(models.LimiterWorkItem)
	require.True(t, ok)
	assert.Equal(t, "job-scrape-123", workItem.ItemID)
	assert.Equal(t, models.LimiterJobTypeScrape, workItem.JobType)
	assert.Equal(t, input, workItem.Params)
}

func TestWorkflowsRepository_StartLimiterWorkItem_already_started(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		signalWithStartFunc: func(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error) {
			return nil, &serviceerror.WorkflowExecutionAlreadyStarted{
				Message: "Workflow already running",
			}
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	itemID, err := repo.StartLimiterWorkItem(context.Background(), "job-analyze-456", models.LimiterJobTypeAnalyze, nil)
	require.NoError(t, err)
	assert.Equal(t, "job-analyze-456", itemID)
}

func TestWorkflowsRepository_StartLimiterWorkItem_failure(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		signalWithStartFunc: func(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error) {
			return nil, errors.New("temporal unavailable")
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	_, err := repo.StartLimiterWorkItem(context.Background(), "job-analyze-789", models.LimiterJobTypeAnalyze, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal with start limiter workflow")
}
