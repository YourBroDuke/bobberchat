//go:build integration

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGetMessagesByTraceID_Success(t *testing.T) {
	env := setupTestEnv(t)
	token, user := registerAndLogin(t, env, "messages-by-trace-success")
	uid := uuid.MustParse(user["id"].(string))
	traceID := uuid.New()

	_, err := env.db.Pool().Exec(context.Background(), `
		INSERT INTO messages (id, from_id, to_id, tag, payload, metadata, "timestamp", trace_id, topic_id)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,$8,$9)
	`, uuid.New(), uid, uid, "request.action", `{"hello":"world"}`, `{}`, time.Now().UTC(), traceID, nil)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}

	resp := env.doRequest(t, http.MethodGet, "/v1/messages?trace_id="+traceID.String(), nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	messages := assertJSONField(t, body, "messages").([]any)
	if len(messages) == 0 {
		t.Fatalf("expected at least one message")
	}
}

func TestGetMessagesByTraceID_MissingParam(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "messages-by-trace-missing")

	resp := env.doRequest(t, http.MethodGet, "/v1/messages", nil, token)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestGetMessagesByTraceID_InvalidTraceID(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "messages-by-trace-invalid")

	resp := env.doRequest(t, http.MethodGet, "/v1/messages?trace_id=not-a-uuid", nil, token)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestGetMessagesByTraceID_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/messages?trace_id="+uuid.NewString(), nil, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestReplayMessage_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, "messages-replay-not-found")

	resp := env.doRequest(t, http.MethodPost, "/v1/messages/"+uuid.NewString()+"/replay", map[string]any{
		"reason": "replay test",
	}, token)
	assertStatus(t, resp, http.StatusNotFound)
}
