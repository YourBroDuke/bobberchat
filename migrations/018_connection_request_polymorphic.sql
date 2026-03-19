ALTER TABLE connection_requests DROP CONSTRAINT IF EXISTS connection_requests_from_agent_id_fkey;
ALTER TABLE connection_requests DROP CONSTRAINT IF EXISTS connection_requests_to_agent_id_fkey;
ALTER TABLE connection_requests DROP CONSTRAINT IF EXISTS connection_requests_from_agent_id_to_agent_id_key;

DROP INDEX IF EXISTS idx_connection_requests_to;
DROP INDEX IF EXISTS idx_connection_requests_from;

DO $$ BEGIN
  CREATE TYPE entity_type AS ENUM ('agent', 'group');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE connection_requests ADD COLUMN sender_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE connection_requests ADD COLUMN from_kind entity_type NOT NULL DEFAULT 'agent';
ALTER TABLE connection_requests ADD COLUMN to_kind entity_type NOT NULL DEFAULT 'agent';

ALTER TABLE connection_requests RENAME COLUMN from_agent_id TO from_id;
ALTER TABLE connection_requests RENAME COLUMN to_agent_id TO to_id;

UPDATE connection_requests SET sender_id = from_id WHERE sender_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE connection_requests ALTER COLUMN sender_id DROP DEFAULT;

ALTER TABLE connection_requests
  ADD CONSTRAINT connection_requests_sender_id_fkey
    FOREIGN KEY (sender_id) REFERENCES agents(agent_id) ON DELETE CASCADE;

ALTER TABLE connection_requests
  ADD CONSTRAINT connection_requests_from_to_unique
    UNIQUE (from_id, from_kind, to_id, to_kind);

CREATE INDEX idx_connection_requests_to ON connection_requests (to_id, to_kind, status);
CREATE INDEX idx_connection_requests_from ON connection_requests (from_id, from_kind, status);
CREATE INDEX idx_connection_requests_sender ON connection_requests (sender_id);
