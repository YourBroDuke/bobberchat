package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/adapter"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/google/uuid"
)

type A2AAdapter struct {
	version string
}

func NewA2AAdapter() *A2AAdapter {
	return &A2AAdapter{version: "1.0.0"}
}

type a2aMessage struct {
	Method string         `json:"method"`
	ID     any            `json:"id,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

var _ adapter.Adapter = (*A2AAdapter)(nil)

func (a *A2AAdapter) Name() string {
	return "a2a"
}

func (a *A2AAdapter) Protocol() string {
	return "a2a-json"
}

func (a *A2AAdapter) Validate(raw []byte) error {
	if len(raw) == 0 {
		return errors.New("raw message is empty")
	}

	var msg a2aMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	method := strings.TrimSpace(msg.Method)
	if method == "" {
		return errors.New("method field is required")
	}

	switch method {
	case "message/send", "agent/card", "task/create", "task/update":
		return nil
	default:
		return fmt.Errorf("unsupported a2a method: %s", msg.Method)
	}
}

func (a *A2AAdapter) Ingest(ctx context.Context, raw []byte, meta adapter.TransportMeta) (*protocol.Envelope, error) {
	_ = ctx

	if err := a.Validate(raw); err != nil {
		return nil, err
	}

	var msg a2aMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal a2a message: %w", err)
	}

	sourceID := sourceIDFromA2AID(msg.ID)
	from := strings.TrimSpace(meta.AgentID)
	if from == "" {
		connectionID := strings.TrimSpace(meta.ConnectionID)
		if connectionID == "" {
			connectionID = "unknown"
		}
		from = "a2a:" + connectionID
	}

	to := ""
	if meta.Headers != nil {
		to = strings.TrimSpace(meta.Headers["X-Target-Agent"])
	}
	if to == "" {
		to = "broadcast"
	}

	env := &protocol.Envelope{
		ID:        uuid.NewString(),
		From:      from,
		To:        to,
		Payload:   map[string]any{},
		Metadata:  map[string]any{},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	switch msg.Method {
	case "message/send":
		tag, payload, err := ingestMessageSend(msg.Params)
		if err != nil {
			return nil, err
		}
		adapter.SetSystemMeta(env, protocol.MetaSysTag, tag)
		env.Payload = payload

	case "agent/card":
		adapter.SetSystemMeta(env, protocol.MetaSysTag, protocol.TagContextProvide)
		payload, err := ingestAgentCard(msg.Params)
		if err != nil {
			return nil, err
		}
		env.Payload = payload

	case "task/create":
		adapter.SetSystemMeta(env, protocol.MetaSysTag, protocol.TagRequestAction)
		payload, err := ingestTaskCreate(msg.Params)
		if err != nil {
			return nil, err
		}
		if action, ok := payload["action"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysAction, action)
			delete(payload, "action")
		}
		if taskID, ok := payload["task_id"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysTaskID, taskID)
			delete(payload, "task_id")
		}
		if status, ok := payload["status"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysStatus, status)
			delete(payload, "status")
		}
		if result, ok := payload["result"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysResult, result)
			delete(payload, "result")
		}
		env.Payload = payload

	case "task/update":
		tag, payload, err := ingestTaskUpdate(msg.Params)
		if err != nil {
			return nil, err
		}
		adapter.SetSystemMeta(env, protocol.MetaSysTag, tag)
		if taskID, ok := payload["task_id"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysTaskID, taskID)
			delete(payload, "task_id")
		}
		if status, ok := payload["status"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysStatus, status)
			delete(payload, "status")
		}
		if result, ok := payload["result"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysResult, result)
			delete(payload, "result")
		}
		if message, ok := payload["message"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysMessage, message)
			delete(payload, "message")
		}
		if code, ok := payload["code"]; ok {
			adapter.SetSystemMeta(env, protocol.MetaSysCode, code)
			delete(payload, "code")
		}
		env.Payload = payload

	default:
		return nil, fmt.Errorf("unsupported a2a method: %s", msg.Method)
	}

	adapter.SetAdapterMetadata(env, a.Name(), a.version, adapter.DirectionInbound, sourceID, a.Protocol())

	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("ingested envelope invalid: %w", err)
	}

	return env, nil
}

func (a *A2AAdapter) Emit(ctx context.Context, env *protocol.Envelope) ([]byte, error) {
	_ = ctx

	if env == nil {
		return nil, errors.New("envelope is nil")
	}

	msg := a2aMessage{}
	tag := protocol.EffectiveTag(env)

	switch {
	case strings.HasPrefix(tag, protocol.TagRequest+"."):
		msg.Method = "message/send"
		msg.ID = requestIDForEmit(env)
		msg.Params = map[string]any{
			"message": buildOutboundMessagePayload(env),
		}

	case tag == protocol.TagResponseSuccess:
		msg.Method = "task/update"
		msg.ID = requestIDForEmit(env)
		params := map[string]any{
			"taskId": taskIDForEmit(env),
			"status": "completed",
		}
		if result, ok := adapter.SystemMeta(env, protocol.MetaSysResult); ok {
			params["result"] = result
		}
		msg.Params = params

	case tag == protocol.TagResponseError:
		msg.Method = "task/update"
		msg.ID = requestIDForEmit(env)
		params := map[string]any{
			"taskId": taskIDForEmit(env),
			"status": "failed",
		}
		result := map[string]any{}
		if code, ok := adapter.SystemMeta(env, protocol.MetaSysCode); ok {
			result["code"] = code
		}
		if message := adapter.SystemMetaString(env, protocol.MetaSysMessage); message != "" {
			result["message"] = message
		}
		if len(result) > 0 {
			params["result"] = result
		}
		msg.Params = params

	case strings.HasPrefix(tag, protocol.TagProgress+"."):
		msg.Method = "task/update"
		msg.ID = requestIDForEmit(env)
		params := map[string]any{
			"taskId": taskIDForEmit(env),
			"status": "in_progress",
		}
		if result, ok := adapter.SystemMeta(env, protocol.MetaSysResult); ok {
			params["result"] = result
		} else if progress, ok := env.Payload["progress"]; ok {
			params["result"] = progress
		} else if len(env.Payload) > 0 {
			params["result"] = env.Payload
		}
		msg.Params = params

	case tag == protocol.TagContextProvide:
		return nil, errors.New("unsupported envelope tag for a2a emit: context-provide")

	default:
		return nil, fmt.Errorf("unsupported envelope tag for a2a emit: %s", tag)
	}

	out, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal a2a message: %w", err)
	}

	return out, nil
}

func ingestMessageSend(params map[string]any) (string, map[string]any, error) {
	message, ok := nestedMap(params, "message")
	if !ok {
		return "", nil, errors.New("message/send params.message is required")
	}

	text := extractA2AMessageText(message)
	capabilities := extractCapabilities(params)
	intent := strings.ToLower(text + " " + strings.Join(capabilities, " "))

	tag := protocol.TagRequestAction
	if hasAnyToken(intent, dataIntentTokens()) {
		tag = protocol.TagRequestData
	} else if hasAnyToken(intent, approvalIntentTokens()) {
		tag = protocol.TagRequestApproval
	}

	payload := map[string]any{
		"message":     text,
		"raw_message": message,
	}
	if messageID, ok := message["messageId"]; ok {
		payload["message_id"] = messageID
	}
	if taskID, ok := message["taskId"]; ok {
		payload["task_id"] = taskID
	}
	if contextID, ok := message["contextId"]; ok {
		payload["context_id"] = contextID
	}

	return tag, payload, nil
}

func ingestAgentCard(params map[string]any) (map[string]any, error) {
	name := strings.TrimSpace(asString(params["name"]))
	if name == "" {
		return nil, errors.New("agent/card params.name is required")
	}

	payload := map[string]any{
		"name":         name,
		"description":  strings.TrimSpace(asString(params["description"])),
		"capabilities": extractCapabilities(params),
		"endpoint":     strings.TrimSpace(asString(params["endpoint"])),
	}

	return payload, nil
}

func ingestTaskCreate(params map[string]any) (map[string]any, error) {
	taskID := strings.TrimSpace(asString(params["taskId"]))
	if taskID == "" {
		return nil, errors.New("task/create params.taskId is required")
	}

	payload := map[string]any{
		"action":  "task.create",
		"task_id": taskID,
	}

	if status := strings.TrimSpace(asString(params["status"])); status != "" {
		payload["status"] = status
	}
	if result, ok := params["result"]; ok {
		payload["result"] = result
	}

	return payload, nil
}

func ingestTaskUpdate(params map[string]any) (string, map[string]any, error) {
	taskID := strings.TrimSpace(asString(params["taskId"]))
	if taskID == "" {
		return "", nil, errors.New("task/update params.taskId is required")
	}

	status := strings.TrimSpace(asString(params["status"]))
	if status == "" {
		return "", nil, errors.New("task/update params.status is required")
	}

	payload := map[string]any{
		"task_id": taskID,
		"status":  status,
	}
	if result, ok := params["result"]; ok {
		payload["result"] = result
	}

	switch status {
	case "completed":
		return protocol.TagResponseSuccess, payload, nil
	case "failed":
		if _, ok := payload["result"]; ok {
			if resultMap, ok := payload["result"].(map[string]any); ok {
				if message, ok := resultMap["message"]; ok {
					payload["message"] = message
				}
				if code, ok := resultMap["code"]; ok {
					payload["code"] = code
				}
			}
		}
		if _, ok := payload["message"]; !ok {
			payload["message"] = "task failed"
		}
		if _, ok := payload["code"]; !ok {
			payload["code"] = "failed"
		}
		return protocol.TagResponseError, payload, nil
	case "in_progress":
		return protocol.TagProgressUpdate, payload, nil
	default:
		return "", nil, fmt.Errorf("unsupported task/update status: %s", status)
	}
}

func extractA2AMessageText(message map[string]any) string {
	partsRaw, ok := message["parts"]
	if !ok {
		return strings.TrimSpace(asString(message["text"]))
	}

	parts, ok := partsRaw.([]any)
	if !ok {
		return strings.TrimSpace(asString(message["text"]))
	}

	texts := make([]string, 0, len(parts))
	for _, partRaw := range parts {
		partMap, ok := partRaw.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(asString(partMap["type"])) != "text" {
			continue
		}
		text := strings.TrimSpace(asString(partMap["text"]))
		if text != "" {
			texts = append(texts, text)
		}
	}

	if len(texts) == 0 {
		return strings.TrimSpace(asString(message["text"]))
	}

	return strings.Join(texts, "\n")
}

func extractCapabilities(params map[string]any) []string {
	raw, ok := params["capabilities"]
	if !ok {
		return []string{}
	}
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}

	capabilities := make([]string, 0, len(list))
	for _, item := range list {
		v := strings.TrimSpace(asString(item))
		if v != "" {
			capabilities = append(capabilities, strings.ToLower(v))
		}
	}

	return capabilities
}

func nestedMap(parent map[string]any, key string) (map[string]any, bool) {
	raw, ok := parent[key]
	if !ok {
		return nil, false
	}
	child, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return child, true
}

func sourceIDFromA2AID(id any) string {
	if id == nil {
		return ""
	}
	return fmt.Sprint(id)
}

func requestIDForEmit(env *protocol.Envelope) any {
	if requestID, ok := adapter.SystemMeta(env, protocol.MetaSysRequestID); ok && requestID != nil {
		return requestID
	}
	return env.ID
}

func taskIDForEmit(env *protocol.Envelope) string {
	if taskID := strings.TrimSpace(adapter.SystemMetaString(env, protocol.MetaSysTaskID)); taskID != "" {
		return taskID
	}
	if requestID := strings.TrimSpace(adapter.SystemMetaString(env, protocol.MetaSysRequestID)); requestID != "" {
		return requestID
	}
	return env.ID
}

func buildOutboundMessagePayload(env *protocol.Envelope) map[string]any {
	message := map[string]any{
		"role":      "user",
		"parts":     []any{map[string]any{"type": "text", "text": outboundMessageText(env)}},
		"messageId": env.ID,
	}
	if taskID := strings.TrimSpace(adapter.SystemMetaString(env, protocol.MetaSysTaskID)); taskID != "" {
		message["taskId"] = taskID
	}
	if contextID := strings.TrimSpace(asString(env.Payload["context_id"])); contextID != "" {
		message["contextId"] = contextID
	}
	return message
}

func outboundMessageText(env *protocol.Envelope) string {
	if text := strings.TrimSpace(adapter.SystemMetaString(env, protocol.MetaSysMessage)); text != "" {
		return text
	}
	if text := strings.TrimSpace(asString(env.Payload["message"])); text != "" {
		return text
	}
	if action := strings.TrimSpace(adapter.SystemMetaString(env, protocol.MetaSysAction)); action != "" {
		return action
	}
	b, err := json.Marshal(env.Payload)
	if err != nil {
		return protocol.EffectiveTag(env)
	}
	return string(b)
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func hasAnyToken(text string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func dataIntentTokens() []string {
	return []string{"data", "fetch", "retrieve", "lookup", "search", "find", "query", "list", "show", "read", "load", "pull", "report"}
}

func approvalIntentTokens() []string {
	return []string{"approve", "approval", "permission", "consent", "authorize", "allow", "deny", "reject", "sign off"}
}
