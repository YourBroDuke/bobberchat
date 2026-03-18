-- Migration 006: Remove topics feature
-- Drops the topics table, topic_status type, topic_id column from messages,
-- and associated indexes.

-- Drop indexes first
DROP INDEX IF EXISTS idx_messages_topic_time;
DROP INDEX IF EXISTS idx_topics_group_status;
DROP INDEX IF EXISTS idx_topics_parent;

-- Drop topic_id column from messages
ALTER TABLE messages DROP COLUMN IF EXISTS topic_id;

-- Drop topics table
DROP TABLE IF EXISTS topics CASCADE;

-- Drop topic_status enum type
DROP TYPE IF EXISTS topic_status;
