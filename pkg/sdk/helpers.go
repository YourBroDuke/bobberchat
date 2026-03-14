package sdk

import (
	"time"

	"github.com/bobberchat/bobberchat/internal/protocol"
	"github.com/google/uuid"
)

func NewMessage(from, to, tag string, payload map[string]any) Message {
	if payload == nil {
		payload = map[string]any{}
	}

	traceID := uuid.NewString()

	return Message{
		ID:        uuid.NewString(),
		From:      from,
		To:        to,
		Tag:       tag,
		Payload:   payload,
		Metadata:  map[string]any{"protocol_version": "1.0.0"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TraceID:   traceID,
	}
}

func NewRequestMessage(from, to string, payload map[string]any) Message {
	return NewMessage(from, to, protocol.TagRequestData, payload)
}

func NewResponseMessage(from, to, requestID string, result map[string]any) Message {
	msg := NewMessage(from, to, protocol.TagResponseSuccess, result)
	msg.Metadata["request_id"] = requestID
	return msg
}
