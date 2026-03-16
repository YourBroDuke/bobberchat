//go:build integration

package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHealth_Success(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/health", nil, "")
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	assertJSONFieldEquals(t, body, "status", "ok")
	assertJSONField(t, body, "version")
	assertJSONField(t, body, "time")
}

func TestMetrics_Success(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/metrics", nil, "")
	assertStatus(t, resp, http.StatusOK)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "bobberchat_") {
		t.Fatalf("expected prometheus metric names in response")
	}
}

func TestWebSocket_NoToken(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/ws/connect", nil, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}
