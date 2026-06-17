# System Map

`recurring-task-creator` is one stage of a longer pipeline that turns recurring scheduling intent (Kubernetes `Schedule` CRs) into background work executed by AI agents. This doc shows where the binary in this repo fits and what each downstream stage does.

## Pipeline

```
┌─────────────────────────┐
│ operator                │
│ kubectl apply Schedule  │
└──────────┬──────────────┘
           │
           ▼
┌─────────────────────────────────────┐
│ recurring-task-creator              │  THIS REPO
│ (binary in this repo)               │
│                                     │
│ • installs the Schedule CRD on boot │
│ • informer over Schedule CRs        │
│ • hourly tick + GET /trigger        │
│ • renders title + period token      │
│ • UUID5(slug+period) = identifier   │
└──────────┬──────────────────────────┘
           │ task.CreateCommand
           │ (Kafka topic per vault)
           ▼
┌─────────────────────────────────────┐
│ Kafka                               │
│ topic: <stage>-agent-task-v1-...    │
└──────────┬──────────────────────────┘
           │
           ▼
┌─────────────────────────────────────┐
│ vault git-rest                      │
│ (private cluster service)           │
│                                     │
│ • consumes task.CreateCommand       │
│ • writes the Obsidian .md file      │
│   into the target vault git repo    │
│ • UUID5 dedup → idempotent          │
└──────────┬──────────────────────────┘
           │ git commit + push
           ▼
┌─────────────────────────────────────┐
│ Obsidian vault git remote           │
│ (the operator's task tree)          │
└──────────┬──────────────────────────┘
           │ file watched / polled
           ▼
┌─────────────────────────────────────┐
│ task-controller                     │
│ (in bborbe/agent)                   │
│                                     │
│ • watches new task .md files        │
│ • classifies by phase / status      │
│ • emits AssignCommand               │
└──────────┬──────────────────────────┘
           │ task.AssignCommand
           ▼
┌─────────────────────────────────────┐
│ task-executor                       │
│ (in bborbe/agent)                   │
│                                     │
│ • consumes AssignCommand            │
│ • spawns a K8s Job per task         │
│ • the Job runs the AI agent that    │
│   actually does the work            │
└─────────────────────────────────────┘
```

## Where the boundary sits

This repo (`recurring-task-creator`) ships exactly the first stage. It knows nothing about vaults, file paths, controllers, agents, or Jobs — it only emits the well-typed `task.CreateCommand` event onto Kafka. Every downstream stage (vault writer, controller, executor) consumes that event independently.

The contract between this binary and everything downstream is the `task.CreateCommand` schema in `github.com/bborbe/agent/lib`. As long as the event shape is preserved, the upstream (this) and downstream (vault/controller/executor) can evolve independently.

## What lives where

| Component | Repo | Role |
|---|---|---|
| `recurring-task-creator` | this repo (public) | Schedule CR → Kafka `task.CreateCommand` |
| vault git-rest | private | Kafka → Obsidian `.md` in vault git remote |
| task-controller | [bborbe/agent](https://github.com/bborbe/agent) | New task file → `task.AssignCommand` |
| task-executor | [bborbe/agent](https://github.com/bborbe/agent) | `task.AssignCommand` → K8s Job spawning an AI agent |
| Schedule CRs | private | The 45+ recurring entries the operator runs (out of scope for the public binary) |

## Why split this way

- **Open-sourcable substrate.** The scheduling-to-Kafka piece is generic; no operator-specific configuration or content lives here. Anyone wanting recurring `task.CreateCommand` events from CRDs can run this binary against their own `Schedule` CRs.
- **Operator content stays private.** The actual 45 recurring tasks (names, body markdown, vault paths) live in a separate private repo. Same binary, different inputs.
- **Replaceable downstream.** Nothing in this repo cares whether the vault is Obsidian-shaped, where it's hosted, or who executes the tasks. A different vault writer or executor could consume the same Kafka events.
