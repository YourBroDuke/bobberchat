package registry

import (
	"context"
	"strings"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/google/uuid"
)

type Service struct {
	db *persistence.DB
}

type DiscoveryQuery struct {
	Capability    string
	SupportedTags []string
	Status        []string
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

func (s *Service) UpdateStatus(ctx context.Context, agentID string, status persistence.AgentStatus) error {
	if s == nil || s.db == nil || agentID == "" {
		return persistence.ErrInvalidInput
	}
	id, err := uuid.Parse(agentID)
	if err != nil {
		return err
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Agents.UpdateStatus(ctx, id, status)
}

func (s *Service) Heartbeat(ctx context.Context, agentID string, status persistence.AgentStatus) error {
	if s == nil || s.db == nil || agentID == "" {
		return persistence.ErrInvalidInput
	}
	id, err := uuid.Parse(agentID)
	if err != nil {
		return err
	}
	if status == "" {
		a, err := s.GetAgent(ctx, agentID)
		if err != nil {
			return err
		}
		status = a.Status
	}
	_, err = s.db.Pool().Exec(ctx, `
		UPDATE agents
		SET status = $2, last_heartbeat = $3
		WHERE agent_id = $1
	`, id, string(status), time.Now().UTC())
	return err
}

func (s *Service) Discover(ctx context.Context, query DiscoveryQuery) ([]persistence.Agent, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)

	statuses := make([]persistence.AgentStatus, 0, len(query.Status))
	for _, st := range query.Status {
		if trimmed := strings.TrimSpace(st); trimmed != "" {
			statuses = append(statuses, persistence.AgentStatus(strings.ToUpper(trimmed)))
		}
	}

	if query.Limit <= 0 {
		query.Limit = 10
	}

	agents, err := repos.Agents.DiscoverByCapability(ctx, query.Capability, statuses, query.Limit)
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
		SELECT agent_id, display_name, owner_user_id, capabilities, version,
			status, api_secret_hash, connected_at, last_heartbeat, created_at
		FROM agents WHERE agent_id = $1
	`, id)

	a := persistence.Agent{}
	var status string
	if err := row.Scan(&a.AgentID, &a.DisplayName, &a.OwnerUserID, &a.Capabilities, &a.Version, &status, &a.APISecretHash, &a.ConnectedAt, &a.LastHeartbeat, &a.CreatedAt); err != nil {
		return nil, err
	}
	a.Status = persistence.AgentStatus(status)
	return &a, nil
}

func (s *Service) ListAgents(ctx context.Context) ([]persistence.Agent, error) {
	if s == nil || s.db == nil {
		return nil, persistence.ErrInvalidInput
	}
	repos := persistence.NewPostgresRepositories(s.db)
	return repos.Agents.ListAll(ctx)
}
