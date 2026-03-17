CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$ BEGIN
  CREATE TYPE agent_status AS ENUM (
    'REGISTERED', 'CONNECTING', 'ONLINE', 'BUSY', 'IDLE', 'OFFLINE', 'DEREGISTERED', 'DEGRADED'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE group_visibility AS ENUM ('public', 'private');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE topic_status AS ENUM ('OPEN', 'IN_PROGRESS', 'RESOLVED', 'ARCHIVED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE approval_status AS ENUM ('PENDING', 'GRANTED', 'DENIED', 'TIMED_OUT', 'ESCALATED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE urgency AS ENUM ('low', 'medium', 'high', 'critical');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE participant_type AS ENUM ('user', 'agent');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email CITEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agents (
  agent_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  display_name TEXT NOT NULL,
  owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
  version TEXT NOT NULL,
  status agent_status NOT NULL DEFAULT 'REGISTERED',
  api_secret_hash TEXT NOT NULL,
  connected_at TIMESTAMPTZ,
  last_heartbeat TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  visibility group_visibility NOT NULL DEFAULT 'private',
  creator_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_group_members (
  group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
  participant_id UUID NOT NULL,
  participant_kind participant_type NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (group_id, participant_id, participant_kind)
);

CREATE TABLE IF NOT EXISTS topics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
  subject TEXT NOT NULL,
  status topic_status NOT NULL DEFAULT 'OPEN',
  parent_topic_id UUID REFERENCES topics(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  from_id UUID NOT NULL,
  to_id UUID NOT NULL,
  tag TEXT NOT NULL,
  payload JSONB NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  "timestamp" TIMESTAMPTZ NOT NULL,
  trace_id UUID NOT NULL,
  topic_id UUID REFERENCES topics(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS approval_requests (
  approval_id UUID PRIMARY KEY,
  agent_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  justification TEXT NOT NULL,
  urgency urgency NOT NULL,
  status approval_status NOT NULL DEFAULT 'PENDING',
  approver_id UUID,
  decided_at TIMESTAMPTZ,
  timeout_ms INTEGER NOT NULL CHECK (timeout_ms > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_log (
  id BIGSERIAL PRIMARY KEY,
  event_type TEXT NOT NULL,
  actor_id UUID,
  agent_id UUID,
  details JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents (status);
CREATE INDEX IF NOT EXISTS idx_agents_owner ON agents (owner_user_id);
CREATE INDEX IF NOT EXISTS idx_agents_capabilities_gin ON agents USING GIN (capabilities jsonb_path_ops);

CREATE INDEX IF NOT EXISTS idx_topics_group_status ON topics (group_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics (parent_topic_id);

CREATE INDEX IF NOT EXISTS idx_messages_trace ON messages (trace_id, "timestamp" DESC);
CREATE INDEX IF NOT EXISTS idx_messages_topic_time ON messages (topic_id, "timestamp" DESC);
CREATE INDEX IF NOT EXISTS idx_messages_to_tag_time ON messages (to_id, tag, "timestamp" DESC);

CREATE INDEX IF NOT EXISTS idx_approvals_pending ON approval_requests (status, urgency, created_at)
WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_log (event_type, created_at DESC);
