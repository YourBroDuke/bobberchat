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
		Payload:   map[string]any{"k": "v"},
		Timestamp: "2026-03-14T10:20:30Z",
		TraceID:   "trace-123",
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
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
			},
			wantErr: "id is required",
		},
		{
			name: "missing from",
			env: &Envelope{
				ID:        "msg-123",
				To:        "server",
				Tag:       TagRequestData,
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
			},
			wantErr: "from is required",
		},
		{
			name: "missing to",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				Tag:       TagRequestData,
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
			},
			wantErr: "to is required",
		},
		{
			name: "missing tag",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
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
				Payload: map[string]any{},
				TraceID: "trace-123",
			},
			wantErr: "timestamp is required",
		},
		{
			name: "missing trace id",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       TagRequestData,
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
			},
			wantErr: "trace_id is required",
		},
		{
			name: "whitespace-only required fields",
			env: &Envelope{
				ID:        "   ",
				From:      "client",
				To:        "server",
				Tag:       TagRequestData,
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
			},
			wantErr: "id is required",
		},
		{
			name: "nil payload",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       TagRequestData,
				Payload:   nil,
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
			},
			wantErr: "payload must not be nil",
		},
		{
			name: "invalid tag format",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       "Request.Data",
				Payload:   map[string]any{},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
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
				Payload:   map[string]any{},
				Timestamp: "not-a-time",
				TraceID:   "trace-123",
			},
			wantErr: "timestamp must be valid ISO8601/RFC3339",
		},
		{
			name: "valid known tag",
			env:  validEnvelope,
		},
		{
			name: "valid custom reverse dns tag",
			env: &Envelope{
				ID:        "msg-123",
				From:      "client",
				To:        "server",
				Tag:       "com.example.feature-action",
				Payload:   map[string]any{"x": 1},
				Timestamp: "2026-03-14T10:20:30Z",
				TraceID:   "trace-123",
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
