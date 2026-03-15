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
		registerCmd(cfg),
		loginCmd(cfg),
		agentCmd(cfg),
		discoverCmd(cfg),
		listAgentsCmd(cfg),
		sendMessageCmd(cfg),
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

func TestRegisterCommand(t *testing.T) {
	t.Run("Success: correct payload sent to /v1/auth/register", func(t *testing.T) {
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

		cfg := testConfig(srv.URL, "")
		cmd := registerCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw", "--tenant-id", "t1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["tenant_id"] != "t1" || got["email"] != "u@example.com" || got["password"] != "pw" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "Missing email", args: []string{"--email", "", "--password", "pw", "--tenant-id", "t1"}, want: "--email, --password and --tenant-id are required"},
		{name: "Missing password", args: []string{"--email", "u@example.com", "--password", "", "--tenant-id", "t1"}, want: "--email, --password and --tenant-id are required"},
		{name: "Missing tenant-id", args: []string{"--email", "u@example.com", "--password", "pw", "--tenant-id", ""}, want: "--email, --password and --tenant-id are required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig("http://localhost:8080", "")
			cmd := registerCmd(cfg)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}

	t.Run("Backend error: 400 response propagated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "bad request"})
		}))
		defer srv.Close()

		cfg := testConfig(srv.URL, "")
		cmd := registerCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "pw", "--tenant-id", "t1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "bad request") {
			t.Fatalf("expected propagated backend error, got %v", err)
		}
	})
}

func TestLoginCommand(t *testing.T) {
	t.Run("Success: sends email/password, gets token", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/auth/login" || r.Method != http.MethodPost {
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

		cmd := loginCmd(cfg)
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

	t.Run("Token persistence: access_token written to config file", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "persist-me"})
		}))
		defer srv.Close()

		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "bobber", "config.yaml")
		cfg := testConfig(srv.URL, "")
		cfg.v.SetConfigFile(cfgPath)

		cmd := loginCmd(cfg)
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

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "Missing email", args: []string{"--email", "", "--password", "pw"}, want: "--email and --password are required"},
		{name: "Missing password", args: []string{"--email", "u@example.com", "--password", ""}, want: "--email and --password are required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig("http://localhost:8080", "")
			cmd := loginCmd(cfg)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}

	t.Run("Wrong credentials: 401 error propagated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid credentials"})
		}))
		defer srv.Close()

		cfg := testConfig(srv.URL, "")
		cmd := loginCmd(cfg)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--email", "u@example.com", "--password", "bad"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid credentials") {
			t.Fatalf("expected invalid credentials error, got %v", err)
		}
	})

	t.Run("No token in response: no crash, no config write", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "config.yaml")
		cfg := testConfig(srv.URL, "")
		cfg.v.SetConfigFile(cfgPath)

		cmd := loginCmd(cfg)
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

func TestAgentSubcommands(t *testing.T) {
	t.Run("agent create: success", func(t *testing.T) {
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
		cmd.SetArgs([]string{"create", "--name", "agent-x", "--version", "1.0.0", "--capabilities", "chat,search"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["display_name"] != "agent-x" || got["version"] != "1.0.0" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("agent create: missing name", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"create", "--name", "", "--version", "1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--name and --version are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("agent create: missing version", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"create", "--name", "a", "--version", ""})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--name and --version are required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("agent create: no token", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"create", "--name", "a", "--version", "1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("agent create: capabilities parsed from CSV", func(t *testing.T) {
		var gotCaps []any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			p := mustJSONMap(t, string(b))
			gotCaps, _ = p["capabilities"].([]any)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"create", "--name", "a", "--version", "1", "--capabilities", " chat, search ,,act "})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if len(gotCaps) != 3 || gotCaps[0] != "chat" || gotCaps[1] != "search" || gotCaps[2] != "act" {
			t.Fatalf("unexpected capabilities: %v", gotCaps)
		}
	})

	t.Run("agent get: success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/agents/a1" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a1"})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"get", "a1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("agent get: no token", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"get", "a1"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("agent get: missing arg", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"get"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "accepts 1 arg(s)") {
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
			t.Fatalf("unexpected grace_period_seconds: %v", got)
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
			t.Fatalf("expected grace_period_seconds=0, got %v", got)
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

	t.Run("agent list: success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/registry/agents" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []any{}})
		}))
		defer srv.Close()

		cmd := agentCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("agent list: no token", func(t *testing.T) {
		cmd := agentCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"list"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestDiscoverCommand(t *testing.T) {
	t.Run("Success: POST /v1/registry/discover with capability and status", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/v1/registry/discover" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := discoverCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--capability", "chat", "--status", "online"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if got["capability"] != "chat" {
			t.Fatalf("unexpected payload: %v", got)
		}
	})

	t.Run("Status CSV parsed", func(t *testing.T) {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			got = mustJSONMap(t, string(b))
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}))
		defer srv.Close()

		cmd := discoverCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--capability", "chat", "--status", "online,busy"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		status, _ := got["status"].([]any)
		if len(status) != 2 || status[0] != "online" || status[1] != "busy" {
			t.Fatalf("unexpected status array: %v", status)
		}
	})

	t.Run("Missing capability", func(t *testing.T) {
		cmd := discoverCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--capability", "", "--status", "online"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--capability is required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := discoverCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--capability", "chat"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestListAgentsCommand(t *testing.T) {
	t.Run("Success: GET /v1/registry/agents", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/registry/agents" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"agents": []any{}})
		}))
		defer srv.Close()

		cmd := listAgentsCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := listAgentsCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestSendMessageCommand(t *testing.T) {
	t.Run("Success: connects, sends envelope, prints sent true", func(t *testing.T) {
		received := make(chan map[string]any, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/ws/connect" {
				t.Fatalf("unexpected ws path: %s", r.URL.Path)
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

		cmd := sendMessageCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--from", "a", "--to", "b", "--tag", "request.data", "--payload", `{"x":1}`})
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
			if env["from"] != "a" || env["to"] != "b" || env["tag"] != "request.data" {
				t.Fatalf("unexpected envelope: %v", env)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for websocket envelope")
		}
	})

	t.Run("Invalid payload JSON", func(t *testing.T) {
		cmd := sendMessageCmd(testConfig("http://localhost:8080", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--from", "a", "--to", "b", "--tag", "t", "--payload", "{"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid payload json") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("No token", func(t *testing.T) {
		cmd := sendMessageCmd(testConfig("http://localhost:8080", ""))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--from", "a", "--to", "b", "--tag", "t", "--payload", `{"x":1}`})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "token required") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	missing := []struct {
		name string
		args []string
		want string
	}{
		{name: "Missing --from", args: []string{"--to", "b", "--tag", "t", "--payload", `{"x":1}`}, want: "required flag(s) \"from\" not set"},
		{name: "Missing --to", args: []string{"--from", "a", "--tag", "t", "--payload", `{"x":1}`}, want: "required flag(s) \"to\" not set"},
		{name: "Missing --tag", args: []string{"--from", "a", "--to", "b", "--payload", `{"x":1}`}, want: "required flag(s) \"tag\" not set"},
		{name: "Missing --payload", args: []string{"--from", "a", "--to", "b", "--tag", "t"}, want: "required flag(s) \"payload\" not set"},
	}
	for _, tc := range missing {
		t.Run(tc.name, func(t *testing.T) {
			cmd := sendMessageCmd(testConfig("http://localhost:8080", "tok"))
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}

	t.Run("Connection failed: unreachable URL", func(t *testing.T) {
		cmd := sendMessageCmd(testConfig("http://127.0.0.1:1", "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--from", "a", "--to", "b", "--tag", "t", "--payload", `{"x":1}`})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected dial error, got nil")
		}
	})

	t.Run("Envelope fields include valid UUIDs and RFC3339 timestamp", func(t *testing.T) {
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

		cmd := sendMessageCmd(testConfig(srv.URL, "tok"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--from", "x", "--to", "y", "--tag", "request.data", "--payload", `{"k":"v"}`})
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

	t.Run("WS URL scheme transformation logic", func(t *testing.T) {
		transform := func(in string) string {
			url := strings.TrimSuffix(in, "/")
			url = strings.Replace(url, "http://", "ws://", 1)
			url = strings.Replace(url, "https://", "wss://", 1)
			return url
		}

		cases := []struct {
			in   string
			want string
		}{
			{in: "http://localhost:8080", want: "ws://localhost:8080"},
			{in: "https://api.example.com/", want: "wss://api.example.com"},
		}

		for _, tc := range cases {
			if got := transform(tc.in); got != tc.want {
				t.Fatalf("transform mismatch: in=%q got=%q want=%q", tc.in, got, tc.want)
			}
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
	t.Run("Root command help exits without error", func(t *testing.T) {
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

	t.Run("Agent subcommand help lists subcommands", func(t *testing.T) {
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
		for _, sub := range []string{"create", "get", "delete", "rotate-secret", "list"} {
			if !strings.Contains(s, sub) {
				t.Fatalf("expected help output to contain %q, got: %s", sub, s)
			}
		}
	})

	t.Run("Send alias works as alias for send-message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade failed: %v", err)
			}
			defer c.Close()
			_ = c.ReadJSON(&map[string]any{})
		}))
		defer srv.Close()

		cfg := testConfig(srv.URL, "tok")
		root := buildRootCmdForTest(cfg)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"send", "--from", "a", "--to", "b", "--tag", "t", "--payload", `{"x":1}`})
		if err := root.Execute(); err != nil {
			t.Fatalf("send alias should work, got %v", err)
		}
	})
}
