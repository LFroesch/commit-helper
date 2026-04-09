# Gitty

Full-featured TUI git client with smart commit suggestions. Stage files, commit with AI-style suggestions, manage branches, rebase, stash, and monitor CI/CD — all from the terminal. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Quick Install

Supported platforms: Linux and macOS. On Windows, use WSL.

Recommended (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/gitty/main/install.sh | bash
```

Or download a binary from [GitHub Releases](https://github.com/LFroesch/gitty/releases).

Or install with Go:

```bash
go install github.com/LFroesch/gitty@latest
```

Or build from source:

```bash
make install
```

Command:

```bash
cd your-repo
gitty
```

## Tabs

### 1. Workspace

File staging with live diff preview.

| Key | Action |
|-----|--------|
| `space` | Stage/unstage file |
| `a` | Stage all |
| `R` | Reset (unstage all) |
| `v` | Toggle diff preview |
| `d` | View full diff |

**Conflict mode** (auto-activates): `o` ours, `t` theirs, `b` both, `c` continue merge.

### 2. Commit

Smart suggestions + custom input.

| Key | Action |
|-----|--------|
| `1-9` | Commit with numbered suggestion |
| `enter/c` | Type custom message |
| `space` | Commit with selected suggestion |

Suggestions are generated from diff analysis — detects keywords, function names, variables, comments, and classifies changes as feat/fix/refactor/etc.

### 3. Branches

Create, switch, delete, merge, compare.

| Key | Action |
|-----|--------|
| `enter` | Switch to branch |
| `n` | Create new branch |
| `d` | Delete branch |
| `m` | Merge into current |
| `c` | Compare with main |
| `f` | Fetch remote |
| `p` | Prune stale remotes |

### 4. Tools

Advanced operations submenu.

| Tool | What it does |
|------|-------------|
| Undo/Revert | Soft/mixed/hard reset, reflog |
| Interactive Rebase | Squash, reword, drop, fixup |
| History | Last 20 commits, hash copy |
| Remote | Push, pull, fetch |

Also: stash (list/push/pop/apply/drop), tags (list/create/delete/push), cherry-pick, revert, clean, git hooks.

### 5. GitHub

CI/CD workflow status via `gh` CLI.

| Key | Action |
|-----|--------|
| `enter` | View run logs |
| `R` | Rerun workflow |
| `r` | Refresh |

## Commit Suggestion Engine

Parses diffs and generates conventional commit messages:

```
[1] feat(auth): add validation to userInput
[2] fix(auth): fix error handling in processPayment
[3] refactor(api): optimize database query performance
```

Detects: keywords (bug, fix, optimize, cache), function names (Go/JS/TS/Python/Java/C#), variable names, code comments, change patterns.

## Git Hooks

Press `h` to install hooks, `H` to remove, `i` to check status:
- Conventional commit message validation
- No large files
- Detect secrets

## License

[AGPL-3.0](LICENSE)
