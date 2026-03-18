-- Remove version, connected_at, and last_heartbeat from agents table
ALTER TABLE agents DROP COLUMN IF EXISTS version;
ALTER TABLE agents DROP COLUMN IF EXISTS connected_at;
ALTER TABLE agents DROP COLUMN IF EXISTS last_heartbeat;
