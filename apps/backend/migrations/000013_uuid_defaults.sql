-- +goose Up
ALTER TABLE users ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE user_jobs ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE user_experiences ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE user_projects ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE matching_evaluations ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE user_educations ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- +goose Down
ALTER TABLE users ALTER COLUMN id DROP DEFAULT;
ALTER TABLE user_jobs ALTER COLUMN id DROP DEFAULT;
ALTER TABLE user_experiences ALTER COLUMN id DROP DEFAULT;
ALTER TABLE user_projects ALTER COLUMN id DROP DEFAULT;
ALTER TABLE matching_evaluations ALTER COLUMN id DROP DEFAULT;
ALTER TABLE user_educations ALTER COLUMN id DROP DEFAULT;
