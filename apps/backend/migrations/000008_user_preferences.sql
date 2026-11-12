-- +goose Up
CREATE TABLE user_preferences (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     TEXT NOT NULL,             -- internal user id (not Clerk sub)
    keywords    TEXT[] NOT NULL,
    locations   TEXT[] NOT NULL,
    boards      TEXT[] NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One logical search-set per owner: same keyword/location set = one row.
CREATE UNIQUE INDEX idx_user_preferences_unique
    ON user_preferences(user_id, keywords, locations)
    WHERE is_active = TRUE;

CREATE INDEX idx_user_preferences_user_active
    ON user_preferences(user_id) WHERE is_active = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_user_preferences_user_active;
DROP INDEX IF EXISTS idx_user_preferences_unique;
DROP TABLE IF EXISTS user_preferences;
