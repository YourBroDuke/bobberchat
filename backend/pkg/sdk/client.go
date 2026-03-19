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
	"sync"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/gorilla/websocket"
)

type Client struct {
	config    Config
	conn      *websocket.Conn
	handler   MessageHandler
	done      chan struct{}
	mu        sync.Mutex
	connected bool
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
	if c == nil {
		return errors.New("client is nil")
	}

	wsURL, err := websocketConnectURL(c.config.BackendURL)
	if err != nil {
		return fmt.Errorf("build websocket url: %w", err)
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.config.APISecret)
	header.Set("X-Agent-ID", c.config.AgentID)
	header.Set("Sec-WebSocket-Protocol", "bobberchat.v1")

	dialer := *websocket.DefaultDialer
	if c.config.RequestTimeout > 0 {
		dialer.HandshakeTimeout = c.config.RequestTimeout
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("websocket dial failed: status=%d body=%s: %w", resp.StatusCode, strings.TrimSpace(string(body)), err)
		}

		return fmt.Errorf("websocket dial failed: %w", err)
	}

	if resp != nil {
		_ = resp.Body.Close()
	}

	c.mu.Lock()
	if c.connected && c.conn != nil {
		safeClose(c.done)
		_ = c.conn.Close()
	}
	c.done = make(chan struct{})
	c.conn = conn
	c.connected = true
	done := c.done
	handler := c.handler
	c.mu.Unlock()

	go c.heartbeat(done)

	if handler != nil {
		go c.readLoop(done)
	}

	return nil
}

func (c *Client) Send(ctx context.Context, msg Message) error {
	if c == nil {
		return errors.New("client is nil")
	}

	env := protocol.Envelope{
		ID:        msg.ID,
		From:      msg.From,
		To:        msg.To,
		Tag:       msg.Tag,
		Content:   msg.Content,
		Metadata:  msg.Metadata,
		Timestamp: msg.Timestamp,
	}

	if err := env.Validate(); err != nil {
		return fmt.Errorf("invalid message envelope: %w", err)
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return errors.New("websocket is not connected")
	}

	if deadline, ok := ctx.Deadline(); ok && !deadline.IsZero() {
		_ = c.conn.SetWriteDeadline(deadline)
	} else {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.RequestTimeout))
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, body); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
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

	c.mu.Lock()
	startReadLoop := c.connected && c.handler == nil
	done := c.done
	c.handler = handler
	c.mu.Unlock()

	if startReadLoop {
		go c.readLoop(done)
	}

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

	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.connected = false
	done := c.done
	c.mu.Unlock()

	safeClose(done)

	if conn == nil {
		return nil
	}

	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(2*time.Second),
	)

	if err := conn.Close(); err != nil {
		return fmt.Errorf("close websocket: %w", err)
	}

	return nil
}

func (c *Client) heartbeat(done chan struct{}) {
	c.mu.Lock()
	conn := c.conn
	if conn == nil {
		c.mu.Unlock()
		return
	}

	lastPong := time.Now()
	conn.SetPongHandler(func(string) error {
		c.mu.Lock()
		lastPong = time.Now()
		c.mu.Unlock()
		return nil
	})
	c.mu.Unlock()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			c.mu.Lock()
			if !c.connected || c.conn == nil {
				c.mu.Unlock()
				return
			}

			if time.Since(lastPong) > 2*c.config.HeartbeatInterval {
				_ = c.conn.Close()
				c.connected = false
				c.mu.Unlock()
				safeClose(done)
				return
			}

			if err := c.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				c.connected = false
				_ = c.conn.Close()
				c.mu.Unlock()
				safeClose(done)
				return
			}
			c.mu.Unlock()
		}
	}
}

func (c *Client) readLoop(done chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
		}

		c.mu.Lock()
		if !c.connected || c.conn == nil {
			c.mu.Unlock()
			return
		}

		conn := c.conn
		handler := c.handler
		c.mu.Unlock()

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				safeClose(done)
				return
			}

			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			safeClose(done)
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		if handler != nil {
			_ = handler(context.Background(), msg)
		}
	}
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

func websocketConnectURL(backendURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(backendURL))
	if err != nil {
		return "", fmt.Errorf("invalid backend url: %w", err)
	}

	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported backend url scheme: %s", u.Scheme)
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/v1/ws/connect"
	return u.String(), nil
}

func backendEndpoint(backendURL, endpointPath string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(backendURL))
	if err != nil {
		return "", fmt.Errorf("invalid backend url: %w", err)
	}

	u.Path = strings.TrimRight(u.Path, "/") + endpointPath
	return u.String(), nil
}
