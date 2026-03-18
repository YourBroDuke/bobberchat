package persistence

import (
	"time"

	"github.com/google/uuid"
)

type GroupVisibility string

const (
	GroupVisibilityPublic  GroupVisibility = "public"
	GroupVisibilityPrivate GroupVisibility = "private"
)

type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "PENDING"
	ApprovalStatusGranted   ApprovalStatus = "GRANTED"
	ApprovalStatusDenied    ApprovalStatus = "DENIED"
	ApprovalStatusTimedOut  ApprovalStatus = "TIMED_OUT"
	ApprovalStatusEscalated ApprovalStatus = "ESCALATED"
)

type Urgency string

const (
	UrgencyLow      Urgency = "low"
	UrgencyMedium   Urgency = "medium"
	UrgencyHigh     Urgency = "high"
	UrgencyCritical Urgency = "critical"
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
	Role                       string     `json:"role"`
	CreatedAt                  time.Time  `json:"created_at"`
	EmailVerified              bool       `json:"email_verified"`
	VerificationToken          *string    `json:"-"`
	VerificationTokenExpiresAt *time.Time `json:"-"`
}

type Agent struct {
	AgentID       uuid.UUID `json:"agent_id"`
	DisplayName   string    `json:"display_name"`
	OwnerUserID   uuid.UUID `json:"owner_user_id"`
	Capabilities  []string  `json:"capabilities"`
	APISecretHash string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

type ChatGroup struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Visibility  GroupVisibility `json:"visibility"`
	CreatorID   uuid.UUID       `json:"creator_id"`
	CreatedAt   time.Time       `json:"created_at"`
}

type ChatGroupMember struct {
	GroupID         uuid.UUID       `json:"group_id"`
	ParticipantID   uuid.UUID       `json:"participant_id"`
	ParticipantKind ParticipantType `json:"participant_kind"`
	JoinedAt        time.Time       `json:"joined_at"`
}

type Message struct {
	ID        uuid.UUID      `json:"id"`
	FromID    uuid.UUID      `json:"from_id"`
	ToID      uuid.UUID      `json:"to_id"`
	Tag       string         `json:"tag"`
	Payload   map[string]any `json:"payload"`
	Metadata  map[string]any `json:"metadata"`
	Timestamp time.Time      `json:"timestamp"`
	TraceID   uuid.UUID      `json:"trace_id"`
}

type ApprovalRequest struct {
	ApprovalID    uuid.UUID      `json:"approval_id"`
	AgentID       uuid.UUID      `json:"agent_id"`
	Action        string         `json:"action"`
	Justification string         `json:"justification"`
	Urgency       Urgency        `json:"urgency"`
	Status        ApprovalStatus `json:"status"`
	ApproverID    *uuid.UUID     `json:"approver_id,omitempty"`
	DecidedAt     *time.Time     `json:"decided_at,omitempty"`
	TimeoutMS     int            `json:"timeout_ms"`
	CreatedAt     time.Time      `json:"created_at"`
}

type AuditLogEntry struct {
	ID        int64          `json:"id"`
	EventType string         `json:"event_type"`
	ActorID   *uuid.UUID     `json:"actor_id,omitempty"`
	AgentID   *uuid.UUID     `json:"agent_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type ConnectionRequestStatus string

const (
	ConnectionRequestStatusPending  ConnectionRequestStatus = "PENDING"
	ConnectionRequestStatusAccepted ConnectionRequestStatus = "ACCEPTED"
	ConnectionRequestStatusRejected ConnectionRequestStatus = "REJECTED"
)

type ConnectionRequest struct {
	ID         uuid.UUID               `json:"id"`
	FromUserID uuid.UUID               `json:"from_user_id"`
	ToUserID   uuid.UUID               `json:"to_user_id"`
	Status     ConnectionRequestStatus `json:"status"`
	CreatedAt  time.Time               `json:"created_at"`
	UpdatedAt  time.Time               `json:"updated_at"`
}

type BlacklistEntry struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	BlockedUserID uuid.UUID `json:"blocked_user_id"`
	CreatedAt     time.Time `json:"created_at"`
}
