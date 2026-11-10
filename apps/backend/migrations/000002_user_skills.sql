-- +goose Up
CREATE TABLE user_skills (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    skill_name VARCHAR(128) NOT NULL,
    skill_key VARCHAR(64) NOT NULL,
    proficiency VARCHAR(32),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_skills_id_user UNIQUE (id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS user_skills;
