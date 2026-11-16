-- +goose Up
ALTER TABLE user_jobs
  ADD COLUMN is_hidden BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE user_jobs
  DROP COLUMN IF EXISTS is_hidden;
