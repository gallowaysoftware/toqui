DROP TABLE IF EXISTS export_requests;
DROP TABLE IF EXISTS deletion_requests;
ALTER TABLE trips DROP COLUMN IF EXISTS archived_at;
ALTER TABLE trips DROP COLUMN IF EXISTS archive_after;
ALTER TABLE trips DROP COLUMN IF EXISTS completed_at;
