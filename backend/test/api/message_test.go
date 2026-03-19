//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestReplayMessage_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "messages-replay-not-found")

	resp := env.doRequest(t, http.MethodPost, "/v1/messages/00000000-0000-0000-0000-000000000000/replay", map[string]any{
		"reason": "replay test",
	}, token)
	assertStatus(t, resp, http.StatusNotFound)
}
