---
status: generating
tags:
    - dark-factory
    - spec
approved: "2026-06-14T11:58:12Z"
generating: "2026-06-14T11:58:12Z"
branch: dark-factory/tick
---

## Summary

- Add an hourly cron loop that asks `pkg/schedule` "what should fire today?" and hands each definition to the `pkg/publisher.Publisher` from Spec 2 — once per hour, forever, until `ctx.Done()`.
- "Today" is the local **Europe/Berlin** civil date, derived from an injected `github.com/bborbe/time.CurrentDateTimeGetter` — no `time.Now()` calls in business logic.
- First tick fires immediately on startup (not after a one-hour warmup); subsequent ticks fire every hour. A failed publish for one task is logged and counted but never blocks the rest of the tick. A failed whole-tick is logged and counted but never blocks the next hour's tick.
- Idempotency is inherited end-to-end: deterministic UUID5 in Spec 2 + controller-side de-dup means re-publishing the same `(slug, date)` every hour is a controller no-op. The loop deliberately republishes the day's full set on each tick.
- This spec also wires `main.go` for real (Kafka SyncProducer → `CreateCommandSender` → `Publisher` → `Tick`), deletes the three inherited go-skeleton example packages (`pkg/factory`'s example funcs, `pkg/handler`, `pkg/mathutil`), and installs Prometheus counters/gauge for observability.

## Problem

Spec 1 froze the inventory; Spec 2 made one `(definition, date)` shippable. Nothing yet drives the pump. Without a loop, the service binary still exposes only the inherited go-skeleton example endpoints — Kafka stays silent, the controller never sees a `CreateCommand`, and the migration off `jira-task-creator` stalls one step short of "tasks actually appear in the vault on the right day." A naive loop is easy to write but easy to get wrong in load-bearing ways: reading `time.Now()` directly couples scheduling to wall-clock testing pain; missing `ctx.Done()` discipline means `Ctrl-C` and SIGTERM hang on the next sleep; failing fast on the first publish error kills the day's remaining tasks; missing pre-initialized Prometheus labels means counters silently appear only after the first failure. Encoding the loop's contract — what it computes, when it ticks, how it isolates failure, what it observes — is the only way to land this once and stop worrying about it.

Concurrently, the binary's `main.go` still contains skeleton plumbing (sentry-alert handler, test-loglevel handler, an unused boltkv directory, the `pkg/factory` example funcs, `pkg/handler/*`, `pkg/mathutil/*`). Spec 2 intentionally left these alone to keep its diff focused on the publisher. Now is the moment to delete them: the publisher and tick give us a real wire graph, and leaving dead skeleton next to live code makes the binary harder to read and audit.

## Goal

After this work, running the binary with valid env vars (Kafka brokers + sentry DSN) starts a process that, every hour starting at boot, computes today's Europe/Berlin civil date, looks up `schedule.TasksForDate(date)`, and calls `publisher.Publisher.Publish(ctx, def, date)` for each entry. The process exits cleanly on `SIGTERM`/`SIGINT` within one second of receipt, with no goroutine leak. Per-task publish errors are logged via `glog` and counted via a Prometheus counter labeled by recurrence kind and result, but never abort the surrounding tick. Whole-tick errors (e.g. clock provider fault) are logged and the next tick still fires on schedule. A Prometheus gauge records the wall-clock timestamp of the last tick start. The binary contains no inherited skeleton example code.

## Non-goals

- Do NOT add an HTTP `/trigger?date=YYYY-MM-DD` endpoint — Spec 4. The `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}` admin endpoints stay; the tick loop runs alongside the HTTP server as a separate `run.Func`.
- Do NOT add or change K8s manifests, secrets, STS, ingress — Spec 5.
- Do NOT add per-task or per-recurrence opt-out flags, runtime feature toggles, or any mechanism to disable individual tasks from env/config — invariant; if a future consumer demands variation, that's a separate spec.
- Do NOT make the tick interval configurable — fixed at one hour per the parent-task design decision. If a future consumer demands variation, that's a separate spec.
- Do NOT make the timezone configurable — Europe/Berlin civil date is the contract. If a future consumer demands variation, that's a separate spec.
- Do NOT introduce per-task retry, backoff, batching, or queueing inside the loop. The next hour's tick IS the retry; deterministic UUID5 makes it safe.
- Do NOT publish a "tick succeeded" Kafka message or any other side effect beyond what `publisher.Publish` already emits.
- Do NOT add state persistence ("which tasks already published today") to disk or KV — controller-side de-dup is the source of truth. The loop is stateless across ticks.
- Do NOT preserve the boltkv directory or `libkv.NewResetHandler` / `NewResetBucketHandler` wiring in `main.go` — they are leftover skeleton; the tick has no persistent state.
- Do NOT keep `pkg/factory.CreateTestLoglevelHandler`, `pkg/factory.CreateSentryAlertHandler`, `pkg/handler/*`, or `pkg/mathutil/*` — they are go-skeleton examples with no role in this service. `pkg/factory` keeps `CreatePublisher` (from Spec 2) plus the new constructor this spec adds; nothing else.
- Do NOT delete or modify `pkg/schedule/*` or `pkg/publisher/*` — they are frozen interfaces from Specs 1 and 2.
- Do NOT add a `SIGHUP` / config-reload pathway — the binary restarts on config change.

## Desired Behavior

1. A single Go package owns the hourly cron loop and exposes one constructor that takes (a) the schedule lookup function, (b) a `publisher.Publisher`, (c) a `github.com/bborbe/time.CurrentDateTimeGetter` clock source, (d) a metrics interface, and returns one type with one `Run(ctx) error` method suitable for use as a `run.Func` inside `service.Run`.
2. `Run` performs an initial tick synchronously before entering its `for { select }` loop, so the binary's first publish happens at boot, not one hour later. The initial tick is subject to the same per-task error isolation as every subsequent tick.
3. After the initial tick, the loop ticks every one hour via `time.NewTicker(time.Hour)`. The ticker is stopped in a `defer` on `Run` exit.
4. Each tick: (i) read the current instant from the injected clock, (ii) convert to a Europe/Berlin civil `schedule.Date` (year/month/day in that zone — never UTC, never the binary's `$TZ`), (iii) call `schedule.TasksForDate(date)`, (iv) iterate and call `publisher.Publish(ctx, def, date)` for each.
5. Inside the iteration, the loop checks `ctx.Err()` between tasks. If the context is cancelled mid-tick, the loop returns immediately without finishing the day's remaining publishes; the next process will republish them on its own first tick.
6. A `publisher.Publish` error is wrapped (slug + ISO date in the message) and logged at `glog.Errorf` level, and the metrics counter is incremented with `result="error"` and the task's recurrence kind. The loop then continues to the next task.
7. A successful publish increments the metrics counter with `result="success"` and the task's recurrence kind. There is no per-task "success" log line at default verbosity; `glog.V(2)` may emit one for tracing.
8. Each tick (including the initial one) updates a gauge `recurring_tasks_last_tick_timestamp_seconds` to the wall-clock time the tick started (as Unix seconds, float). If a tick panics or returns early due to context cancellation, the gauge still reflects the start time of that tick.
9. The Prometheus counter `recurring_tasks_published_total{result, recurrence}` is registered with all ten combinations (`recurrence ∈ {daily, weekly, monthly, quarterly, yearly}` × `result ∈ {success, error}`) pre-initialized to zero at startup, so Prometheus scrapers see the metric series before the first event. The gauge is registered once with no labels.
10. The loop returns `nil` (clean shutdown) when `ctx.Done()` fires between ticks or between tasks. It returns a wrapped error only if a non-recoverable startup invariant fails (e.g. the clock source is nil at construction — caught at constructor time, not run time).
11. `main.go` wires the binary's startup as follows, in order: env config → `service.Main` entrypoint → sentry init → Kafka `SyncProducer` (existing) → `task.CreateCommandSender` via `pkg/factory.CreateKafkaCreateSender`-equivalent → `publisher.Publisher` via `pkg/factory.CreatePublisher` (Spec 2) → `tick.Tick` via a new factory constructor → `service.Run` with two `run.Func`s in parallel: the tick loop and the existing HTTP admin server. `run.CancelOnFirstFinish` semantics: if one exits, the other is cancelled.
12. The HTTP admin server keeps `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`. It does NOT keep `/resetdb`, `/resetbucket/{BucketName}`, `/gc`, `/testloglevel`, `/sentryalert`. The boltkv `DataDir` env var and its `libboltkv.OpenDir` call are removed from `main.go`.
13. The inherited skeleton example packages are deleted entirely: `pkg/handler/` (the whole directory), `pkg/mathutil/` (the whole directory). `pkg/factory/factory.go` is rewritten to keep only `CreatePublisher` (from Spec 2) plus the new `CreateTick` constructor this spec adds. The `pkg/factory/factory_suite_test.go` Ginkgo bootstrap stays.

## Constraints

- Module pre-check: this spec adds direct dependencies on `github.com/bborbe/time` (for `CurrentDateTimeGetter`), `github.com/bborbe/run` (for `run.Func` and `run.CancelOnFirstFinish` — already present transitively), and `github.com/prometheus/client_golang/prometheus` (already present). All are part of the bborbe ecosystem and importable from this repo as of the spec date.
- The package MUST NOT call `time.Now()` directly anywhere in business logic (the loop, the date computation, the gauge update). Wall-clock time comes from the injected `CurrentDateTimeGetter`. The `time.NewTicker(time.Hour)` call is permitted because the ticker is a relative-duration scheduler, not a wall-clock read; see `~/Documents/workspaces/coding/docs/go-time-injection.md`.
- The package MUST NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/bborbe/jira-task-creator/...`, or anything that opens a network connection. Kafka I/O is hidden behind the `publisher.Publisher` interface from Spec 2.
- The package MUST NOT walk Kafka topics, KV stores, files, or env vars. Inputs are constructor-injected; outputs are the publisher and the metrics.
- The package MUST follow the Interface → Constructor → Struct → Method pattern from `~/Documents/workspaces/coding/docs/go-architecture-patterns.md`. The interface is named `Tick`; the constructor returns `Tick`; the underlying struct is unexported.
- Context-cancellation discipline MUST follow `~/Documents/workspaces/coding/docs/go-context-cancellation-in-loops.md`: every blocking `select` includes `<-ctx.Done()`, every iteration body that runs without blocking still checks `ctx.Err()` before doing meaningful work.
- Error wrapping MUST use `github.com/bborbe/errors.Wrap`/`Wrapf` per `~/Documents/workspaces/coding/docs/go-error-wrapping-guide.md`.
- Prometheus metrics MUST follow the pre-initialization pattern from `~/Documents/workspaces/coding/docs/go-prometheus-metrics-guide.md` and the shape used in `~/Documents/workspaces/maintainer/watcher/github-build/pkg/metrics.go`: `prometheus.MustRegister` in `init()`, then a loop that calls `.WithLabelValues(...).Add(0)` for every label combination.
- The Europe/Berlin timezone MUST be loaded via `time.LoadLocation("Europe/Berlin")` once at package or struct init, not per-tick. If the load fails (impossible in standard Go runtimes, but possible in stripped containers without tzdata), the constructor returns a wrapped error — the binary will not start.
- Tests follow Ginkgo v2 / Gomega per `~/Documents/workspaces/coding/docs/go-testing-guide.md`. Mocks generated with counterfeiter per `~/Documents/workspaces/coding/docs/go-mocking-guide.md` against the `publisher.Publisher` interface and the `time.CurrentDateTimeGetter` interface. No `time.Sleep` in tests — `time.NewTicker` is replaced for testing via a constructor knob OR the loop is tested by driving one iteration directly through an unexported method called from a same-package test; pick whichever keeps the test surface narrowest (agent decides at impl time).
- `pkg/factory` keeps the file-and-package shape from Spec 2 (`CreatePublisher` lives here). The new `CreateTick` constructor MUST live alongside it, not in a sibling package.
- `make precommit` MUST pass in the changed module.
- License headers MUST be present on every new `.go` file per `~/Documents/workspaces/coding/docs/go-licensing-guide.md`.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---------|-------------------|----------|-----------|---------------|-------------|
| `publisher.Publish` returns an error for one task | Loop logs at `glog.Errorf` with slug + ISO date; increments counter `recurring_tasks_published_total{result="error",recurrence="<kind>"}`; continues to next task | Next hour's tick republishes the failed task; deterministic UUID5 + controller de-dup keeps it safe | `glog.Errorf` line containing the slug substring; counter delta visible at `/metrics`; remaining tasks in the same tick still publish | Reversible — next tick retries | Two ticks one hour apart both publishing the same `(slug, date)` produce identical commands; controller no-ops on the duplicate |
| `publisher.Publish` returns an error for every task in a tick (e.g. Kafka broker hard down) | Every per-task error is logged + counted; counter `result="error"` increments N times; tick completes "successfully" from the loop's perspective | Next hour's tick retries the whole day; operator alerts on counter rate via Prometheus | Counter rate spike on `result="error"`; broker-down also visible on the Kafka producer's own metrics surface | Reversible — next tick retries; if the broker stays down for >1 hour multiple ticks worth of errors accumulate | N/A — single loop, no concurrent ticks |
| Clock source returns a zero `time.Time` (defensive — the injected interface in healthy code never does this) | Tick aborts early, logs at `glog.Errorf`, gauge `recurring_tasks_last_tick_timestamp_seconds` is NOT updated for this tick; loop continues; next tick fires on schedule | Bug in the clock provider — fix and redeploy | Returned error from the tick body is logged; gauge stays at the previous tick's value | Reversible — next tick retries with a fresh clock read | N/A |
| `schedule.TasksForDate` returns an empty slice (no tasks fire today — possible for civil dates that hit no predicate) | Tick logs at `glog.V(2)` "no tasks for date YYYY-MM-DD"; gauge updates normally; no counter increment; loop continues | None needed — this is normal | Gauge update visible; counter unchanged | N/A | N/A |
| `ctx.Done()` fires mid-tick (process receiving SIGTERM) | Loop returns `nil` from the current iteration's task loop; the next `select` exits the outer loop; `Run` returns `nil`; the binary exits clean | None needed — clean shutdown | Process exits with code 0; `glog.V(2)` line "tick loop: context cancelled, exiting cleanly" appears | Reversible — next process startup performs the initial-tick republish | The currently-publishing call to `publisher.Publish` is allowed to finish (it owns the context too); after it returns, the loop exits |
| `ctx.Done()` fires between ticks (process receiving SIGTERM while waiting for the next hour) | Outer `select` receives from `<-ctx.Done()`; `Run` returns `nil` | None needed — clean shutdown | Process exits with code 0; `glog.V(2)` log line appears | Reversible | N/A |
| `time.LoadLocation("Europe/Berlin")` fails at startup (tzdata missing from the container image) | Constructor returns a wrapped error; `service.Main` exits non-zero; the process never enters the main loop | Add tzdata to the container image and redeploy; this is a container-build bug, not a runtime fault | Process exits at startup with `loadLocation Europe/Berlin` in the error chain; non-zero exit code; pod enters CrashLoopBackOff | Irreversible at the running-process level (cannot recover without restart) — but no Kafka I/O occurred | N/A — startup failure |
| Panic inside `publisher.Publish` (defensive — Spec 2 should not panic) | Process exits; `service.Main` (or `libsentry`) records the panic; pod restarts; restart performs an initial tick | Fix the upstream panic and redeploy | Panic stack visible in pod logs and Sentry | Reversible at the pod level — pod restart re-runs the initial tick | The pod restart is a clean restart; no leftover state |
| Two pods running simultaneously (e.g. rolling update overlap) | Both pods perform their initial tick; both publish the same day's set; controller de-dups on the deterministic UUID5; one create, one no-op per task | None needed — STS replicas=1 is the deploy assumption; even if violated, idempotency holds | Two `recurring_tasks_published_total{result="success"}` increments per task for the same `(slug, date)` until one pod is killed; controller logs show duplicate-skip per second pod | Idempotent by design | Safe — `publisher.Publish` holds no state between calls (per Spec 2's contract) |

For specs touching real-world side effects: external unavailability (Kafka down) is covered; schema drift is the publisher's concern (Spec 2); partial-progress crash is covered (pod restart re-runs initial tick); rate-limiting and resource exhaustion are not anticipated (one tick per hour, ~45 tasks per tick, well under any Kafka producer limit); clock skew up to several seconds is harmless because the civil-date computation rounds to year/month/day in Europe/Berlin — only skew of multiple hours could shift the tick across the day boundary and even then idempotency holds.

## Security / Abuse Cases

The tick package has no HTTP surface, no file I/O, no env reads (all inputs come from constructor injection), and no user-controlled input crossing any trust boundary. The only adversarial vector is upstream: a malicious or misconfigured inventory entry from `pkg/schedule` (frozen in Spec 1) or a malicious `CreateCommandSender` (constructed in `main.go` from env-configured Kafka brokers). Both are well outside this package's threat model. The `main.go` change inherits the existing env-config trust model: `KAFKA_BROKERS` and `SENTRY_DSN` are read from environment, validated by `service.Main`, and never logged at default verbosity. No new env vars are introduced (the `LISTEN`, `KAFKA_BROKERS`, `SENTRY_DSN`, `SENTRY_PROXY` fields stay; `BATCH_SIZE` and `DATADIR` are removed — they were boltkv leftovers).

## Acceptance Criteria

- [ ] `make precommit` exits 0 in the recurring-task-creator module — evidence: exit code 0.
- [ ] A Go package owns the tick loop and exposes one interface, one constructor, one struct, one `Run` method per the Interface → Constructor → Struct → Method pattern — evidence: `grep -E '^(type Tick|func NewTick|type tick )' pkg/tick/*.go` lists exactly one interface declaration, one constructor function, and one unexported struct.
- [ ] The constructor accepts four parameters in order: schedule lookup, publisher, clock, metrics — evidence: `grep -nE 'func NewTick\(' pkg/tick/*.go` shows a signature matching `func NewTick(<schedule type>, publisher.Publisher, time.CurrentDateTimeGetter, Metrics) Tick`.
- [ ] The constructor returns a wrapped error if `time.LoadLocation("Europe/Berlin")` fails — evidence: a Ginkgo test stubs `LoadLocation` indirection OR (if not stubbable) a unit test that the error path code exists by reading the source; the prompt-writer picks the simpler path. Evidence shape: either Ginkgo `Expect(err).To(MatchError(ContainSubstring("Europe/Berlin")))` or `grep -n 'Europe/Berlin' pkg/tick/*.go` returning the load call and error-wrap site.
- [ ] `Run(ctx)` performs an initial tick BEFORE entering the ticker `for { select }` — evidence: Ginkgo test with a counterfeiter `Publisher` and a counterfeiter clock observes `publisher.PublishCallCount() > 0` within 100ms of `Run` being called in a goroutine, without any ticker advance.
- [ ] After the initial tick, ticks fire on a 1-hour `time.Ticker` — evidence: Ginkgo test where the ticker is constructor-injectable (or replaced via a test seam) advances the tick channel once and observes a second iteration's worth of `publisher.PublishCallCount()` increment.
- [ ] Each tick reads the current date from the injected clock and converts to a Europe/Berlin civil `schedule.Date` — evidence: Ginkgo test where the clock returns `2025-01-04 23:30 UTC` (which is `2025-01-05 00:30` in Europe/Berlin) observes that `publisher.Publish` is called with `date == schedule.NewDate(2025, time.January, 5)`, not `2025-01-04`. (Two assertion cases: one summer date, one winter date, to verify DST behaviour is correct.)
- [ ] Each tick calls `publisher.Publish` once for every entry returned by `schedule.TasksForDate(date)` — evidence: Ginkgo test with a fixed clock and the real `schedule.TasksForDate` observes `publisher.PublishCallCount() == len(schedule.TasksForDate(expectedDate))` after one tick. (Uses one of the representative dates from Spec 1's tests, e.g. `2025-01-04`.)
- [ ] An error from `publisher.Publish` for one task is logged via `glog.Errorf` (or wrapped+returned-then-logged at the caller) and does NOT prevent subsequent `Publish` calls in the same tick — evidence: Ginkgo test where the counterfeiter `Publisher.PublishReturnsOnCall(0, errors.New("kafka down"))` observes (a) `PublishCallCount() == N` (all calls happened) and (b) the metrics fake records one `error` increment and `N-1` `success` increments for the recurrence kinds involved.
- [ ] The loop checks `ctx.Err()` between per-task `Publish` calls and exits early if the context is cancelled — evidence: Ginkgo test where the context is cancelled after the first `Publish` returns; observes `PublishCallCount() == 1` (not N) and `Run` returns `nil` within 100ms.
- [ ] `ctx.Done()` between ticks causes `Run` to return `nil` cleanly — evidence: Ginkgo test cancels the context while the loop is waiting on the ticker; observes `Run` returns `nil` within 100ms with no error.
- [ ] Prometheus counter `recurring_tasks_published_total` is registered with labels `{result, recurrence}` and pre-initialized to zero for all ten combinations at startup — evidence: a Ginkgo test calls `prometheus.DefaultGatherer.Gather()` (or the project's standard test helper) AFTER package init but BEFORE any tick and observes ten metric families with value `0` matching the cartesian product of `{success, error} × {daily, weekly, monthly, quarterly, yearly}`. Alternative evidence shape: `grep -nE 'WithLabelValues' pkg/tick/metrics.go` shows the pre-init loop and asserts ten label pairs.
- [ ] Prometheus gauge `recurring_tasks_last_tick_timestamp_seconds` is registered and set to the tick start time as Unix seconds (float) on every tick — evidence: Ginkgo test observes the gauge value equals the clock's Unix-second value (within 1) after one tick; `grep -n 'last_tick_timestamp' pkg/tick/metrics.go` returns the gauge declaration.
- [ ] The package does NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/bborbe/jira-task-creator/...`, and does NOT call `time.Now()` directly anywhere — evidence: `grep -E '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/tick/*.go` returns no matches.
- [ ] A counterfeiter mock for `publisher.Publisher` exists at the project-standard mocks path and is used by tick tests — evidence: file exists at `mocks/publisher.go` (or equivalent project-standard path); tick test files import and use it.
- [ ] A counterfeiter mock for the tick metrics interface exists and is used by tick tests — evidence: file exists at `mocks/tick-metrics.go` (or equivalent path); used in tick tests.
- [ ] `pkg/factory` exposes `CreateTick` that builds the tick from a publisher, clock, and the tick metrics — evidence: `grep -nE 'func CreateTick' pkg/factory/*.go` returns exactly one match; a factory test injects fakes and confirms the constructor returns a non-nil `Tick`.
- [ ] `main.go` wires the binary's startup: env config → `service.Main` → sentry init → Kafka SyncProducer → `task.CreateCommandSender` via factory → `publisher.Publisher` via factory → `tick.Tick` via factory → `service.Run` with two `run.Func`s (tick loop + HTTP admin server) — evidence: `grep -nE 'CreatePublisher|CreateTick|service\.Run|libkafka\.NewSyncProducer' main.go` returns at least one match per symbol; `make precommit` succeeds (proves the wiring compiles end-to-end).
- [ ] The HTTP admin router contains exactly `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}` and nothing else — evidence: `grep -nE 'router\.Path' main.go` returns exactly four lines matching these four paths; no `/resetdb`, `/resetbucket`, `/gc`, `/testloglevel`, `/sentryalert` lines remain.
- [ ] `main.go` no longer reads `BATCH_SIZE` or `DATADIR` env vars; no longer opens `libboltkv.OpenDir` — evidence: `grep -nE 'BATCH_SIZE|DATADIR|boltkv' main.go` returns no matches.
- [ ] `pkg/handler/` directory is deleted — evidence: `ls pkg/handler 2>&1` returns "No such file or directory" or equivalent; `grep -rE 'pkg/handler' main.go pkg/` returns no matches outside of removed files.
- [ ] `pkg/mathutil/` directory is deleted — evidence: `ls pkg/mathutil 2>&1` returns "No such file or directory"; `grep -rE 'pkg/mathutil' main.go pkg/` returns no matches.
- [ ] `pkg/factory/factory.go` no longer contains `CreateTestLoglevelHandler` or `CreateSentryAlertHandler` — evidence: `grep -nE 'CreateTestLoglevelHandler|CreateSentryAlertHandler' pkg/factory/*.go` returns no matches; the file contains `CreatePublisher` (from Spec 2) and `CreateTick` (this spec).
- [ ] No scenario test added — covered by unit tests and the existing `make precommit` end-to-end build/test gate; see scenario rule below.

Scenario coverage: NO new scenario. The tick loop is purely in-process: a counterfeiter `Publisher` and counterfeiter clock reach every behavior, including the error-isolation, context-cancellation, DST, and metric-pre-init cases. The real-Kafka end-to-end exercise happens at the Spec 5 deploy step, when the binary is pushed to dev and observed publishing to a real cluster. Adding a scenario here would require Docker + a Kafka broker + a fake controller to confirm de-dup — that is the deploy-spec's verification, not this spec's.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-mvp
make precommit
```

Expected: exit code 0, all tests green, lint clean, license headers present, no forbidden imports. The binary compiles with the new wire graph; the deleted skeleton packages are not referenced anywhere.

Additionally, a smoke check after merge (manually, not part of `make precommit`):

```
cd ~/Documents/workspaces/recurring-task-creator-mvp
go build ./...
./recurring-task-creator --help 2>&1 | grep -E 'kafka-brokers|sentry-dsn|listen'
```

Expected: the `--help` output lists the four kept env vars (`SENTRY_DSN`, `SENTRY_PROXY`, `LISTEN`, `KAFKA_BROKERS`) and does NOT list `BATCH_SIZE` or `DATADIR`.

## Suggested Decomposition

This spec touches three layers: a new `pkg/tick` package, a rewrite of `pkg/factory/factory.go`, and a rewrite of `main.go` plus three package deletions. Raw DB × AC ≈ 13 × 24 = 312, well over threshold, but the prompts decompose cleanly along the layer seams.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `pkg/tick` package: `Tick` interface + constructor + struct + `Run` method + metrics + Europe/Berlin loader + per-task error isolation + context-cancellation discipline + counterfeiter mocks for `Publisher` and the tick metrics + unit tests (initial tick, hourly tick, DST date conversion, error isolation, cancel mid-tick, cancel between ticks, pre-init counter, gauge update) | 1, 2, 3, 4, 5, 6, 7, 8, 9, 10 | tick-package-shape, constructor-shape, location-error, initial-tick, hourly-tick, dst-date, fan-out-publish, error-isolation, ctx-mid-tick, ctx-between-ticks, counter-pre-init, gauge-update, no-forbidden-imports, mocks-exist | — |
| 2 | `pkg/factory` `CreateTick` constructor + factory test that wires fakes; rewrite `main.go` to use Kafka SyncProducer → CreateCommandSender → Publisher → Tick → `service.Run` with two `run.Func`s; trim HTTP router to four endpoints; remove `BATCH_SIZE` / `DATADIR` / `boltkv` wiring; delete `pkg/handler/` and `pkg/mathutil/` directories; rewrite `pkg/factory/factory.go` to drop skeleton handlers | 11, 12, 13 | factory-create-tick, main-wires-tick, http-router-trimmed, boltkv-removed, handler-dir-deleted, mathutil-dir-deleted, factory-skeleton-removed, `make precommit` | prompt 1 |

Rationale: prompt 1 lands the loop's contract in isolation — its only dependency is `pkg/schedule` and `pkg/publisher`, both frozen by Specs 1 and 2. Prompt 2 plugs it into the binary's wire graph and removes the skeleton in one diff so the binary is clean immediately. No cycles. If the executor wants a single prompt, the layer count is within the "judgment call" zone, but two prompts keeps the tick-loop tests landable and reviewable on their own.

## Do-Nothing Option

Without the tick loop, the binary cannot run — Specs 1 and 2 have given us a building and a vault, but no pump. Skipping this spec to jump straight to the HTTP `/trigger` endpoint (Spec 4) would land a service that publishes only when manually poked, defeating the migration goal of "tasks appear without human action." Folding the loop into Spec 2 would re-tangle the publisher's contract with scheduling concerns and force the prompt to land both at once — the exact problem the spec split was designed to prevent. Keeping the skeleton example packages (sentry-alert handler, mathutil clamp, test-loglevel) costs nothing functionally but every future read of `main.go` re-pays the "what is this for?" tax. The existing `jira-task-creator` continues to fire correctly, so there is no production pressure — but every week of delay is another week of the new service consuming reviewer attention without producing value. Recommendation: do this spec next, exactly as scoped.
