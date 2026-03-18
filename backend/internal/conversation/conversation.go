package conversation

import (
	"context"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/google/uuid"
)

type Service struct {
	db *persistence.DB
}

func NewService(db *persistence.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateGroup(ctx context.Context, name, description, visibility, creatorID string) (*persistence.ChatGroup, error) {
	if s == nil || s.db == nil || name == "" || creatorID == "" {
		return nil, persistence.ErrInvalidInput
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
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.GetByID(ctx, gid)
}

func (s *Service) ListGroups(ctx context.Context) ([]persistence.ChatGroup, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Groups.ListAll(ctx)
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
