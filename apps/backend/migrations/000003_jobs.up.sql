-- +goose Up
CREATE TABLE jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    slug TEXT NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'new',
    analyze_result JSONB,
    failure_message TEXT,

    source TEXT NOT NULL,
    external_id TEXT,
    source_url TEXT,
    apply_url TEXT,

    title TEXT NOT NULL,
    company TEXT NOT NULL,
    location TEXT NOT NULL,
    description TEXT,
    posted_date DATE,
    is_remote BOOLEAN,
    visa_relocation BOOLEAN,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    updated_by_id TEXT NOT NULL DEFAULT 'system'
);
