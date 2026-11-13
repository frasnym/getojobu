package mocks

import (
	context "context"
	reflect "reflect"

	models "github.com/frasnym/getojobu/apps/backend/internal/models"
	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

// MockMatchingEvaluations is a mock of MatchingEvaluations interface.
type MockMatchingEvaluations struct {
	ctrl     *gomock.Controller
	recorder *MockMatchingEvaluationsMockRecorder
}

// MockMatchingEvaluationsMockRecorder is the mock recorder for MockMatchingEvaluations.
type MockMatchingEvaluationsMockRecorder struct {
	mock *MockMatchingEvaluations
}

// NewMockMatchingEvaluations creates a new mock instance.
func NewMockMatchingEvaluations(ctrl *gomock.Controller) *MockMatchingEvaluations {
	mock := &MockMatchingEvaluations{ctrl: ctrl}
	mock.recorder = &MockMatchingEvaluationsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMatchingEvaluations) EXPECT() *MockMatchingEvaluationsMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockMatchingEvaluations) Create(ctx context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", ctx, eval)
	ret0, _ := ret[0].(*models.MatchingEvaluation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockMatchingEvaluationsMockRecorder) Create(ctx, eval any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockMatchingEvaluations)(nil).Create), ctx, eval)
}

// GetByID mocks base method.
func (m *MockMatchingEvaluations) GetByID(ctx context.Context, id uuid.UUID) (*models.MatchingEvaluation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByID", ctx, id)
	ret0, _ := ret[0].(*models.MatchingEvaluation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByID indicates an expected call of GetByID.
func (mr *MockMatchingEvaluationsMockRecorder) GetByID(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByID", reflect.TypeOf((*MockMatchingEvaluations)(nil).GetByID), ctx, id)
}

// GetLatestByUserAndJob mocks base method.
func (m *MockMatchingEvaluations) GetLatestByUserAndJob(ctx context.Context, userID uuid.UUID, jobID int64) (*models.MatchingEvaluation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestByUserAndJob", ctx, userID, jobID)
	ret0, _ := ret[0].(*models.MatchingEvaluation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLatestByUserAndJob indicates an expected call of GetLatestByUserAndJob.
func (mr *MockMatchingEvaluationsMockRecorder) GetLatestByUserAndJob(ctx, userID, jobID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestByUserAndJob", reflect.TypeOf((*MockMatchingEvaluations)(nil).GetLatestByUserAndJob), ctx, userID, jobID)
}

// Update mocks base method.
func (m *MockMatchingEvaluations) Update(ctx context.Context, id uuid.UUID, params models.UpdateMatchingEvaluationParams) (*models.MatchingEvaluation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", ctx, id, params)
	ret0, _ := ret[0].(*models.MatchingEvaluation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockMatchingEvaluationsMockRecorder) Update(ctx, id, params any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockMatchingEvaluations)(nil).Update), ctx, id, params)
}
