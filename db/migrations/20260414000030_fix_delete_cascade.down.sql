-- Revert to original FK constraints (no ON DELETE CASCADE)
ALTER TABLE deletion_requests DROP CONSTRAINT IF EXISTS deletion_requests_user_id_fkey;
ALTER TABLE deletion_requests ADD CONSTRAINT deletion_requests_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE trip_collaborators DROP CONSTRAINT IF EXISTS trip_collaborators_invited_by_fkey;
ALTER TABLE trip_collaborators ADD CONSTRAINT trip_collaborators_invited_by_fkey
    FOREIGN KEY (invited_by) REFERENCES users(id);
