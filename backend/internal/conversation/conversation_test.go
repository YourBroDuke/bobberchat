package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
)

func TestCreateGroup_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "nil service",
			err: func() error {
				var s *Service
				_, err := s.CreateGroup(context.Background(), "name", "desc", "creator")
				return err
			}(),
		},
		{
			name: "empty name",
			err: func() error {
				_, err := (&Service{}).CreateGroup(context.Background(), "", "desc", "creator")
				return err
			}(),
		},
		{
			name: "empty creatorID",
			err: func() error {
				_, err := (&Service{}).CreateGroup(context.Background(), "name", "desc", "")
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

func TestConversationMethods_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "GetGroup empty groupID",
			err: func() error {
				_, err := (&Service{}).GetGroup(context.Background(), "")
				return err
			}(),
		},
		{
			name: "ListGroups nil db",
			err: func() error {
				_, err := (&Service{}).ListGroups(context.Background())
				return err
			}(),
		},
		{
			name: "JoinGroup empty groupID",
			err:  (&Service{}).JoinGroup(context.Background(), "", "participant", persistence.ParticipantTypeUser),
		},
		{
			name: "JoinGroup empty participantID",
			err:  (&Service{}).JoinGroup(context.Background(), "group", "", persistence.ParticipantTypeUser),
		},
		{
			name: "JoinGroup empty kind",
			err:  (&Service{}).JoinGroup(context.Background(), "group", "participant", ""),
		},
		{
			name: "LeaveGroup empty groupID",
			err:  (&Service{}).LeaveGroup(context.Background(), "", "participant", persistence.ParticipantTypeUser),
		},
		{
			name: "LeaveGroup empty participantID",
			err:  (&Service{}).LeaveGroup(context.Background(), "group", "", persistence.ParticipantTypeUser),
		},
		{
			name: "LeaveGroup empty kind",
			err:  (&Service{}).LeaveGroup(context.Background(), "group", "participant", ""),
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
