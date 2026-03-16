//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestListAdapters_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "adapter-list-success")

	resp := env.doRequest(t, http.MethodGet, "/v1/adapter", nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	adapters := assertJSONField(t, body, "adapters").([]any)
	if len(adapters) != 3 {
		t.Fatalf("expected 3 adapters (mcp,a2a,grpc), got %d", len(adapters))
	}
}

func TestListAdapters_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/adapter", nil, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestAdapterIngest_UnknownAdapter(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "adapter-unknown")

	resp := env.doRequest(t, http.MethodPost, "/v1/adapter/unknown/ingest", map[string]any{"foo": "bar"}, token)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestAdapterIngest_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodPost, "/v1/adapter/mcp/ingest", map[string]any{}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestAdapterIngest_EmptyBody(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "adapter-empty-body")

	resp := env.doRawRequest(t, http.MethodPost, "/v1/adapter/mcp/ingest", []byte{}, token, nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestAdapterIngest_MCP(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "adapter-mcp")

	resp := env.doRequest(t, http.MethodPost, "/v1/adapter/mcp/ingest", map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "demo.action",
			"arguments": map[string]any{"foo": "bar"},
		},
	}, token)
	assertStatus(t, resp, http.StatusAccepted)
}

func TestAdapterIngest_A2A(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "adapter-a2a")

	resp := env.doRequest(t, http.MethodPost, "/v1/adapter/a2a/ingest", map[string]any{
		"method": "task/create",
		"id":     "task-req-1",
		"params": map[string]any{
			"taskId": "task-123",
			"status": "queued",
		},
	}, token)
	assertStatus(t, resp, http.StatusAccepted)
}

func TestAdapterIngest_GRPC(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "adapter-grpc")

	resp := env.doRequest(t, http.MethodPost, "/v1/adapter/grpc/ingest", map[string]any{
		"type":       "unary",
		"service":    "AgentService",
		"method":     "DoThing",
		"request_id": "grpc-req-1",
		"body": map[string]any{
			"input": "value",
		},
	}, token)
	assertStatus(t, resp, http.StatusAccepted)
}
