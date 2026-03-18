//go:build integration

package api

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCreateAgent_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-create-success")

	resp := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": "agent-create-success",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	body := env.readJSON(t, resp)

	assertJSONField(t, body, "id")
	assertJSONField(t, body, "api_secret")
	assertJSONFieldEquals(t, body, "display_name", "agent-create-success")
}

func TestCreateAgent_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": "agent-no-auth",
	}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateAgent_MissingFields(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-create-missing")

	respNoName := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{}, token)
	assertStatus(t, respNoName, http.StatusBadRequest)

	respEmpty := env.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": "",
	}, token)
	assertStatus(t, respEmpty, http.StatusBadRequest)
}

func TestGetAgent_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-get-success")
	agentID, _ := env.createAgent(t, token, "agent-get-success")

	resp := env.doRequest(t, http.MethodGet, "/v1/agents/"+agentID, nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)

	assertJSONFieldEquals(t, body, "id", agentID)
	assertJSONFieldEquals(t, body, "display_name", "agent-get-success")
	assertJSONField(t, body, "owner_user_id")
	assertJSONField(t, body, "created_at")
}

func TestGetAgent_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-get-not-found")

	resp := env.doRequest(t, http.MethodGet, "/v1/agents/"+uuid.NewString(), nil, token)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestGetAgent_CrossOwner(t *testing.T) {
	env := setupTestEnv(t)
	tokenA, _ := registerAndLogin(t, env, "agent-cross-owner-a")
	tokenB, _ := registerAndLogin(t, env, "agent-cross-owner-b")
	agentID, _ := env.createAgent(t, tokenA, "agent-cross-owner")

	resp := env.doRequest(t, http.MethodGet, "/v1/agents/"+agentID, nil, tokenB)
	assertStatus(t, resp, http.StatusForbidden)
}

func TestDeleteAgent_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-delete-success")
	agentID, _ := env.createAgent(t, token, "agent-delete-success")

	resp := env.doRequest(t, http.MethodDelete, "/v1/agents/"+agentID, nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	assertJSONFieldEquals(t, body, "deleted", true)
}

func TestDeleteAgent_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-delete-not-found")

	resp := env.doRequest(t, http.MethodDelete, "/v1/agents/"+uuid.NewString(), nil, token)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestRotateSecret_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "agent-rotate-success")
	agentID, oldSecret := env.createAgent(t, token, "agent-rotate-success")

	resp := env.doRequest(t, http.MethodPost, "/v1/agents/"+agentID+"/rotate-secret", nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	assertJSONFieldEquals(t, body, "id", agentID)
	newSecret, _ := assertJSONField(t, body, "api_secret").(string)
	if newSecret == "" || newSecret == oldSecret {
		t.Fatalf("expected new api secret different from old")
	}
}
