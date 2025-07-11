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
	state       string // "files", "suggestions", "custom", "edit"
	changes     []GitChange
	suggestions []CommitSuggestion

	filesList       list.Model
	suggestionsList list.Model
	customInput     textinput.Model
	editInput       textinput.Model

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

	// Initialize edit input
	m.editInput = textinput.New()
	m.editInput.Placeholder = "Edit commit message..."
	m.editInput.CharLimit = 200

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

		case "e":
			if m.state == "suggestions" && len(m.suggestions) > 0 {
				selected := m.suggestionsList.SelectedItem()
				if suggestion, ok := selected.(CommitSuggestion); ok {
					m.state = "edit"
					m.editInput.SetValue(suggestion.Message)
					m.editInput.Focus()
				}
			}
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
			case "edit":
				if m.editInput.Value() != "" {
					return m, m.commitWithMessage(m.editInput.Value())
				}
			}
			return m, nil

		case "esc":
			if m.state == "custom" {
				m.customInput.Blur()
				m.customInput.SetValue("")
				m.state = "files"
			} else if m.state == "edit" {
				m.editInput.Blur()
				m.editInput.SetValue("")
				m.state = "suggestions"
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
	case "edit":
		m.editInput, cmd = m.editInput.Update(msg)
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

	case "edit":
		inputLabel := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Render("Edit Commit Message:")
		content = fmt.Sprintf("%s\n\n%s", inputLabel, m.editInput.View())
	}

	// Footer with help and status
	var footer string
	switch m.state {
	case "files":
		footer = "1-3: switch mode • ↑↓: navigate • r: refresh • a: git add • s: status • q: quit"
	case "suggestions":
		footer = "1-3: switch mode • ↑↓: navigate • enter: commit • e: edit • a: git add • p: push • q: quit"
	case "custom":
		footer = "1-3: switch mode • enter: commit • esc: cancel • a: git add • p: push • q: quit"
	case "edit":
		footer = "enter: commit • esc: back to suggestions • a: git add • p: push • q: quit"
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

	// Generate individual suggestions
	individualSuggestions := []CommitSuggestion{}
	for _, change := range changes {
		diffInfo := getFileDiff(change.File)
		analysis := analyzeFileChange(change, diffInfo)

		suggestion := CommitSuggestion{
			Type:    analysis.Type,
			Message: analysis.Message,
		}
		individualSuggestions = append(individualSuggestions, suggestion)
	}

	// Create combined suggestion as first option
	combinedSuggestion := generateCombinedSuggestion(individualSuggestions, nil)
	if combinedSuggestion.Message != "" {
		suggestions = append(suggestions, combinedSuggestion)
	}

	// Only add individual suggestions if there are 3 or fewer files
	if len(individualSuggestions) <= 3 {
		suggestions = append(suggestions, individualSuggestions...)
	}

	// Limit to max 5 suggestions total
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

func generateCombinedSuggestion(individual []CommitSuggestion, grouped []CommitSuggestion) CommitSuggestion {
	if len(individual) == 0 {
		return CommitSuggestion{}
	}

	// Count types to determine the main focus
	typeCounts := make(map[string]int)
	for _, suggestion := range individual {
		typeCounts[suggestion.Type]++
	}

	// Find the most common type
	var mainType string
	maxCount := 0
	for commitType, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			mainType = commitType
		}
	}

	// Generate a simple combined message
	var message string
	totalFiles := len(individual)

	if len(typeCounts) == 1 {
		// All changes are the same type - keep it simple
		switch mainType {
		case "feat":
			message = "add features"
		case "fix":
			message = "fix issues"
		case "docs":
			message = "update docs"
		case "test":
			message = "update tests"
		case "chore":
			message = "update config"
		case "refactor":
			message = "refactor code"
		default:
			message = "update files"
		}
	} else {
		// Mixed types - just use "update" for simplicity
		message = "update multiple files"
	}

	// Only add file count if more than 1 file
	if totalFiles > 1 {
		message = fmt.Sprintf("%s (%d files)", message, totalFiles)
	}

	return CommitSuggestion{
		Type:    mainType,
		Message: message,
	}
}

type FileAnalysis struct {
	Type    string
	Message string
	Scope   string
}

type DiffInfo struct {
	LinesAdded   int
	LinesRemoved int
	Functions    []string
	Imports      []string
	HasTests     bool
	HasDocs      bool
}

func getFileDiff(filePath string) DiffInfo {
	cmd := exec.Command("git", "diff", "--cached", filePath)
	output, err := cmd.Output()
	if err != nil {
		// Try unstaged diff if no staged changes
		cmd = exec.Command("git", "diff", filePath)
		output, _ = cmd.Output()
	}

	return parseDiffOutput(string(output))
}

func parseDiffOutput(diff string) DiffInfo {
	info := DiffInfo{}
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			info.LinesAdded++

			// Detect function definitions (basic patterns for common languages)
			if strings.Contains(line, "func ") ||
				strings.Contains(line, "function ") ||
				strings.Contains(line, "def ") ||
				strings.Contains(line, "class ") {
				// Extract function name
				funcName := extractFunctionName(line)
				if funcName != "" {
					info.Functions = append(info.Functions, funcName)
				}
			}

			// Detect imports
			if strings.Contains(line, "import ") ||
				strings.Contains(line, "#include") ||
				strings.Contains(line, "from ") {
				info.Imports = append(info.Imports, strings.TrimSpace(line))
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			info.LinesRemoved++
		}
	}

	return info
}

func extractFunctionName(line string) string {
	// Simple function name extraction for Go
	if idx := strings.Index(line, "func "); idx != -1 {
		parts := strings.Fields(line[idx:])
		if len(parts) > 1 {
			name := parts[1]
			if parenIdx := strings.Index(name, "("); parenIdx != -1 {
				name = name[:parenIdx]
			}
			return name
		}
	}

	// Check for other languages
	if idx := strings.Index(line, "function "); idx != -1 {
		parts := strings.Fields(line[idx:])
		if len(parts) > 1 {
			return parts[1]
		}
	}

	if idx := strings.Index(line, "def "); idx != -1 {
		parts := strings.Fields(line[idx:])
		if len(parts) > 1 {
			return parts[1]
		}
	}

	return ""
}

func analyzeFileChange(change GitChange, diff DiffInfo) FileAnalysis {
	file := change.File
	status := change.Status

	// Determine scope and type based on file path and content
	scope := determineScope(file)
	commitType := determineAdvancedCommitType(file, status, diff)
	message := generateSmartCommitMessage(file, status, diff, commitType)

	return FileAnalysis{
		Type:    commitType,
		Message: message,
		Scope:   scope,
	}
}

func determineAdvancedCommitType(file, status string, diff DiffInfo) string {
	fileName := filepath.Base(file)

	// New files
	if strings.Contains(status, "A") {
		if strings.Contains(file, "test") || strings.HasSuffix(file, "_test.go") {
			return "test"
		}
		if strings.HasSuffix(file, ".md") || strings.Contains(file, "README") {
			return "docs"
		}
		if len(diff.Functions) > 0 {
			return "feat"
		}
		return "feat"
	}

	// Deleted files
	if strings.Contains(status, "D") {
		return "chore"
	}

	// Modified files - analyze the changes
	if strings.Contains(status, "M") {
		// Documentation changes
		if strings.HasSuffix(file, ".md") || strings.Contains(file, "README") || strings.Contains(file, "doc") {
			return "docs"
		}

		// Test files
		if strings.Contains(file, "test") || strings.HasSuffix(file, "_test.go") || strings.HasSuffix(file, ".test.js") {
			return "test"
		}

		// Configuration files
		if strings.Contains(file, "config") || strings.HasSuffix(file, ".json") ||
			strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") ||
			strings.HasSuffix(file, ".toml") || fileName == "Dockerfile" ||
			fileName == "Makefile" || strings.HasSuffix(file, ".env") {
			return "chore"
		}

		// Package management
		if fileName == "package.json" || fileName == "go.mod" || fileName == "requirements.txt" ||
			fileName == "Cargo.toml" || fileName == "pom.xml" {
			if len(diff.Imports) > 0 {
				return "feat" // Adding new dependencies
			}
			return "chore"
		}

		// Bug fixes - look for keywords in diff
		if containsBugFixKeywords(diff) {
			return "fix"
		}

		// New functionality
		if len(diff.Functions) > 0 || diff.LinesAdded > diff.LinesRemoved*2 {
			return "feat"
		}

		// Performance or refactoring
		if diff.LinesAdded > 0 && diff.LinesRemoved > 0 &&
			abs(diff.LinesAdded-diff.LinesRemoved) < 10 {
			return "refactor"
		}

		// Small changes/fixes
		if diff.LinesAdded+diff.LinesRemoved < 10 {
			return "fix"
		}

		return "feat"
	}

	return "chore"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func containsBugFixKeywords(diff DiffInfo) bool {
	// This would need to analyze the actual diff content for keywords
	// For now, we'll use a simple heuristic
	return diff.LinesRemoved > 0 && diff.LinesAdded < diff.LinesRemoved
}

func generateSmartCommitMessage(file, status string, diff DiffInfo, commitType string) string {
	fileName := filepath.Base(file)
	fileExt := filepath.Ext(file)
	baseName := strings.TrimSuffix(fileName, fileExt)

	// Simple, clean messages
	switch status {
	case "A":
		if len(diff.Functions) > 0 && len(diff.Functions) == 1 {
			return fmt.Sprintf("add %s function", diff.Functions[0])
		}
		return fmt.Sprintf("add %s", fileName)

	case "D":
		return fmt.Sprintf("remove %s", fileName)

	case "M":
		// Short, focused messages
		if commitType == "docs" {
			return fmt.Sprintf("update %s", baseName)
		}

		if commitType == "test" {
			return fmt.Sprintf("update %s tests", baseName)
		}

		if len(diff.Functions) > 0 && len(diff.Functions) == 1 {
			return fmt.Sprintf("update %s function", diff.Functions[0])
		}

		return fmt.Sprintf("update %s", baseName)

	default:
		return fmt.Sprintf("modify %s", fileName)
	}
}

func determineScope(file string) string {
	// Enhanced scope detection
	parts := strings.Split(file, "/")

	// Check for common project structures
	if len(parts) > 1 {
		firstDir := parts[0]

		// Common directory patterns
		switch firstDir {
		case "src", "lib":
			if len(parts) > 2 {
				return parts[1]
			}
			return "core"
		case "tests", "test":
			return "test"
		case "docs", "documentation":
			return "docs"
		case "config", "configs":
			return "config"
		case "api":
			return "api"
		case "ui", "frontend", "client":
			return "ui"
		case "backend", "server":
			return "api"
		case "scripts", "tools":
			return "tools"
		default:
			return firstDir
		}
	}

	// File-based scope detection
	fileName := filepath.Base(file)
	if strings.Contains(fileName, "test") {
		return "test"
	}
	if strings.HasSuffix(fileName, ".md") {
		return "docs"
	}
	if strings.Contains(fileName, "config") {
		return "config"
	}

	return ""
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
