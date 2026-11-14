package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

type mockTemporalWorkflowClient struct {
	executeFunc           func(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	signalWithStartFunc   func(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
	describeExecutionFunc func(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
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

func (m *mockTemporalWorkflowClient) DescribeWorkflowExecution(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	if m.describeExecutionFunc != nil {
		return m.describeExecutionFunc(ctx, workflowID, runID)
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

func TestWorkflowsRepository_IsWorkflowRunning_NilClient(t *testing.T) {
	repo := NewWorkflowsRepository(nil, "test-queue")
	running, err := repo.IsWorkflowRunning(context.Background(), "job-analyze-1")
	require.Error(t, err)
	assert.False(t, running)
	assert.Contains(t, err.Error(), "temporal client not configured")
}

func TestWorkflowsRepository_IsWorkflowRunning_Running(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		describeExecutionFunc: func(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
			assert.Equal(t, "job-analyze-101", workflowID)
			assert.Empty(t, runID)
			return &workflowservice.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{
					Status: enums.WORKFLOW_EXECUTION_STATUS_RUNNING,
				},
			}, nil
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	running, err := repo.IsWorkflowRunning(context.Background(), "job-analyze-101")
	require.NoError(t, err)
	assert.True(t, running)
}

func TestWorkflowsRepository_IsWorkflowRunning_Completed(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		describeExecutionFunc: func(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
			return &workflowservice.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{
					Status: enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
				},
			}, nil
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	running, err := repo.IsWorkflowRunning(context.Background(), "job-analyze-102")
	require.NoError(t, err)
	assert.False(t, running)
}

func TestWorkflowsRepository_IsWorkflowRunning_NotFound(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		describeExecutionFunc: func(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
			return nil, serviceerror.NewNotFound("workflow execution not found")
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	running, err := repo.IsWorkflowRunning(context.Background(), "job-analyze-103")
	require.NoError(t, err)
	assert.False(t, running)
}

func TestWorkflowsRepository_IsWorkflowRunning_OtherError(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		describeExecutionFunc: func(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
			return nil, errors.New("temporal grpc connection failure")
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	running, err := repo.IsWorkflowRunning(context.Background(), "job-analyze-104")
	require.Error(t, err)
	assert.False(t, running)
	assert.Contains(t, err.Error(), "describe workflow execution")
}

