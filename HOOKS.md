# Git Commit Hooks in Commit Helper

## What are Git Hooks?

Git hooks are scripts that Git automatically runs at certain points in the Git workflow. They allow you to customize and automate parts of your development process.

## Commit Message Hook

The commit-helper tool can install a `commit-msg` hook that:

1. **Validates commit messages** against conventional commit format
2. **Runs automatically** every time you try to commit
3. **Blocks invalid commits** that don't follow the format

## Installation Location

The hook is installed at: `your-repo/.git/hooks/commit-msg`

## How to Use

### Install the Hook
- Press `h` in the commit-helper tool
- Or manually copy the script to `.git/hooks/commit-msg` and make it executable

### Remove the Hook
- Press `H` (Shift+h) in the commit-helper tool
- Or manually delete the file `.git/hooks/commit-msg`

### Check Hook Status
- Press `i` in the commit-helper tool
- The repository header shows 🔒 when a hook is active

## Conventional Commit Format

The hook enforces this format:
```
type(scope): description
```

**Valid types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc)
- `refactor`: Code refactoring
- `test`: Adding or modifying tests
- `chore`: Build process or auxiliary tool changes

**Examples:**
- `feat(auth): add user authentication`
- `fix(api): resolve timeout issue`
- `docs: update README installation steps`
- `test(utils): add validation tests`

## What Happens When You Commit?

### Without Hook
- Any commit message is accepted
- No validation

### With Hook
- Git runs the validation script before finalizing the commit
- Valid messages: commit proceeds normally
- Invalid messages: commit is rejected with helpful error message

## Benefits

1. **Consistent commit history** - All commits follow the same format
2. **Better readability** - Easy to understand what each commit does
3. **Automated changelog generation** - Tools can parse conventional commits
4. **Team standardization** - Everyone follows the same conventions

## Removing/Disabling

The hook can be easily removed:
1. Use the commit-helper tool (`H` key)
2. Delete the file manually: `rm .git/hooks/commit-msg`
3. Rename the file to disable temporarily: `mv .git/hooks/commit-msg .git/hooks/commit-msg.disabled`

## Notes

- Hooks are **local to each repository** - they don't get pushed to remote repos
- Each developer needs to install the hook individually
- Hooks only affect commits made through `git commit` - they don't affect existing commits
