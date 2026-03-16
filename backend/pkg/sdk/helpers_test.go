package sdk

import (
	"testing"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/google/uuid"
)

func TestNewMessage(t *testing.T) {
	payload := map[string]any{"k": "v"}
	msg := NewMessage("agent-a", "agent-b", "request.data", payload)

	if msg.From != "agent-a" {
		t.Fatalf("expected From=agent-a, got %q", msg.From)
	}
	if msg.To != "agent-b" {
		t.Fatalf("expected To=agent-b, got %q", msg.To)
	}
	if msg.Tag != "request.data" {
		t.Fatalf("expected Tag=request.data, got %q", msg.Tag)
	}
	if msg.Payload == nil {
		t.Fatal("expected non-nil payload")
	}
	if got := msg.Payload["k"]; got != "v" {
		t.Fatalf("expected payload[k]=v, got %v", got)
	}

	if msg.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}
	if got := msg.Metadata["protocol_version"]; got != "1.0.0" {
		t.Fatalf("expected protocol_version=1.0.0, got %v", got)
	}

	assertValidUUID(t, msg.ID, "ID")
	assertValidUUID(t, msg.TraceID, "TraceID")
	assertRFC3339Timestamp(t, msg.Timestamp)
}

func TestNewMessageNilPayloadDefaultsToEmptyMap(t *testing.T) {
	msg := NewMessage("from", "to", "request.data", nil)
	if msg.Payload == nil {
		t.Fatal("expected payload to default to empty map, got nil")
	}
	if len(msg.Payload) != 0 {
		t.Fatalf("expected empty payload map, got len=%d", len(msg.Payload))
	}
}

func TestNewRequestMessageUsesRequestDataTag(t *testing.T) {
	msg := NewRequestMessage("from", "to", map[string]any{"q": 1})
	if msg.Tag != protocol.TagRequestData {
		t.Fatalf("expected tag %q, got %q", protocol.TagRequestData, msg.Tag)
	}
}

func TestNewResponseMessageSetsSuccessTagAndRequestID(t *testing.T) {
	msg := NewResponseMessage("from", "to", "req-123", map[string]any{"ok": true})

	if msg.Tag != protocol.TagResponseSuccess {
		t.Fatalf("expected tag %q, got %q", protocol.TagResponseSuccess, msg.Tag)
	}
	if msg.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}
	if got := msg.Metadata["request_id"]; got != "req-123" {
		t.Fatalf("expected metadata request_id=req-123, got %v", got)
	}
}

func assertValidUUID(t *testing.T, value, field string) {
	t.Helper()
	if _, err := uuid.Parse(value); err != nil {
		t.Fatalf("expected %s to be valid UUID, got %q (err: %v)", field, value, err)
	}
}

func assertRFC3339Timestamp(t *testing.T, ts string) {
	t.Helper()
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Fatalf("expected timestamp to be RFC3339, got %q (err: %v)", ts, err)
	}
}
