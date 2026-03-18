-- Migration 009: Rename agent_id_low/agent_id_high to id_low/id_high
-- DM conversations use generic participant IDs (not agent-specific).

-- 1. Drop the old constraint and unique index
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS chk_conversations_direct_ids;
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS uq_conversations_direct_pair;

-- 2. Rename columns
ALTER TABLE conversations RENAME COLUMN agent_id_low  TO id_low;
ALTER TABLE conversations RENAME COLUMN agent_id_high TO id_high;

-- 3. Re-create the unique constraint on the renamed columns
ALTER TABLE conversations
  ADD CONSTRAINT uq_conversations_direct_pair UNIQUE (id_low, id_high);

-- 4. Re-create the CHECK constraint with new column names
ALTER TABLE conversations
  ADD CONSTRAINT chk_conversations_direct_ids CHECK (
    (type = 'group' AND id_low IS NULL AND id_high IS NULL)
    OR (type = 'direct' AND id_low IS NOT NULL AND id_high IS NOT NULL AND id_low < id_high)
  );
