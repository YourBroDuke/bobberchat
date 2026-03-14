package persistence

import (
	"time"

	"github.com/google/uuid"
)

type AgentStatus string

const (
	AgentStatusRegistered   AgentStatus = "REGISTERED"
	AgentStatusConnecting   AgentStatus = "CONNECTING"
	AgentStatusOnline       AgentStatus = "ONLINE"
	AgentStatusBusy         AgentStatus = "BUSY"
	AgentStatusIdle         AgentStatus = "IDLE"
	AgentStatusOffline      AgentStatus = "OFFLINE"
	AgentStatusDeregistered AgentStatus = "DEREGISTERED"
	AgentStatusDegraded     AgentStatus = "DEGRADED"
)

type GroupVisibility string

const (
	GroupVisibilityPublic  GroupVisibility = "public"
	GroupVisibilityPrivate GroupVisibility = "private"
)

type TopicStatus string

const (
	TopicStatusOpen       TopicStatus = "OPEN"
	TopicStatusInProgress TopicStatus = "IN_PROGRESS"
	TopicStatusResolved   TopicStatus = "RESOLVED"
	TopicStatusArchived   TopicStatus = "ARCHIVED"
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
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

type Agent struct {
	AgentID       uuid.UUID
	TenantID      uuid.UUID
	DisplayName   string
	OwnerUserID   uuid.UUID
	Capabilities  []string
	Version       string
	Status        AgentStatus
	APISecretHash string
	ConnectedAt   *time.Time
	LastHeartbeat *time.Time
	CreatedAt     time.Time
}

type ChatGroup struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description *string
	Visibility  GroupVisibility
	CreatorID   uuid.UUID
	CreatedAt   time.Time
}

type ChatGroupMember struct {
	GroupID          uuid.UUID
	ParticipantID    uuid.UUID
	ParticipantKind  ParticipantType
	JoinedAt         time.Time
}

type Topic struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	GroupID       uuid.UUID
	Subject       string
	Status        TopicStatus
	ParentTopicID *uuid.UUID
	CreatedAt     time.Time
}

type Message struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	FromID    uuid.UUID
	ToID      uuid.UUID
	Tag       string
	Payload   map[string]any
	Metadata  map[string]any
	Timestamp time.Time
	TraceID   uuid.UUID
	TopicID   *uuid.UUID
}

type ApprovalRequest struct {
	ApprovalID     uuid.UUID
	TenantID       uuid.UUID
	AgentID        uuid.UUID
	Action         string
	Justification  string
	Urgency        Urgency
	Status         ApprovalStatus
	ApproverID     *uuid.UUID
	DecidedAt      *time.Time
	TimeoutMS      int
	CreatedAt      time.Time
}

type AuditLogEntry struct {
	ID        int64
	EventType string
	ActorID   *uuid.UUID
	AgentID   *uuid.UUID
	TenantID  uuid.UUID
	Details   map[string]any
	CreatedAt time.Time
}
