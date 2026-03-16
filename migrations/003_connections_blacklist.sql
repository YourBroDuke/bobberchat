DO $$ BEGIN
  CREATE TYPE connection_request_status AS ENUM ('PENDING', 'ACCEPTED', 'REJECTED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS connection_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  from_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  to_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status connection_request_status NOT NULL DEFAULT 'PENDING',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, from_user_id, to_user_id)
);

CREATE TABLE IF NOT EXISTS blacklist_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, user_id, blocked_user_id)
);

CREATE INDEX IF NOT EXISTS idx_connection_requests_to ON connection_requests (tenant_id, to_user_id, status);
CREATE INDEX IF NOT EXISTS idx_connection_requests_from ON connection_requests (tenant_id, from_user_id, status);
CREATE INDEX IF NOT EXISTS idx_blacklist_user ON blacklist_entries (tenant_id, user_id);
