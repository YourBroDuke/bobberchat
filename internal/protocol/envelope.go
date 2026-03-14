package protocol

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Envelope struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Tag       string         `json:"tag"`
	Payload   map[string]any `json:"payload"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp string         `json:"timestamp"`
	TraceID   string         `json:"trace_id"`
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
	if strings.TrimSpace(e.Tag) == "" {
		return errors.New("tag is required")
	}
	if strings.TrimSpace(e.Timestamp) == "" {
		return errors.New("timestamp is required")
	}
	if strings.TrimSpace(e.TraceID) == "" {
		return errors.New("trace_id is required")
	}

	if e.Payload == nil {
		return errors.New("payload must not be nil")
	}

	if !IsValidTag(e.Tag) {
		return fmt.Errorf("invalid tag: %s", e.Tag)
	}

	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		return fmt.Errorf("timestamp must be valid ISO8601/RFC3339: %w", err)
	}

	return nil
}
