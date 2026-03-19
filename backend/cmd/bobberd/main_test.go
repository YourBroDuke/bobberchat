package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/observability"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/bobberchat/bobberchat/backend/internal/ratelimit"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

type fakeBroker struct {
	published []*protocol.Envelope
	err       error
}

func (f *fakeBroker) PublishMessage(_ context.Context, env *protocol.Envelope) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, env)
	return nil
}

func newTestApp(limiterCfg *ratelimit.Config) (*app, *fakeBroker) {
	fb := &fakeBroker{}

	var lim *ratelimit.Limiter
	if limiterCfg != nil {
		lim = ratelimit.New(*limiterCfg)
	}

	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	a := &app{
		limiter:   lim,
		metrics:   metrics,
		publisher: fb,
	}
	return a, fb
}

func makeEnvelope(from, to, tag string) *protocol.Envelope {
	return &protocol.Envelope{
		ID:        uuid.NewString(),
		From:      from,
		To:        to,
		Tag:       tag,
		Content:   `{"text":"hello"}`,
		Metadata:  map[string]any{},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func TestPublishAndAudit_RateLimited(t *testing.T) {
	cfg := ratelimit.Config{
		PerAgentMPS: 1,
		PerGroupMPS: 100,
		PerTagMPS:   100,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     true,
	}
	a, _ := newTestApp(&cfg)

	env := makeEnvelope("agent1", "agent2", "chat.message")

	err := a.publishAndAudit(context.Background(), env)
	if err != nil {
		t.Fatalf("first call should succeed, got %v", err)
	}

	env2 := makeEnvelope("agent1", "agent2", "chat.message")
	err = a.publishAndAudit(context.Background(), env2)
	if !errors.Is(err, errRateLimited) {
		t.Fatalf("second call should be rate-limited, got %v", err)
	}
}

func TestPublishAndAudit_RateLimitDisabled(t *testing.T) {
	cfg := ratelimit.Config{
		PerAgentMPS: 1,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     false,
	}
	a, _ := newTestApp(&cfg)

	for i := 0; i < 100; i++ {
		env := makeEnvelope("agent1", "agent2", "chat.message")
		err := a.publishAndAudit(context.Background(), env)
		if err != nil {
			t.Fatalf("disabled limiter should not block, iteration %d: %v", i, err)
		}
	}
}

func TestPublishAndAudit_GroupRateLimit(t *testing.T) {
	cfg := ratelimit.Config{
		PerAgentMPS: 100,
		PerGroupMPS: 1,
		PerTagMPS:   100,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     true,
	}
	a, _ := newTestApp(&cfg)

	env := makeEnvelope("agent1", "group:g1", "chat.message")
	err := a.publishAndAudit(context.Background(), env)
	if err != nil {
		t.Fatalf("first group message should succeed, got %v", err)
	}

	env2 := makeEnvelope("agent1", "group:g1", "chat.message")
	err = a.publishAndAudit(context.Background(), env2)
	if !errors.Is(err, errRateLimited) {
		t.Fatalf("second group message should be rate-limited, got %v", err)
	}
}

func TestPublishAndAudit_TagRateLimit(t *testing.T) {
	cfg := ratelimit.Config{
		PerAgentMPS: 100,
		PerGroupMPS: 100,
		PerTagMPS:   1,
		BurstFactor: 1,
		CleanupSecs: 60,
		Enabled:     true,
	}
	a, _ := newTestApp(&cfg)

	env := makeEnvelope("agent1", "agent2", "request.action")
	err := a.publishAndAudit(context.Background(), env)
	if err != nil {
		t.Fatalf("first call should succeed, got %v", err)
	}

	env2 := makeEnvelope("agent1", "agent2", "request.action")
	err = a.publishAndAudit(context.Background(), env2)
	if !errors.Is(err, errRateLimited) {
		t.Fatalf("second call should be rate-limited by tag, got %v", err)
	}
}
