package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

func getValidOutput() models.MatchEvaluationOutput {
	return models.MatchEvaluationOutput{
		Evaluations: []models.CategoryEvaluation{
			{
				Category: "technical_skills",
				Score:    90,
				Reason:   "Strong background in Go",
			},
			{
				Category: "experience",
				Score:    80,
				Reason:   "6 years experience",
			},
		},
		MandatoryRequirements: []models.MandatoryRequirementAssessment{
			{
				Requirement: "Golang",
				Status:      "met",
				Reason:      "Demonstrated in past jobs",
			},
		},
	}
}

func TestValidateEvaluationOutput(t *testing.T) {
	configs := []models.CategoryConfig{
		{Name: "technical_skills", Weight: 0.5},
		{Name: "experience", Weight: 0.5},
	}

	t.Run("valid output passes", func(t *testing.T) {
		err := models.ValidateEvaluationOutput(configs, getValidOutput())
		assert.NoError(t, err)
	})

	t.Run("missing category fails", func(t *testing.T) {
		badOutput := getValidOutput()
		badOutput.Evaluations = []models.CategoryEvaluation{
			{
				Category: "technical_skills",
				Score:    90,
				Reason:   "Only one category",
			},
		}
		err := models.ValidateEvaluationOutput(configs, badOutput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing evaluation for category: experience")
	})

	t.Run("duplicate category fails", func(t *testing.T) {
		badOutput := getValidOutput()
		badOutput.Evaluations = append(badOutput.Evaluations, models.CategoryEvaluation{
			Category: "technical_skills",
			Score:    85,
			Reason:   "Duplicate",
		})
		err := models.ValidateEvaluationOutput(configs, badOutput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate evaluation for category")
	})

	t.Run("unexpected category fails", func(t *testing.T) {
		badOutput := getValidOutput()
		badOutput.Evaluations = append(badOutput.Evaluations, models.CategoryEvaluation{
			Category: "unexpected_cat",
			Score:    50,
			Reason:   "Extra category",
		})
		err := models.ValidateEvaluationOutput(configs, badOutput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected evaluation category")
	})

	t.Run("score below 0 fails", func(t *testing.T) {
		badOutput := getValidOutput()
		badOutput.Evaluations[0].Score = -5
		err := models.ValidateEvaluationOutput(configs, badOutput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "score must be between 0 and 100")
	})

	t.Run("score above 100 fails", func(t *testing.T) {
		badOutput := getValidOutput()
		badOutput.Evaluations[0].Score = 105
		err := models.ValidateEvaluationOutput(configs, badOutput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "score must be between 0 and 100")
	})

	t.Run("invalid requirement status fails", func(t *testing.T) {
		badOutput := getValidOutput()
		badOutput.MandatoryRequirements = []models.MandatoryRequirementAssessment{
			{
				Requirement: "Golang",
				Status:      "invalid_status",
				Reason:      "Bad status",
			},
		}
		err := models.ValidateEvaluationOutput(configs, badOutput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid requirement status")
	})
}
