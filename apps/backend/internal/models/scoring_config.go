package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CategoryConfig represents a single scoring category with its weight and description.
type CategoryConfig struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// ScoringConfig represents a versioned configuration for evaluating candidates against jobs in a specific family.
type ScoringConfig struct {
	ID         uuid.UUID        `json:"id"`
	Family     string           `json:"family"`
	Version    int              `json:"version"`
	Categories []CategoryConfig `json:"categories"`
	IsActive   bool             `json:"is_active"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// CreateScoringConfigParams contains inputs to create a new scoring configuration.
type CreateScoringConfigParams struct {
	Family     string           `json:"family"`
	Version    int              `json:"version"`
	Categories []CategoryConfig `json:"categories"`
	IsActive   *bool            `json:"is_active,omitempty"`
}

// UpdateScoringConfigParams contains inputs to update a scoring configuration.
type UpdateScoringConfigParams struct {
	Categories []CategoryConfig `json:"categories,omitempty"`
	IsActive   *bool            `json:"is_active,omitempty"`
}

// ListScoringConfigsParams defines filters for listing scoring configurations.
type ListScoringConfigsParams struct {
	Family   *string
	IsActive *bool
}

// ValidateCategories validates category configurations:
// - Non-empty
// - Unique names
// - Non-negative weights
// - Weights sum to 1.0 (with floating point tolerance 0.999 to 1.001)
func ValidateCategories(categories []CategoryConfig) error {
	if len(categories) == 0 {
		return errors.New("at least one category is required")
	}

	seen := make(map[string]bool, len(categories))
	var totalWeight float64

	for _, cat := range categories {
		trimmedName := strings.TrimSpace(cat.Name)
		if trimmedName == "" {
			return errors.New("category name cannot be empty")
		}

		lowerName := strings.ToLower(trimmedName)
		if seen[lowerName] {
			return fmt.Errorf("duplicate category name: %s", cat.Name)
		}
		seen[lowerName] = true

		if cat.Weight < 0 {
			return fmt.Errorf("negative weight for category: %s", cat.Name)
		}

		totalWeight += cat.Weight
	}

	if totalWeight < 0.999 || totalWeight > 1.001 {
		return fmt.Errorf("weights must sum to 1.0, got %f", totalWeight)
	}

	return nil
}
