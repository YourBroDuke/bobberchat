package registry

import (
	"context"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/google/uuid"
)

type Service struct {
	db *persistence.DB
}

type DiscoveryQuery struct {
	SupportedTags []string
	Limit         int
}

func NewService(db *persistence.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Register(ctx context.Context, agent *persistence.Agent) error {
	if s == nil || s.db == nil || agent == nil {
		return persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)
	_, err := repos.Agents.Create(ctx, *agent)
	return err
}

func (s *Service) Deregister(ctx context.Context, agentID string) error {
	if s == nil || s.db == nil || agentID == "" {
		return persistence.ErrInvalidInput
	}
	id, err := uuid.Parse(agentID)
	if err != nil {
		return err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Agents.Delete(ctx, id)
}

func (s *Service) Heartbeat(ctx context.Context, agentID string) error {
	return nil
}

func (s *Service) Discover(ctx context.Context, query DiscoveryQuery) ([]persistence.Agent, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)

	if query.Limit <= 0 {
		query.Limit = 10
	}

	agents, err := repos.Agents.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	if len(agents) > query.Limit {
		agents = agents[:query.Limit]
	}

	return agents, nil
}

func (s *Service) GetAgent(ctx context.Context, agentID string) (*persistence.Agent, error) {
	if s == nil || s.db == nil || agentID == "" {
		return nil, persistence.ErrInvalidInput
	}
	id, err := uuid.Parse(agentID)
	if err != nil {
		return nil, err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Agents.GetByID(ctx, id)
}

func (s *Service) ListAgents(ctx context.Context) ([]persistence.Agent, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Agents.ListAll(ctx)
}
