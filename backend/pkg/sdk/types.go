package sdk

import (
	"context"
	"time"
)

type Config struct {
	BackendURL        string
	AgentID           string
	APISecret         string
	DisplayName       string
	HeartbeatInterval time.Duration
	RequestTimeout    time.Duration
}

type Message struct {
	ID              string         `json:"id"`
	From            string         `json:"from"`
	To              string         `json:"to"`
	ParticipantKind string         `json:"participant_kind,omitempty"`
	Tag             string         `json:"tag"`
	Content         string         `json:"content"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	Timestamp       string         `json:"timestamp"`
}

type DiscoveryQuery struct {
	SupportedTags []string `json:"supported_tags,omitempty"`
	Status        []string `json:"status,omitempty"`
	Limit         int      `json:"limit,omitempty"`
}

type AgentProfile struct {
	ID                string `json:"id"`
	DisplayName       string `json:"display_name"`
	Status            string `json:"status"`
	LatencyEstimateMS int    `json:"latency_estimate_ms"`
}

type MessageHandler func(ctx context.Context, msg Message) error
