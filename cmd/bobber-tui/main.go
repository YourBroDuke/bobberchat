package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

type model struct {
	width, height int
	activePane    int

	agents      []agentEntry
	agentCursor int

	messages    []messageEntry
	msgCursor   int
	msgViewport viewport.Model

	contextInfo string

	textInput textinput.Model
	inputMode bool

	backendURL string
	wsConn     *websocket.Conn
	token      string
	tenantID   string
	connected  bool

	approvals     []approvalEntry
	approvalCursor int
	showApprovals bool

	statusMsg string
	err       error
}

type agentEntry struct {
	ID, Name, Status string
	Capabilities     []string
	LastHeartbeat    string
}

type messageEntry struct {
	ID, From, To, Tag string
	Payload           string
	Timestamp         string
	TraceID           string
}

type approvalEntry struct {
	ID, AgentID, Action  string
	Urgency, Justification string
	CreatedAt            string
}

type agentsMsg []agentEntry
type messageMsg messageEntry
type approvalsMsg []approvalEntry
type errMsg error
type tickMsg time.Time
type connectedMsg struct{ conn *websocket.Conn }
type wsDisconnectedMsg struct{ err error }
type statusUpdateMsg string

type wireEnvelope struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Tag       string         `json:"tag"`
	Payload   any            `json:"payload"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp string         `json:"timestamp"`
	TraceID   string         `json:"trace_id"`
}

var (
	headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)

	activeBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("14")).Padding(0, 1)
	inactiveBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)

	statusOnline  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	statusOffline = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	statusBusy    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	tagBadge       = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14")).Padding(0, 1)
	timestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Background(lipgloss.Color("0")).Padding(0, 1)
	inputBarStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("236")).Padding(0, 1)
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
)

func newModel(backendURL, token, tenantID string) model {
	ti := textinput.New()
	ti.Placeholder = "Type message or command..."
	ti.Prompt = "> "
	ti.CharLimit = 2000
	ti.Width = 80

	vp := viewport.New(0, 0)
	vp.SetContent("Waiting for messages...")

	return model{
		backendURL: strings.TrimRight(strings.TrimSpace(backendURL), "/"),
		token:      strings.TrimSpace(token),
		tenantID:   strings.TrimSpace(tenantID),
		textInput:  ti,
		msgViewport: vp,
		statusMsg:  "Starting...",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		connectWSCmd(m.backendURL, m.token),
		fetchAgentsCmd(m.backendURL, m.token),
		fetchApprovalsCmd(m.backendURL, m.token),
		tickCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.inputMode {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		m.rebuildMessageViewport()

	case tea.KeyMsg:
		if m.inputMode {
			switch msg.String() {
			case "esc":
				m.inputMode = false
				m.textInput.Blur()
				return m, nil
			case "enter":
				text := strings.TrimSpace(m.textInput.Value())
				m.textInput.SetValue("")
				m.inputMode = false
				m.textInput.Blur()
				if text == "" {
					return m, nil
				}
				if strings.HasPrefix(text, "/approve ") {
					parts := strings.Fields(text)
					if len(parts) < 3 {
						m.statusMsg = "Usage: /approve <id> <grant|deny> [reason]"
						return m, nil
					}
					reason := ""
					if len(parts) > 3 {
						reason = strings.Join(parts[3:], " ")
					}
					return m, tea.Batch(
						decideApprovalCmd(m.backendURL, m.token, parts[1], parts[2], reason),
						fetchApprovalsCmd(m.backendURL, m.token),
					)
				}
				return m, sendMessageCmd(m.wsConn, m.selectedTarget(), text)
			default:
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.wsConn != nil {
				_ = m.wsConn.Close()
				m.wsConn = nil
			}
			return m, tea.Quit

		case "tab":
			m.activePane = (m.activePane + 1) % 3
			m.rebuildContext()

		case "up", "k":
			m.moveCursor(-1)

		case "down", "j":
			m.moveCursor(1)

		case "enter":
			m.rebuildContext()

		case "a":
			m.showApprovals = !m.showApprovals
			m.rebuildContext()

		case "i":
			m.inputMode = true
			m.textInput.Focus()

		case "r":
			m.statusMsg = "Refreshing agents and approvals..."
			return m, tea.Batch(fetchAgentsCmd(m.backendURL, m.token), fetchApprovalsCmd(m.backendURL, m.token))

		case "y":
			if m.showApprovals && len(m.approvals) > 0 {
				ap := m.approvals[m.approvalCursor]
				return m, tea.Batch(
					decideApprovalCmd(m.backendURL, m.token, ap.ID, "grant", "approved from TUI"),
					fetchApprovalsCmd(m.backendURL, m.token),
				)
			}

		case "n":
			if m.showApprovals && len(m.approvals) > 0 {
				ap := m.approvals[m.approvalCursor]
				return m, tea.Batch(
					decideApprovalCmd(m.backendURL, m.token, ap.ID, "deny", "denied from TUI"),
					fetchApprovalsCmd(m.backendURL, m.token),
				)
			}
		}

	case connectedMsg:
		m.wsConn = msg.conn
		m.connected = true
		m.statusMsg = "Connected to backend WebSocket"
		return m, wsReadCmd(m.wsConn)

	case wsDisconnectedMsg:
		m.connected = false
		if m.wsConn != nil {
			_ = m.wsConn.Close()
			m.wsConn = nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = "Disconnected; retrying..."
		}
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return statusUpdateMsg("reconnect")
		})

	case statusUpdateMsg:
		if string(msg) == "reconnect" && !m.connected {
			return m, connectWSCmd(m.backendURL, m.token)
		}

	case agentsMsg:
		m.agents = msg
		if m.agentCursor >= len(m.agents) {
			m.agentCursor = maxInt(0, len(m.agents)-1)
		}
		online := 0
		for _, a := range m.agents {
			if strings.EqualFold(a.Status, "ONLINE") {
				online++
			}
		}
		m.statusMsg = fmt.Sprintf("Agent list refreshed (%d total, %d online)", len(m.agents), online)
		m.rebuildContext()

	case messageMsg:
		m.messages = append(m.messages, messageEntry(msg))
		if len(m.messages) > 2000 {
			m.messages = m.messages[len(m.messages)-2000:]
		}
		m.msgCursor = len(m.messages) - 1
		m.rebuildMessageViewport()
		m.rebuildContext()

		if m.wsConn != nil {
			return m, wsReadCmd(m.wsConn)
		}

	case approvalsMsg:
		m.approvals = msg
		if m.approvalCursor >= len(m.approvals) {
			m.approvalCursor = maxInt(0, len(m.approvals)-1)
		}
		m.statusMsg = fmt.Sprintf("Approvals updated (%d pending)", len(m.approvals))
		m.rebuildContext()

	case errMsg:
		m.err = msg
		m.statusMsg = "Error: " + msg.Error()

	case tickMsg:
		cmds := []tea.Cmd{tickCmd(), fetchAgentsCmd(m.backendURL, m.token)}
		if m.showApprovals {
			cmds = append(cmds, fetchApprovalsCmd(m.backendURL, m.token))
		}
		if !m.connected {
			cmds = append(cmds, connectWSCmd(m.backendURL, m.token))
		}
		return m, tea.Batch(cmds...)
	}

	return m, cmd
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	leftW, centerW, rightW := paneWidths(m.width)
	bodyH := maxInt(8, m.height-3)

	left := m.renderPane("Agent Directory", m.renderAgents(), leftW, bodyH, m.activePane == 0)
	center := m.renderPane("Messages", m.msgViewport.View(), centerW, bodyH, m.activePane == 1)
	right := m.renderPane("Context Panel", m.renderContext(), rightW, bodyH, m.activePane == 2)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)

	inputValue := m.textInput.View()
	if !m.inputMode {
		inputValue = "> " + mutedStyle.Render("Type message or command... (press i)")
	}
	inputBar := inputBarStyle.Width(m.width).Render(inputValue)

	connStatus := "disconnected"
	if m.connected {
		connStatus = "connected"
	}
	online := 0
	for _, a := range m.agents {
		if strings.EqualFold(a.Status, "ONLINE") {
			online++
		}
	}
	status := fmt.Sprintf("%s | %d agents online | active pane: %d | Tab:switch i:input a:approvals r:refresh q:quit", connStatus, online, m.activePane+1)
	if strings.TrimSpace(m.statusMsg) != "" {
		status = m.statusMsg + " | " + status
	}
	statusBar := statusBarStyle.Width(m.width).Render(truncate(status, m.width-2))

	return lipgloss.JoinVertical(lipgloss.Left, body, inputBar, statusBar)
}

func (m *model) recalcLayout() {
	_, centerW, _ := paneWidths(m.width)
	bodyH := maxInt(8, m.height-3)
	innerW := maxInt(10, centerW-4)
	innerH := maxInt(2, bodyH-3)
	m.msgViewport.Width = innerW
	m.msgViewport.Height = innerH
	if m.textInput.Width > m.width-4 || m.textInput.Width == 0 {
		m.textInput.Width = maxInt(8, m.width-8)
	}
}

func (m *model) renderPane(title, content string, width, height int, active bool) string {
	style := inactiveBorder
	if active {
		style = activeBorder
	}
	header := headerStyle.Render(title)
	full := lipgloss.JoinVertical(lipgloss.Left, header, content)
	return style.Width(width).Height(height).Render(full)
}

func (m *model) renderAgents() string {
	if len(m.agents) == 0 {
		return mutedStyle.Render("No agents found")
	}
	lines := make([]string, 0, len(m.agents))
	for i, a := range m.agents {
		marker := " "
		if i == m.agentCursor {
			marker = "▸"
		}
		statusIcon, statusText := statusGlyph(a.Status)
		line := fmt.Sprintf("%s %s %s [%s]", marker, statusIcon, safeName(a.Name, a.ID), statusText)
		if i == m.agentCursor {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, truncate(line, maxInt(8, m.width/4-4)))
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderContext() string {
	if m.showApprovals {
		if len(m.approvals) == 0 {
			return "Approval queue is empty"
		}
		ap := m.approvals[m.approvalCursor]
		return strings.Join([]string{
			headerStyle.Render("Approval Queue"),
			fmt.Sprintf("%d pending", len(m.approvals)),
			"",
			fmt.Sprintf("ID: %s", ap.ID),
			fmt.Sprintf("Agent: %s", ap.AgentID),
			fmt.Sprintf("Action: %s", ap.Action),
			fmt.Sprintf("Urgency: %s", ap.Urgency),
			fmt.Sprintf("Created: %s", prettyTime(ap.CreatedAt)),
			"",
			"Justification:",
			wrap(ap.Justification, maxInt(20, m.width/4-8)),
			"",
			mutedStyle.Render("Press y=grant n=deny"),
		}, "\n")
	}

	if m.activePane == 1 && len(m.messages) > 0 {
		msg := m.messages[m.msgCursor]
		return strings.Join([]string{
			fmt.Sprintf("Message ID: %s", msg.ID),
			fmt.Sprintf("From: %s", msg.From),
			fmt.Sprintf("To: %s", msg.To),
			fmt.Sprintf("Tag: %s", msg.Tag),
			fmt.Sprintf("Time: %s", prettyTime(msg.Timestamp)),
			fmt.Sprintf("Trace: %s", msg.TraceID),
			"",
			"Payload:",
			msg.Payload,
		}, "\n")
	}

	if len(m.agents) > 0 {
		a := m.agents[m.agentCursor]
		return strings.Join([]string{
			fmt.Sprintf("Agent: %s", safeName(a.Name, a.ID)),
			fmt.Sprintf("Status: %s", strings.ToUpper(a.Status)),
			fmt.Sprintf("Caps: %s", strings.Join(a.Capabilities, ", ")),
			fmt.Sprintf("Last HB: %s", prettyTime(a.LastHeartbeat)),
			fmt.Sprintf("ID: %s", a.ID),
		}, "\n")
	}

	return "No context selected"
}

func (m *model) rebuildContext() {
	m.contextInfo = m.renderContext()
}

func (m *model) moveCursor(delta int) {
	switch m.activePane {
	case 0:
		if len(m.agents) == 0 {
			return
		}
		m.agentCursor = clamp(m.agentCursor+delta, 0, len(m.agents)-1)
		m.rebuildContext()
	case 1:
		if len(m.messages) == 0 {
			return
		}
		m.msgCursor = clamp(m.msgCursor+delta, 0, len(m.messages)-1)
		m.rebuildMessageViewport()
		m.rebuildContext()
	case 2:
		if m.showApprovals && len(m.approvals) > 0 {
			m.approvalCursor = clamp(m.approvalCursor+delta, 0, len(m.approvals)-1)
			m.rebuildContext()
		}
	}
}

func (m *model) rebuildMessageViewport() {
	if m.msgViewport.Width <= 0 || m.msgViewport.Height <= 0 {
		return
	}
	if len(m.messages) == 0 {
		m.msgViewport.SetContent(mutedStyle.Render("Waiting for live messages..."))
		return
	}

	var b strings.Builder
	for i, msg := range m.messages {
		cursor := " "
		if i == m.msgCursor {
			cursor = "▸"
		}
		timeText := timestampStyle.Render("[" + shortTime(msg.Timestamp) + "]")
		headline := fmt.Sprintf("%s %s %s→%s %s", cursor, timeText, msg.From, msg.To, tagBadge.Render(msg.Tag))
		payloadLine := firstLine(msg.Payload)
		if payloadLine == "" {
			payloadLine = "{}"
		}
		payloadLine = "  " + truncate(payloadLine, maxInt(12, m.msgViewport.Width-2))
		if i == m.msgCursor {
			headline = selectedStyle.Render(headline)
		}
		b.WriteString(truncate(headline, maxInt(12, m.msgViewport.Width-1)))
		b.WriteString("\n")
		b.WriteString(payloadLine)
		b.WriteString("\n")
	}

	m.msgViewport.SetContent(b.String())
	line := m.msgCursor * 2
	if line > m.msgViewport.Height {
		m.msgViewport.YOffset = maxInt(0, line-m.msgViewport.Height/2)
	}
}

func (m *model) selectedTarget() string {
	if len(m.agents) > 0 {
		return m.agents[m.agentCursor].ID
	}
	return ""
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func connectWSCmd(backendURL, token string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(token) == "" {
			return errMsg(errors.New("missing token; use --token or BOBBERCHAT_TOKEN"))
		}
		wsURL, err := toWebsocketURL(backendURL + "/v1/ws/connect")
		if err != nil {
			return errMsg(fmt.Errorf("build websocket url: %w", err))
		}

		header := http.Header{}
		header.Set("Authorization", "Bearer "+token)
		header.Set("Sec-WebSocket-Protocol", "bobberchat.v1")

		dialer := websocket.Dialer{HandshakeTimeout: 12 * time.Second, Subprotocols: []string{"bobberchat.v1"}}
		conn, resp, err := dialer.Dial(wsURL, header)
		if err != nil {
			if resp != nil {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				return errMsg(fmt.Errorf("ws connect failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body))))
			}
			return errMsg(fmt.Errorf("ws connect failed: %w", err))
		}
		return connectedMsg{conn: conn}
	}
}

func wsReadCmd(conn *websocket.Conn) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return wsDisconnectedMsg{err: errors.New("ws connection is nil")}
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return wsDisconnectedMsg{err: err}
		}
		var env wireEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return errMsg(fmt.Errorf("decode ws message: %w", err))
		}

		payload := "{}"
		if env.Payload != nil {
			payload = prettyJSON(env.Payload)
		}
		return messageMsg(messageEntry{
			ID:        env.ID,
			From:      env.From,
			To:        env.To,
			Tag:       env.Tag,
			Payload:   payload,
			Timestamp: env.Timestamp,
			TraceID:   env.TraceID,
		})
	}
}

func fetchAgentsCmd(backendURL, token string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(backendURL, "/")+"/v1/registry/agents", nil)
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		cli := &http.Client{Timeout: 10 * time.Second}
		resp, err := cli.Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("fetch agents: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("fetch agents failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body))))
		}

		var payload struct {
			Agents []struct {
				AgentID       string   `json:"agent_id"`
				DisplayName   string   `json:"display_name"`
				Name          string   `json:"name"`
				Status        string   `json:"status"`
				Capabilities  []string `json:"capabilities"`
				LastHeartbeat string   `json:"last_heartbeat"`
			} `json:"agents"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return errMsg(fmt.Errorf("decode agents response: %w", err))
		}

		agents := make([]agentEntry, 0, len(payload.Agents))
		for _, a := range payload.Agents {
			name := strings.TrimSpace(a.DisplayName)
			if name == "" {
				name = strings.TrimSpace(a.Name)
			}
			agents = append(agents, agentEntry{
				ID:            a.AgentID,
				Name:          name,
				Status:        strings.ToUpper(strings.TrimSpace(a.Status)),
				Capabilities:  a.Capabilities,
				LastHeartbeat: a.LastHeartbeat,
			})
		}

		sort.SliceStable(agents, func(i, j int) bool {
			ri := statusRank(agents[i].Status)
			rj := statusRank(agents[j].Status)
			if ri != rj {
				return ri < rj
			}
			return safeName(agents[i].Name, agents[i].ID) < safeName(agents[j].Name, agents[j].ID)
		})

		return agentsMsg(agents)
	}
}

func fetchApprovalsCmd(backendURL, token string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(backendURL, "/")+"/v1/approvals/pending", nil)
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		cli := &http.Client{Timeout: 10 * time.Second}
		resp, err := cli.Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("fetch approvals: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("fetch approvals failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body))))
		}

		var payload struct {
			Approvals []struct {
				ApprovalID     string `json:"approval_id"`
				AgentID        string `json:"agent_id"`
				Action         string `json:"action"`
				Urgency        string `json:"urgency"`
				Justification  string `json:"justification"`
				CreatedAt      string `json:"created_at"`
			} `json:"approvals"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return errMsg(fmt.Errorf("decode approvals response: %w", err))
		}

		items := make([]approvalEntry, 0, len(payload.Approvals))
		for _, a := range payload.Approvals {
			items = append(items, approvalEntry{
				ID:            a.ApprovalID,
				AgentID:       a.AgentID,
				Action:        a.Action,
				Urgency:       a.Urgency,
				Justification: a.Justification,
				CreatedAt:     a.CreatedAt,
			})
		}
		return approvalsMsg(items)
	}
}

func decideApprovalCmd(backendURL, token, approvalID, decision, reason string) tea.Cmd {
	return func() tea.Msg {
		payload := map[string]any{
			"decision": strings.ToUpper(strings.TrimSpace(decision)),
			"reason":   reason,
		}
		body, _ := json.Marshal(payload)
		endpoint := strings.TrimRight(backendURL, "/") + "/v1/approvals/" + approvalID + "/decide"
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("decide approval: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("approval decision failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))))
		}
		return statusUpdateMsg("approval updated")
	}
}

func sendMessageCmd(conn *websocket.Conn, target, text string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return errMsg(errors.New("not connected to websocket"))
		}
		if strings.TrimSpace(target) == "" {
			return errMsg(errors.New("no target selected; pick an agent first"))
		}

		env := wireEnvelope{
			ID:        fmt.Sprintf("tui-%d", time.Now().UnixNano()),
			From:      "tui",
			To:        target,
			Tag:       "request.data",
			Payload:   map[string]any{"text": text},
			Metadata:  map[string]any{"source": "bobber-tui"},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			TraceID:   fmt.Sprintf("trace-%d", time.Now().UnixNano()),
		}
		if err := conn.WriteJSON(env); err != nil {
			return errMsg(fmt.Errorf("send message: %w", err))
		}
		return statusUpdateMsg("message sent")
	}
}

func toWebsocketURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	return u.String(), nil
}

func statusGlyph(status string) (string, string) {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "ONLINE", "IDLE":
		return statusOnline.Render("●"), s
	case "BUSY":
		return statusBusy.Render("◐"), s
	default:
		return statusOffline.Render("○"), firstNonEmpty(s, "OFFLINE")
	}
}

func paneWidths(total int) (left, center, right int) {
	if total < 30 {
		return maxInt(10, total/3), maxInt(10, total/3), maxInt(10, total-total*2/3)
	}
	left = total / 4
	center = total / 2
	right = total - left - center
	return
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func prettyTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("15:04:05")
}

func shortTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		if len(ts) >= 8 {
			return ts[:8]
		}
		return ts
	}
	return t.Local().Format("15:04:05")
}

func firstLine(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "\n")
	return strings.TrimSpace(parts[0])
}

func wrap(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	lines := []string{}
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
			continue
		}
		if len(cur)+1+len(w) > width {
			lines = append(lines, cur)
			cur = w
		} else {
			cur += " " + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func statusRank(s string) int {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ONLINE":
		return 0
	case "BUSY":
		return 1
	case "IDLE":
		return 2
	default:
		return 3
	}
}

func safeName(name, fallback string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return fallback
	}
	return n
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 1 {
		return "…"
	}
	return s[:limit-1] + "…"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func main() {
	backendURL := flag.String("backend-url", envOr("BOBBERCHAT_BACKEND_URL", "http://localhost:8080"), "BobberChat backend URL")
	token := flag.String("token", envOr("BOBBERCHAT_TOKEN", ""), "JWT bearer token")
	tenantID := flag.String("tenant-id", envOr("BOBBERCHAT_TENANT_ID", ""), "tenant ID")
	flag.Parse()

	m := newModel(*backendURL, *token, *tenantID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bobber-tui error: %v\n", err)
		os.Exit(1)
	}
}
