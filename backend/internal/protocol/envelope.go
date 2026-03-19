package protocol

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// System metadata key constants. All system-injected data lives under these
// keys in Metadata so that Tag and Content remain pure user input.
const (
	MetaSysTag               = "_tag"
	MetaSysAction            = "_action"
	MetaSysArgs              = "_args"
	MetaSysResult            = "_result"
	MetaSysRequestID         = "_request_id"
	MetaSysCode              = "_code"
	MetaSysMessage           = "_message"
	MetaSysStreamID          = "_stream_id"
	MetaSysUpdate            = "_update"
	MetaSysPercentage        = "_percentage"
	MetaSysReplayed          = "_replayed"
	MetaSysOriginalMessageID = "_original_message_id"
	MetaSysReplayReason      = "_replay_reason"
	MetaSysTaskID            = "_task_id"
	MetaSysStatus            = "_status"
)

type Envelope struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Tag       string         `json:"tag,omitempty"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp string         `json:"timestamp"`
}

// EffectiveTag returns the tag for routing and protocol logic. It prefers the
// user-supplied Tag field; when Tag is empty it falls back to the system tag
// stored in Metadata["_tag"].
func EffectiveTag(env *Envelope) string {
	if env == nil {
		return ""
	}
	if t := strings.TrimSpace(env.Tag); t != "" {
		return t
	}
	if env.Metadata == nil {
		return ""
	}
	if raw, ok := env.Metadata[MetaSysTag]; ok {
		if s, ok := raw.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func (e *Envelope) Validate() error {
	if e == nil {
		return errors.New("envelope is nil")
	}

	if strings.TrimSpace(e.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(e.From) == "" {
		return errors.New("from is required")
	}
	if strings.TrimSpace(e.To) == "" {
		return errors.New("to is required")
	}
	if strings.TrimSpace(e.Timestamp) == "" {
		return errors.New("timestamp is required")
	}
	tag := EffectiveTag(e)
	if tag == "" {
		return errors.New("tag is required (set Tag field or Metadata[\"_tag\"])")
	}

	if !IsValidTag(tag) {
		return fmt.Errorf("invalid tag: %s", tag)
	}

	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		return fmt.Errorf("timestamp must be valid ISO8601/RFC3339: %w", err)
	}

	return nil
}
