//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestListAgents_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "registry-list-success")
	env.createAgent(t, token, "registry-agent-1", []string{"search"})
	env.createAgent(t, token, "registry-agent-2", []string{"summarize"})

	resp := env.doRequest(t, http.MethodGet, "/v1/registry/agents", nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	agents := assertJSONField(t, body, "agents").([]any)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
}

func TestListAgents_Empty(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "registry-list-empty")

	resp := env.doRequest(t, http.MethodGet, "/v1/registry/agents", nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	agents := assertJSONField(t, body, "agents").([]any)
	if len(agents) != 0 {
		t.Fatalf("expected empty agents list, got %d", len(agents))
	}
}

func TestListAgents_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/registry/agents", nil, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestDiscover_ByCapability(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "registry-discover-cap")
	env.createAgent(t, token, "discover-search", []string{"search"})
	env.createAgent(t, token, "discover-plan", []string{"plan"})

	resp := env.doRequest(t, http.MethodPost, "/v1/registry/discover", map[string]any{
		"capability": "search",
		"status":     []string{"REGISTERED"},
		"limit":      10,
	}, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	agents := assertJSONField(t, body, "agents").([]any)
	if len(agents) != 1 {
		t.Fatalf("expected 1 discovered agent, got %d", len(agents))
	}
}

func TestDiscover_ByStatus(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "registry-discover-status")
	env.createAgent(t, token, "discover-status", []string{"search"})

	resp := env.doRequest(t, http.MethodPost, "/v1/registry/discover", map[string]any{
		"capability": "search",
		"status":     []string{"REGISTERED"},
		"limit":      10,
	}, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	agents := assertJSONField(t, body, "agents").([]any)
	if len(agents) == 0 {
		t.Fatalf("expected discovered agents filtered by status")
	}
}

func TestDiscover_WithLimit(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "registry-discover-limit")
	env.createAgent(t, token, "discover-limit-1", []string{"search"})
	env.createAgent(t, token, "discover-limit-2", []string{"search"})

	resp := env.doRequest(t, http.MethodPost, "/v1/registry/discover", map[string]any{
		"capability": "search",
		"status":     []string{"REGISTERED"},
		"limit":      1,
	}, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	agents := assertJSONField(t, body, "agents").([]any)
	if len(agents) > 1 {
		t.Fatalf("expected at most 1 discovered agent, got %d", len(agents))
	}
}
