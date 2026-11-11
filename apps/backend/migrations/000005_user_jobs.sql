-- +goose Up
CREATE TABLE user_jobs (
    id       UUID PRIMARY KEY,
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id   BIGINT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_jobs_user_job UNIQUE (user_id, job_id)
);

CREATE INDEX idx_user_jobs_job_id ON user_jobs(job_id);
CREATE INDEX idx_user_jobs_user_id ON user_jobs(user_id);

-- +goose Down
DROP TABLE IF EXISTS user_jobs;
