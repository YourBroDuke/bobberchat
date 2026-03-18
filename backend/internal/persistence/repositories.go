package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrConflict      = errors.New("conflict")
)

type AgentRepository interface {
	Create(ctx context.Context, agent Agent) (*Agent, error)
	GetByID(ctx context.Context, agentID uuid.UUID) (*Agent, error)
	Delete(ctx context.Context, agentID uuid.UUID) error
	ListAll(ctx context.Context) ([]Agent, error)
	ListByOwner(ctx context.Context, ownerUserID uuid.UUID) ([]Agent, error)
}

type UserRepository interface {
	Create(ctx context.Context, user User) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (*User, error)
	SetVerificationToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	VerifyEmail(ctx context.Context, token string) (*User, error)
	GetByVerificationToken(ctx context.Context, token string) (*User, error)
}

type ChatGroupRepository interface {
	Create(ctx context.Context, group ChatGroup) (*ChatGroup, error)
	GetByID(ctx context.Context, groupID uuid.UUID) (*ChatGroup, error)
	ListAll(ctx context.Context) ([]ChatGroup, error)
	AddMember(ctx context.Context, member ChatGroupMember) error
	RemoveMember(ctx context.Context, member ChatGroupMember) error
}

type MessageRepository interface {
	Save(ctx context.Context, message Message) (*Message, error)
	GetByTraceID(ctx context.Context, traceID uuid.UUID) ([]Message, error)
	GetByID(ctx context.Context, messageID uuid.UUID, timestamp time.Time) (*Message, error)
	GetByPeer(ctx context.Context, userID, peerID uuid.UUID, limit int, sinceTS *time.Time, sinceID *uuid.UUID) ([]Message, error)
}

type ConnectionRequestRepository interface {
	Create(ctx context.Context, req ConnectionRequest) (*ConnectionRequest, error)
	GetPendingForUser(ctx context.Context, userID uuid.UUID) ([]ConnectionRequest, error)
	UpdateStatus(ctx context.Context, requestID uuid.UUID, status ConnectionRequestStatus) error
	GetByFromAndTo(ctx context.Context, fromUserID, toUserID uuid.UUID) (*ConnectionRequest, error)
}

type BlacklistRepository interface {
	Create(ctx context.Context, entry BlacklistEntry) (*BlacklistEntry, error)
	Delete(ctx context.Context, userID, blockedUserID uuid.UUID) error
	IsBlocked(ctx context.Context, userID, blockedUserID uuid.UUID) (bool, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]BlacklistEntry, error)
}

type ApprovalRepository interface {
	Create(ctx context.Context, approval ApprovalRequest) (*ApprovalRequest, error)
	GetPending(ctx context.Context) ([]ApprovalRequest, error)
	Decide(ctx context.Context, approvalID, approverID uuid.UUID, status ApprovalStatus, decidedAt time.Time) error
}

type AuditLogRepository interface {
	Append(ctx context.Context, entry AuditLogEntry) (*AuditLogEntry, error)
	QueryRecent(ctx context.Context, limit int) ([]AuditLogEntry, error)
}

type PostgresRepositories struct {
	Agents             AgentRepository
	Users              UserRepository
	Groups             ChatGroupRepository
	Messages           MessageRepository
	Approvals          ApprovalRepository
	AuditLogs          AuditLogRepository
	ConnectionRequests ConnectionRequestRepository
	Blacklist          BlacklistRepository
}

func NewPostgresRepositories(db *DB) *PostgresRepositories {
	return &PostgresRepositories{
		Agents:             &pgAgentRepository{db: db},
		Users:              &pgUserRepository{db: db},
		Groups:             &pgChatGroupRepository{db: db},
		Messages:           &pgMessageRepository{db: db},
		Approvals:          &pgApprovalRepository{db: db},
		AuditLogs:          &pgAuditLogRepository{db: db},
		ConnectionRequests: &pgConnectionRequestRepository{db: db},
		Blacklist:          &pgBlacklistRepository{db: db},
	}
}

type pgAgentRepository struct{ db *DB }

func (r *pgAgentRepository) Create(ctx context.Context, agent Agent) (*Agent, error) {
	if agent.AgentID == uuid.Nil {
		agent.AgentID = uuid.New()
	}
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = time.Now().UTC()
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO agents (
			agent_id, display_name, owner_user_id,
			api_secret_hash, created_at
		) VALUES ($1,$2,$3,$4,$5)
		RETURNING agent_id, display_name, owner_user_id,
			api_secret_hash, created_at
	`,
		agent.AgentID, agent.DisplayName, agent.OwnerUserID,
		agent.APISecretHash, agent.CreatedAt,
	)

	created, err := scanAgent(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgAgentRepository) GetByID(ctx context.Context, agentID uuid.UUID) (*Agent, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT agent_id, display_name, owner_user_id,
			api_secret_hash, created_at
		FROM agents
		WHERE agent_id = $1
	`, agentID)
	return scanAgent(row)
}

func (r *pgAgentRepository) Delete(ctx context.Context, agentID uuid.UUID) error {
	res, err := r.db.Pool().Exec(ctx, `DELETE FROM agents WHERE agent_id = $1`, agentID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgAgentRepository) ListAll(ctx context.Context) ([]Agent, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT agent_id, display_name, owner_user_id,
			api_secret_hash, created_at
		FROM agents
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	results := make([]Agent, 0)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}

	return results, nil
}

func (r *pgAgentRepository) ListByOwner(ctx context.Context, ownerUserID uuid.UUID) ([]Agent, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT agent_id, display_name, owner_user_id,
			api_secret_hash, created_at
		FROM agents
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list agents by owner: %w", err)
	}
	defer rows.Close()

	results := make([]Agent, 0)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents by owner: %w", err)
	}

	return results, nil
}

type pgUserRepository struct{ db *DB }

func (r *pgUserRepository) Create(ctx context.Context, user User) (*User, error) {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	if user.Role == "" {
		user.Role = "member"
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO users (
			id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
	`, user.ID, strings.ToLower(strings.TrimSpace(user.Email)), user.PasswordHash, user.Role, user.CreatedAt, user.EmailVerified, user.VerificationToken, user.VerificationTokenExpiresAt)

	created := User{}
	err := row.Scan(
		&created.ID,
		&created.Email,
		&created.PasswordHash,
		&created.Role,
		&created.CreatedAt,
		&created.EmailVerified,
		&created.VerificationToken,
		&created.VerificationTokenExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &created, nil
}

func (r *pgUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email)))

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.EmailVerified,
		&u.VerificationToken,
		&u.VerificationTokenExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *pgUserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*User, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE id = $1
	`, userID)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.EmailVerified,
		&u.VerificationToken,
		&u.VerificationTokenExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *pgUserRepository) SetVerificationToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE users
		SET verification_token = $2, verification_token_expires_at = $3
		WHERE id = $1
	`, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("set verification token: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgUserRepository) VerifyEmail(ctx context.Context, token string) (*User, error) {
	row := r.db.Pool().QueryRow(ctx, `
		UPDATE users
		SET email_verified = TRUE,
			verification_token = NULL,
			verification_token_expires_at = NULL
		WHERE verification_token = $1
			AND verification_token IS NOT NULL
			AND verification_token_expires_at > NOW()
		RETURNING id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
	`, token)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.EmailVerified,
		&u.VerificationToken,
		&u.VerificationTokenExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("verify email: %w", err)
	}

	return &u, nil
}

func (r *pgUserRepository) GetByVerificationToken(ctx context.Context, token string) (*User, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE verification_token = $1
			AND verification_token IS NOT NULL
			AND verification_token_expires_at > NOW()
	`, token)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.EmailVerified,
		&u.VerificationToken,
		&u.VerificationTokenExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by verification token: %w", err)
	}

	return &u, nil
}

type pgChatGroupRepository struct{ db *DB }

func (r *pgChatGroupRepository) Create(ctx context.Context, group ChatGroup) (*ChatGroup, error) {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now().UTC()
	}
	if group.Visibility == "" {
		group.Visibility = GroupVisibilityPrivate
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO chat_groups (id, name, description, visibility, creator_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, name, description, visibility, creator_id, created_at
	`, group.ID, group.Name, group.Description, string(group.Visibility), group.CreatorID, group.CreatedAt)

	created := ChatGroup{}
	var visibility string
	err := row.Scan(&created.ID, &created.Name, &created.Description, &visibility, &created.CreatorID, &created.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create chat group: %w", err)
	}
	created.Visibility = GroupVisibility(visibility)
	return &created, nil
}

func (r *pgChatGroupRepository) GetByID(ctx context.Context, groupID uuid.UUID) (*ChatGroup, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, name, description, visibility, creator_id, created_at
		FROM chat_groups
		WHERE id = $1
	`, groupID)

	g := ChatGroup{}
	var visibility string
	err := row.Scan(&g.ID, &g.Name, &g.Description, &visibility, &g.CreatorID, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chat group: %w", err)
	}
	g.Visibility = GroupVisibility(visibility)
	return &g, nil
}

func (r *pgChatGroupRepository) ListAll(ctx context.Context) ([]ChatGroup, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, name, description, visibility, creator_id, created_at
		FROM chat_groups
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list chat groups: %w", err)
	}
	defer rows.Close()

	groups := make([]ChatGroup, 0)
	for rows.Next() {
		g := ChatGroup{}
		var visibility string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &visibility, &g.CreatorID, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat group: %w", err)
		}
		g.Visibility = GroupVisibility(visibility)
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat groups: %w", err)
	}

	return groups, nil
}

func (r *pgChatGroupRepository) AddMember(ctx context.Context, member ChatGroupMember) error {
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO chat_group_members (group_id, participant_id, participant_kind, joined_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (group_id, participant_id, participant_kind) DO NOTHING
	`, member.GroupID, member.ParticipantID, string(member.ParticipantKind), member.JoinedAt)
	if err != nil {
		return fmt.Errorf("add group member: %w", err)
	}
	return nil
}

func (r *pgChatGroupRepository) RemoveMember(ctx context.Context, member ChatGroupMember) error {
	_, err := r.db.Pool().Exec(ctx, `
		DELETE FROM chat_group_members
		WHERE group_id = $1 AND participant_id = $2 AND participant_kind = $3
	`, member.GroupID, member.ParticipantID, string(member.ParticipantKind))
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	return nil
}

type pgMessageRepository struct{ db *DB }

func (r *pgMessageRepository) Save(ctx context.Context, message Message) (*Message, error) {
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(message.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal message payload: %w", err)
	}
	metadata, err := json.Marshal(message.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal message metadata: %w", err)
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO messages (id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id
	`, message.ID, message.FromID, message.ToID, message.Tag, payload, metadata, message.Timestamp, message.TraceID)

	out, err := scanMessage(row)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pgMessageRepository) GetByTraceID(ctx context.Context, traceID uuid.UUID) ([]Message, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id
		FROM messages
		WHERE trace_id = $1
		ORDER BY "timestamp" ASC
	`, traceID)
	if err != nil {
		return nil, fmt.Errorf("get messages by trace id: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages by trace id: %w", err)
	}
	return messages, nil
}

func (r *pgMessageRepository) GetByID(ctx context.Context, messageID uuid.UUID, timestamp time.Time) (*Message, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id
		FROM messages
		WHERE id = $1 AND "timestamp" = $2
	`, messageID, timestamp)
	return scanMessage(row)
}

func (r *pgMessageRepository) GetByPeer(ctx context.Context, userID, peerID uuid.UUID, limit int, sinceTS *time.Time, sinceID *uuid.UUID) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id
		FROM messages
		WHERE ((from_id = $1 AND to_id = $2) OR (from_id = $2 AND to_id = $1))
			AND ($3::timestamptz IS NULL OR "timestamp" > $3)
			AND ($4::uuid IS NULL OR id > $4)
		ORDER BY "timestamp" DESC
		LIMIT $5
	`, userID, peerID, sinceTS, sinceID, limit)
	if err != nil {
		return nil, fmt.Errorf("get messages by peer: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages by peer: %w", err)
	}
	return messages, nil
}

type pgApprovalRepository struct{ db *DB }

func (r *pgApprovalRepository) Create(ctx context.Context, approval ApprovalRequest) (*ApprovalRequest, error) {
	if approval.CreatedAt.IsZero() {
		approval.CreatedAt = time.Now().UTC()
	}
	if approval.Status == "" {
		approval.Status = ApprovalStatusPending
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO approval_requests (
			approval_id, agent_id, action, justification, urgency,
			status, approver_id, decided_at, timeout_ms, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING approval_id, agent_id, action, justification, urgency,
			status, approver_id, decided_at, timeout_ms, created_at
	`, approval.ApprovalID, approval.AgentID, approval.Action,
		approval.Justification, string(approval.Urgency), string(approval.Status), approval.ApproverID,
		approval.DecidedAt, approval.TimeoutMS, approval.CreatedAt)

	out, err := scanApprovalRequest(row)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pgApprovalRepository) GetPending(ctx context.Context) ([]ApprovalRequest, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT approval_id, agent_id, action, justification, urgency,
			status, approver_id, decided_at, timeout_ms, created_at
		FROM approval_requests
		WHERE status = 'PENDING'
		ORDER BY urgency DESC, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("get pending approvals: %w", err)
	}
	defer rows.Close()

	approvals := make([]ApprovalRequest, 0)
	for rows.Next() {
		approval, err := scanApprovalRequest(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, *approval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending approvals: %w", err)
	}
	return approvals, nil
}

func (r *pgApprovalRepository) Decide(ctx context.Context, approvalID, approverID uuid.UUID, status ApprovalStatus, decidedAt time.Time) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE approval_requests
		SET status = $1, approver_id = $2, decided_at = $3
		WHERE approval_id = $4 AND status = 'PENDING'
	`, string(status), approverID, decidedAt, approvalID)
	if err != nil {
		return fmt.Errorf("decide approval: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type pgAuditLogRepository struct{ db *DB }

func (r *pgAuditLogRepository) Append(ctx context.Context, entry AuditLogEntry) (*AuditLogEntry, error) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return nil, fmt.Errorf("marshal audit details: %w", err)
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO audit_log (event_type, actor_id, agent_id, details, created_at)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, event_type, actor_id, agent_id, details, created_at
	`, entry.EventType, entry.ActorID, entry.AgentID, details, entry.CreatedAt)

	out, err := scanAuditLogEntry(row)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pgAuditLogRepository) QueryRecent(ctx context.Context, limit int) ([]AuditLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, event_type, actor_id, agent_id, details, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent audit log: %w", err)
	}
	defer rows.Close()

	entries := make([]AuditLogEntry, 0)
	for rows.Next() {
		entry, err := scanAuditLogEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log entries: %w", err)
	}

	return entries, nil
}

type pgConnectionRequestRepository struct{ db *DB }

func (r *pgConnectionRequestRepository) Create(ctx context.Context, req ConnectionRequest) (*ConnectionRequest, error) {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	now := time.Now().UTC()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	if req.UpdatedAt.IsZero() {
		req.UpdatedAt = now
	}
	if req.Status == "" {
		req.Status = ConnectionRequestStatusPending
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO connection_requests (id, from_user_id, to_user_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, from_user_id, to_user_id, status, created_at, updated_at
	`, req.ID, req.FromUserID, req.ToUserID, string(req.Status), req.CreatedAt, req.UpdatedAt)

	created, err := scanConnectionRequest(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgConnectionRequestRepository) GetPendingForUser(ctx context.Context, userID uuid.UUID) ([]ConnectionRequest, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, from_user_id, to_user_id, status, created_at, updated_at
		FROM connection_requests
		WHERE to_user_id = $1 AND status = 'PENDING'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get pending connection requests: %w", err)
	}
	defer rows.Close()

	requests := make([]ConnectionRequest, 0)
	for rows.Next() {
		req, err := scanConnectionRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, *req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending connection requests: %w", err)
	}
	return requests, nil
}

func (r *pgConnectionRequestRepository) UpdateStatus(ctx context.Context, requestID uuid.UUID, status ConnectionRequestStatus) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE connection_requests
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, string(status), requestID)
	if err != nil {
		return fmt.Errorf("update connection request status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgConnectionRequestRepository) GetByFromAndTo(ctx context.Context, fromUserID, toUserID uuid.UUID) (*ConnectionRequest, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, from_user_id, to_user_id, status, created_at, updated_at
		FROM connection_requests
		WHERE from_user_id = $1 AND to_user_id = $2
	`, fromUserID, toUserID)
	return scanConnectionRequest(row)
}

type pgBlacklistRepository struct{ db *DB }

func (r *pgBlacklistRepository) Create(ctx context.Context, entry BlacklistEntry) (*BlacklistEntry, error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO blacklist_entries (id, user_id, blocked_user_id, created_at)
		VALUES ($1,$2,$3,$4)
		RETURNING id, user_id, blocked_user_id, created_at
	`, entry.ID, entry.UserID, entry.BlockedUserID, entry.CreatedAt)

	created, err := scanBlacklistEntry(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgBlacklistRepository) Delete(ctx context.Context, userID, blockedUserID uuid.UUID) error {
	_, err := r.db.Pool().Exec(ctx, `
		DELETE FROM blacklist_entries
		WHERE user_id = $1 AND blocked_user_id = $2
	`, userID, blockedUserID)
	if err != nil {
		return fmt.Errorf("delete blacklist entry: %w", err)
	}
	return nil
}

func (r *pgBlacklistRepository) IsBlocked(ctx context.Context, userID, blockedUserID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM blacklist_entries
			WHERE user_id = $1 AND blocked_user_id = $2
		)
	`, userID, blockedUserID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check blacklist entry: %w", err)
	}
	return exists, nil
}

func (r *pgBlacklistRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]BlacklistEntry, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, user_id, blocked_user_id, created_at
		FROM blacklist_entries
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list blacklist entries: %w", err)
	}
	defer rows.Close()

	entries := make([]BlacklistEntry, 0)
	for rows.Next() {
		entry, err := scanBlacklistEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blacklist entries: %w", err)
	}
	return entries, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgent(scanner rowScanner) (*Agent, error) {
	out := Agent{}

	err := scanner.Scan(
		&out.AgentID,
		&out.DisplayName,
		&out.OwnerUserID,
		&out.APISecretHash,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan agent: %w", err)
	}

	return &out, nil
}

func scanMessage(scanner rowScanner) (*Message, error) {
	out := Message{}
	var payloadRaw []byte
	var metadataRaw []byte

	err := scanner.Scan(
		&out.ID,
		&out.FromID,
		&out.ToID,
		&out.Tag,
		&payloadRaw,
		&metadataRaw,
		&out.Timestamp,
		&out.TraceID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan message: %w", err)
	}

	out.Payload = map[string]any{}
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &out.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal message payload: %w", err)
		}
	}

	out.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal message metadata: %w", err)
		}
	}

	return &out, nil
}

func scanApprovalRequest(scanner rowScanner) (*ApprovalRequest, error) {
	out := ApprovalRequest{}
	var urgency string
	var status string
	err := scanner.Scan(
		&out.ApprovalID,
		&out.AgentID,
		&out.Action,
		&out.Justification,
		&urgency,
		&status,
		&out.ApproverID,
		&out.DecidedAt,
		&out.TimeoutMS,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan approval request: %w", err)
	}
	out.Urgency = Urgency(urgency)
	out.Status = ApprovalStatus(status)
	return &out, nil
}

func scanAuditLogEntry(scanner rowScanner) (*AuditLogEntry, error) {
	out := AuditLogEntry{}
	var detailsRaw []byte
	err := scanner.Scan(
		&out.ID,
		&out.EventType,
		&out.ActorID,
		&out.AgentID,
		&detailsRaw,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan audit log entry: %w", err)
	}
	out.Details = map[string]any{}
	if len(detailsRaw) > 0 {
		if err := json.Unmarshal(detailsRaw, &out.Details); err != nil {
			return nil, fmt.Errorf("unmarshal audit log details: %w", err)
		}
	}
	return &out, nil
}

func scanConnectionRequest(scanner rowScanner) (*ConnectionRequest, error) {
	out := ConnectionRequest{}
	var status string
	err := scanner.Scan(
		&out.ID,
		&out.FromUserID,
		&out.ToUserID,
		&status,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan connection request: %w", err)
	}
	out.Status = ConnectionRequestStatus(status)
	return &out, nil
}

func scanBlacklistEntry(scanner rowScanner) (*BlacklistEntry, error) {
	out := BlacklistEntry{}
	err := scanner.Scan(
		&out.ID,
		&out.UserID,
		&out.BlockedUserID,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan blacklist entry: %w", err)
	}
	return &out, nil
}
