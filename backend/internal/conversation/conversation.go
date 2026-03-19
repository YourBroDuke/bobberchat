package conversation

import (
	"context"
	"errors"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/google/uuid"
)

type Service struct {
	db *persistence.DB
}

func NewService(db *persistence.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateGroup(ctx context.Context, name, description, creatorID string) (*persistence.ChatGroup, error) {
	if s == nil || s.db == nil || name == "" || creatorID == "" {
		return nil, persistence.ErrInvalidInput
	}
	cid, err := uuid.Parse(creatorID)
	if err != nil {
		return nil, err
	}

	var desc *string
	if description != "" {
		desc = &description
	}

	repos := persistence.NewPostgresRepositories(s.db)

	conv, err := repos.Conversations.Create(ctx, persistence.Conversation{
		Type: persistence.ConversationTypeGroup,
	})
	if err != nil {
		return nil, err
	}

	group, err := repos.Groups.Create(ctx, persistence.ChatGroup{
		Name:           name,
		Description:    desc,
		OwnerID:        cid,
		ConversationID: &conv.ID,
	})
	if err != nil {
		return nil, err
	}

	if err := repos.ConversationParticipants.Add(ctx, persistence.ConversationParticipant{
		ConversationID:  conv.ID,
		ParticipantID:   cid,
		ParticipantKind: persistence.ParticipantTypeUser,
	}); err != nil {
		return nil, err
	}

	return group, nil
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

func (s *Service) ListConversationsByType(ctx context.Context, participantID uuid.UUID, kind persistence.ParticipantType, convType persistence.ConversationType) ([]persistence.Conversation, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	if participantID == uuid.Nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Conversations.ListByParticipantAndType(ctx, participantID, kind, convType)
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
	group, err := repos.Groups.GetByID(ctx, gid)
	if err != nil {
		return err
	}
	if group.ConversationID == nil {
		return errors.New("group has no conversation")
	}

	return repos.ConversationParticipants.Add(ctx, persistence.ConversationParticipant{
		ConversationID:  *group.ConversationID,
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
	group, err := repos.Groups.GetByID(ctx, gid)
	if err != nil {
		return err
	}
	if group.ConversationID == nil {
		return errors.New("group has no conversation")
	}

	return repos.ConversationParticipants.Remove(ctx, *group.ConversationID, pid, kind)
}

// GetOrCreateDirect finds or creates a DM conversation between two participants.
// IDs are canonically ordered (low < high) for the unique constraint.
func (s *Service) GetOrCreateDirect(ctx context.Context, id1, id2 uuid.UUID, kind1, kind2 persistence.ParticipantType) (*persistence.Conversation, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	if id1 == uuid.Nil || id2 == uuid.Nil {
		return nil, persistence.ErrInvalidInput
	}

	idLow, idHigh := id1, id2
	kindLow, kindHigh := kind1, kind2
	if id1.String() > id2.String() {
		idLow, idHigh = id2, id1
		kindLow, kindHigh = kind2, kind1
	}

	repos := persistence.NewPostgresRepositories(s.db)

	conv, err := repos.Conversations.GetDirectByPair(ctx, idLow, idHigh)
	if err == nil {
		return conv, nil
	}
	if !errors.Is(err, persistence.ErrNotFound) {
		return nil, err
	}

	conv, err = repos.Conversations.Create(ctx, persistence.Conversation{
		Type:   persistence.ConversationTypeDirect,
		IDLow:  &idLow,
		IDHigh: &idHigh,
	})
	if err != nil {
		return nil, err
	}

	if err := repos.ConversationParticipants.Add(ctx, persistence.ConversationParticipant{
		ConversationID:  conv.ID,
		ParticipantID:   idLow,
		ParticipantKind: kindLow,
	}); err != nil {
		return nil, err
	}
	if err := repos.ConversationParticipants.Add(ctx, persistence.ConversationParticipant{
		ConversationID:  conv.ID,
		ParticipantID:   idHigh,
		ParticipantKind: kindHigh,
	}); err != nil {
		return nil, err
	}

	return conv, nil
}
