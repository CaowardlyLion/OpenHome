package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CaowardlyLion/OpenHome/internal/agents"
	"github.com/CaowardlyLion/OpenHome/internal/commands"
	"github.com/CaowardlyLion/OpenHome/internal/permissions"
)

type Handler interface {
	Handle(context.Context, string) (agents.Result, error)
	NewSession()
}

type Model struct {
	handler             Handler
	commands            *commands.Registry
	viewport            viewport.Model
	input               textarea.Model
	spinner             spinner.Model
	transcript          []transcriptEntry
	status              string
	busy                bool
	verbose             bool
	permissions         *permissions.Manager
	prompter            *permissions.ChannelPrompter
	approval            *permissions.Pending
	approvalChoice      int
	choosingPermissions bool
	permissionChoice    int
	confirmAllow        bool
	cancel              context.CancelFunc
	width               int
	height              int
}

type transcriptEntry struct {
	label string
	text  string
}

type StatusMsg agents.StatusEvent
type approvalMsg permissions.Pending
type resultMsg struct {
	result agents.Result
	err    error
}

func New(handler Handler, options ...any) Model {
	input := textarea.New()
	input.Placeholder = "Ask OpenHome..."
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.Focus()
	model := Model{
		handler: handler, commands: commands.Default(), viewport: viewport.New(), input: input,
		spinner: spinner.New(), transcript: []transcriptEntry{
			{label: "system", text: "OpenHome"},
			{label: "system", text: "Type a request or use /new and /exit."},
		},
		width: 80, height: 24,
	}
	for _, option := range options {
		switch value := option.(type) {
		case *permissions.Manager:
			model.permissions = value
		case *permissions.ChannelPrompter:
			model.prompter = value
		}
	}
	model.resize()
	model.refreshTranscript()
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.input.Focus(), waitForApproval(m.prompter))
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var commandsToRun []tea.Cmd
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.resize()
	case tea.KeyPressMsg:
		if m.approval != nil {
			switch message.String() {
			case "up":
				m.approvalChoice = max(0, m.approvalChoice-1)
				return m, nil
			case "down":
				m.approvalChoice = min(2, m.approvalChoice+1)
				return m, nil
			case "enter":
				return m.resolveApproval([]permissions.Decision{permissions.AllowOnce, permissions.AllowSimilar, permissions.Deny}[m.approvalChoice])
			case "1", "a":
				return m.resolveApproval(permissions.AllowOnce)
			case "2", "s":
				return m.resolveApproval(permissions.AllowSimilar)
			case "3", "d", "esc":
				return m.resolveApproval(permissions.Deny)
			}
			return m, nil
		}
		if m.confirmAllow {
			switch message.String() {
			case "y", "enter":
				if m.permissions != nil {
					_ = m.permissions.SetMode(permissions.Allow)
				}
				m.confirmAllow = false
				m.appendTranscript("system", "Permission mode set to allow.")
			case "n", "esc":
				m.confirmAllow = false
			}
			return m, nil
		}
		if m.choosingPermissions {
			switch message.String() {
			case "up":
				m.permissionChoice = max(0, m.permissionChoice-1)
			case "down":
				m.permissionChoice = min(2, m.permissionChoice+1)
			case "enter":
				m.choosePermissionMode()
			case "1", "a":
				m.setPermissionMode(permissions.Ask)
			case "2", "d":
				m.setPermissionMode(permissions.Default)
			case "3", "l":
				m.choosingPermissions = false
				m.confirmAllow = true
			case "esc":
				m.choosingPermissions = false
			}
			return m, nil
		}
		switch message.String() {
		case "ctrl+c":
			if m.busy && m.cancel != nil {
				m.cancel()
				m.status = "Cancelling current task"
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if m.busy {
				m.status = "Current task is still running. Press Ctrl+C to cancel it."
				return m, nil
			}
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				return m, nil
			}
			m.input.Reset()
			if strings.HasPrefix(value, "/permissions") {
				return m.handlePermissionsCommand(value)
			}
			if command, ok := m.commands.Lookup(value); ok {
				if command.Action == commands.Exit {
					return m, tea.Quit
				}
				if command.Action == commands.Verbose {
					m.verbose = !m.verbose
					state := "hidden"
					if m.verbose {
						state = "shown"
					}
					m.appendTranscript("system", "Verbose execution details "+state+".")
					return m, nil
				}
				m.handler.NewSession()
				m.appendTranscript("system", "Started new session.")
				return m, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			m.busy = true
			m.status = "Understanding request"
			m.appendTranscript("you", value)
			return m, runTask(m.handler, ctx, value)
		}
	case StatusMsg:
		event := agents.StatusEvent(message)
		if event.Type == "token" {
			return m, nil
		}
		m.status = event.Message
		switch event.Type {
		case "intent", "lane", "plan", "skill", "tool", "verification":
			if m.verbose {
				m.appendTranscript(event.Type, event.Message)
			}
		}
	case approvalMsg:
		pending := permissions.Pending(message)
		m.approval = &pending
		m.approvalChoice = 0
		m.status = "Waiting for permission"
	case resultMsg:
		m.busy = false
		m.cancel = nil
		if message.err != nil {
			m.status = "Task failed"
			m.appendTranscript("error", message.err.Error())
		} else {
			m.status = "Ready"
			m.appendTranscript("assistant", message.result.Answer)
			m.appendTranscript("log", message.result.LogPath)
		}
	case spinner.TickMsg:
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(message)
		commandsToRun = append(commandsToRun, command)
	}
	var command tea.Cmd
	m.input, command = m.input.Update(message)
	commandsToRun = append(commandsToRun, command)
	m.viewport, command = m.viewport.Update(message)
	commandsToRun = append(commandsToRun, command)
	return m, tea.Batch(commandsToRun...)
}

func (m Model) View() tea.View {
	status := m.status
	if status == "" {
		status = "Ready"
	}
	if m.busy {
		status = m.spinner.View() + " " + status
	}
	status = wrapText(status, m.width)
	header := lipgloss.NewStyle().Bold(true).Render("OpenHome")
	verbose := "off"
	if m.verbose {
		verbose = "on"
	}
	mode := permissions.Default
	if m.permissions != nil {
		mode = m.permissions.Mode()
	}
	footer := lipgloss.NewStyle().Faint(true).Render(wrapText("/new  /verbose details:"+verbose+"  /permissions:"+string(mode)+"  /exit  Ctrl+C", m.width))
	extra := ""
	if m.approval != nil {
		extra = fmt.Sprintf("Permission required: %s\nReason: %s\nTarget: %s\n%s\n%s",
			m.approval.Request.Tool, m.approval.Request.Reason, m.approval.Request.Target, m.approval.Request.Details,
			choiceLine(m.approvalChoice, "Allow once", "Always allow similar", "Deny"))
	} else if m.confirmAllow {
		extra = "Always allow advanced tools for this process? [y] Confirm  [n] Cancel"
	} else if m.choosingPermissions {
		extra = "Permission mode: " + choiceLine(m.permissionChoice, "Always ask", "Default", "Always allow")
	}
	extra = wrapText(extra, m.width)
	viewport := m.viewport
	sections := []string{header, "", status, extra, m.input.View(), footer}
	chromeHeight := len(sections) - 1
	for _, section := range sections {
		if section != "" {
			chromeHeight += lipgloss.Height(section)
		}
	}
	viewport.SetHeight(max(1, m.height-chromeHeight))
	sections[1] = viewport.View()
	return tea.NewView(strings.Join(sections, "\n"))
}

func (m Model) resolveApproval(decision permissions.Decision) (tea.Model, tea.Cmd) {
	m.approval.Response <- decision
	m.approval = nil
	m.status = "Permission response sent"
	return m, waitForApproval(m.prompter)
}

func (m *Model) setPermissionMode(mode permissions.Mode) {
	if m.permissions != nil {
		_ = m.permissions.SetMode(mode)
	}
	m.choosingPermissions = false
	m.appendTranscript("system", "Permission mode set to "+string(mode)+".")
}

func (m Model) handlePermissionsCommand(value string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(value)
	if len(fields) == 1 {
		m.choosingPermissions = true
		m.permissionChoice = 1
		return m, nil
	}
	mode := permissions.Mode(fields[1])
	if mode == permissions.Allow {
		m.confirmAllow = true
		return m, nil
	}
	if mode != permissions.Ask && mode != permissions.Default {
		m.appendTranscript("error", "Usage: /permissions ask|default|allow")
		return m, nil
	}
	m.setPermissionMode(mode)
	return m, nil
}

func (m *Model) choosePermissionMode() {
	switch m.permissionChoice {
	case 0:
		m.setPermissionMode(permissions.Ask)
	case 1:
		m.setPermissionMode(permissions.Default)
	case 2:
		m.choosingPermissions = false
		m.confirmAllow = true
	}
}

func choiceLine(selected int, choices ...string) string {
	result := make([]string, len(choices))
	for index, choice := range choices {
		prefix := " "
		if index == selected {
			prefix = ">"
		}
		result[index] = fmt.Sprintf("%s[%d] %s", prefix, index+1, choice)
	}
	return strings.Join(result, "  ")
}

func waitForApproval(prompter *permissions.ChannelPrompter) tea.Cmd {
	if prompter == nil {
		return nil
	}
	return func() tea.Msg {
		return approvalMsg(<-prompter.Requests)
	}
}

func (m *Model) appendTranscript(label, text string) {
	m.transcript = append(m.transcript, transcriptEntry{label: label, text: text})
	m.refreshTranscript()
	m.viewport.GotoBottom()
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(max(1, m.height-8))
	m.input.SetWidth(m.width)
	m.refreshTranscript()
}

func (m *Model) refreshTranscript() {
	entries := make([]string, len(m.transcript))
	for index, entry := range m.transcript {
		entries[index] = renderTranscriptEntry(entry, m.width)
	}
	m.viewport.SetContent(strings.Join(entries, "\n\n"))
}

func renderTranscriptEntry(entry transcriptEntry, width int) string {
	prefix := "[" + entry.label + "] "
	bodyWidth := max(10, width-lipgloss.Width(prefix))
	if entry.label == "assistant" {
		return prefix + renderMarkdown(entry.text, bodyWidth, strings.Repeat(" ", lipgloss.Width(prefix)))
	}
	return wrapText(prefix+entry.text, width)
}

func renderMarkdown(text string, width int, continuation string) string {
	var lines []string
	inCode := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			lines = append(lines, continuation+trimmed)
			continue
		}
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		if inCode {
			lines = append(lines, continuation+"  "+line)
			continue
		}
		marker, rest := markdownMarker(trimmed)
		if marker != "" {
			indent := continuation + strings.Repeat(" ", lipgloss.Width(marker))
			lines = append(lines, continuation+marker+wrapText(rest, width-lipgloss.Width(marker)))
			if len(lines) > 0 {
				lines[len(lines)-1] = strings.ReplaceAll(lines[len(lines)-1], "\n", "\n"+indent)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			lines = append(lines, continuation+lipgloss.NewStyle().Bold(true).Render(heading))
			continue
		}
		lines = append(lines, continuation+wrapText(trimmed, width))
	}
	if len(lines) == 0 {
		return ""
	}
	first := strings.TrimPrefix(lines[0], continuation)
	lines[0] = first
	return strings.Join(lines, "\n")
}

func markdownMarker(line string) (string, string) {
	for _, marker := range []string{"- ", "* "} {
		if strings.HasPrefix(line, marker) {
			return marker, strings.TrimSpace(line[len(marker):])
		}
	}
	dot := strings.Index(line, ". ")
	if dot > 0 {
		for _, char := range line[:dot] {
			if char < '0' || char > '9' {
				return "", ""
			}
		}
		return line[:dot+2], strings.TrimSpace(line[dot+2:])
	}
	return "", ""
}

func wrapText(text string, width int) string {
	if width <= 0 || text == "" {
		return text
	}
	return lipgloss.Wrap(text, width, "")
}

func runTask(handler Handler, ctx context.Context, input string) tea.Cmd {
	return func() tea.Msg {
		result, err := handler.Handle(ctx, input)
		return resultMsg{result: result, err: err}
	}
}
