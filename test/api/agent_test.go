//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestCreateAgent_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-create-success")

	resp := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": "agent-create-success",
		"capabilities": []string{"test"},
		"version":      "1.0.0",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	body := env.readJSON(t, resp)

	assertJSONField(t, body, "agent_id")
	assertJSONField(t, body, "api_secret")
	assertJSONFieldEquals(t, body, "status", "REGISTERED")
	assertJSONFieldEquals(t, body, "display_name", "agent-create-success")
}

func TestCreateAgent_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": "agent-no-auth",
		"capabilities": []string{"test"},
		"version":      "1.0.0",
	}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateAgent_MissingFields(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-create-missing")

	respNoName := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"capabilities": []string{"test"},
		"version":      "1.0.0",
	}, token)
	assertStatus(t, respNoName, http.StatusBadRequest)

	respNoVersion := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": "agent-create-missing",
		"capabilities": []string{"test"},
	}, token)
	assertStatus(t, respNoVersion, http.StatusBadRequest)
}

func TestGetAgent_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-get-success")
	agentID, _ := env.createAgent(t, token, "agent-get-success", []string{"cap"})

	resp := env.doRequest(t, http.MethodGet, "/v1/agents/"+agentID, nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)

	assertJSONFieldEquals(t, body, "agent_id", agentID)
	assertJSONField(t, body, "tenant_id")
	assertJSONFieldEquals(t, body, "display_name", "agent-get-success")
	assertJSONField(t, body, "owner_user_id")
	assertJSONField(t, body, "capabilities")
	assertJSONField(t, body, "version")
	assertJSONField(t, body, "status")
	assertJSONField(t, body, "created_at")
}

func TestGetAgent_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-get-not-found")

	resp := env.doRequest(t, http.MethodGet, "/v1/agents/"+newTenantID(), nil, token)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestGetAgent_CrossTenant(t *testing.T) {
	env := setupTestEnv(t)
	tokenA, _ := registerAndLogin(t, env, newTenantID(), "agent-cross-tenant-a")
	tokenB, _ := registerAndLogin(t, env, newTenantID(), "agent-cross-tenant-b")
	agentID, _ := env.createAgent(t, tokenA, "agent-cross-tenant", []string{"cap"})

	resp := env.doRequest(t, http.MethodGet, "/v1/agents/"+agentID, nil, tokenB)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestDeleteAgent_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-delete-success")
	agentID, _ := env.createAgent(t, token, "agent-delete-success", []string{"cap"})

	resp := env.doRequest(t, http.MethodDelete, "/v1/agents/"+agentID, nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	assertJSONFieldEquals(t, body, "deleted", true)
}

func TestDeleteAgent_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-delete-not-found")

	resp := env.doRequest(t, http.MethodDelete, "/v1/agents/"+newTenantID(), nil, token)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestRotateSecret_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-rotate-success")
	agentID, oldSecret := env.createAgent(t, token, "agent-rotate-success", []string{"cap"})

	resp := env.doRequest(t, http.MethodPost, "/v1/agents/"+agentID+"/rotate-secret", map[string]any{
		"grace_period_seconds": 0,
	}, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	assertJSONFieldEquals(t, body, "agent_id", agentID)
	newSecret, _ := assertJSONField(t, body, "api_secret").(string)
	if newSecret == "" || newSecret == oldSecret {
		t.Fatalf("expected new api secret different from old")
	}
}

func TestRotateSecret_GracePeriod(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "agent-rotate-grace")
	agentID, oldSecret := env.createAgent(t, token, "agent-rotate-grace", []string{"cap"})

	groupResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":       "group-for-agent-grace",
		"visibility": "private",
	}, token)
	assertStatus(t, groupResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, groupResp)["id"].(string)
	if groupID == "" {
		t.Fatalf("expected group id")
	}

	rotateResp := env.doRequest(t, http.MethodPost, "/v1/agents/"+agentID+"/rotate-secret", map[string]any{
		"grace_period_seconds": 60,
	}, token)
	assertStatus(t, rotateResp, http.StatusOK)

	joinResp := env.doRequestWithHeaders(t, http.MethodPost, "/v1/groups/"+groupID+"/join", map[string]any{}, "", map[string]string{
		"X-Agent-ID":   agentID,
		"X-API-Secret": oldSecret,
	})
	assertStatus(t, joinResp, http.StatusOK)
	body := env.readJSON(t, joinResp)
	assertJSONFieldEquals(t, body, "joined", true)
}
