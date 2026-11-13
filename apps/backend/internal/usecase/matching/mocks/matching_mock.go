package mocks

import (
	context "context"
	reflect "reflect"

	models "github.com/frasnym/getojobu/apps/backend/internal/models"
	matching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

// MockMatching is a mock of Matching interface.
type MockMatching struct {
	ctrl     *gomock.Controller
	recorder *MockMatchingMockRecorder
}

// MockMatchingMockRecorder is the mock recorder for MockMatching.
type MockMatchingMockRecorder struct {
	mock *MockMatching
}

// NewMockMatching creates a new mock instance.
func NewMockMatching(ctrl *gomock.Controller) *MockMatching {
	mock := &MockMatching{ctrl: ctrl}
	mock.recorder = &MockMatchingMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMatching) EXPECT() *MockMatchingMockRecorder {
	return m.recorder
}

// TriggerCompatibilityBulk mocks base method.
func (m *MockMatching) TriggerCompatibilityBulk(ctx context.Context, items []models.BulkAnalyzeCompatibilityItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TriggerCompatibilityBulk", ctx, items, caller)
	ret0, _ := ret[0].([]*models.MatchingEvaluationSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TriggerCompatibilityBulk indicates an expected call of TriggerCompatibilityBulk.
func (mr *MockMatchingMockRecorder) TriggerCompatibilityBulk(ctx, items, caller any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TriggerCompatibilityBulk", reflect.TypeOf((*MockMatching)(nil).TriggerCompatibilityBulk), ctx, items, caller)
}

// UpdateEvaluation mocks base method.
func (m *MockMatching) UpdateEvaluation(ctx context.Context, id uuid.UUID, req matching.UpdateEvaluationRequest) (*models.MatchingEvaluation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEvaluation", ctx, id, req)
	ret0, _ := ret[0].(*models.MatchingEvaluation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateEvaluation indicates an expected call of UpdateEvaluation.
func (mr *MockMatchingMockRecorder) UpdateEvaluation(ctx, id, req any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEvaluation", reflect.TypeOf((*MockMatching)(nil).UpdateEvaluation), ctx, id, req)
}

// UpdateEvaluationReport mocks base method.
func (m *MockMatching) UpdateEvaluationReport(ctx context.Context, id uuid.UUID, req matching.UpdateEvaluationReportRequest) (*models.MatchingEvaluation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEvaluationReport", ctx, id, req)
	ret0, _ := ret[0].(*models.MatchingEvaluation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateEvaluationReport indicates an expected call of UpdateEvaluationReport.
func (mr *MockMatchingMockRecorder) UpdateEvaluationReport(ctx, id, req any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEvaluationReport", reflect.TypeOf((*MockMatching)(nil).UpdateEvaluationReport), ctx, id, req)
}

// GetReport mocks base method.
func (m *MockMatching) GetReport(ctx context.Context, id uuid.UUID, caller *models.Principal) (*models.MatchingEvaluationReportResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetReport", ctx, id, caller)
	ret0, _ := ret[0].(*models.MatchingEvaluationReportResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetReport indicates an expected call of GetReport.
func (mr *MockMatchingMockRecorder) GetReport(ctx, id, caller any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetReport", reflect.TypeOf((*MockMatching)(nil).GetReport), ctx, id, caller)
}

// BulkQueueGenerateReport mocks base method.
func (m *MockMatching) BulkQueueGenerateReport(ctx context.Context, items []models.BulkGenerateReportItem, caller *models.Principal) ([]*models.MatchingEvaluationSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BulkQueueGenerateReport", ctx, items, caller)
	ret0, _ := ret[0].([]*models.MatchingEvaluationSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BulkQueueGenerateReport indicates an expected call of BulkQueueGenerateReport.
func (mr *MockMatchingMockRecorder) BulkQueueGenerateReport(ctx, items, caller any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BulkQueueGenerateReport", reflect.TypeOf((*MockMatching)(nil).BulkQueueGenerateReport), ctx, items, caller)
}
