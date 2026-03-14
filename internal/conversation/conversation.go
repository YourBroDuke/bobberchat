package conversation

import (
	"context"

	"github.com/bobberchat/bobberchat/internal/persistence"
	"github.com/google/uuid"
)

type Service struct {
	db *persistence.DB
}

func NewService(db *persistence.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateGroup(ctx context.Context, tenantID, name, description, visibility, creatorID string) (*persistence.ChatGroup, error) {
	if s == nil || s.db == nil || tenantID == "" || name == "" || creatorID == "" {
		return nil, persistence.ErrInvalidInput
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}
	cid, err := uuid.Parse(creatorID)
	if err != nil {
		return nil, err
	}

	v := persistence.GroupVisibility(visibility)
	if v == "" {
		v = persistence.GroupVisibilityPrivate
	}
	var desc *string
	if description != "" {
		desc = &description
	}

	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.Create(ctx, persistence.ChatGroup{
		TenantID:    tid,
		Name:        name,
		Description: desc,
		Visibility:  v,
		CreatorID:   cid,
	})
}

func (s *Service) GetGroup(ctx context.Context, groupID string) (*persistence.ChatGroup, error) {
	if s == nil || s.db == nil || groupID == "" {
		return nil, persistence.ErrInvalidInput
	}
	gid, err := uuid.Parse(groupID)
	if err != nil {
		return nil, err
	}
	row := s.db.Pool().QueryRow(ctx, `SELECT tenant_id FROM chat_groups WHERE id = $1`, gid)
	var tid uuid.UUID
	if err := row.Scan(&tid); err != nil {
		return nil, err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.GetByID(ctx, tid, gid)
}

func (s *Service) ListGroups(ctx context.Context, tenantID string) ([]persistence.ChatGroup, error) {
	if s == nil || s.db == nil || tenantID == "" {
		return nil, persistence.ErrInvalidInput
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.ListByTenant(ctx, tid)
}

func (s *Service) JoinGroup(ctx context.Context, groupID, participantID string, kind persistence.ParticipantType) error {
	if s == nil || s.db == nil || groupID == "" || participantID == "" || kind == "" {
		return persistence.ErrInvalidInput
	}
	gid, err := uuid.Parse(groupID)
	if err != nil {
		return err
	}
	pid, err := uuid.Parse(participantID)
	if err != nil {
		return err
	}

	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.AddMember(ctx, persistence.ChatGroupMember{
		GroupID:         gid,
		ParticipantID:   pid,
		ParticipantKind: kind,
	})
}

func (s *Service) LeaveGroup(ctx context.Context, groupID, participantID string, kind persistence.ParticipantType) error {
	if s == nil || s.db == nil || groupID == "" || participantID == "" || kind == "" {
		return persistence.ErrInvalidInput
	}
	gid, err := uuid.Parse(groupID)
	if err != nil {
		return err
	}
	pid, err := uuid.Parse(participantID)
	if err != nil {
		return err
	}

	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.RemoveMember(ctx, persistence.ChatGroupMember{
		GroupID:         gid,
		ParticipantID:   pid,
		ParticipantKind: kind,
	})
}

func (s *Service) CreateTopic(ctx context.Context, tenantID, groupID, subject string, parentTopicID *string) (*persistence.Topic, error) {
	if s == nil || s.db == nil || tenantID == "" || groupID == "" || subject == "" {
		return nil, persistence.ErrInvalidInput
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, err
	}
	gid, err := uuid.Parse(groupID)
	if err != nil {
		return nil, err
	}

	var parent *uuid.UUID
	if parentTopicID != nil && *parentTopicID != "" {
		pid, err := uuid.Parse(*parentTopicID)
		if err != nil {
			return nil, err
		}
		parent = &pid
	}

	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Topics.Create(ctx, persistence.Topic{
		TenantID:      tid,
		GroupID:       gid,
		Subject:       subject,
		Status:        persistence.TopicStatusOpen,
		ParentTopicID: parent,
	})
}

func (s *Service) GetTopic(ctx context.Context, topicID string) (*persistence.Topic, error) {
	if s == nil || s.db == nil || topicID == "" {
		return nil, persistence.ErrInvalidInput
	}
	tid, err := uuid.Parse(topicID)
	if err != nil {
		return nil, err
	}
	row := s.db.Pool().QueryRow(ctx, `SELECT tenant_id FROM topics WHERE id = $1`, tid)
	var tenantID uuid.UUID
	if err := row.Scan(&tenantID); err != nil {
		return nil, err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Topics.GetByID(ctx, tenantID, tid)
}

func (s *Service) ListTopics(ctx context.Context, groupID string) ([]persistence.Topic, error) {
	if s == nil || s.db == nil || groupID == "" {
		return nil, persistence.ErrInvalidInput
	}
	gid, err := uuid.Parse(groupID)
	if err != nil {
		return nil, err
	}
	row := s.db.Pool().QueryRow(ctx, `SELECT tenant_id FROM chat_groups WHERE id = $1`, gid)
	var tenantID uuid.UUID
	if err := row.Scan(&tenantID); err != nil {
		return nil, err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Topics.ListByGroup(ctx, tenantID, gid)
}

func (s *Service) UpdateTopicStatus(ctx context.Context, topicID string, status persistence.TopicStatus) error {
	if s == nil || s.db == nil || topicID == "" || status == "" {
		return persistence.ErrInvalidInput
	}
	topicUUID, err := uuid.Parse(topicID)
	if err != nil {
		return err
	}
	row := s.db.Pool().QueryRow(ctx, `SELECT tenant_id FROM topics WHERE id = $1`, topicUUID)
	var tenantID uuid.UUID
	if err := row.Scan(&tenantID); err != nil {
		return err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Topics.UpdateStatus(ctx, tenantID, topicUUID, status)
}
