package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

func marshalKeys(t *testing.T, v any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(v)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func sampleJob() *models.Job {
	return &models.Job{
		ID:        648,
		Slug:      "german-workspace-ai-reviewer-vienna",
		Status:    "new",
		Title:     "German Workspace AI Reviewer – Vienna",
		Company:   "Alignerr",
		Location:  "Vienna, Vienna, Austria",
		CreatedAt: time.Date(2026, 11, 16, 7, 18, 5, 0, time.UTC),
		UpdatedAt: time.Date(2026, 11, 16, 7, 18, 5, 0, time.UTC),
	}
}

func TestJobResponse_omitsOwnershipForAnonymous(t *testing.T) {
	keys := marshalKeys(t, NewJobResponse(sampleJob(), nil))

	assert.NotContains(t, keys, "is_owned")
	assert.NotContains(t, keys, "user_job")
	assert.NotContains(t, keys, "is_hidden")
	assert.NotContains(t, keys, "user_job_id")
	assert.EqualValues(t, 648, keys["id"])
}

func TestJobResponse_stripsAnalyzeResultMetadata(t *testing.T) {
	job := sampleJob()
	job.AnalyzeResult = json.RawMessage(`{"family":"technology","metadata":{"model":"gpt-x","prompt_version":"extract-job-v1"}}`)

	keys := marshalKeys(t, NewJobResponse(job, nil))
	analyzeResult, ok := keys["analyze_result"].(map[string]any)
	require.True(t, ok, "analyze_result must be an object")
	assert.Equal(t, "technology", analyzeResult["family"])
	assert.NotContains(t, analyzeResult, "metadata")

	var stored map[string]any
	require.NoError(t, json.Unmarshal(job.AnalyzeResult, &stored))
	assert.Contains(t, stored, "metadata")
}

func TestJobResponse_keepsAnalyzeResultWithoutMetadata(t *testing.T) {
	job := sampleJob()
	job.AnalyzeResult = json.RawMessage(`{"family":"technology"}`)

	keys := marshalKeys(t, NewJobResponse(job, nil))
	analyzeResult, ok := keys["analyze_result"].(map[string]any)
	require.True(t, ok, "analyze_result must be an object")
	assert.Equal(t, "technology", analyzeResult["family"])
}

func TestJobResponse_includesOwnershipFlagWhenKnown(t *testing.T) {
	owned := false
	keys := marshalKeys(t, NewJobResponse(sampleJob(), &owned))

	assert.Equal(t, false, keys["is_owned"])
	assert.NotContains(t, keys, "user_job")
}

func TestJobDetailResponse_includesUserJobID(t *testing.T) {
	userJobID := uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737")
	owned := true

	keys := marshalKeys(t, NewJobDetailResponse(sampleJob(), &owned, &userJobID))

	assert.Equal(t, true, keys["is_owned"])
	userJob, ok := keys["user_job"].(map[string]any)
	require.True(t, ok, "user_job must be an object")
	assert.Equal(t, "ee46d5ea-893d-4951-a3d3-8c32d7459737", userJob["id"])
	assert.Len(t, userJob, 1, "user_job carries the link id only")
}

func TestJobDetailResponse_omitsUserJobWhenNotOwned(t *testing.T) {
	owned := false
	keys := marshalKeys(t, NewJobDetailResponse(sampleJob(), &owned, nil))

	assert.NotContains(t, keys, "user_job")
	assert.Equal(t, false, keys["is_owned"])
}

func TestJobDetailResponse_handlesNilJob(t *testing.T) {
	userJobID := uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737")
	res := NewJobDetailResponse(nil, nil, &userJobID)
	assert.Equal(t, int64(0), res.ID)
	assert.Nil(t, res.UserJob)
	assert.Nil(t, res.IsOwned)
}

func TestJobDetailResponse_omitsOwnershipForAnonymous(t *testing.T) {
	keys := marshalKeys(t, NewJobDetailResponse(sampleJob(), nil, nil))

	assert.NotContains(t, keys, "user_job")
	assert.NotContains(t, keys, "is_owned")
}

func TestUserJobResponse_rootKeys(t *testing.T) {
	evalID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	userJob := &models.UserJob{
		ID:                 uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737"),
		JobID:              648,
		IsHidden:           true,
		LatestEvaluationID: &evalID,
		CreatedAt:          time.Date(2026, 11, 16, 7, 20, 11, 0, time.UTC),
		UpdatedAt:          time.Date(2026, 11, 16, 7, 21, 0, 0, time.UTC),
	}

	evalStatus := "completed"
	eligibilityStatus := "eligible"
	recommendationLabel := "strong_match"
	reportStatus := "ready"
	resumeStatus := "generated"
	coverLetterStatus := "success"

	row := &models.UserJobWithJob{
		UserJob: userJob,
		Job:     sampleJob(),
		Evaluation: &models.MatchingEvaluation{
			ID:                  evalID,
			UserJobID:           userJob.ID,
			EvaluationStatus:    evalStatus,
			EligibilityStatus:   &eligibilityStatus,
			RecommendationLabel: &recommendationLabel,
			ReportStatus:        &reportStatus,
			ResumeStatus:        &resumeStatus,
			CoverLetterStatus:   &coverLetterStatus,
		},
	}

	keys := marshalKeys(t, NewUserJobResponse(row))

	for _, want := range []string{"id", "is_hidden", "created_at", "updated_at", "job", "evaluation"} {
		assert.Contains(t, keys, want)
	}
	for _, unwanted := range []string{"title", "slug", "is_owned", "user_job_id"} {
		assert.NotContains(t, keys, unwanted)
	}

	assert.Equal(t, "ee46d5ea-893d-4951-a3d3-8c32d7459737", keys["id"])
	assert.Equal(t, true, keys["is_hidden"])
	assert.Equal(t, "2026-11-16T07:20:11Z", keys["created_at"])

	job, ok := keys["job"].(map[string]any)
	require.True(t, ok, "job must be an object")
	assert.EqualValues(t, 648, job["id"])
	assert.NotContains(t, job, "is_owned")

	eval, ok := keys["evaluation"].(map[string]any)
	require.True(t, ok, "evaluation must be an object")
	assert.Equal(t, evalID.String(), eval["id"])
	assert.Equal(t, "completed", eval["evaluation_status"])
	assert.Equal(t, "eligible", eval["eligibility_status"])
	assert.Equal(t, "strong_match", eval["recommendation_label"])
	assert.Equal(t, "ready", eval["report_status"])
	assert.Equal(t, "generated", eval["resume_status"])
	assert.Equal(t, "success", eval["cover_letter_status"])

	// Verify omitempty when evaluation is nil
	emptyUserJob := &models.UserJob{
		ID:        userJob.ID,
		JobID:     userJob.JobID,
		IsHidden:  userJob.IsHidden,
		CreatedAt: userJob.CreatedAt,
		UpdatedAt: userJob.UpdatedAt,
	}
	emptyRow := &models.UserJobWithJob{
		UserJob: emptyUserJob,
		Job:     sampleJob(),
	}
	emptyKeys := marshalKeys(t, NewUserJobResponse(emptyRow))
	assert.NotContains(t, emptyKeys, "evaluation")
}

func TestMatchingEvaluationResponse_hasNoJobKey(t *testing.T) {
	eval := &models.MatchingEvaluation{
		ID:               uuid.MustParse("5f0c1d8e-0000-0000-0000-000000000000"),
		UserJobID:        uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737"),
		EvaluationStatus: "success",
		CreatedAt:        time.Date(2026, 11, 16, 7, 30, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 11, 16, 7, 30, 0, 0, time.UTC),
	}

	keys := marshalKeys(t, NewMatchingEvaluationResponse(eval))

	assert.NotContains(t, keys, "job")
	assert.Contains(t, keys, "id")
	assert.Contains(t, keys, "user_job_id")
	assert.Contains(t, keys, "evaluation_status")
}

func TestMatchingEvaluationResponse_mapsCoverLetterFields(t *testing.T) {
	status := "success"
	failure := "2026-11-17T10:00:00Z — Cover letter generation failed"
	eval := &models.MatchingEvaluation{
		EvaluationStatus:          "success",
		CoverLetterStatus:         &status,
		CoverLetterContent:        json.RawMessage(`{"text":"Dear Hiring Manager, ...","metadata":{"model":"use-combo","prompt_version":"generate-cover-letter-short-v1"}}`),
		CoverLetterFailureMessage: &failure,
		CoverLetterFileName:       "Cover Letter_Jane Doe_Software Engineer.pdf",
	}

	resp := NewMatchingEvaluationResponse(eval)

	require.NotNil(t, resp.CoverLetterText)
	assert.Equal(t, "Dear Hiring Manager, ...", *resp.CoverLetterText)
	assert.Equal(t, &status, resp.CoverLetterStatus)
	assert.Equal(t, &failure, resp.CoverLetterFailureMessage)
	assert.Equal(t, "Cover Letter_Jane Doe_Software Engineer.pdf", resp.CoverLetterFileName)

	keys := marshalKeys(t, resp)
	assert.Contains(t, keys, "cover_letter_status")
	assert.Contains(t, keys, "cover_letter_text")
	assert.NotContains(t, keys, "cover_letter_content")
	assert.Contains(t, keys, "cover_letter_failure_message")
	assert.Contains(t, keys, "cover_letter_file_name")
}

func TestEvaluationResponse_includesJob(t *testing.T) {
	eval := &models.MatchingEvaluation{
		ID:               uuid.MustParse("5f0c1d8e-0000-0000-0000-000000000000"),
		UserJobID:        uuid.MustParse("ee46d5ea-893d-4951-a3d3-8c32d7459737"),
		EvaluationStatus: "success",
	}

	keys := marshalKeys(t, NewEvaluationResponsePublic(eval, sampleJob()))

	assert.Contains(t, keys, "job")
	assert.Contains(t, keys, "evaluation")
	assert.NotContains(t, keys, "user_job")
	job, ok := keys["job"].(map[string]any)
	require.True(t, ok, "job must be an object")
	assert.EqualValues(t, 648, job["id"])

	// Evaluation is nested under its own key, not merged at root.
	evalObj, ok := keys["evaluation"].(map[string]any)
	require.True(t, ok, "evaluation must be an object")
	assert.Equal(t, "5f0c1d8e-0000-0000-0000-000000000000", evalObj["id"])
	assert.Equal(t, "success", evalObj["evaluation_status"])
	assert.NotContains(t, keys, "evaluation_status")
	assert.Contains(t, keys, "user_job_id")
}

func TestEvaluationResponse_omitsResumeKeysOnlyForCandidates(t *testing.T) {
	pdfKey := "resumes/resume.pdf"
	jsonKey := "resumes/resume.json"
	eval := &models.MatchingEvaluation{
		EvaluationStatus: "success",
		ResumePDFKey:     &pdfKey,
		ResumeJSONKey:    &jsonKey,
	}

	workerKeys := marshalKeys(t, NewMatchingEvaluationResponse(eval))
	assert.Contains(t, workerKeys, "resume_pdf_key")
	assert.Contains(t, workerKeys, "resume_json_key")

	candidateKeys := marshalKeys(t, NewEvaluationResponsePublic(eval, sampleJob()))
	evalObj, ok := candidateKeys["evaluation"].(map[string]any)
	require.True(t, ok, "evaluation must be an object")
	assert.NotContains(t, evalObj, "resume_pdf_key")
	assert.NotContains(t, evalObj, "resume_json_key")
}

func TestCreateJobResponse_snakeCase(t *testing.T) {
	result := &models.CreateJobResult{Job: sampleJob(), Created: true, AddedToUserList: true}

	keys := marshalKeys(t, NewCreateJobResponse(result))

	assert.Contains(t, keys, "job")
	assert.Contains(t, keys, "created")
	assert.Contains(t, keys, "added_to_user_list")
	assert.NotContains(t, keys, "Job")
	assert.NotContains(t, keys, "Created")
}
