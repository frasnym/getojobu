-- +goose Up
ALTER TABLE user_jobs
    ADD COLUMN latest_evaluation_id UUID REFERENCES matching_evaluations(id) ON DELETE SET NULL;

CREATE INDEX idx_user_jobs_latest_eval ON user_jobs(latest_evaluation_id);
CREATE INDEX idx_matching_evaluations_user_job_id ON matching_evaluations(user_job_id);

UPDATE user_jobs uj
SET latest_evaluation_id = sub.id
FROM (
    SELECT DISTINCT ON (user_job_id) id, user_job_id
    FROM matching_evaluations
    ORDER BY user_job_id, created_at DESC
) sub
WHERE uj.id = sub.user_job_id;

-- +goose Down
DROP INDEX IF EXISTS idx_matching_evaluations_user_job_id;
DROP INDEX IF EXISTS idx_user_jobs_latest_eval;

ALTER TABLE user_jobs
    DROP COLUMN IF EXISTS latest_evaluation_id;
