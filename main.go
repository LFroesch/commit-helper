package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GitChange struct {
	File             string `json:"file"`
	Status           string `json:"status"`
	SuggestedType    string `json:"suggested_type"`
	SuggestedScope   string `json:"suggested_scope"`
	SuggestedMessage string `json:"suggested_message"`
}

type CommitSuggestion struct {
	Type       string  `json:"type"`
	Scope      string  `json:"scope"`
	Message    string  `json:"message"`
	Breaking   bool    `json:"breaking"`
	Confidence float64 `json:"confidence"`
}

type model struct {
	currentPage string // "changes", "suggestions", "history"
	changes     []GitChange
	suggestions []CommitSuggestion

	changesTable     table.Model
	suggestionsTable table.Model
	historyTable     table.Model

	editMode  bool
	editRow   int
	editCol   int
	textInput textinput.Model

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

func showStatus(msg string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{message: msg}
	}
}

func main() {
	repoPath, err := findGitRepo()
	if err != nil {
		log.Fatal("Error: Not in a git repository")
	}

	m := model{
		currentPage: "changes",
		repoPath:    repoPath,
		width:       100,
		height:      24,
		editMode:    false,
		editRow:     -1,
		editCol:     -1,
	}

	m.textInput = textinput.New()
	m.textInput.CharLimit = 200
	m.initializeTables()

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

func (m *model) initializeTables() {
	changeColumns := []table.Column{
		{Title: "Status", Width: 8},
		{Title: "File", Width: 40},
		{Title: "Type", Width: 12},
		{Title: "Scope", Width: 15},
		{Title: "Message", Width: 35},
	}

	m.changesTable = table.New(
		table.WithColumns(changeColumns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	suggestionColumns := []table.Column{
		{Title: "Type", Width: 12},
		{Title: "Scope", Width: 15},
		{Title: "Message", Width: 50},
		{Title: "Confidence", Width: 12},
		{Title: "Breaking", Width: 10},
	}

	m.suggestionsTable = table.New(
		table.WithColumns(suggestionColumns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	historyColumns := []table.Column{
		{Title: "Hash", Width: 8},
		{Title: "Type", Width: 12},
		{Title: "Scope", Width: 15},
		{Title: "Message", Width: 45},
		{Title: "Author", Width: 15},
		{Title: "Date", Width: 12},
	}

	m.historyTable = table.New(
		table.WithColumns(historyColumns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	m.changesTable.SetStyles(s)
	m.suggestionsTable.SetStyles(s)
	m.historyTable.SetStyles(s)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("Git Commit Message Generator"),
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

	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case gitChangesMsg:
		m.changes = []GitChange(msg)
		m.updateChangesTable()
		return m, tea.Batch(
			showStatus(fmt.Sprintf("✅ Loaded %d changes", len(m.changes))),
			m.generateSuggestions(),
		)

	case commitSuggestionsMsg:
		m.suggestions = []CommitSuggestion(msg)
		m.updateSuggestionsTable()
		return m, showStatus(fmt.Sprintf("🤖 Generated %d suggestions", len(m.suggestions)))

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustLayout()
		return m, nil

	case tea.KeyMsg:
		if m.editMode {
			return m.updateEdit(msg)
		}
		return m.updateNormal(msg)
	}

	if !m.editMode {
		switch m.currentPage {
		case "changes":
			m.changesTable, cmd = m.changesTable.Update(msg)
		case "suggestions":
			m.suggestionsTable, cmd = m.suggestionsTable.Update(msg)
		case "history":
			m.historyTable, cmd = m.historyTable.Update(msg)
		}
	}

	return m, cmd
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "1":
		m.currentPage = "changes"
		return m, nil
	case "2":
		m.currentPage = "suggestions"
		return m, nil
	case "3":
		m.currentPage = "history"
		return m, m.loadGitHistory()

	case "e":
		m.startEdit()
		return m, nil
	case "r":
		return m, tea.Batch(
			m.loadGitChanges(),
			showStatus("🔄 Refreshing..."),
		)
	case "g":
		return m, m.generateSuggestions()
	case "c", "enter":
		if m.currentPage == "suggestions" && len(m.suggestions) > 0 {
			return m, m.commitWithSuggestion()
		}
		return m, nil
	default:
		var cmd tea.Cmd
		switch m.currentPage {
		case "changes":
			m.changesTable, cmd = m.changesTable.Update(msg)
		case "suggestions":
			m.suggestionsTable, cmd = m.suggestionsTable.Update(msg)
		case "history":
			m.historyTable, cmd = m.historyTable.Update(msg)
		}
		return m, cmd
	}
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelEdit()
		return m, nil
	case "enter":
		m.saveEdit()
		m.cancelEdit()
		return m, showStatus("✅ Updated")
	case "tab":
		m.saveEdit()
		m.editCol = (m.editCol + 1) % 3
		if m.editCol == 0 {
			m.editCol = 2
		}
		m.setEditValue()
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *model) adjustLayout() {
	tableHeight := m.height - 8
	if tableHeight < 10 {
		tableHeight = 10
	}

	m.changesTable.SetHeight(tableHeight)
	m.suggestionsTable.SetHeight(tableHeight)
	m.historyTable.SetHeight(tableHeight)
}

func (m *model) updateChangesTable() {
	var rows []table.Row

	for _, change := range m.changes {
		statusIcon := getStatusIcon(change.Status)
		rows = append(rows, table.Row{
			statusIcon,
			truncateString(change.File, 38),
			change.SuggestedType,
			change.SuggestedScope,
			truncateString(change.SuggestedMessage, 33),
		})
	}

	m.changesTable.SetRows(rows)
}

func (m *model) updateSuggestionsTable() {
	var rows []table.Row

	for _, suggestion := range m.suggestions {
		breaking := "No"
		if suggestion.Breaking {
			breaking = "⚠️ YES"
		}

		confidence := fmt.Sprintf("%.0f%%", suggestion.Confidence*100)

		rows = append(rows, table.Row{
			suggestion.Type,
			suggestion.Scope,
			truncateString(suggestion.Message, 48),
			confidence,
			breaking,
		})
	}

	m.suggestionsTable.SetRows(rows)
}

func (m *model) startEdit() {
	if m.currentPage == "changes" && len(m.changes) > 0 {
		m.editMode = true
		m.editRow = m.changesTable.Cursor()
		m.editCol = 2
		m.setEditValue()
		m.textInput.Focus()
	}
}

func (m *model) setEditValue() {
	if m.currentPage == "changes" && m.editRow >= 0 && m.editRow < len(m.changes) {
		change := &m.changes[m.editRow]
		switch m.editCol {
		case 2:
			m.textInput.SetValue(change.SuggestedType)
		case 3:
			m.textInput.SetValue(change.SuggestedScope)
		case 4:
			m.textInput.SetValue(change.SuggestedMessage)
		}
	}
}

func (m *model) saveEdit() {
	if m.currentPage == "changes" && m.editRow >= 0 && m.editRow < len(m.changes) {
		value := m.textInput.Value()
		change := &m.changes[m.editRow]

		switch m.editCol {
		case 2:
			change.SuggestedType = value
		case 3:
			change.SuggestedScope = value
		case 4:
			change.SuggestedMessage = value
		}

		m.updateChangesTable()
	}
}

func (m *model) cancelEdit() {
	m.editMode = false
	m.editRow = -1
	m.editCol = -1
	m.textInput.Blur()
	m.textInput.SetValue("")
}

func (m model) loadGitHistory() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "log", "--oneline", "-10", "--pretty=format:%h|%s|%an|%ad", "--date=short")
		cmd.Dir = m.repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load history: %v", err)}
		}

		var rows []table.Row
		lines := strings.Split(string(output), "\n")

		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}

			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				commitType, scope, message := parseCommitMessage(parts[1])

				rows = append(rows, table.Row{
					parts[0],
					commitType,
					scope,
					truncateString(message, 43),
					truncateString(parts[2], 13),
					parts[3],
				})
			}
		}

		m.historyTable.SetRows(rows)
		return statusMsg{message: "📜 Loaded history"}
	}
}

func (m model) commitWithSuggestion() tea.Cmd {
	return func() tea.Msg {
		suggestion := m.suggestions[m.suggestionsTable.Cursor()]

		commitMsg := fmt.Sprintf("%s", suggestion.Type)
		if suggestion.Scope != "" {
			commitMsg = fmt.Sprintf("%s(%s)", suggestion.Type, suggestion.Scope)
		}
		commitMsg = fmt.Sprintf("%s: %s", commitMsg, suggestion.Message)

		if suggestion.Breaking {
			commitMsg = commitMsg + "\n\nBREAKING CHANGE: This commit introduces breaking changes"
		}

		cmd := exec.Command("git", "commit", "-m", commitMsg)
		cmd.Dir = m.repoPath
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Commit failed: %v - %s", err, string(output))}
		}

		return statusMsg{message: fmt.Sprintf("✅ Committed: %s", suggestion.Message)}
	}
}

func (m model) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("🔄 Git Commit Message Generator")

	repoInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("Repository: %s", filepath.Base(m.repoPath)))

	tabs := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderTab("1", "Changes", m.currentPage == "changes"),
		m.renderTab("2", "Suggestions", m.currentPage == "suggestions"),
		m.renderTab("3", "History", m.currentPage == "history"),
	)

	var tableView string
	var footer string

	switch m.currentPage {
	case "changes":
		tableView = m.changesTable.View()
		if m.editMode {
			colNames := []string{"", "", "Type", "Scope", "Message"}
			footer = fmt.Sprintf("Editing %s: %s | tab: next • enter: save • esc: cancel",
				colNames[m.editCol], m.textInput.View())
		} else {
			footer = "1-3: switch page • ↑↓: navigate • e: edit • r: refresh • q: quit"
		}

	case "suggestions":
		tableView = m.suggestionsTable.View()
		footer = "1-3: switch page • ↑↓: navigate • c/enter: commit • g: regenerate • q: quit"

	case "history":
		tableView = m.historyTable.View()
		footer = "1-3: switch page • ↑↓: navigate • q: quit"
	}

	var statusMessage string
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		color := lipgloss.Color("86")
		if strings.Contains(m.statusMsg, "❌") {
			color = lipgloss.Color("196")
		}
		statusStyle := lipgloss.NewStyle().Foreground(color)
		statusMessage = " > " + statusStyle.Render(m.statusMsg)
	}

	fullFooter := footer + statusMessage

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n\n%s", header, repoInfo, tabs, tableView, fullFooter)
}

func (m model) renderTab(key, label string, active bool) string {
	if active {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("240")).
			Padding(0, 2).
			Render(fmt.Sprintf("[%s] %s", key, label))
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2).
		Render(fmt.Sprintf("[%s] %s", key, label))
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

		analyzeChange(&change)
		changes = append(changes, change)
	}

	return changes, nil
}

func analyzeChange(change *GitChange) {
	file := change.File
	status := change.Status

	scope := determineScope(file)
	commitType := determineCommitType(file, status)
	message := generateCommitMessage(file, status, commitType)

	change.SuggestedType = commitType
	change.SuggestedScope = scope
	change.SuggestedMessage = message
}

func determineScope(file string) string {
	if strings.Contains(file, "test") {
		return "test"
	}
	if strings.Contains(file, "doc") || strings.HasSuffix(file, ".md") {
		return "docs"
	}
	if strings.Contains(file, "config") || strings.Contains(file, ".json") || strings.Contains(file, ".yaml") {
		return "config"
	}
	if strings.Contains(file, "api") {
		return "api"
	}
	if strings.Contains(file, "ui") || strings.Contains(file, "component") {
		return "ui"
	}

	parts := strings.Split(file, "/")
	if len(parts) > 1 {
		dir := parts[0]
		if dir == "src" && len(parts) > 2 {
			return parts[1]
		}
		return dir
	}

	return ""
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

func analyzeChangesForCommits(changes []GitChange) []CommitSuggestion {
	var suggestions []CommitSuggestion

	typeGroups := make(map[string][]GitChange)
	for _, change := range changes {
		key := fmt.Sprintf("%s:%s", change.SuggestedType, change.SuggestedScope)
		typeGroups[key] = append(typeGroups[key], change)
	}

	for key, groupChanges := range typeGroups {
		parts := strings.Split(key, ":")
		commitType := parts[0]
		scope := parts[1]

		var message string

		if len(groupChanges) == 1 {
			message = groupChanges[0].SuggestedMessage
		} else {
			switch commitType {
			case "feat":
				message = fmt.Sprintf("add %s functionality", scope)
			case "fix":
				message = fmt.Sprintf("fix %s issues", scope)
			case "docs":
				message = fmt.Sprintf("update %s documentation", scope)
			case "test":
				message = fmt.Sprintf("add %s tests", scope)
			case "chore":
				message = fmt.Sprintf("update %s configuration", scope)
			default:
				message = fmt.Sprintf("update %s", scope)
			}
		}

		confidence := 0.8
		if len(groupChanges) == 1 {
			confidence = 0.9
		}

		suggestion := CommitSuggestion{
			Type:       commitType,
			Scope:      scope,
			Message:    message,
			Breaking:   false,
			Confidence: confidence,
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions
}

func parseCommitMessage(message string) (string, string, string) {
	re := regexp.MustCompile(`^([a-zA-Z]+)(?:\(([^)]+)\))?\s*:\s*(.+)$`)
	matches := re.FindStringSubmatch(message)

	if len(matches) >= 4 {
		return matches[1], matches[2], matches[3]
	}

	commitType := "chore"
	if strings.Contains(strings.ToLower(message), "fix") {
		commitType = "fix"
	} else if strings.Contains(strings.ToLower(message), "add") || strings.Contains(strings.ToLower(message), "feat") {
		commitType = "feat"
	} else if strings.Contains(strings.ToLower(message), "doc") {
		commitType = "docs"
	}

	return commitType, "", message
}

func getStatusIcon(status string) string {
	switch status {
	case "A":
		return "➕ A"
	case "M":
		return "📝 M"
	case "D":
		return "🗑️ D"
	case "R":
		return "📛 R"
	case "C":
		return "📋 C"
	case "U":
		return "⚠️ U"
	case "??":
		return "❓ ??"
	default:
		return status
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
