package sdk

import (
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/google/uuid"
)

func NewMessage(from, to, tag, content string) Message {
	return Message{
		ID:        uuid.NewString(),
		From:      from,
		To:        to,
		Tag:       tag,
		Content:   content,
		Metadata:  map[string]any{"protocol_version": "1.0.0"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func NewRequestMessage(from, to, content string) Message {
	return NewMessage(from, to, protocol.TagRequestData, content)
}

func NewResponseMessage(from, to, requestID, content string) Message {
	msg := NewMessage(from, to, protocol.TagResponseSuccess, content)
	msg.Metadata["request_id"] = requestID
	return msg
}
