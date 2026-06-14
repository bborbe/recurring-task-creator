# CLAUDE.md

`recurring-task-creator` — a private Go service that publishes `task.CreateCommand` events to Kafka on a schedule so the agent task-controller materializes recurring tasks (daily/weekly/monthly/quarterly/yearly) as Obsidian vault files. Replaces `jira-task-creator`.

## Dark Factory Workflow

The headline reason to use prompts/specs: **safe unattended execution**. They run inside a YOLO Claude container with permission checks disabled, sandboxed from the host. Queue work, step away, come back to commits — no "Approve this Bash command?" interruptions. Documentation, decomposition (specs), and token savings (Sonnet vs Opus) follow as side benefits.

The decision is about **what artifact deserves to be committed alongside the change**, not size or complexity.

### Choosing a Flow

| Kind of change | Flow | What gets committed | Why this flow |
|----------------|------|---------------------|---------------|
| Doc / config / yaml — no code | **Direct** — edit + commit yourself | Just the diff | Ceremony adds no value when there are no tests to run and no business "why" to document |
| Code change of any size | **Prompt** — write a prompt, audit, approve, daemon executes | Prompt + diff | The prompt provides structure (tests run, auto-commit, auto-release) and is the technical "how" record. Even small refactors benefit. |
| Feature delivering business value | **Spec → prompts** — write spec, audit, approve, daemon auto-generates prompts, audit each, approve, daemon executes | Spec + prompts + diff | The spec is the durable record of *why this feature exists*. Prompts handle the mechanical breakdown. |

### How to decide

- **Is there code changing?** No → direct. Yes → prompt or spec.
- **Is there a business-level "why" that deserves its own document?** No → prompt is enough. Yes → spec first.

The split between prompt and spec is **business-why vs technical-how**, not big vs small.

### Complete Flow

**Direct (trivial doc / config / single-line change):** edit + commit + push yourself.

**Standalone prompts (simple code changes):**
1. Create prompt → `/dark-factory:create-prompt`
2. Audit prompt → `/dark-factory:audit-prompt`
3. User confirms → `dark-factory prompt approve <name>`
4. Start daemon → `dark-factory daemon` (Bash `run_in_background: true`)

**Spec-based (multi-prompt features):**
1. Create spec → `/dark-factory:create-spec`
2. Audit spec → `/dark-factory:audit-spec`
3. User confirms → `dark-factory spec approve <name>`
4. Daemon auto-generates prompts from spec
5. Audit prompts → `/dark-factory:audit-prompt`
6. User confirms → `dark-factory prompt approve <name>`
7. Start daemon → `dark-factory daemon` (Bash `run_in_background: true`)

### Read the relevant guide before starting — every time, not from memory

- Writing a spec → [[Dark Factory - Write Spec]] and [[Dark Factory Guide#Specs What Makes a Good Spec]]
- Writing prompts → [[Dark Factory - Write Prompts]] and [[Dark Factory Guide#Prompts What Makes a Good Prompt]]
- Running prompts → [[Dark Factory - Run Prompt]]

### Claude Code Commands

| Command | Purpose |
|---------|---------|
| `/dark-factory:create-spec` | Create a spec file interactively |
| `/dark-factory:create-prompt` | Create a prompt file from spec or task description |
| `/dark-factory:audit-spec` | Audit spec against preflight checklist |
| `/dark-factory:audit-prompt` | Audit prompt against Definition of Done |
| `/dark-factory:verify-spec` | End-to-end verify a spec interactively, then mark complete |

### CLI Commands

| Command | Purpose |
|---------|---------|
| `dark-factory spec approve <name>` | Approve spec (inbox → queue, triggers prompt generation) |
| `dark-factory prompt approve <name>` | Approve prompt (inbox → queue) |
| `dark-factory daemon` | Start daemon (watches queue, executes prompts) |
| `dark-factory run` | One-shot mode (process all queued, then exit) |
| `dark-factory status` | Show combined status |
| `dark-factory prompt list` | List all prompts |
| `dark-factory spec list` | List all specs |
| `dark-factory prompt retry` | Re-queue failed prompts |
| `dark-factory prompt cancel <name>` | Cancel prompt (never `docker kill`) |

### Key rules

- Prompts go to **`prompts/`** (inbox) — never to `prompts/in-progress/` or `prompts/completed/`
- Specs go to **`specs/`** (inbox) — never to `specs/in-progress/` or `specs/completed/`
- Never number filenames — dark-factory assigns numbers on approve
- Never manually edit frontmatter status — use CLI commands
- Always audit before approving
- Always verify before completing — use `/dark-factory:verify-spec <id>` over manual `dark-factory spec complete`
- **Spec-linked prompts are daemon-generated** — never hand-write prompts for an approved spec
- **Standing workflow: audit → fix → approve.** After creating a spec or prompt: (1) run the auditor, (2) apply fixes from the audit report, (3) if the resulting artifact has no critical/blocker issues, run `dark-factory spec approve <name>` / `dark-factory prompt approve <name>` **without asking** — this is pre-approved. If the audit surfaces unresolvable blockers, stop and report.
- **BLOCKING: Never run `dark-factory daemon` without explicit user confirmation** (separate from approve — daemon spawns containers, makes commits, opens PRs)
- **Before starting daemon** — run `dark-factory status` first
- **Start daemon in background** — use Bash `run_in_background: true`

## Development Standards

Follows Benjamin Borbe's coding guidelines (https://github.com/bborbe/coding-guidelines).

**For the YOLO agent**: read all markdown files in `~/Documents/workspaces/coding-guidelines/` before any code change. Critical ones for this project:

- **[go-architecture-patterns.md](~/Documents/workspaces/coding-guidelines/go-architecture-patterns.md)** — Interface → Constructor → Struct → Method pattern
- **[go-testing-guide.md](~/Documents/workspaces/coding-guidelines/go-testing-guide.md)** — Ginkgo v2 / Gomega
- **[go-makefile-commands.md](~/Documents/workspaces/coding-guidelines/go-makefile-commands.md)** — `make test`, `make precommit`, `make buca`
- **[go-mocking-guide.md](~/Documents/workspaces/coding-guidelines/go-mocking-guide.md)** — Counterfeiter
- **[git-commit-workflow.md](~/Documents/workspaces/coding-guidelines/git-commit-workflow.md)** — commits + changelog

### Build Commands

| Command | Purpose |
|---------|---------|
| `make test` | Run unit tests with race detector |
| `make precommit` | Lint + test + security scan + license headers |
| `make run` | Run service locally (reads `example.env`) |
| `BRANCH=dev make buca` | Build, upload image, commit k8s, apply (dev) |
| `BRANCH=prod make buca` | Same for prod |

### Toolchain

- Go 1.26.x (see `go.mod`)
- Ginkgo v2 / Gomega
- Counterfeiter v6 for mocks
- bborbe libraries: `errors`, `time`, `http`, `kafka`, `sentry`, `service`, `agent/lib`

## Architecture

```
recurring-task-creator/
├── main.go                   — startup (env config, service.Main, deps wiring)
├── pkg/
│   └── schedule/             — (Spec 1) typed recurring-task inventory + TasksForDate()
│   ├── (publisher)           — (Spec 2) Kafka task.CreateCommandSender + UUID5 + frontmatter
│   ├── (tick)                — (Spec 3) hourly cron loop wiring schedule + publisher
│   └── (handler/trigger)     — (Spec 4) HTTP /trigger?date=YYYY-MM-DD manual replay
└── k8s/                      — STS, service, ingress, secret, user
```

Inherited skeleton example packages (`pkg/factory`, `pkg/handler`, `pkg/mathutil`) will be deleted in the publisher spec, not the schedule spec.

## Key Design Decisions

(Reference: parent task `~/Documents/Obsidian/Personal/24 Tasks/Migrate recurring Jira subtasks to vault task system.md`)

- **Hourly idempotent cron tick** — service runs hourly, computes "what should exist for today", publishes missing tasks. Idempotency via deterministic `TaskIdentifier = UUID5("recurring-<slug>-<YYYY-MM-DD>")` — controller no-ops on duplicate.
- **Per-subtask vault tasks** — no story container; one vault task per subtask. Matches `/start-day` `[ ]` surfacing.
- **Drop K3s LatestVersionGetter** — no external HTTP call from the schedule; static title "Update K3s" is enough.
- **Europe/Berlin civil date** — schedule predicates take a civil date in Europe/Berlin, not `time.Time` with location ambiguity.
- **Closed predicate primitive set** — weekday-in-set, day-of-month-in-set, month-and-day, every-day, quarter-boundary, year-boundary. Adding a new kind is a new spec.
- **No Jira / ADF / Kafka / HTTP imports in `pkg/schedule/`** — that package is pure data and pure functions.
- **Slugs are frozen** — renaming a slug after merge breaks the deterministic UUID5 and is itself a new spec.
- **No `LICENSE` file, no Claude PR review workflow** — private personal-infra repo, solo maintainer.

## Cutover Pattern

This service runs in parallel with `jira-task-creator` for one cycle (Phase 3 of the parent task), then `jira-task-creator` is scaled to zero and its repo archived. No double-write coordination is needed because both services write to different sinks (Jira API vs Kafka).
