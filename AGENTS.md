# Agent Instructions

## Shell Commands (Non-Interactive)
ALWAYS use non-interactive flags to prevent hanging prompts:
- **Files/Dirs**: Use `-f` (force) for `cp`, `mv`, `rm` (e.g., `rm -rf dir`, `cp -f src dest`).
- **Network**: Use `-o BatchMode=yes` for `ssh` and `scp`.
- **Packages**: Use `-y` for `apt-get` and set `HOMEBREW_NO_AUTO_UPDATE=1` for `brew`.

## Beads Issue Tracker & Workflow
Use **bd (beads)** for ALL task tracking. Do NOT use TodoWrite, TaskCreate, or MEMORY.md. Use `bd remember` for knowledge.
- **Commands**: `bd ready` (find), `bd show <id>`, `bd update <id> --claim`, `bd close <id>`, `bd prime` (full context).
- **Session Completion (MANDATORY)**: Work is NOT complete until `git push` succeeds.
  1. File issues for remaining work.
  2. Run quality gates (tests, linters, builds).
  3. Close or update in-progress `bd` issues.
  4. **Push**: Run `git pull --rebase`, then `bd dolt push`, and `git push`. (Verify with `git status`).
  5. Clean stashes, prune branches, and hand off context.
## Coding Context & Standards
- **Read First**: Review `CONTEXT.md` and related docs before starting.
- **Context 7 (REQUIRED)**: Always prioritize the latest internet references. Use `context7` BEFORE writing code to fetch best practices and current documentation.
- **DRY Comments**: Reference issue IDs in code comments instead of duplicating issue descriptions.
- **Max 120 Lines**: Break code into smaller, manageable functions/files if exceeding this limit.
## GitNexus Code Intelligence (dbsync)
Use GitNexus MCP tools to navigate safely. Run `npx gitnexus analyze` if the index is stale.
- **MANDATORY Rules**:
  - Run `gitnexus_impact({target: "symbol", direction: "upstream"})` BEFORE editing. Warn the user if risk is HIGH/CRITICAL and do not proceed blindly.
  - Run `gitnexus_detect_changes()` BEFORE committing to verify scope.
- **Navigating & Refactoring**:
  - NEVER rename via find-and-replace; use `gitnexus_rename`.
  - NEVER use `grep`; use `gitnexus_query` (for concepts) or `gitnexus_context` (for symbols).
- **Resources (`gitnexus://repo/dbsync/...`)**:
  - `/context` (Overview/Freshness), `/clusters` (Functional areas), `/processes` (Execution flows), `/process/{name}` (Trace).
- **Skills Directory** (`.claude/skills/gitnexus/`): Read specific `SKILL.md` files for Exploring, Impact Analysis, Debugging, Refactoring, Guide, or CLI usage.