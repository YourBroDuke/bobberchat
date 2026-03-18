-- Migration 007: Remove capabilities from agents table
-- The capability concept is being removed from the agent entity.

DROP INDEX IF EXISTS idx_agents_capabilities_gin;
ALTER TABLE agents DROP COLUMN IF EXISTS capabilities;
