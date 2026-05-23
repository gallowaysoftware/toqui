CREATE TABLE trip_collaborators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,  -- NULL until invite accepted
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'editor' CHECK (role IN ('owner', 'editor', 'viewer')),
    invite_token TEXT UNIQUE,
    invited_by UUID NOT NULL REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    UNIQUE(trip_id, email)
);
CREATE INDEX idx_trip_collaborators_trip ON trip_collaborators(trip_id);
CREATE INDEX idx_trip_collaborators_user ON trip_collaborators(user_id);
CREATE INDEX idx_trip_collaborators_email ON trip_collaborators(email);
CREATE INDEX idx_trip_collaborators_invite_token ON trip_collaborators(invite_token);
