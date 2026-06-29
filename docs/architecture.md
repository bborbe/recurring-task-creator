# Architecture

`recurring-task-creator` publishes `task.CreateCommand` events to Kafka on a schedule. The agent task-controller consumes the commands and materializes them as Obsidian vault `.md` files. Replaces the legacy `jira-task-creator` (which wrote subtasks to Jira directly).

## System Boundary

```
        ┌──────────────────┐
        │ recurring-task-  │
        │ creator (this)   │   pkg/schedule  : pure data + lookup
        │                  │   pkg/publisher : build CreateCommand + send
        │                  │   pkg/tick      : hourly cron + metrics
        │                  │   pkg/handler   : /healthz + /trigger
        │                  │   pkg/factory   : composition root
        └─────────┬────────┘
                  │ task.CreateCommand (Kafka)
                  ▼
        ┌──────────────────┐
        │ task-controller  │   external service (in `agent/` repo)
        │ (consumes Kafka) │   writes .md to vault
        └─────────┬────────┘
                  │
                  ▼
            ~/Obsidian/Personal/24 Tasks/*.md
```

The service has no direct knowledge of Obsidian, the vault, or Jira. It speaks only to Kafka.

## Packages

| Package | Role | Imports allowed |
|---------|------|-----------------|
| `pkg/schedule` | The 45-entry recurring-task inventory + `Date` + `RecurrenceKind` + `TasksForDate(date)`. Pure data, no I/O, no clock. | stdlib only (`time` for calendar math) |
| `pkg/publisher` | Builds `task.CreateCommand` from `(TaskDefinition, Date)`: renders templates, builds frontmatter, computes deterministic UUID5 identifier, sends via injected `task.CreateCommandSender`. Optional dry-run mode logs + skips send. | `pkg/schedule`, `agent/lib/command/task`, `bborbe/errors`, `glog` |
| `pkg/tick` | Hourly cron loop. Reads clock (Europe/Berlin civil date), calls `schedule.TasksForDate`, publishes via `publisher.Publisher`. Emits Prometheus counter + gauge metrics. | `pkg/schedule`, `pkg/publisher`, `bborbe/time`, `prometheus` |
| `pkg/cleanup` | Auto-abort of prior-period `in_progress` tasks via git-rest. `Supersedance` orchestrator + `VaultReader`/`VaultWriter` git-rest HTTP client. | `pkg/schedule`, `pkg/store`, `bborbe/time`, `prometheus` |
| `pkg/handler` | HTTP handlers — `/healthz` JSON, `/trigger?date=YYYY-MM-DD` manual replay. | `pkg/publisher`, `pkg/schedule` |
| `pkg/factory` | Composition root — `Create*` constructors that wire everything together. Includes `CreateCleanup` for the cleanup cron. No business logic, no error return. | every other package |
| `main.go` | Server entry point. Long-lived HTTP + hourly tick. | `pkg/factory`, `pkg/publisher`, `pkg/tick` |
| `cmd/run-once` | CLI entry point. Single-tick smoke-test. Same wiring, but exits after one tick. | same as `main.go` |
| `cmd/cleanup` | Deployed cleanup cron binary. Hourly tick via `cron.NewExpressionCron`; reads/writes vault via git-rest; no HTTP server, no Kafka. | `pkg/factory`, `pkg/cleanup` |
| `cmd/cleanup-run-once` | Local smoke-test CLI for the cleanup binary. Resolves Berlin civil date, runs one `Supersedance.Run` tick, exits. | same as `cmd/cleanup` |

The dependency graph is a strict DAG: `schedule` ← `publisher` ← `tick` ← `factory` ← `main`, with `handler` consuming `schedule` + `publisher`. No cycles, no upward edges.

## Idempotency Contract

Every published `CreateCommand` carries a deterministic UUID5 `TaskIdentifier`:

```
TaskIdentifier = UUID5(namespace, "recurring-<slug>-<YYYY-MM-DD>")
```

- `namespace` is a frozen package-level constant in `pkg/publisher/uuid_namespace.go`.
- `<slug>` is the entry's stable identifier from the inventory (e.g. `weekly-review`, `update-finances`).
- `<YYYY-MM-DD>` is the civil date the tick computes.

**Property**: same `(slug, date)` always produces the same identifier. The task-controller dedups by identifier — a retry on the next hourly tick is a no-op; a manual `/trigger?date=...` replay is safe; a pod restart that re-publishes today's set is safe.

**Trade-off**: identifiers shift daily. A monthly task published on Jun 1 and Jun 14 yield DIFFERENT identifiers. If Jun 1's tick is missed, that month's task never lands. → see Spec 6 (period-anchored UUID5) for the planned fix.

## Schedule Inventory

45 entries hard-coded in `pkg/schedule/inventory.go`:

| Kind | Count | Fires when |
|------|-------|-----------|
| Weekly Saturday | 12 | every Saturday |
| Weekly Sunday | 9 | every Sunday |
| Day-of-month=5 | 1 | the 5th of every month |
| May 1st | 2 | yearly on May 1 |
| Monthly day=1 | 17 | the 1st of every month |
| Quarterly boundary | 2 | Jan 1, Apr 1, Jul 1, Oct 1 |
| Yearly Jan-1 | 2 | Jan 1 |

Frozen invariants (any change requires a separate spec):

- **Slugs are frozen.** Renaming a slug changes its UUID5, orphaning any in-flight vault tasks.
- **Inventory shape is frozen** for `pkg/schedule` tests — fidelity asserted against the Jira-source for migration safety.
- **Recurrence-kind enum is closed** — `daily`, `weekly`, `monthly`, `quarterly`, `yearly`. New kinds = new spec.

## Schedule CR Weekday Field

The `spec.schedule` trigger has two mutually exclusive weekday fields. Exactly one of `weekday` (single long-form day) or `weekdays` (non-empty list of at most 7 long-or-short day names) is required iff recurrence == `Weekday`; both fields are rejected on other recurrences. The CEL rule in the CRD schema enforces the exactly-one-of constraint at `kubectl apply` time.

```yaml
spec:
  schedule:
    recurrence: Weekday
    weekday: Saturday  # one of {weekday, weekdays} required iff recurrence == Weekday
# weekdays: [Mon, Wed, Fri]  # alternative: list form (short or long names mix)
```

Go-side, the store adapter reads from either field — both converge on the same `[]time.Weekday` output that the publisher consumes. Single-string CRs (`weekday: Saturday`) produce byte-identical UUID5 / period token / title / body to pre-list behavior.

## Time

- Business logic never calls `time.Now()`. The clock is injected as `libtime.CurrentDateTimeGetter`.
- The hourly tick reads `clock.Now().Time().In(berlin)` once per tick, converts to a civil `schedule.Date`, then publishes against that date.
- ISO-week and quarter boundaries are computed via stdlib `time.ISOWeek()` — calendar math, not clock reads. See `pkg/publisher/render.go` for the formatter helpers.

## HTTP Surface

| Path | Method | Purpose |
|------|--------|---------|
| `/healthz` | GET | k8s liveness — JSON `{"status":"ok"}` |
| `/readiness` | GET | k8s readiness — plain `OK` |
| `/metrics` | GET | Prometheus scrape |
| `/setloglevel/{level}` | GET | runtime glog verbosity |
| `/trigger?date=YYYY-MM-DD` | GET | operator replay — publishes the day's full task set |

**Security**: the service has no k8s `Ingress` — cluster-internal `Service` only. All external access goes through the bborbe `trading/frontend/gateway`, which owns auth. `/trigger` and `/setloglevel` are intentionally unauthenticated at this layer; the gateway is the auth boundary.

## Deployment

Single-replica `StatefulSet` per stage (`dev`, `prod`):
- Sidecars: none
- Volumes: `tmp` emptyDir (read-only root FS otherwise)
- Resources: 100m CPU / 128Mi memory (mostly idle between ticks)
- Kafka auth: strimzi mTLS via `KafkaUser` CRD (`recurring-task-creator-user.yaml`)
- Prometheus: pod-annotation scrape (no `ServiceMonitor` CRD)

Build + deploy: `BRANCH=dev make buca` (or `/make-buca . dev`). See [[Development Guide]] for the canonical worktree + PR flow.

## Local Smoke-Test

```bash
DRY_RUN=true ./recurring-task-creator-run-once -logtostderr -v=2
```

Skips Kafka client init entirely (uses `publisher.NewNoopSender`); logs every `(slug, date, identifier)` triple the publisher WOULD send. Use to verify inventory output before deploying to dev.

## Future Work

- **Spec 6 (in flight)**: period-anchored UUID5 — `recurring-<slug>-<period>` where `<period>` is `YYYYWww` / `YYYY-MM` / `YYYYQq` / `YYYY` depending on recurrence kind. Lets every tick publish the full inventory; controller dedups per period; missed ticks no longer mean missed tasks.
- **Kubernetes CRD for task definitions**: replace the hard-coded `pkg/schedule/inventory.go` with a `RecurringTask` custom resource so task definitions are deployable as Kubernetes objects. Frees the inventory from requiring a code release per change.
- **Per-task disable flag**: runtime opt-out so a stale task can be skipped without a code release.

## Related

- [Development Guide](<vault-deeplink>)
- Parent task: [Migrate recurring Jira subtasks to vault task system](<vault-deeplink>)
- Predecessor: `jira-task-creator` (`<predecessor-repo>/`)
- Downstream consumer: `agent/task/controller`
