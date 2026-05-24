-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_periodic_job_changed()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM pg_notify('periodic_job_changed', COALESCE(NEW.id::text, OLD.id::text));
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_periodic_job_changed ON periodic_jobs;

CREATE TRIGGER trg_periodic_job_changed
    AFTER INSERT OR UPDATE OR DELETE
    ON periodic_jobs
    FOR EACH ROW
    EXECUTE FUNCTION notify_periodic_job_changed();

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_periodic_job_changed ON periodic_jobs;
DROP FUNCTION IF EXISTS notify_periodic_job_changed();
-- +goose StatementEnd
