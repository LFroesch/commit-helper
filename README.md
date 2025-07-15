# 🚀 Git Commit Helper

A simple, user-friendly terminal UI for creating git commits with smart suggestions.

## ✨ Features

- **Simple Interface**: Clean, intuitive design with three main modes
- **Smart Suggestions**: Analyzes git diffs, file content, and patterns to suggest contextual commit messages
- **Function Detection**: Recognizes when you add/modify functions and suggests specific messages
- **Conventional Commits**: Automatic formatting using conventional commit standards (type(scope): description)
- **Commit Hook Management**: Install/remove git hooks to enforce commit message validation
- **Quick Actions**: One-key shortcuts for common git operations
- **Custom Messages**: Easy custom commit message input with validation
- **Error Prevention**: Validates workflows and shows helpful error messages

## 🎯 How to Use

### Three Simple Modes:

1. **📁 Files Mode** (`1` key)
   - View all your changed files with status icons
   - See what's been modified, added, or deleted
   - Quick overview of your working directory

2. **💡 Suggestions Mode** (`2` key)
   - **Combined suggestion** as the first option - intelligently merges all your changes
   - Individual file-specific suggestions based on actual code changes
   - Analyzes git diffs, detects function additions/modifications
   - Groups related changes for comprehensive commits
   - Follows conventional commit standards (feat, fix, docs, etc.)
   - Press `Enter` to commit with a suggestion
   - Press `e` to **edit any suggestion** before committing

3. **✏️ Custom Mode** (`3` key)
   - Write your own commit message
   - Full control over the commit text
   - Press `Enter` to commit

### Quick Actions:
- `a` - Git add all files
- `p` - Git push to remote
- `s` - Git status check
- `r` - Refresh/reload changes
- `q` - Quit
test

### Navigation:
- `1`, `2`, `3` - Switch between modes
- `↑`/`↓` or `j`/`k` - Navigate lists
- `Enter` - Commit selected suggestion or custom message
- `e` - Edit selected suggestion (in suggestions mode)
- `Esc` - Cancel custom input or go back

## 🎨 Visual Design

The interface maintains a clean, modern look with:
- Intuitive icons and colors
- Clear status messages
- Helpful keyboard shortcuts
- Real-time feedback

## 🚦 Workflow

1. Make your code changes
2. Run `./git-helper`
3. Review files in mode 1
4. Check suggestions in mode 2, or write custom in mode 3
5. Commit with `Enter`
6. Optionally push with `p`

Simple as that! No complex navigation or confusing options.

## 📦 Installation

### Global Installation

1. Build the application:
   ```bash
   go build -o git-helper .
   ```

2. Install to your local bin directory:
   ```bash
   cp git-helper ~/.local/bin/
   ```

3. Make sure `~/.local/bin` is in your PATH (add to your shell config if needed):
   ```bash
   export PATH="$HOME/.local/bin:$PATH"
   ```

4. Now you can use `git-helper` from any git repository!

### Usage
Simply run from any git repository:
```bash
git-helper
```