-- +goose Up
ALTER TABLE matching_evaluations ALTER COLUMN evaluation_status SET DEFAULT 'queued';
UPDATE jobs SET status = 'queued_retry' WHERE status = 'new-retry';
UPDATE matching_evaluations SET evaluation_status = 'queued' WHERE evaluation_status = 'new';
UPDATE matching_evaluations SET evaluation_status = 'queued_retry' WHERE evaluation_status = 'new-retry';
UPDATE matching_evaluations SET report_status = 'queued_retry' WHERE report_status = 'pending-retry';
UPDATE matching_evaluations SET resume_status = 'queued_retry' WHERE resume_status = 'pending-retry';

-- +goose Down
UPDATE matching_evaluations SET resume_status = 'pending-retry' WHERE resume_status = 'queued_retry';
UPDATE matching_evaluations SET report_status = 'pending-retry' WHERE report_status = 'queued_retry';
UPDATE matching_evaluations SET evaluation_status = 'new-retry' WHERE evaluation_status = 'queued_retry';
UPDATE matching_evaluations SET evaluation_status = 'new' WHERE evaluation_status = 'queued';
UPDATE jobs SET status = 'new-retry' WHERE status = 'queued_retry';
ALTER TABLE matching_evaluations ALTER COLUMN evaluation_status SET DEFAULT 'new';
