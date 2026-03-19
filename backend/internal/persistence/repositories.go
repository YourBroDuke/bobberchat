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

type ConversationRepository interface {
	Create(ctx context.Context, conv Conversation) (*Conversation, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Conversation, error)
	GetDirectByPair(ctx context.Context, idLow, idHigh uuid.UUID) (*Conversation, error)
	ListByParticipant(ctx context.Context, participantID uuid.UUID, kind ParticipantType) ([]Conversation, error)
	ListByParticipantAndType(ctx context.Context, participantID uuid.UUID, kind ParticipantType, convType ConversationType) ([]Conversation, error)
}

type ConversationParticipantRepository interface {
	Add(ctx context.Context, p ConversationParticipant) error
	Remove(ctx context.Context, conversationID, participantID uuid.UUID, kind ParticipantType) error
	ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]ConversationParticipant, error)
	UpdateMuted(ctx context.Context, conversationID, participantID uuid.UUID, kind ParticipantType, muted bool) error
	UpdateLastRead(ctx context.Context, conversationID, participantID uuid.UUID, kind ParticipantType, messageID uuid.UUID) error
}

type ChatGroupRepository interface {
	Create(ctx context.Context, group ChatGroup) (*ChatGroup, error)
	GetByID(ctx context.Context, groupID uuid.UUID) (*ChatGroup, error)
	ListAll(ctx context.Context) ([]ChatGroup, error)
	SetConversationID(ctx context.Context, groupID, conversationID uuid.UUID) error
}

type MessageRepository interface {
	Save(ctx context.Context, message Message) (*Message, error)
	GetByID(ctx context.Context, messageID uuid.UUID) (*Message, error)
	GetByConversation(ctx context.Context, conversationID uuid.UUID, limit int, sinceTS *time.Time, sinceID *uuid.UUID) ([]Message, error)
}

type ConnectionRequestRepository interface {
	Create(ctx context.Context, req ConnectionRequest) (*ConnectionRequest, error)
	GetPendingForAgent(ctx context.Context, agentID uuid.UUID) ([]ConnectionRequest, error)
	UpdateStatus(ctx context.Context, requestID uuid.UUID, status ConnectionRequestStatus) error
	GetByFromAndTo(ctx context.Context, fromAgentID, toAgentID uuid.UUID) (*ConnectionRequest, error)
}

type BlacklistRepository interface {
	Create(ctx context.Context, entry BlacklistEntry) (*BlacklistEntry, error)
	Delete(ctx context.Context, userID, blockedUserID uuid.UUID) error
	IsBlocked(ctx context.Context, userID, blockedUserID uuid.UUID) (bool, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]BlacklistEntry, error)
}

type PostgresRepositories struct {
	Agents                   AgentRepository
	Users                    UserRepository
	Conversations            ConversationRepository
	ConversationParticipants ConversationParticipantRepository
	Groups                   ChatGroupRepository
	Messages                 MessageRepository
	ConnectionRequests       ConnectionRequestRepository
	Blacklist                BlacklistRepository
}

func NewPostgresRepositories(db *DB) *PostgresRepositories {
	return &PostgresRepositories{
		Agents:                   &pgAgentRepository{db: db},
		Users:                    &pgUserRepository{db: db},
		Conversations:            &pgConversationRepository{db: db},
		ConversationParticipants: &pgConversationParticipantRepository{db: db},
		Groups:                   &pgChatGroupRepository{db: db},
		Messages:                 &pgMessageRepository{db: db},
		ConnectionRequests:       &pgConnectionRequestRepository{db: db},
		Blacklist:                &pgBlacklistRepository{db: db},
	}
}

type pgAgentRepository struct{ db *DB }

func (r *pgAgentRepository) Create(ctx context.Context, agent Agent) (*Agent, error) {
	if agent.ID == uuid.Nil {
		agent.ID = uuid.New()
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
		agent.ID, agent.DisplayName, agent.OwnerUserID,
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
	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO users (
			id, email, password_hash, created_at,
			email_verified, verification_token, verification_token_expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, password_hash, created_at,
			email_verified, verification_token, verification_token_expires_at
	`, user.ID, strings.ToLower(strings.TrimSpace(user.Email)), user.PasswordHash, user.CreatedAt, user.EmailVerified, user.VerificationToken, user.VerificationTokenExpiresAt)

	created := User{}
	err := row.Scan(
		&created.ID,
		&created.Email,
		&created.PasswordHash,
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
		SELECT id, email, password_hash, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email)))

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
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
		SELECT id, email, password_hash, created_at,
			email_verified, verification_token, verification_token_expires_at
		FROM users
		WHERE id = $1
	`, userID)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
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
		RETURNING id, email, password_hash, created_at,
			email_verified, verification_token, verification_token_expires_at
	`, token)

	u := User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
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
		SELECT id, email, password_hash, created_at,
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

type pgConversationRepository struct{ db *DB }

func (r *pgConversationRepository) Create(ctx context.Context, conv Conversation) (*Conversation, error) {
	if conv.ID == uuid.Nil {
		conv.ID = uuid.New()
	}
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = time.Now().UTC()
	}

	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO conversations (id, type, id_low, id_high, created_at)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, type, id_low, id_high, created_at, last_message_id, last_message_at
	`, conv.ID, string(conv.Type), conv.IDLow, conv.IDHigh, conv.CreatedAt)

	return scanConversation(row)
}

func (r *pgConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*Conversation, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, type, id_low, id_high, created_at, last_message_id, last_message_at
		FROM conversations
		WHERE id = $1
	`, id)
	return scanConversation(row)
}

func (r *pgConversationRepository) GetDirectByPair(ctx context.Context, idLow, idHigh uuid.UUID) (*Conversation, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, type, id_low, id_high, created_at, last_message_id, last_message_at
		FROM conversations
		WHERE id_low = $1 AND id_high = $2
	`, idLow, idHigh)
	return scanConversation(row)
}

func (r *pgConversationRepository) ListByParticipant(ctx context.Context, participantID uuid.UUID, kind ParticipantType) ([]Conversation, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT c.id, c.type, c.id_low, c.id_high, c.created_at, c.last_message_id, c.last_message_at
		FROM conversations c
		JOIN conversation_participants cp ON cp.conversation_id = c.id
		WHERE cp.participant_id = $1 AND cp.participant_kind = $2
		ORDER BY c.created_at DESC
	`, participantID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("list conversations by participant: %w", err)
	}
	defer rows.Close()

	out := make([]Conversation, 0)
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations by participant: %w", err)
	}

	return out, nil
}

func (r *pgConversationRepository) ListByParticipantAndType(ctx context.Context, participantID uuid.UUID, kind ParticipantType, convType ConversationType) ([]Conversation, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT c.id, c.type, c.id_low, c.id_high, c.created_at, c.last_message_id, c.last_message_at
		FROM conversations c
		JOIN conversation_participants cp ON cp.conversation_id = c.id
		WHERE cp.participant_id = $1 AND cp.participant_kind = $2 AND c.type = $3
		ORDER BY c.created_at DESC
	`, participantID, string(kind), string(convType))
	if err != nil {
		return nil, fmt.Errorf("list conversations by participant and type: %w", err)
	}
	defer rows.Close()

	out := make([]Conversation, 0)
	for rows.Next() {
		conv, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations by participant and type: %w", err)
	}

	return out, nil
}

type pgConversationParticipantRepository struct{ db *DB }

func (r *pgConversationParticipantRepository) Add(ctx context.Context, p ConversationParticipant) error {
	if p.JoinedAt.IsZero() {
		p.JoinedAt = time.Now().UTC()
	}

	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO conversation_participants (
			conversation_id, participant_id, participant_kind, muted, last_read_message_id, joined_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (conversation_id, participant_id, participant_kind) DO NOTHING
	`, p.ConversationID, p.ParticipantID, string(p.ParticipantKind), p.Muted, p.LastReadMessageID, p.JoinedAt)
	if err != nil {
		return fmt.Errorf("add conversation participant: %w", err)
	}

	return nil
}

func (r *pgConversationParticipantRepository) Remove(ctx context.Context, conversationID, participantID uuid.UUID, kind ParticipantType) error {
	_, err := r.db.Pool().Exec(ctx, `
		DELETE FROM conversation_participants
		WHERE conversation_id = $1 AND participant_id = $2 AND participant_kind = $3
	`, conversationID, participantID, string(kind))
	if err != nil {
		return fmt.Errorf("remove conversation participant: %w", err)
	}

	return nil
}

func (r *pgConversationParticipantRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]ConversationParticipant, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT conversation_id, participant_id, participant_kind, muted, last_read_message_id, joined_at
		FROM conversation_participants
		WHERE conversation_id = $1
		ORDER BY joined_at ASC
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list conversation participants: %w", err)
	}
	defer rows.Close()

	out := make([]ConversationParticipant, 0)
	for rows.Next() {
		p, err := scanConversationParticipant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation participants: %w", err)
	}

	return out, nil
}

func (r *pgConversationParticipantRepository) UpdateMuted(ctx context.Context, conversationID, participantID uuid.UUID, kind ParticipantType, muted bool) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE conversation_participants
		SET muted = $4
		WHERE conversation_id = $1 AND participant_id = $2 AND participant_kind = $3
	`, conversationID, participantID, string(kind), muted)
	if err != nil {
		return fmt.Errorf("update conversation participant muted: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *pgConversationParticipantRepository) UpdateLastRead(ctx context.Context, conversationID, participantID uuid.UUID, kind ParticipantType, messageID uuid.UUID) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE conversation_participants
		SET last_read_message_id = $4
		WHERE conversation_id = $1 AND participant_id = $2 AND participant_kind = $3
	`, conversationID, participantID, string(kind), messageID)
	if err != nil {
		return fmt.Errorf("update conversation participant last read: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

type pgChatGroupRepository struct{ db *DB }

func (r *pgChatGroupRepository) Create(ctx context.Context, group ChatGroup) (*ChatGroup, error) {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now().UTC()
	}
	row := r.db.Pool().QueryRow(ctx, `
		INSERT INTO chat_groups (id, name, description, owner_id, conversation_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, name, description, owner_id, conversation_id, created_at
	`, group.ID, group.Name, group.Description, group.OwnerID, group.ConversationID, group.CreatedAt)

	created := ChatGroup{}
	err := row.Scan(&created.ID, &created.Name, &created.Description, &created.OwnerID, &created.ConversationID, &created.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create chat group: %w", err)
	}
	return &created, nil
}

func (r *pgChatGroupRepository) GetByID(ctx context.Context, groupID uuid.UUID) (*ChatGroup, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, name, description, owner_id, conversation_id, created_at
		FROM chat_groups
		WHERE id = $1
	`, groupID)

	g := ChatGroup{}
	err := row.Scan(&g.ID, &g.Name, &g.Description, &g.OwnerID, &g.ConversationID, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chat group: %w", err)
	}
	return &g, nil
}

func (r *pgChatGroupRepository) ListAll(ctx context.Context) ([]ChatGroup, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, name, description, owner_id, conversation_id, created_at
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
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.OwnerID, &g.ConversationID, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat groups: %w", err)
	}

	return groups, nil
}

func (r *pgChatGroupRepository) SetConversationID(ctx context.Context, groupID, conversationID uuid.UUID) error {
	res, err := r.db.Pool().Exec(ctx, `
		UPDATE chat_groups
		SET conversation_id = $2
		WHERE id = $1
	`, groupID, conversationID)
	if err != nil {
		return fmt.Errorf("set chat group conversation id: %w", err)
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

	metadata, err := json.Marshal(message.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal message metadata: %w", err)
	}

	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	row := tx.QueryRow(ctx, `
		INSERT INTO messages (id, from_id, conversation_id, participant_kind, tag, content, metadata, "timestamp")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, from_id, conversation_id, participant_kind, tag, content, metadata, "timestamp"
	`, message.ID, message.FromID, message.ConversationID, string(message.ParticipantKind), message.Tag, message.Content, metadata, message.Timestamp)

	out, err := scanMessage(row)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE conversations
		SET last_message_id = $1, last_message_at = $2
		WHERE id = $3
	`, out.ID, out.Timestamp, out.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("update conversation last message: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return out, nil
}

func (r *pgMessageRepository) GetByID(ctx context.Context, messageID uuid.UUID) (*Message, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, from_id, conversation_id, participant_kind, tag, content, metadata, "timestamp"
		FROM messages
		WHERE id = $1
	`, messageID)
	return scanMessage(row)
}

func (r *pgMessageRepository) GetByConversation(ctx context.Context, conversationID uuid.UUID, limit int, sinceTS *time.Time, sinceID *uuid.UUID) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, from_id, conversation_id, participant_kind, tag, content, metadata, "timestamp"
		FROM messages
		WHERE conversation_id = $1
			AND ($2::timestamptz IS NULL OR "timestamp" > $2)
			AND ($3::uuid IS NULL OR id > $3)
		ORDER BY "timestamp" DESC
		LIMIT $4
	`, conversationID, sinceTS, sinceID, limit)
	if err != nil {
		return nil, fmt.Errorf("get messages by conversation: %w", err)
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
		return nil, fmt.Errorf("iterate messages by conversation: %w", err)
	}
	return messages, nil
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
		INSERT INTO connection_requests (id, from_agent_id, to_agent_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, from_agent_id, to_agent_id, status, created_at, updated_at
	`, req.ID, req.FromAgentID, req.ToAgentID, string(req.Status), req.CreatedAt, req.UpdatedAt)

	created, err := scanConnectionRequest(row)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *pgConnectionRequestRepository) GetPendingForAgent(ctx context.Context, agentID uuid.UUID) ([]ConnectionRequest, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, from_agent_id, to_agent_id, status, created_at, updated_at
		FROM connection_requests
		WHERE to_agent_id = $1 AND status = 'PENDING'
		ORDER BY created_at DESC
	`, agentID)
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

func (r *pgConnectionRequestRepository) GetByFromAndTo(ctx context.Context, fromAgentID, toAgentID uuid.UUID) (*ConnectionRequest, error) {
	row := r.db.Pool().QueryRow(ctx, `
		SELECT id, from_agent_id, to_agent_id, status, created_at, updated_at
		FROM connection_requests
		WHERE from_agent_id = $1 AND to_agent_id = $2
	`, fromAgentID, toAgentID)
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
		&out.ID,
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
	var metadataRaw []byte
	var participantKind string

	err := scanner.Scan(
		&out.ID,
		&out.FromID,
		&out.ConversationID,
		&participantKind,
		&out.Tag,
		&out.Content,
		&metadataRaw,
		&out.Timestamp,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan message: %w", err)
	}

	out.ParticipantKind = ParticipantType(participantKind)
	out.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal message metadata: %w", err)
		}
	}

	return &out, nil
}

func scanConversation(scanner rowScanner) (*Conversation, error) {
	out := Conversation{}
	var conversationType string

	err := scanner.Scan(
		&out.ID,
		&conversationType,
		&out.IDLow,
		&out.IDHigh,
		&out.CreatedAt,
		&out.LastMessageID,
		&out.LastMessageAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan conversation: %w", err)
	}

	out.Type = ConversationType(conversationType)
	return &out, nil
}

func scanConversationParticipant(scanner rowScanner) (*ConversationParticipant, error) {
	out := ConversationParticipant{}
	var participantKind string

	err := scanner.Scan(
		&out.ConversationID,
		&out.ParticipantID,
		&participantKind,
		&out.Muted,
		&out.LastReadMessageID,
		&out.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan conversation participant: %w", err)
	}

	out.ParticipantKind = ParticipantType(participantKind)
	return &out, nil
}

func scanConnectionRequest(scanner rowScanner) (*ConnectionRequest, error) {
	out := ConnectionRequest{}
	var status string
	err := scanner.Scan(
		&out.ID,
		&out.FromAgentID,
		&out.ToAgentID,
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
