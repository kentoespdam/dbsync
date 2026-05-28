# Project Instructions for AI Agents

**Canonical instructions for ALL agents on this project (Claude, Gemini, Codex, etc.).**
`AGENTS.md` and `GEMINI.md` are thin pointers — keep new rules here.

## Project Overview

`dbsync` = single-binary Go tool untuk **one-way MySQL table sync** (source → dest). TUI (bubbletea) untuk setup interaktif + CLI (cobra) untuk cron. SQLite = single source of truth. Pure Go (no CGo).

**Read first:** `CONTEXT.md` (quick orientation) → `docs/PRD-v1.md` (full spec) → `docs/ARCHITECTURE.md` (design). Per-issue brief di `docs/issues/00N-*.md`.

## Shell — Non-Interactive Only

Cegah hang prompt:
- Files/Dirs: `-f` untuk `cp`/`mv`/`rm`.
- SSH/SCP: `-o BatchMode=yes`.
- Packages: `-y` untuk `apt-get`; `HOMEBREW_NO_AUTO_UPDATE=1` untuk `brew`.

## Coding Standards

- **Read `CONTEXT.md` + relevant `docs/` before starting.**
- **`context7` REQUIRED** untuk lib (bubbletea, cobra, mysql, sqlite, scrypt) sebelum tulis kode — internal knowledge stale.
- **DRY comments:** reference issue ID (`bd-XX`), jangan duplikasi deskripsi issue di kode.
- **Max 120 lines per file/function** — pecah kalau lebih.
- **No DI framework, no premature abstraction** — junior dev + local AI readable.
- **Test wajib:** `crypto`, `storage`, `mysql`, `engine`. `cli`/`tui` cukup manual QA.
- Integration test: tag `//go:build integration` agar `go test ./...` default tidak butuh Docker.

## Build & Test

```bash
go build -o dbsync ./cmd/dbsync
./dbsync                                            # TUI
./dbsync run --connection=<name> --table=<table>    # CLI
go test ./...                                       # unit
go test -tags=integration ./...                     # + MySQL containers
```

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **dbsync** (2044 symbols, 6068 relationships, 133 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/dbsync/context` | Codebase overview, check index freshness |
| `gitnexus://repo/dbsync/clusters` | All functional areas |
| `gitnexus://repo/dbsync/processes` | All execution flows |
| `gitnexus://repo/dbsync/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
