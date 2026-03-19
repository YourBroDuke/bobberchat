package approval

import (
	"context"
	"fmt"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/broker"
	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
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
		ID:      created.ApprovalID.String(),
		From:    created.AgentID.String(),
		To:      "approval_queue",
		Content: "",
		Metadata: map[string]any{
			protocol.MetaSysTag:           protocol.TagApprovalRequest,
			protocol.MetaSysApprovalID:    created.ApprovalID.String(),
			protocol.MetaSysAction:        created.Action,
			protocol.MetaSysJustification: created.Justification,
			"timeout_ms":                  created.TimeoutMS,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.broker.PublishMessage(ctx, env); err != nil {
		return fmt.Errorf("publish approval request event: %w", err)
	}

	return nil
}

func (s *Service) GetPending(ctx context.Context) ([]persistence.ApprovalRequest, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}

	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Approvals.GetPending(ctx)
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
		SELECT status, agent_id
		FROM approval_requests
		WHERE approval_id = $1
	`, aid)

	var currentStatus string
	var agentID uuid.UUID
	if err := row.Scan(&currentStatus, &agentID); err != nil {
		return err
	}

	if persistence.ApprovalStatus(currentStatus) != persistence.ApprovalStatusPending {
		return persistence.ErrConflict
	}
	if decision != persistence.ApprovalStatusGranted && decision != persistence.ApprovalStatusDenied {
		return persistence.ErrInvalidInput
	}

	repos := persistence.NewPostgresRepositories(s.db)
	if err := repos.Approvals.Decide(ctx, aid, uid, decision, time.Now().UTC()); err != nil {
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
		ID:      uuid.NewString(),
		From:    approverID,
		To:      agentID.String(),
		Content: "",
		Metadata: map[string]any{
			protocol.MetaSysTag:        tag,
			protocol.MetaSysApprovalID: approvalID,
			protocol.MetaSysDecision:   string(decision),
			protocol.MetaSysReason:     reason,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return s.broker.PublishMessage(ctx, env)
}
