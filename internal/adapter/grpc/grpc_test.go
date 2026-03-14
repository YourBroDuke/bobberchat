package grpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bobberchat/bobberchat/internal/adapter"
	"github.com/bobberchat/bobberchat/internal/protocol"
	"github.com/google/uuid"
)

func TestGRPCAdapterName(t *testing.T) {
	a := NewGRPCAdapter()

	if got := a.Name(); got != "grpc" {
		t.Fatalf("Name() = %q, want %q", got, "grpc")
	}
}

func TestGRPCAdapterProtocol(t *testing.T) {
	a := NewGRPCAdapter()

	if got := a.Protocol(); got != "grpc-json" {
		t.Fatalf("Protocol() = %q, want %q", got, "grpc-json")
	}
}

func TestGRPCAdapterValidate(t *testing.T) {
	a := NewGRPCAdapter()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid unary request",
			raw:  `{"type":"unary","service":"agent.AgentService","method":"Execute","request_id":"rpc-1","body":{"action":"compile"}}`,
		},
		{
			name: "valid unary ok response",
			raw:  `{"type":"unary","request_id":"rpc-2","status":"OK","body":{"output":"done"}}`,
		},
		{
			name: "valid unary error response",
			raw:  `{"type":"unary","request_id":"rpc-3","status":"ERROR","code":3,"message":"invalid argument"}`,
		},
		{
			name: "valid stream frame",
			raw:  `{"type":"stream","request_id":"rpc-4","stream_id":"stream-1","body":{"progress":42}}`,
		},
		{
			name:    "empty raw",
			raw:     "",
			wantErr: "raw message is empty",
		},
		{
			name:    "invalid json",
			raw:     `{not-json`,
			wantErr: "invalid json",
		},
		{
			name:    "missing type",
			raw:     `{"service":"agent.AgentService","method":"Execute"}`,
			wantErr: "type field is required",
		},
		{
			name:    "invalid type",
			raw:     `{"type":"event"}`,
			wantErr: "invalid type",
		},
		{
			name:    "unary missing service method and status",
			raw:     `{"type":"unary","request_id":"rpc-5"}`,
			wantErr: "unary message must include service+method or status",
		},
		{
			name:    "unary with only service",
			raw:     `{"type":"unary","service":"agent.AgentService"}`,
			wantErr: "unary call must include both service and method",
		},
		{
			name:    "unary invalid status",
			raw:     `{"type":"unary","status":"PENDING"}`,
			wantErr: "invalid unary status",
		},
		{
			name:    "stream missing stream id",
			raw:     `{"type":"stream","request_id":"rpc-6"}`,
			wantErr: "stream_id field is required",
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

func TestGRPCAdapterIngestUnaryRequest(t *testing.T) {
	a := NewGRPCAdapter()
	raw := []byte(`{"type":"unary","service":"agent.AgentService","method":"Execute","request_id":"rpc-123","body":{"action":"compile","args":{"target":"main.go"}}}`)

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
	if env.From != "grpc:conn-1" {
		t.Fatalf("env.From = %q, want %q", env.From, "grpc:conn-1")
	}
	if env.To != "tenant-1" {
		t.Fatalf("env.To = %q, want %q", env.To, "tenant-1")
	}
	if got := env.Payload["action"]; got != "agent.AgentService/Execute" {
		t.Fatalf("env.Payload[action] = %v, want %q", got, "agent.AgentService/Execute")
	}

	args, ok := env.Payload["args"].(map[string]any)
	if !ok {
		t.Fatalf("env.Payload[args] type = %T, want map[string]any", env.Payload["args"])
	}
	if got := args["action"]; got != "compile" {
		t.Fatalf("env.Payload[args][action] = %v, want %q", got, "compile")
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

	if got := metaMap[adapter.MetaKeyAdapterName]; got != "grpc" {
		t.Fatalf("adapter metadata name = %v, want %q", got, "grpc")
	}
	if got := metaMap[adapter.MetaKeyAdapterVersion]; got != "1.0.0" {
		t.Fatalf("adapter metadata version = %v, want %q", got, "1.0.0")
	}
	if got := metaMap[adapter.MetaKeyDirection]; got != adapter.DirectionInbound {
		t.Fatalf("adapter metadata direction = %v, want %q", got, adapter.DirectionInbound)
	}
	if got := metaMap[adapter.MetaKeySourceID]; got != "rpc-123" {
		t.Fatalf("adapter metadata source_id = %v, want %q", got, "rpc-123")
	}
	if got := metaMap[adapter.MetaKeySourceProtocol]; got != "grpc-json" {
		t.Fatalf("adapter metadata source_protocol = %v, want %q", got, "grpc-json")
	}
}

func TestGRPCAdapterIngestUnarySuccess(t *testing.T) {
	a := NewGRPCAdapter()
	raw := []byte(`{"type":"unary","request_id":"rpc-777","status":"OK","body":{"output":"compiled successfully"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{
		ConnectionID: "conn-2",
		TenantID:     "tenant-2",
		AgentID:      "agent-123",
		Headers: map[string]string{
			"X-Target-Agent": "agent-target",
		},
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagResponseSuccess {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagResponseSuccess)
	}
	if env.From != "agent-123" {
		t.Fatalf("env.From = %q, want %q", env.From, "agent-123")
	}
	if env.To != "agent-target" {
		t.Fatalf("env.To = %q, want %q", env.To, "agent-target")
	}
	if got := env.Payload["request_id"]; got != "rpc-777" {
		t.Fatalf("env.Payload[request_id] = %v, want %q", got, "rpc-777")
	}

	result, ok := env.Payload["result"].(map[string]any)
	if !ok {
		t.Fatalf("env.Payload[result] type = %T, want map[string]any", env.Payload["result"])
	}
	if got := result["output"]; got != "compiled successfully" {
		t.Fatalf("env.Payload[result][output] = %v, want %q", got, "compiled successfully")
	}
}

func TestGRPCAdapterIngestUnaryError(t *testing.T) {
	a := NewGRPCAdapter()
	raw := []byte(`{"type":"unary","request_id":"rpc-888","status":"ERROR","code":3,"message":"Invalid argument: missing target"}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-3"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagResponseError {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagResponseError)
	}
	if env.To != "broadcast" {
		t.Fatalf("env.To = %q, want %q", env.To, "broadcast")
	}
	if got := env.Payload["code"]; got != "3" {
		t.Fatalf("env.Payload[code] = %v, want %q", got, "3")
	}
	if got := env.Payload["message"]; got != "Invalid argument: missing target" {
		t.Fatalf("env.Payload[message] = %v, want %q", got, "Invalid argument: missing target")
	}
	if got := env.Payload["request_id"]; got != "rpc-888" {
		t.Fatalf("env.Payload[request_id] = %v, want %q", got, "rpc-888")
	}
}

func TestGRPCAdapterIngestStreamProgress(t *testing.T) {
	a := NewGRPCAdapter()
	raw := []byte(`{"type":"stream","request_id":"rpc-456","stream_id":"stream-1","body":{"progress":42,"stage":"compiling","message":"Processing file 42/100"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-4", TenantID: "tenant-4"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagProgressUpdate {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagProgressUpdate)
	}
	if got := env.Payload["request_id"]; got != "rpc-456" {
		t.Fatalf("env.Payload[request_id] = %v, want %q", got, "rpc-456")
	}
	if got := env.Payload["stream_id"]; got != "stream-1" {
		t.Fatalf("env.Payload[stream_id] = %v, want %q", got, "stream-1")
	}

	update, ok := env.Payload["update"].(map[string]any)
	if !ok {
		t.Fatalf("env.Payload[update] type = %T, want map[string]any", env.Payload["update"])
	}
	if got := update["stage"]; got != "compiling" {
		t.Fatalf("env.Payload[update][stage] = %v, want %q", got, "compiling")
	}

	pct, ok := env.Payload["percentage"].(float64)
	if !ok {
		t.Fatalf("env.Payload[percentage] type = %T, want float64", env.Payload["percentage"])
	}
	if pct != 42 {
		t.Fatalf("env.Payload[percentage] = %v, want %v", pct, 42)
	}
}

func TestGRPCAdapterIngestStreamNonNumericProgress(t *testing.T) {
	a := NewGRPCAdapter()
	raw := []byte(`{"type":"stream","stream_id":"stream-9","body":{"progress":"halfway","stage":"compiling"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-9", TenantID: "tenant-9"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if _, ok := env.Payload["percentage"]; ok {
		t.Fatalf("env.Payload[percentage] present, want absent")
	}
}

func TestGRPCAdapterEmitRequestAction(t *testing.T) {
	a := NewGRPCAdapter()
	env := &protocol.Envelope{
		ID:  "env-1",
		Tag: protocol.TagRequestAction,
		Payload: map[string]any{
			"request_id": "rpc-req-1",
			"action":     "agent.AgentService/Execute",
			"args":       map[string]any{"action": "compile", "args": map[string]any{"target": "main.go"}},
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg grpcMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Type != "unary" {
		t.Fatalf("msg.Type = %q, want %q", msg.Type, "unary")
	}
	if msg.Service != "agent.AgentService" {
		t.Fatalf("msg.Service = %q, want %q", msg.Service, "agent.AgentService")
	}
	if msg.Method != "Execute" {
		t.Fatalf("msg.Method = %q, want %q", msg.Method, "Execute")
	}
	if msg.RequestID != "rpc-req-1" {
		t.Fatalf("msg.RequestID = %q, want %q", msg.RequestID, "rpc-req-1")
	}
	if got := msg.Body["action"]; got != "compile" {
		t.Fatalf("msg.Body[action] = %v, want %q", got, "compile")
	}
}

func TestGRPCAdapterEmitResponseSuccess(t *testing.T) {
	a := NewGRPCAdapter()
	env := &protocol.Envelope{
		ID:  "env-2",
		Tag: protocol.TagResponseSuccess,
		Payload: map[string]any{
			"request_id": "rpc-ok-2",
			"result": map[string]any{
				"output": "compiled successfully",
			},
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg grpcMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Type != "unary" {
		t.Fatalf("msg.Type = %q, want %q", msg.Type, "unary")
	}
	if msg.Status != "OK" {
		t.Fatalf("msg.Status = %q, want %q", msg.Status, "OK")
	}
	if msg.RequestID != "rpc-ok-2" {
		t.Fatalf("msg.RequestID = %q, want %q", msg.RequestID, "rpc-ok-2")
	}
	if got := msg.Body["output"]; got != "compiled successfully" {
		t.Fatalf("msg.Body[output] = %v, want %q", got, "compiled successfully")
	}
}

func TestGRPCAdapterEmitResponseError(t *testing.T) {
	a := NewGRPCAdapter()
	env := &protocol.Envelope{
		ID:  "env-3",
		Tag: protocol.TagResponseError,
		Payload: map[string]any{
			"request_id": "rpc-err-3",
			"code":       "7",
			"message":    "permission denied",
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg grpcMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Type != "unary" {
		t.Fatalf("msg.Type = %q, want %q", msg.Type, "unary")
	}
	if msg.Status != "ERROR" {
		t.Fatalf("msg.Status = %q, want %q", msg.Status, "ERROR")
	}
	if msg.RequestID != "rpc-err-3" {
		t.Fatalf("msg.RequestID = %q, want %q", msg.RequestID, "rpc-err-3")
	}
	if msg.Code != 7 {
		t.Fatalf("msg.Code = %d, want %d", msg.Code, 7)
	}
	if msg.Message != "permission denied" {
		t.Fatalf("msg.Message = %q, want %q", msg.Message, "permission denied")
	}
}

func TestGRPCAdapterEmitProgressUpdate(t *testing.T) {
	a := NewGRPCAdapter()
	env := &protocol.Envelope{
		ID:  "env-4",
		Tag: protocol.TagProgressUpdate,
		Payload: map[string]any{
			"request_id": "rpc-prog-4",
			"stream_id":  "stream-4",
			"update": map[string]any{
				"progress": 42,
				"stage":    "compiling",
			},
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg grpcMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Type != "stream" {
		t.Fatalf("msg.Type = %q, want %q", msg.Type, "stream")
	}
	if msg.RequestID != "rpc-prog-4" {
		t.Fatalf("msg.RequestID = %q, want %q", msg.RequestID, "rpc-prog-4")
	}
	if msg.StreamID != "stream-4" {
		t.Fatalf("msg.StreamID = %q, want %q", msg.StreamID, "stream-4")
	}
	if got := msg.Body["stage"]; got != "compiling" {
		t.Fatalf("msg.Body[stage] = %v, want %q", got, "compiling")
	}
}

func TestGRPCAdapterEmitProgressFamilyFromGenericPayload(t *testing.T) {
	a := NewGRPCAdapter()
	env := &protocol.Envelope{
		ID:  "env-5",
		Tag: protocol.TagProgressStage,
		Payload: map[string]any{
			"request_id": "rpc-stage-5",
			"stage":      "compiling",
			"message":    "processing",
			"percentage": 73,
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg grpcMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Type != "stream" {
		t.Fatalf("msg.Type = %q, want %q", msg.Type, "stream")
	}
	if msg.RequestID != "rpc-stage-5" {
		t.Fatalf("msg.RequestID = %q, want %q", msg.RequestID, "rpc-stage-5")
	}
	if msg.StreamID != "env-5" {
		t.Fatalf("msg.StreamID = %q, want %q", msg.StreamID, "env-5")
	}
	if got := msg.Body["stage"]; got != "compiling" {
		t.Fatalf("msg.Body[stage] = %v, want %q", got, "compiling")
	}
	if got := msg.Body["message"]; got != "processing" {
		t.Fatalf("msg.Body[message] = %v, want %q", got, "processing")
	}
	if got := msg.Body["progress"]; got != float64(73) {
		t.Fatalf("msg.Body[progress] = %v, want %v", got, float64(73))
	}
}

func TestGRPCAdapterEmitErrors(t *testing.T) {
	a := NewGRPCAdapter()

	tests := []struct {
		name    string
		env     *protocol.Envelope
		wantErr string
	}{
		{
			name:    "nil envelope",
			env:     nil,
			wantErr: "envelope is nil",
		},
		{
			name:    "unsupported tag",
			env:     &protocol.Envelope{Tag: protocol.TagContextProvide, Payload: map[string]any{}},
			wantErr: "unsupported envelope tag",
		},
		{
			name:    "request action missing action",
			env:     &protocol.Envelope{ID: "env-e1", Tag: protocol.TagRequestAction, Payload: map[string]any{}},
			wantErr: "payload.action is required",
		},
		{
			name:    "request action invalid action format",
			env:     &protocol.Envelope{ID: "env-e2", Tag: protocol.TagRequestAction, Payload: map[string]any{"action": "Execute"}},
			wantErr: "service/method format",
		},
		{
			name:    "request action args not object",
			env:     &protocol.Envelope{ID: "env-e3", Tag: protocol.TagRequestAction, Payload: map[string]any{"action": "svc/method", "args": "bad"}},
			wantErr: "payload.args must be an object",
		},
		{
			name:    "response success missing result",
			env:     &protocol.Envelope{ID: "env-e4", Tag: protocol.TagResponseSuccess, Payload: map[string]any{}},
			wantErr: "payload.result is required",
		},
		{
			name:    "response success result not object",
			env:     &protocol.Envelope{ID: "env-e5", Tag: protocol.TagResponseSuccess, Payload: map[string]any{"result": "ok"}},
			wantErr: "payload.result must be an object",
		},
		{
			name:    "response error missing code",
			env:     &protocol.Envelope{ID: "env-e6", Tag: protocol.TagResponseError, Payload: map[string]any{"message": "bad"}},
			wantErr: "payload.code is required",
		},
		{
			name:    "response error non numeric code",
			env:     &protocol.Envelope{ID: "env-e7", Tag: protocol.TagResponseError, Payload: map[string]any{"code": "abc", "message": "bad"}},
			wantErr: "payload.code must be numeric",
		},
		{
			name:    "response error missing message",
			env:     &protocol.Envelope{ID: "env-e8", Tag: protocol.TagResponseError, Payload: map[string]any{"code": "3"}},
			wantErr: "payload.message is required",
		},
		{
			name:    "response error blank message",
			env:     &protocol.Envelope{ID: "env-e9", Tag: protocol.TagResponseError, Payload: map[string]any{"code": "3", "message": ""}},
			wantErr: "payload.message must be a non-empty string",
		},
		{
			name:    "progress update not object",
			env:     &protocol.Envelope{ID: "env-e10", Tag: protocol.TagProgressUpdate, Payload: map[string]any{"update": "bad"}},
			wantErr: "progress payload.update must be an object",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := a.Emit(context.Background(), tt.env)
			if err == nil {
				t.Fatalf("Emit() error = nil, want contains %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Emit() error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}
