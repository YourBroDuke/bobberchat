package broker

import (
	"testing"

	"github.com/bobberchat/bobberchat/backend/internal/protocol"
)

func TestSubjectForEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		env         *protocol.Envelope
		wantSubject string
		wantErr     string
	}{
		{
			name: "default family with agent recipient routes to msg subject",
			env: &protocol.Envelope{
				To:       "agent-1",
				Tag:      protocol.TagRequestAction,
				Metadata: map[string]any{},
			},
			wantSubject: "bobberchat.msg.agent-1",
		},
		{
			name: "system family routes to system subject",
			env: &protocol.Envelope{
				To:       "ignored-for-system",
				Tag:      "system.agent.connected",
				Metadata: map[string]any{},
			},
			wantSubject: "bobberchat.system.agent.connected",
		},
		{
			name: "default family with group prefix routes to group subject",
			env: &protocol.Envelope{
				To:       "group:team-42",
				Tag:      protocol.TagRequestAction,
				Metadata: map[string]any{},
			},
			wantSubject: "bobberchat.group.team-42",
		},
		{
			name: "default family with empty group id returns error",
			env: &protocol.Envelope{
				To:       "group:",
				Tag:      protocol.TagRequestAction,
				Metadata: map[string]any{},
			},
			wantErr: "group id missing",
		},
		{
			name: "default family with normal recipient routes to msg subject",
			env: &protocol.Envelope{
				To:       "agent-9",
				Tag:      protocol.TagRequestAction,
				Metadata: map[string]any{},
			},
			wantSubject: "bobberchat.msg.agent-9",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotSubject, err := subjectForEnvelope(tt.env)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				if gotSubject != "" {
					t.Fatalf("expected empty subject on error, got %q", gotSubject)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if gotSubject != tt.wantSubject {
				t.Fatalf("expected subject %q, got %q", tt.wantSubject, gotSubject)
			}
		})
	}
}
