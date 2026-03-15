//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestRegister_Success(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	email := newEmail("register-success")

	resp := env.doRequest(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"tenant_id": tenantID,
		"email":     email,
		"password":  "password-123",
	}, "")
	assertStatus(t, resp, http.StatusCreated)
	body := env.readJSON(t, resp)

	assertJSONField(t, body, "id")
	assertJSONFieldEquals(t, body, "tenant_id", tenantID)
	assertJSONFieldEquals(t, body, "email", email)
	assertJSONFieldEquals(t, body, "role", "member")
	assertJSONField(t, body, "created_at")
}

func TestRegister_DuplicateEmail(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	email := newEmail("register-dup")

	env.registerUser(t, tenantID, email, "password-123")

	resp := env.doRequest(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"tenant_id": tenantID,
		"email":     email,
		"password":  "password-123",
	}, "")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestRegister_MissingFields(t *testing.T) {
	env := setupTestEnv(t)

	respEmail := env.doRequest(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"tenant_id": newTenantID(),
		"password":  "password-123",
	}, "")
	assertStatus(t, respEmail, http.StatusBadRequest)

	respPassword := env.doRequest(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"tenant_id": newTenantID(),
		"email":     newEmail("register-missing-password"),
	}, "")
	assertStatus(t, respPassword, http.StatusBadRequest)
}

func TestRegister_InvalidTenantID(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"tenant_id": "not-a-uuid",
		"email":     newEmail("register-invalid-tenant"),
		"password":  "password-123",
	}, "")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestLogin_Success(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	email := newEmail("login-success")
	password := "password-123"
	env.registerUser(t, tenantID, email, password)

	resp := env.doRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, "")
	assertStatus(t, resp, http.StatusOK)
	body := env.readJSON(t, resp)

	assertJSONField(t, body, "access_token")
	assertJSONFieldEquals(t, body, "token_type", "Bearer")
	assertJSONField(t, body, "expires_in")
	user := assertJSONField(t, body, "user").(map[string]any)
	assertJSONFieldEquals(t, user, "tenant_id", tenantID)
	assertJSONFieldEquals(t, user, "email", email)
}

func TestLogin_WrongPassword(t *testing.T) {
	env := setupTestEnv(t)
	tenantID := newTenantID()
	email := newEmail("login-wrong-password")
	env.registerUser(t, tenantID, email, "password-123")

	resp := env.doRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": "wrong-password",
	}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLogin_NonExistentUser(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    newEmail("login-missing-user"),
		"password": "password-123",
	}, "")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLogin_MissingFields(t *testing.T) {
	env := setupTestEnv(t)

	respEmail := env.doRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{
		"password": "password-123",
	}, "")
	assertStatus(t, respEmail, http.StatusUnauthorized)

	respPassword := env.doRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{
		"email": newEmail("login-missing-password"),
	}, "")
	assertStatus(t, respPassword, http.StatusUnauthorized)
}
