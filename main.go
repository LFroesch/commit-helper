package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GitChange struct {
	File   string
	Status string
}

func (g GitChange) Title() string {
	return fmt.Sprintf("%s %s", getStatusIcon(g.Status), g.File)
}

func (g GitChange) Description() string {
	return fmt.Sprintf("Status: %s", g.Status)
}

func (g GitChange) FilterValue() string {
	return g.File
}

type CommitSuggestion struct {
	Message string
	Type    string
}

func (c CommitSuggestion) Title() string {
	return c.Message
}

func (c CommitSuggestion) Description() string {
	return fmt.Sprintf("Type: %s", c.Type)
}

func (c CommitSuggestion) FilterValue() string {
	return c.Message
}

type model struct {
	state       string // "files", "suggestions", "custom"
	changes     []GitChange
	suggestions []CommitSuggestion
	
	filesList        list.Model
	suggestionsList  list.Model
	customInput      textinput.Model
	
	width        int
	height       int
	statusMsg    string
	statusExpiry time.Time
	
	repoPath string
}

type statusMsg struct {
	message string
}

type gitChangesMsg []GitChange
type commitSuggestionsMsg []CommitSuggestion

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)
	
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)
	
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)
	
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

func main() {
	repoPath, err := findGitRepo()
	if err != nil {
		log.Fatal("Error: Not in a git repository")
	}

	m := model{
		state:    "files",
		repoPath: repoPath,
		width:    100,
		height:   24,
	}

	// Initialize lists
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	
	m.filesList = list.New([]list.Item{}, delegate, 0, 0)
	m.filesList.Title = "📁 Changed Files"
	m.filesList.SetShowStatusBar(false)
	m.filesList.SetFilteringEnabled(false)
	m.filesList.SetShowHelp(false)
	
	m.suggestionsList = list.New([]list.Item{}, delegate, 0, 0)
	m.suggestionsList.Title = "💡 Commit Suggestions"
	m.suggestionsList.SetShowStatusBar(false)
	m.suggestionsList.SetFilteringEnabled(false)
	m.suggestionsList.SetShowHelp(false)
	
	// Initialize custom input
	m.customInput = textinput.New()
	m.customInput.Placeholder = "Enter your custom commit message..."
	m.customInput.CharLimit = 200

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func findGitRepo() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("Git Commit Helper"),
		m.loadGitChanges(),
	)
}

func (m model) loadGitChanges() tea.Cmd {
	return func() tea.Msg {
		changes, err := getGitChanges(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load changes: %v", err)}
		}
		return gitChangesMsg(changes)
	}
}

func (m model) generateSuggestions() tea.Cmd {
	return func() tea.Msg {
		suggestions := analyzeChangesForCommits(m.changes)
		return commitSuggestionsMsg(suggestions)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case gitChangesMsg:
		m.changes = []GitChange(msg)
		
		// Convert to list items
		items := make([]list.Item, len(m.changes))
		for i, change := range m.changes {
			items[i] = change
		}
		m.filesList.SetItems(items)
		
		// Auto-generate suggestions
		cmds = append(cmds, m.generateSuggestions())
		m.statusMsg = fmt.Sprintf("✅ Loaded %d changed files", len(m.changes))
		m.statusExpiry = time.Now().Add(3 * time.Second)
		
		return m, tea.Batch(cmds...)

	case commitSuggestionsMsg:
		m.suggestions = []CommitSuggestion(msg)
		
		// Convert to list items
		items := make([]list.Item, len(m.suggestions))
		for i, suggestion := range m.suggestions {
			items[i] = suggestion
		}
		m.suggestionsList.SetItems(items)
		
		m.statusMsg = fmt.Sprintf("🤖 Generated %d commit suggestions", len(m.suggestions))
		m.statusExpiry = time.Now().Add(3 * time.Second)
		
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		listHeight := m.height - 8
		m.filesList.SetSize(m.width-4, listHeight)
		m.suggestionsList.SetSize(m.width-4, listHeight)
		
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "1":
			m.state = "files"
			return m, nil
			
		case "2":
			if len(m.suggestions) > 0 {
				m.state = "suggestions"
			}
			return m, nil
			
		case "3":
			m.state = "custom"
			m.customInput.Focus()
			return m, nil

		case "r":
			return m, tea.Batch(
				m.loadGitChanges(),
				func() tea.Msg {
					return statusMsg{message: "🔄 Refreshing..."}
				},
			)

		case "a":
			return m, m.gitAddAll()

		case "p":
			return m, m.gitPush()

		case "s":
			return m, m.gitStatus()

		case "enter":
			switch m.state {
			case "suggestions":
				if len(m.suggestions) > 0 {
					selected := m.suggestionsList.SelectedItem()
					if suggestion, ok := selected.(CommitSuggestion); ok {
						return m, m.commitWithMessage(suggestion.Message)
					}
				}
			case "custom":
				if m.customInput.Value() != "" {
					return m, m.commitWithMessage(m.customInput.Value())
				}
			}
			return m, nil

		case "esc":
			if m.state == "custom" {
				m.customInput.Blur()
				m.customInput.SetValue("")
				m.state = "files"
			}
			return m, nil
		}
	}

	// Update the appropriate component based on state
	switch m.state {
	case "files":
		m.filesList, cmd = m.filesList.Update(msg)
	case "suggestions":
		m.suggestionsList, cmd = m.suggestionsList.Update(msg)
	case "custom":
		m.customInput, cmd = m.customInput.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	var content string
	
	// Header
	header := titleStyle.Render("🚀 Git Commit Helper")
	repoInfo := helpStyle.Render(fmt.Sprintf("Repository: %s", filepath.Base(m.repoPath)))
	
	// Navigation tabs
	tabs := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderTab("1", "📁 Files", m.state == "files"),
		m.renderTab("2", "💡 Suggestions", m.state == "suggestions"),
		m.renderTab("3", "✏️  Custom", m.state == "custom"),
	)
	
	// Content based on current state
	switch m.state {
	case "files":
		if len(m.changes) == 0 {
			content = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render("No changes found. Run 'git add' to stage files or make some changes.")
		} else {
			content = m.filesList.View()
		}
		
	case "suggestions":
		if len(m.suggestions) == 0 {
			content = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render("No suggestions available. Please add some files first.")
		} else {
			content = m.suggestionsList.View()
		}
		
	case "custom":
		inputLabel := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Render("Custom Commit Message:")
		content = fmt.Sprintf("%s\n\n%s", inputLabel, m.customInput.View())
	}
	
	// Footer with help and status
	var footer string
	switch m.state {
	case "files":
		footer = "1-3: switch mode • ↑↓: navigate • r: refresh • a: git add • s: status • q: quit"
	case "suggestions":
		footer = "1-3: switch mode • ↑↓: navigate • enter: commit • a: git add • p: push • q: quit"
	case "custom":
		footer = "1-3: switch mode • enter: commit • esc: cancel • a: git add • p: push • q: quit"
	}
	
	// Add status message if present
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		var statusColor lipgloss.Color = "86"
		if strings.Contains(m.statusMsg, "❌") {
			statusColor = "196"
		}
		statusLine := lipgloss.NewStyle().
			Foreground(statusColor).
			Bold(true).
			Render(" > " + m.statusMsg)
		footer = footer + "\n" + statusLine
	}
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		repoInfo,
		"",
		tabs,
		"",
		content,
		"",
		helpStyle.Render(footer),
	)
}

func (m model) renderTab(key, label string, active bool) string {
	style := lipgloss.NewStyle().Padding(0, 2)
	
	if active {
		style = style.
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("240"))
	} else {
		style = style.Foreground(lipgloss.Color("240"))
	}
	
	return style.Render(fmt.Sprintf("[%s] %s", key, label))
}

func (m model) gitAddAll() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = m.repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Git add failed: %v - %s", err, string(output))}
		}

		return statusMsg{message: "✅ All changes staged (git add .)"}
	}
}

func (m model) gitPush() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "push")
		cmd.Dir = m.repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Git push failed: %v - %s", err, string(output))}
		}

		return statusMsg{message: "✅ Pushed to remote repository"}
	}
}

func (m model) gitStatus() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "status", "--short")
		cmd.Dir = m.repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Git status failed: %v", err)}
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) == 1 && lines[0] == "" {
			return statusMsg{message: "✅ Working tree clean"}
		}

		return statusMsg{message: fmt.Sprintf("📊 %d files modified", len(lines))}
	}
}

// Git operation functions
func (m model) commitWithMessage(message string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "commit", "-m", message)
		cmd.Dir = m.repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Commit failed: %v - %s", err, string(output))}
		}

		return statusMsg{message: fmt.Sprintf("✅ Committed: %s", message)}
	}
}

func getGitChanges(repoPath string) ([]GitChange, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var changes []GitChange
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		status := strings.TrimSpace(line[:2])
		file := strings.TrimSpace(line[3:])

		change := GitChange{
			File:   file,
			Status: status,
		}

		changes = append(changes, change)
	}

	return changes, nil
}

func analyzeChangesForCommits(changes []GitChange) []CommitSuggestion {
	var suggestions []CommitSuggestion

	// Group by type for better suggestions
	typeGroups := make(map[string][]GitChange)
	for _, change := range changes {
		commitType := determineCommitType(change.File, change.Status)
		typeGroups[commitType] = append(typeGroups[commitType], change)
	}

	for commitType, groupChanges := range typeGroups {
		var message string

		if len(groupChanges) == 1 {
			message = generateCommitMessage(groupChanges[0].File, groupChanges[0].Status, commitType)
		} else {
			switch commitType {
			case "feat":
				message = fmt.Sprintf("add %d new features", len(groupChanges))
			case "fix":
				message = fmt.Sprintf("fix %d issues", len(groupChanges))
			case "docs":
				message = fmt.Sprintf("update documentation (%d files)", len(groupChanges))
			case "test":
				message = fmt.Sprintf("add tests (%d files)", len(groupChanges))
			case "chore":
				message = fmt.Sprintf("update configuration (%d files)", len(groupChanges))
			default:
				message = fmt.Sprintf("update %d files", len(groupChanges))
			}
		}

		suggestion := CommitSuggestion{
			Type:    commitType,
			Message: message,
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions
}

func determineCommitType(file, status string) string {
	if strings.Contains(status, "A") {
		return "feat"
	}

	if strings.Contains(status, "D") {
		return "chore"
	}

	if strings.HasSuffix(file, ".md") || strings.Contains(file, "README") || strings.Contains(file, "doc") {
		return "docs"
	}

	if strings.Contains(file, "test") || strings.Contains(file, ".test.") || strings.Contains(file, "_test.") {
		return "test"
	}

	if strings.Contains(file, "config") || strings.HasSuffix(file, ".json") || strings.HasSuffix(file, ".yaml") {
		return "chore"
	}

	if strings.Contains(status, "M") {
		return "feat"
	}

	return "chore"
}

func generateCommitMessage(file, status, commitType string) string {
	fileName := filepath.Base(file)
	fileExt := filepath.Ext(file)
	baseName := strings.TrimSuffix(fileName, fileExt)

	switch status {
	case "A":
		return fmt.Sprintf("add %s", fileName)
	case "D":
		return fmt.Sprintf("remove %s", fileName)
	case "M":
		return fmt.Sprintf("update %s", baseName)
	case "R":
		return fmt.Sprintf("rename %s", fileName)
	default:
		return fmt.Sprintf("modify %s", fileName)
	}
}

func getStatusIcon(status string) string {
	switch status {
	case "A":
		return "➕"
	case "M":
		return "📝"
	case "D":
		return "🗑️"
	case "R":
		return "📛"
	case "C":
		return "📋"
	case "U":
		return "⚠️"
	case "??":
		return "❓"
	default:
		return "📄"
	}
}
