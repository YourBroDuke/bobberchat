package approval

import (
	"context"
	"fmt"
	"time"

	"github.com/bobberchat/bobberchat/internal/broker"
	"github.com/bobberchat/bobberchat/internal/persistence"
	"github.com/bobberchat/bobberchat/internal/protocol"
	"github.com/google/uuid"
)

type Service struct {
	db     *persistence.DB
	broker *broker.Broker
}

func NewService(db *persistence.DB, broker *broker.Broker) *Service {
	return &Service{db: db, broker: broker}
}

func (s *Service) SubmitRequest(ctx context.Context, req *persistence.ApprovalRequest) error {
	if s == nil || s.db == nil || s.broker == nil || req == nil {
		return persistence.ErrInvalidInput
	}

	repos := persistence.NewPostgresRepositories(s.db)
	created, err := repos.Approvals.Create(ctx, *req)
	if err != nil {
		return err
	}

	env := &protocol.Envelope{
		ID:        created.ApprovalID.String(),
		From:      created.AgentID.String(),
		To:        "approval_queue",
		Tag:       protocol.TagApprovalRequest,
		Payload:   map[string]any{"approval_id": created.ApprovalID.String(), "action": created.Action, "urgency": string(created.Urgency), "justification": created.Justification},
		Metadata:  map[string]any{"tenant_id": created.TenantID.String(), "timeout_ms": created.TimeoutMS},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TraceID:   created.ApprovalID.String(),
	}

	if err := s.broker.PublishMessage(ctx, env); err != nil {
		return fmt.Errorf("publish approval request event: %w", err)
	}

	return nil
}

func (s *Service) GetPending(ctx context.Context, tenantID string) ([]persistence.ApprovalRequest, error) {
	if s == nil || s.db == nil || tenantID == "" {
		return nil, persistence.ErrInvalidInput
	}

	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Approvals.GetPending(ctx, tid)
}

func (s *Service) Decide(ctx context.Context, approvalID string, decision persistence.ApprovalStatus, approverID, reason string) error {
	if s == nil || s.db == nil || s.broker == nil || approvalID == "" || approverID == "" {
		return persistence.ErrInvalidInput
	}

	aid, err := uuid.Parse(approvalID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(approverID)
	if err != nil {
		return err
	}

	row := s.db.Pool().QueryRow(ctx, `
		SELECT tenant_id, status, agent_id
		FROM approval_requests
		WHERE approval_id = $1
	`, aid)

	var tenantID uuid.UUID
	var currentStatus string
	var agentID uuid.UUID
	if err := row.Scan(&tenantID, &currentStatus, &agentID); err != nil {
		return err
	}

	if persistence.ApprovalStatus(currentStatus) != persistence.ApprovalStatusPending {
		return persistence.ErrConflict
	}
	if decision != persistence.ApprovalStatusGranted && decision != persistence.ApprovalStatusDenied {
		return persistence.ErrInvalidInput
	}

	repos := persistence.NewPostgresRepositories(s.db)
	if err := repos.Approvals.Decide(ctx, tenantID, aid, uid, decision, time.Now().UTC()); err != nil {
		if err == persistence.ErrNotFound {
			return persistence.ErrConflict
		}
		return err
	}

	tag := protocol.TagApprovalDenied
	if decision == persistence.ApprovalStatusGranted {
		tag = protocol.TagApprovalGranted
	}
	env := &protocol.Envelope{
		ID:        uuid.NewString(),
		From:      approverID,
		To:        agentID.String(),
		Tag:       tag,
		Payload:   map[string]any{"approval_id": approvalID, "decision": string(decision), "reason": reason},
		Metadata:  map[string]any{"tenant_id": tenantID.String()},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TraceID:   approvalID,
	}

	return s.broker.PublishMessage(ctx, env)
}
