CREATE TABLE inquiries (
    id          BIGSERIAL PRIMARY KEY,
    profile_id  UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    email       TEXT,
    phone       TEXT,
    message     TEXT NOT NULL,
    ip_hash     TEXT,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (email IS NOT NULL OR phone IS NOT NULL)
);

CREATE INDEX inquiries_profile_id_idx ON inquiries(profile_id);
CREATE INDEX inquiries_created_at_idx ON inquiries(created_at);
CREATE INDEX inquiries_profile_unread_idx ON inquiries(profile_id, read_at, created_at DESC);
