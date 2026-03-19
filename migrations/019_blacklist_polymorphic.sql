DO $$ BEGIN
  ALTER TYPE entity_type ADD VALUE 'user';
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DROP INDEX IF EXISTS idx_blacklist_user;
ALTER TABLE blacklist_entries DROP CONSTRAINT IF EXISTS blacklist_entries_user_id_blocked_user_id_key;

ALTER TABLE blacklist_entries ADD COLUMN from_id UUID;
ALTER TABLE blacklist_entries ADD COLUMN from_kind entity_type;
ALTER TABLE blacklist_entries ADD COLUMN to_id UUID;
ALTER TABLE blacklist_entries ADD COLUMN to_kind entity_type;

UPDATE blacklist_entries
SET from_id = user_id,
    from_kind = 'user',
    to_id = blocked_user_id,
    to_kind = 'user';

ALTER TABLE blacklist_entries ALTER COLUMN from_id SET NOT NULL;
ALTER TABLE blacklist_entries ALTER COLUMN from_kind SET NOT NULL;
ALTER TABLE blacklist_entries ALTER COLUMN to_id SET NOT NULL;
ALTER TABLE blacklist_entries ALTER COLUMN to_kind SET NOT NULL;

ALTER TABLE blacklist_entries DROP CONSTRAINT IF EXISTS blacklist_entries_user_id_fkey;
ALTER TABLE blacklist_entries DROP CONSTRAINT IF EXISTS blacklist_entries_blocked_user_id_fkey;

ALTER TABLE blacklist_entries DROP COLUMN user_id;
ALTER TABLE blacklist_entries DROP COLUMN blocked_user_id;

ALTER TABLE blacklist_entries
  ADD CONSTRAINT blacklist_entries_from_to_unique
    UNIQUE (from_id, from_kind, to_id, to_kind);

CREATE INDEX idx_blacklist_from ON blacklist_entries (from_id, from_kind);
CREATE INDEX idx_blacklist_to ON blacklist_entries (to_id, to_kind);
