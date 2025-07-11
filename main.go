package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Commit struct {
	Hash      string    `json:"hash"`
	Type      string    `json:"type"`
	Scope     string    `json:"scope"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Date      time.Time `json:"date"`
	Breaking  bool      `json:"breaking"`
	Validated bool      `json:"validated"`
	Category  string    `json:"category"`
}

type ChangelogConfig struct {
	RepoPath        string            `json:"repo_path"`
	FromVersion     string            `json:"from_version"`
	ToVersion       string            `json:"to_version"`
	OutputFormat    string            `json:"output_format"`
	IncludeBreaking bool              `json:"include_breaking"`
	GroupByType     bool              `json:"group_by_type"`
	CommitTypes     map[string]string `json:"commit_types"`
	LastGenerated   time.Time         `json:"last_generated"`
}

type model struct {
	currentPage string // "commits", "config", "changelog"
	commits     []Commit
	config      ChangelogConfig
	configFile  string

	// Table management
	commitsTable   table.Model
	configTable    table.Model
	changelogTable table.Model

	// Edit mode
	editMode  bool
	editRow   int
	editCol   int
	textInput textinput.Model

	// Layout
	width        int
	height       int
	statusMsg    string
	statusExpiry time.Time
	lastUpdate   time.Time
}

type statusMsg struct {
	message string
}

type tickMsg time.Time

type gitCommitsMsg []Commit

func showStatus(msg string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{message: msg}
	}
}

func loadConfig(configFile string) ChangelogConfig {
	var config ChangelogConfig
	data, err := os.ReadFile(configFile)
	if err != nil {
		// Default configuration
		config = ChangelogConfig{
			RepoPath:        "./",
			FromVersion:     "HEAD~10",
			ToVersion:       "HEAD",
			OutputFormat:    "markdown",
			IncludeBreaking: true,
			GroupByType:     true,
			CommitTypes: map[string]string{
				"feat":     "✨ Features",
				"fix":      "🐛 Bug Fixes",
				"docs":     "📚 Documentation",
				"style":    "💄 Styles",
				"refactor": "♻️ Refactoring",
				"test":     "🧪 Tests",
				"chore":    "🔧 Chore",
				"perf":     "⚡ Performance",
				"ci":       "👷 CI/CD",
				"build":    "📦 Build",
				"revert":   "⏪ Reverts",
			},
		}
		saveConfig(config, configFile)
		return config
	}

	json.Unmarshal(data, &config)
	return config
}

func saveConfig(config ChangelogConfig, configFile string) {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configFile, data, 0644)
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configFile := filepath.Join(homeDir, ".local/bin/changelog-config.json")
	config := loadConfig(configFile)

	m := model{
		currentPage: "commits",
		config:      config,
		configFile:  configFile,
		width:       100,
		height:      24,
		editMode:    false,
		editRow:     -1,
		editCol:     -1,
		lastUpdate:  time.Now(),
	}

	// Initialize text input
	m.textInput = textinput.New()
	m.textInput.CharLimit = 200

	// Initialize tables
	m.initializeTables()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func (m *model) initializeTables() {
	// Commits table
	commitColumns := []table.Column{
		{Title: "Hash", Width: 8},
		{Title: "Type", Width: 10},
		{Title: "Scope", Width: 12},
		{Title: "Subject", Width: 40},
		{Title: "Author", Width: 15},
		{Title: "Date", Width: 12},
		{Title: "Status", Width: 10},
	}

	m.commitsTable = table.New(
		table.WithColumns(commitColumns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Config table
	configColumns := []table.Column{
		{Title: "Setting", Width: 20},
		{Title: "Value", Width: 50},
		{Title: "Description", Width: 30},
	}

	m.configTable = table.New(
		table.WithColumns(configColumns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Changelog table
	changelogColumns := []table.Column{
		{Title: "Type", Width: 15},
		{Title: "Count", Width: 8},
		{Title: "Preview", Width: 60},
	}

	m.changelogTable = table.New(
		table.WithColumns(changelogColumns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Apply consistent styling
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

	m.commitsTable.SetStyles(s)
	m.configTable.SetStyles(s)
	m.changelogTable.SetStyles(s)
}

func (m *model) updateCommitsTable() {
	var rows []table.Row

	for _, commit := range m.commits {
		status := "✅ Valid"
		if !commit.Validated {
			status = "❌ Invalid"
		}
		if commit.Breaking {
			status = "⚠️ Breaking"
		}

		rows = append(rows, table.Row{
			commit.Hash[:8],
			commit.Type,
			commit.Scope,
			truncateString(commit.Subject, 38),
			truncateString(commit.Author, 13),
			commit.Date.Format("01-02 15:04"),
			status,
		})
	}

	m.commitsTable.SetRows(rows)
}

func (m *model) updateConfigTable() {
	var rows []table.Row

	rows = append(rows, table.Row{
		"Repository Path",
		m.config.RepoPath,
		"Path to git repository",
	})
	rows = append(rows, table.Row{
		"From Version",
		m.config.FromVersion,
		"Starting commit/tag",
	})
	rows = append(rows, table.Row{
		"To Version",
		m.config.ToVersion,
		"Ending commit/tag",
	})
	rows = append(rows, table.Row{
		"Output Format",
		m.config.OutputFormat,
		"markdown, json, text",
	})
	rows = append(rows, table.Row{
		"Include Breaking",
		fmt.Sprintf("%t", m.config.IncludeBreaking),
		"Highlight breaking changes",
	})
	rows = append(rows, table.Row{
		"Group By Type",
		fmt.Sprintf("%t", m.config.GroupByType),
		"Group commits by type",
	})

	m.configTable.SetRows(rows)
}

func (m *model) updateChangelogTable() {
	var rows []table.Row

	// Group commits by type
	typeGroups := make(map[string][]Commit)
	for _, commit := range m.commits {
		if commit.Validated {
			typeGroups[commit.Type] = append(typeGroups[commit.Type], commit)
		}
	}

	// Sort types by importance
	typeOrder := []string{"feat", "fix", "perf", "refactor", "docs", "style", "test", "chore", "ci", "build", "revert"}

	for _, commitType := range typeOrder {
		if commits, exists := typeGroups[commitType]; exists {
			typeLabel := m.config.CommitTypes[commitType]
			if typeLabel == "" {
				typeLabel = commitType
			}

			preview := ""
			if len(commits) > 0 {
				preview = truncateString(commits[0].Subject, 55)
				if len(commits) > 1 {
					preview += fmt.Sprintf(" (+%d more)", len(commits)-1)
				}
			}

			rows = append(rows, table.Row{
				typeLabel,
				fmt.Sprintf("%d", len(commits)),
				preview,
			})
		}
	}

	m.changelogTable.SetRows(rows)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("Changelog Generator"),
		tickCmd(),
		m.loadGitHistory(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) loadGitHistory() tea.Cmd {
	return func() tea.Msg {
		commits, err := getGitCommits(m.config.RepoPath, m.config.FromVersion, m.config.ToVersion)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load git history: %v", err)}
		}
		return gitCommitsMsg(commits)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		return m, tickCmd()

	case gitCommitsMsg:
		m.commits = []Commit(msg)
		m.updateCommitsTable()
		m.updateChangelogTable()
		return m, showStatus(fmt.Sprintf("✅ Loaded %d commits", len(m.commits)))

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

	// Update the current table
	switch m.currentPage {
	case "commits":
		m.commitsTable, cmd = m.commitsTable.Update(msg)
	case "config":
		m.configTable, cmd = m.configTable.Update(msg)
	case "changelog":
		m.changelogTable, cmd = m.changelogTable.Update(msg)
	}

	return m, cmd
}

func (m *model) adjustLayout() {
	tableHeight := m.height - 8
	if tableHeight < 10 {
		tableHeight = 10
	}

	availableWidth := m.width - 10

	// Adjust commits table
	if m.currentPage == "commits" {
		hashWidth := 8
		typeWidth := 10
		scopeWidth := 12
		authorWidth := 15
		dateWidth := 12
		statusWidth := 10
		subjectWidth := availableWidth - hashWidth - typeWidth - scopeWidth - authorWidth - dateWidth - statusWidth

		if subjectWidth < 20 {
			subjectWidth = 20
		}

		commitColumns := []table.Column{
			{Title: "Hash", Width: hashWidth},
			{Title: "Type", Width: typeWidth},
			{Title: "Scope", Width: scopeWidth},
			{Title: "Subject", Width: subjectWidth},
			{Title: "Author", Width: authorWidth},
			{Title: "Date", Width: dateWidth},
			{Title: "Status", Width: statusWidth},
		}

		m.commitsTable.SetColumns(commitColumns)
		m.commitsTable.SetHeight(tableHeight)
	}

	// Adjust config table
	if m.currentPage == "config" {
		settingWidth := 20
		descWidth := 30
		valueWidth := availableWidth - settingWidth - descWidth

		if valueWidth < 20 {
			valueWidth = 20
		}

		configColumns := []table.Column{
			{Title: "Setting", Width: settingWidth},
			{Title: "Value", Width: valueWidth},
			{Title: "Description", Width: descWidth},
		}

		m.configTable.SetColumns(configColumns)
		m.configTable.SetHeight(tableHeight)
	}

	// Adjust changelog table
	if m.currentPage == "changelog" {
		typeWidth := 15
		countWidth := 8
		previewWidth := availableWidth - typeWidth - countWidth

		if previewWidth < 30 {
			previewWidth = 30
		}

		changelogColumns := []table.Column{
			{Title: "Type", Width: typeWidth},
			{Title: "Count", Width: countWidth},
			{Title: "Preview", Width: previewWidth},
		}

		m.changelogTable.SetColumns(changelogColumns)
		m.changelogTable.SetHeight(tableHeight)
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
		if m.currentPage == "commits" {
			m.editCol = (m.editCol + 1) % 4 // Skip non-editable columns
			if m.editCol == 0 {             // Skip hash
				m.editCol = 1
			}
		} else if m.currentPage == "config" {
			m.editCol = 1 // Only value column is editable
		}
		m.setEditValue()
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	// Page navigation
	case "1":
		m.currentPage = "commits"
		return m, nil
	case "2":
		m.currentPage = "config"
		m.updateConfigTable()
		return m, nil
	case "3":
		m.currentPage = "changelog"
		m.updateChangelogTable()
		return m, nil

	// Actions
	case "e":
		m.startEdit()
		return m, nil
	case "r":
		return m, tea.Batch(
			m.loadGitHistory(),
			showStatus("🔄 Refreshing git history..."),
		)
	case "g":
		return m, m.generateChangelog()
	case "s":
		saveConfig(m.config, m.configFile)
		return m, showStatus("💾 Configuration saved")
	case "v":
		if m.currentPage == "commits" && len(m.commits) > 0 {
			return m, m.validateCommit()
		}
		return m, nil
	case "enter":
		if m.currentPage == "commits" && len(m.commits) > 0 {
			return m, m.validateCommit()
		}
		return m, nil
	}

	return m, nil
}

func (m *model) startEdit() {
	switch m.currentPage {
	case "commits":
		if len(m.commits) == 0 {
			return
		}
		m.editMode = true
		m.editRow = m.commitsTable.Cursor()
		m.editCol = 1 // Start with type
		m.setEditValue()

	case "config":
		m.editMode = true
		m.editRow = m.configTable.Cursor()
		m.editCol = 1 // Value column
		m.setEditValue()
	}

	m.textInput.Focus()
}

func (m *model) setEditValue() {
	switch m.currentPage {
	case "commits":
		if m.editRow >= 0 && m.editRow < len(m.commits) {
			commit := &m.commits[m.editRow]
			switch m.editCol {
			case 1:
				m.textInput.SetValue(commit.Type)
			case 2:
				m.textInput.SetValue(commit.Scope)
			case 3:
				m.textInput.SetValue(commit.Subject)
			}
		}

	case "config":
		switch m.editRow {
		case 0:
			m.textInput.SetValue(m.config.RepoPath)
		case 1:
			m.textInput.SetValue(m.config.FromVersion)
		case 2:
			m.textInput.SetValue(m.config.ToVersion)
		case 3:
			m.textInput.SetValue(m.config.OutputFormat)
		case 4:
			m.textInput.SetValue(fmt.Sprintf("%t", m.config.IncludeBreaking))
		case 5:
			m.textInput.SetValue(fmt.Sprintf("%t", m.config.GroupByType))
		}
	}
}

func (m *model) saveEdit() {
	value := m.textInput.Value()

	switch m.currentPage {
	case "commits":
		if m.editRow >= 0 && m.editRow < len(m.commits) {
			commit := &m.commits[m.editRow]
			switch m.editCol {
			case 1:
				commit.Type = value
			case 2:
				commit.Scope = value
			case 3:
				commit.Subject = value
			}
			// Re-validate after edit
			commit.Validated = validateCommitFormat(*commit)
			m.updateCommitsTable()
			m.updateChangelogTable()
		}

	case "config":
		switch m.editRow {
		case 0:
			m.config.RepoPath = value
		case 1:
			m.config.FromVersion = value
		case 2:
			m.config.ToVersion = value
		case 3:
			m.config.OutputFormat = value
		case 4:
			m.config.IncludeBreaking = value == "true"
		case 5:
			m.config.GroupByType = value == "true"
		}
		m.updateConfigTable()
	}
}

func (m *model) cancelEdit() {
	m.editMode = false
	m.editRow = -1
	m.editCol = -1
	m.textInput.Blur()
	m.textInput.SetValue("")
}

func (m model) validateCommit() tea.Cmd {
	return func() tea.Msg {
		if m.editRow >= 0 && m.editRow < len(m.commits) {
			commit := &m.commits[m.editRow]
			commit.Validated = validateCommitFormat(*commit)
			return statusMsg{message: fmt.Sprintf("✅ Commit %s validated", commit.Hash[:8])}
		}
		return statusMsg{message: "❌ No commit selected"}
	}
}

func (m model) generateChangelog() tea.Cmd {
	return func() tea.Msg {
		changelog, err := generateChangelogContent(m.commits, m.config)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to generate changelog: %v", err)}
		}

		filename := fmt.Sprintf("CHANGELOG_%s.%s",
			time.Now().Format("20060102_150405"),
			getFileExtension(m.config.OutputFormat))

		err = os.WriteFile(filename, []byte(changelog), 0644)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to write changelog: %v", err)}
		}

		return statusMsg{message: fmt.Sprintf("✅ Changelog generated: %s", filename)}
	}
}

func (m model) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("🔄 Changelog Generator")

	// Page tabs
	tabs := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderTab("1", "Commits", m.currentPage == "commits"),
		m.renderTab("2", "Config", m.currentPage == "config"),
		m.renderTab("3", "Preview", m.currentPage == "changelog"),
	)

	// Current table view
	var tableView string
	var footer string

	switch m.currentPage {
	case "commits":
		tableView = m.commitsTable.View()
		if m.editMode {
			colNames := []string{"", "Type", "Scope", "Subject"}
			footer = fmt.Sprintf("Editing %s: %s | tab: next • enter: save • esc: cancel",
				colNames[m.editCol], m.textInput.View())
		} else {
			footer = "1-3: switch page • ↑↓: navigate • e: edit • v/enter: validate • r: refresh • g: generate • q: quit"
		}

	case "config":
		tableView = m.configTable.View()
		if m.editMode {
			footer = fmt.Sprintf("Editing: %s | enter: save • esc: cancel", m.textInput.View())
		} else {
			footer = "1-3: switch page • ↑↓: navigate • e: edit • s: save config • r: refresh • q: quit"
		}

	case "changelog":
		tableView = m.changelogTable.View()
		footer = "1-3: switch page • g: generate changelog • r: refresh • q: quit"
	}

	// Status message
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

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", header, tabs, tableView, fullFooter)
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

// Git operations
func getGitCommits(repoPath, fromVersion, toVersion string) ([]Commit, error) {
	cmd := exec.Command("git", "log",
		fmt.Sprintf("%s..%s", fromVersion, toVersion),
		"--pretty=format:%H|%s|%an|%ae|%ad|%b",
		"--date=iso")
	cmd.Dir = repoPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[4])

		commit := Commit{
			Hash:   parts[0],
			Author: parts[2],
			Email:  parts[3],
			Date:   date,
		}

		if len(parts) > 5 {
			commit.Body = parts[5]
		}

		// Parse conventional commit format
		parseConventionalCommit(&commit, parts[1])
		commit.Validated = validateCommitFormat(commit)

		commits = append(commits, commit)
	}

	// Sort by date (newest first)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Date.After(commits[j].Date)
	})

	return commits, nil
}

func parseConventionalCommit(commit *Commit, subject string) {
	// Pattern: type(scope): subject
	// or: type: subject
	re := regexp.MustCompile(`^([a-zA-Z]+)(?:\(([^)]+)\))?\s*:\s*(.+)$`)
	matches := re.FindStringSubmatch(subject)

	if len(matches) >= 4 {
		commit.Type = matches[1]
		commit.Scope = matches[2]
		commit.Subject = matches[3]

		// Check for breaking changes
		if strings.Contains(strings.ToLower(commit.Subject), "breaking") ||
			strings.Contains(strings.ToLower(commit.Body), "breaking change") {
			commit.Breaking = true
		}
	} else {
		// Not conventional format - try to guess type
		commit.Type = guessCommitType(subject)
		commit.Subject = subject
	}
}

func guessCommitType(subject string) string {
	subject = strings.ToLower(subject)

	if strings.Contains(subject, "fix") || strings.Contains(subject, "bug") {
		return "fix"
	}
	if strings.Contains(subject, "feat") || strings.Contains(subject, "add") {
		return "feat"
	}
	if strings.Contains(subject, "doc") {
		return "docs"
	}
	if strings.Contains(subject, "test") {
		return "test"
	}
	if strings.Contains(subject, "refactor") {
		return "refactor"
	}
	if strings.Contains(subject, "style") || strings.Contains(subject, "format") {
		return "style"
	}
	if strings.Contains(subject, "perf") || strings.Contains(subject, "performance") {
		return "perf"
	}

	return "chore"
}

func validateCommitFormat(commit Commit) bool {
	// Valid conventional commit types
	validTypes := []string{"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "ci", "build", "revert"}

	for _, validType := range validTypes {
		if commit.Type == validType {
			return true
		}
	}

	return false
}

func generateChangelogContent(commits []Commit, config ChangelogConfig) (string, error) {
	var content strings.Builder

	switch config.OutputFormat {
	case "markdown":
		content.WriteString("# Changelog\n\n")
		content.WriteString(fmt.Sprintf("Generated on %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

		if config.GroupByType {
			// Group by type
			typeGroups := make(map[string][]Commit)
			for _, commit := range commits {
				if commit.Validated {
					typeGroups[commit.Type] = append(typeGroups[commit.Type], commit)
				}
			}

			typeOrder := []string{"feat", "fix", "perf", "refactor", "docs", "style", "test", "chore", "ci", "build", "revert"}

			for _, commitType := range typeOrder {
				if commitList, exists := typeGroups[commitType]; exists {
					typeLabel := config.CommitTypes[commitType]
					if typeLabel == "" {
						typeLabel = commitType
					}

					content.WriteString(fmt.Sprintf("## %s (%d)\n\n", typeLabel, len(commitList)))

					for _, commit := range commitList {
						scope := ""
						if commit.Scope != "" {
							scope = fmt.Sprintf("**%s**: ", commit.Scope)
						}

						breaking := ""
						if commit.Breaking {
							breaking = " ⚠️ **BREAKING CHANGE**"
						}

						content.WriteString(fmt.Sprintf("- %s%s%s\n", scope, commit.Subject, breaking))
					}
					content.WriteString("\n")
				}
			}
		} else {
			// Simple chronological list
			content.WriteString("## Commits\n\n")
			for _, commit := range commits {
				if commit.Validated {
					scope := ""
					if commit.Scope != "" {
						scope = fmt.Sprintf("**%s**: ", commit.Scope)
					}

					breaking := ""
					if commit.Breaking {
						breaking = " ⚠️ **BREAKING CHANGE**"
					}

					content.WriteString(fmt.Sprintf("- **%s**: %s%s%s (%s)\n",
						commit.Type, scope, commit.Subject, breaking, commit.Hash[:8]))
				}
			}
		}

	case "json":
		data, err := json.MarshalIndent(commits, "", "  ")
		if err != nil {
			return "", err
		}
		content.Write(data)

	case "text":
		content.WriteString("CHANGELOG\n")
		content.WriteString("=========\n\n")
		content.WriteString(fmt.Sprintf("Generated on %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

		for _, commit := range commits {
			if commit.Validated {
				scope := ""
				if commit.Scope != "" {
					scope = fmt.Sprintf("(%s) ", commit.Scope)
				}

				breaking := ""
				if commit.Breaking {
					breaking = " [BREAKING CHANGE]"
				}

				content.WriteString(fmt.Sprintf("%s - %s: %s%s%s\n",
					commit.Hash[:8], commit.Type, scope, commit.Subject, breaking))
				content.WriteString(fmt.Sprintf("  Author: %s <%s>\n", commit.Author, commit.Email))
				content.WriteString(fmt.Sprintf("  Date: %s\n\n", commit.Date.Format("2006-01-02 15:04:05")))
			}
		}

	default:
		return "", fmt.Errorf("unsupported output format: %s", config.OutputFormat)
	}

	return content.String(), nil
}

func getFileExtension(format string) string {
	switch format {
	case "markdown":
		return "md"
	case "json":
		return "json"
	case "text":
		return "txt"
	default:
		return "txt"
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
