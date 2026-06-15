---
status: completed
spec: [006-period-anchored-uuid]
summary: 'Switched pkg/tick to full-inventory publisher: added schedule.Inventory() accessor, changed NewTick signature to take []schedule.TaskDefinition instead of schedule.ScheduleLookup, updated tick struct/work loop/factory wiring, rewrote all existing tick tests for the new constructor, added a new full-inventory test that asserts publish count equals len(schedule.Inventory()) for three different civil dates, and added a feat: CHANGELOG bullet. make precommit exits 0 with coverage 80.6% in pkg/tick.'
container: recurring-task-creator-always-fire-exec-012-full-inventory-tick
dark-factory-version: v0.177.1
created: "2026-06-14T20:30:00Z"
queued: "2026-06-14T20:21:21Z"
started: "2026-06-14T20:21:22Z"
completed: "2026-06-14T20:31:46Z"
branch: dark-factory/period-anchored-uuid
---

<summary>
- Switches the hourly tick from a per-day-filtered publisher to a full-inventory publisher: every hour, the tick iterates every entry in the recurring-task inventory and calls `publisher.Publish` once per entry.
- Adds a new `pkg/schedule.Inventory()` accessor that returns the canonical inventory slice (a fresh copy to keep callers from mutating the package-level state). The tick consumes this directly; `schedule.TasksForDate` is no longer called by the tick.
- The tick's constructor signature drops the `ScheduleLookup` parameter. The tick now takes `(ctx, inventory, publisher, clock, metrics)` and returns `(Tick, error)`. The existing `schedule.ScheduleLookup` type stays in the tree for the `pkg/handler/trigger` HTTP handler, which still needs per-day filtering for `?date=` manual replay.
- The `?date=` trigger handler (`pkg/handler/trigger.go`) is unchanged — manual replay of a specific date still filters by `TasksForDate`. The factory wiring (`pkg/factory/factory.go`) is updated to call the new `NewTick` with `schedule.Inventory` (tick path) and to keep `schedule.TasksForDate` for the trigger handler.
- Per-task error isolation, Prometheus counter + gauge, and the initial-then-hourly loop structure are preserved. The per-task `context-cancellation` discipline is preserved.
- A new tick test asserts `publisher.PublishCallCount() == len(schedule.AllDefinitionsForTest())` (derived at test time, not hardcoded) for at least three distinct civil dates spanning different weekdays and months.
- `make precommit` exits 0; coverage on `pkg/tick` stays at or above 80%; `pkg/schedule` tests still pass with no inventory edits.

</summary>

<objective>

Change the hourly tick in `pkg/tick` from a per-day-filtered publisher (which used `schedule.TasksForDate(date)` to compute the day's task set and then published only those entries) to a full-inventory publisher (which iterates every entry in `schedule.Inventory()` and calls `publisher.Publish` once per entry, every hour). This relies on the period-anchored identifier introduced by Prompt 1: every retry within the same period produces the same UUID5, so the controller's de-dup absorbs the duplicates and a missed tick is fully recovered by the next tick in the same period. The `pkg/handler/trigger.go` HTTP handler (manual replay of a specific date) is unchanged and still uses `schedule.ScheduleLookup` / `schedule.TasksForDate`.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before writing code:

- `/workspace/pkg/tick/tick.go` — current tick. The `Tick` interface (with `Run(ctx) error` and `RunOnce(ctx) error` methods), the `NewTick(ctx, scheduleFn, pub, clock, metrics) (Tick, error)` constructor, the unexported `tick` struct, the `Run` and `RunOnce` methods, and the internal `tick(ctx)` work loop. The current constructor parameter is `scheduleFn schedule.ScheduleLookup` (the function type from `pkg/schedule/lookup.go`); the prompt replaces this with a `[]schedule.TaskDefinition` parameter (the inventory). The counterfeiter directive on the `Tick` interface stays as-is (`-o ../../mocks/tick-tick.go --fake-name TickTick . Tick`).
- `/workspace/pkg/tick/metrics.go` — `Metrics` interface (`IncPublished(result, recurrence string)`, `SetLastTickTimestamp(seconds float64)`), the `NewPrometheusMetrics()` constructor, the `recurringTasksPublishedTotal` counter, the `recurringTasksLastTickTimestamp` gauge, and the `init()` block that pre-initializes the 10 label combinations. The metrics surface is unchanged by this prompt; the tick still calls `IncPublished` once per `Publish` attempt and `SetLastTickTimestamp` once per tick.
- `/workspace/pkg/tick/tick_test.go` — existing Ginkgo test suite (21 cases). The test setup in `BeforeEach` constructs a 1-entry `scheduleFn` for fast feedback and overrides it per-test. The `publisher.PublishCallCount()` assertions are the key evidence of the new contract.
- `/workspace/pkg/schedule/lookup.go` — `type ScheduleLookup func(date Date) []TaskDefinition`. KEEP this type — `pkg/handler/trigger` still uses it (it consumes `schedule.TasksForDate` as the per-day filter for manual replay of a specific `?date=` parameter). The trigger handler is the only remaining consumer outside the tick; once Prompt 2 lands, the tick is no longer a consumer.
- `/workspace/pkg/schedule/tasks_for_date.go` — `func TasksForDate(d Date) []TaskDefinition`. KEEP this function — the trigger handler still calls it. The spec explicitly allows leaving this and `predicate.go` in the tree as library code; the implementer's call is at impl time (the spec says "whether the predicate code is deleted or left as dead code is the implementer's call at impl time"). For this prompt, leave them in place — the trigger handler is a real consumer.
- `/workspace/pkg/schedule/inventory.go` — package-level `var inventory = []TaskDefinition{ ... }` slice (currently 45 entries). The new `schedule.Inventory()` function returns a fresh copy of this slice. Add `Inventory()` to this same file (do NOT create a new file — `inventory.go` is the natural home for the exported accessor next to the unexported slice).
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest() []TaskDefinition` returns a fresh copy. The new full-inventory test uses this accessor to compute the expected `PublishCallCount()` at test time (NOT a hardcoded literal — the spec's AC #7 says the inventory may grow and the test must not regress).
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` constants. Unchanged.
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition` struct. Unchanged.
- `/workspace/pkg/schedule/date.go` — `Date{Year, Month, Day}`. Unchanged. The tick still converts wall-clock to Berlin civil `Date` before publishing.
- `/workspace/pkg/handler/trigger.go` — `NewTriggerHandler(publisher, lookup schedule.ScheduleLookup) http.Handler`. The handler signature is unchanged; the factory still wires it with `schedule.TasksForDate` as the lookup. The `?date=` filter path stays intact.
- `/workspace/pkg/handler/healthz.go` — liveness handler. Unchanged.
- `/workspace/pkg/factory/factory.go` — `CreateTick(ctx, pub, clock, metrics) tick.Tick` and `CreateTriggerHandler(publisher, lookup schedule.ScheduleLookup) http.Handler`. The `CreateTick` function needs to be updated to pass the new `schedule.Inventory` to `NewTick`. `CreateTriggerHandler` is unchanged (still passes `schedule.TasksForDate`).
- `/workspace/cmd/run-once/main.go` — wires `factory.CreateTick(ctx, pub, clock, metrics)` and calls `tickLoop.RunOnce(ctx)`. Unchanged — it uses the factory, so the new `NewTick` signature is invisible to the binary entry point.
- `/workspace/main.go` — same; uses `factory.CreateTick`. Unchanged.
- `/workspace/CHANGELOG.md` — `## Unreleased` already has bullets from Specs 1-5 plus the Prompt 1 `feat:` bullet. Append a new `feat:` bullet for this change.

Verified external symbols (no new deps are needed by this prompt; all are already in `go.mod`):

- `github.com/bborbe/time` (v1.27.1): `type DateTime time.Time`; `type CurrentDateTimeGetter interface { Now() DateTime }`; `func NewCurrentDateTime() CurrentDateTime`; `(c CurrentDateTime) SetNow(DateTime)`. The `SetNow` is for tests only.
- `github.com/bborbe/time/test` (`libtimetest`): `func ParseDateTime(value string) DateTime` (single arg, no ctx, no error). Takes a RFC3339 string. Verified by reading the existing 007 prompt (`clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))`) and the existing tick test suite.
- `github.com/bborbe/run` (v1.9.28): `type Func func(ctx context.Context) error`; `func CancelOnFirstFinish(ctx, fns...) error`. Already used by `main.go` to wire tick + HTTP server in parallel; not modified by this prompt.
- `github.com/bborbe/errors` (v1.5.13): `func Wrap(ctx, err, msg) error` for the `LoadLocation` failure.
- `github.com/golang/glog` (v1.2.5): `func Errorf(format, args...)` for per-task publish errors; `func V(level) Verbose` and `func (v Verbose) Infof(format, args...)` for the canonical clean-cancel log line.
- `github.com/prometheus/client_golang` (v1.23.2): unchanged from Spec 3; the metrics surface is owned by `pkg/tick/metrics.go` and is not modified.

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — `Tick` interface → `NewTick(...) (Tick, error)` → `tick` unexported struct → `(t *tick) Run(ctx)` method. The constructor signature changes (drops `ScheduleLookup`, takes `[]TaskDefinition`); the rest of the pattern is unchanged.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — `run.Func` semantics. `Run` continues to use a `time.NewTicker` + `for { select }` loop with `<-ctx.Done()` and `<-ticker.C` arms.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — non-blocking `select { case <-ctx.Done(): ...; default: }` check at the top of every per-task iteration.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` for the `LoadLocation` failure; `glog.Errorf` for per-task publish errors.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — wall-clock via injected `libtime.CurrentDateTimeGetter`; the spec permits `time.NewTicker(time.Hour)` because the ticker is a relative-duration scheduler, not a wall-clock read.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; external test package (`package tick_test`); dot-imports; `DescribeTable` / `Entry`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-prometheus-metrics-guide.md` — `prometheus.MustRegister` in `init()`, pre-initialize all known label combinations. The `pkg/tick/metrics.go` 10-series pre-init is already in place from Spec 3; this prompt does NOT change it.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter directive on the `Tick` interface and the `Metrics` interface; output filenames `tick-tick.go` and `tick-metrics.go` in `/workspace/mocks/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for new code; counterfeiter mocks (never hand-written).

Load-bearing snippets inlined for the executor's verification (read fresh before writing):

```go
// pkg/tick/tick.go (FROZEN interface, signature changes)
//counterfeiter:generate -o ../../mocks/tick-tick.go --fake-name TickTick . Tick
type Tick interface {
    Run(ctx context.Context) error
    RunOnce(ctx context.Context) error
}

// pkg/schedule/lookup.go (TYPE STAYS — handler/trigger still uses it)
type ScheduleLookup func(date Date) []TaskDefinition

// pkg/schedule/tasks_for_date.go (FUNCTION STAYS — handler/trigger still calls it)
func TasksForDate(d Date) []TaskDefinition

// pkg/schedule/inventory_export_test.go (REUSED for the new full-inventory test)
func AllDefinitionsForTest() []TaskDefinition

// libtimetest usage pattern (matches the existing 007 prompt)
clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))
```

</context>

<requirements>

## 1. Add `schedule.Inventory()` accessor

In `/workspace/pkg/schedule/inventory.go` (the file already exists with the unexported `inventory` slice — extend it), append after the existing `var inventory` declaration and its doc comment:

```go
// Inventory returns a fresh copy of the canonical recurring-task inventory.
// The tick consumes this directly — every entry is published every hour.
// Callers receive a defensive copy: mutating the returned slice does NOT
// affect the package-level inventory state. Pure function; no I/O, no
// clock, no env.
func Inventory() []TaskDefinition {
    out := make([]TaskDefinition, len(inventory))
    copy(out, inventory)
    return out
}
```

Notes:
- The signature returns `[]TaskDefinition` (a fresh slice on every call). This is the same copy pattern as `AllDefinitionsForTest` in `inventory_export_test.go` — a defensive copy keeps the package's state immutable from the outside.
- Do NOT delete the `inventory` slice variable or the `AllDefinitionsForTest` accessor — `AllDefinitionsForTest` is used by tests in `pkg/schedule` and by the new full-inventory test in `pkg/tick`.
- The function is exported. It is the public seam the tick consumes.
- The 2026 BSD copyright header is at the top of `inventory.go`; the new function goes BELOW the existing `inventory` slice and its doc comment.

## 2. Update the `NewTick` constructor signature

In `/workspace/pkg/tick/tick.go`, change the constructor signature:

```go
// NewTick builds the hourly cron loop. inventory is the full canonical
// task set; the tick publishes every entry every hour, regardless of
// the civil date. publisher is called once per entry per tick; clock is
// the wall-clock source; metrics records per-publish outcomes and the
// tick-start timestamp. Returns a wrapped error if
// time.LoadLocation("Europe/Berlin") fails at struct init.
func NewTick(
    ctx context.Context,
    inventory []schedule.TaskDefinition,
    pub publisher.Publisher,
    clock libtime.CurrentDateTimeGetter,
    metrics Metrics,
) (Tick, error) {
    berlin, err := time.LoadLocation("Europe/Berlin")
    if err != nil {
        return nil, errors.Wrap(ctx, err, "load location Europe/Berlin failed")
    }
    return &tick{
        inventory: inventory,
        publisher: pub,
        clock:     clock,
        metrics:   metrics,
        berlin:    berlin,
    }, nil
}
```

And update the unexported `tick` struct:

```go
type tick struct {
    inventory []schedule.TaskDefinition
    publisher publisher.Publisher
    clock     libtime.CurrentDateTimeGetter
    metrics   Metrics
    berlin    *time.Location
}
```

The `scheduleFn` field is removed; `inventory` replaces it. The `ScheduleLookup` type in `pkg/schedule/lookup.go` stays (the trigger handler still uses it) but the tick no longer references it.

## 3. Update the `tick(ctx)` work loop

In `/workspace/pkg/tick/tick.go`, replace the body of the unexported `tick(ctx)` method. The new body does NOT call `scheduleFn`; it iterates the `inventory` field directly. The date computation, gauge update, per-task error isolation, and per-task context-cancellation discipline are unchanged:

```go
// tick performs one full pass: read clock, convert to Berlin civil date,
// update the gauge, iterate the full inventory, and call publisher.Publish
// for each entry. Per-task errors are logged and counted but never abort
// the pass. The inventory is published in slug-ascending order (the
// underlying slice is sorted on init).
func (t *tick) tick(ctx context.Context) {
    now := t.clock.Now().Time().In(t.berlin)
    t.metrics.SetLastTickTimestamp(float64(now.Unix()))
    year, month, day := now.Date()
    date := schedule.NewDate(year, month, day)

    if len(t.inventory) == 0 {
        glog.V(2).Infof("no tasks in inventory")
        return
    }

    for _, def := range t.inventory {
        select {
        case <-ctx.Done():
            return
        default:
        }

        if err := t.publisher.Publish(ctx, def, date); err != nil {
            glog.Errorf(
                "tick: publish failed for slug %q on %04d-%02d-%02d: %v",
                def.Slug, date.Year, date.Month, date.Day, err,
            )
            t.metrics.IncPublished("error", string(def.Recurrence))
            continue
        }
        t.metrics.IncPublished("success", string(def.Recurrence))
    }
}
```

Notes:
- The `scheduleFn(date)` call is gone. The "no tasks for date" branch becomes "no tasks in inventory" — under the new contract, an empty inventory is a deployment misconfiguration, not a per-day empty result. The log line and the early return are preserved for the (unlikely) zero-length inventory case.
- The Prometheus counter labels are unchanged: `result ∈ {success, error}`, `recurrence ∈ {daily, weekly, monthly, quarterly, yearly}`. The `Recurrence` field on each `def` drives the label.
- The per-task `select` with `<-ctx.Done()` and `default` is unchanged — the context-cancellation discipline is preserved.

## 4. Update the factory wiring

In `/workspace/pkg/factory/factory.go`, update `CreateTick` to pass `schedule.Inventory()` (not `schedule.TasksForDate`) to `NewTick`:

```go
// CreateTick builds the hourly cron loop. The full inventory is published
// every tick; per-day filtering is gone (Spec 6). pub sends one
// CreateCommand per inventory entry; clock is the wall-clock source;
// metrics records per-publish outcomes and the tick-start timestamp.
//
// NewTick can fail at construction time if time.LoadLocation("Europe/Berlin")
// fails (tzdata missing from the container image). That is a container-build
// bug, not a runtime fault — CreateTick panics with a wrapped error if it
// happens, per the factory pattern's "no error return" rule. The binary
// will CrashLoopBackOff with the tzdata error visible in the pod logs.
func CreateTick(
    ctx context.Context,
    pub publisher.Publisher,
    clock libtime.CurrentDateTimeGetter,
    metrics tick.Metrics,
) tick.Tick {
    t, err := tick.NewTick(ctx, schedule.Inventory(), pub, clock, metrics)
    if err != nil {
        panic(errors.Wrap(ctx, err, "create tick failed"))
    }
    return t
}
```

`CreateTriggerHandler` is unchanged — the trigger handler still uses `schedule.TasksForDate` for per-day filtering on `?date=` manual replay:

```go
// CreateTriggerHandler returns the operator-replay HTTP handler. lookup is
// injected as the per-date task source; production wiring passes
// schedule.TasksForDate. Unchanged by Spec 6 — manual replay of a specific
// date is still useful for the operator (e.g. backfilling a missed period
// for a known date), and the trigger path is independent of the hourly
// tick's full-inventory loop.
func CreateTriggerHandler(
    publisher publisher.Publisher,
    lookup schedule.ScheduleLookup,
) http.Handler {
    return handler.NewTriggerHandler(publisher, lookup)
}
```

## 5. Update the existing tick tests

In `/workspace/pkg/tick/tick_test.go`, the constructor call sites need to change. The existing tests use a 1- to 3-entry `scheduleFn`; replace each with a 1- to 3-entry inventory literal. Specifically:

a. The top-level `BeforeEach` (lines 33-55) currently constructs a 1-entry `scheduleFn`. Replace with a 1-entry inventory literal and pass it as the second arg to `NewTick`:

```go
var (
    pub       *pubmocks.PublisherPublisher
    clock     libtime.CurrentDateTime
    metrics   *pubmocks.TickMetrics
    inventory []schedule.TaskDefinition
    tk        tick.Tick
)

BeforeEach(func() {
    pub = &pubmocks.PublisherPublisher{}
    pub.PublishReturns(nil)

    clock = libtime.NewCurrentDateTime()
    clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))

    metrics = &pubmocks.TickMetrics{}

    inventory = []schedule.TaskDefinition{
        {
            Slug:          "weekly-review",
            TitleTemplate: "t",
            Recurrence:    schedule.RecurrenceWeekly,
        },
    }

    var err error
    tk, err = tick.NewTick(context.Background(), inventory, pub, clock, metrics)
    Expect(err).NotTo(HaveOccurred())
})
```

b. The "publish-per-entry" tests (lines 94-137) currently override `scheduleFn` to a 3-entry function. Replace with a 3-entry inventory literal and re-call `NewTick`:

```go
It("calls Publish once for every entry in the inventory", func() {
    inventory = []schedule.TaskDefinition{
        {Slug: "a", TitleTemplate: "t", Recurrence: schedule.RecurrenceDaily},
        {Slug: "b", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly},
        {Slug: "c", TitleTemplate: "t", Recurrence: schedule.RecurrenceMonthly},
    }
    var err error
    tk, err = tick.NewTick(context.Background(), inventory, pub, clock, metrics)
    Expect(err).NotTo(HaveOccurred())

    // ... rest of the test (goroutine + Eventually + cancel)
})
```

c. The "Europe/Berlin date conversion" tests (lines 139-193) override `scheduleFn` to a 1-entry function and use `tk.Run(ctx)` to drive the initial tick. Replace the `scheduleFn` overrides with inventory literals; the date-capture assertions (`pub.PublishArgsForCall(0)`) are unchanged because the publisher still receives the Berlin civil `date`.

d. The "no tasks for date" test (lines 195-224) currently asserts an empty `scheduleFn` returns no publishes and no counter increments, but the gauge DOES update. Under the new contract, an empty inventory is a deployment misconfiguration; the equivalent test is "zero inventory" (which the executor can leave in place as-is, since the new `tick(ctx)` body has the same `len(t.inventory) == 0` early-return with the same metric behavior). Update the log-line assertion to match the new "no tasks in inventory" wording.

e. The "per-task error isolation" tests (lines 226-309), "context cancellation" tests (lines 311-366), "metrics gauge" test (lines 368-391), and "recurrence label coverage" table (lines 393-439) all need the same `scheduleFn` → `inventory` rename in the `BeforeEach` / per-test override blocks. The semantic assertions (call counts, error counts, label values) are unchanged.

f. The "Prometheus pre-initialization" tests (lines 442-493) are unchanged — they assert on the registered counter / gauge families and do not construct a tick.

g. The "constructor" tests (lines 57-73) are unchanged — the `NewTick` happy path and the `LoadLocation` failure-mode test do not depend on the inventory parameter.

## 6. Add the full-inventory test (the new AC #7)

In `/workspace/pkg/tick/tick_test.go`, add a new `Describe("full inventory")` block. This is the load-bearing test for the new contract — every hour, every entry in `schedule.AllDefinitionsForTest()` is published exactly once:

```go
Describe("full inventory", func() {
    It("publishes every entry in the canonical inventory regardless of the civil date", func() {
        // Derive expected count at test time (NOT a hardcoded literal).
        expected := len(schedule.AllDefinitionsForTest())
        Expect(expected).To(BeNumerically(">", 0)) // sanity: inventory is non-empty

        for _, instant := range []string{
            "2025-01-15T10:00:00Z", // Wednesday
            "2025-07-04T10:00:00Z", // Friday (different month)
            "2026-03-01T10:00:00Z", // Sunday (different year)
        } {
            clock.SetNow(libtimetest.ParseDateTime(instant))
            tk, err := tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
            Expect(err).NotTo(HaveOccurred())

            ctx, cancel := context.WithCancel(context.Background())
            done := make(chan struct{})
            go func() {
                _ = tk.Run(ctx)
                close(done)
            }()

            Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
                Should(Equal(expected))
            cancel()
            Eventually(done, "200ms", "5ms").Should(BeClosed())
        }
    })
})
```

Imports required for the new test: `schedule` (already imported), `tick` (already imported), `libtimetest` (already imported), `pubmocks` (already imported). No new imports. The `libtime` import is for the `libtime.CurrentDateTime` type, not used in this test body; the existing `clock` variable in scope has the right type.

The test runs the tick loop three times, each time on a different civil date spanning different weekdays, months, and years. The expected count is `len(schedule.AllDefinitionsForTest())`, derived at test time — if the inventory grows, the assertion tracks it without code change. This is the AC #7 contract.

## 7. Verify the `?date=` trigger path still works

The trigger handler test at `/workspace/pkg/handler/trigger_test.go` is unchanged. The handler still calls `lookup(date)` (which is `schedule.TasksForDate` in production) to filter by date. The new `schedule.Inventory()` accessor and the unchanged `schedule.TasksForDate` function coexist; the trigger path is the sole remaining consumer of `TasksForDate`. Run `go test ./pkg/handler/...` after the tick change to confirm.

## 8. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (one bullet, `feat:` prefix per `changelog-guide.md`):

```markdown
- feat: Switch `pkg/tick` to publish the full inventory every hour (drop the per-day `schedule.TasksForDate` filter); add `schedule.Inventory()` accessor; tick constructor now takes `[]schedule.TaskDefinition` instead of `schedule.ScheduleLookup`; trigger HTTP handler (`?date=` manual replay) is unchanged
```

## 9. Imports and conventions

- Every modified `.go` file retains the 2026 copyright header.
- Use `goimports-reviser` style: standard library first, then third-party (alphabetical: `github.com/bborbe/...`, `github.com/golang/...`, `github.com/onsi/...`, `github.com/prometheus/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use `github.com/bborbe/errors` for error wrapping.
- Do NOT call `time.Now()` in any non-test file.
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Do NOT touch `main.go`, `pkg/schedule/` files outside of `inventory.go`, `pkg/publisher/`, `pkg/handler/`, the Makefile, or any K8s manifest. The only `pkg/schedule` edit is appending `Inventory()` to `inventory.go` in §1.
- Do NOT add a runtime feature flag, a per-task opt-out, a tick-interval knob, a timezone knob, or any other configurability. Spec Non-goals forbid all of these.
- Do NOT regenerate the counterfeiter mock for `Tick` or `Metrics` — the existing `/workspace/mocks/tick-tick.go` and `/workspace/mocks/tick-metrics.go` cover the same interfaces (signatures are unchanged).

</requirements>

<constraints>

- The `NewTick` constructor signature is `NewTick(ctx context.Context, inventory []schedule.TaskDefinition, pub publisher.Publisher, clock libtime.CurrentDateTimeGetter, metrics Metrics) (Tick, error)`. The previous `scheduleFn schedule.ScheduleLookup` parameter is GONE.
- The `Tick` interface (`Run(ctx) error` and `RunOnce(ctx) error`) is UNCHANGED. The counterfeiter directive stays as-is.
- The `Metrics` interface and the Prometheus implementation in `pkg/tick/metrics.go` are UNCHANGED.
- `pkg/schedule/lookup.go` and `pkg/schedule/tasks_for_date.go` STAY in the tree — the trigger HTTP handler still uses both. The implementer may not delete them.
- `pkg/schedule/predicate.go` and `pkg/schedule/predicate_test.go` STAY in the tree for the same reason: `OnWeekdays`, `OnDaysOfMonth`, `OnMonthAndDay`, `EveryDay`, `OnFirstDayOfQuarter`, `OnFirstDayOfYear`, and `OnFirstDayOfMonth` are called by the inventory's `Fires` predicates. Removing them would break the inventory. The spec's "may stay as dead code" allowance is moot — the trigger handler's continued use of `TasksForDate` keeps them live.
- The `pkg/handler/trigger.go` HTTP handler is UNCHANGED. The factory's `CreateTriggerHandler` is unchanged (still wires `schedule.TasksForDate`).
- The `pkg/handler/healthz.go` handler is UNCHANGED.
- The hourly tick iterates the full inventory. The per-day filter is GONE. The `date` argument passed to `publisher.Publish` is the Berlin civil date for the current tick (used for placeholder rendering of `{{date}}` and friends), but does NOT affect the set of entries published.
- Per-task error isolation is preserved: `glog.Errorf` with slug + ISO date, Prometheus `IncPublished("error", recurrence)`, continue to the next entry.
- Context-cancellation discipline is preserved: non-blocking `select { case <-ctx.Done(): ...; default: }` at the top of every per-task iteration body.
- The Europe/Berlin timezone is loaded once at struct init via `time.LoadLocation("Europe/Berlin")`; the constructor returns a wrapped error if it fails.
- The first tick fires BEFORE the `for { select }` loop, not after. The initial tick is subject to the same per-task error isolation as every subsequent tick.
- Tests use Ginkgo v2 / Gomega; mocks are counterfeiter-generated; coverage ≥80% for new code; external test packages (`package tick_test`).
- The full-inventory test derives the expected count from `len(schedule.AllDefinitionsForTest())` at test time — NEVER a hardcoded literal.
- Do NOT add a `SIGHUP` / config-reload pathway, a tick-interval knob, a timezone knob, a per-task opt-out flag, a Prometheus histogram of publish latency, or any other YAGNI knob from the spec's Non-goals.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test -mod=mod -cover -race ./pkg/tick/...` — all Ginkgo specs green, coverage ≥80% for `pkg/tick`.
3. `go test ./pkg/schedule/...` — all schedule tests still green (canonical slugs, inventory validation, no-forbidden-imports).
4. `go test ./pkg/handler/...` — all handler tests still green; the trigger handler path with `?date=` is preserved.
5. `go test ./...` — all package tests still green.
6. `grep -E '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/tick/*.go` — must return no matches (production files only; the forbidden-imports Ginkgo test enforces this).
7. `grep -nE 'func NewTick\(' pkg/tick/tick.go` — must show the new signature `NewTick(ctx context.Context, inventory []schedule.TaskDefinition, pub publisher.Publisher, clock libtime.CurrentDateTimeGetter, metrics Metrics) (Tick, error)`. The old `scheduleFn schedule.ScheduleLookup` parameter is GONE.
8. `grep -n 'scheduleFn' pkg/tick/*.go` — must return no matches (the field and the parameter are renamed to `inventory`).
9. `grep -nE '^(type ScheduleLookup|func TasksForDate)' pkg/schedule/lookup.go pkg/schedule/tasks_for_date.go` — both still present (the trigger handler still uses them).
10. `grep -n 'schedule.Inventory' pkg/schedule/inventory.go pkg/factory/factory.go` — both must return at least one match (the new accessor and the factory's call site).
11. `ls mocks/` — must list `mocks.go`, `publisher-publisher.go`, `tick-metrics.go`, and `tick-tick.go` (the latter two unchanged; no regeneration needed).
12. Spot-check: open `pkg/tick/tick.go` and visually confirm (a) the constructor takes `inventory []schedule.TaskDefinition`, (b) the unexported `tick` struct has an `inventory` field (not `scheduleFn`), (c) the `tick(ctx)` method body iterates `t.inventory` (not `t.scheduleFn(date)`), (d) the initial `t.tick(ctx)` is still BEFORE `time.NewTicker(time.Hour)`.
13. Spot-check: open `pkg/tick/tick_test.go` and visually confirm (a) the `BeforeEach` constructs an `inventory` slice literal and passes it to `NewTick` (not a `scheduleFn` closure), (b) the new `Describe("full inventory")` block is present and uses `len(schedule.AllDefinitionsForTest())` as the expected count.

</verification>
