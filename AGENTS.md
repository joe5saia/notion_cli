# Repository Guidelines

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**
```bash
bd ready --json
```

**Plan validation before work:**
- Think through the tests, commands, or manual checks that will validate success.
- Record those validation steps directly in the bd issue when you create or update it.

**Create new issues:**
```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**
```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**
```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Create first, then execute**: Every task must exist in bd before any implementation begins; include the validation steps you will run in the issue details.
3. **Claim your task**: `bd update <id> --status in_progress`
4. **Work on it**: Implement, test, document following the recorded validation plan.
5. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
6. **Complete**: `bd close <id> --reason "Done"`
7. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

### Auto-Sync

bd automatically syncs with git:
- Exports to `.beads/issues.jsonl` after changes (5s debounce)
- Imports from JSONL when newer (e.g., after `git pull`)
- No manual export/import needed!

### MCP Server (Recommended)

If using Claude or MCP-compatible clients, install the beads MCP server:

```bash
pip install beads-mcp
```

Add to MCP config (e.g., `~/.config/claude/config.json`):
```json
{
  "beads": {
    "command": "beads-mcp",
    "args": []
  }
}
```

Then use `mcp__beads__*` functions instead of CLI commands.

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Document the validation steps inside each bd issue before starting work
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and QUICKSTART.md.

## Tooling & Quality Commands
- Install required tooling with `go install mvdan.cc/gofumpt@v0.9.2` and `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8`; ensure `$(go env GOBIN)` is on `PATH`.
- Format code:  
  `find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' -print0 | xargs -0 gofumpt -w`
- Check formatting (no writes):  
  `find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' -print0 | xargs -0 gofumpt -l`
- Lint: `golangci-lint run`
- Build: `go build -o ./bin/notionctl .`

## Testing Guidelines
- Run `go test ./...` before submitting changes; add focused packages when iterating locally.
- Prefer deterministic tests by mocking external services (Notion API, filesystem edges, etc.).
- Document notable coverage gaps or newly added scenarios in PR descriptions.

## Commit & Pull Request Guidelines
- Commit subjects follow `scope: concise summary (#issue)`; include regenerated artifacts (`docs/flows/**`, `dist/`, index outputs) with the source change.
- Pull requests must use the template’s `### 📝 Description` section, list validation steps (tests, doc rebuilds, deploys), link tickets, and attach screenshots or diffs for doc updates when relevant.
- Commit `.beads/issues.jsonl` immediately after any bd issue change (create/update/close), push to GitHub without delay, and resolve merge conflicts on that file before proceeding with other work.
- After completing the task, commit the code and issue updates, then push the branch to GitHub using `gh`.

## Agent Behaviors
- Delete unused or obsolete files when your changes make them irrelevant (refactors, feature removals, etc.), and revert files only when the change is yours or explicitly requested. If a git operation leaves you unsure about other agents' in-flight work, stop and coordinate instead of deleting.
- Before attempting to delete a file to resolve a local type/lint failure, stop and ask the user. Other agents are often editing adjacent files; deleting their work to silence an error is never acceptable without explicit approval.
- NEVER edit .env or any environment variable files—only the user may change them.
- Coordinate with other agents before removing their in-progress edits—don't revert or delete work you didn't author unless everyone agrees.
- Moving/renaming and restoring files is allowed.
- ABSOLUTELY NEVER run destructive git operations (e.g., git reset --hard, rm, git checkout/git restore to an older commit) unless the user gives an explicit, written instruction in this conversation. Treat these commands as catastrophic; if you are even slightly unsure, stop and ask before touching them. (When working within Cursor or Codex Web, these git limitations do not apply; use the tooling's capabilities as needed.)
- Never use git restore (or similar commands) to revert files you didn't author—coordinate with other agents instead so their in-progress work stays intact.
- Always double-check git status before any commit
- Keep commits atomic: commit only the files you touched and list each path explicitly. For tracked files run git commit -m "<scoped message>" -- path/to/file1 path/to/file2. For brand-new files, use the one-liner git restore --staged :/ && git add "path/to/file1" "path/to/file2" && git commit -m "<scoped message>" -- path/to/file1 path/to/file2.
- Quote any git paths containing brackets or parentheses (e.g., src/app/[candidate]/**) when staging or committing so the shell does not treat them as globs or subshells.
- When running git rebase, avoid opening editors—export GIT_EDITOR=: and GIT_SEQUENCE_EDITOR=: (or pass --no-edit) so the default messages are used automatically.
- Never amend commits unless you have explicit written approval in the task thread.
