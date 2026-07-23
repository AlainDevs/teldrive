-- +goose Up
ALTER TABLE file_shares
ADD COLUMN IF NOT EXISTS allow_upload BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE file_shares DROP COLUMN IF EXISTS allow_upload;
