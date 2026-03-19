package grpc

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

type GRPCAdapter struct {
	version string
}

func NewGRPCAdapter() *GRPCAdapter {
	return &GRPCAdapter{version: "1.0.0"}
}

type grpcMessage struct {
	Type      string         `json:"type"`
	Service   string         `json:"service,omitempty"`
	Method    string         `json:"method,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	StreamID  string         `json:"stream_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Code      int            `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Body      map[string]any `json:"body,omitempty"`
}

var _ adapter.Adapter = (*GRPCAdapter)(nil)

func (a *GRPCAdapter) Name() string {
	return "grpc"
}

func (a *GRPCAdapter) Protocol() string {
	return "grpc-json"
}

func (a *GRPCAdapter) Validate(raw []byte) error {
	if len(raw) == 0 {
		return errors.New("raw message is empty")
	}

	var msg grpcMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	msgType := strings.TrimSpace(msg.Type)
	if msgType == "" {
		return errors.New("type field is required")
	}

	switch msgType {
	case "unary":
		hasService := strings.TrimSpace(msg.Service) != ""
		hasMethod := strings.TrimSpace(msg.Method) != ""
		hasStatus := strings.TrimSpace(msg.Status) != ""

		if hasService != hasMethod {
			return errors.New("unary call must include both service and method")
		}

		if !hasStatus && !hasService {
			return errors.New("unary message must include service+method or status")
		}

		if hasStatus {
			status := strings.TrimSpace(msg.Status)
			if status != "OK" && status != "ERROR" {
				return fmt.Errorf("invalid unary status: %s", status)
			}
		}

	case "stream":
		if strings.TrimSpace(msg.StreamID) == "" {
			return errors.New("stream_id field is required for stream messages")
		}

	default:
		return fmt.Errorf("invalid type: %s", msgType)
	}

	return nil
}

func (a *GRPCAdapter) Ingest(ctx context.Context, raw []byte, meta adapter.TransportMeta) (*protocol.Envelope, error) {
	_ = ctx

	if err := a.Validate(raw); err != nil {
		return nil, err
	}

	var msg grpcMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal grpc message: %w", err)
	}

	sourceID := strings.TrimSpace(msg.RequestID)
	from := strings.TrimSpace(meta.AgentID)
	if from == "" {
		connectionID := strings.TrimSpace(meta.ConnectionID)
		if connectionID == "" {
			connectionID = "unknown"
		}
		from = "grpc:" + connectionID
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

	msgType := strings.TrimSpace(msg.Type)
	status := strings.TrimSpace(msg.Status)

	switch {
	case msgType == "unary" && strings.TrimSpace(msg.Service) != "" && strings.TrimSpace(msg.Method) != "" && status == "":
		adapter.SetSystemMeta(env, protocol.MetaSysTag, protocol.TagRequestAction)
		adapter.SetSystemMeta(env, protocol.MetaSysAction, strings.TrimSpace(msg.Service)+"/"+strings.TrimSpace(msg.Method))
		if msg.Body != nil {
			adapter.SetSystemMeta(env, protocol.MetaSysArgs, msg.Body)
		} else {
			adapter.SetSystemMeta(env, protocol.MetaSysArgs, map[string]any{})
		}

	case msgType == "unary" && status == "OK":
		adapter.SetSystemMeta(env, protocol.MetaSysTag, protocol.TagResponseSuccess)
		if msg.Body != nil {
			adapter.SetSystemMeta(env, protocol.MetaSysResult, msg.Body)
		} else {
			adapter.SetSystemMeta(env, protocol.MetaSysResult, map[string]any{})
		}
		if sourceID != "" {
			adapter.SetSystemMeta(env, protocol.MetaSysRequestID, sourceID)
		}

	case msgType == "unary" && status == "ERROR":
		adapter.SetSystemMeta(env, protocol.MetaSysTag, protocol.TagResponseError)
		adapter.SetSystemMeta(env, protocol.MetaSysCode, msg.Code)
		adapter.SetSystemMeta(env, protocol.MetaSysMessage, msg.Message)
		if sourceID != "" {
			adapter.SetSystemMeta(env, protocol.MetaSysRequestID, sourceID)
		}

	case msgType == "stream":
		adapter.SetSystemMeta(env, protocol.MetaSysTag, protocol.TagProgressUpdate)
		if msg.Body != nil {
			adapter.SetSystemMeta(env, protocol.MetaSysUpdate, msg.Body)
			if progressRaw, ok := msg.Body["progress"]; ok {
				if percentage, ok := numericAsFloat64(progressRaw); ok {
					adapter.SetSystemMeta(env, protocol.MetaSysPercentage, percentage)
				}
			}
		} else {
			adapter.SetSystemMeta(env, protocol.MetaSysUpdate, map[string]any{})
		}
		if sourceID != "" {
			adapter.SetSystemMeta(env, protocol.MetaSysRequestID, sourceID)
		}
		adapter.SetSystemMeta(env, protocol.MetaSysStreamID, msg.StreamID)

	default:
		return nil, errors.New("unsupported grpc message shape")
	}

	adapter.SetAdapterMetadata(env, a.Name(), a.version, adapter.DirectionInbound, sourceID, a.Protocol())

	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("ingested envelope invalid: %w", err)
	}

	return env, nil
}

func (a *GRPCAdapter) Emit(ctx context.Context, env *protocol.Envelope) ([]byte, error) {
	_ = ctx

	if env == nil {
		return nil, errors.New("envelope is nil")
	}

	msg := grpcMessage{}
	tag := protocol.EffectiveTag(env)

	switch {
	case tag == protocol.TagRequestAction:
		action := adapter.SystemMetaString(env, protocol.MetaSysAction)
		if strings.TrimSpace(action) == "" {
			return nil, errors.New("request.action metadata.action is required")
		}

		service, method, found := strings.Cut(strings.TrimSpace(action), "/")
		if !found || strings.TrimSpace(service) == "" || strings.TrimSpace(method) == "" {
			return nil, errors.New("request.action payload.action must be in service/method format")
		}

		msg.Type = "unary"
		msg.Service = strings.TrimSpace(service)
		msg.Method = strings.TrimSpace(method)
		msg.RequestID = requestIDForEmit(env)

		if args, ok := adapter.SystemMeta(env, protocol.MetaSysArgs); ok {
			body, err := normalizeBody(args, "request.action payload.args")
			if err != nil {
				return nil, err
			}
			msg.Body = body
		} else {
			msg.Body = map[string]any{}
		}

	case tag == protocol.TagResponseSuccess:
		resultRaw, ok := adapter.SystemMeta(env, protocol.MetaSysResult)
		if !ok {
			return nil, errors.New("response.success metadata.result is required")
		}

		body, err := normalizeBody(resultRaw, "response.success payload.result")
		if err != nil {
			return nil, err
		}

		msg.Type = "unary"
		msg.RequestID = requestIDForEmit(env)
		msg.Status = "OK"
		msg.Body = body

	case tag == protocol.TagResponseError:
		codeRaw, ok := adapter.SystemMeta(env, protocol.MetaSysCode)
		if !ok {
			return nil, errors.New("response.error metadata.code is required")
		}
		message := adapter.SystemMetaString(env, protocol.MetaSysMessage)
		if strings.TrimSpace(message) == "" {
			return nil, errors.New("response.error metadata.message must be a non-empty string")
		}

		code, err := parseErrorCode(codeRaw)
		if err != nil {
			return nil, err
		}

		msg.Type = "unary"
		msg.RequestID = requestIDForEmit(env)
		msg.Status = "ERROR"
		msg.Code = code
		msg.Message = message

	case protocol.ParseTagFamily(tag) == protocol.TagProgress:
		msg.Type = "stream"
		msg.RequestID = requestIDForEmit(env)
		msg.StreamID = streamIDForEmit(env)

		if updateRaw, ok := adapter.SystemMeta(env, protocol.MetaSysUpdate); ok {
			body, err := normalizeBody(updateRaw, "progress payload.update")
			if err != nil {
				return nil, err
			}
			msg.Body = body
		} else {
			msg.Body = map[string]any{}
			for k, v := range env.Payload {
				msg.Body[k] = v
			}
		}

		if msg.Body == nil {
			msg.Body = map[string]any{}
		}

		if _, exists := msg.Body["progress"]; !exists {
			if pctRaw, ok := adapter.SystemMeta(env, protocol.MetaSysPercentage); ok {
				if pct, ok := numericAsFloat64(pctRaw); ok {
					msg.Body["progress"] = pct
				}
			}
		}

	default:
		return nil, fmt.Errorf("unsupported envelope tag for grpc emit: %s", tag)
	}

	out, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal grpc message: %w", err)
	}

	return out, nil
}

func requestIDForEmit(env *protocol.Envelope) string {
	requestID := adapter.SystemMetaString(env, protocol.MetaSysRequestID)
	if strings.TrimSpace(requestID) != "" {
		return requestID
	}
	return env.ID
}

func streamIDForEmit(env *protocol.Envelope) string {
	streamID := adapter.SystemMetaString(env, protocol.MetaSysStreamID)
	if strings.TrimSpace(streamID) != "" {
		return streamID
	}
	return env.ID
}

func normalizeBody(raw any, field string) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}

	if body, ok := raw.(map[string]any); ok {
		return body, nil
	}

	return nil, fmt.Errorf("%s must be an object", field)
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

func numericAsFloat64(raw any) (float64, bool) {
	switch v := raw.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}
