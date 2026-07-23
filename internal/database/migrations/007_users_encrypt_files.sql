-- +goose Up
ALTER TABLE users
ADD COLUMN IF NOT EXISTS encrypt_files BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS encrypt_files;
