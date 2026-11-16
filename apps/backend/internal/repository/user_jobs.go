package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// UserJobs is the interface for per-user job ownership data access.
type UserJobs interface {
	Create(ctx context.Context, userID string, jobID int64) (created bool, err error)
	BulkCreate(ctx context.Context, links []models.UserJobLink) (addedMap map[string]bool, err error)
	IsOwned(ctx context.Context, userID string, jobID int64) (bool, error)
	GetByUserAndJob(ctx context.Context, userID string, jobID int64) (*models.UserJob, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.UserJob, error)
	GetUsersByJobID(ctx context.Context, jobID int64) ([]string, error)
	GetByUser(ctx context.Context, userID string, limit, offset int) ([]*models.Job, int, error)
	SetHideStatus(ctx context.Context, userID, userJobID string, isHidden bool) error
}

type userJobsRepository struct {
	*database.PG
}

// NewUserJobsRepository creates a UserJobs backed by the given PG transaction manager.
func NewUserJobsRepository(pg *database.PG) UserJobs {
	return &userJobsRepository{
		PG: pg,
	}
}

// Create inserts an ownership row linking userID to jobID. Returns created=true
// when a new row was inserted, created=false when the pair already exists.
func (r *userJobsRepository) Create(ctx context.Context, userID string, jobID int64) (created bool, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return false, fmt.Errorf("create user_jobs start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	var insertedID string
	query := `
		INSERT INTO user_jobs (user_id, job_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, job_id) DO NOTHING
		RETURNING id
	`
	err = tx.QueryRow(ctx, query, userID, jobID).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Ownership conflict: the row already exists. Not an error.
			return false, nil
		}
		return false, fmt.Errorf("create user_jobs ownership: %w", err)
	}
	return true, nil
}

// BulkCreate inserts ownership rows linking user IDs to job IDs in batches using ChunkBy.
// Returns a map keyed by "userID:jobID" indicating which pairs were newly linked (true)
// vs already linked (false).
func (r *userJobsRepository) BulkCreate(ctx context.Context, links []models.UserJobLink) (addedMap map[string]bool, err error) {
	if len(links) == 0 {
		return map[string]bool{}, nil
	}

	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("bulk create user_jobs start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	addedMap = make(map[string]bool, len(links))
	deduped := make([]models.UserJobLink, 0, len(links))
	seen := make(map[string]bool, len(links))

	for _, link := range links {
		key := fmt.Sprintf("%s:%d", link.UserID, link.JobID)
		addedMap[key] = false
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, link)
		}
	}

	chunks := database.ChunkBy(deduped, 50)
	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}

		var (
			valuesPlaceholders []string
			args               []any
		)

		for i, link := range chunk {
			// Each link contributes exactly two args (user_id, job_id), so
			// placeholder numbering must increment by 2 per row. Using i * 3
			// left gaps ($3, $6, ...) unbound and caused SQLSTATE 42P18.
			offset := i * 2
			valuesPlaceholders = append(valuesPlaceholders, fmt.Sprintf(
				"($%d, $%d)",
				offset+1, offset+2,
			))
			args = append(args, link.UserID, link.JobID)
		}

		query := fmt.Sprintf(`
			INSERT INTO user_jobs (user_id, job_id)
			VALUES %s
			ON CONFLICT (user_id, job_id) DO NOTHING
			RETURNING user_id, job_id
		`, strings.Join(valuesPlaceholders, ", "))

		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("bulk insert user_jobs: %w", err)
		}

		for rows.Next() {
			var (
				uid string
				jid int64
			)
			if err := rows.Scan(&uid, &jid); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan inserted user_jobs: %w", err)
			}
			addedMap[fmt.Sprintf("%s:%d", uid, jid)] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate inserted user_jobs: %w", err)
		}
	}

	return addedMap, nil
}

// IsOwned reports whether the given user owns the given job.
func (r *userJobsRepository) IsOwned(ctx context.Context, userID string, jobID int64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM user_jobs WHERE user_id = $1 AND job_id = $2
		)
	`
	row := r.QueryRow(ctx, query, userID, jobID)
	var owned bool
	if err := row.Scan(&owned); err != nil {
		return false, fmt.Errorf("check job ownership: %w", err)
	}
	return owned, nil
}

// GetByUserAndJob fetches the user_job row for a given user and job.
func (r *userJobsRepository) GetByUserAndJob(ctx context.Context, userID string, jobID int64) (*models.UserJob, error) {
	query := `
		SELECT id, user_id, job_id, created_at, updated_at
		FROM user_jobs
		WHERE user_id = $1 AND job_id = $2
	`
	uj := &models.UserJob{}
	err := r.DB().QueryRow(ctx, query, userID, jobID).Scan(
		&uj.ID,
		&uj.UserID,
		&uj.JobID,
		&uj.CreatedAt,
		&uj.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return uj, nil
}

// GetByID fetches the user_job row by id.
func (r *userJobsRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.UserJob, error) {
	query := `
		SELECT id, user_id, job_id, created_at, updated_at
		FROM user_jobs
		WHERE id = $1
	`
	uj := &models.UserJob{}
	err := r.DB().QueryRow(ctx, query, id).Scan(
		&uj.ID,
		&uj.UserID,
		&uj.JobID,
		&uj.CreatedAt,
		&uj.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get user_job by id: %w", err)
	}
	return uj, nil
}

// GetUsersByJobID returns all user IDs linked to the given job.
func (r *userJobsRepository) GetUsersByJobID(ctx context.Context, jobID int64) ([]string, error) {
	query := `
		SELECT user_id
		FROM user_jobs
		WHERE job_id = $1
	`
	rows, err := r.DB().Query(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("query users by job id: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, rows.Err()
}

// GetByUser returns jobs owned by the given user (joined with the jobs table),
// ordered by created_at descending, paginated by limit and offset. Each job carries
// isOwned=true.
func (r *userJobsRepository) GetByUser(ctx context.Context, userID string, limit, offset int) (jobs []*models.Job, total int, err error) {
	if err := r.QueryRow(ctx, `SELECT COUNT(*) FROM user_jobs WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user jobs: %w", err)
	}

	query := `
		SELECT j.id, j.slug, j.status, j.analyze_result, j.failure_message,
			j.source, j.external_id, j.source_url, j.apply_url,
			j.title, j.company, j.location, j.description, j.posted_date,
			j.is_remote, j.visa_relocation,
			j.created_at, j.updated_at, j.updated_by_type, j.updated_by_id,
			true AS is_owned,
			uj.id AS user_job_id,
			uj.is_hidden AS is_hidden
		FROM jobs j
		JOIN user_jobs uj ON j.id = uj.job_id AND uj.user_id = $1
		ORDER BY j.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get jobs by user: %w", err)
	}
	defer rows.Close()
	jobs, err = scanJobWithOwnership(rows)
	if err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

// SetHideStatus toggles the is_hidden flag on the user↔job link identified by
// userJobID. Returns ErrNotFound when the user does not own that link.
func (r *userJobsRepository) SetHideStatus(ctx context.Context, userID, userJobID string, isHidden bool) error {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return fmt.Errorf("set hide status start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	res, err := tx.Exec(ctx, `
		UPDATE user_jobs
		SET is_hidden = $3
		WHERE id = $1 AND user_id = $2
	`, userJobID, userID, isHidden)
	if err != nil {
		return fmt.Errorf("set user_jobs hide status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return constants.ErrNotFound
	}
	return nil
}
