//go:build integration

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func seedPendingApproval(t *testing.T, env *testEnv, tenantID string, approverUserID string) string {
	t.Helper()

	tid := uuid.MustParse(tenantID)
	approverID := uuid.MustParse(approverUserID)
	ownerID := uuid.New()
	agentID := uuid.New()
	approvalID := uuid.New()

	_, err := env.db.Pool().Exec(context.Background(), `
		INSERT INTO users (id, tenant_id, email, password_hash, role, created_at)
		VALUES ($1,$2,$3,$4,$5,NOW())
	`, ownerID, tid, newEmail("approval-owner"), "hash", "member")
	if err != nil {
		t.Fatalf("insert owner user: %v", err)
	}

	_, err = env.db.Pool().Exec(context.Background(), `
		INSERT INTO agents (agent_id, tenant_id, display_name, owner_user_id, capabilities, version, status, api_secret_hash, created_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,NOW())
	`, agentID, tid, "approval-agent", ownerID, `[]`, "1.0.0", "REGISTERED", "hash")
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	_, err = env.db.Pool().Exec(context.Background(), `
		INSERT INTO approval_requests (approval_id, tenant_id, agent_id, action, justification, urgency, status, approver_id, decided_at, timeout_ms, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
	`, approvalID, tid, agentID, "deploy", "needs approval", "medium", "PENDING", nil, nil, 60000)
	if err != nil {
		t.Fatalf("insert approval request: %v", err)
	}

	_ = approverID
	return approvalID.String()
}

func TestGetPendingApprovals_Success(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	token, user := registerAndLogin(t, env, tenantID, "approvals-pending-success")
	seedPendingApproval(t, env, tenantID, user["id"].(string))

	resp := env.doRequest(t, http.MethodGet, "/v1/approvals/pending", nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	items := assertJSONField(t, body, "approvals").([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
}

func TestGetPendingApprovals_Empty(t *testing.T) {
	env := setupTestEnv(t)
	token, _ := registerAndLogin(t, env, newTenantID(), "approvals-pending-empty")

	resp := env.doRequest(t, http.MethodGet, "/v1/approvals/pending", nil, token)
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)
	items := assertJSONField(t, body, "approvals").([]any)
	if len(items) != 0 {
		t.Fatalf("expected 0 approvals, got %d", len(items))
	}
}

func TestGetPendingApprovals_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	resp := env.doRequest(t, http.MethodGet, "/v1/approvals/pending", nil, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestDecideApproval_Success(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	token, user := registerAndLogin(t, env, tenantID, "approvals-decide-success")
	approvalID := seedPendingApproval(t, env, tenantID, user["id"].(string))

	resp := env.doRequest(t, http.MethodPost, "/v1/approvals/"+approvalID+"/decide", map[string]any{
		"decision": "GRANTED",
		"reason":   "approved",
	}, token)
	assertStatus(t, resp, http.StatusOK)
}

func TestDecideApproval_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	_, user := registerAndLogin(t, env, tenantID, "approvals-decide-noauth")
	approvalID := seedPendingApproval(t, env, tenantID, user["id"].(string))

	resp := env.doRequest(t, http.MethodPost, "/v1/approvals/"+approvalID+"/decide", map[string]any{
		"decision": "GRANTED",
		"reason":   "approved",
	}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}
