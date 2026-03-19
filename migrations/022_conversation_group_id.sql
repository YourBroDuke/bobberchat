-- Migration 022: Add group_id to conversations for accelerated group lookups
-- Adds a reverse pointer from conversations to chat_groups, enabling direct
-- conversation-by-group queries without joining through conversation_participants.

-- 1. Add the column
ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS group_id UUID REFERENCES chat_groups(id) ON DELETE SET NULL;

-- 2. Backfill from existing chat_groups that already have a conversation_id
UPDATE conversations c
SET group_id = cg.id
FROM chat_groups cg
WHERE cg.conversation_id = c.id
  AND c.group_id IS NULL;

-- 3. Index for fast lookups by group_id
CREATE INDEX IF NOT EXISTS idx_conversations_group_id
  ON conversations (group_id);
