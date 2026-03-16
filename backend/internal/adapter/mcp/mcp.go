package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/adapter"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/google/uuid"
)

type MCPAdapter struct {
	version string
}

func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{version: "1.0.0"}
}

type jsonRPCMessage struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *jsonRPCError  `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

var _ adapter.Adapter = (*MCPAdapter)(nil)

func (a *MCPAdapter) Name() string {
	return "mcp"
}

func (a *MCPAdapter) Protocol() string {
	return "json-rpc"
}

func (a *MCPAdapter) Validate(raw []byte) error {
	if len(raw) == 0 {
		return errors.New("raw message is empty")
	}

	var msg jsonRPCMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	if strings.TrimSpace(msg.JSONRPC) == "" {
		return errors.New("jsonrpc field is required")
	}

	if msg.JSONRPC != "2.0" {
		return fmt.Errorf("invalid jsonrpc version: %s", msg.JSONRPC)
	}

	if strings.TrimSpace(msg.Method) == "" && msg.Result == nil && msg.Error == nil {
		return errors.New("json-rpc message must include method, result, or error")
	}

	return nil
}

func (a *MCPAdapter) Ingest(ctx context.Context, raw []byte, meta adapter.TransportMeta) (*protocol.Envelope, error) {
	_ = ctx

	if err := a.Validate(raw); err != nil {
		return nil, err
	}

	var msg jsonRPCMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal json-rpc message: %w", err)
	}

	sourceID := sourceIDFromJSONRPCID(msg.ID)
	from := strings.TrimSpace(meta.AgentID)
	if from == "" {
		connectionID := strings.TrimSpace(meta.ConnectionID)
		if connectionID == "" {
			connectionID = "unknown"
		}
		from = "mcp:" + connectionID
	}

	to := ""
	if meta.Headers != nil {
		to = strings.TrimSpace(meta.Headers["X-Target-Agent"])
	}
	if to == "" {
		to = strings.TrimSpace(meta.TenantID)
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
		TraceID:   uuid.NewString(),
	}

	switch {
	case msg.Error != nil:
		if msg.ID == nil {
			return nil, errors.New("json-rpc error message missing id")
		}
		env.Tag = protocol.TagResponseError
		env.Payload["code"] = strconv.Itoa(msg.Error.Code)
		env.Payload["message"] = msg.Error.Message
		if sourceID != "" {
			env.Payload["request_id"] = sourceID
		}

	case msg.Result != nil:
		if msg.ID == nil {
			return nil, errors.New("json-rpc result message missing id")
		}
		env.Tag = protocol.TagResponseSuccess
		if sourceID != "" {
			env.Payload["request_id"] = sourceID
		}
		env.Payload["result"] = msg.Result

	case strings.TrimSpace(msg.Method) != "" && msg.ID == nil:
		env.Tag = protocol.TagContextProvide
		env.Payload["summary"] = buildNotificationSummary(msg.Method, msg.Params)

	case msg.Method == "tools/call" && msg.ID != nil:
		env.Tag = protocol.TagRequestAction
		nameRaw, ok := msg.Params["name"]
		if !ok {
			return nil, errors.New("tools/call params.name is required")
		}
		action, ok := nameRaw.(string)
		if !ok || strings.TrimSpace(action) == "" {
			return nil, errors.New("tools/call params.name must be a non-empty string")
		}
		env.Payload["action"] = action
		if args, ok := msg.Params["arguments"]; ok {
			env.Payload["args"] = args
		} else {
			env.Payload["args"] = map[string]any{}
		}

	default:
		return nil, errors.New("unsupported mcp message shape")
	}

	adapter.SetAdapterMetadata(env, a.Name(), a.version, adapter.DirectionInbound, sourceID, a.Protocol())

	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("ingested envelope invalid: %w", err)
	}

	return env, nil
}

func (a *MCPAdapter) Emit(ctx context.Context, env *protocol.Envelope) ([]byte, error) {
	_ = ctx

	if env == nil {
		return nil, errors.New("envelope is nil")
	}

	msg := jsonRPCMessage{JSONRPC: "2.0"}

	switch env.Tag {
	case protocol.TagRequestAction:
		actionRaw, ok := env.Payload["action"]
		if !ok {
			return nil, errors.New("request.action payload.action is required")
		}
		action, ok := actionRaw.(string)
		if !ok || strings.TrimSpace(action) == "" {
			return nil, errors.New("request.action payload.action must be a non-empty string")
		}

		msg.ID = requestIDForEmit(env)
		msg.Method = "tools/call"
		msg.Params = map[string]any{
			"name": action,
		}
		if args, ok := env.Payload["args"]; ok {
			msg.Params["arguments"] = args
		} else {
			msg.Params["arguments"] = map[string]any{}
		}

	case protocol.TagResponseSuccess:
		msg.ID = requestIDForEmit(env)
		if result, ok := env.Payload["result"]; ok {
			msg.Result = result
		} else {
			return nil, errors.New("response.success payload.result is required")
		}

	case protocol.TagResponseError:
		codeRaw, ok := env.Payload["code"]
		if !ok {
			return nil, errors.New("response.error payload.code is required")
		}
		messageRaw, ok := env.Payload["message"]
		if !ok {
			return nil, errors.New("response.error payload.message is required")
		}

		code, err := parseErrorCode(codeRaw)
		if err != nil {
			return nil, err
		}
		message, ok := messageRaw.(string)
		if !ok || strings.TrimSpace(message) == "" {
			return nil, errors.New("response.error payload.message must be a non-empty string")
		}

		msg.ID = requestIDForEmit(env)
		msg.Error = &jsonRPCError{Code: code, Message: message}

	default:
		return nil, fmt.Errorf("unsupported envelope tag for mcp emit: %s", env.Tag)
	}

	out, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal json-rpc message: %w", err)
	}

	return out, nil
}

func sourceIDFromJSONRPCID(id any) string {
	if id == nil {
		return ""
	}

	switch v := id.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func buildNotificationSummary(method string, params map[string]any) string {
	method = strings.TrimSpace(method)
	if len(params) == 0 {
		return method
	}

	b, err := json.Marshal(params)
	if err != nil {
		return method
	}

	return method + " " + string(b)
}

func requestIDForEmit(env *protocol.Envelope) any {
	if requestID, ok := env.Payload["request_id"]; ok {
		return requestID
	}
	return env.ID
}

func parseErrorCode(raw any) (int, error) {
	switch v := raw.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		code, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("response.error payload.code must be numeric: %w", err)
		}
		return code, nil
	default:
		return 0, errors.New("response.error payload.code must be numeric")
	}
}
