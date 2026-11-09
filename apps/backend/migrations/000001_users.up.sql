-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    clerk_user_id VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(128),
    full_name VARCHAR(128),

    role VARCHAR(16) NOT NULL,

    onboarding_completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);