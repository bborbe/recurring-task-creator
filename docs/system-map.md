# System Map

`recurring-task-creator` is one of several **task producers** that feed a shared task pipeline. The pipeline materializes tasks as Obsidian vault `.md` files, then routes them to either a human or an AI agent via a Kubernetes-spawned Job. This doc shows the whole pipeline so a reader can place this binary in context.

## High-level shape

```
            ┌─────────────────────────────────────────────────────────┐
            │                                                         │
            │           PRODUCERS  (multiple, listed below)           │
            │                                                         │
            └─────────────────┬───────────────────────────────────────┘
                              │ task.CreateCommand
                              ▼
                    ┌───────────────────┐
                    │      Kafka        │
                    │ <stage>-agent-    │
                    │ task-v1-request   │
                    └─────────┬─────────┘
                              │
                              ▼
                ┌─────────────────────────────┐
                │      task-controller        │  bborbe/agent
                │                             │
                │  • consume CreateCommand    │
                │  • write vault .md via      │
                │    git-rest HTTP            │
                │  • emit TaskUpdated         │
                └─────────┬───────────────────┘
                          │
                          ├─────────────► git-rest ──► Obsidian vault git remote
                          │               (HTTP CRUD over a git repo;
                          │                bborbe/git-rest)
                          │
                          ▼
                ┌───────────────────┐
                │      Kafka        │
                │ <stage>-agent-    │
                │ task-v1-event     │
                └─────────┬─────────┘
                          │
                          ▼
                ┌─────────────────────────────────────────────────┐
                │                task-executor                    │  bborbe/agent
                │                                                 │
                │  • consume TaskUpdated                          │
                │  • look up assignee in Config CRD               │
                │  • if human:    no-op (operator picks it up)    │
                │  • if AI agent: spawn batch/v1 Job from         │
                │                 agent's container image         │
                └─────────┬───────────────────────────────────────┘
                          │
                          ▼
                ┌─────────────────────────────────────────────────┐
                │           K8s Job (per task, per phase)         │
                │                                                 │
                │  Container image from AgentConfig.Image:        │
                │    agent-claude  · agent-pi · agent-code        │
                │    agent-gemini                                 │
                │    maintainer-agent-pr-reviewer                 │
                │    maintainer-agent-github-releaser             │
                │    (plus operator-specific domain agents)       │
                │                                                 │
                │  Env: TASK_ID, TASK_CONTENT, PHASE,             │
                │       KAFKA_BROKERS, BRANCH                     │
                │                                                 │
                │  Runs one phase (planning / execution /         │
                │  ai_review), publishes UpdateFrontmatter back   │
                │  to Kafka → controller updates vault → next     │
                │  phase event fires → loop until phase=done      │
                └─────────────────────────────────────────────────┘
```

## Task producers

Anything that emits `task.CreateCommand` onto the shared Kafka topic counts as a producer. Today's list:

| Producer | Source | Trigger | Repo |
|---|---|---|---|
| **`recurring-task-creator`** | Kubernetes `Schedule` CR | hourly tick (cron) | THIS repo |
| **`github-pr-watcher`** | GitHub REST | new / updated PR on allowlisted repo | `bborbe/maintainer` |
| **`github-build-watcher`** | GitHub REST | red→green or green→red CI episode | `bborbe/maintainer` |
| **`github-release-watcher`** | git (master poll) | non-empty `## Unreleased` in CHANGELOG + `.maintainer.yaml: release.autoRelease: true` | `bborbe/maintainer` |
| **Manual** | operator | `/vault-cli:create-task` or `kubectl apply Schedule` (one-shot) | `bborbe/vault-cli` |

All producers use the same `task.CreateCommand` schema from `github.com/bborbe/agent/lib`. Downstream stages do not care which producer fired the event.

## Cleanup cron

`recurring-task-creator-cleanup` is a sibling binary to the publisher. It does NOT emit Kafka events — it reads and writes the vault directly via git-rest HTTP. On an hourly cron tick (default `17 * * * *`), it:

1. Lists all `Schedule` CRs for today.
2. For each schedule, computes the prior-period token.
3. Checks whether the prior-period `in_progress` file exists AND the next-period file also exists.
4. If both exist, rewrites the prior file's frontmatter to `status: aborted` / `phase: done` (merge-aware via re-read → mutate → write, with 409 detection).

```
   Schedule CRs
        │ watch
        ▼
   recurring-task-creator-cleanup pod  ← hourly cron, no Kafka, no HTTP server
        │ git-rest HTTP GET /files?prefix=<slug>
        ▼
   git-rest  ──► Obsidian vault git remote
        │ git-rest HTTP PUT /files/<path>
        ▲
   (mutate prior in_progress file)
```

The cleanup binary shares the `Schedule` CRD informer with the publisher but has no other coupling to it.

## Task assignees

Every task carries an `assignee:` frontmatter key. The executor's routing decision is binary:

- **`assignee: <username>`** (human) → executor does **not** spawn a Job. The task sits in the vault waiting for the human; the operator finds it via `vault-cli`, `task-orchestrator` (web UI), or directly in Obsidian. When the human marks it done in the vault, the controller picks up the file mutation and emits the result event.
- **`assignee: <agent-name>`** (AI agent) → executor reads the matching `Config` CR of group `agent.benjamin-borbe.de/v1` to get the container image; spawns one `batch/v1` Job per phase; the Job container runs the agent's main, emits an `UpdateFrontmatter` command, exits.

The set of registered AI assignees is discoverable at runtime:

```bash
$ kubectl get configs.agent.benjamin-borbe.de -A
NAMESPACE   NAME                               AGE
dev         agent-claude                       60d
dev         agent-pi                           22d
dev         maintainer-agent-github-releaser   18d
dev         maintainer-agent-pr-reviewer       42d
prod        ... (same set)
```

(An operator's cluster also typically registers their own domain-specific agents; only the generic + platform set is shown above.)

### Reference agents

Four example agents demonstrate that the pipeline is technology-agnostic — each fills the same `planning / execution / ai_review` phase contract using a different backing technology:

| Agent | Backing technology | What it shows |
|---|---|---|
| `agent-code` | no LLM — pure Go | the contract works for deterministic / rule-based work; no AI dependency |
| `agent-claude` | [Claude Code](https://docs.claude.com/claude-code) (Anthropic CLI) | LLM-driven phases using a subscription-billable agent CLI |
| `agent-gemini` | [Gemini API](https://ai.google.dev) | LLM-driven phases via a usage-billed HTTP API |
| `agent-pi` | [pi.dev](https://pi.dev) (MiniMax) | LLM-driven phases via a cheap third-party API (cost-tier swap) |

Same controller / executor contract for all four; they differ only in what each phase actually executes inside its container.

### Platform agents

| Agent | Role |
|---|---|
| `maintainer-agent-pr-reviewer` | reviews `bborbe/*` PRs |
| `maintainer-agent-github-releaser` | semver classify + tag releases |

## Operator surfaces

These never produce or consume Kafka events — they read the vault directly and act as the human's window into the pipeline:

- **`vault-cli`** — Claude Code plugin / CLI for vault CRUD (`/vault-cli:create-task`, `/vault-cli:work-on-task`, `/vault-cli:sync-progress`, `…`). Pure local filesystem I/O against vault `.md` files.
- **`task-orchestrator`** — web UI (FastAPI + Kanban) layered on top of `vault-cli`; watches vault file changes and broadcasts to connected clients. Used to run Claude Code sessions from a board view.

Both are **consumers** of the pipeline: they read tasks that the pipeline already wrote into the vault and mutate frontmatter / body locally. Neither talks to Kafka, CRDs, or the cluster.

## Where this repo's boundary sits

This repo ships **exactly the first producer**: Schedule CR → Kafka `task.CreateCommand`. It knows nothing about controllers, vaults, agents, or Jobs. The contract is the `task.CreateCommand` schema in `github.com/bborbe/agent/lib`; everything downstream can evolve independently as long as that schema holds.

## What lives where

| Component | Repo | Public? | Role |
|---|---|---|---|
| `recurring-task-creator` | [bborbe/recurring-task-creator](https://github.com/bborbe/recurring-task-creator) | ✅ public | THIS — Schedule CR → Kafka `CreateCommand` |
| `git-rest` | [bborbe/git-rest](https://github.com/bborbe/git-rest) | ✅ public | HTTP CRUD over a git remote (vault writer) |
| `task-controller`, `task-executor`, `agent-{claude,pi,code,gemini}` | [bborbe/agent](https://github.com/bborbe/agent) | ✅ public | shared task pipeline + reference agents |
| `github-pr-watcher`, `github-build-watcher`, `github-release-watcher`, `maintainer-agent-pr-reviewer`, `maintainer-agent-github-releaser` | [bborbe/maintainer](https://github.com/bborbe/maintainer) | ✅ public | GitHub-source producers + platform agents |
| `vault-cli` | [bborbe/vault-cli](https://github.com/bborbe/vault-cli) | ✅ public | operator CLI / Claude Code plugin |
| `task-orchestrator` | [bborbe/task-orchestrator](https://github.com/bborbe/task-orchestrator) | ✅ public | operator web UI (Kanban + session driver) |
| Cluster infra (helm services, monitoring, the operator's own Schedule CRs) | bborbe/quant | (private) | operational config + per-operator content |

## Why split this way

- **Open-sourcable substrate.** The scheduling-to-Kafka piece is generic; no operator-specific content lives here.
- **Operator content stays private.** The actual recurring schedules (titles, body markdown, vault paths, assignees) live in a separate private repo (`bborbe/quant`). Same binary, different inputs.
- **Replaceable downstream.** Nothing in this repo cares whether the vault is Obsidian-shaped, where it's hosted, or who executes the tasks. A different vault writer, controller, or executor could consume the same Kafka events.
