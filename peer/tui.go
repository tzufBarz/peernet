package main

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model struct {
	viewport  viewport.Model
	textinput textinput.Model
	local     *LocalPeer
	history   []string
}

type LogEvent string

type ChatEvent struct {
	Sender  string
	Content string
}

var headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#5A56E0")).Bold(true).Width(sha256.Size * 2)

func initialModel(local *LocalPeer) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	vp := viewport.New()

	return model{
		textinput: ti,
		viewport:  vp,
		local:     local,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 2
		footerHeight := 1

		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height - headerHeight - footerHeight)
		m.textinput.SetWidth(msg.Width)

		m.viewport.SetContent(m.viewportContent())
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			val := strings.TrimSpace(m.textinput.Value())
			if val != "" {
				m.writeLinef("> %s", val)
				m.textinput.SetValue("")

				exit, err := executeCommand(m.local, val)
				if err != nil {
					m.writeLinef("Error executing command: %v", err)
				}

				if exit {
					return m, tea.Quit
				}
			}
		}
	case LogEvent:
		m.writeLine(string(msg))
	case ChatEvent:
		m.writeLinef("<%s> %s", msg.Sender, msg.Content)
	}

	m.textinput, tiCmd = m.textinput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() tea.View {
	header := headerStyle.Render(fmt.Sprintf("%x\nListening on :%d", m.local.identity.PeerID, m.local.listenPort))

	s := fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), m.textinput.View())

	return tea.NewView(s)
}

func (m model) viewportContent() string {
	rawContent := strings.Join(m.history, "\n")

	wrappedContent := lipgloss.NewStyle().Width(m.viewport.Width()).Render(rawContent)

	contentHeight := lipgloss.Height(wrappedContent)

	if contentHeight >= m.viewport.Height() {
		return rawContent
	}

	padding := strings.Repeat("\n", m.viewport.Height()-contentHeight)
	return padding + rawContent
}

func (m *model) writeLine(line string) {
	m.history = append(m.history, line)

	m.viewport.SetContent(m.viewportContent())
	m.viewport.GotoBottom()
}

func (m *model) writeLinef(format string, a ...any) {
	m.writeLine(fmt.Sprintf(format, a...))
}
