ALTER TABLE connection_requests DROP CONSTRAINT IF EXISTS connection_requests_from_user_id_fkey;
ALTER TABLE connection_requests DROP CONSTRAINT IF EXISTS connection_requests_to_user_id_fkey;
ALTER TABLE connection_requests DROP CONSTRAINT IF EXISTS connection_requests_from_user_id_to_user_id_key;

DROP INDEX IF EXISTS idx_connection_requests_to;
DROP INDEX IF EXISTS idx_connection_requests_from;

ALTER TABLE connection_requests RENAME COLUMN from_user_id TO from_agent_id;
ALTER TABLE connection_requests RENAME COLUMN to_user_id TO to_agent_id;

ALTER TABLE connection_requests
  ADD CONSTRAINT connection_requests_from_agent_id_fkey
    FOREIGN KEY (from_agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE;

ALTER TABLE connection_requests
  ADD CONSTRAINT connection_requests_to_agent_id_fkey
    FOREIGN KEY (to_agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE;

ALTER TABLE connection_requests
  ADD CONSTRAINT connection_requests_from_agent_id_to_agent_id_key
    UNIQUE (from_agent_id, to_agent_id);

CREATE INDEX IF NOT EXISTS idx_connection_requests_to ON connection_requests (to_agent_id, status);
CREATE INDEX IF NOT EXISTS idx_connection_requests_from ON connection_requests (from_agent_id, status);
