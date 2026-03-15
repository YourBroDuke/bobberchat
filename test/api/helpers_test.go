//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bobberchat/bobberchat/internal/persistence"
	"github.com/google/uuid"
)

type testEnv struct {
	baseURL string
	db      *persistence.DB
	client  *http.Client
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BOBBERCHAT_TEST_URL")), "/")
	dsn := strings.TrimSpace(os.Getenv("BOBBERCHAT_TEST_DSN"))
	if baseURL == "" || dsn == "" {
		t.Skip("integration env not configured: both BOBBERCHAT_TEST_URL and BOBBERCHAT_TEST_DSN are required")
	}

	db, err := persistence.NewDB(dsn)
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	e := &testEnv{
		baseURL: baseURL,
		db:      db,
		client:  &http.Client{Timeout: 15 * time.Second},
	}

	e.resetSchema(t)
	e.waitForServer(t)
	t.Cleanup(func() { e.cleanup(t) })

	return e
}

func (e *testEnv) resetSchema(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	_, _ = e.db.Pool().Exec(ctx, `
		DROP TABLE IF EXISTS audit_log, approval_requests, messages_default, messages, topics, chat_group_members, chat_groups, agents, users CASCADE;
		DROP TYPE IF EXISTS participant_type, urgency, approval_status, topic_status, group_visibility, agent_status CASCADE;
	`)

	migrationPath := filepath.Clean("../../migrations/001_initial_schema.sql")
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration %s: %v", migrationPath, err)
	}

	if _, err := e.db.Pool().Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
}

func (e *testEnv) waitForServer(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := e.client.Get(e.baseURL + "/v1/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("server did not become healthy at %s/v1/health: %v", e.baseURL, lastErr)
}

func (e *testEnv) cleanup(t *testing.T) {
	t.Helper()
	if e == nil || e.db == nil {
		return
	}

	ctx := context.Background()
	_, _ = e.db.Pool().Exec(ctx, `
		DROP TABLE IF EXISTS audit_log, approval_requests, messages_default, messages, topics, chat_group_members, chat_groups, agents, users CASCADE;
		DROP TYPE IF EXISTS participant_type, urgency, approval_status, topic_status, group_visibility, agent_status CASCADE;
	`)
	e.db.Close()
}

func (e *testEnv) doRequest(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()
	return e.doRequestWithHeaders(t, method, path, body, token, nil)
}

func (e *testEnv) doRequestWithHeaders(t *testing.T, method, path string, body any, token string, headers map[string]string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, e.baseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}

	return resp
}

func (e *testEnv) doRawRequest(t *testing.T, method, path string, rawBody []byte, token string, headers map[string]string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if rawBody != nil {
		bodyReader = bytes.NewReader(rawBody)
	}

	req, err := http.NewRequest(method, e.baseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request %s %s: %v", method, path, err)
	}
	if rawBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("do raw request %s %s: %v", method, path, err)
	}
	return resp
}

func (e *testEnv) readJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode response json: %v; body=%s", err, string(body))
	}
	return out
}

func (e *testEnv) registerUser(t *testing.T, tenantID, email, password string) map[string]any {
	t.Helper()
	resp := e.doRequest(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"tenant_id": tenantID,
		"email":     email,
		"password":  password,
	}, "")
	assertStatus(t, resp, http.StatusCreated)
	return e.readJSON(t, resp)
}

func (e *testEnv) loginUser(t *testing.T, email, password string) (string, map[string]any) {
	t.Helper()
	resp := e.doRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, "")
	assertStatus(t, resp, http.StatusOK)
	body := e.readJSON(t, resp)

	token, _ := body["access_token"].(string)
	if token == "" {
		t.Fatalf("login response missing access_token")
	}
	user, _ := body["user"].(map[string]any)
	if user == nil {
		t.Fatalf("login response missing user object")
	}

	return token, user
}

func (e *testEnv) createAgent(t *testing.T, token, displayName string, capabilities []string) (agentID, apiSecret string) {
	t.Helper()
	resp := e.doRequest(t, http.MethodPost, "/v1/agents", map[string]any{
		"display_name": displayName,
		"capabilities": capabilities,
		"version":      "1.0.0",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	body := e.readJSON(t, resp)

	agentID, _ = body["agent_id"].(string)
	apiSecret, _ = body["api_secret"].(string)
	if agentID == "" || apiSecret == "" {
		t.Fatalf("create agent response missing agent_id/api_secret: %#v", body)
	}

	return agentID, apiSecret
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode == expected {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	t.Fatalf("unexpected status: got=%d want=%d body=%s", resp.StatusCode, expected, string(body))
}

func assertJSONField(t *testing.T, body map[string]any, field string) any {
	t.Helper()
	v, ok := body[field]
	if !ok {
		t.Fatalf("expected json field %q in body: %#v", field, body)
	}
	return v
}

func assertJSONFieldEquals(t *testing.T, body map[string]any, field string, expected any) {
	t.Helper()
	actual := assertJSONField(t, body, field)
	if fmt.Sprint(actual) != fmt.Sprint(expected) {
		t.Fatalf("json field %q mismatch: got=%v want=%v", field, actual, expected)
	}
}

func newTenantID() string {
	return uuid.NewString()
}

func newEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@example.com", prefix, uuid.NewString())
}

func registerAndLogin(t *testing.T, env *testEnv, tenantID string, prefix string) (string, map[string]any) {
	t.Helper()
	email := newEmail(prefix)
	password := "password-123"
	env.registerUser(t, tenantID, email, password)
	return env.loginUser(t, email, password)
}
