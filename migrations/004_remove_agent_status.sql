-- Remove agent_status concept from agents table
ALTER TABLE agents DROP COLUMN IF EXISTS status;
DROP INDEX IF EXISTS idx_agents_status;
DROP TYPE IF EXISTS agent_status;
