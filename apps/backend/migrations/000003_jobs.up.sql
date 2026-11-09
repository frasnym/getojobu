-- +goose Up
CREATE TABLE jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    slug VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'new',
    analyze_result JSONB,
    failure_message TEXT,

    source VARCHAR(64) NOT NULL,
    external_id VARCHAR(255),
    source_url VARCHAR(2048),
    apply_url VARCHAR(2048),

    title VARCHAR(255) NOT NULL,
    company VARCHAR(255) NOT NULL,
    location VARCHAR(255) NOT NULL,
    description TEXT,
    posted_date DATE,
    is_remote BOOLEAN,
    visa_relocation BOOLEAN,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    updated_by_id TEXT NOT NULL DEFAULT 'system'
);
