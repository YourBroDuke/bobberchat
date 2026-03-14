package ratelimit

import (
	"strings"
	"testing"
	"time"
)

func TestAllow_DisabledLimiter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	l := New(cfg)

	for i := 0; i < 10000; i++ {
		if !l.Allow(DimensionAgent, "t/a") {
			t.Fatalf("disabled limiter should always allow")
		}
	}
}

func TestAllow_BurstThenThrottle(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 10,
		BurstFactor: 5,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)

	frozen := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return frozen }

	allowed := 0
	for i := 0; i < 200; i++ {
		if l.Allow(DimensionAgent, AgentKey("tenant1", "agent1")) {
			allowed++
		}
	}

	// burst = 10 * 5 = 50 tokens
	if allowed != 50 {
		t.Fatalf("expected 50 allowed in burst, got %d", allowed)
	}
}

func TestAllow_RefillsOverTime(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 10,
		BurstFactor: 2,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	key := AgentKey("t", "a")
	for i := 0; i < 20; i++ {
		l.Allow(DimensionAgent, key)
	}
	if l.Allow(DimensionAgent, key) {
		t.Fatal("should be throttled after burst exhaustion")
	}

	now = now.Add(1 * time.Second)
	allowed := 0
	for i := 0; i < 15; i++ {
		if l.Allow(DimensionAgent, key) {
			allowed++
		}
	}
	if allowed != 10 {
		t.Fatalf("expected 10 refilled tokens after 1s at 10 MPS, got %d", allowed)
	}
}

func TestAllow_DimensionIsolation(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 5,
		PerGroupMPS: 5,
		PerTagMPS:   5,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)
	frozen := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return frozen }

	agentKey := AgentKey("t", "a")
	groupKey := GroupKey("t", "g")

	for i := 0; i < 5; i++ {
		l.Allow(DimensionAgent, agentKey)
	}
	if l.Allow(DimensionAgent, agentKey) {
		t.Fatal("agent dimension should be exhausted")
	}

	if !l.Allow(DimensionGroup, groupKey) {
		t.Fatal("group dimension should be independent of agent dimension")
	}
}

func TestAllow_PerKeyIsolation(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 5,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)
	frozen := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return frozen }

	for i := 0; i < 5; i++ {
		l.Allow(DimensionAgent, AgentKey("t", "agent1"))
	}
	if l.Allow(DimensionAgent, AgentKey("t", "agent1")) {
		t.Fatal("agent1 should be exhausted")
	}

	if !l.Allow(DimensionAgent, AgentKey("t", "agent2")) {
		t.Fatal("agent2 should be independent")
	}
}

func TestAllow_ZeroRate(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 0,
		BurstFactor: 5,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)

	for i := 0; i < 100; i++ {
		if !l.Allow(DimensionAgent, "anything") {
			t.Fatal("zero rate should allow everything")
		}
	}
}

func TestAllow_UnknownDimension(t *testing.T) {
	cfg := DefaultConfig()
	l := New(cfg)

	for i := 0; i < 100; i++ {
		if !l.Allow("unknown", "key") {
			t.Fatal("unknown dimension should allow everything")
		}
	}
}

func TestCleanup(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 10,
		BurstFactor: 5,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	l.Allow(DimensionAgent, "stale-key")
	l.Allow(DimensionAgent, "fresh-key")

	if l.Len() != 2 {
		t.Fatalf("expected 2 buckets, got %d", l.Len())
	}

	now = now.Add(90 * time.Second)
	l.Allow(DimensionAgent, "fresh-key")

	now = now.Add(30 * time.Second)
	l.Cleanup()

	if l.Len() != 1 {
		t.Fatalf("expected 1 bucket after cleanup, got %d", l.Len())
	}
}

func TestKeyFunctions(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string, string) string
		a, b string
		want string
	}{
		{"AgentKey", AgentKey, "t1", "a1", "t1/a1"},
		{"GroupKey", GroupKey, "t1", "g1", "t1/g1"},
		{"TagKey", TagKey, "t1", "request.action", "t1/request.action"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Fatal("default config should be enabled")
	}
	if cfg.PerAgentMPS <= 0 {
		t.Fatal("PerAgentMPS should be positive")
	}
	if cfg.BurstFactor <= 0 {
		t.Fatal("BurstFactor should be positive")
	}
}

func TestAllow_AllDimensions(t *testing.T) {
	cfg := Config{
		PerAgentMPS: 2,
		PerGroupMPS: 2,
		PerTagMPS:   2,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     true,
	}
	l := New(cfg)
	frozen := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return frozen }

	dimensions := []string{DimensionAgent, DimensionGroup, DimensionTag}
	for _, dim := range dimensions {
		t.Run(dim, func(t *testing.T) {
			key := strings.Join([]string{"t", dim}, "/")
			l.Allow(dim, key)
			l.Allow(dim, key)
			if l.Allow(dim, key) {
				t.Fatalf("dimension %s should be exhausted after burst", dim)
			}
		})
	}
}
