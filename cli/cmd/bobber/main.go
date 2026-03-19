// Package main implements the bobber CLI.
//
// # Design Principle: Agent-First CLI
//
// BobberChat has two participant types: agents and user accounts.
// Agents are the primary citizens of the platform — they are the autonomous
// workloads that communicate, discover peers, and coordinate actions.
// User accounts exist to own and manage agents.
//
// The top-level CLI commands (login, whoami, logout, send, poll, connect, etc.)
// operate on the agent identity by default. The "bobber account" subcommand
// tree is reserved for user-account operations (register, login with
// email/password, etc.).
//
//	bobber login --agent-id <ID> --secret <SECRET>   # authenticate as an agent
//	bobber whoami                                      # show current agent identity
//	bobber logout                                      # clear agent credentials
//
//	bobber account register ...                        # user account operations
//	bobber account login ...                           # user account operations
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
	"strconv"
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

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func accountCmd(cfg *cliConfig) *cobra.Command {
	account := &cobra.Command{Use: "account", Short: "Account management commands"}
	account.AddCommand(accountRegisterCmd(cfg), accountLoginCmd(cfg))
	return account
}

func accountRegisterCmd(cfg *cliConfig) *cobra.Command {
	var email, password string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new user",
		RunE: func(_ *cobra.Command, _ []string) error {
			if email == "" || password == "" {
				return errors.New("--email and --password are required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/auth/register", "", map[string]any{
				"email":    email,
				"password": password,
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
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func accountLoginCmd(cfg *cliConfig) *cobra.Command {
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

func agentCreateCmd(cfg *cliConfig) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create agent",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			displayName := name
			if strings.TrimSpace(displayName) == "" {
				displayName = uuid.NewString()
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/agents", cfg.token(), map[string]any{
				"display_name": displayName,
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "agent display name")
	return cmd
}

func agentCmd(cfg *cliConfig) *cobra.Command {
	agent := &cobra.Command{Use: "agent", Short: "Agent management commands"}
	agent.AddCommand(agentCreateCmd(cfg), agentUseCmd(cfg), agentRotateSecretCmd(cfg), agentDeleteCmd(cfg))
	return agent
}

func agentUseCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "use <agent_id>",
		Short: "Use an agent as current identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg.v.Set("agent_id", args[0])
			if err := saveConfig(cfg.v); err != nil {
				return err
			}
			prettyPrint(map[string]any{"agent_id": args[0], "active": true})
			return nil
		},
	}
}

func agentDeleteCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <agent_id>",
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
	cmd := &cobra.Command{
		Use:   "rotate-secret <agent_id>",
		Short: "Rotate agent API secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/agents/"+args[0]+"/rotate-secret", cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	return cmd
}

func loginCmd(cfg *cliConfig) *cobra.Command {
	var agentID, secret string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login as an agent using API secret",
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(agentID) == "" {
				return errors.New("--agent-id is required")
			}
			if strings.TrimSpace(secret) == "" {
				return errors.New("--secret is required")
			}
			cfg.v.Set("agent_id", agentID)
			cfg.v.Set("api_secret", secret)
			if err := saveConfig(cfg.v); err != nil {
				return err
			}
			prettyPrint(map[string]any{"agent_id": agentID, "saved": true})
			return nil
		},
	}
	cmd.Flags().StringVar(&agentID, "agent-id", "", "agent ID")
	cmd.Flags().StringVar(&secret, "secret", "", "agent API secret")
	_ = cmd.MarkFlagRequired("agent-id")
	_ = cmd.MarkFlagRequired("secret")
	return cmd
}

func whoamiCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current agent identity",
		RunE: func(_ *cobra.Command, _ []string) error {
			aid := cfg.agentID()
			sec := cfg.apiSecret()
			if aid == "" || sec == "" {
				return errors.New("not logged in as agent (run bobber login --agent-id <ID> --secret <SECRET>)")
			}
			resp, err := doJSONAgent(http.MethodGet, cfg.backendURL()+"/v1/agents/"+aid, aid, sec, nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func logoutCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout by clearing agent credentials",
		RunE: func(_ *cobra.Command, _ []string) error {
			return clearAgentCreds(cfg)
		},
	}
}

func lsCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "ls [dms|groups]",
		Short: "List DM conversations or groups",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}

			kind := "dms"
			if len(args) == 1 {
				kind = args[0]
			}

			var endpoint string
			switch kind {
			case "dms":
				endpoint = "/v1/conversations?type=direct"
			case "groups":
				endpoint = "/v1/groups"
			default:
				return errors.New("invalid list target: must be dms or groups")
			}

			resp, err := doJSON(http.MethodGet, cfg.backendURL()+endpoint, cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func connectCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "connect <target_id>",
		Short: "Request a connection with target",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/connections/request", cfg.token(), map[string]any{
				"target_id": args[0],
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func inboxCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "inbox",
		Short: "Show pending connects and unread chats",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodGet, cfg.backendURL()+"/v1/connections/inbox", cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func acceptCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "accept <target_id>",
		Short: "Accept incoming request from target",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/connections/"+args[0]+"/accept", cfg.token(), map[string]any{})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func rejectCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "reject <target_id>",
		Short: "Reject incoming request from target",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/connections/"+args[0]+"/reject", cfg.token(), map[string]any{})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func blacklistCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "blacklist <target_id>",
		Short: "Blacklist target",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/blacklist", cfg.token(), map[string]any{
				"target_id": args[0],
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func infoCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "info <target_id>",
		Short: "Get information about a user, agent, or group",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodGet, cfg.backendURL()+"/v1/info/"+args[0], cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func sendCmd(cfg *cliConfig) *cobra.Command {
	var tag, content string
	cmd := &cobra.Command{
		Use:   "send <target_id>",
		Short: "Send one message over websocket",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			if tag == "" || content == "" {
				return errors.New("--tag and --content are required")
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
				"from":      "",
				"to":        args[0],
				"tag":       tag,
				"payload":   map[string]any{"content": content},
				"metadata":  map[string]any{},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			if err := conn.WriteJSON(env); err != nil {
				return err
			}
			prettyPrint(map[string]any{"sent": true, "envelope": env})
			return nil
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "message tag")
	cmd.Flags().StringVar(&content, "content", "", "message content")
	_ = cmd.MarkFlagRequired("tag")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func pollCmd(cfg *cliConfig) *cobra.Command {
	var limit int
	var sinceTS, sinceID string
	cmd := &cobra.Command{
		Use:   "poll <conversation_id>",
		Short: "Poll messages from conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			endpoint := cfg.backendURL() + "/v1/messages/poll?conversation_id=" + args[0]
			if limit > 0 {
				endpoint += "&limit=" + strconv.Itoa(limit)
			}
			if sinceTS != "" {
				endpoint += "&since_ts=" + sinceTS
			}
			if sinceID != "" {
				endpoint += "&since_id=" + sinceID
			}
			resp, err := doJSON(http.MethodGet, endpoint, cfg.token(), nil)
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum message count")
	cmd.Flags().StringVar(&sinceTS, "since_ts", "", "fetch messages after timestamp")
	cmd.Flags().StringVar(&sinceID, "since_id", "", "fetch messages after message id")
	return cmd
}

func groupCmd(cfg *cliConfig) *cobra.Command {
	group := &cobra.Command{Use: "group", Short: "Group management commands"}
	group.AddCommand(groupCreateCmd(cfg), groupLeaveCmd(cfg), groupInviteCmd(cfg))
	return group
}

func groupCreateCmd(cfg *cliConfig) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create group",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			if strings.TrimSpace(name) == "" {
				return errors.New("--name is required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/groups", cfg.token(), map[string]any{
				"name": name,
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "group name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func groupLeaveCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "leave <target_id>",
		Short: "Leave group",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/groups/"+args[0]+"/leave", cfg.token(), map[string]any{
				"participant_id":   "",
				"participant_kind": "user",
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
}

func groupInviteCmd(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "invite <target_group_id> <target_user_id>",
		Short: "Invite user to group",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if cfg.token() == "" {
				return errors.New("token required")
			}
			resp, err := doJSON(http.MethodPost, cfg.backendURL()+"/v1/groups/"+args[0]+"/join", cfg.token(), map[string]any{
				"participant_id":   args[1],
				"participant_kind": "user",
			})
			if err != nil {
				return err
			}
			prettyPrint(resp)
			return nil
		},
	}
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

func clearAgentCreds(cfg *cliConfig) error {
	cfg.v.Set("agent_id", "")
	cfg.v.Set("api_secret", "")
	cfg.v.Set("token", "")
	return saveConfig(cfg.v)
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

func (c *cliConfig) agentID() string {
	return strings.TrimSpace(c.v.GetString("agent_id"))
}

func (c *cliConfig) apiSecret() string {
	return strings.TrimSpace(c.v.GetString("api_secret"))
}

func doJSONAgent(method, url, agentID, apiSecret string, body any) (map[string]any, error) {
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
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-API-Secret", apiSecret)

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
