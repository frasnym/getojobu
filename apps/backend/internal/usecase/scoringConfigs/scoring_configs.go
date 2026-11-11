package scoringConfigs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
)

// ScoringConfig defines the business logic methods for scoring configurations.
type ScoringConfig interface {
	Create(ctx context.Context, params models.CreateScoringConfigParams) (*models.ScoringConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ScoringConfig, error)
	GetActiveByFamily(ctx context.Context, family string) (*models.ScoringConfig, error)
	List(ctx context.Context, params models.ListScoringConfigsParams) ([]*models.ScoringConfig, error)
	Update(ctx context.Context, id uuid.UUID, params models.UpdateScoringConfigParams) (*models.ScoringConfig, error)
}

type scoringConfigService struct {
	repo repo.ScoringConfigs
}

// NewScoringConfigService constructs a new scoring configuration usecase service.
func NewScoringConfigService(r repo.ScoringConfigs) ScoringConfig {
	return &scoringConfigService{
		repo: r,
	}
}

func (s *scoringConfigService) Create(ctx context.Context, params models.CreateScoringConfigParams) (*models.ScoringConfig, error) {
	family := strings.TrimSpace(params.Family)
	if family == "" {
		return nil, errors.New("family cannot be empty")
	}

	if params.Version <= 0 {
		params.Version = 1
	}

	if err := models.ValidateCategories(params.Categories); err != nil {
		return nil, fmt.Errorf("invalid categories: %w", err)
	}

	isActive := true
	if params.IsActive != nil {
		isActive = *params.IsActive
	}

	cfg := &models.ScoringConfig{
		Family:     family,
		Version:    params.Version,
		Categories: params.Categories,
		IsActive:   isActive,
	}

	return s.repo.Create(ctx, cfg)
}

func (s *scoringConfigService) GetByID(ctx context.Context, id uuid.UUID) (*models.ScoringConfig, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid scoring config id")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *scoringConfigService) GetActiveByFamily(ctx context.Context, family string) (*models.ScoringConfig, error) {
	trimmed := strings.TrimSpace(family)
	if trimmed == "" {
		return nil, errors.New("family cannot be empty")
	}
	return s.repo.GetActiveByFamily(ctx, trimmed)
}

func (s *scoringConfigService) List(ctx context.Context, params models.ListScoringConfigsParams) ([]*models.ScoringConfig, error) {
	if params.Family != nil {
		trimmed := strings.TrimSpace(*params.Family)
		params.Family = &trimmed
	}
	return s.repo.List(ctx, params)
}

func (s *scoringConfigService) Update(ctx context.Context, id uuid.UUID, params models.UpdateScoringConfigParams) (*models.ScoringConfig, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid scoring config id")
	}

	if params.Categories != nil {
		if err := models.ValidateCategories(params.Categories); err != nil {
			return nil, fmt.Errorf("invalid categories: %w", err)
		}

		existing, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if existing.IsActive {
			return nil, errors.New("cannot update categories of an active configuration; create a new version instead")
		}
	}

	return s.repo.Update(ctx, id, params)
}
