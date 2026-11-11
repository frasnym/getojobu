-- +goose Up
ALTER TABLE jobs ALTER COLUMN description TYPE TEXT;
ALTER TABLE jobs ALTER COLUMN failure_message TYPE TEXT;

-- +goose Down
ALTER TABLE jobs ALTER COLUMN description TYPE VARCHAR(4000);
ALTER TABLE jobs ALTER COLUMN failure_message TYPE VARCHAR(4000);
