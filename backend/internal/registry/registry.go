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
	Capability    string
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

	agents, err := repos.Agents.DiscoverByCapability(ctx, query.Capability, query.Limit)
	if err != nil {
		return nil, err
	}

	if len(query.SupportedTags) == 0 {
		return agents, nil
	}

	filtered := make([]persistence.Agent, 0, len(agents))
	for _, a := range agents {
		filtered = append(filtered, a)
	}
	return filtered, nil
}

func (s *Service) GetAgent(ctx context.Context, agentID string) (*persistence.Agent, error) {
	if s == nil || s.db == nil || agentID == "" {
		return nil, persistence.ErrInvalidInput
	}
	id, err := uuid.Parse(agentID)
	if err != nil {
		return nil, err
	}
	row := s.db.Pool().QueryRow(ctx, `
		SELECT agent_id, display_name, owner_user_id, capabilities,
			api_secret_hash, created_at
		FROM agents WHERE agent_id = $1
	`, id)

	a := persistence.Agent{}
	if err := row.Scan(&a.AgentID, &a.DisplayName, &a.OwnerUserID, &a.Capabilities, &a.APISecretHash, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) ListAgents(ctx context.Context) ([]persistence.Agent, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Agents.ListAll(ctx)
}
