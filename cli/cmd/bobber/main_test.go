package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func testConfig(backendURL, token string) *cliConfig {
	v := viper.New()
	v.Set("backend_url", backendURL)
	v.Set("token", token)
	return &cliConfig{v: v}
}

func testAgentConfig(backendURL, agentID, apiSecret string) *cliConfig {
	v := viper.New()
	v.Set("backend_url", backendURL)
	v.Set("agent_id", agentID)
	v.Set("api_secret", apiSecret)
	return &cliConfig{v: v}
}

func buildRootCmdForTest(cfg *cliConfig) *cobra.Command {
	root := &cobra.Command{
		Use:   "bobber",
		Short: "BobberChat CLI",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			_ = cfg.v.BindPFlag("backend_url", cmd.Flags().Lookup("backend-url"))
			_ = cfg.v.BindPFlag("token", cmd.Flags().Lookup("token"))
		},
	}
	root.PersistentFlags().String("backend-url", cfg.v.GetString("backend_url"), "backend url")
	root.PersistentFlags().String("token", cfg.v.GetString("token"), "jwt token")
	root.AddCommand(
		accountCmd(cfg),
		agentCmd(cfg),
		loginCmd(cfg),
		whoamiCmd(cfg),
		logoutCmd(cfg),
		lsCmd(cfg),
		connectCmd(cfg),
		inboxCmd(cfg),
		acceptCmd(cfg),
		rejectCmd(cfg),
		blacklistCmd(cfg),
		infoCmd(cfg),
		sendCmd(cfg),
		pollCmd(cfg),
		groupCmd(cfg),
	)
	return root
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}
	os.Stdout = orig

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe failed: %v", err)
	}
	return string(b)
}

func mustJSONMap(t *testing.T, s string) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("unmarshal json failed: %v, body=%q", err, s)
	}
	return out
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "basic split", in: "a,b,c", want: []string{"a", "b", "c"}},
		{name: "whitespace trimming", in: " a, b ,c ", want: []string{"a", "b", "c"}},
		{name: "empty string", in: "", want: []string{}},
		{name: "trailing comma", in: "a,b,", want: []string{"a", "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitCSV(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got=%v want=%v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("index %d mismatch: got=%q want=%q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDefaultConfigFile(t *testing.T) {
	got := defaultConfigFile()
	if !strings.HasSuffix(filepath.ToSlash(got), "bobber/config.yaml") && got != ".bobber.yaml" {
		t.Fatalf("unexpected default config path: %q", got)
	}
}

func TestCLIConfigBackendURL(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{name: "default value", val: "", want: "http://localhost:8080"},
		{name: "override value", val: "http://example:9000", want: "http://example:9000"},
		{name: "whitespace-only falls back to default", val: "   \t ", want: "http://localhost:8080"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(tc.val, "")
			if got := cfg.backendURL(); got != tc.want {
				t.Fatalf("backendURL mismatch: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestCLIConfigToken(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{name: "empty when not set", val: "", want: ""},
		{name: "trimmed whitespace", val: "  abc123  ", want: "abc123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig("http://localhost:8080", tc.val)
			if got := cfg.token(); got != tc.want {
				t.Fatalf("token mismatch: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestCLIConfigAPISecret(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{name: "empty when not set", val: "", want: ""},
		{name: "trimmed whitespace", val: "  secret123  ", want: "secret123"},
		{name: "exact value", val: "my-secret", want: "my-secret"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testAgentConfig("http://localhost:8080", "agent-1", tc.val)
			if got := cfg.apiSecret(); got != tc.want {
				t.Fatalf("apiSecret mismatch: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestPrettyPrint(t *testing.T) {
	out := captureStdout(t, func() {
		prettyPrint(map[string]any{"a": 1, "b": map[string]any{"c": true}})
	})
	if !strings.Contains(out, "\n  \"a\": 1") {
		t.Fatalf("expected 2-space indented json, got: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got: %q", out)
	}
}

func TestUUIDString(t *testing.T) {
	a := uuidString()
	b := uuidString()

	u1, err := uuid.Parse(a)
	if err != nil {
		t.Fatalf("first uuid parse failed: %v", err)
	}
	if u1.Version() != 4 {
		t.Fatalf("expected uuid v4, got v%d", u1.Version())
	}
	if _, err := uuid.Parse(b); err != nil {
		t.Fatalf("second uuid parse failed: %v", err)
	}
	if a == b {
		t.Fatalf("expected two uuid calls to differ, got same value %q", a)
	}
}

func TestDoJSON(t *testing.T) {
	t.Run("GET success: correct method, returns parsed JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method mismatch: got=%s want=GET", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		resp, err := doJSON(http.MethodGet, srv.URL+"/x", "", nil)
		if err != nil {
			t.Fatalf("doJSON failed: %v", err)
		}
		if got, _ := resp["ok"].(bool); !got {
			t.Fatalf("expected ok=true, got %v", resp["ok"])
		}
	})

	t.Run("POST success: sends JSON body, receives parsed response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method mismatch: got=%s want=POST", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			got := mustJSONMap(t, string(b))
			if got["name"] != "bob" {
				t.Fatalf("unexpected body: %v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "1"})
		}))
		defer srv.Close()

		resp, err := doJSON(http.MethodPost, srv.URL+"/create", "", map[string]any{"name": "bob"})
		if err != nil {
			t.Fatalf("doJSON failed: %v", err)
		}
		if resp["id"] != "1" {
			t.Fatalf("unexpected response: %v", resp)
		}
	})

	t.Run("Auth header present when token non-empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
				t.Fatalf("auth header mismatch: %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		_, err := doJSON(http.MethodGet, srv.URL, "tok123", nil)
		if err != nil {
			t.Fatalf("doJSON failed: %v", err)
		}
	})

	t.Run("No auth header when token is empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("expected no auth header, got=%q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		_, err := doJSON(http.MethodGet, srv.URL, "", nil)
		if err != nil {
			t.Fatalf("doJSON failed: %v", err)
		}
	})

	t.Run("4xx with error field returns that message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "msg"})
		}))
		defer srv.Close()

		_, err := doJSON(http.MethodGet, srv.URL, "", nil)
		if err == nil || err.Error() != "msg" {
			t.Fatalf("expected error msg, got %v", err)
		}
	})

	t.Run("4xx without error field returns status message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"oops": "x"})
		}))
		defer srv.Close()

		_, err := doJSON(http.MethodGet, srv.URL, "", nil)
		if err == nil || !strings.Contains(err.Error(), "request failed with status 400") {
			t.Fatalf("expected status error, got %v", err)
		}
	})

	t.Run("Non-JSON response returns decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not json"))
		}))
		defer srv.Close()

		_, err := doJSON(http.MethodGet, srv.URL, "", nil)
		if err == nil {
			t.Fatal("expected decode error, got nil")
		}
	})

	t.Run("Connection refused returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL
		srv.Close()

		_, err := doJSON(http.MethodGet, url, "", nil)
		if err == nil {
			t.Fatal("expected connection error, got nil")
		}
	})

	t.Run("nil body sends empty request body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if len(b) != 0 {
				t.Fatalf("expected empty body, got=%q", string(b))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		_, err := doJSON(http.MethodPost, srv.URL, "", nil)
		if err != nil {
			t.Fatalf("doJSON failed: %v", err)
		}
	})

	t.Run("Server timeout returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(16 * time.Second)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		start := time.Now()
		_, err := doJSON(http.MethodGet, srv.URL, "", nil)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if time.Since(start) < 14*time.Second {
			t.Fatalf("expected call to run near client timeout, elapsed=%v", time.Since(start))
		}
	})
}

func TestDoJSONAgent(t *testing.T) {
	t.Run("GET success: correct method, returns parsed JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method mismatch: got=%s want=GET", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		resp, err := doJSONAgent(http.MethodGet, srv.URL+"/x", "a1", "s1", nil)
		if err != nil {
			t.Fatalf("doJSONAgent failed: %v", err)
		}
		if got, _ := resp["ok"].(bool); !got {
			t.Fatalf("expected ok=true, got %v", resp["ok"])
		}
	})

	t.Run("POST success: sends JSON body, receives parsed response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method mismatch: got=%s want=POST", r.Method)
			}
			b, _ := io.ReadAll(r.Body)
			got := mustJSONMap(t, string(b))
			if got["name"] != "bob" {
				t.Fatalf("unexpected body: %v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "1"})
		}))
		defer srv.Close()

		resp, err := doJSONAgent(http.MethodPost, srv.URL+"/create", "a1", "s1", map[string]any{"name": "bob"})
		if err != nil {
			t.Fatalf("doJSONAgent failed: %v", err)
		}
		if resp["id"] != "1" {
			t.Fatalf("unexpected response: %v", resp)
		}
	})

	t.Run("Agent headers present", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-Agent-ID"); got != "agent-42" {
				t.Fatalf("X-Agent-ID mismatch: %q", got)
			}
			if got := r.Header.Get("X-API-Secret"); got != "supersecret" {
				t.Fatalf("X-API-Secret mismatch: %q", got)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type mismatch: %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		_, err := doJSONAgent(http.MethodGet, srv.URL, "agent-42", "supersecret", nil)
		if err != nil {
			t.Fatalf("doJSONAgent failed: %v", err)
		}
	})

	t.Run("No Authorization header present", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("expected no Authorization header, got=%q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		_, err := doJSONAgent(http.MethodGet, srv.URL, "a1", "s1", nil)
		if err != nil {
			t.Fatalf("doJSONAgent failed: %v", err)
		}
	})

	t.Run("4xx with error field returns that message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid secret"})
		}))
		defer srv.Close()

		_, err := doJSONAgent(http.MethodGet, srv.URL, "a1", "bad", nil)
		if err == nil || err.Error() != "invalid secret" {
			t.Fatalf("expected error 'invalid secret', got %v", err)
		}
	})

	t.Run("4xx without error field returns status message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"oops": "x"})
		}))
		defer srv.Close()

		_, err := doJSONAgent(http.MethodGet, srv.URL, "a1", "s1", nil)
		if err == nil || !strings.Contains(err.Error(), "request failed with status 401") {
			t.Fatalf("expected status error, got %v", err)
		}
	})

	t.Run("Connection refused returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL
		srv.Close()

		_, err := doJSONAgent(http.MethodGet, url, "a1", "s1", nil)
		if err == nil {
			t.Fatal("expected connection error, got nil")
		}
	})

	t.Run("nil body sends empty request body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if len(b) != 0 {
				t.Fatalf("expected empty body, got=%q", string(b))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		_, err := doJSONAgent(http.MethodPost, srv.URL, "a1", "s1", nil)
		if err != nil {
			t.Fatalf("doJSONAgent failed: %v", err)
		}
	})
}

func TestClearAgentCreds(t *testing.T) {
	tmp := t.TempDir()
	cfg := testAgentConfig("http://localhost:8080", "agent-1", "secret-1")
	cfg.v.Set("token", "has-token")
	cfg.v.SetConfigFile(filepath.Join(tmp, "config.yaml"))

	if err := clearAgentCreds(cfg); err != nil {
		t.Fatalf("clearAgentCreds failed: %v", err)
	}
	if got := cfg.token(); got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
	if got := cfg.agentID(); got != "" {
		t.Fatalf("expected empty agent_id, got %q", got)
	}
	if got := cfg.apiSecret(); got != "" {
		t.Fatalf("expected empty api_secret, got %q", got)
	}
}

func TestAccountRegister(t *testing.T) {
	t.Run("Success: correct payload to /v1/auth/register", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/auth/register" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := accountRegisterCmd(testConfig(srv.URL, ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if got["email"] != "u@example.com" || got["password"] != "pw" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("Missing email", func(t *testing.T) {
		cmd := accountRegisterCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "", "--password", "pw"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--email and --password are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Missing password", func(t *testing.T) {
		cmd := accountRegisterCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", ""})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--email and --password are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Backend error: propagated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "bad request"})
		}))
		defer srv.Close()

		cmd := accountRegisterCmd(testConfig(srv.URL, ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "bad request") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestAccountLogin(t *testing.T) {
	t.Run("Success: sends email/password to /v1/auth/login, gets token", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/auth/login" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "abc"})
		}))
		defer srv.Close()

		tmp := t.TempDir()
		cfg := testConfig(srv.URL, "")
		cfg.v.SetConfigFile(filepath.Join(tmp, "config.yaml"))

		cmd := accountLoginCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["email"] != "u@example.com" || got["password"] != "pw" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("Token persistence: access_token written to config", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "persist-me"})
		}))
		defer srv.Close()

		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "bobber", "config.yaml")
		cfg := testConfig(srv.URL, "")
		cfg.v.SetConfigFile(cfgPath)

		cmd := accountLoginCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		b, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("reading config failed: %v", err)
		}
		if !strings.Contains(string(b), "persist-me") {
			t.Fatalf("expected token in config file, got: %s", string(b))
		}
	})

	t.Run("Missing email", func(t *testing.T) {
		cmd := accountLoginCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "", "--password", "pw"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--email and --password are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Missing password", func(t *testing.T) {
		cmd := accountLoginCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", ""})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--email and --password are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Wrong credentials: 401 propagated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid credentials"})
		}))
		defer srv.Close()

		cmd := accountLoginCmd(testConfig(srv.URL, ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "bad"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid credentials") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("No token in response: no crash", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "config.yaml")
		cfg := testConfig(srv.URL, "")
		cfg.v.SetConfigFile(cfgPath)

		cmd := accountLoginCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
			t.Fatalf("expected no config write, stat err=%v", err)
		}
	})
}

func TestAgentCreate(t *testing.T) {
	t.Run("Success with explicit name", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/agents" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a1"})
		}))
		defer srv.Close()

		cmd := agentCreateCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--name", "agent-x"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if got["display_name"] != "agent-x" || got["version"] != "1.0.0" {
			t.Fatalf("unexpected payload: %v", got)
		}
		caps, ok := got["capabilities"].([]any)
		if !ok || len(caps) != 0 {
			t.Fatalf("expected empty capabilities array, got: %v", got["capabilities"])
		}
	})

	t.Run("Success with default name", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a1"})
		}))
		defer srv.Close()

		cmd := agentCreateCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		name, _ := got["display_name"].(string)
		if _, err := uuid.Parse(name); err != nil {
			t.Fatalf("display_name should be UUID, got %q (%v)", name, err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := agentCreateCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestAgentSubcommands(t *testing.T) {
	t.Run("agent create via parent command", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/agents" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a1"})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"create", "--name", "agent-x"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["display_name"] != "agent-x" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("agent rotate-secret: success", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/agents/a1/rotate-secret" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"rotate-secret", "a1", "--grace-period", "30"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["grace_period_seconds"].(float64) != 30 {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("agent rotate-secret: grace period default is 0", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"rotate-secret", "a1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["grace_period_seconds"].(float64) != 0 {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("agent rotate-secret: no token", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"rotate-secret", "a1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("agent delete: success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete || r.URL.Path != "/v1/agents/a1" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"delete", "a1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("agent delete: no token", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"delete", "a1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("agent use: saves agent_id", func(t *testing.T) {
		tmp := t.TempDir()
		cfg := testConfig("http://localhost:8080", "tok")
		cfg.v.SetConfigFile(filepath.Join(tmp, "config.yaml"))

		cmd := agentCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"use", "a1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got := cfg.agentID(); got != "a1" {
			t.Fatalf("expected saved agent_id, got %q", got)
		}
	})
}

func TestLoginCommand(t *testing.T) {
	t.Run("Success: saves agent_id and api_secret to config", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "config.yaml")
		cfg := testConfig("http://localhost:8080", "")
		cfg.v.SetConfigFile(cfgPath)

		cmd := loginCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--agent-id", "agent-1", "--secret", "s3cret"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got := cfg.agentID(); got != "agent-1" {
			t.Fatalf("expected saved agent_id, got %q", got)
		}
		if got := cfg.apiSecret(); got != "s3cret" {
			t.Fatalf("expected saved api_secret, got %q", got)
		}
	})

	t.Run("Missing agent-id", func(t *testing.T) {
		cmd := loginCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--agent-id", "", "--secret", "s3cret"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--agent-id is required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Missing secret", func(t *testing.T) {
		cmd := loginCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--agent-id", "agent-1", "--secret", ""})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--secret is required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Credentials persisted to config file", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "bobber", "config.yaml")
		cfg := testConfig("http://localhost:8080", "")
		cfg.v.SetConfigFile(cfgPath)

		cmd := loginCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--agent-id", "persist-agent", "--secret", "persist-secret"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		b, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("reading config failed: %v", err)
		}
		s := string(b)
		if !strings.Contains(s, "persist-agent") {
			t.Fatalf("expected agent_id in config file, got: %s", s)
		}
		if !strings.Contains(s, "persist-secret") {
			t.Fatalf("expected api_secret in config file, got: %s", s)
		}
	})
}

func TestWhoamiCommand(t *testing.T) {
	t.Run("Success: GET /v1/agents/{id} with agent secret headers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/agents/agent-1" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			if got := r.Header.Get("X-Agent-ID"); got != "agent-1" {
				t.Fatalf("expected X-Agent-ID header, got %q", got)
			}
			if got := r.Header.Get("X-API-Secret"); got != "s3cret" {
				t.Fatalf("expected X-API-Secret header, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "agent-1", "display_name": "test-agent"})
		}))
		defer srv.Close()

		cmd := whoamiCmd(testAgentConfig(srv.URL, "agent-1", "s3cret"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("No agent credentials", func(t *testing.T) {
		cmd := whoamiCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "not logged in as agent") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Missing api_secret only", func(t *testing.T) {
		cfg := testConfig("http://localhost:8080", "")
		cfg.v.Set("agent_id", "agent-1")
		cmd := whoamiCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "not logged in as agent") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Backend error propagated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid agent secret"})
		}))
		defer srv.Close()

		cmd := whoamiCmd(testAgentConfig(srv.URL, "agent-1", "bad-secret"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid agent secret") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestLogoutCommand(t *testing.T) {
	tmp := t.TempDir()
	cfg := testAgentConfig("http://localhost:8080", "agent-1", "s3cret")
	cfg.v.Set("token", "some-token")
	cfg.v.SetConfigFile(filepath.Join(tmp, "config.yaml"))

	cmd := logoutCmd(cfg)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if got := cfg.agentID(); got != "" {
		t.Fatalf("expected agent_id cleared, got %q", got)
	}
	if got := cfg.apiSecret(); got != "" {
		t.Fatalf("expected api_secret cleared, got %q", got)
	}
	if got := cfg.token(); got != "" {
		t.Fatalf("expected token cleared, got %q", got)
	}
}

func TestLsCommand(t *testing.T) {
	t.Run("Default (no arg): GET /v1/registry/agents", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/registry/agents" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []any{}})
		}))
		defer srv.Close()

		cmd := lsCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("ls users: GET /v1/registry/agents", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/registry/agents" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []any{}})
		}))
		defer srv.Close()

		cmd := lsCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"users"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("ls groups: GET /v1/groups", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/groups" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"groups": []any{}})
		}))
		defer srv.Close()

		cmd := lsCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"groups"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("Invalid arg", func(t *testing.T) {
		cmd := lsCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"nope"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid list target") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := lsCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestConnectCommand(t *testing.T) {
	t.Run("Success: POST /v1/connections/request", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/connections/request" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := connectCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if got["target_id"] != "user-id" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := connectCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestInboxCommand(t *testing.T) {
	t.Run("Success: GET /v1/connections/inbox", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/connections/inbox" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		}))
		defer srv.Close()

		cmd := inboxCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := inboxCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestAcceptCommand(t *testing.T) {
	t.Run("Success: POST /v1/connections/{id}/accept", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/connections/user-id/accept" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := acceptCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if len(got) != 0 {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := acceptCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestRejectCommand(t *testing.T) {
	t.Run("Success: POST /v1/connections/{id}/reject", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/connections/user-id/reject" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := rejectCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if len(got) != 0 {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := rejectCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestBlacklistCommand(t *testing.T) {
	t.Run("Success: POST /v1/blacklist", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/blacklist" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := blacklistCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if got["target_id"] != "user-id" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := blacklistCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestPollCommand(t *testing.T) {
	t.Run("Success: GET /v1/messages/poll?peer_id={id}", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/messages/poll" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			if got := r.URL.Query().Get("peer_id"); got != "user-id" {
				t.Fatalf("unexpected peer_id query: %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"messages": []any{}})
		}))
		defer srv.Close()

		cmd := pollCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := pollCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"user-id"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestGroupInviteCommand(t *testing.T) {
	t.Run("Success: POST /v1/groups/{id}/join", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/groups/group-id/join" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := groupCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"invite", "group-id", "user-id"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if got["participant_id"] != "user-id" || got["participant_kind"] != "user" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := groupCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"invite", "group-id", "user-id"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestInfoCommand(t *testing.T) {
	t.Run("Success: GET /v1/agents/{id}", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/agents/a1" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a1"})
		}))
		defer srv.Close()

		cmd := infoCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := infoCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestSendCommand(t *testing.T) {
	t.Run("Success: connects WS and sends envelope", func(t *testing.T) {
		received := make(chan map[string]any, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/ws/connect" {
				t.Fatalf("unexpected ws path: %s", r.URL.Path)
			}
			if r.URL.Query().Get("token") != "tok" {
				t.Fatalf("expected token query param, got %q", r.URL.Query().Get("token"))
			}
			up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade failed: %v", err)
			}
			defer c.Close()
			msg := map[string]any{}
			if err := c.ReadJSON(&msg); err != nil {
				t.Fatalf("read json failed: %v", err)
			}
			received <- msg
		}))
		defer srv.Close()

		cmd := sendCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a-target", "--tag", "request.data", "--content", "hello"})
		out := captureStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute failed: %v", err)
			}
		})
		if !strings.Contains(out, `"sent": true`) {
			t.Fatalf("expected sent output, got: %q", out)
		}

		select {
		case env := <-received:
			if env["from"] != "" || env["to"] != "a-target" || env["tag"] != "request.data" {
				t.Fatalf("unexpected envelope: %v", env)
			}
			payload, ok := env["payload"].(map[string]any)
			if !ok || payload["content"] != "hello" {
				t.Fatalf("unexpected payload: %v", env["payload"])
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for websocket envelope")
		}
	})

	t.Run("Missing tag", func(t *testing.T) {
		cmd := sendCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a-target", "--tag", "", "--content", "hello"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--tag and --content are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Missing content", func(t *testing.T) {
		cmd := sendCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a-target", "--tag", "request.data", "--content", ""})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--tag and --content are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := sendCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a-target", "--tag", "request.data", "--content", "hello"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Connection failed", func(t *testing.T) {
		cmd := sendCmd(testConfig("http://127.0.0.1:1", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a-target", "--tag", "request.data", "--content", "hello"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected dial error, got nil")
		}
	})

	t.Run("Envelope fields: valid UUIDs and RFC3339 timestamp", func(t *testing.T) {
		received := make(chan map[string]any, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade failed: %v", err)
			}
			defer c.Close()
			msg := map[string]any{}
			if err := c.ReadJSON(&msg); err != nil {
				t.Fatalf("read json failed: %v", err)
			}
			received <- msg
		}))
		defer srv.Close()

		cmd := sendCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"a-target", "--tag", "request.data", "--content", "hello"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		select {
		case env := <-received:
			id, _ := env["id"].(string)
			trace, _ := env["trace_id"].(string)
			ts, _ := env["timestamp"].(string)
			if _, err := uuid.Parse(id); err != nil {
				t.Fatalf("invalid id uuid: %q (%v)", id, err)
			}
			if _, err := uuid.Parse(trace); err != nil {
				t.Fatalf("invalid trace_id uuid: %q (%v)", trace, err)
			}
			if _, err := time.Parse(time.RFC3339, ts); err != nil {
				t.Fatalf("invalid timestamp: %q (%v)", ts, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for envelope")
		}
	})
}

func TestGroupCreate(t *testing.T) {
	t.Run("Success: POST /v1/groups with name and visibility public", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/groups" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "g1"})
		}))
		defer srv.Close()

		cmd := groupCreateCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--name", "dev-group"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["name"] != "dev-group" || got["visibility"] != "public" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("Missing name", func(t *testing.T) {
		cmd := groupCreateCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--name", ""})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--name is required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := groupCreateCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--name", "dev-group"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestGroupLeave(t *testing.T) {
	t.Run("Success: POST /v1/groups/{id}/leave", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/groups/g1/leave" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := groupLeaveCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"g1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["participant_id"] != "" || got["participant_kind"] != "user" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := groupLeaveCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"g1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestConfigAndFlagPrecedence(t *testing.T) {
	t.Run("Env var overrides default backend_url", func(t *testing.T) {
		t.Setenv("BOBBER_BACKEND_URL", "http://env.example:9090")
		v := viper.New()
		v.SetEnvPrefix("BOBBER")
		v.AutomaticEnv()
		v.SetDefault("backend_url", "http://localhost:8080")
		cfg := &cliConfig{v: v}
		if got := cfg.backendURL(); got != "http://env.example:9090" {
			t.Fatalf("expected env backend url, got %q", got)
		}
	})

	t.Run("Flag overrides env var", func(t *testing.T) {
		t.Setenv("BOBBER_BACKEND_URL", "http://env.example:9090")
		v := viper.New()
		v.SetEnvPrefix("BOBBER")
		v.AutomaticEnv()
		v.SetDefault("backend_url", "http://localhost:8080")
		v.SetDefault("token", "")
		cfg := &cliConfig{v: v}

		var got string
		root := &cobra.Command{
			Use: "bobber",
			PersistentPreRun: func(cmd *cobra.Command, _ []string) {
				_ = cfg.v.BindPFlag("backend_url", cmd.Flags().Lookup("backend-url"))
			},
			RunE: func(_ *cobra.Command, _ []string) error {
				got = cfg.backendURL()
				return nil
			},
		}
		root.PersistentFlags().String("backend-url", v.GetString("backend_url"), "backend url")
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"--backend-url", "http://flag.example:7777"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got != "http://flag.example:7777" {
			t.Fatalf("expected flag value, got %q", got)
		}
	})

	t.Run("Token from env", func(t *testing.T) {
		t.Setenv("BOBBER_TOKEN", "env-token")
		v := viper.New()
		v.SetEnvPrefix("BOBBER")
		v.AutomaticEnv()
		v.SetDefault("token", "")
		cfg := &cliConfig{v: v}
		if got := cfg.token(); got != "env-token" {
			t.Fatalf("expected env token, got %q", got)
		}
	})

	t.Run("Config file values applied", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "config.yaml")
		content := []byte("backend_url: http://from-file:8081\ntoken: file-token\n")
		if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}

		v := viper.New()
		v.SetConfigFile(cfgPath)
		if err := v.ReadInConfig(); err != nil {
			t.Fatalf("read config failed: %v", err)
		}
		cfg := &cliConfig{v: v}
		if got := cfg.backendURL(); got != "http://from-file:8081" {
			t.Fatalf("backend mismatch: %q", got)
		}
		if got := cfg.token(); got != "file-token" {
			t.Fatalf("token mismatch: %q", got)
		}
	})

	t.Run("saveConfig creates parent directories", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "a", "b", "config.yaml")
		v := viper.New()
		v.SetConfigFile(cfgPath)
		v.Set("token", "tok")
		if err := saveConfig(v); err != nil {
			t.Fatalf("saveConfig failed: %v", err)
		}
		if _, err := os.Stat(filepath.Dir(cfgPath)); err != nil {
			t.Fatalf("expected parent dir exists: %v", err)
		}
	})

	t.Run("saveConfig writes existing file", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "config.yaml")
		if err := os.WriteFile(cfgPath, []byte("token: old\n"), 0o644); err != nil {
			t.Fatalf("prewrite file failed: %v", err)
		}
		v := viper.New()
		v.SetConfigFile(cfgPath)
		v.Set("token", "new-token")
		if err := saveConfig(v); err != nil {
			t.Fatalf("saveConfig failed: %v", err)
		}
		b, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("read file failed: %v", err)
		}
		if !strings.Contains(string(b), "new-token") {
			t.Fatalf("expected updated token in file, got: %s", string(b))
		}
	})

	t.Run("saveConfig writes new file", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "new-config.yaml")
		v := viper.New()
		v.SetConfigFile(cfgPath)
		v.Set("token", "fresh")
		if err := saveConfig(v); err != nil {
			t.Fatalf("saveConfig failed: %v", err)
		}
		if _, err := os.Stat(cfgPath); err != nil {
			t.Fatalf("expected config file to exist: %v", err)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("Root help exits without error", func(t *testing.T) {
		cfg := testConfig("http://localhost:8080", "tok")
		root := buildRootCmdForTest(cfg)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"--help"})
		if err := root.Execute(); err != nil {
			t.Fatalf("help should not error, got %v", err)
		}
	})

	t.Run("Unknown subcommand returns error", func(t *testing.T) {
		cfg := testConfig("http://localhost:8080", "tok")
		root := buildRootCmdForTest(cfg)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"does-not-exist"})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("expected unknown command error, got %v", err)
		}
	})

	t.Run("Account subcommand help lists register/login", func(t *testing.T) {
		cfg := testConfig("http://localhost:8080", "tok")
		root := buildRootCmdForTest(cfg)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"account", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatalf("account help should not error, got %v", err)
		}
		s := out.String()
		for _, sub := range []string{"register", "login"} {
			if !strings.Contains(s, sub) {
				t.Fatalf("expected help output to contain %q, got: %s", sub, s)
			}
		}
		for _, removed := range []string{"create-agent", "logout"} {
			if strings.Contains(s, removed) {
				t.Fatalf("expected help output NOT to contain %q, got: %s", removed, s)
			}
		}
	})

	t.Run("Agent subcommand help lists create/use/rotate-secret/delete", func(t *testing.T) {
		cfg := testConfig("http://localhost:8080", "tok")
		root := buildRootCmdForTest(cfg)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"agent", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatalf("agent help should not error, got %v", err)
		}
		s := out.String()
		for _, sub := range []string{"create", "use", "rotate-secret", "delete"} {
			if !strings.Contains(s, sub) {
				t.Fatalf("expected help output to contain %q, got: %s", sub, s)
			}
		}
	})

	t.Run("Group subcommand help lists create/leave/invite", func(t *testing.T) {
		cfg := testConfig("http://localhost:8080", "tok")
		root := buildRootCmdForTest(cfg)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"group", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatalf("group help should not error, got %v", err)
		}
		s := out.String()
		for _, sub := range []string{"create", "leave", "invite"} {
			if !strings.Contains(s, sub) {
				t.Fatalf("expected help output to contain %q, got: %s", sub, s)
			}
		}
	})
}
