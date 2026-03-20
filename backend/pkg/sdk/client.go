package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	config  Config
	handler MessageHandler
	done    chan struct{}
}

func NewClient(config Config) (*Client, error) {
	config.BackendURL = strings.TrimSpace(config.BackendURL)
	config.AgentID = strings.TrimSpace(config.AgentID)
	config.APISecret = strings.TrimSpace(config.APISecret)

	if config.BackendURL == "" {
		return nil, errors.New("backend_url is required")
	}

	if config.AgentID == "" {
		return nil, errors.New("agent_id is required")
	}

	if config.APISecret == "" {
		return nil, errors.New("api_secret is required")
	}

	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = defaultHeartbeatInterval
	}

	if config.RequestTimeout <= 0 {
		config.RequestTimeout = defaultRequestTimeout
	}

	return &Client{
		config: config,
		done:   make(chan struct{}),
	}, nil
}

func (c *Client) Connect(ctx context.Context) error {
	_ = ctx
	return nil
}

func (c *Client) Send(ctx context.Context, msg Message) error {
	if c == nil {
		return errors.New("client is nil")
	}

	endpoint, err := backendEndpoint(c.config.BackendURL, "/v1/messages/send")
	if err != nil {
		return fmt.Errorf("build send endpoint: %w", err)
	}

	body, err := json.Marshal(map[string]any{
		"to":       msg.To,
		"tag":      msg.Tag,
		"content":  msg.Content,
		"metadata": msg.Metadata,
	})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create send request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APISecret)
	req.Header.Set("X-Agent-ID", c.config.AgentID)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: c.config.RequestTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute send request: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read send response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		trimmed := strings.TrimSpace(string(rawResp))
		var errBody struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rawResp, &errBody); err == nil && strings.TrimSpace(errBody.Error) != "" {
			return fmt.Errorf("send failed: status=%d error=%s", resp.StatusCode, strings.TrimSpace(errBody.Error))
		}

		return fmt.Errorf("send failed: status=%d body=%s", resp.StatusCode, trimmed)
	}

	return nil
}

func (c *Client) Subscribe(ctx context.Context, handler MessageHandler) error {
	_ = ctx

	if c == nil {
		return errors.New("client is nil")
	}

	if handler == nil {
		return errors.New("handler is nil")
	}

	c.handler = handler

	return nil
}

func (c *Client) Discover(ctx context.Context, query DiscoveryQuery) ([]AgentProfile, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}

	endpoint, err := backendEndpoint(c.config.BackendURL, "/v1/registry/discover")
	if err != nil {
		return nil, fmt.Errorf("build discover endpoint: %w", err)
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal discovery query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create discovery request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APISecret)
	req.Header.Set("X-Agent-ID", c.config.AgentID)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: c.config.RequestTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute discovery request: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read discovery response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("discovery failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(rawResp)))
	}

	var profiles []AgentProfile
	if err := json.Unmarshal(rawResp, &profiles); err == nil {
		return profiles, nil
	}

	var wrapped struct {
		Agents []AgentProfile `json:"agents"`
	}
	if err := json.Unmarshal(rawResp, &wrapped); err != nil {
		return nil, fmt.Errorf("parse discovery response: %w", err)
	}

	return wrapped.Agents, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	safeClose(c.done)
	return nil
}

func safeClose(ch chan struct{}) {
	if ch == nil {
		return
	}

	defer func() {
		_ = recover()
	}()
	close(ch)
}

func backendEndpoint(backendURL, endpointPath string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(backendURL))
	if err != nil {
		return "", fmt.Errorf("invalid backend url: %w", err)
	}

	u.Path = strings.TrimRight(u.Path, "/") + endpointPath
	return u.String(), nil
}
