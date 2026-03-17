package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
)

func TestRegister_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "nil service",
			err: func() error {
				var s *Service
				return s.Register(context.Background(), &persistence.Agent{})
			}(),
		},
		{
			name: "nil db",
			err: func() error {
				s := &Service{}
				return s.Register(context.Background(), &persistence.Agent{})
			}(),
		},
		{
			name: "nil agent",
			err: func() error {
				s := &Service{}
				return s.Register(context.Background(), nil)
			}(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.err, persistence.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", tc.err)
			}
		})
	}
}

func TestRegistryMethods_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "Deregister with empty agentID",
			err:  (&Service{}).Deregister(context.Background(), ""),
		},
		{
			name: "UpdateStatus with empty agentID",
			err:  (&Service{}).UpdateStatus(context.Background(), "", persistence.AgentStatusOnline),
		},
		{
			name: "Heartbeat with empty agentID",
			err:  (&Service{}).Heartbeat(context.Background(), "", persistence.AgentStatusOnline),
		},
		{
			name: "Discover nil db",
			err: func() error {
				_, err := (&Service{}).Discover(context.Background(), DiscoveryQuery{})
				return err
			}(),
		},
		{
			name: "GetAgent with empty agentID",
			err: func() error {
				_, err := (&Service{}).GetAgent(context.Background(), "")
				return err
			}(),
		},
		{
			name: "ListAgents nil db",
			err: func() error {
				_, err := (&Service{}).ListAgents(context.Background())
				return err
			}(),
		},
		{
			name: "nil service Deregister",
			err: func() error {
				var s *Service
				return s.Deregister(context.Background(), "agent")
			}(),
		},
		{
			name: "nil service UpdateStatus",
			err: func() error {
				var s *Service
				return s.UpdateStatus(context.Background(), "agent", persistence.AgentStatusOnline)
			}(),
		},
		{
			name: "nil service Heartbeat",
			err: func() error {
				var s *Service
				return s.Heartbeat(context.Background(), "agent", persistence.AgentStatusOnline)
			}(),
		},
		{
			name: "nil service Discover",
			err: func() error {
				var s *Service
				_, err := s.Discover(context.Background(), DiscoveryQuery{})
				return err
			}(),
		},
		{
			name: "nil service GetAgent",
			err: func() error {
				var s *Service
				_, err := s.GetAgent(context.Background(), "agent")
				return err
			}(),
		},
		{
			name: "nil service ListAgents",
			err: func() error {
				var s *Service
				_, err := s.ListAgents(context.Background())
				return err
			}(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.err, persistence.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", tc.err)
			}
		})
	}
}
