---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-06-29T17:21:44Z"
generating: "2026-06-29T17:29:46Z"
prompted: "2026-06-29T17:41:09Z"
branch: dark-factory/recurring-task-cleanup-cron
---

## Summary

- Add a new `cleanup` sub-binary (or subcommand) inside `recurring-task-creator` that, on an hourly cron tick, transitions prior-period recurring-task vault instances left `status: in_progress` to `status: aborted` once the next instance has already materialized.
- Extend the `Schedule` CRD with an optional `spec.skipAutoCleanup: bool` field (default `false`) and mirror it onto the generated task's frontmatter as `audit_style: bool` for cross-check.
- Reuse the tagesschau cron-factory pattern: `cron.NewExpressionCron` from `github.com/bborbe/trading/lib/cron` (a thin wrapper around `libcron.Expression` + `libtime.CurrentTimeGetter` + `libsentry.Client`) so we get sentry-wrapped, ctx-cancellable ticks without re-inventing the wiring.
- Reads vault files via git-rest (HTTP) so vault-cli mid-edits don't race; no Kafka roundtrip on the cleanup path — the materialize pipeline (`agent-task-controller-personal`) is untouched.
- Default cron expression `17 * * * *` (hourly at minute 17) so it does not collide with the publisher's hourly tick; the offset is configurable via env.

## Problem

Inbox-style recurring tasks (`cleanup-obsidian-inbox`, `cleanup-omnifocus-inbox`, `Aquascape PWC`) accumulate stale `in_progress` instances whenever a day or week is skipped. The next period's instance materializes cleanly (because the publisher keeps emitting hourly and the downstream controller dedups by UUID5), but the **prior** instance is never closed — it sits `in_progress` forever, surfaces in `/start-day`, and demands attention. Today's instance covers yesterday's intent, so the prior is noise; manual close-out is a steady operator tax.

Audit-style dailies (`check-prometheus-alerts`, `ibkr-swing-trading`) are the opposite: each missed firing IS the signal — silently aborting the missed day would destroy "we missed Tuesday." Any auto-abort needs a per-Schedule opt-out so the operator can mark which tasks are "abandon cleanly" vs "preserve every missed day."

The original v1 design (an `autoSupersedePrevious` flag + controller change in `agent-task-controller-personal`) coupled the cleanup to the Kafka materialize path and forced every consumer to learn a new field. This spec pivots: put the cleanup cron **inside `recurring-task-creator`** so it shares the Schedule CR lifecycle, runs independently as a sibling binary, and only adds one new field (`skipAutoCleanup`).

## Goal

After this work:

- The `Schedule` CRD accepts `spec.skipAutoCleanup: bool` (default `false`). When `false` (the default), the cleanup cron treats the Schedule as eligible for auto-abort of prior in-progress instances; when `true`, the Schedule is fully skipped.
- A new `cleanup` binary runs in the same namespace as the publisher, sharing the Schedule informer cache (separate informer instance is fine — same CRD, different consumer). On its hourly tick it: lists every Schedule, computes the prior-period period-token per Schedule using the existing `pkg/publisher.PeriodTokenBuilder`, looks up vault files matching `<slug> - <prior-token>` via git-rest `GET /api/v1/files`, and for each file whose frontmatter `status == "in_progress"` AND whose next-period instance already exists (or whose age exceeds cadence), issues a merge-aware `POST` to git-rest setting `status: aborted`, `phase: done`, `completed_date: <now>`, and appending `superseded_by: auto-cleanup-<unix-ts>`.
- The `recurring_task_cleanup_superseded_total` Prometheus counter increments once per superseded file, labelled by `recurrence` and `result` (`success` | `error`).
- The publisher stamps `audit_style: <bool>` onto every generated task's frontmatter, mirroring `spec.skipAutoCleanup`, so an operator can grep the vault and cross-check that "this task IS the audit-style one." This is a cross-check, not a runtime control — the runtime reads `spec.skipAutoCleanup` from the CR.

## Non-goals

- Bulk retro-abort of historical stale instances (separate one-shot script if needed; this cron only walks the most recent prior period per Schedule).
- Cross-recurrence-kind changes (Daily → Weekly mid-flight) — not a real case; the decrement function takes the prior period of the same kind.
- Notification / dashboard surface for what was superseded — the `git diff` on the vault + the Prometheus counter are the observability surface.
- Touching the materialize path in `agent-task-controller-personal` — cleanup runs entirely on read-the-vault, no Kafka roundtrip, no controller change.
- Changing the UUID5 / period-token semantics — Spec 6/9 invariants stand; the cleanup cron consumes the same `PeriodTokenBuilder` the publisher uses.
- Retro-fitting audit-style detection onto legacy instances materialized before the spec shipped — they carry no `audit_style` key; the cron treats absence-of-key as `false` (inbox-style, eligible for cleanup) which matches operator intent for any pre-existing file.
- Changing the `/trigger?date=` handler or the publisher's hourly tick.

## Desired Behavior

1. The `Schedule` CRD's `spec` accepts an optional `skipAutoCleanup: bool` field (default `false`). When unset, the field is treated as `false`. The OpenAPI schema validates it as `type: boolean`, no enum, no required-ness.
2. The `ScheduleTrigger` Go struct in `k8s/apis/task.benjamin-borbe.de/v1` gains a `SkipAutoCleanup *bool` field with `json:"skipAutoCleanup,omitempty"`. Pointer-to-bool so an unset field is distinguishable from an explicit `false`. The store adapter maps it to `schedule.TaskDefinition.SkipAutoCleanup bool` (default `false` on nil).
3. The publisher's frontmatter formatter stamps `audit_style: <bool>` onto every published task, mirroring `def.SkipAutoCleanup`. `audit_style: true` matches `skipAutoCleanup: true`; `audit_style: false` matches `skipAutoCleanup: false` or absent.
4. A new package `pkg/cleanup` contains: (a) `PeriodTokenDecrementor` — pure function `PriorPeriodToken(def, currentToken) (priorToken, error)` consuming the existing `publisher.PeriodTokenBuilder` to produce the prior period's token deterministically; (b) `VaultReader` interface — `GetFile(ctx, path) ([]byte, error)` and `ListFiles(ctx, prefix) ([]string, error)`; (c) `VaultWriter` interface — `UpdateFile(ctx, path, mutator) error` where `mutator([]byte) ([]byte, error)` is a merge-aware callback so vault-cli mid-edits don't get clobbered; (d) `Supersedance` — top-level orchestrator with `Run(ctx, date) error` that, given a Berlin civil date, iterates Schedules, filters out `SkipAutoCleanup == true`, decrements each Schedule's current period token, fetches the matching vault file, and (only when the next-period instance already exists OR the prior file's `planned_date` is older than the cadence) calls the writer.
5. A new top-level entrypoint `cmd/cleanup/main.go` wires the package the same way `cmd/run-once/main.go` does: connector (CRD self-install), informer cache, sentry client, git-rest HTTP client, then `cleanup.Supersedance{...}.Run(...)` wrapped in `cron.NewExpressionCron(sentryClient, run, libcron.MustParse("17 * * * *"))`. The binary exits non-zero on context-cancelled error; otherwise it runs the hourly loop until SIGTERM.
6. Cron expression is configurable via `CLEANUP_CRON` env (default `17 * * * *`). Hourly default at minute 17 so it does not collide with the publisher's hourly tick (`time.NewTicker(time.Hour)` fires on the hour boundary, no minute-of-hour pin).
7. Safety: a prior vault file is **never** superseded when (a) the next-period instance file does NOT exist AND (b) the prior file's `planned_date` is within one cadence of now. This rule makes "today's daily before tomorrow's materializes" a no-op — we never abort the *current* period.
8. Idempotency: re-running on the same hour is a no-op on the second pass. After a successful supersede, the file's frontmatter carries `superseded_by: auto-cleanup-<ts>`, so a second pass sees `status: aborted` and skips.
9. The writer is merge-aware: it reads the current file via git-rest, runs the operator-supplied `mutator` against the existing bytes, then writes the result back. If the file changed between read and write, git-rest returns 409 (per its existing semantics) and the cron logs `git-rest conflict, will retry next tick` — no overwrite, no data loss.
10. Prometheus counter `recurring_task_cleanup_superseded_total{recurrence, result}` increments once per file the cron attempts to supersede, where `result` is `success | conflict | error`. Pre-initialized to 0 for all `recurrence ∈ AllRecurrenceKinds × result ∈ {success, conflict, error}` combinations so scrapes see the series before the first event.

## Constraints

- **Do NOT change** the publisher, the tick, the `/trigger` handler, or the period-token / UUID5 derivation. The cleanup cron is a new sibling; the publisher emits a `task.CreateCommand` with `skipAutoCleanup` mirrored to `audit_style` only as a stamped frontmatter value.
- **CRD invariants**: group, version, kind, plural, singular, short name, scope, and `Names` from `k8s/apis/task.benjamin-borbe.de/v1` remain frozen. The new field is additive (`omitempty`) on `ScheduleTrigger`. CEL rules: no new validation required for `skipAutoCleanup`; the bool is free-form.
- **Period-token formulas** (frozen by Specs 6, 7, 9, 11) MUST be reused verbatim. The cleanup cron consumes `publisher.PeriodTokenBuilder`; it does not re-implement formatting. The decrementor shifts the input `Date` by `-1 period` and calls `Build` again — leveraging the existing `PeriodOffset` machinery where present and a date-shift for the date-anchored kinds.
- **Git-rest is the only writer.** No direct git CLI, no `os.WriteFile`, no in-process git library. The writer interface is satisfied by an HTTP client pointed at the existing `vault-obsidian-personal` git-rest service. New env: `GIT_REST_URL` (required), `GATEWAY_SECRET` (optional, forwarded as `X-Gateway-Secret` per the auth contract established in agent-spec 018 + 096).
- **No Kafka writes** from the cleanup path. The publisher + the controller stay on the Kafka materialize contract; cleanup is read-the-vault + write-the-vault only.
- **Europe/Berlin civil date** is the only clock surface the cron reads; same `time.LoadLocation("Europe/Berlin")` path as the publisher.
- **Project DoD** (`docs/dod.md`) applies: `bborbe/errors` 3-arg `Wrap`, Ginkgo v2 / Gomega, no `time.Now()` / `context.Background()` in business logic, GoDoc on exports, `make precommit` clean.
- **Single-namespace informer** mirrors the publisher: the cleanup binary watches Schedule CRs only in its own pod namespace via `NAMESPACE` Downward-API env.
- **No Prometheus metric sprawl**: exactly one new counter (`recurring_task_cleanup_superseded_total`). No new gauges, no histograms. Reuse `recurring_tasks_last_tick_timestamp_seconds` if a "last cleanup tick" gauge is needed; the publisher's gauge already records the publisher tick.
- **Idempotency contract**: the same `(slug, prior-period)` pair MUST always produce the same period-token — derived from the existing `PeriodTokenBuilder` with the prior-period date as input, so the lookup is deterministic.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility |
|---------|-------------------|-----------|----------|---------------|
| Git-rest unreachable (DNS, 5xx, timeout) | Cron logs `cleanup: git-rest unreachable`, increments `recurring_task_cleanup_superseded_total{result="error"}`, returns error from Run; sentry captures; cron waits for next tick. | Prometheus counter increments; pod log line at V=2; sentry event. | Operator restarts git-rest; next hourly tick retries the same Schedules (idempotent). | Reversible — no state change attempted. |
| Git-rest returns 409 on write (file changed between read and write — vault-cli mid-edit) | Mutator's bytes are discarded; counter increments `result="conflict"`; cron logs and moves to next Schedule. No data loss. | Prometheus counter increments; pod log line. | Next tick retries; if vault-cli finished mid-edit, this tick's re-read sees the latest content and applies the supersede. | Reversible — no write happened. |
| Schedule CR has `skipAutoCleanup: true` | Cron skips the Schedule entirely. No git-rest call. No counter increment. | Pod log line at V=3 if debug-level enabled; no metric delta. | Operator removes the field to re-enable. | N/A — invariant. |
| Schedule CR's prior-period file is missing (never materialized — first-ever instance or pre-Cutover) | Cron skips — there is nothing to supersede. No error, no counter increment. | Pod log line at V=3. | Operator manually backfills if desired; otherwise no action. | N/A. |
| Schedule CR's next-period file does NOT exist AND prior file's `planned_date` is within one cadence of now | Cron skips — protects the *current* period from being aborted prematurely. | Pod log line at V=3. | None — this is the safety property; no recovery needed. | N/A. |
| Frontmatter parser fails on a vault file (malformed YAML, missing required keys) | Cron logs the parse error, increments `result="error"`, moves to next file. Does not abort the whole tick. | Prometheus counter increments; pod log line. | Operator fixes the file manually. | Reversible — no write happened. |
| Two cleanup replicas running concurrently (deployment mistake) | Both replicas run their own cron; the second one's writes hit 409 because the first already wrote `superseded_by`. The second's counter increments `result="conflict"` for the now-already-aborted file. No double-abort, no data corruption. | `result="conflict"` spike in Prometheus. | Fix the deployment to replicas=1. | Reversible. |
| Cron tick returns error | sentry captures; cron waits for next tick. No data loss. | sentry event; pod log line. | Next tick retries. | Reversible. |
| CRD schema rejects `skipAutoCleanup: "yes"` (string instead of bool) at apply time | API server admission rejects the CR — `kubectl apply` exits non-zero. No cleanup cron involvement. | `kubectl apply` output. | Operator fixes the value to a real bool. | Reversible — no state change. |

## Security / Abuse Cases

- **Trust boundary**: the cleanup cron sits inside the cluster and reads/writes the vault via git-rest. The only inbound trust boundary is git-rest auth (`X-Gateway-Secret` + `X-Gateway-Initator` when configured); the cron uses the same auth header pattern as `agent-task-controller-personal`'s `pkg/gitrestclient`.
- **Input validation**: every file the cron reads is parsed as YAML frontmatter; a malformed file is logged and skipped, never written back. Counter input (Prometheus labels) is bounded to the closed `RecurrenceKind` enum + the closed `result` set — no attacker-controlled label cardinality.
- **Path safety**: the cron only ever computes vault paths via `<slug> - <period-token>.md` where `<period-token>` is built by `publisher.PeriodTokenBuilder` (a closed enum of formats). Slugs come from `Schedule.metadata.name` (k8s-validated to match `^[a-z][a-z0-9-]*$` at the CRD boundary per the existing `vaultPattern`). No `..` traversal, no absolute paths, no symlink resolution.
- **Race safety**: the writer is merge-aware; concurrent vault-cli writes are detected via git-rest 409 and the cron defers. Cron-vs-cron races are tolerated (one wins, the other logs conflict). Cron-vs-controller races: the controller writes a brand-new file with a new period token; the cron never touches a file whose period token matches the *current* period — only prior periods.
- **Hang / retry**: the HTTP client has a finite timeout (10s per the convention used in `pkg/gitrestclient`); retry is the next hourly tick, not inline retries. No infinite loops, no exponential-backoff pile-up.
- **Data exfiltration**: the cron reads vault frontmatter and writes a supersede marker; it does not log task body content. PII risk is bounded to the existing vault-access surface (git-rest itself), which is already cluster-internal.

## Period-token decrement table

The cleanup cron needs the **prior period's token** for a Schedule. Per `[[Per-Kind Firing Semantics for Recurring Task Schedulers]]`, the decrement function takes `(def, currentDate)` and produces the prior period's token using the existing `PeriodTokenBuilder`:

| Kind | Decrement rule | Example (today = 2026-06-29, weekday = Monday for Weekday examples) |
|------|----------------|--------------------------------------------------------------------|
| Daily | `currentDate - 1 day` | current token `2026-06-29`; prior token `2026-06-28` |
| Weekday (list) | The most recent firing day on or before `currentDate - 1 day` whose weekday is in `def.Weekdays` | `Weekdays={Sat,Sun}`, today Sunday 2026-06-29: prior firing is Saturday 2026-06-28 → token `2026W27-sat`. If today is Monday 2026-06-29: prior firing is Sunday 2026-06-28 → token `2026W27-sun`. |
| Weekly | `currentDate - 7 days`, then `Build` | current token `2026W27` (Mon 2026-06-29 start); prior token `2026W26` (Mon 2026-06-22 start) |
| Monthly | `currentDate.AddDate(0, -1, 0)` | current `2026-06`; prior `2026-05` |
| Quarterly | `currentDate.AddDate(0, -3, 0)` | current `2026Q2`; prior `2026Q1` |
| Yearly | `currentDate.AddDate(-1, 0, 0)` | current `2026`; prior `2025` |

The decrementor uses the existing `publisher.PeriodTokenBuilder` for the formatting half; it only computes the prior `Date` and feeds it back into `Build`. The `PeriodOffset` field (when non-zero on Monthly/Quarterly/Yearly) applies to the *current* token — the prior token is computed by shifting the date and re-invoking `Build` with the same `PeriodOffset`, so the prior token's offset symmetry is preserved (e.g. a Monthly schedule with `periodOffset=-1` reading "2026-05" on 2026-07-01 has a prior token of `2026-04`, also read with offset -1).

## Acceptance Criteria

Every AC declares its evidence shape. Each is binary, testable from the deployed binary or via unit/integration tests in the implementation prompt.

- [ ] **CRD schema accepts `skipAutoCleanup`.** A Ginkgo `It` in `pkg/k8s_connector_validation_test.go` validates a CR with `spec.skipAutoCleanup: true` succeeds; with `spec.skipAutoCleanup: false` succeeds; with `skipAutoCleanup` absent (default) succeeds. **Evidence**: `go test -v -run 'SkipAutoCleanup' ./pkg/...` prints PASS for each of three scenarios.
- [ ] **CRD schema rejects non-bool.** A Ginkgo `It` validates a CR with `spec.skipAutoCleanup: "yes"` returns a wrapped error whose message mentions `skipAutoCleanup`. **Evidence**: `go test -v -run 'SkipAutoCleanup.*invalid' ./pkg/...` prints PASS.
- [ ] **Go struct field exists.** `grep -n 'SkipAutoCleanup \*bool' k8s/apis/task.benjamin-borbe.de/v1/*.go` returns line ≥1; the field has `json:"skipAutoCleanup,omitempty"` tag. **Evidence**: matched line printed.
- [ ] **Adapter maps `*bool → bool` (default false on nil).** `store.AdaptScheduleForTest` returns `TaskDefinition.SkipAutoCleanup == true` when `cr.Spec.Schedule.SkipAutoCleanup != nil && *cr.Spec.Schedule.SkipAutoCleanup == true`; returns `false` when the field is `nil`; returns `false` when the field points at `false`. **Evidence**: three Ginkgo `It` blocks, `go test -v -run 'SkipAutoCleanup' ./pkg/store/...` prints PASS.
- [ ] **Publisher stamps `audit_style` onto frontmatter.** `FrontmatterFormatter.Format(operator, slug, date)` returns a map containing `audit_style: true` when the def carries `SkipAutoCleanup: true`; `audit_style: false` when the def carries `SkipAutoCleanup: false`. **Evidence**: Ginkgo `It` asserts both, `go test -v -run 'AuditStyle' ./pkg/publisher/...` prints PASS.
- [ ] **`PeriodTokenDecrementor` returns the prior token per kind.** A Ginkgo table-driven test enumerates all 6 recurrence kinds with 3 representative `(today, def)` triples each and asserts the prior token matches the row in the **Period-token decrement table** above. **Evidence**: `go test -v -run 'PriorPeriodToken' ./pkg/cleanup/...` prints PASS for all 18 rows.
- [ ] **`Supersedance.Run` skips when `SkipAutoCleanup == true`.** Stub ScheduleStore returns a def with `SkipAutoCleanup: true`; counter's `VaultReader.ListFiles` is invoked zero times for that def; `recurring_task_cleanup_superseded_total` counter has zero increments. **Evidence**: `go test -v -run 'Supersedance.*SkipAutoCleanup' ./pkg/cleanup/...` prints PASS; mock assertion in the test confirms zero ListFiles calls.
- [ ] **`Supersedance.Run` supersedes a prior in-progress file when next-period exists.** Stub ScheduleStore returns a `Daily` def; stub `VaultReader.GetFile("<slug> - 2026-06-28.md")` returns bytes with `status: in_progress`; stub `VaultReader.ListFiles("24 Tasks")` returns `["<slug> - 2026-06-28.md", "<slug> - 2026-06-29.md"]` (next-period present). `Supersedance.Run` calls `VaultWriter.UpdateFile` once with a mutator that, applied to the bytes, produces frontmatter with `status: aborted, phase: done, completed_date: <now>, superseded_by: auto-cleanup-<ts>`. **Evidence**: Ginkgo `It` asserts the write bytes via a fake writer; `go test -v -run 'Supersedance.*next-period-exists' ./pkg/cleanup/...` prints PASS.
- [ ] **`Supersedance.Run` skips when next-period does NOT exist AND prior is within its own firing window.** Stub returns the same Daily def with `today = 2026-06-29`, prior file `2026-06-28` exists with `status: in_progress`, but the `2026-06-29` file is absent. `Supersedance.Run` does NOT call `VaultWriter.UpdateFile`; counter unchanged. **Evidence**: Ginkgo `It` asserts zero writes; `go test -v -run 'Supersedance.*firing-window' ./pkg/cleanup/...` prints PASS.
- [ ] **`Supersedance.Run` is idempotent on re-run.** Two consecutive `Supersedance.Run` invocations against the same Schedule: first call performs one write with the supersede markers; second call's stub `GetFile` returns the post-write bytes (with `status: aborted`); second call performs zero writes. **Evidence**: Ginkgo `It` asserts second-pass writer invocations = 0; `go test -v -run 'Supersedance.*idempotent' ./pkg/cleanup/...` prints PASS.
- [ ] **First-ever instance is a no-op.** Stub ScheduleStore returns a def whose `<slug> - <prior-token>` file does NOT exist (`ListFiles` does not contain it). `Supersedance.Run` performs zero writes. **Evidence**: Ginkgo `It` asserts zero writes; `go test -v -run 'Supersedance.*first-ever' ./pkg/cleanup/...` prints PASS.
- [ ] **Git-rest 409 conflict is tolerated.** Stub `VaultWriter.UpdateFile` returns a `409 Conflict` error. `Supersedance.Run` increments `recurring_task_cleanup_superseded_total{result="conflict"}` and does NOT panic / does NOT abort the tick. **Evidence**: Ginkgo `It` asserts counter value via a fake metrics interface; `go test -v -run 'Supersedance.*conflict' ./pkg/cleanup/...` prints PASS.
- [ ] **All other writer errors increment `result="error"`.** Stub `VaultWriter.UpdateFile` returns a generic error. `Supersedance.Run` increments the error counter and continues to the next Schedule. **Evidence**: Ginkgo `It` asserts error counter; `go test -v -run 'Supersedance.*error' ./pkg/cleanup/...` prints PASS.
- [ ] **`pkg/cleanup.Supersedance` is wired into `cmd/cleanup/main.go` as a sibling binary.** The new binary file exists, builds (`go build ./cmd/cleanup/...`), and runs one tick via `cmd/cleanup-run-once` (smoke test, mirrors `cmd/run-once`). **Evidence**: `go build ./cmd/cleanup/...` exits 0; `go test -v ./cmd/cleanup/...` exits 0.
- [ ] **Counterfeiter mocks exist for `VaultReader`, `VaultWriter`, `Metrics`.** `mocks/cleanup-vault-reader.go`, `mocks/cleanup-vault-writer.go`, `mocks/cleanup-metrics.go` are generated and committed. **Evidence**: `grep -l 'counterfeiter:generate' pkg/cleanup/*.go | wc -l` reports ≥3; `ls mocks/cleanup-*.go | wc -l` reports ≥3.
- [ ] **`cmd/cleanup` STS manifest added under `k8s/recurring-task-creator-cleanup-sts.yaml`** with the same shape as `k8s/recurring-task-creator-sts.yaml` plus `CLEANUP_CRON` env (default `17 * * * *`) and `GIT_REST_URL` env pointing at the personal vault git-rest service. **Evidence**: file exists; `grep -c 'CLEANUP_CRON' k8s/recurring-task-creator-cleanup-sts.yaml` returns ≥2 (env name + value reference).
- [ ] **`make precommit` exits 0 from repo root after all prompts land.** **Evidence**: exit code 0.
- [ ] **Post-Deploy (Rung-2): dev pod runs and emits the first tick.** `kubectlquant -n dev get pod recurring-task-creator-cleanup-0 -o jsonpath='{.status.phase}'` returns `Running`; `kubectlquant -n dev logs recurring-task-creator-cleanup-0 | grep -c 'cleanup: started'` returns ≥1 within 60s of rollout. **Evidence**: kubectl output captured; log line counted.
  - deploy_check: kubectlquant -n dev get pod recurring-task-creator-cleanup-0 -o jsonpath='{.status.phase}' returns Running
  - deploy_target: recurring-task-creator-cleanup-0
- [ ] **Post-Deploy (Rung-2): E2E verify on a deliberately-missed-day drill.** Operator pauses firing on `cleanup-obsidian-inbox` Schedule for one cycle (sets `pause: true` per existing runbook, or temporarily deletes and re-applies), lets next instance materialize + one cron tick pass, then observes `git diff` on `~/Documents/Obsidian/Personal` shows the prior instance frontmatter with `status: aborted, phase: done, completed_date: <ts>, superseded_by: auto-cleanup-<ts>`. **Evidence**: `cd ~/Documents/Obsidian/Personal && git status` shows a modified `24 Tasks/<title>.md`; `grep '^superseded_by' <file>` returns ≥1 line.
  - deploy_check: cd ~/Documents/Obsidian/Personal && git diff --stat shows a modified 24 Tasks/Cleanup Obsidian Inbox - 2026-MM-DD.md file
  - deploy_target: 24 Tasks/Cleanup Obsidian Inbox - 2026-MM-DD.md (prior instance)
- [ ] **Post-Deploy (Rung-2): audit-style Schedule is preserved.** Operator sets `spec.skipAutoCleanup: true` on `check-prometheus-alerts` and `ibkr-swing-trading` Schedules; deliberately misses one firing; verifies (a) the prior instance's `audit_style: true` frontmatter is unchanged, (b) `recurring_task_cleanup_superseded_total` for `recurrence="Daily"` does not increment for those slugs (verified via Prometheus query or via the cleanup pod logs which name the skipped Schedule). **Evidence**: `git diff` shows zero changes to the audit-style task files; cleanup pod log line `cleanup: skipping <slug>: skipAutoCleanup=true` appears.
  - deploy_check: kubectlquant -n dev logs recurring-task-creator-cleanup-0 | grep -c 'cleanup: skipping' returns ≥1 (covering both check-prometheus-alerts and ibkr-swing-trading); cd ~/Documents/Obsidian/Personal && git diff --stat shows no modification to the audit-style task files
  - deploy_target: check-prometheus-alerts + ibkr-swing-trading Schedules (skipped)
- [ ] **CHANGELOG entry under `## Unreleased`.** **Evidence**: `grep -nE 'feat:.*cleanup|feat:.*skipAutoCleanup' CHANGELOG.md` returns line ≥1 under the `## Unreleased` heading.
- [ ] **README updated with the new binary's commands and env vars** (`CLEANUP_CRON`, `GIT_REST_URL`). **Evidence**: `grep -cE 'CLEANUP_CRON|GIT_REST_URL' README.md` returns ≥2.

**Scenario coverage — default: NO new scenario.** Unit tests in `pkg/cleanup/` cover every AC above (Counterfeiter mocks for the writer / reader / metrics). The Rung-2 Post-Deploy E2E drill is the only "live" verification step, executed by the operator against dev cluster, not by a YOLO scenario.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-cleanup
make precommit
go test -v -run 'SkipAutoCleanup|AuditStyle|PriorPeriodToken|Supersedance' ./...

# Local smoke (mirrors cmd/run-once for the cleanup binary):
go build ./cmd/cleanup/...
GIT_REST_URL=http://localhost:8080 NAMESPACE=test ./cleanup -v=2 -logtostderr

# Post-deploy on dev:
kubectlquant -n dev rollout restart statefulset/recurring-task-creator-cleanup
kubectlquant -n dev get pod recurring-task-creator-cleanup-0 -o jsonpath='{.status.phase}'   # expect Running
kubectlquant -n dev logs recurring-task-creator-cleanup-0 | grep -c 'cleanup: started'      # expect >=1

# E2E verify (operator-driven, post-deploy):
cd ~/Documents/Obsidian/Personal && git status   # expect modified <title>.md after deliberately-missed-day drill
```

Expected:

- `make precommit` exits 0.
- Targeted `go test` prints PASS for every matched spec (CRD + adapter + publisher + decrementor + Supersedance).
- `kubectlquant get pod` returns `Running` within 60s.
- The grep counts return ≥1 within 60s of rollout.
- The vault `git status` shows the supersede on the deliberately-missed-day Schedule and zero changes on the audit-style ones.

## File-by-file change list

| File | Change |
|------|--------|
| `k8s/apis/task.benjamin-borbe.de/v1/types.go` | Add `SkipAutoCleanup *bool` to `ScheduleTrigger`. |
| `pkg/k8s_connector_schema.go` | Add `skipAutoCleanup: { type: boolean }` to `scheduleSpecSchema` properties. No CEL rule needed. |
| `pkg/k8s_connector_validation_test.go` | Add CRD validation tests for the new field (accept bool, reject string). |
| `pkg/store/adapter.go` | Map `cr.Spec.Schedule.SkipAutoCleanup` to `TaskDefinition.SkipAutoCleanup bool` (default false on nil). |
| `pkg/store/store_export_test.go` | Export the new field for tests if needed (likely not — adapter is already exported). |
| `pkg/schedule/task_definition.go` | Add `SkipAutoCleanup bool` field to `TaskDefinition`. |
| `pkg/publisher/frontmatter.go` | Stamp `audit_style: <bool>` from `def.SkipAutoCleanup` after operator keys, before `created_by` (so `created_by` still wins on collision as a force-set provenance). |
| `pkg/publisher/frontmatter_test.go` | Add Ginkgo spec asserting `audit_style` is set for both true and false defs. |
| `pkg/cleanup/period_token_decrementor.go` | New. Pure function `PriorPeriodToken(def, currentDate) (PeriodToken, error)`. |
| `pkg/cleanup/period_token_decrementor_test.go` | New. Table-driven test over all 6 recurrence kinds. |
| `pkg/cleanup/vault.go` | New. `VaultReader`, `VaultWriter`, `VaultClient` interfaces. |
| `pkg/cleanup/supersedance.go` | New. `Supersedance` orchestrator struct + `Run(ctx, date) error` method. |
| `pkg/cleanup/supersedance_test.go` | New. Counterfeiter-driven Ginkgo suite covering all safety / idempotency / conflict ACs. |
| `pkg/cleanup/metrics.go` | New. `Metrics` interface + `NewPrometheusMetrics` impl + counter pre-init. |
| `pkg/cleanup/metrics_test.go` | New. Counter increments labelled by `recurrence` and `result`. |
| `pkg/cleanup/cleanup_suite_test.go` | New. Ginkgo suite entry. |
| `pkg/cleanup/pkg_export_test.go` | New (if needed). Exports for cross-package tests. |
| `pkg/factory/factory.go` | Add `CreateCleanup(scheduleStore, vaultClient, clock, sentryClient, cronExpr, metrics) run.Func`. |
| `cmd/cleanup/main.go` | New. Mirrors `cmd/run-once/main.go` structure but calls `CreateCleanup` and runs `cron.NewExpressionCron(...).Run(ctx)`. Env: `SENTRY_DSN`, `SENTRY_PROXY`, `GIT_REST_URL`, `GATEWAY_SECRET`, `NAMESPACE`, `CLEANUP_CRON`. |
| `cmd/cleanup/main_test.go` | New. Compile test. |
| `cmd/cleanup/Makefile` | New. Mirrors `cmd/run-once/Makefile`. |
| `k8s/recurring-task-creator-cleanup-sts.yaml` | New. Mirrors `recurring-task-creator-sts.yaml` shape with `CLEANUP_CRON` + `GIT_REST_URL` env. |
| `k8s/Makefile` | Add cleanup image target (mirrors publisher image target). |
| `Dockerfile` | Add cleanup binary build step (or a sibling Dockerfile, agent pattern: separate image per binary). |
| `go.mod` | Add `github.com/bborbe/cron`, `github.com/bborbe/trading/lib/cron`, `github.com/bborbe/agent/pkg/gitrestclient` (or relevant HTTP client lib). |
| `mocks/cleanup-*.go` | New. Generated Counterfeiter mocks for the three new interfaces. |
| `CHANGELOG.md` | New entry under `## Unreleased`. |
| `README.md` | Document new binary, env vars, deploy workflow. |
| `docs/architecture.md` | Add `pkg/cleanup` row to the packages table. |
| `docs/system-map.md` | Add the cleanup cron to the system diagram. |

## Suggested Decomposition

This spec touches 6 code layers (`k8s/apis/.../v1`, CRD schema, store adapter, publisher frontmatter, new `pkg/cleanup`, new `cmd/cleanup`, new STS manifest, README/CHANGELOG/docs) and has 24 ACs. Splitting along natural seams avoids a 30-min cross-layer research block per prompt.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | **CRD delta + store adapter + publisher frontmatter**. Add `SkipAutoCleanup *bool` to `ScheduleTrigger`; add `skipAutoCleanup` field to CRD schema (no CEL); widen `store.adapter` to populate `TaskDefinition.SkipAutoCleanup` (default false on nil); publisher's `FrontmatterFormatter` stamps `audit_style` mirroring `def.SkipAutoCleanup`. New tests for each layer. No git-rest, no cron, no new package yet. | 1, 2, 3, 4, 5 | 1, 2, 3, 4, 5 | — |
| 2 | **`pkg/cleanup` package + Counterfeiter mocks**. Build the new package from scratch: interfaces (`VaultReader`, `VaultWriter`, `Metrics`), `PeriodTokenDecrementor`, `Supersedance` orchestrator, Prometheus counter. All ACs covered by Ginkgo specs with mocks. No binary yet, no STS yet. | 6, 7, 8, 9, 10, 11, 12, 13 | 6, 7, 8, 9, 10, 11, 12, 13, 15 | prompt 1 (needs `TaskDefinition.SkipAutoCleanup` and the frontmatter key to exist) |
| 3 | **`cmd/cleanup` entrypoint + git-rest HTTP client + factory wiring + STS manifest + Dockerfile**. New sibling binary mirroring `cmd/run-once/main.go`; new git-rest HTTP client (or reuse of an existing lib); STS manifest + Makefile target + Dockerfile step; CHANGELOG + README + docs/architecture update. | 14, 15, 16 | 14, 16, 17 (binary-side), 21, 22 | prompt 2 (needs `pkg/cleanup.Supersedance` and its constructor) |

Rationale: prompt 1 lands the wire-format delta and the publisher's new stamped key — every other layer already understands `skipAutoCleanup` and `audit_style`. Prompt 2 then builds the cron logic against stable types. Prompt 3 ships the binary + STS — the only step that needs git-rest auth, Dockerfile surgery, and cluster YAML, so isolating it keeps the deployable artifact's blast radius small. Splitting differently (e.g. STS in prompt 2) would mean prompt 2 must wire a real HTTP client before the cron logic is testable in isolation, dragging git-rest connectivity into every test run.

## Rollout plan

Two-step rollout, both via the cron being opt-out (default-on):

1. **Default-on for the three inbox-style Schedules** (`cleanup-obsidian-inbox`, `cleanup-omnifocus-inbox`, `aquascape-pwc`). No `skipAutoCleanup` field — they get cleaned by default.
2. **Opt-out for the two audit-style Schedules** (`check-prometheus-alerts`, `ibkr-swing-trading`). Operator adds `spec.skipAutoCleanup: true` to each CR via `kubectlquant -n prod edit ts <slug>`.

CR edit is hot-reloaded by the existing informer (per the project's operating notes: `kubectl edit ts <slug>` triggers a watch event → informer cache updates → next tick uses the new value, no pod restart). The cleanup cron reads from the same informer cache (separate informer instance is fine — same CRD, different consumer), so the opt-out takes effect on the next hourly tick.

## E2E verify procedure

1. Pick `cleanup-obsidian-inbox` as the test Schedule.
2. Pause its firing for one cycle: `kubectlquant -n prod edit ts cleanup-obsidian-inbox` and set the existing `pause: true` field (per the project runbook), OR temporarily `kubectlquant -n prod delete ts cleanup-obsidian-inbox` and `kubectlquant apply` a paused copy. The downstream controller should NOT materialize today's instance.
3. Restore firing within 24h so tomorrow's tick materializes a fresh instance for today (clean catch-up via the publisher's idempotent UUID5 semantics).
4. Wait for one cleanup-cron tick to pass (hourly, default minute 17).
5. `cd ~/Documents/Obsidian/Personal && git status` — expect a modified `24 Tasks/Cleanup Obsidian Inbox - <prior-token>.md`.
6. `grep -E '^(status|phase|completed_date|superseded_by)' <file>` — expect `status: aborted`, `phase: done`, `completed_date: <ts>`, `superseded_by: auto-cleanup-<ts>`.
7. Repeat steps 1-4 with `check-prometheus-alerts` (which carries `spec.skipAutoCleanup: true`). Expect `git status` to show **zero** modifications to the `Check Prometheus Alerts` task file.

If steps 5-7 hold, the spec is verified.

## Related

- **Task**: `[[Auto-supersede Prior Recurring Task Instance on New Materialization]]` (in Personal vault `24 Tasks/`) — the source task description, including Success Criteria, Out of Scope, and the v1 → v2 pivot rationale.
- **Service KB**: `[[recurring-task-creator]]` (Personal vault `50 Knowledge Base/`) — CRD shape, publisher architecture, operating notes.
- **Period-token reference**: `[[Per-Kind Firing Semantics for Recurring Task Schedulers]]` (Personal vault `50 Knowledge Base/`) — source of truth for the period-token decrement table.
- **Adjacent problem**: `[[vault-cli Recurring Field Conflicts with CR-Driven Re-materialization]]` (Personal vault `50 Knowledge Base/`) — the inverse problem (extra instances from `recurring:` frontmatter); now mitigated by the 2026-06-25 sweep that stripped `recurring:` from all 55 prod CRs.
- **Workflow**: `[[Development Guide]]` (Personal vault `50 Knowledge Base/`) — worktree + PR + merge + dev/prod deploy flow.
- **Repo deltas**: `~/Documents/workspaces/recurring-task-creator/CLAUDE.md` — dark-factory workflow + coding-guidelines pointer.
- **DoD reference**: `[[Closure Patterns]]` (Personal vault) — DoD formatting conventions for vault-side closure; mirrored by `docs/dod.md` for code-side.
- **Cron factory reference**: `~/Documents/workspaces/trading/tagesschau/controller/pkg/factory/factory.go` + `~/Documents/workspaces/trading/lib/cron/cron_expression.go` — the `cron.NewExpressionCron` factory the cleanup cron wraps.
- **Git-rest client reference**: `~/Documents/workspaces/agent/task/controller/pkg/gitrestclient/` (in `bborbe/agent`) — the HTTP client + auth header pattern the cleanup binary reuses.

## Assumptions

- The personal vault's git-rest service (`vault-obsidian-personal`) is the only writer; no concurrent direct-vault writers exist outside of vault-cli. Vault-cli's writes are via the same git-rest and so already share the 409-conflict semantics.
- The `Schedule` CR's `metadata.name` continues to match `^[a-z][a-z0-9-]*$` (existing CRD pattern); the cleanup cron's vault path computation `<slug> - <period-token>.md` is therefore path-safe.
- All 5 currently-materialized `check-prometheus-alerts` and `ibkr-swing-trading` task instances in the vault carry `audit_style` already (post-2026-06-25 the publisher stamps it; legacy pre-spec files without the key are treated as `audit_style: false` and would be eligible for cleanup — this is intentional and matches operator intent for any pre-existing file).
- `kubectlquant -n prod edit ts <slug>` on `check-prometheus-alerts` and `ibkr-swing-trading` to set `spec.skipAutoCleanup: true` is acceptable; no canonical YAML changes are required in `bborbe/quant` for this spec (the operator-driven edit IS the rollout for the audit-style subset).
- The `bborbe/cron` library's `Expression` type and `MustParse` are stable; same library used by `trading/lib/cron` since 2023.
- `github.com/bborbe/agent/pkg/gitrestclient` is importable as a Go module (it's a sub-package of the public `bborbe/agent` repo).
- The publisher's hourly tick and the cleanup's hourly tick can race on the same hour (publisher fires at minute 0, cleanup fires at minute 17). Worst case: the cleanup sees the next-period file present (good), supersedes the prior (intended), and the publisher's next hour does nothing new (idempotent).

## Do-Nothing Option

If we don't do this: inbox-style dailies keep accumulating stale `in_progress` instances on every skipped day/week. Operator manually closes them once a week (steady 5-minute tax). Audit-style dailies stay correct as-is. The status quo is acceptable for a single operator; it does NOT scale to the current growth in inbox-style schedule count (3 today, likely 8+ by Q3 as more "cleanup-X" tasks get added). The cron + opt-out flag is the cheapest design that lets the inbox-style family grow without compounding the close-out tax, while preserving the audit-style family's "every missed day is the signal" property.