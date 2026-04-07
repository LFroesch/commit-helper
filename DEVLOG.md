## DevLog
### 2026-03-23: Doc suite added
Added CLAUDE.md, agent_spec.md. Updated README to scout standard (concise, tables, no emoji headers). Updated WORK.md with feature ideas.

### 2026-01-18: Fixed GitHub Tab Not Showing Correct Repo's Workflows
`gh run list` was running without specifying repo path. Added `repoPath` parameter to all GitHub API calls.

### 2026-01-18: Fixed GitHub Tab & Removed Init Tool
GitHub tab showed "not installed" even when gh CLI was installed. Fixed message batching pattern.

### 2026-01-17: Fixed GitHub Tab Log Viewing
Added proper full-screen log view with scrolling.

### 2026-01-16: GitHub Tab (CI/CD Status)
New tab [5] showing workflow run status via `gh` CLI.

### 2026-01-16: Merge Support & Tools Menu Reorder
Merge functionality with conflict handling. Tools menu reordered.

### 2026-01-15: Core feature completion
Reset commit, confirmation checks, ahead/behind tracking, hooks polish, rebase, clone/init, cherry-pick, revert, clean.

### 2026-01-14: Major Tools Expansion
Git hooks, stash, tags, cherry-pick, revert, clean. Scout-style status bar.

### 2026-01-13: Initial release
Renamed from git-helper. Fixed input handling, scrolling, styling.
