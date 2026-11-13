-- +goose Up
ALTER TABLE matching_evaluations
    RENAME COLUMN status TO evaluation_status;

ALTER TABLE matching_evaluations
    DROP COLUMN prompt_version,
    DROP COLUMN provider,
    DROP COLUMN model;

ALTER TABLE matching_evaluations
    ADD COLUMN report_data JSONB,
    ADD COLUMN report_status VARCHAR(32),
    ADD COLUMN report_failure_message TEXT;

-- +goose Down
ALTER TABLE matching_evaluations
    DROP COLUMN IF EXISTS report_failure_message,
    DROP COLUMN IF EXISTS report_status,
    DROP COLUMN IF EXISTS report_data;

ALTER TABLE matching_evaluations
    ADD COLUMN prompt_version VARCHAR(64) NOT NULL,
    ADD COLUMN provider VARCHAR(64),
    ADD COLUMN model VARCHAR(64);

ALTER TABLE matching_evaluations
    RENAME COLUMN evaluation_status TO status;
