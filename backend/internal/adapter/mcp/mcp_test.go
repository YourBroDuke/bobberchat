package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bobberchat/bobberchat/backend/internal/adapter"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/google/uuid"
)

func TestMCPAdapterName(t *testing.T) {
	a := NewMCPAdapter()

	if got := a.Name(); got != "mcp" {
		t.Fatalf("Name() = %q, want %q", got, "mcp")
	}
}

func TestMCPAdapterProtocol(t *testing.T) {
	a := NewMCPAdapter()

	if got := a.Protocol(); got != "json-rpc" {
		t.Fatalf("Protocol() = %q, want %q", got, "json-rpc")
	}
}

func TestMCPAdapterValidate(t *testing.T) {
	a := NewMCPAdapter()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid tool call",
			raw:  `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"test"}}}`,
		},
		{
			name: "valid result",
			raw:  `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}`,
		},
		{
			name: "valid notification",
			raw:  `{"jsonrpc":"2.0","method":"notifications/resources/updated","params":{"uri":"file:///data.csv"}}`,
		},
		{
			name:    "invalid json",
			raw:     `{not-json`,
			wantErr: "invalid json",
		},
		{
			name:    "missing jsonrpc field",
			raw:     `{"id":1,"method":"tools/call","params":{"name":"search"}}`,
			wantErr: "jsonrpc field is required",
		},
		{
			name:    "invalid jsonrpc version",
			raw:     `{"jsonrpc":"1.0","id":1,"method":"tools/call","params":{"name":"search"}}`,
			wantErr: "invalid jsonrpc version",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := a.Validate([]byte(tt.raw))

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

func TestMCPAdapterIngestToolCall(t *testing.T) {
	a := NewMCPAdapter()
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"test"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{
		ConnectionID: "conn-1",
		TenantID:     "tenant-1",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagRequestAction {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagRequestAction)
	}
	if env.From != "mcp:conn-1" {
		t.Fatalf("env.From = %q, want %q", env.From, "mcp:conn-1")
	}
	if env.To != "tenant-1" {
		t.Fatalf("env.To = %q, want %q", env.To, "tenant-1")
	}
	if got := env.Payload["action"]; got != "search" {
		t.Fatalf("env.Payload[action] = %v, want %q", got, "search")
	}

	args, ok := env.Payload["args"].(map[string]any)
	if !ok {
		t.Fatalf("env.Payload[args] type = %T, want map[string]any", env.Payload["args"])
	}
	if got := args["query"]; got != "test" {
		t.Fatalf("env.Payload[args][query] = %v, want %q", got, "test")
	}

	if _, err := uuid.Parse(env.ID); err != nil {
		t.Fatalf("env.ID = %q is not valid UUID: %v", env.ID, err)
	}
	if _, err := uuid.Parse(env.TraceID); err != nil {
		t.Fatalf("env.TraceID = %q is not valid UUID: %v", env.TraceID, err)
	}

	metaRaw, ok := env.Metadata[adapter.MetaKeyAdapter]
	if !ok {
		t.Fatalf("env.Metadata[%q] missing", adapter.MetaKeyAdapter)
	}
	metaMap, ok := metaRaw.(map[string]any)
	if !ok {
		t.Fatalf("env.Metadata[%q] type = %T, want map[string]any", adapter.MetaKeyAdapter, metaRaw)
	}

	if got := metaMap[adapter.MetaKeyAdapterName]; got != "mcp" {
		t.Fatalf("adapter metadata name = %v, want %q", got, "mcp")
	}
	if got := metaMap[adapter.MetaKeyAdapterVersion]; got != "1.0.0" {
		t.Fatalf("adapter metadata version = %v, want %q", got, "1.0.0")
	}
	if got := metaMap[adapter.MetaKeyDirection]; got != adapter.DirectionInbound {
		t.Fatalf("adapter metadata direction = %v, want %q", got, adapter.DirectionInbound)
	}
	if got := metaMap[adapter.MetaKeySourceID]; got != "1" {
		t.Fatalf("adapter metadata source_id = %v, want %q", got, "1")
	}
	if got := metaMap[adapter.MetaKeySourceProtocol]; got != "json-rpc" {
		t.Fatalf("adapter metadata source_protocol = %v, want %q", got, "json-rpc")
	}
}

func TestMCPAdapterIngestResult(t *testing.T) {
	a := NewMCPAdapter()
	raw := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"found 3 results"}]}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-2", TenantID: "tenant-2"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagResponseSuccess {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagResponseSuccess)
	}
	if got := env.Payload["request_id"]; got != "1" {
		t.Fatalf("env.Payload[request_id] = %v, want %q", got, "1")
	}
	if _, ok := env.Payload["result"]; !ok {
		t.Fatalf("env.Payload[result] missing")
	}
}

func TestMCPAdapterIngestErrorResult(t *testing.T) {
	a := NewMCPAdapter()
	raw := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-3", TenantID: "tenant-3"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagResponseError {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagResponseError)
	}
	if got := env.Payload["code"]; got != "-32600" {
		t.Fatalf("env.Payload[code] = %v, want %q", got, "-32600")
	}
	if got := env.Payload["message"]; got != "Invalid Request" {
		t.Fatalf("env.Payload[message] = %v, want %q", got, "Invalid Request")
	}
}

func TestMCPAdapterIngestNotification(t *testing.T) {
	a := NewMCPAdapter()
	raw := []byte(`{"jsonrpc":"2.0","method":"notifications/resources/updated","params":{"uri":"file:///data.csv"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-4", TenantID: "tenant-4"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagContextProvide {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagContextProvide)
	}
	summary, ok := env.Payload["summary"].(string)
	if !ok {
		t.Fatalf("env.Payload[summary] type = %T, want string", env.Payload["summary"])
	}
	if !strings.Contains(summary, "notifications/resources/updated") {
		t.Fatalf("summary = %q, want contains method", summary)
	}
}

func TestMCPAdapterIngestUsesProvidedAgentID(t *testing.T) {
	a := NewMCPAdapter()
	raw := []byte(`{"jsonrpc":"2.0","id":"req-1","method":"tools/call","params":{"name":"search","arguments":{"query":"test"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{
		ConnectionID: "conn-5",
		TenantID:     "tenant-5",
		AgentID:      "agent-123",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.From != "agent-123" {
		t.Fatalf("env.From = %q, want %q", env.From, "agent-123")
	}
}

func TestMCPAdapterIngestUsesTargetAgentHeader(t *testing.T) {
	a := NewMCPAdapter()
	raw := []byte(`{"jsonrpc":"2.0","id":"req-2","method":"tools/call","params":{"name":"search","arguments":{"query":"test"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{
		ConnectionID: "conn-6",
		TenantID:     "tenant-6",
		Headers: map[string]string{
			"X-Target-Agent": "agent-target",
		},
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.To != "agent-target" {
		t.Fatalf("env.To = %q, want %q", env.To, "agent-target")
	}
}

func TestMCPAdapterEmitRequestAction(t *testing.T) {
	a := NewMCPAdapter()
	env := &protocol.Envelope{
		ID:      "env-1",
		Tag:     protocol.TagRequestAction,
		Payload: map[string]any{"action": "search", "args": map[string]any{"query": "test"}},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg jsonRPCMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.JSONRPC != "2.0" {
		t.Fatalf("msg.JSONRPC = %q, want %q", msg.JSONRPC, "2.0")
	}
	if msg.Method != "tools/call" {
		t.Fatalf("msg.Method = %q, want %q", msg.Method, "tools/call")
	}
	if got := msg.Params["name"]; got != "search" {
		t.Fatalf("msg.Params[name] = %v, want %q", got, "search")
	}
}

func TestMCPAdapterEmitResponseSuccess(t *testing.T) {
	a := NewMCPAdapter()
	env := &protocol.Envelope{
		ID:  "env-2",
		Tag: protocol.TagResponseSuccess,
		Payload: map[string]any{
			"request_id": "req-22",
			"result": map[string]any{
				"content": []any{map[string]any{"type": "text", "text": "ok"}},
			},
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg jsonRPCMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.ID != "req-22" {
		t.Fatalf("msg.ID = %v, want %q", msg.ID, "req-22")
	}
	if msg.Result == nil {
		t.Fatalf("msg.Result is nil, want value")
	}
}

func TestMCPAdapterEmitResponseError(t *testing.T) {
	a := NewMCPAdapter()
	env := &protocol.Envelope{
		ID:  "env-3",
		Tag: protocol.TagResponseError,
		Payload: map[string]any{
			"request_id": "req-33",
			"code":       "-32600",
			"message":    "Invalid Request",
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg jsonRPCMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.ID != "req-33" {
		t.Fatalf("msg.ID = %v, want %q", msg.ID, "req-33")
	}
	if msg.Error == nil {
		t.Fatalf("msg.Error is nil, want value")
	}
	if msg.Error.Code != -32600 {
		t.Fatalf("msg.Error.Code = %d, want %d", msg.Error.Code, -32600)
	}
	if msg.Error.Message != "Invalid Request" {
		t.Fatalf("msg.Error.Message = %q, want %q", msg.Error.Message, "Invalid Request")
	}
}

func TestMCPAdapterEmitUnsupportedTag(t *testing.T) {
	a := NewMCPAdapter()
	env := &protocol.Envelope{Tag: protocol.TagProgressUpdate, Payload: map[string]any{}}

	_, err := a.Emit(context.Background(), env)
	if err == nil {
		t.Fatalf("Emit() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported envelope tag") {
		t.Fatalf("Emit() error = %q, want contains %q", err.Error(), "unsupported envelope tag")
	}
}
