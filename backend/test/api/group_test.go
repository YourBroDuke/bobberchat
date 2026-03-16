//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestCreateGroup_Success(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	token, _ := registerAndLogin(t, env, tenantID, "group-create-success")

	resp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":        "group-create-success",
		"description": "desc",
		"visibility":  "private",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	body := env.readJSON(t, resp)

	assertJSONField(t, body, "id")
	assertJSONFieldEquals(t, body, "name", "group-create-success")
	assertJSONFieldEquals(t, body, "visibility", "private")
	assertJSONFieldEquals(t, body, "tenant_id", tenantID)
}

func TestCreateGroup_MissingName(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "group-create-missing")

	resp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"description": "desc",
		"visibility":  "private",
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

func TestListGroups_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "group-list-success")

	for i := 0; i < 2; i++ {
		resp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
			"name":       "group-list-success-" + newTenantID(),
			"visibility": "private",
		}, token)
		assertStatus(t, resp, http.StatusCreated)
		_ = env.readJSON(t, resp)
	}

	listResp := env.doRequest(t, http.MethodGet, "/v1/groups", nil, token)
	assertStatus(t, listResp, http.StatusOK)
	body := env.readJSON(t, listResp)
	groups := assertJSONField(t, body, "groups").([]any)
	if len(groups) < 2 {
		t.Fatalf("expected at least 2 groups, got %d", len(groups))
	}
}

func TestJoinGroup_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "group-join-success")

	createResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":       "group-join-success",
		"visibility": "private",
	}, token)
	assertStatus(t, createResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, createResp)["id"].(string)

	joinResp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/join", map[string]any{}, token)
	assertStatus(t, joinResp, http.StatusOK)
	body := env.readJSON(t, joinResp)
	assertJSONFieldEquals(t, body, "joined", true)
}

func TestLeaveGroup_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "group-leave-success")

	createResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":       "group-leave-success",
		"visibility": "private",
	}, token)
	assertStatus(t, createResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, createResp)["id"].(string)

	joinResp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/join", map[string]any{}, token)
	assertStatus(t, joinResp, http.StatusOK)
	_ = env.readJSON(t, joinResp)

	leaveResp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/leave", map[string]any{}, token)
	assertStatus(t, leaveResp, http.StatusOK)
	body := env.readJSON(t, leaveResp)
	assertJSONFieldEquals(t, body, "left", true)
}

func TestCreateTopic_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "topic-create-success")

	createGroupResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":       "topic-create-success",
		"visibility": "private",
	}, token)
	assertStatus(t, createGroupResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, createGroupResp)["id"].(string)

	resp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/topics", map[string]any{
		"subject": "topic subject",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	body := env.readJSON(t, resp)
	assertJSONField(t, body, "id")
	assertJSONFieldEquals(t, body, "subject", "topic subject")
}

func TestCreateTopic_MissingSubject(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "topic-create-missing")

	createGroupResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":       "topic-create-missing",
		"visibility": "private",
	}, token)
	assertStatus(t, createGroupResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, createGroupResp)["id"].(string)

	resp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/topics", map[string]any{}, token)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestListTopics_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "topic-list-success")

	createGroupResp := env.doRequest(t, http.MethodPost, "/v1/groups", map[string]any{
		"name":       "topic-list-success",
		"visibility": "private",
	}, token)
	assertStatus(t, createGroupResp, http.StatusCreated)
	groupID, _ := env.readJSON(t, createGroupResp)["id"].(string)

	createTopicResp := env.doRequest(t, http.MethodPost, "/v1/groups/"+groupID+"/topics", map[string]any{
		"subject": "topic list subject",
	}, token)
	assertStatus(t, createTopicResp, http.StatusCreated)
	_ = env.readJSON(t, createTopicResp)

	listResp := env.doRequest(t, http.MethodGet, "/v1/groups/"+groupID+"/topics", nil, token)
	assertStatus(t, listResp, http.StatusOK)
	body := env.readJSON(t, listResp)
	topics := assertJSONField(t, body, "topics").([]any)
	if len(topics) == 0 {
		t.Fatalf("expected at least one topic")
	}
}
