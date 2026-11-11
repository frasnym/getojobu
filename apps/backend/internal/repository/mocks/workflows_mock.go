package mocks

import (
	context "context"
	reflect "reflect"

	models "github.com/frasnym/getojobu/apps/backend/internal/models"
	gomock "go.uber.org/mock/gomock"
)

// MockWorkflows is a mock of Workflows interface.
type MockWorkflows struct {
	ctrl     *gomock.Controller
	recorder *MockWorkflowsMockRecorder
}

// MockWorkflowsMockRecorder is the mock recorder for MockWorkflows.
type MockWorkflowsMockRecorder struct {
	mock *MockWorkflows
}

// NewMockWorkflows creates a new mock instance.
func NewMockWorkflows(ctrl *gomock.Controller) *MockWorkflows {
	mock := &MockWorkflows{ctrl: ctrl}
	mock.recorder = &MockWorkflowsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWorkflows) EXPECT() *MockWorkflowsMockRecorder {
	return m.recorder
}

// StartJobAnalyze mocks base method.
func (m *MockWorkflows) StartJobAnalyze(ctx context.Context, workflowID string, input models.JobAnalyzeWorkflowInput) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartJobAnalyze", ctx, workflowID, input)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartJobAnalyze indicates an expected call of StartJobAnalyze.
func (mr *MockWorkflowsMockRecorder) StartJobAnalyze(ctx, workflowID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartJobAnalyze", reflect.TypeOf((*MockWorkflows)(nil).StartJobAnalyze), ctx, workflowID, input)
}
