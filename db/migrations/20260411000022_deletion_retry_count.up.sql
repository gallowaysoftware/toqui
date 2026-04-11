-- Add retry_count to deletion_requests for failed GDPR deletion retry tracking
ALTER TABLE deletion_requests ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;
