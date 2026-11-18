-- +goose Up
ALTER TABLE matching_evaluations
    ADD COLUMN cover_letter_status VARCHAR(32),
    ADD COLUMN cover_letter_content JSONB,
    ADD COLUMN cover_letter_failure_message TEXT;

-- +goose Down
ALTER TABLE matching_evaluations
    DROP COLUMN IF EXISTS cover_letter_failure_message,
    DROP COLUMN IF EXISTS cover_letter_content,
    DROP COLUMN IF EXISTS cover_letter_status;
