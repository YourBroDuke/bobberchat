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
	leftSection int

	groups      []groupEntry
	groupCursor int

	messages    []messageEntry
	msgCursor   int
	msgViewport viewport.Model

	topics      []topicEntry
	topicCursor int
	showTopics  bool

	filterMode       bool
	filterText       string
	filteredMessages []int

	agentFilterMode bool
	agentFilterText string

	contextInfo string

	textInput textinput.Model
	inputMode bool

	backendURL string
	wsConn     *websocket.Conn
	token      string
	tenantID   string
	connected  bool

	approvals      []approvalEntry
	approvalCursor int
	showApprovals  bool

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
	ID, AgentID, Action    string
	Urgency, Justification string
	CreatedAt              string
}

type groupEntry struct {
	ID, Name, Visibility string
	MemberCount          int
}

type topicEntry struct {
	ID, GroupID, Title, Status string
}

type agentsMsg []agentEntry
type groupsMsg []groupEntry
type topicsMsg []topicEntry
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

	activeBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("14")).Padding(0, 1)
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
		backendURL:  strings.TrimRight(strings.TrimSpace(backendURL), "/"),
		token:       strings.TrimSpace(token),
		tenantID:    strings.TrimSpace(tenantID),
		textInput:   ti,
		msgViewport: vp,
		statusMsg:   "Starting...",
		leftSection: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		connectWSCmd(m.backendURL, m.token),
		fetchAgentsCmd(m.backendURL, m.token),
		fetchGroupsCmd(m.backendURL, m.token),
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
				if strings.HasPrefix(text, "/join ") {
					parts := strings.Fields(text)
					if len(parts) != 2 {
						m.statusMsg = "Usage: /join <group_id>"
						return m, nil
					}
					return m, tea.Batch(joinGroupCmd(m.backendURL, m.token, parts[1]), fetchGroupsCmd(m.backendURL, m.token))
				}
				if strings.HasPrefix(text, "/leave ") {
					parts := strings.Fields(text)
					if len(parts) != 2 {
						m.statusMsg = "Usage: /leave <group_id>"
						return m, nil
					}
					return m, tea.Batch(leaveGroupCmd(m.backendURL, m.token, parts[1]), fetchGroupsCmd(m.backendURL, m.token))
				}
				if text == "/groups" {
					m.statusMsg = "Refreshing groups..."
					return m, fetchGroupsCmd(m.backendURL, m.token)
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

		if m.filterMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.filterMode = false
				m.filterText = ""
				m.applyMessageFilter()
				m.rebuildMessageViewport()
				m.rebuildContext()
				return m, nil
			case tea.KeyEnter:
				m.filterMode = false
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete:
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
					m.applyMessageFilter()
					m.rebuildMessageViewport()
					m.rebuildContext()
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.filterText += string(msg.Runes)
					m.applyMessageFilter()
					m.rebuildMessageViewport()
					m.rebuildContext()
				}
				return m, nil
			}
		}

		if m.agentFilterMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.agentFilterMode = false
				m.agentFilterText = ""
				m.adjustAgentCursorToFilter()
				m.rebuildContext()
				return m, nil
			case tea.KeyEnter:
				m.agentFilterMode = false
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete:
				if len(m.agentFilterText) > 0 {
					m.agentFilterText = m.agentFilterText[:len(m.agentFilterText)-1]
					m.adjustAgentCursorToFilter()
					m.rebuildContext()
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.agentFilterText += string(msg.Runes)
					m.adjustAgentCursorToFilter()
					m.rebuildContext()
				}
				return m, nil
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
			if m.activePane == 0 && m.leftSection == 1 && len(m.groups) > 0 {
				group := m.groups[m.groupCursor]
				m.showTopics = true
				m.topics = nil
				m.topicCursor = 0
				m.statusMsg = fmt.Sprintf("Loading topics for %s...", safeName(group.Name, group.ID))
				m.rebuildContext()
				return m, fetchTopicsCmd(m.backendURL, m.token, group.ID)
			}
			if m.activePane == 0 && m.leftSection == 0 {
				m.showTopics = false
			}
			m.rebuildContext()

		case "a":
			m.showApprovals = !m.showApprovals
			m.rebuildContext()

		case "i":
			m.inputMode = true
			m.textInput.Focus()

		case "/":
			m.showTopics = false
			m.filterMode = true
			m.filterText = ""
			m.applyMessageFilter()
			m.rebuildMessageViewport()

		case "f":
			if m.activePane == 0 {
				m.leftSection = 0
				m.agentFilterMode = !m.agentFilterMode
				if !m.agentFilterMode {
					m.agentFilterText = ""
				}
				m.adjustAgentCursorToFilter()
				m.rebuildContext()
			}

		case "esc":
			cleared := false
			if strings.TrimSpace(m.filterText) != "" || m.filterMode {
				m.filterMode = false
				m.filterText = ""
				m.applyMessageFilter()
				m.rebuildMessageViewport()
				cleared = true
			}
			if strings.TrimSpace(m.agentFilterText) != "" || m.agentFilterMode {
				m.agentFilterMode = false
				m.agentFilterText = ""
				m.adjustAgentCursorToFilter()
				cleared = true
			}
			if cleared {
				m.rebuildContext()
				return m, nil
			}

		case "r":
			m.statusMsg = "Refreshing agents, groups, and approvals..."
			return m, tea.Batch(fetchAgentsCmd(m.backendURL, m.token), fetchGroupsCmd(m.backendURL, m.token), fetchApprovalsCmd(m.backendURL, m.token))

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
		m.statusMsg = string(msg)

	case agentsMsg:
		m.agents = msg
		m.adjustAgentCursorToFilter()
		online := 0
		for _, a := range m.agents {
			if strings.EqualFold(a.Status, "ONLINE") {
				online++
			}
		}
		m.statusMsg = fmt.Sprintf("Agent list refreshed (%d total, %d online)", len(m.agents), online)
		m.rebuildContext()

	case groupsMsg:
		m.groups = msg
		if m.groupCursor >= len(m.groups) {
			m.groupCursor = maxInt(0, len(m.groups)-1)
		}
		if len(m.groups) == 0 {
			m.leftSection = 0
			m.showTopics = false
			m.topics = nil
			m.topicCursor = 0
		}
		m.statusMsg = fmt.Sprintf("Group list refreshed (%d total)", len(m.groups))
		m.rebuildContext()

	case topicsMsg:
		m.topics = msg
		if m.topicCursor >= len(m.topics) {
			m.topicCursor = maxInt(0, len(m.topics)-1)
		}
		m.showTopics = true
		m.statusMsg = fmt.Sprintf("Topics updated (%d items)", len(m.topics))
		m.rebuildContext()

	case messageMsg:
		m.messages = append(m.messages, messageEntry(msg))
		if len(m.messages) > 2000 {
			m.messages = m.messages[len(m.messages)-2000:]
		}
		m.msgCursor = len(m.messages) - 1
		m.applyMessageFilter()
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
		cmds := []tea.Cmd{tickCmd(), fetchAgentsCmd(m.backendURL, m.token), fetchGroupsCmd(m.backendURL, m.token)}
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

	left := m.renderPane("Agent Directory", m.renderLeftPane(), leftW, bodyH, m.activePane == 0)
	centerTitle := "Messages"
	centerContent := m.msgViewport.View()
	if m.showTopics {
		centerTitle = "Topic Board"
		centerContent = m.renderTopicsBoard(centerW)
	}
	if len(m.filteredMessages) > 0 || strings.TrimSpace(m.filterText) != "" {
		centerTitle = fmt.Sprintf("%s (%d of %d)", centerTitle, len(m.filteredMessages), len(m.messages))
	}
	center := m.renderPane(centerTitle, centerContent, centerW, bodyH, m.activePane == 1)
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
	status := fmt.Sprintf("%s | %d agents online | active pane: %d | Tab:switch i:input /:msg-filter f:agent-filter Esc:clear a:approvals r:refresh q:quit", connStatus, online, m.activePane+1)
	if m.filterMode {
		status = fmt.Sprintf("filter: %q | Enter:apply Esc:clear | %s", m.filterText, status)
	}
	if m.agentFilterMode {
		status = fmt.Sprintf("agent filter: %q | Enter:apply Esc:clear | %s", m.agentFilterText, status)
	}
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

func (m *model) renderLeftPane() string {
	lines := make([]string, 0, len(m.agents)+len(m.groups)+6)
	agentIdx := m.filteredAgentIndices()
	if len(agentIdx) == 0 {
		noAgents := "  No agents found"
		if strings.TrimSpace(m.agentFilterText) != "" {
			noAgents = "  No agents match filter"
		}
		lines = append(lines, mutedStyle.Render(noAgents))
	} else {
		for _, idx := range agentIdx {
			a := m.agents[idx]
			marker := " "
			if m.leftSection == 0 && idx == m.agentCursor {
				marker = "▸"
			}
			statusIcon, statusText := statusGlyph(a.Status)
			line := fmt.Sprintf("%s %s %s [%s]", marker, statusIcon, safeName(a.Name, a.ID), statusText)
			if m.leftSection == 0 && idx == m.agentCursor {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, truncate(line, maxInt(8, m.width/4-4)))
		}
	}

	if strings.TrimSpace(m.agentFilterText) != "" {
		lines = append(lines, mutedStyle.Render("  filter: "+m.agentFilterText))
	}

	lines = append(lines, mutedStyle.Render("───Groups───"))
	if len(m.groups) == 0 {
		lines = append(lines, mutedStyle.Render("  No groups found"))
	} else {
		for i, g := range m.groups {
			marker := " "
			if m.leftSection == 1 && i == m.groupCursor {
				marker = "▸"
			}
			line := fmt.Sprintf("%s %s (%d)", marker, safeName(g.Name, g.ID), g.MemberCount)
			if m.leftSection == 1 && i == m.groupCursor {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, truncate(line, maxInt(8, m.width/4-4)))
		}
	}

	return strings.Join(lines, "\n")
}

func (m *model) renderTopicsBoard(width int) string {
	if len(m.topics) == 0 {
		return mutedStyle.Render("No topics found for selected group")
	}
	maxW := maxInt(10, width-6)
	lines := make([]string, 0, len(m.topics)*2)
	for i, t := range m.topics {
		marker := " "
		if i == m.topicCursor {
			marker = "▸"
		}
		head := fmt.Sprintf("%s %s", marker, truncate(firstNonEmpty(strings.TrimSpace(t.Title), t.ID), maxW-2))
		meta := fmt.Sprintf("  %s", mutedStyle.Render(fmt.Sprintf("status:%s", strings.ToUpper(strings.TrimSpace(t.Status)))))
		if i == m.topicCursor {
			head = selectedStyle.Render(head)
		}
		lines = append(lines, truncate(head, maxW), truncate(meta, maxW))
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

	if m.activePane == 1 && m.showTopics && len(m.topics) > 0 {
		t := m.topics[m.topicCursor]
		selectedGroup := ""
		if len(m.groups) > 0 {
			selectedGroup = safeName(m.groups[m.groupCursor].Name, m.groups[m.groupCursor].ID)
		}
		return strings.Join([]string{
			fmt.Sprintf("Group: %s", selectedGroup),
			fmt.Sprintf("Topic ID: %s", t.ID),
			fmt.Sprintf("Title: %s", t.Title),
			fmt.Sprintf("Status: %s", strings.ToUpper(strings.TrimSpace(t.Status))),
			"Assignee: (not provided)",
			fmt.Sprintf("Group ID: %s", t.GroupID),
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

	if m.leftSection == 1 && len(m.groups) > 0 {
		g := m.groups[m.groupCursor]
		return strings.Join([]string{
			fmt.Sprintf("Group: %s", safeName(g.Name, g.ID)),
			fmt.Sprintf("ID: %s", g.ID),
			fmt.Sprintf("Visibility: %s", firstNonEmpty(strings.TrimSpace(g.Visibility), "UNKNOWN")),
			fmt.Sprintf("Members: %d", g.MemberCount),
			fmt.Sprintf("Topics loaded: %d", len(m.topics)),
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
		agentIdx := m.filteredAgentIndices()
		if m.leftSection == 0 {
			if len(agentIdx) == 0 {
				if len(m.groups) > 0 {
					m.leftSection = 1
				}
				m.rebuildContext()
				return
			}
			pos := m.positionInFilteredAgents(m.agentCursor)
			if pos < 0 {
				pos = 0
			}
			newPos := pos + delta
			if newPos < 0 {
				newPos = 0
			}
			if newPos >= len(agentIdx) {
				if delta > 0 && len(m.groups) > 0 {
					m.leftSection = 1
					m.groupCursor = 0
				} else {
					newPos = len(agentIdx) - 1
				}
			} else {
				m.agentCursor = agentIdx[newPos]
			}
		} else {
			if len(m.groups) == 0 {
				if len(agentIdx) > 0 {
					m.leftSection = 0
				}
				m.rebuildContext()
				return
			}
			newGroup := m.groupCursor + delta
			if newGroup < 0 {
				if len(agentIdx) > 0 {
					m.leftSection = 0
					m.agentCursor = agentIdx[len(agentIdx)-1]
				} else {
					m.groupCursor = 0
				}
			} else {
				m.groupCursor = clamp(newGroup, 0, len(m.groups)-1)
			}
		}
		m.rebuildContext()
	case 1:
		if m.showTopics {
			if len(m.topics) == 0 {
				return
			}
			m.topicCursor = clamp(m.topicCursor+delta, 0, len(m.topics)-1)
		} else {
			vis := m.visibleMessageIndices()
			if len(vis) == 0 {
				return
			}
			pos := m.positionInVisibleMessages(m.msgCursor)
			if pos < 0 {
				pos = len(vis) - 1
			}
			pos = clamp(pos+delta, 0, len(vis)-1)
			m.msgCursor = vis[pos]
			m.rebuildMessageViewport()
		}
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
	visible := m.visibleMessageIndices()
	if len(visible) == 0 {
		m.msgViewport.SetContent(mutedStyle.Render("No messages match current filter"))
		return
	}

	var b strings.Builder
	for _, i := range visible {
		msg := m.messages[i]
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
	pos := m.positionInVisibleMessages(m.msgCursor)
	if pos < 0 {
		pos = 0
	}
	line := pos * 2
	if line > m.msgViewport.Height {
		m.msgViewport.YOffset = maxInt(0, line-m.msgViewport.Height/2)
	}
}

func (m *model) selectedTarget() string {
	agentIdx := m.filteredAgentIndices()
	if len(agentIdx) > 0 {
		if m.positionInFilteredAgents(m.agentCursor) < 0 {
			m.agentCursor = agentIdx[0]
		}
		return m.agents[m.agentCursor].ID
	}
	return ""
}

func (m *model) visibleMessageIndices() []int {
	if strings.TrimSpace(m.filterText) == "" {
		idx := make([]int, 0, len(m.messages))
		for i := range m.messages {
			idx = append(idx, i)
		}
		return idx
	}
	return m.filteredMessages
}

func (m *model) positionInVisibleMessages(absIndex int) int {
	vis := m.visibleMessageIndices()
	for i, idx := range vis {
		if idx == absIndex {
			return i
		}
	}
	return -1
}

func (m *model) applyMessageFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterText))
	if query == "" {
		m.filteredMessages = nil
		if len(m.messages) > 0 {
			m.msgCursor = clamp(m.msgCursor, 0, len(m.messages)-1)
		}
		return
	}

	filtered := make([]int, 0, len(m.messages))
	for i, msg := range m.messages {
		blob := strings.ToLower(msg.Tag + "\n" + msg.From + "\n" + msg.To + "\n" + msg.Payload)
		if strings.Contains(blob, query) {
			filtered = append(filtered, i)
		}
	}
	m.filteredMessages = filtered
	if len(filtered) == 0 {
		return
	}
	if m.positionInVisibleMessages(m.msgCursor) < 0 {
		m.msgCursor = filtered[len(filtered)-1]
	}
}

func (m *model) filteredAgentIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.agentFilterText))
	indices := make([]int, 0, len(m.agents))
	for i, a := range m.agents {
		if query == "" {
			indices = append(indices, i)
			continue
		}
		caps := strings.ToLower(strings.Join(a.Capabilities, " "))
		name := strings.ToLower(safeName(a.Name, a.ID))
		if strings.Contains(name, query) || strings.Contains(caps, query) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *model) positionInFilteredAgents(absIndex int) int {
	filtered := m.filteredAgentIndices()
	for i, idx := range filtered {
		if idx == absIndex {
			return i
		}
	}
	return -1
}

func (m *model) adjustAgentCursorToFilter() {
	filtered := m.filteredAgentIndices()
	if len(filtered) == 0 {
		m.agentCursor = 0
		if m.leftSection == 0 && len(m.groups) > 0 {
			m.leftSection = 1
		}
		return
	}
	if m.positionInFilteredAgents(m.agentCursor) < 0 {
		m.agentCursor = filtered[0]
	}
	if m.leftSection == 1 || len(m.groups) == 0 {
		m.leftSection = 0
	}
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
				ApprovalID    string `json:"approval_id"`
				AgentID       string `json:"agent_id"`
				Action        string `json:"action"`
				Urgency       string `json:"urgency"`
				Justification string `json:"justification"`
				CreatedAt     string `json:"created_at"`
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

func fetchGroupsCmd(backendURL, token string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(backendURL, "/")+"/v1/groups", nil)
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		cli := &http.Client{Timeout: 10 * time.Second}
		resp, err := cli.Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("fetch groups: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("fetch groups failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body))))
		}

		var payload struct {
			Groups []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Visibility  string `json:"visibility"`
				MemberCount int    `json:"member_count"`
			} `json:"groups"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return errMsg(fmt.Errorf("decode groups response: %w", err))
		}

		groups := make([]groupEntry, 0, len(payload.Groups))
		for _, g := range payload.Groups {
			groups = append(groups, groupEntry{
				ID:          strings.TrimSpace(g.ID),
				Name:        strings.TrimSpace(g.Name),
				Visibility:  strings.TrimSpace(g.Visibility),
				MemberCount: g.MemberCount,
			})
		}

		sort.SliceStable(groups, func(i, j int) bool {
			return safeName(groups[i].Name, groups[i].ID) < safeName(groups[j].Name, groups[j].ID)
		})

		return groupsMsg(groups)
	}
}

func fetchTopicsCmd(backendURL, token, groupID string) tea.Cmd {
	return func() tea.Msg {
		gid := strings.TrimSpace(groupID)
		if gid == "" {
			return errMsg(errors.New("missing group id"))
		}
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(backendURL, "/")+"/v1/groups/"+url.PathEscape(gid)+"/topics", nil)
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		cli := &http.Client{Timeout: 10 * time.Second}
		resp, err := cli.Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("fetch topics: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("fetch topics failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body))))
		}

		var payload struct {
			Topics []struct {
				ID      string `json:"id"`
				GroupID string `json:"group_id"`
				Title   string `json:"title"`
				Status  string `json:"status"`
			} `json:"topics"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return errMsg(fmt.Errorf("decode topics response: %w", err))
		}

		topics := make([]topicEntry, 0, len(payload.Topics))
		for _, t := range payload.Topics {
			topics = append(topics, topicEntry{
				ID:      strings.TrimSpace(t.ID),
				GroupID: strings.TrimSpace(t.GroupID),
				Title:   strings.TrimSpace(t.Title),
				Status:  strings.TrimSpace(t.Status),
			})
		}

		sort.SliceStable(topics, func(i, j int) bool {
			return firstNonEmpty(topics[i].Title, topics[i].ID) < firstNonEmpty(topics[j].Title, topics[j].ID)
		})

		return topicsMsg(topics)
	}
}

func joinGroupCmd(backendURL, token, groupID string) tea.Cmd {
	return func() tea.Msg {
		gid := strings.TrimSpace(groupID)
		if gid == "" {
			return errMsg(errors.New("missing group id"))
		}
		endpoint := strings.TrimRight(backendURL, "/") + "/v1/groups/" + url.PathEscape(gid) + "/join"
		req, err := http.NewRequest(http.MethodPost, endpoint, nil)
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("join group: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("join group failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))))
		}
		return statusUpdateMsg("joined group " + gid)
	}
}

func leaveGroupCmd(backendURL, token, groupID string) tea.Cmd {
	return func() tea.Msg {
		gid := strings.TrimSpace(groupID)
		if gid == "" {
			return errMsg(errors.New("missing group id"))
		}
		endpoint := strings.TrimRight(backendURL, "/") + "/v1/groups/" + url.PathEscape(gid) + "/leave"
		req, err := http.NewRequest(http.MethodPost, endpoint, nil)
		if err != nil {
			return errMsg(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			return errMsg(fmt.Errorf("leave group: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return errMsg(fmt.Errorf("leave group failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))))
		}
		return statusUpdateMsg("left group " + gid)
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
	left = total * 3 / 10
	center = total * 45 / 100
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
