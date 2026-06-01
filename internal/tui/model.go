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
)

type Handler interface {
	Handle(context.Context, string) (agents.Result, error)
	NewSession()
}

type Model struct {
	handler    Handler
	commands   *commands.Registry
	viewport   viewport.Model
	input      textarea.Model
	spinner    spinner.Model
	transcript []string
	status     string
	busy       bool
	verbose    bool
	cancel     context.CancelFunc
	width      int
	height     int
}

type StatusMsg agents.StatusEvent
type resultMsg struct {
	result agents.Result
	err    error
}

func New(handler Handler) Model {
	input := textarea.New()
	input.Placeholder = "Ask OpenHome..."
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.Focus()
	model := Model{
		handler: handler, commands: commands.Default(), viewport: viewport.New(), input: input,
		spinner: spinner.New(), transcript: []string{"OpenHome", "Type a request or use /new and /exit."},
		width: 80, height: 24,
	}
	model.resize()
	model.viewport.SetContent(strings.Join(model.transcript, "\n\n"))
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.input.Focus())
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var commandsToRun []tea.Cmd
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.resize()
	case tea.KeyPressMsg:
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
	header := lipgloss.NewStyle().Bold(true).Render("OpenHome")
	verbose := "off"
	if m.verbose {
		verbose = "on"
	}
	footer := lipgloss.NewStyle().Faint(true).Render("/new new session  /verbose details:" + verbose + "  /exit quit  Ctrl+C cancel or quit")
	return tea.NewView(strings.Join([]string{header, m.viewport.View(), status, m.input.View(), footer}, "\n"))
}

func (m *Model) appendTranscript(label, text string) {
	m.transcript = append(m.transcript, fmt.Sprintf("[%s] %s", label, text))
	m.viewport.SetContent(strings.Join(m.transcript, "\n\n"))
	m.viewport.GotoBottom()
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(max(1, m.height-8))
	m.input.SetWidth(m.width)
}

func runTask(handler Handler, ctx context.Context, input string) tea.Cmd {
	return func() tea.Msg {
		result, err := handler.Handle(ctx, input)
		return resultMsg{result: result, err: err}
	}
}
