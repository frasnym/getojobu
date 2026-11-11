package models_test

import (
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestValidateCategories_Valid(t *testing.T) {
	cats := []models.CategoryConfig{
		{Name: "technical_skills", Weight: 0.35, Description: "skills"},
		{Name: "responsibilities", Weight: 0.25, Description: "resp"},
		{Name: "experience", Weight: 0.20, Description: "exp"},
		{Name: "seniority", Weight: 0.15, Description: "sen"},
		{Name: "domain", Weight: 0.05, Description: "dom"},
	}
	err := models.ValidateCategories(cats)
	assert.NoError(t, err)
}

func TestValidateCategories_InvalidSum(t *testing.T) {
	cats := []models.CategoryConfig{
		{Name: "technical_skills", Weight: 0.50, Description: "skills"},
		{Name: "responsibilities", Weight: 0.20, Description: "resp"},
	}
	err := models.ValidateCategories(cats)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weights must sum to 1.0")
}

func TestValidateCategories_DuplicateName(t *testing.T) {
	cats := []models.CategoryConfig{
		{Name: "skills", Weight: 0.50, Description: "skills 1"},
		{Name: "skills", Weight: 0.50, Description: "skills 2"},
	}
	err := models.ValidateCategories(cats)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate category")
}

func TestValidateCategories_NegativeWeight(t *testing.T) {
	cats := []models.CategoryConfig{
		{Name: "skills", Weight: -0.10, Description: "skills"},
		{Name: "domain", Weight: 1.10, Description: "domain"},
	}
	err := models.ValidateCategories(cats)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "negative weight")
}

func TestValidateCategories_EmptyCategories(t *testing.T) {
	err := models.ValidateCategories(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one category is required")
}
