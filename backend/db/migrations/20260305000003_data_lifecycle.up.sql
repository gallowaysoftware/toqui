-- Trip archival and data lifecycle support

-- Track when a trip was completed and when data should be purged
ALTER TABLE trips ADD COLUMN completed_at TIMESTAMPTZ;
ALTER TABLE trips ADD COLUMN archive_after TIMESTAMPTZ; -- NULL = no auto-archive
ALTER TABLE trips ADD COLUMN archived_at TIMESTAMPTZ;   -- NULL = not yet archived

-- User deletion requests (GDPR requires completion within 30 days)
CREATE TABLE deletion_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'pending' -- pending, processing, completed, failed
);
CREATE INDEX idx_deletion_requests_status ON deletion_requests(status) WHERE status != 'completed';

-- Data export requests (GDPR Article 20)
CREATE TABLE export_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    download_url TEXT,     -- signed URL, expires after 24h
    expires_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'pending'
);
CREATE INDEX idx_export_requests_user ON export_requests(user_id);
