CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE agent_status AS ENUM (
  'REGISTERED', 'CONNECTING', 'ONLINE', 'BUSY', 'IDLE', 'OFFLINE', 'DEREGISTERED', 'DEGRADED'
);

CREATE TYPE group_visibility AS ENUM ('public', 'private');

CREATE TYPE topic_status AS ENUM ('OPEN', 'IN_PROGRESS', 'RESOLVED', 'ARCHIVED');

CREATE TYPE approval_status AS ENUM ('PENDING', 'GRANTED', 'DENIED', 'TIMED_OUT', 'ESCALATED');

CREATE TYPE urgency AS ENUM ('low', 'medium', 'high', 'critical');

CREATE TYPE participant_type AS ENUM ('user', 'agent');

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  email CITEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, email)
);

CREATE TABLE agents (
  agent_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
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

CREATE TABLE chat_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  visibility group_visibility NOT NULL DEFAULT 'private',
  creator_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, name)
);

CREATE TABLE chat_group_members (
  group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
  participant_id UUID NOT NULL,
  participant_kind participant_type NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (group_id, participant_id, participant_kind)
);

CREATE TABLE topics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  group_id UUID NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
  subject TEXT NOT NULL,
  status topic_status NOT NULL DEFAULT 'OPEN',
  parent_topic_id UUID REFERENCES topics(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE messages (
  id UUID NOT NULL,
  tenant_id UUID NOT NULL,
  from_id UUID NOT NULL,
  to_id UUID NOT NULL,
  tag TEXT NOT NULL,
  payload JSONB NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  "timestamp" TIMESTAMPTZ NOT NULL,
  trace_id UUID NOT NULL,
  topic_id UUID REFERENCES topics(id) ON DELETE SET NULL,
  PRIMARY KEY (tenant_id, "timestamp", id)
) PARTITION BY LIST (tenant_id);

CREATE TABLE approval_requests (
  approval_id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
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

CREATE TABLE audit_log (
  id BIGSERIAL PRIMARY KEY,
  event_type TEXT NOT NULL,
  actor_id UUID,
  agent_id UUID,
  tenant_id UUID NOT NULL,
  details JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_tenant_status ON agents (tenant_id, status);
CREATE INDEX idx_agents_tenant_owner ON agents (tenant_id, owner_user_id);
CREATE INDEX idx_agents_capabilities_gin ON agents USING GIN (capabilities jsonb_path_ops);

CREATE INDEX idx_topics_group_status ON topics (tenant_id, group_id, status, created_at DESC);
CREATE INDEX idx_topics_parent ON topics (tenant_id, parent_topic_id);

CREATE INDEX idx_messages_trace ON messages (tenant_id, trace_id, "timestamp" DESC);
CREATE INDEX idx_messages_topic_time ON messages (tenant_id, topic_id, "timestamp" DESC);
CREATE INDEX idx_messages_to_tag_time ON messages (tenant_id, to_id, tag, "timestamp" DESC);

CREATE INDEX idx_approvals_pending ON approval_requests (tenant_id, status, urgency, created_at)
WHERE status = 'PENDING';

CREATE INDEX idx_audit_tenant_time ON audit_log (tenant_id, created_at DESC);
CREATE INDEX idx_audit_event_type ON audit_log (tenant_id, event_type, created_at DESC);
