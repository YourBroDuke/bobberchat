-- Migration 021: Default last_message_at to created_at for conversations
-- Ensures newly created conversations always have a non-null last_message_at.

-- Set column default so any future INSERTs that omit last_message_at get now().
ALTER TABLE conversations
    ALTER COLUMN last_message_at SET DEFAULT now();

-- Backfill existing rows where last_message_at is still NULL.
UPDATE conversations
SET last_message_at = created_at
WHERE last_message_at IS NULL;
