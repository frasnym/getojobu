-- +goose Up
ALTER TABLE users
    ADD COLUMN profile_data JSONB,
    ADD COLUMN facts_data JSONB;

ALTER TABLE matching_evaluations
    DROP COLUMN IF EXISTS candidate_profile_data,
    DROP COLUMN IF EXISTS job_profile_data;

-- +goose Down
ALTER TABLE matching_evaluations
    ADD COLUMN candidate_profile_data JSONB,
    ADD COLUMN job_profile_data JSONB;

ALTER TABLE users
    DROP COLUMN IF EXISTS facts_data,
    DROP COLUMN IF EXISTS profile_data;
