-- Fix P0: deletion_requests FK blocks DeleteUser CASCADE
-- The original FK had no ON DELETE clause, causing DeleteUser to fail
-- when a deletion_request row references the user being deleted.
ALTER TABLE deletion_requests DROP CONSTRAINT IF EXISTS deletion_requests_user_id_fkey;
ALTER TABLE deletion_requests ADD CONSTRAINT deletion_requests_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Fix: trip_collaborators.invited_by FK blocks DeleteUser CASCADE
-- When a user who invited collaborators is deleted, the invited_by
-- reference blocks the deletion.
ALTER TABLE trip_collaborators DROP CONSTRAINT IF EXISTS trip_collaborators_invited_by_fkey;
ALTER TABLE trip_collaborators ADD CONSTRAINT trip_collaborators_invited_by_fkey
    FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE CASCADE;
