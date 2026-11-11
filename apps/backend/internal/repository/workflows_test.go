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
	executeFunc func(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
}

func (m *mockTemporalWorkflowClient) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, options, workflow, args...)
	}
	return nil, nil
}

func TestWorkflowsRepository_StartJobAnalyze_nil_client(t *testing.T) {
	repo := NewWorkflowsRepository(nil, "test-queue")
	_, err := repo.StartJobAnalyze(context.Background(), "wf-1", models.JobAnalyzeWorkflowInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temporal client not configured")
}

func TestWorkflowsRepository_StartJobAnalyze_success(t *testing.T) {
	var capturedOptions client.StartWorkflowOptions
	var capturedWorkflow string
	var capturedInput models.JobAnalyzeWorkflowInput

	mockClient := &mockTemporalWorkflowClient{
		executeFunc: func(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
			capturedOptions = options
			capturedWorkflow = workflow.(string)
			capturedInput = args[0].(models.JobAnalyzeWorkflowInput)
			return nil, nil
		},
	}

	repo := NewWorkflowsRepository(mockClient, "custom-queue")
	input := models.JobAnalyzeWorkflowInput{
		JobDescription: "Go Backend Developer",
		JobID:          "123",
		Slug:           "go-dev",
	}

	wfID, err := repo.StartJobAnalyze(context.Background(), "job-analyze-123", input)
	require.NoError(t, err)
	assert.Equal(t, "job-analyze-123", wfID)
	assert.Equal(t, "job-analyze-123", capturedOptions.ID)
	assert.Equal(t, "custom-queue", capturedOptions.TaskQueue)
	assert.Equal(t, "JobAnalyzeWorkflow", capturedWorkflow)
	assert.Equal(t, "Go Backend Developer", capturedInput.JobDescription)
	assert.Equal(t, "123", capturedInput.JobID)
	assert.Equal(t, "go-dev", capturedInput.Slug)
}

func TestWorkflowsRepository_StartJobAnalyze_already_started(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		executeFunc: func(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
			return nil, &serviceerror.WorkflowExecutionAlreadyStarted{
				Message: "Workflow already running",
			}
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	wfID, err := repo.StartJobAnalyze(context.Background(), "job-analyze-456", models.JobAnalyzeWorkflowInput{})
	require.NoError(t, err)
	assert.Equal(t, "job-analyze-456", wfID)
}

func TestWorkflowsRepository_StartJobAnalyze_failure(t *testing.T) {
	mockClient := &mockTemporalWorkflowClient{
		executeFunc: func(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
			return nil, errors.New("connection reset by peer")
		},
	}

	repo := NewWorkflowsRepository(mockClient, "test-queue")
	_, err := repo.StartJobAnalyze(context.Background(), "job-analyze-789", models.JobAnalyzeWorkflowInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute temporal workflow")
}
