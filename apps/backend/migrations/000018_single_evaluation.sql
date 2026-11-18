-- +goose Up
-- Collapse evaluation history to the single latest evaluation per user/job.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_job_id ORDER BY created_at DESC, id DESC) AS rn
    FROM matching_evaluations
)
DELETE FROM matching_evaluations e
USING ranked r
WHERE e.id = r.id AND r.rn > 1;

ALTER TABLE matching_evaluations
    ADD CONSTRAINT uq_matching_evaluations_user_job UNIQUE (user_job_id);

ALTER TABLE user_jobs
    DROP COLUMN IF EXISTS latest_evaluation_id;

DROP INDEX IF EXISTS idx_user_jobs_latest_eval;
DROP INDEX IF EXISTS idx_matching_evaluations_user_job_id;

-- +goose Down
-- The column and indexes are restored, but the duplicate evaluation rows removed by
-- the Up migration cannot be recovered by a rollback.
ALTER TABLE matching_evaluations
    DROP CONSTRAINT IF EXISTS uq_matching_evaluations_user_job;

ALTER TABLE user_jobs
    ADD COLUMN latest_evaluation_id UUID REFERENCES matching_evaluations(id) ON DELETE SET NULL;

CREATE INDEX idx_user_jobs_latest_eval ON user_jobs(latest_evaluation_id);
CREATE INDEX idx_matching_evaluations_user_job_id ON matching_evaluations(user_job_id);
