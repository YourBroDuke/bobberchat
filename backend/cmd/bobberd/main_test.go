package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/observability"
	"github.com/bobberchat/bobberchat/backend/internal/persistence"
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

type fakeAuditRepo struct {
	entries []persistence.AuditLogEntry
}

func (f *fakeAuditRepo) Append(_ context.Context, entry persistence.AuditLogEntry) (*persistence.AuditLogEntry, error) {
	entry.ID = int64(len(f.entries) + 1)
	entry.CreatedAt = time.Now().UTC()
	f.entries = append(f.entries, entry)
	return &entry, nil
}

func (f *fakeAuditRepo) QueryRecent(_ context.Context, limit int) ([]persistence.AuditLogEntry, error) {
	if limit > len(f.entries) {
		limit = len(f.entries)
	}
	return f.entries[:limit], nil
}

func newTestApp(limiterCfg *ratelimit.Config) (*app, *fakeBroker, *fakeAuditRepo) {
	fb := &fakeBroker{}
	fa := &fakeAuditRepo{}

	var lim *ratelimit.Limiter
	if limiterCfg != nil {
		lim = ratelimit.New(*limiterCfg)
	}

	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	a := &app{
		limiter:   lim,
		auditRepo: fa,
		metrics:   metrics,
		publisher: fb,
	}
	return a, fb, fa
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
	a, _, _ := newTestApp(&cfg)

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
	a, _, _ := newTestApp(&cfg)

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
	a, _, _ := newTestApp(&cfg)

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
	a, _, _ := newTestApp(&cfg)

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

func TestPublishAndAudit_AuditDetails(t *testing.T) {
	a, _, fa := newTestApp(nil)

	env := makeEnvelope("from-agent", "to-agent", "chat.message")

	err := a.publishAndAudit(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fa.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(fa.entries))
	}

	entry := fa.entries[0]
	if entry.Details["from"] != "from-agent" {
		t.Fatalf("expected from=from-agent, got %v", entry.Details["from"])
	}
	if entry.Details["to"] != "to-agent" {
		t.Fatalf("expected to=to-agent, got %v", entry.Details["to"])
	}
	if entry.Details["tag"] != "chat.message" {
		t.Fatalf("expected tag=chat.message, got %v", entry.Details["tag"])
	}
}

func TestPublishAndAudit_NoAuditRepo(t *testing.T) {
	fb := &fakeBroker{}
	a := &app{
		auditRepo: nil,
		limiter:   nil,
		publisher: fb,
	}

	env := makeEnvelope("agent1", "agent2", "chat.message")
	err := a.publishAndAudit(context.Background(), env)
	if err != nil {
		t.Fatalf("should succeed without audit repo: %v", err)
	}
}
