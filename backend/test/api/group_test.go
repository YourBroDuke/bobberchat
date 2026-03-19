//go:build integration

package api

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCreateGroup_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "group-create-success")

	resp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":        "group-create-success",
		"description": "desc",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	body := env.readJSON(t, resp)

	assertJSONField(t, body, "id")
	assertJSONFieldEquals(t, body, "name", "group-create-success")
}

func TestCreateGroup_MissingName(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "group-create-missing")

	resp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"description": "desc",
	}, token)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCreateGroup_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name": "group-no-auth",
	}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLeaveGroup_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "group-leave-success")

	createResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name": "group-leave-success",
	}, token)
	assertStatus(t, createResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, createResp)["id"].(string)

	// Creator is automatically a participant, can leave directly
	leaveResp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/leave", map[string]any{}, token)
	assertStatus(t, leaveResp, http.StatusOK)
	body := env.readJSON(t, leaveResp)
	assertJSONFieldEquals(t, body, "left", true)
}
