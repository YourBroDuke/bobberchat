-- Migration 013: Add last_message_id and last_message_at to conversations
-- Tracks the most recent message in each conversation for efficient sorting
-- and preview display without joining messages.

ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS last_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMPTZ;

-- Back-fill from existing messages
UPDATE conversations c
SET last_message_id = sub.id,
    last_message_at = sub.ts
FROM (
  SELECT DISTINCT ON (conversation_id)
    conversation_id,
    id,
    "timestamp" AS ts
  FROM messages
  ORDER BY conversation_id, "timestamp" DESC
) sub
WHERE c.id = sub.conversation_id;

CREATE INDEX IF NOT EXISTS idx_conversations_last_message_at
  ON conversations (last_message_at DESC NULLS LAST);
