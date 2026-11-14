-- +goose Up
ALTER TABLE users
    ADD COLUMN headline VARCHAR(256),
    ADD COLUMN phone TEXT,
    ADD COLUMN portfolio_url TEXT,
    ADD COLUMN linkedin_url TEXT,
    ADD COLUMN github_url TEXT;

CREATE TABLE user_educations (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    institution VARCHAR(255) NOT NULL CHECK (BTRIM(institution) <> ''),
    degree VARCHAR(255) NOT NULL CHECK (BTRIM(degree) <> ''),
    field_of_study VARCHAR(255) NOT NULL CHECK (BTRIM(field_of_study) <> ''),
    start_year INT,
    end_year INT,
    gpa VARCHAR(16),
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_educations_id_user UNIQUE (id, user_id)
);

ALTER TABLE matching_evaluations
    ADD COLUMN resume_status VARCHAR(32),
    ADD COLUMN resume_failure_message TEXT,
    ADD COLUMN resume_pdf_key TEXT,
    ADD COLUMN resume_json_key TEXT;

-- +goose Down
ALTER TABLE matching_evaluations
    DROP COLUMN IF EXISTS resume_json_key,
    DROP COLUMN IF EXISTS resume_pdf_key,
    DROP COLUMN IF EXISTS resume_failure_message,
    DROP COLUMN IF EXISTS resume_status;

DROP TABLE IF EXISTS user_educations;

ALTER TABLE users
    DROP COLUMN IF EXISTS github_url,
    DROP COLUMN IF EXISTS linkedin_url,
    DROP COLUMN IF EXISTS portfolio_url,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS headline;
