package sdk

import (
	"context"
	"time"
)

type Config struct {
	BackendURL        string
	AgentID           string
	APISecret         string
	TenantID          string
	DisplayName       string
	Capabilities      []string
	HeartbeatInterval time.Duration
	RequestTimeout    time.Duration
}

type Message struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Tag       string         `json:"tag"`
	Payload   map[string]any `json:"payload"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp string         `json:"timestamp"`
	TraceID   string         `json:"trace_id"`
}

type DiscoveryQuery struct {
	Capability    string   `json:"capability,omitempty"`
	SupportedTags []string `json:"supported_tags,omitempty"`
	Status        []string `json:"status,omitempty"`
	Limit         int      `json:"limit,omitempty"`
}

type AgentProfile struct {
	AgentID           string   `json:"agent_id"`
	DisplayName       string   `json:"display_name"`
	Capabilities      []string `json:"capabilities"`
	Status            string   `json:"status"`
	Version           string   `json:"version"`
	LatencyEstimateMS int      `json:"latency_estimate_ms"`
	LastHeartbeat     string   `json:"last_heartbeat"`
}

type MessageHandler func(ctx context.Context, msg Message) error
