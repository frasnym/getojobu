-- +goose Up
ALTER TABLE user_preferences
  ADD COLUMN auto_hide_low_match_evaluation BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE user_preferences
  DROP COLUMN IF EXISTS auto_hide_low_match_evaluation;
