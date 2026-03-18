-- Migration 008: Add conversations concept
-- Introduces a unified conversation entity (direct/group) that all messages
-- belong to, with a participant relation table supporting muted/lastRead state.

-- 1. Create conversation_type enum
DO $$ BEGIN
  CREATE TYPE conversation_type AS ENUM ('direct', 'group');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 2. Create conversations table
CREATE TABLE IF NOT EXISTS conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type conversation_type NOT NULL,
  -- For direct conversations: canonical pair (low < high) of participant IDs.
  -- NULL for group conversations.
  agent_id_low UUID,
  agent_id_high UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Enforce exactly one DM per pair
  CONSTRAINT uq_conversations_direct_pair UNIQUE (agent_id_low, agent_id_high),
  -- Ensure low < high ordering for direct conversations
  CONSTRAINT chk_conversations_direct_ids CHECK (
    (type = 'group' AND agent_id_low IS NULL AND agent_id_high IS NULL)
    OR (type = 'direct' AND agent_id_low IS NOT NULL AND agent_id_high IS NOT NULL AND agent_id_low < agent_id_high)
  )
);

-- 3. Create conversation_participants table (replaces chat_group_members)
CREATE TABLE IF NOT EXISTS conversation_participants (
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  participant_id UUID NOT NULL,
  participant_kind participant_type NOT NULL,
  muted BOOLEAN NOT NULL DEFAULT FALSE,
  last_read_message_id UUID,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (conversation_id, participant_id, participant_kind)
);

CREATE INDEX IF NOT EXISTS idx_conv_participants_participant
  ON conversation_participants (participant_id, participant_kind);

-- 4. Add conversation_id to chat_groups
ALTER TABLE chat_groups
  ADD COLUMN IF NOT EXISTS conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_chat_groups_conversation
  ON chat_groups (conversation_id);

-- 5. Migrate messages: add conversation_id, drop to_id
ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_messages_conversation_time
  ON messages (conversation_id, "timestamp" DESC);

-- Drop the old to_id-based index (no longer needed)
DROP INDEX IF EXISTS idx_messages_to_tag_time;

ALTER TABLE messages DROP COLUMN IF EXISTS to_id;

-- 6. Migrate chat_group_members data into conversation_participants
-- For each existing chat_group, create a conversation row and link it.
DO $$
DECLARE
  g RECORD;
  conv_id UUID;
BEGIN
  FOR g IN SELECT id FROM chat_groups WHERE conversation_id IS NULL LOOP
    INSERT INTO conversations (id, type, agent_id_low, agent_id_high)
    VALUES (gen_random_uuid(), 'group', NULL, NULL)
    RETURNING id INTO conv_id;

    UPDATE chat_groups SET conversation_id = conv_id WHERE id = g.id;

    INSERT INTO conversation_participants (conversation_id, participant_id, participant_kind, joined_at)
    SELECT conv_id, participant_id, participant_kind, joined_at
    FROM chat_group_members
    WHERE group_id = g.id;
  END LOOP;
END $$;

-- 7. Drop old chat_group_members table
DROP TABLE IF EXISTS chat_group_members;
