-- Migration 017: add participant_kind column to messages table
-- Reuses the existing participant_type enum from 001_initial_schema.sql
ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS participant_kind participant_type NOT NULL DEFAULT 'user';
