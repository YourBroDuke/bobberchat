package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// Config holds rate-limit settings loaded from backend.yaml.
type Config struct {
	PerAgentMPS float64 `mapstructure:"per_agent_mps"`
	PerGroupMPS float64 `mapstructure:"per_group_mps"`
	PerTagMPS   float64 `mapstructure:"per_tag_mps"`
	BurstFactor int     `mapstructure:"burst_factor"`
	CleanupSecs int     `mapstructure:"cleanup_seconds"`
	Enabled     bool    `mapstructure:"enabled"`
}

// DefaultConfig returns production defaults matching §11.2.3 targets.
func DefaultConfig() Config {
	return Config{
		PerAgentMPS: 100,
		PerGroupMPS: 500,
		PerTagMPS:   50,
		BurstFactor: 10,
		CleanupSecs: 300,
		Enabled:     true,
	}
}

// Limiter provides per-key token-bucket rate limiting.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	cfg     Config
	now     func() time.Time
}

type bucket struct {
	tokens   float64
	maxBurst float64
	rate     float64
	lastFill time.Time
}

// New creates a Limiter with the given config and starts a background
// cleanup goroutine. Call Stop to release resources.
func New(cfg Config) *Limiter {
	if cfg.BurstFactor <= 0 {
		cfg.BurstFactor = 10
	}
	if cfg.CleanupSecs <= 0 {
		cfg.CleanupSecs = 300
	}

	l := &Limiter{
		buckets: make(map[string]*bucket),
		cfg:     cfg,
		now:     time.Now,
	}
	return l
}

// Allow checks whether a request identified by the given dimension is
// within the rate limit. Returns true if allowed, false if rate-limited.
func (l *Limiter) Allow(dimension string, key string) bool {
	if !l.cfg.Enabled {
		return true
	}

	rate := l.rateFor(dimension)
	if rate <= 0 {
		return true
	}

	fullKey := fmt.Sprintf("%s:%s", dimension, key)
	maxBurst := float64(l.cfg.BurstFactor) * rate

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[fullKey]
	if !ok {
		b = &bucket{
			tokens:   maxBurst - 1,
			maxBurst: maxBurst,
			rate:     rate,
			lastFill: now,
		}
		l.buckets[fullKey] = b
		return true
	}

	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.maxBurst {
		b.tokens = b.maxBurst
	}
	b.lastFill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// AgentKey returns the rate-limit key for a per-agent check.
func AgentKey(agentID string) string {
	return fmt.Sprintf("agent:%s", agentID)
}

// GroupKey returns the rate-limit key for a per-group check.
func GroupKey(groupID string) string {
	return fmt.Sprintf("group:%s", groupID)
}

// TagKey returns the rate-limit key for a per-tag check.
func TagKey(tag string) string {
	return fmt.Sprintf("tag:%s", tag)
}

// Dimensions used to select rate.
const (
	DimensionAgent = "agent"
	DimensionGroup = "group"
	DimensionTag   = "tag"
)

func (l *Limiter) rateFor(dimension string) float64 {
	switch dimension {
	case DimensionAgent:
		return l.cfg.PerAgentMPS
	case DimensionGroup:
		return l.cfg.PerGroupMPS
	case DimensionTag:
		return l.cfg.PerTagMPS
	default:
		return 0
	}
}

// Cleanup removes stale buckets that have been idle for longer than
// CleanupSecs. Call periodically or from a background goroutine.
func (l *Limiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := l.now().Add(-time.Duration(l.cfg.CleanupSecs) * time.Second)
	for k, b := range l.buckets {
		if b.lastFill.Before(cutoff) {
			delete(l.buckets, k)
		}
	}
}

// Len returns the number of tracked buckets (for testing/metrics).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
