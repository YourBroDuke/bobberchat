package protocol

import (
	"strings"
	"testing"
)

func TestEnvelopeValidate(t *testing.T) {
	validEnvelope := &Envelope{
		ID:        "msg-123",
		From:      "client",
		To:        "server",
		Tag:       TagRequestData,
		Content:   `{"k":"v"}`,
		Timestamp: "2026-03-14T10:20:30Z",
	}

	tests := []struct {
		name    string
		env     *Envelope
		wantErr string
	}{
		{
			name:    "nil envelope",
			env:     nil,
			wantErr: "envelope is nil",
		},
		{
			name: "missing id",
			env: &Envelope{
				From:      "client",
				To:        "server",
				Tag:       TagRequestData,
				Content:   "",
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "id is required",
		},
		{
			name: "missing from",
			env: &Envelope{
				ID:        "msg-123",
				To:        "server",
				Tag:       TagRequestData,
				Content:   "",
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "from is required",
		},
		{
			name: "missing to",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				Tag:       TagRequestData,
				Content:   "",
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "to is required",
		},
		{
			name: "missing tag",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Content:   "",
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "tag is required",
		},
		{
			name: "missing timestamp",
			env: &Envelope{
				ID:      "msg-123",
				From:    "client",
				To:      "server",
				Tag:     TagRequestData,
				Content: "",
			},
			wantErr: "timestamp is required",
		},
		{
			name: "whitespace-only required fields",
			env: &Envelope{
				ID:        "   ",
				From:      "client",
				To:        "server",
				Tag:       TagRequestData,
				Content:   "",
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "id is required",
		},
		{
			name: "invalid tag format",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       "Request.Data",
				Content:   "",
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "invalid tag: Request.Data",
		},
		{
			name: "invalid timestamp",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       TagRequestData,
				Content:   "",
				Timestamp: "not-a-time",
			},
			wantErr: "timestamp must be valid ISO8601/RFC3339",
		},
		{
			name: "valid known tag",
			env:  validEnvelope,
		},
		{
			name: "valid with empty tag and _tag in metadata",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Content:   "",
				Metadata:  map[string]any{MetaSysTag: TagRequestAction},
				Timestamp: "2026-03-14T10:20:30Z",
			},
		},
		{
			name: "invalid _tag in metadata",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Content:   "",
				Metadata:  map[string]any{MetaSysTag: "Request.Data"},
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "invalid tag: Request.Data",
		},
		{
			name: "valid custom reverse dns tag",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       "com.example.feature-action",
				Content:   `{"x":1}`,
				Timestamp: "2026-03-14T10:20:30Z",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.env.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() error = nil, want contains %q", tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}
