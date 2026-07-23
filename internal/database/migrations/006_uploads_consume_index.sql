-- +goose Up
CREATE INDEX IF NOT EXISTS idx_uploads_upload_id_user_id_part_no
ON uploads (upload_id, user_id, part_no);

-- +goose Down
DROP INDEX IF EXISTS idx_uploads_upload_id_user_id_part_no;
