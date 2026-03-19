package persistence

import (
	"time"

	"github.com/google/uuid"
)

type ConversationType string

const (
	ConversationTypeDirect ConversationType = "direct"
	ConversationTypeGroup  ConversationType = "group"
)

type ParticipantType string

const (
	ParticipantTypeUser  ParticipantType = "user"
	ParticipantTypeAgent ParticipantType = "agent"
)

type User struct {
	ID                         uuid.UUID  `json:"id"`
	Email                      string     `json:"email"`
	PasswordHash               string     `json:"-"`
	CreatedAt                  time.Time  `json:"created_at"`
	EmailVerified              bool       `json:"email_verified"`
	VerificationToken          *string    `json:"-"`
	VerificationTokenExpiresAt *time.Time `json:"-"`
}

type Agent struct {
	ID            uuid.UUID `json:"id"`
	DisplayName   string    `json:"display_name"`
	OwnerUserID   uuid.UUID `json:"owner_user_id"`
	APISecretHash string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

type Conversation struct {
	ID            uuid.UUID        `json:"id"`
	Type          ConversationType `json:"type"`
	IDLow         *uuid.UUID       `json:"id_low,omitempty"`
	IDHigh        *uuid.UUID       `json:"id_high,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	LastMessageID *uuid.UUID       `json:"last_message_id,omitempty"`
	LastMessageAt *time.Time       `json:"last_message_at,omitempty"`
}

type ConversationParticipant struct {
	ConversationID    uuid.UUID       `json:"conversation_id"`
	ParticipantID     uuid.UUID       `json:"participant_id"`
	ParticipantKind   ParticipantType `json:"participant_kind"`
	Muted             bool            `json:"muted"`
	LastReadMessageID *uuid.UUID      `json:"last_read_message_id,omitempty"`
	JoinedAt          time.Time       `json:"joined_at"`
}

type ChatGroup struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    *string    `json:"description,omitempty"`
	OwnerID        uuid.UUID  `json:"owner_id"`
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Message struct {
	ID              uuid.UUID       `json:"id"`
	FromID          uuid.UUID       `json:"from_id"`
	ConversationID  uuid.UUID       `json:"conversation_id"`
	ParticipantKind ParticipantType `json:"participant_kind"`
	Tag             string          `json:"tag"`
	Content         string          `json:"content"`
	Metadata        map[string]any  `json:"metadata"`
	Timestamp       time.Time       `json:"timestamp"`
}

type EntityType string

const (
	EntityTypeAgent EntityType = "agent"
	EntityTypeGroup EntityType = "group"
)

type ConnectionRequestStatus string

const (
	ConnectionRequestStatusPending  ConnectionRequestStatus = "PENDING"
	ConnectionRequestStatusAccepted ConnectionRequestStatus = "ACCEPTED"
	ConnectionRequestStatusRejected ConnectionRequestStatus = "REJECTED"
)

type ConnectionRequest struct {
	ID        uuid.UUID               `json:"id"`
	SenderID  uuid.UUID               `json:"sender_id"`
	FromID    uuid.UUID               `json:"from_id"`
	FromKind  EntityType              `json:"from_kind"`
	ToID      uuid.UUID               `json:"to_id"`
	ToKind    EntityType              `json:"to_kind"`
	Status    ConnectionRequestStatus `json:"status"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type BlacklistEntry struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	BlockedUserID uuid.UUID `json:"blocked_user_id"`
	CreatedAt     time.Time `json:"created_at"`
}
