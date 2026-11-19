-- +goose Up
ALTER TABLE jobs ADD COLUMN owner_id UUID REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX idx_jobs_owner_id ON jobs (owner_id) WHERE owner_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_owner_id;
ALTER TABLE jobs DROP COLUMN IF EXISTS owner_id;
