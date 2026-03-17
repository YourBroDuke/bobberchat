package a2a

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bobberchat/bobberchat/backend/internal/adapter"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/google/uuid"
)

func TestA2AAdapterName(t *testing.T) {
	a := NewA2AAdapter()

	if got := a.Name(); got != "a2a" {
		t.Fatalf("Name() = %q, want %q", got, "a2a")
	}
}

func TestA2AAdapterProtocol(t *testing.T) {
	a := NewA2AAdapter()

	if got := a.Protocol(); got != "a2a-json" {
		t.Fatalf("Protocol() = %q, want %q", got, "a2a-json")
	}
}

func TestA2AAdapterValidate(t *testing.T) {
	a := NewA2AAdapter()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid message send",
			raw:  `{"method":"message/send","id":"msg-1","params":{"message":{"role":"user","parts":[{"type":"text","text":"run deployment"}]}}}`,
		},
		{
			name: "valid agent card",
			raw:  `{"method":"agent/card","id":"card-1","params":{"name":"helper","description":"agent","capabilities":["search"],"endpoint":"https://example"}}`,
		},
		{
			name: "valid task create",
			raw:  `{"method":"task/create","id":"task-1","params":{"taskId":"t-1","status":"created"}}`,
		},
		{
			name: "valid task update",
			raw:  `{"method":"task/update","id":"task-2","params":{"taskId":"t-1","status":"completed","result":{"ok":true}}}`,
		},
		{
			name:    "invalid json",
			raw:     `{not-json`,
			wantErr: "invalid json",
		},
		{
			name:    "missing method",
			raw:     `{"id":"m1","params":{}}`,
			wantErr: "method field is required",
		},
		{
			name:    "unsupported method",
			raw:     `{"method":"foo/bar","id":"m1","params":{}}`,
			wantErr: "unsupported a2a method",
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

func TestA2AAdapterIngestMessageSendAction(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-1","params":{"message":{"role":"user","parts":[{"type":"text","text":"please run deployment"}],"messageId":"m-11","taskId":"task-11","contextId":"ctx-11"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagRequestAction {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagRequestAction)
	}
	if env.From != "a2a:conn-1" {
		t.Fatalf("env.From = %q, want %q", env.From, "a2a:conn-1")
	}
	if env.To != "tenant-1" {
		t.Fatalf("env.To = %q, want %q", env.To, "tenant-1")
	}
	if got := env.Payload["message"]; got != "please run deployment" {
		t.Fatalf("env.Payload[message] = %v, want %q", got, "please run deployment")
	}
	if got := env.Payload["message_id"]; got != "m-11" {
		t.Fatalf("env.Payload[message_id] = %v, want %q", got, "m-11")
	}
	if got := env.Payload["task_id"]; got != "task-11" {
		t.Fatalf("env.Payload[task_id] = %v, want %q", got, "task-11")
	}
	if got := env.Payload["context_id"]; got != "ctx-11" {
		t.Fatalf("env.Payload[context_id] = %v, want %q", got, "ctx-11")
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

	if got := metaMap[adapter.MetaKeyAdapterName]; got != "a2a" {
		t.Fatalf("adapter metadata name = %v, want %q", got, "a2a")
	}
	if got := metaMap[adapter.MetaKeyAdapterVersion]; got != "1.0.0" {
		t.Fatalf("adapter metadata version = %v, want %q", got, "1.0.0")
	}
	if got := metaMap[adapter.MetaKeyDirection]; got != adapter.DirectionInbound {
		t.Fatalf("adapter metadata direction = %v, want %q", got, adapter.DirectionInbound)
	}
	if got := metaMap[adapter.MetaKeySourceID]; got != "msg-1" {
		t.Fatalf("adapter metadata source_id = %v, want %q", got, "msg-1")
	}
	if got := metaMap[adapter.MetaKeySourceProtocol]; got != "a2a-json" {
		t.Fatalf("adapter metadata source_protocol = %v, want %q", got, "a2a-json")
	}
}

func TestA2AAdapterIngestMessageSendDataIntent(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-2","params":{"message":{"role":"user","parts":[{"type":"text","text":"fetch usage data report"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-2"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagRequestData {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagRequestData)
	}
}

func TestA2AAdapterIngestMessageSendApprovalIntent(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-3","params":{"message":{"role":"user","parts":[{"type":"text","text":"please approve this deploy"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-3"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagRequestApproval {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagRequestApproval)
	}
}

func TestA2AAdapterIngestAgentCard(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"agent/card","id":"card-1","params":{"name":"alpha-agent","description":"handles retrieval","capabilities":["Search","Summarize"],"endpoint":"https://agents.example/alpha"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-4"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagContextProvide {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagContextProvide)
	}
	if got := env.Payload["name"]; got != "alpha-agent" {
		t.Fatalf("env.Payload[name] = %v, want %q", got, "alpha-agent")
	}
	if got := env.Payload["description"]; got != "handles retrieval" {
		t.Fatalf("env.Payload[description] = %v, want %q", got, "handles retrieval")
	}
	if got := env.Payload["endpoint"]; got != "https://agents.example/alpha" {
		t.Fatalf("env.Payload[endpoint] = %v, want %q", got, "https://agents.example/alpha")
	}

	capabilities, ok := env.Payload["capabilities"].([]string)
	if !ok {
		t.Fatalf("env.Payload[capabilities] type = %T, want []string", env.Payload["capabilities"])
	}
	if len(capabilities) != 2 {
		t.Fatalf("len(capabilities) = %d, want 2", len(capabilities))
	}
	if capabilities[0] != "search" || capabilities[1] != "summarize" {
		t.Fatalf("capabilities = %v, want [search summarize]", capabilities)
	}
}

func TestA2AAdapterIngestTaskCreate(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/create","id":"task-create-1","params":{"taskId":"task-100","status":"created","result":{"scope":"full"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-5"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagRequestAction {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagRequestAction)
	}
	if got := env.Payload["action"]; got != "task.create" {
		t.Fatalf("env.Payload[action] = %v, want %q", got, "task.create")
	}
	if got := env.Payload["task_id"]; got != "task-100" {
		t.Fatalf("env.Payload[task_id] = %v, want %q", got, "task-100")
	}
	if got := env.Payload["status"]; got != "created" {
		t.Fatalf("env.Payload[status] = %v, want %q", got, "created")
	}
	if _, ok := env.Payload["result"]; !ok {
		t.Fatalf("env.Payload[result] missing")
	}
}

func TestA2AAdapterIngestTaskUpdateCompleted(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/update","id":"task-up-1","params":{"taskId":"task-200","status":"completed","result":{"output":"done"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-6"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagResponseSuccess {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagResponseSuccess)
	}
	if got := env.Payload["task_id"]; got != "task-200" {
		t.Fatalf("env.Payload[task_id] = %v, want %q", got, "task-200")
	}
	if got := env.Payload["status"]; got != "completed" {
		t.Fatalf("env.Payload[status] = %v, want %q", got, "completed")
	}
}

func TestA2AAdapterIngestTaskUpdateFailed(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/update","id":"task-up-2","params":{"taskId":"task-201","status":"failed","result":{"code":"E42","message":"boom"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-7"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagResponseError {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagResponseError)
	}
	if got := env.Payload["task_id"]; got != "task-201" {
		t.Fatalf("env.Payload[task_id] = %v, want %q", got, "task-201")
	}
	if got := env.Payload["code"]; got != "E42" {
		t.Fatalf("env.Payload[code] = %v, want %q", got, "E42")
	}
	if got := env.Payload["message"]; got != "boom" {
		t.Fatalf("env.Payload[message] = %v, want %q", got, "boom")
	}
}

func TestA2AAdapterIngestTaskUpdateInProgress(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/update","id":"task-up-3","params":{"taskId":"task-202","status":"in_progress","result":{"percent":50}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-8"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != protocol.TagProgressUpdate {
		t.Fatalf("env.Tag = %q, want %q", env.Tag, protocol.TagProgressUpdate)
	}
	if got := env.Payload["task_id"]; got != "task-202" {
		t.Fatalf("env.Payload[task_id] = %v, want %q", got, "task-202")
	}
}

func TestA2AAdapterIngestUsesProvidedAgentID(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-10","params":{"message":{"role":"user","parts":[{"type":"text","text":"run action"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-9", AgentID: "agent-123"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.From != "agent-123" {
		t.Fatalf("env.From = %q, want %q", env.From, "agent-123")
	}
}

func TestA2AAdapterIngestFallbackUnknownConnection(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-11","params":{"message":{"role":"user","parts":[{"type":"text","text":"run action"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.From != "a2a:unknown" {
		t.Fatalf("env.From = %q, want %q", env.From, "a2a:unknown")
	}
}

func TestA2AAdapterIngestUsesTargetAgentHeader(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-12","params":{"message":{"role":"user","parts":[{"type":"text","text":"run action"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{
		ConnectionID: "conn-10",
		Headers:      map[string]string{"X-Target-Agent": "agent-target"},
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.To != "agent-target" {
		t.Fatalf("env.To = %q, want %q", env.To, "agent-target")
	}
}

func TestA2AAdapterIngestFallbackBroadcast(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-13","params":{"message":{"role":"user","parts":[{"type":"text","text":"run action"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-11"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.To != "broadcast" {
		t.Fatalf("env.To = %q, want %q", env.To, "broadcast")
	}
}

func TestA2AAdapterEmitRequestTag(t *testing.T) {
	a := NewA2AAdapter()
	env := &protocol.Envelope{
		ID:      "env-1",
		Tag:     protocol.TagRequestAction,
		Payload: map[string]any{"request_id": "req-1", "message": "execute workflow", "task_id": "task-1", "context_id": "ctx-1"},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg a2aMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Method != "message/send" {
		t.Fatalf("msg.Method = %q, want %q", msg.Method, "message/send")
	}
	if msg.ID != "req-1" {
		t.Fatalf("msg.ID = %v, want %q", msg.ID, "req-1")
	}

	message, ok := msg.Params["message"].(map[string]any)
	if !ok {
		t.Fatalf("msg.Params[message] type = %T, want map[string]any", msg.Params["message"])
	}
	if got := message["role"]; got != "user" {
		t.Fatalf("message[role] = %v, want %q", got, "user")
	}
	if got := message["taskId"]; got != "task-1" {
		t.Fatalf("message[taskId] = %v, want %q", got, "task-1")
	}
	if got := message["contextId"]; got != "ctx-1" {
		t.Fatalf("message[contextId] = %v, want %q", got, "ctx-1")
	}
}

func TestA2AAdapterEmitResponseSuccess(t *testing.T) {
	a := NewA2AAdapter()
	env := &protocol.Envelope{
		ID:  "env-2",
		Tag: protocol.TagResponseSuccess,
		Payload: map[string]any{
			"request_id": "req-22",
			"task_id":    "task-22",
			"result":     map[string]any{"output": "ok"},
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg a2aMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Method != "task/update" {
		t.Fatalf("msg.Method = %q, want %q", msg.Method, "task/update")
	}
	if msg.ID != "req-22" {
		t.Fatalf("msg.ID = %v, want %q", msg.ID, "req-22")
	}
	if got := msg.Params["status"]; got != "completed" {
		t.Fatalf("msg.Params[status] = %v, want %q", got, "completed")
	}
	if got := msg.Params["taskId"]; got != "task-22" {
		t.Fatalf("msg.Params[taskId] = %v, want %q", got, "task-22")
	}
	if _, ok := msg.Params["result"]; !ok {
		t.Fatalf("msg.Params[result] missing")
	}
}

func TestA2AAdapterEmitResponseError(t *testing.T) {
	a := NewA2AAdapter()
	env := &protocol.Envelope{
		ID:  "env-3",
		Tag: protocol.TagResponseError,
		Payload: map[string]any{
			"request_id": "req-33",
			"task_id":    "task-33",
			"code":       "E500",
			"message":    "failure",
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg a2aMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Method != "task/update" {
		t.Fatalf("msg.Method = %q, want %q", msg.Method, "task/update")
	}
	if got := msg.Params["status"]; got != "failed" {
		t.Fatalf("msg.Params[status] = %v, want %q", got, "failed")
	}
	if got := msg.Params["taskId"]; got != "task-33" {
		t.Fatalf("msg.Params[taskId] = %v, want %q", got, "task-33")
	}
	result, ok := msg.Params["result"].(map[string]any)
	if !ok {
		t.Fatalf("msg.Params[result] type = %T, want map[string]any", msg.Params["result"])
	}
	if got := result["code"]; got != "E500" {
		t.Fatalf("result[code] = %v, want %q", got, "E500")
	}
	if got := result["message"]; got != "failure" {
		t.Fatalf("result[message] = %v, want %q", got, "failure")
	}
}

func TestA2AAdapterEmitProgressTag(t *testing.T) {
	a := NewA2AAdapter()
	env := &protocol.Envelope{
		ID:  "env-4",
		Tag: protocol.TagProgressUpdate,
		Payload: map[string]any{
			"request_id": "req-44",
			"task_id":    "task-44",
			"progress":   map[string]any{"percent": 75},
		},
	}

	out, err := a.Emit(context.Background(), env)
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}

	var msg a2aMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if msg.Method != "task/update" {
		t.Fatalf("msg.Method = %q, want %q", msg.Method, "task/update")
	}
	if got := msg.Params["status"]; got != "in_progress" {
		t.Fatalf("msg.Params[status] = %v, want %q", got, "in_progress")
	}
	if got := msg.Params["taskId"]; got != "task-44" {
		t.Fatalf("msg.Params[taskId] = %v, want %q", got, "task-44")
	}
}

func TestA2AAdapterEmitUnsupportedTag(t *testing.T) {
	a := NewA2AAdapter()
	env := &protocol.Envelope{Tag: protocol.TagSystemHeartbeat, Payload: map[string]any{}}

	_, err := a.Emit(context.Background(), env)
	if err == nil {
		t.Fatalf("Emit() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported envelope tag") {
		t.Fatalf("Emit() error = %q, want contains %q", err.Error(), "unsupported envelope tag")
	}
}

func TestA2AAdapterEmitContextProvideUnsupported(t *testing.T) {
	a := NewA2AAdapter()
	env := &protocol.Envelope{Tag: protocol.TagContextProvide, Payload: map[string]any{"name": "agent"}}

	_, err := a.Emit(context.Background(), env)
	if err == nil {
		t.Fatalf("Emit() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "context-provide") {
		t.Fatalf("Emit() error = %q, want contains %q", err.Error(), "context-provide")
	}
}
