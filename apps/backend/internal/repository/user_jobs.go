package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// UserJobs is the interface for per-user job ownership data access.
type UserJobs interface {
	// Create inserts an ownership row linking a user to a job. If the pair
	// already exists (unique constraint on user_id, job_id), nothing happens
	// and created is false.
	Create(ctx context.Context, userID string, jobID int64) (created bool, err error)
	BulkCreate(ctx context.Context, links []models.UserJobLink) (addedMap map[string]bool, err error)
	IsOwned(ctx context.Context, userID string, jobID int64) (bool, error)
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

	id := uuid.NewString()
	var insertedID string
	query := `
		INSERT INTO user_jobs (id, user_id, job_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, job_id) DO NOTHING
		RETURNING id
	`
	err = tx.QueryRow(ctx, query, id, userID, jobID).Scan(&insertedID)
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
			id := uuid.NewString()
			offset := i * 3
			valuesPlaceholders = append(valuesPlaceholders, fmt.Sprintf(
				"($%d, $%d, $%d)",
				offset+1, offset+2, offset+3,
			))
			args = append(args, id, link.UserID, link.JobID)
		}

		query := fmt.Sprintf(`
			INSERT INTO user_jobs (id, user_id, job_id)
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
