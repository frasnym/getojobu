package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// ScoringConfigs is the interface for scoring configuration data access.
type ScoringConfigs interface {
	Create(ctx context.Context, cfg *models.ScoringConfig) (*models.ScoringConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ScoringConfig, error)
	GetActiveByFamily(ctx context.Context, family string) (*models.ScoringConfig, error)
	List(ctx context.Context, params models.ListScoringConfigsParams) ([]*models.ScoringConfig, error)
	Update(ctx context.Context, id uuid.UUID, params models.UpdateScoringConfigParams) (*models.ScoringConfig, error)
}

type scoringConfigsRepository struct {
	*database.PG
}

// NewScoringConfigsRepository creates a new ScoringConfigs repository backed by PG.
func NewScoringConfigsRepository(pg *database.PG) ScoringConfigs {
	return &scoringConfigsRepository{
		PG: pg,
	}
}

func (r *scoringConfigsRepository) Create(ctx context.Context, cfg *models.ScoringConfig) (res *models.ScoringConfig, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("create scoring config start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	// If the new configuration is active, deactivate existing active version for this family.
	if cfg.IsActive {
		deactivateQuery := `
			UPDATE scoring_configs
			SET is_active = false, updated_at = NOW()
			WHERE family = $1 AND is_active = true
		`
		if _, err = tx.Exec(ctx, deactivateQuery, cfg.Family); err != nil {
			return nil, fmt.Errorf("deactivate existing active scoring config: %w", err)
		}
	}

	categoriesJSON, err := json.Marshal(cfg.Categories)
	if err != nil {
		return nil, fmt.Errorf("marshal categories: %w", err)
	}

	insertQuery := `
		INSERT INTO scoring_configs (family, version, categories, is_active)
		VALUES ($1, $2, $3, $4)
		RETURNING id, family, version, categories, is_active, created_at, updated_at
	`

	var (
		rawCategories []byte
		created       models.ScoringConfig
	)

	err = tx.QueryRow(ctx, insertQuery, cfg.Family, cfg.Version, categoriesJSON, cfg.IsActive).Scan(
		&created.ID,
		&created.Family,
		&created.Version,
		&rawCategories,
		&created.IsActive,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, constants.ErrDuplicateScoringConfigVersion
		}
		return nil, fmt.Errorf("insert scoring config: %w", err)
	}

	if err = json.Unmarshal(rawCategories, &created.Categories); err != nil {
		return nil, fmt.Errorf("unmarshal created categories: %w", err)
	}

	return &created, nil
}

func (r *scoringConfigsRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ScoringConfig, error) {
	query := `
		SELECT id, family, version, categories, is_active, created_at, updated_at
		FROM scoring_configs
		WHERE id = $1
		LIMIT 1
	`
	var (
		rawCategories []byte
		cfg           models.ScoringConfig
	)

	err := r.DB().QueryRow(ctx, query, id).Scan(
		&cfg.ID,
		&cfg.Family,
		&cfg.Version,
		&rawCategories,
		&cfg.IsActive,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrScoringConfigNotFound
		}
		return nil, fmt.Errorf("get scoring config by id: %w", err)
	}

	if err = json.Unmarshal(rawCategories, &cfg.Categories); err != nil {
		return nil, fmt.Errorf("unmarshal categories: %w", err)
	}

	return &cfg, nil
}

func (r *scoringConfigsRepository) GetActiveByFamily(ctx context.Context, family string) (*models.ScoringConfig, error) {
	query := `
		SELECT id, family, version, categories, is_active, created_at, updated_at
		FROM scoring_configs
		WHERE family = $1 AND is_active = true
		LIMIT 1
	`
	var (
		rawCategories []byte
		cfg           models.ScoringConfig
	)

	err := r.DB().QueryRow(ctx, query, family).Scan(
		&cfg.ID,
		&cfg.Family,
		&cfg.Version,
		&rawCategories,
		&cfg.IsActive,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrScoringConfigNotFound
		}
		return nil, fmt.Errorf("get active scoring config by family: %w", err)
	}

	if err = json.Unmarshal(rawCategories, &cfg.Categories); err != nil {
		return nil, fmt.Errorf("unmarshal categories: %w", err)
	}

	return &cfg, nil
}

func (r *scoringConfigsRepository) List(ctx context.Context, params models.ListScoringConfigsParams) ([]*models.ScoringConfig, error) {
	var (
		conditions []string
		args       []any
		idx        = 1
	)

	if params.Family != nil && *params.Family != "" {
		conditions = append(conditions, fmt.Sprintf("family = $%d", idx))
		args = append(args, *params.Family)
		idx++
	}

	if params.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *params.IsActive)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, family, version, categories, is_active, created_at, updated_at
		FROM scoring_configs
		%s
		ORDER BY family ASC, version DESC
	`, whereClause)

	rows, err := r.DB().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scoring configs: %w", err)
	}
	defer rows.Close()

	var result []*models.ScoringConfig
	for rows.Next() {
		var (
			rawCategories []byte
			cfg           models.ScoringConfig
		)
		if err := rows.Scan(
			&cfg.ID,
			&cfg.Family,
			&cfg.Version,
			&rawCategories,
			&cfg.IsActive,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scoring config: %w", err)
		}
		if err := json.Unmarshal(rawCategories, &cfg.Categories); err != nil {
			return nil, fmt.Errorf("unmarshal categories: %w", err)
		}
		result = append(result, &cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func (r *scoringConfigsRepository) Update(ctx context.Context, id uuid.UUID, params models.UpdateScoringConfigParams) (res *models.ScoringConfig, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("update scoring config start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	// Lock and fetch existing row inside transaction to check existence and retrieve family
	var (
		existingFamily   string
		existingIsActive bool
	)
	lockQuery := `
		SELECT family, is_active
		FROM scoring_configs
		WHERE id = $1
		FOR UPDATE
	`
	if err = tx.QueryRow(ctx, lockQuery, id).Scan(&existingFamily, &existingIsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrScoringConfigNotFound
		}
		return nil, fmt.Errorf("lock scoring config: %w", err)
	}

	// If activating, deactivate other versions in the same family
	if params.IsActive != nil && *params.IsActive {
		deactivateQuery := `
			UPDATE scoring_configs
			SET is_active = false, updated_at = NOW()
			WHERE family = $1 AND is_active = true AND id != $2
		`
		if _, err = tx.Exec(ctx, deactivateQuery, existingFamily, id); err != nil {
			return nil, fmt.Errorf("deactivate other configs in family: %w", err)
		}
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []any{id}
	idx := 2

	if params.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *params.IsActive)
		idx++
	}

	if params.Categories != nil {
		catsJSON, err := json.Marshal(params.Categories)
		if err != nil {
			return nil, fmt.Errorf("marshal categories: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("categories = $%d", idx))
		args = append(args, catsJSON)
		idx++
	}

	updateQuery := fmt.Sprintf(`
		UPDATE scoring_configs
		SET %s
		WHERE id = $1
		RETURNING id, family, version, categories, is_active, created_at, updated_at
	`, strings.Join(setClauses, ", "))

	var (
		rawCategories []byte
		updated       models.ScoringConfig
	)

	err = tx.QueryRow(ctx, updateQuery, args...).Scan(
		&updated.ID,
		&updated.Family,
		&updated.Version,
		&rawCategories,
		&updated.IsActive,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update scoring config query: %w", err)
	}

	if err = json.Unmarshal(rawCategories, &updated.Categories); err != nil {
		return nil, fmt.Errorf("unmarshal updated categories: %w", err)
	}

	return &updated, nil
}
