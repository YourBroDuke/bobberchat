package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type cliConfig struct{ v *viper.Viper }

func main() {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("BOBBER")
	v.AutomaticEnv()
	v.SetDefault("backend_url", "http://localhost:8080")
	v.SetDefault("token", "")

	configFile := defaultConfigFile()
	v.SetConfigFile(configFile)
	_ = v.ReadInConfig()

	cfg := &cliConfig{v: v}

	root := &cobra.Command{
		Use:   "bobber",
		Short: "BobberChat CLI",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			_ = cfg.v.BindPFlag("backend_url", cmd.Flags().Lookup("backend-url"))
			_ = cfg.v.BindPFlag("token", cmd.Flags().Lookup("token"))
		},
	}
	root.PersistentFlags().String("backend-url", v.GetString("backend_url"), "backend url")
	root.PersistentFlags().String("token", v.GetString("token"), "jwt token")

	root.AddCommand(
		registerCmd(cfg),
		loginCmd(cfg),
		agentCmd(cfg),
		discoverCmd(cfg),
		listAgentsCmd(cfg),
		sendMessageCmd(cfg),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func registerCmd(cfg *cliConfig) *cobra.Command {
	var email, password, tenantID string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new user",
		RunE: func(_ *cobra.Command, _ []string) error {
			if email == "" || password == "" || tenantID == "" {
				return errors.New("--email, --password and --tenant-id are required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/auth/register", "", map[string]any{
				"tenant_id": tenantID,
				"email":     email,
				"password":  password,
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "user email")
	cmd.Flags().StringVar(&password, "password", "", "user password")
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "tenant id")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")
	_ = cmd.MarkFlagRequired("tenant-id")
	return cmd
}

func loginCmd(cfg *cliConfig) *cobra.Command {
	var email, password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login and persist JWT token",
		RunE: func(_ *cobra.Command, _ []string) error {
			if email == "" || password == "" {
				return errors.New("--email and --password are required")
			}

			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/auth/login", "", map[string]any{
				"email":    email,
				"password": password,
			})
			if err != nil {
				return err
			}
			tok, ok := resp["access_token"].(string)
			if ok && tok != "" {
				cfg.v.Set("token", tok)
				if err := saveConfig(cfg.v); err != nil {
					return err
				}
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "user email")
	cmd.Flags().StringVar(&password, "password", "", "user password")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func agentCmd(cfg *cliConfig) *cobra.Command {
	agent := &cobra.Command{Use: "agent", Short: "Agent management commands"}
	agent.AddCommand(agentCreateCmd(cfg), agentGetCmd(cfg), agentDeleteCmd(cfg), agentRotateSecretCmd(cfg), agentListCmd(cfg))
	return agent
}

func agentCreateCmd(cfg *cliConfig) *cobra.Command {
	var name, version string
	var capabilities string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create agent",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			if name == "" || version == "" {
				return errors.New("--name and --version are required")
			}
			caps := splitCSV(capabilities)
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/agents", cfg.token(), map[string]any{
				"display_name":  name,
				"capabilities": caps,
				"version":      version,
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "agent display name")
	cmd.Flags().StringVar(&capabilities, "capabilities", "", "comma-separated capabilities")
	cmd.Flags().StringVar(&version, "version", "", "agent version")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func agentGetCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get agent by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodGet, cfg.backendURL()+"/v1/agents/"+args[0], cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func agentDeleteCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodDelete, cfg.backendURL()+"/v1/agents/"+args[0], cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func agentRotateSecretCmd(cfg *cliConfig) *cobra.Command {
	var grace int
	cmd := &cobra.Command{
		Use:   "rotate-secret <id>",
		Short: "Rotate agent API secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/agents/"+args[0]+"/rotate-secret", cfg.token(), map[string]any{
				"grace_period_seconds": grace,
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().IntVar(&grace, "grace-period", 0, "old secret grace period in seconds")
	return cmd
}

func agentListCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodGet, cfg.backendURL()+"/v1/registry/agents", cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func discoverCmd(cfg *cliConfig) *cobra.Command {
	var capability, status string
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			if capability == "" {
				return errors.New("--capability is required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/registry/discover", cfg.token(), map[string]any{
				"capability": capability,
				"status":     splitCSV(status),
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&capability, "capability", "", "capability")
	cmd.Flags().StringVar(&status, "status", "", "comma-separated statuses")
	_ = cmd.MarkFlagRequired("capability")
	return cmd
}

func listAgentsCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "list-agents",
		Short: "List all agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodGet, cfg.backendURL()+"/v1/registry/agents", cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func sendMessageCmd(cfg *cliConfig) *cobra.Command {
	var from, to, tag, payload string
	cmd := &cobra.Command{
		Use:     "send-message",
		Aliases: []string{"send"},
		Short: "Send one message over websocket",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			if from == "" || to == "" || tag == "" || payload == "" {
				return errors.New("--from --to --tag --payload are required")
			}
			var payloadObj map[string]any
			if err := json.Unmarshal([]byte(payload), &payloadObj); err != nil {
				return fmt.Errorf("invalid payload json: %w", err)
			}

			url := strings.TrimSuffix(cfg.backendURL(), "/")
			url = strings.Replace(url, "http://", "ws://", 1)
			url = strings.Replace(url, "https://", "wss://", 1)
			wsURL := url + "/v1/ws/connect?token=" + cfg.token()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
			if err != nil {
				return err
			}
			defer conn.Close()

			env := map[string]any{
				"id":        uuidString(),
				"from":      from,
				"to":        to,
				"tag":       tag,
				"payload":   payloadObj,
				"metadata":  map[string]any{},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"trace_id":  uuidString(),
			}
			if err := conn.WriteJSON(env); err != nil {
				return err
			}
			prettyPrint(map[string]any{"sent": true, "envelope": env})
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "sender id")
	cmd.Flags().StringVar(&to, "to", "", "recipient id")
	cmd.Flags().StringVar(&tag, "tag", "", "message tag")
	cmd.Flags().StringVar(&payload, "payload", "", "json payload")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("tag")
	_ = cmd.MarkFlagRequired("payload")
	return cmd
}

func doJSON(method, url, token string, body any) (map[string]any, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resp := map[string]any{}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		if msg, ok := resp["error"].(string); ok {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("request failed with status %d", res.StatusCode)
	}

	return resp, nil
}

func prettyPrint(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func (c *cliConfig) backendURL() string {
	v := strings.TrimSpace(c.v.GetString("backend_url"))
	if v == "" {
		return "http://localhost:8080"
	}
	return v
}

func (c *cliConfig) token() string {
	return strings.TrimSpace(c.v.GetString("token"))
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func defaultConfigFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".bobber.yaml"
	}
	return filepath.Join(dir, "bobber", "config.yaml")
}

func saveConfig(v *viper.Viper) error {
	file := v.ConfigFileUsed()
	if file == "" {
		file = defaultConfigFile()
		v.SetConfigFile(file)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(file); err == nil {
		return v.WriteConfig()
	}
	return v.SafeWriteConfigAs(file)
}

func uuidString() string {
	return uuid.NewString()
}
