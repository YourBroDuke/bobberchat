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

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagRequestAction {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagRequestAction)
	}
	if env.From != "a2a:conn-1" {
		t.Fatalf("env.From = %q, want %q", env.From, "a2a:conn-1")
	}
	if env.To != "broadcast" {
		t.Fatalf("env.To = %q, want %q", env.To, "broadcast")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(env.Content), &payload); err != nil {
		t.Fatalf("parse env.Content: %v", err)
	}
	if got := payload["message"]; got != "please run deployment" {
		t.Fatalf("payload[message] = %v, want %q", got, "please run deployment")
	}
	if got := payload["message_id"]; got != "m-11" {
		t.Fatalf("payload[message_id] = %v, want %q", got, "m-11")
	}
	if got := payload["task_id"]; got != "task-11" {
		t.Fatalf("payload[task_id] = %v, want %q", got, "task-11")
	}
	if got := payload["context_id"]; got != "ctx-11" {
		t.Fatalf("payload[context_id] = %v, want %q", got, "ctx-11")
	}

	if _, err := uuid.Parse(env.ID); err != nil {
		t.Fatalf("env.ID = %q is not valid UUID: %v", env.ID, err)
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

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagRequestData {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagRequestData)
	}
}

func TestA2AAdapterIngestMessageSendApprovalIntent(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"message/send","id":"msg-3","params":{"message":{"role":"user","parts":[{"type":"text","text":"please approve this deploy"}]}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-3"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagRequestApproval {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagRequestApproval)
	}
}

func TestA2AAdapterIngestAgentCard(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"agent/card","id":"card-1","params":{"name":"alpha-agent","description":"handles retrieval","capabilities":["Search","Summarize"],"endpoint":"https://agents.example/alpha"}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-4"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagContextProvide {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagContextProvide)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(env.Content), &payload); err != nil {
		t.Fatalf("parse env.Content: %v", err)
	}
	if got := payload["name"]; got != "alpha-agent" {
		t.Fatalf("payload[name] = %v, want %q", got, "alpha-agent")
	}
	if got := payload["description"]; got != "handles retrieval" {
		t.Fatalf("payload[description] = %v, want %q", got, "handles retrieval")
	}
	if got := payload["endpoint"]; got != "https://agents.example/alpha" {
		t.Fatalf("payload[endpoint] = %v, want %q", got, "https://agents.example/alpha")
	}

	capabilities, ok := payload["capabilities"].([]any)
	if !ok {
		t.Fatalf("payload[capabilities] type = %T, want []any", payload["capabilities"])
	}
	if len(capabilities) != 2 {
		t.Fatalf("len(capabilities) = %d, want 2", len(capabilities))
	}
	cap0, ok0 := capabilities[0].(string)
	cap1, ok1 := capabilities[1].(string)
	if !ok0 || !ok1 || cap0 != "search" || cap1 != "summarize" {
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

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagRequestAction {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagRequestAction)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysAction); got != "task.create" {
		t.Fatalf("system meta action = %q, want %q", got, "task.create")
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTaskID); got != "task-100" {
		t.Fatalf("system meta task_id = %q, want %q", got, "task-100")
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysStatus); got != "created" {
		t.Fatalf("system meta status = %q, want %q", got, "created")
	}
	if _, ok := adapter.SystemMeta(env, protocol.MetaSysResult); !ok {
		t.Fatalf("system meta result missing")
	}
	contentPayload := map[string]any{}
	if env.Content != "" {
		if err := json.Unmarshal([]byte(env.Content), &contentPayload); err != nil {
			t.Fatalf("parse env.Content: %v", err)
		}
	}
	if len(contentPayload) != 0 {
		t.Fatalf("env.Content payload = %v, want empty", contentPayload)
	}
}

func TestA2AAdapterIngestTaskUpdateCompleted(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/update","id":"task-up-1","params":{"taskId":"task-200","status":"completed","result":{"output":"done"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-6"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagResponseSuccess {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagResponseSuccess)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTaskID); got != "task-200" {
		t.Fatalf("system meta task_id = %q, want %q", got, "task-200")
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysStatus); got != "completed" {
		t.Fatalf("system meta status = %q, want %q", got, "completed")
	}
	if _, ok := adapter.SystemMeta(env, protocol.MetaSysResult); !ok {
		t.Fatalf("system meta result missing")
	}
	contentPayload := map[string]any{}
	if env.Content != "" {
		if err := json.Unmarshal([]byte(env.Content), &contentPayload); err != nil {
			t.Fatalf("parse env.Content: %v", err)
		}
	}
	if len(contentPayload) != 0 {
		t.Fatalf("env.Content payload = %v, want empty", contentPayload)
	}
}

func TestA2AAdapterIngestTaskUpdateFailed(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/update","id":"task-up-2","params":{"taskId":"task-201","status":"failed","result":{"code":"E42","message":"boom"}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-7"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagResponseError {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagResponseError)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTaskID); got != "task-201" {
		t.Fatalf("system meta task_id = %q, want %q", got, "task-201")
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysCode); got != "E42" {
		t.Fatalf("system meta code = %q, want %q", got, "E42")
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysMessage); got != "boom" {
		t.Fatalf("system meta message = %q, want %q", got, "boom")
	}
	contentPayload := map[string]any{}
	if env.Content != "" {
		if err := json.Unmarshal([]byte(env.Content), &contentPayload); err != nil {
			t.Fatalf("parse env.Content: %v", err)
		}
	}
	if len(contentPayload) != 0 {
		t.Fatalf("env.Content payload = %v, want empty", contentPayload)
	}
}

func TestA2AAdapterIngestTaskUpdateInProgress(t *testing.T) {
	a := NewA2AAdapter()
	raw := []byte(`{"method":"task/update","id":"task-up-3","params":{"taskId":"task-202","status":"in_progress","result":{"percent":50}}}`)

	env, err := a.Ingest(context.Background(), raw, adapter.TransportMeta{ConnectionID: "conn-8"})
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}

	if env.Tag != "" {
		t.Fatalf("env.Tag = %q, want empty", env.Tag)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTag); got != protocol.TagProgressUpdate {
		t.Fatalf("system meta tag = %q, want %q", got, protocol.TagProgressUpdate)
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysTaskID); got != "task-202" {
		t.Fatalf("system meta task_id = %q, want %q", got, "task-202")
	}
	if got := adapter.SystemMetaString(env, protocol.MetaSysStatus); got != "in_progress" {
		t.Fatalf("system meta status = %q, want %q", got, "in_progress")
	}
	if _, ok := adapter.SystemMeta(env, protocol.MetaSysResult); !ok {
		t.Fatalf("system meta result missing")
	}
	contentPayload := map[string]any{}
	if env.Content != "" {
		if err := json.Unmarshal([]byte(env.Content), &contentPayload); err != nil {
			t.Fatalf("parse env.Content: %v", err)
		}
	}
	if len(contentPayload) != 0 {
		t.Fatalf("env.Content payload = %v, want empty", contentPayload)
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
		ID:       "env-1",
		Content:  mustMarshalString(map[string]any{"message": "execute workflow", "context_id": "ctx-1"}),
		Metadata: map[string]any{protocol.MetaSysTag: protocol.TagRequestAction, protocol.MetaSysRequestID: "req-1", protocol.MetaSysTaskID: "task-1"},
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
		ID:       "env-2",
		Content:  mustMarshalString(map[string]any{}),
		Metadata: map[string]any{protocol.MetaSysTag: protocol.TagResponseSuccess, protocol.MetaSysRequestID: "req-22", protocol.MetaSysTaskID: "task-22", protocol.MetaSysResult: map[string]any{"output": "ok"}},
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
		ID:       "env-3",
		Content:  mustMarshalString(map[string]any{}),
		Metadata: map[string]any{protocol.MetaSysTag: protocol.TagResponseError, protocol.MetaSysRequestID: "req-33", protocol.MetaSysTaskID: "task-33", protocol.MetaSysCode: "E500", protocol.MetaSysMessage: "failure"},
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
		ID: "env-4",
		Content: mustMarshalString(map[string]any{
			"progress": map[string]any{"percent": 75},
		}),
		Metadata: map[string]any{protocol.MetaSysTag: protocol.TagProgressUpdate, protocol.MetaSysRequestID: "req-44", protocol.MetaSysTaskID: "task-44"},
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
	env := &protocol.Envelope{Tag: protocol.TagSystemHeartbeat, Content: mustMarshalString(map[string]any{})}

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
	env := &protocol.Envelope{Tag: protocol.TagContextProvide, Content: mustMarshalString(map[string]any{"name": "agent"})}

	_, err := a.Emit(context.Background(), env)
	if err == nil {
		t.Fatalf("Emit() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "context-provide") {
		t.Fatalf("Emit() error = %q, want contains %q", err.Error(), "context-provide")
	}
}
