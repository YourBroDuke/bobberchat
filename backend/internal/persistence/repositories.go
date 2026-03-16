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
	GetByID(ctx context.Context, tenantID, agentID uuid.UUID) (*Agent, error)
	UpdateStatus(ctx context.Context, tenantID, agentID uuid.UUID, status AgentStatus) error
	Delete(ctx context.Context, tenantID, agentID uuid.UUID) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Agent, error)
	ListByOwner(ctx context.Context, tenantID, ownerUserID uuid.UUID) ([]Agent, error)
	DiscoverByCapability(ctx context.Context, tenantID uuid.UUID, capability string, statuses []AgentStatus, limit int) ([]Agent, error)
}

type UserRepository interface {
	Create(ctx context.Context, user User) (*User, error)
	GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error)
	GetByID(ctx context.Context, tenantID, userID uuid.UUID) (*User, error)
	SetVerificationToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	VerifyEmail(ctx context.Context, token string) (*User, error)
	GetByVerificationToken(ctx context.Context, token string) (*User, error)
}

type ChatGroupRepository interface {
	Create(ctx context.Context, group ChatGroup) (*ChatGroup, error)
	GetByID(ctx context.Context, tenantID, groupID uuid.UUID) (*ChatGroup, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]ChatGroup, error)
	AddMember(ctx context.Context, member ChatGroupMember) error
	RemoveMember(ctx context.Context, member ChatGroupMember) error
}

type TopicRepository interface {
	Create(ctx context.Context, topic Topic) (*Topic, error)
	GetByID(ctx context.Context, tenantID, topicID uuid.UUID) (*Topic, error)
	ListByGroup(ctx context.Context, tenantID, groupID uuid.UUID) ([]Topic, error)
	UpdateStatus(ctx context.Context, tenantID, topicID uuid.UUID, status TopicStatus) error
}

type MessageRepository interface {
	Save(ctx context.Context, message Message) (*Message, error)
	GetByTraceID(ctx context.Context, tenantID, traceID uuid.UUID) ([]Message, error)
	GetByTopic(ctx context.Context, tenantID, topicID uuid.UUID) ([]Message, error)
	GetByID(ctx context.Context, tenantID, messageID uuid.UUID, timestamp time.Time) (*Message, error)
	GetByPeer(ctx context.Context, tenantID, userID, peerID uuid.UUID, limit int, sinceTS *time.Time, sinceID *uuid.UUID) ([]Message, error)
}

type ConnectionRequestRepository interface {
	Create(ctx context.Context, req ConnectionRequest) (*ConnectionRequest, error)
	GetPendingForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]ConnectionRequest, error)
	UpdateStatus(ctx context.Context, tenantID, requestID uuid.UUID, status ConnectionRequestStatus) error
	GetByFromAndTo(ctx context.Context, tenantID, fromUserID, toUserID uuid.UUID) (*ConnectionRequest, error)
}

type BlacklistRepository interface {
	Create(ctx context.Context, entry BlacklistEntry) (*BlacklistEntry, error)
	Delete(ctx context.Context, tenantID, userID, blockedUserID uuid.UUID) error
	IsBlocked(ctx context.Context, tenantID, userID, blockedUserID uuid.UUID) (bool, error)
	ListByUser(ctx context.Context, tenantID, userID uuid.UUID) ([]BlacklistEntry, error)
}

type ApprovalRepository interface {
	Create(ctx context.Context, approval ApprovalRequest) (*ApprovalRequest, error)
	GetPending(ctx context.Context, tenantID uuid.UUID) ([]ApprovalRequest, error)
	Decide(ctx context.Context, tenantID, approvalID, approverID uuid.UUID, status ApprovalStatus, decidedAt time.Time) error
}

type AuditLogRepository interface {
	Append(ctx context.Context, entry AuditLogEntry) (*AuditLogEntry, error)
	QueryByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]AuditLogEntry, error)
}

type PostgresRepositories struct {
	Agents             AgentRepository
	Users              UserRepository
	Groups             ChatGroupRepository
	Topics             TopicRepository
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
		Topics:             &pgTopicRepository{db: db},
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
	if agent.Status == "" {
		agent.Status = AgentStatusRegistered
	}

	capabilities, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return nil, fmt.Errorf("marshal capabilities: %w", err)
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO agents (
			agent_id, tenant_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING agent_id, tenant_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
	`,
		agent.AgentID, agent.TenantID, agent.DisplayName, agent.OwnerUserID, capabilities,
		agent.Version, string(agent.Status), agent.APISecretHash, agent.ConnectedAt,
		agent.LastHeartbeat, agent.CreatedAt,
	)

	created, err := scanAgent(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgAgentRepository) GetByID(ctx context.Context, tenantID, agentID uuid.UUID) (*Agent, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT agent_id, tenant_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
		FROM agents
		WHERE tenant_id = $1 AND agent_id = $2
	`, tenantID, agentID)
	return scanAgent(row)
}

func (r *pgAgentRepository) UpdateStatus(ctx context.Context, tenantID, agentID uuid.UUID, status AgentStatus) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE agents SET status = $1 WHERE tenant_id = $2 AND agent_id = $3
	`, string(status), tenantID, agentID)
	if err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgAgentRepository) Delete(ctx context.Context, tenantID, agentID uuid.UUID) error {
	res, err := r.db.Pool().Exec(ctx, `DELETE FROM agents WHERE tenant_id = $1 AND agent_id = $2`, tenantID, agentID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgAgentRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Agent, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT agent_id, tenant_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
		FROM agents
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
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

func (r *pgAgentRepository) ListByOwner(ctx context.Context, tenantID, ownerUserID uuid.UUID) ([]Agent, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT agent_id, tenant_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
		FROM agents
		WHERE tenant_id = $1 AND owner_user_id = $2
		ORDER BY created_at DESC
	`, tenantID, ownerUserID)
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

func (r *pgAgentRepository) DiscoverByCapability(ctx context.Context, tenantID uuid.UUID, capability string, statuses []AgentStatus, limit int) ([]Agent, error) {
	if limit <= 0 {
		limit = 10
	}

	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusValues = append(statusValues, string(status))
	}

	rows, err := r.db.Pool().Query(ctx, `
		SELECT agent_id, tenant_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
		FROM agents
		WHERE tenant_id = $1
			AND capabilities @> to_jsonb($2::text[])
			AND (
				cardinality($3::text[]) = 0
				OR status::text = ANY($3::text[])
			)
		ORDER BY last_heartbeat DESC NULLS LAST, created_at DESC
		LIMIT $4
	`, tenantID, []string{capability}, statusValues, limit)
	if err != nil {
		return nil, fmt.Errorf("discover agents: %w", err)
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
		return nil, fmt.Errorf("iterate discovered agents: %w", err)
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
			id, tenant_id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
	`, user.ID, user.TenantID, strings.ToLower(strings.TrimSpace(user.Email)), user.PasswordHash, user.Role, user.CreatedAt, user.EmailVerified, user.VerificationToken, user.VerificationTokenExpiresAt)

	created := User{}
	err := row.Scan(
		&created.ID,
		&created.TenantID,
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

func (r *pgUserRepository) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE tenant_id = $1 AND email = $2
	`, tenantID, strings.ToLower(strings.TrimSpace(email)))

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.TenantID,
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

func (r *pgUserRepository) GetByID(ctx context.Context, tenantID, userID uuid.UUID) (*User, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, userID)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.TenantID,
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
		RETURNING id, tenant_id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
	`, token)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.TenantID,
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
		SELECT id, tenant_id, email, password_hash, role, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE verification_token = $1
			AND verification_token IS NOT NULL
			AND verification_token_expires_at > NOW()
	`, token)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.TenantID,
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
		INSERT INTO chat_groups (id, tenant_id, name, description, visibility, creator_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant_id, name, description, visibility, creator_id, created_at
	`, group.ID, group.TenantID, group.Name, group.Description, string(group.Visibility), group.CreatorID, group.CreatedAt)

	created := ChatGroup{}
	var visibility string
	err := row.Scan(&created.ID, &created.TenantID, &created.Name, &created.Description, &visibility, &created.CreatorID, &created.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create chat group: %w", err)
	}
	created.Visibility = GroupVisibility(visibility)
	return &created, nil
}

func (r *pgChatGroupRepository) GetByID(ctx context.Context, tenantID, groupID uuid.UUID) (*ChatGroup, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, name, description, visibility, creator_id, created_at
		FROM chat_groups
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, groupID)

	g := ChatGroup{}
	var visibility string
	err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &visibility, &g.CreatorID, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chat group: %w", err)
	}
	g.Visibility = GroupVisibility(visibility)
	return &g, nil
}

func (r *pgChatGroupRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]ChatGroup, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, name, description, visibility, creator_id, created_at
		FROM chat_groups
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list chat groups: %w", err)
	}
	defer rows.Close()

	groups := make([]ChatGroup, 0)
	for rows.Next() {
		g := ChatGroup{}
		var visibility string
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &visibility, &g.CreatorID, &g.CreatedAt); err != nil {
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

type pgTopicRepository struct{ db *DB }

func (r *pgTopicRepository) Create(ctx context.Context, topic Topic) (*Topic, error) {
	if topic.ID == uuid.Nil {
		topic.ID = uuid.New()
	}
	if topic.CreatedAt.IsZero() {
		topic.CreatedAt = time.Now().UTC()
	}
	if topic.Status == "" {
		topic.Status = TopicStatusOpen
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO topics (id, tenant_id, group_id, subject, status, parent_topic_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant_id, group_id, subject, status, parent_topic_id, created_at
	`, topic.ID, topic.TenantID, topic.GroupID, topic.Subject, string(topic.Status), topic.ParentTopicID, topic.CreatedAt)

	out := Topic{}
	var status string
	err := row.Scan(&out.ID, &out.TenantID, &out.GroupID, &out.Subject, &status, &out.ParentTopicID, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create topic: %w", err)
	}
	out.Status = TopicStatus(status)
	return &out, nil
}

func (r *pgTopicRepository) GetByID(ctx context.Context, tenantID, topicID uuid.UUID) (*Topic, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, group_id, subject, status, parent_topic_id, created_at
		FROM topics
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, topicID)

	t := Topic{}
	var status string
	err := row.Scan(&t.ID, &t.TenantID, &t.GroupID, &t.Subject, &status, &t.ParentTopicID, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get topic: %w", err)
	}
	t.Status = TopicStatus(status)
	return &t, nil
}

func (r *pgTopicRepository) ListByGroup(ctx context.Context, tenantID, groupID uuid.UUID) ([]Topic, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, group_id, subject, status, parent_topic_id, created_at
		FROM topics
		WHERE tenant_id = $1 AND group_id = $2
		ORDER BY created_at DESC
	`, tenantID, groupID)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	topics := make([]Topic, 0)
	for rows.Next() {
		t := Topic{}
		var status string
		if err := rows.Scan(&t.ID, &t.TenantID, &t.GroupID, &t.Subject, &status, &t.ParentTopicID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		t.Status = TopicStatus(status)
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topics: %w", err)
	}

	return topics, nil
}

func (r *pgTopicRepository) UpdateStatus(ctx context.Context, tenantID, topicID uuid.UUID, status TopicStatus) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE topics SET status = $1 WHERE tenant_id = $2 AND id = $3
	`, string(status), tenantID, topicID)
	if err != nil {
		return fmt.Errorf("update topic status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
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
		INSERT INTO messages (id, tenant_id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, tenant_id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id
	`, message.ID, message.TenantID, message.FromID, message.ToID, message.Tag, payload, metadata, message.Timestamp, message.TraceID, message.TopicID)

	out, err := scanMessage(row)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pgMessageRepository) GetByTraceID(ctx context.Context, tenantID, traceID uuid.UUID) ([]Message, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id
		FROM messages
		WHERE tenant_id = $1 AND trace_id = $2
		ORDER BY "timestamp" ASC
	`, tenantID, traceID)
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

func (r *pgMessageRepository) GetByTopic(ctx context.Context, tenantID, topicID uuid.UUID) ([]Message, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id
		FROM messages
		WHERE tenant_id = $1 AND topic_id = $2
		ORDER BY "timestamp" ASC
	`, tenantID, topicID)
	if err != nil {
		return nil, fmt.Errorf("get messages by topic: %w", err)
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
		return nil, fmt.Errorf("iterate messages by topic: %w", err)
	}
	return messages, nil
}

func (r *pgMessageRepository) GetByID(ctx context.Context, tenantID, messageID uuid.UUID, timestamp time.Time) (*Message, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id
		FROM messages
		WHERE tenant_id = $1 AND id = $2 AND "timestamp" = $3
	`, tenantID, messageID, timestamp)
	return scanMessage(row)
}

func (r *pgMessageRepository) GetByPeer(ctx context.Context, tenantID, userID, peerID uuid.UUID, limit int, sinceTS *time.Time, sinceID *uuid.UUID) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id
		FROM messages
		WHERE tenant_id = $1
			AND ((from_id = $2 AND to_id = $3) OR (from_id = $3 AND to_id = $2))
			AND ($4::timestamptz IS NULL OR "timestamp" > $4)
			AND ($5::uuid IS NULL OR id > $5)
		ORDER BY "timestamp" DESC
		LIMIT $6
	`, tenantID, peerID, userID, sinceTS, sinceID, limit)
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
			approval_id, tenant_id, agent_id, action, justification, urgency,
			status, approver_id, decided_at, timeout_ms, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING approval_id, tenant_id, agent_id, action, justification, urgency,
			status, approver_id, decided_at, timeout_ms, created_at
	`, approval.ApprovalID, approval.TenantID, approval.AgentID, approval.Action,
		approval.Justification, string(approval.Urgency), string(approval.Status), approval.ApproverID,
		approval.DecidedAt, approval.TimeoutMS, approval.CreatedAt)

	out, err := scanApprovalRequest(row)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pgApprovalRepository) GetPending(ctx context.Context, tenantID uuid.UUID) ([]ApprovalRequest, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT approval_id, tenant_id, agent_id, action, justification, urgency,
			status, approver_id, decided_at, timeout_ms, created_at
		FROM approval_requests
		WHERE tenant_id = $1 AND status = 'PENDING'
		ORDER BY urgency DESC, created_at ASC
	`, tenantID)
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

func (r *pgApprovalRepository) Decide(ctx context.Context, tenantID, approvalID, approverID uuid.UUID, status ApprovalStatus, decidedAt time.Time) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE approval_requests
		SET status = $1, approver_id = $2, decided_at = $3
		WHERE tenant_id = $4 AND approval_id = $5 AND status = 'PENDING'
	`, string(status), approverID, decidedAt, tenantID, approvalID)
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
		INSERT INTO audit_log (event_type, actor_id, agent_id, tenant_id, details, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, event_type, actor_id, agent_id, tenant_id, details, created_at
	`, entry.EventType, entry.ActorID, entry.AgentID, entry.TenantID, details, entry.CreatedAt)

	out, err := scanAuditLogEntry(row)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *pgAuditLogRepository) QueryByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]AuditLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, event_type, actor_id, agent_id, tenant_id, details, created_at
		FROM audit_log
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit log by tenant: %w", err)
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
		INSERT INTO connection_requests (id, tenant_id, from_user_id, to_user_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant_id, from_user_id, to_user_id, status, created_at, updated_at
	`, req.ID, req.TenantID, req.FromUserID, req.ToUserID, string(req.Status), req.CreatedAt, req.UpdatedAt)

	created, err := scanConnectionRequest(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgConnectionRequestRepository) GetPendingForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]ConnectionRequest, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, from_user_id, to_user_id, status, created_at, updated_at
		FROM connection_requests
		WHERE tenant_id = $1 AND to_user_id = $2 AND status = 'PENDING'
		ORDER BY created_at DESC
	`, tenantID, userID)
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

func (r *pgConnectionRequestRepository) UpdateStatus(ctx context.Context, tenantID, requestID uuid.UUID, status ConnectionRequestStatus) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE connection_requests
		SET status = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND id = $3
	`, string(status), tenantID, requestID)
	if err != nil {
		return fmt.Errorf("update connection request status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgConnectionRequestRepository) GetByFromAndTo(ctx context.Context, tenantID, fromUserID, toUserID uuid.UUID) (*ConnectionRequest, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, from_user_id, to_user_id, status, created_at, updated_at
		FROM connection_requests
		WHERE tenant_id = $1 AND from_user_id = $2 AND to_user_id = $3
	`, tenantID, fromUserID, toUserID)
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
		INSERT INTO blacklist_entries (id, tenant_id, user_id, blocked_user_id, created_at)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, user_id, blocked_user_id, created_at
	`, entry.ID, entry.TenantID, entry.UserID, entry.BlockedUserID, entry.CreatedAt)

	created, err := scanBlacklistEntry(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgBlacklistRepository) Delete(ctx context.Context, tenantID, userID, blockedUserID uuid.UUID) error {
	_, err := r.db.Pool().Exec(ctx, `
		DELETE FROM blacklist_entries
		WHERE tenant_id = $1 AND user_id = $2 AND blocked_user_id = $3
	`, tenantID, userID, blockedUserID)
	if err != nil {
		return fmt.Errorf("delete blacklist entry: %w", err)
	}
	return nil
}

func (r *pgBlacklistRepository) IsBlocked(ctx context.Context, tenantID, userID, blockedUserID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Pool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM blacklist_entries
			WHERE tenant_id = $1 AND user_id = $2 AND blocked_user_id = $3
		)
	`, tenantID, userID, blockedUserID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check blacklist entry: %w", err)
	}
	return exists, nil
}

func (r *pgBlacklistRepository) ListByUser(ctx context.Context, tenantID, userID uuid.UUID) ([]BlacklistEntry, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, tenant_id, user_id, blocked_user_id, created_at
		FROM blacklist_entries
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`, tenantID, userID)
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
	var status string
	var capabilitiesRaw []byte

	err := scanner.Scan(
		&out.AgentID,
		&out.TenantID,
		&out.DisplayName,
		&out.OwnerUserID,
		&capabilitiesRaw,
		&out.Version,
		&status,
		&out.APISecretHash,
		&out.ConnectedAt,
		&out.LastHeartbeat,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan agent: %w", err)
	}

	out.Status = AgentStatus(status)
	if len(capabilitiesRaw) > 0 {
		if err := json.Unmarshal(capabilitiesRaw, &out.Capabilities); err != nil {
			return nil, fmt.Errorf("unmarshal agent capabilities: %w", err)
		}
	} else {
		out.Capabilities = make([]string, 0)
	}

	return &out, nil
}

func scanMessage(scanner rowScanner) (*Message, error) {
	out := Message{}
	var payloadRaw []byte
	var metadataRaw []byte

	err := scanner.Scan(
		&out.ID,
		&out.TenantID,
		&out.FromID,
		&out.ToID,
		&out.Tag,
		&payloadRaw,
		&metadataRaw,
		&out.Timestamp,
		&out.TraceID,
		&out.TopicID,
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
		&out.TenantID,
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
		&out.TenantID,
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
		&out.TenantID,
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
		&out.TenantID,
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
