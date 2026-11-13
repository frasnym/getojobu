-- +goose Up
CREATE TABLE IF NOT EXISTS matching_evaluations (
    id UUID PRIMARY KEY,
    user_job_id UUID NOT NULL REFERENCES user_jobs(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'new',
    candidate_profile_data JSONB,
    job_profile_data JSONB,
    scoring_config_id UUID NOT NULL REFERENCES scoring_configs(id) ON DELETE RESTRICT,
    evaluation_data JSONB,
    overall_score NUMERIC(5,2),
    eligibility_status VARCHAR(32),
    recommendation_label VARCHAR(32),
    failure_message TEXT,
    prompt_version VARCHAR(64) NOT NULL DEFAULT 'evaluate-match-v1',
    provider VARCHAR(64),
    model VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS matching_evaluations;
