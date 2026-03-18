-- Migration 011: Remove urgency from approval_requests table
-- The urgency field is being removed from the approval request entity.

DROP INDEX IF EXISTS idx_approvals_pending;
ALTER TABLE approval_requests DROP COLUMN IF EXISTS urgency;

CREATE INDEX IF NOT EXISTS idx_approvals_pending ON approval_requests (status, created_at)
WHERE status = 'PENDING';

DROP TYPE IF EXISTS urgency;
