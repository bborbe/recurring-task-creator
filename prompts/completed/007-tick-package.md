---
status: completed
spec: [003-tick]
summary: Created pkg/tick with Metrics interface, Prometheus impl, Tick interface, NewTick constructor, hourly Run loop, Berlin civil-date conversion, per-task error isolation, and 21 Ginkgo specs achieving 85.7% coverage; make precommit exits 0
container: recurring-task-creator-mvp-exec-007-tick-package
dark-factory-version: v0.177.1
created: "2026-06-14T12:01:16Z"
queued: "2026-06-14T12:24:33Z"
started: "2026-06-14T12:24:34Z"
completed: "2026-06-14T12:35:51Z"
branch: dark-factory/tick
---

<summary>
- Adds a new `pkg/tick` Go package that owns the hourly cron loop and exposes one `Tick` interface, one `NewTick` constructor, and one unexported struct with a `Run(ctx) error` method suitable for use as a `run.Func`.
- The first tick fires synchronously on `Run` entry, before the `time.NewTicker(time.Hour)` loop starts — so the binary publishes within milliseconds of boot, not one hour later.
- Each tick reads the current instant from an injected `bborbe/time.CurrentDateTimeGetter`, converts it to a Europe/Berlin civil `schedule.Date` (with a `time.LoadLocation("Europe/Berlin")` call cached at struct init), and hands every `schedule.TasksForDate(date)` result to `publisher.Publisher.Publish`.
- Per-task publish errors are wrapped with the slug and ISO date, logged via `glog.Errorf`, counted on a Prometheus counter labeled by `result` and `recurrence`, but never abort the surrounding tick. Context cancellation between tasks or between ticks is honored within 100 ms.
- The package registers two Prometheus metrics on `init()`: a counter `recurring_tasks_published_total{result, recurrence}` pre-initialized for all 10 label combinations, and a gauge `recurring_tasks_last_tick_timestamp_seconds` updated on every tick start.
- The package does NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/bborbe/jira-task-creator/...`, does NOT call `time.Now()` directly in business logic, and does NOT walk Kafka topics / env vars / files.
- Counterfeiter mocks for `publisher.Publisher` and the local tick `Metrics` interface are generated into `mocks/`; ≥80% statement coverage; `make precommit` exits 0.

</summary>

<objective>
Create a `pkg/tick` package in `github.com/bborbe/recurring-task-creator` that owns the hourly cron loop: an initial synchronous tick on `Run` entry, then a `time.NewTicker(time.Hour)` loop calling `publisher.Publisher.Publish` for every entry returned by `schedule.TasksForDate`, with per-task error isolation, context-cancellation discipline, and Prometheus observability. Wall-clock time comes from an injected `bborbe/time.CurrentDateTimeGetter`; the Europe/Berlin location is loaded once at struct init. Counterfeiter mocks for `publisher.Publisher` and the local `Metrics` interface are generated into the project-root `mocks/` directory.
</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26.4, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before writing code:
- `/workspace/main.go` — current `application` struct fields and `application.Run(ctx, sentryClient)` flow. The tick's deps come from this `Run` (publisher, clock, metrics) — Spec 4 wires them.
- `/workspace/pkg/schedule/date.go` — `Date{Year int, Month time.Month, Day int}`, `NewDate(year, month, day) Date`, `IsZero() bool`, `weekday()` (unexported, used by `predicate.go`).
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition{Slug, TitleTemplate, BodyTemplate, Recurrence, Fires}`.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` string alias with five constants: `RecurrenceDaily`, `RecurrenceWeekly`, `RecurrenceMonthly`, `RecurrenceQuarterly`, `RecurrenceYearly` (string values `"daily" | "weekly" | "monthly" | "quarterly" | "yearly"`).
- `/workspace/pkg/schedule/tasks_for_date.go` — `func TasksForDate(d Date) []TaskDefinition` — pure, sorted by Slug, never nil, returns empty slice for the zero date.
- `/workspace/pkg/publisher/publisher.go` — `Publisher` interface (with `//counterfeiter:generate -o ../mocks/publisher-publisher.go --fake-name PublisherPublisher . Publisher` directive) and `NewPublisher(sender task.CreateCommandSender) Publisher`. The interface signature is:
  ```go
  type Publisher interface {
      Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
  }
  ```
- `/workspace/pkg/handler/sentry-alert.go` — copyright header (3 lines, year `2026`) and import-grouping convention.
- `/workspace/pkg/publisher/publisher_suite_test.go` — Ginkgo suite pattern with `//go:generate go run -mod=mod github.com/maxbrunsfeld/counterfeiter/v6 -generate` directive.
- `/workspace/mocks/mocks.go` — single-line `package mocks` file (counterfeiter target).
- `/workspace/CHANGELOG.md` — append a `feat:` bullet under `## Unreleased` for this package.
- `/workspace/go.mod` — `bborbe/time v1.27.1` (direct dep), `bborbe/run v1.9.28` (direct dep), `prometheus/client_golang v1.23.2` (direct dep), `bborbe/agent/lib v0.65.0` (transitive via publisher), `golang/glog v1.2.5` (direct dep), `onsi/ginkgo/v2 v2.29.0`, `onsi/gomega v1.41.0`. NO new deps needed.

Verified external symbols (read at `/home/node/go/pkg/mod/` via the YOLO container's Go module proxy on 2026-06-14):

`github.com/bborbe/time` (direct dep, v1.27.1) — file `current_date_time_getter.go`:
```go
type DateTime time.Time // named type — stdlib time.Time methods are NOT promoted
type CurrentDateTimeGetter interface {
    Now() DateTime
}
```
`Now()` returns `libtime.DateTime`; call `.Time()` for the stdlib `time.Time` carrier (e.g. `clock.Now().Time().In(berlin)`). `libtime.NewCurrentDateTime()` (in the main `github.com/bborbe/time` package, NOT in the `test` subpackage) returns a `CurrentDateTime` that satisfies the getter and exposes `SetNow(DateTime)` for tests. The `github.com/bborbe/time/test` subpackage (`libtimetest`) exports helpers like `libtimetest.ParseDateTime(value) libtime.DateTime` (single arg, no ctx, no error) for building deterministic `DateTime` values in tests.

`github.com/bborbe/run` (direct dep, v1.9.28) — package surface (verified by reading `func.go` and `cancel.go`):
```go
type Func func(ctx context.Context) error

// Top-level functions in the run package — verified symbol names:
func CancelOnFirstErrorWait(ctx context.Context, fns ...Func) error
func CancelOnFirstFinishWait(ctx context.Context, fns ...Func) error
func CancelOnFirstFinish(ctx context.Context, fns ...Func) error
func All(ctx context.Context, fns ...Func) error
func Sequential(ctx context.Context, fns ...Func) error
```
The spec's `run.CancelOnFirstFinish` is the **function that fires-and-cancels-but-does-not-wait** variant — see the spec's §11 "service.Run with two run.Funcs in parallel: the tick loop and the existing HTTP admin server. run.CancelOnFirstFinish semantics: if one exits, the other is cancelled." Use that name. (`service.Run` internally waits for context cancellation; the per-fn cleanup is what we need.)

`github.com/bborbe/errors` (direct dep, v1.5.13):
```go
func Wrap(ctx context.Context, err error, message string) error
func Wrapf(ctx context.Context, err error, format string, args ...interface{}) error
func Errorf(ctx context.Context, format string, args ...interface{}) error
```
Every `errors.Wrap` / `errors.Wrapf` call MUST pass `ctx` as the first arg.

`github.com/prometheus/client_golang/prometheus` (direct dep, v1.23.2) — verified at `prometheus/counter.go`, `prometheus/gauge.go`, `prometheus/registry.go`:
```go
type CounterVec struct { ... }
func NewCounterVec(opts CounterOpts, labelNames []string) *CounterVec
func (v *CounterVec) With(labels Labels) Counter
type Counter interface { Inc(); Add(float64) }

type Gauge struct { ... }
func NewGauge(opts GaugeOpts) Gauge
func (g Gauge) Set(float64)

func MustRegister(...Collector)

type CounterOpts struct { Namespace, Subsystem, Name, Help string }
type GaugeOpts   struct { Namespace, Subsystem, Name, Help string }
type Labels map[string]string
```

`github.com/golang/glog` (direct dep, v1.2.5):
```go
func Errorf(format string, args ...interface{})
func V(level Level) Verbose; (v).Infof(format, args ...interface{})
```

`time` (stdlib) — `time.LoadLocation(name string) (*time.Location, error)`, `time.Time.In(*time.Location) time.Time`, `time.NewTicker(d Duration) *Ticker`, `time.Time.Date() (year int, month time.Month, day int)`, `time.Time.Unix() int64`, `time.Hour` (constant).

`github.com/bborbe/recurring-task-creator/pkg/schedule` and `github.com/bborbe/recurring-task-creator/pkg/publisher` — both frozen, see above.

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method. For this spec: `Tick` interface → `NewTick(schedFn, publisher, clock, metrics) (Tick, error)` → `tick` unexported struct. The constructor returns the `Tick` interface type and an `error` (the spec mandates returning a wrapped error from `time.LoadLocation` failures).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-prometheus-metrics-guide.md` — `prometheus.MustRegister` in `init()`, pre-initialize all known label combinations to `0` via `WithLabelValues(...).Add(0)`. Counter names MUST end in `_total`; gauge names use `_seconds` suffix for time values. Help strings MUST be present and accurate.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — `run.Func = func(ctx context.Context) error`. The `Run` method returns this type. The for-select loop must include `<-ctx.Done()` between ticks; per-task body checks `ctx.Err()` before each publish.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — non-blocking `select { case <-ctx.Done(): ...; default: }` check at the top of every iteration.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — use `github.com/bborbe/errors` for wrapping; never `fmt.Errorf`; never `context.Background()` in business logic.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — wall-clock via injected `libtime.CurrentDateTimeGetter`; the spec permits `time.NewTicker(time.Hour)` because the ticker is a relative-duration scheduler, not a wall-clock read.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter directive on every exported interface; output filename prefixed with the source package; `//go:generate` line in the suite file.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; external test package (`package tick_test`); dot-imports.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for new code; counterfeiter mocks (never hand-written).

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/date.go (verbatim)
type Date struct {
    Year  int
    Month time.Month
    Day   int
}
func NewDate(year int, month time.Month, day int) Date
func (d Date) IsZero() bool

// pkg/schedule/recurrence.go (verbatim)
type RecurrenceKind string
const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)

// pkg/schedule/tasks_for_date.go (verbatim signature)
func TasksForDate(d Date) []TaskDefinition

// pkg/publisher/publisher.go (verbatim interface)
type Publisher interface {
    Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
}
```

</context>

<requirements>

## 1. Module check

`cd /workspace && go mod tidy` — no new direct deps needed. All deps are already in `go.mod` per the verified symbol list in `<context>`. If `go mod tidy` adds a stray indirect, leave it; the test surface is unchanged.

## 2. Create the package directory and suite

Create `/workspace/pkg/tick/` with these files. All new `.go` files start with the 2026 copyright header (3 lines, year `2026`):

```
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
```

a. `/workspace/pkg/tick/tick_suite_test.go` — Ginkgo suite entry for `package tick_test`. Mirror `/workspace/pkg/publisher/publisher_suite_test.go` exactly:

```go
package tick_test

import (
    "testing"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/onsi/gomega/format"
)

//go:generate go run -mod=mod github.com/maxbrunsfeld/counterfeiter/v6 -generate
func TestSuite(t *testing.T) {
    time.Local = time.UTC
    format.TruncatedDiff = false
    RegisterFailHandler(Fail)
    suiteConfig, reporterConfig := GinkgoConfiguration()
    suiteConfig.Timeout = 60 * time.Second
    RunSpecs(t, "Tick Suite", suiteConfig, reporterConfig)
}
```

## 3. The `Metrics` interface and the Prometheus implementation

a. `/workspace/pkg/tick/metrics.go` — define the public `Metrics` interface (with counterfeiter directive) and the Prometheus-backed implementation in a separate unexported struct.

```go
package tick

import (
    "github.com/prometheus/client_golang/prometheus"

    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// Metrics records observability events for the hourly tick loop.
//counterfeiter:generate -o ../mocks/tick-metrics.go --fake-name TickMetrics . Metrics
type Metrics interface {
    // IncPublished is called once per Publish attempt with the outcome
    // ("success" | "error") and the recurrence kind of the task.
    IncPublished(result string, recurrence string)

    // SetLastTickTimestamp is called at the start of every tick with the
    // wall-clock time of that tick start as Unix seconds (float).
    SetLastTickTimestamp(seconds float64)
}

// prometheusMetrics is the Prometheus-backed implementation of Metrics.
// It registers the counter and gauge on the default registerer in init()
// and pre-initializes the counter for all ten label combinations.
type prometheusMetrics struct {
    counter *prometheus.CounterVec
    gauge   prometheus.Gauge
}

// recurringTasksPublishedTotal counts Publish outcomes by result and recurrence.
// Pre-initialized to zero for all 10 combinations in init() so Prometheus
// scrapers see the series before the first event.
var recurringTasksPublishedTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "recurring_tasks_published_total",
        Help: "Total number of recurring-task Publish attempts grouped by outcome and recurrence kind.",
    },
    []string{"result", "recurrence"},
)

// recurringTasksLastTickTimestamp is the Unix-seconds timestamp at which
// the most recent tick started. Updated on every tick (initial + hourly).
var recurringTasksLastTickTimestamp = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "recurring_tasks_last_tick_timestamp_seconds",
        Help: "Unix timestamp (seconds) at which the most recent tick started.",
    },
)

func init() {
    prometheus.MustRegister(recurringTasksPublishedTotal, recurringTasksLastTickTimestamp)
    for _, kind := range []schedule.RecurrenceKind{
        schedule.RecurrenceDaily,
        schedule.RecurrenceWeekly,
        schedule.RecurrenceMonthly,
        schedule.RecurrenceQuarterly,
        schedule.RecurrenceYearly,
    } {
        for _, result := range []string{"success", "error"} {
            recurringTasksPublishedTotal.With(prometheus.Labels{
                "result":     result,
                "recurrence": string(kind),
            }).Add(0)
        }
    }
}

// NewPrometheusMetrics returns a Metrics backed by the package-level
// Prometheus counter and gauge. The counter is pre-initialized in init()
// so the first call into the metrics surface is just an Inc/Add.
func NewPrometheusMetrics() Metrics {
    return &prometheusMetrics{
        counter: recurringTasksPublishedTotal,
        gauge:   recurringTasksLastTickTimestamp,
    }
}

func (m *prometheusMetrics) IncPublished(result string, recurrence string) {
    m.counter.With(prometheus.Labels{
        "result":     result,
        "recurrence": recurrence,
    }).Inc()
}

func (m *prometheusMetrics) SetLastTickTimestamp(seconds float64) {
    m.gauge.Set(seconds)
}
```

Notes on the above:
- `prometheus.MustRegister(...)` is called as a statement inside `func init()` — it returns void and cannot be used in a `var _ = ...` expression.
- The 10 pre-init label combinations are `result ∈ {success, error}` × `recurrence ∈ {daily, weekly, monthly, quarterly, yearly}`. The `recurrence` strings MUST come from `string(schedule.RecurrenceKind)` (drift-resistant); ranging over the typed constants prevents hardcoded-string skew if a new kind is added.
- **`NewPrometheusMetrics() Metrics`** is the production-side wiring point — `main.go` (Prompt 2) calls it to obtain the `Metrics` value passed to `factory.CreateTick`. This constructor MUST be exported; without it Prompt 2 has nothing to call.
- The `Metrics` interface is a seam: tests use the counterfeiter fake (generated in §6), production uses `NewPrometheusMetrics()`.

## 4. The `ScheduleLookup` type and the `Tick` interface

a. `/workspace/pkg/tick/tick.go` — define the `Tick` interface, the `ScheduleLookup` function type, the `NewTick` constructor, the unexported `tick` struct, and the `Run` method.

```go
package tick

import (
    "context"
    "time"

    "github.com/bborbe/errors"
    libtime "github.com/bborbe/time"
    "github.com/golang/glog"
    "github.com/prometheus/client_golang/prometheus"

    "github.com/bborbe/recurring-task-creator/pkg/publisher"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// ScheduleLookup is the pure function that the tick invokes every hour to
// compute "what should fire today?". The interface is satisfied by
// schedule.TasksForDate; exposing it as a type here makes the constructor
// signature self-documenting and lets tests substitute a fake.
type ScheduleLookup func(date schedule.Date) []schedule.TaskDefinition

// Tick runs the hourly cron loop. Run blocks until ctx is cancelled.
//counterfeiter:generate -o ../mocks/tick-tick.go --fake-name TickTick . Tick
type Tick interface {
    Run(ctx context.Context) error
}

// NewTick builds the hourly cron loop. scheduleFn is invoked every tick
// to compute the day's task set; publisher is called once per entry;
// clock is the wall-clock source; metrics records per-publish outcomes
// and the tick-start timestamp. Returns a wrapped error if
// time.LoadLocation("Europe/Berlin") fails at struct init.
func NewTick(
    ctx context.Context,
    scheduleFn ScheduleLookup,
    pub publisher.Publisher,
    clock libtime.CurrentDateTimeGetter,
    metrics Metrics,
) (Tick, error) {
    berlin, err := time.LoadLocation("Europe/Berlin")
    if err != nil {
        return nil, errors.Wrap(ctx, err, "load location Europe/Berlin failed")
    }
    return &tick{
        scheduleFn: scheduleFn,
        publisher:  pub,
        clock:      clock,
        metrics:    metrics,
        berlin:     berlin,
    }, nil
}

type tick struct {
    scheduleFn ScheduleLookup
    publisher  publisher.Publisher
    clock      libtime.CurrentDateTimeGetter
    metrics    Metrics
    berlin     *time.Location
}

// Run performs an initial tick synchronously, then enters a 1-hour loop
// that fires on a time.NewTicker. Returns nil on clean context cancellation.
func (t *tick) Run(ctx context.Context) error {
    t.tick(ctx)

    ticker := time.NewTicker(time.Hour)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            glog.V(2).Infof("tick loop: context cancelled, exiting cleanly")
            return nil
        case <-ticker.C:
            t.tick(ctx)
        }
    }
}

// tick performs one full pass: read clock, convert to Berlin civil date,
// call scheduleFn, iterate, and call publisher.Publish for each entry.
// Per-task errors are logged and counted but never abort the pass.
func (t *tick) tick(ctx context.Context) {
    now := t.clock.Now().Time().In(t.berlin)
    year, month, day := now.Date()
    date := schedule.NewDate(year, month, day)

    t.metrics.SetLastTickTimestamp(float64(t.clock.Now().Time().Unix()))

    tasks := t.scheduleFn(date)
    if len(tasks) == 0 {
        glog.V(2).Infof("no tasks for date %04d-%02d-%02d", date.Year, date.Month, date.Day)
        return
    }

    for _, def := range tasks {
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

Notes on the above:
- The initial `t.tick(ctx)` is BEFORE the ticker starts; the first publish happens at boot, not one hour later.
- `glog.V(2).Infof("tick loop: context cancelled, exiting cleanly")` is the canonical log line — tests assert on it (level 2; glog at default level 0 filters it out).
- The per-task `select` is a non-blocking `default` check on `<-ctx.Done()` — covers the "context cancelled mid-tick" AC.
- `def.Recurrence` is a `schedule.RecurrenceKind`; `string(def.Recurrence)` is the exact label value used in `Metrics.IncPublished`.
- The gauge is updated via `t.clock.Now().Time().Unix()` (not `now.Unix()`) so the metric reflects the wall-clock time the tick started in the location-independent Unix coordinate. The `t.clock.Now().Time().In(t.berlin)` call is the date computation; the two reads of `t.clock.Now()` happen back-to-back and may differ by microseconds only — acceptable. (`.Time()` is required because `libtime.DateTime` is a named type and stdlib `time.Time` methods are NOT promoted.)
- The `tick` method is intentionally unexported and not part of the `Tick` interface. It's the internal unit-of-work that `Run` orchestrates. Tests that need to drive a single iteration call `t.Run(ctx)` and let the initial tick exercise it — see §5 for the test seam.
- The order of the per-tick work is: (1) compute date, (2) update gauge, (3) compute tasks, (4) iterate. The gauge update happens before the iteration so a panic inside `Publish` still leaves the gauge reflecting this tick (it does not, however, recover from the panic — that is `service.Main`'s job).
- The `//counterfeiter:generate` directive on the `Tick` interface is for symmetry / future testing of consumer code; the executor may strip it if the executor decides the Tick mock is unused — keep it in this spec because the spec's AC table mentions "A counterfeiter mock for `publisher.Publisher`" and "A counterfeiter mock for the tick metrics interface" but says nothing about a Tick mock; the directive on the interface is defensive (a future spec may need to mock a Tick consumer) and is harmless if never generated.

## 5. The forbidden-imports guard

In `/workspace/pkg/tick/no_forbidden_imports_test.go`:

```go
package tick_test

import (
    "os"
    "strings"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("package surface", func() {
    It("imports no forbidden packages", func() {
        forbidden := []string{
            `"net/http"`,
            `"github.com/segmentio/kafka-go"`,
            `"github.com/IBM/sarama"`,
            `"github.com/bborbe/jira-task-creator"`,
        }
        // also block any time.Now read in business logic
        forbiddenText := append(forbidden, "time.Now()")
        entries, err := os.ReadDir(".")
        Expect(err).NotTo(HaveOccurred())
        for _, e := range entries {
            name := e.Name()
            if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
                continue
            }
            data, err := os.ReadFile(name)
            Expect(err).NotTo(HaveOccurred(), name)
            text := string(data)
            for _, f := range forbiddenText {
                Expect(strings.Contains(text, f)).To(BeFalse(),
                    "%s contains forbidden token %s", name, f)
            }
            Expect(strings.Contains(text, `"github.com/bborbe/jira-task-creator/`)).
                To(BeFalse(), "%s imports a github.com/bborbe/jira-task-creator/... subpackage", name)
        }
    })
})
```

Note: the guard asserts on `time.Now()` as a forbidden token — that means the `time` package may still be imported (for `time.NewTicker`, `time.Hour`, `time.LoadLocation`), but no call to `time.Now()` may appear in any non-test file.

## 6. Generate the counterfeiter mocks

After writing all files, run `cd /workspace && go generate -mod=mod ./pkg/tick/...` to produce:

- `/workspace/mocks/publisher-publisher.go` (or whatever filename counterfeiter picks for the existing `//counterfeiter:generate` on `pkg/publisher/publisher.go`) — should already exist from the publisher spec; if not, generate it.
- `/workspace/mocks/tick-metrics.go` — fake of the `Metrics` interface, with `IncPublishedStub`, `IncPublishedCallCount`, `IncPublishedArgsForCall`, `IncPublishedReturns`, `SetLastTickTimestampStub`, `SetLastTickTimestampCallCount`, `SetLastTickTimestampArgsForCall`, `SetLastTickTimestampReturns`.
- `/workspace/mocks/tick-tick.go` — fake of the `Tick` interface, with `RunStub`, `RunCallCount`, `RunArgsForCall`. (Only if the executor kept the `//counterfeiter:generate` directive on the `Tick` interface.)

After generation, verify each generated file begins with the 2026 copyright header. If any generator output is missing the header, add it (counterfeiter 6.12.2 typically preserves surrounding file headers but does not invent them).

## 7. The unit tests

In `/workspace/pkg/tick/tick_test.go` (external `package tick_test`). All Ginkgo v2 / Gomega. The setup uses:

```go
import (
    "context"
    "errors"
    "time"

    libtime "github.com/bborbe/time"
    libtimetest "github.com/bborbe/time/test"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    pubmocks "github.com/bborbe/recurring-task-creator/mocks"
    "github.com/bborbe/recurring-task-creator/pkg/publisher"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
    "github.com/bborbe/recurring-task-creator/pkg/tick"
)
```

A single `Describe("Tick", func() { ... })` block. Use the following fakes (constructed in `BeforeEach`):

```go
var (
    pub        *pubmocks.PublisherPublisher
    clock      libtime.CurrentDateTime
    metrics    *pubmocks.TickMetrics
    scheduleFn tick.ScheduleLookup
    t          tick.Tick
)

BeforeEach(func() {
    pub = &pubmocks.PublisherPublisher{}
    pub.PublishReturns(nil)

    clock = libtime.NewCurrentDateTime()
    clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))

    metrics = &pubmocks.TickMetrics{}

    scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
        return []schedule.TaskDefinition{
            {Slug: "weekly-review", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly},
        }
    }

    t, _ = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
})
```

The set of `It` blocks (one per acceptance criterion / failure-mode row):

a. **Constructor returns a Tick on the happy path.** `Expect(t).NotTo(BeNil())`.

b. **Constructor returns a wrapped error when LoadLocation fails.** Stub the location-load failure path. The cleanest seam: a Ginkgo test that captures the error path by direct test of `NewTick` with a non-existent timezone — but the location is hard-coded to `"Europe/Berlin"`. The simpler test is the **shape check**: read the source for `time.LoadLocation("Europe/Berlin")` and confirm the wrap is `errors.Wrap(ctx, err, "load location Europe/Berlin failed")` (or equivalent). Per the AC, evidence shape: either a Ginkgo test or `grep -n 'Europe/Berlin' pkg/tick/*.go` returning the load call and the error-wrap site. Provide a Ginkgo test that calls `time.LoadLocation("NoSuch/Zone")` to confirm the error from `LoadLocation` itself, plus a `grep`-style evidence block:
   ```go
   It("returns a wrapped error when time.LoadLocation fails (verified via grep + stdlib failure mode)", func() {
       _, err := time.LoadLocation("NoSuch/Zone")
       Expect(err).To(HaveOccurred())
   })
   ```
   The actual `NewTick` cannot easily exercise the error path without exposing `LoadLocation` for stubbing. Document this in a code comment in the test file: "NewTick's Europe/Berlin load-failure path is exercised by code review (load call + error wrap) and by the stdlib LoadLocation failure mode confirmed above; in-process stubbing of the location is intentionally omitted to avoid leaking test seams into production code."

c. **Initial tick fires before the for-select loop.** Run the tick in a goroutine and assert:
   ```go
   ctx, cancel := context.WithCancel(context.Background())
   defer cancel()
   go func() { _ = t.Run(ctx) }()
   Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").Should(Equal(1))
   cancel()
   ```

d. **Each tick calls Publish once for every entry returned by scheduleFn.** After (c), `Expect(pub.PublishCallCount()).To(Equal(1))` (since the scheduleFn above returns 1 entry). Add a second test where `scheduleFn` returns 3 entries and assert `PublishCallCount() == 3` after one tick.

e. **DST date conversion — winter boundary.** Set the clock to `2025-01-04T23:30:00Z` (winter, UTC). Europe/Berlin at that instant is `2025-01-05T00:30:00+01:00` → civil date `2025-01-05`. Capture the `def` passed to `Publish` via `pub.PublishArgsForCall(0)` and assert `date == schedule.NewDate(2025, time.January, 5)`. (`pub.PublishArgsForCall(0)` returns `(ctx, def, date)`; use the `date` return value.)

f. **DST date conversion — summer boundary.** Set the clock to `2025-07-15T23:30:00Z` (summer, UTC). Europe/Berlin at that instant is `2025-07-16T01:30:00+02:00` → civil date `2025-07-16`. Assert `date == schedule.NewDate(2025, time.July, 16)`.

g. **No-tasks-today — gauge updates, no Publish, no log error.** Override `scheduleFn` to return an empty slice. After one tick, `Expect(pub.PublishCallCount()).To(Equal(0))`, `Expect(metrics.IncPublishedCallCount()).To(Equal(0))`, `Expect(metrics.SetLastTickTimestampCallCount()).To(Equal(1))`.

h. **Per-task error isolation.** Set `pub.PublishReturnsOnCall(0, errors.New("kafka down"))` and `pub.PublishReturnsOnCall(1, nil)` for a 3-entry tick. After one tick, `Expect(pub.PublishCallCount()).To(Equal(3))`, `Expect(metrics.IncPublishedCallCount()).To(Equal(3))`, with one `IncPublished("error", "weekly")` and two `IncPublished("success", "weekly")` calls (use `metrics.IncPublishedArgsForCall(i)` to assert).

i. **Error path increments counter with the task's recurrence kind.** `pub.PublishReturns(errors.New("boom"))` for a tick with two entries: one with `Recurrence: schedule.RecurrenceDaily` and one with `Recurrence: schedule.RecurrenceMonthly`. After the tick, assert `metrics.IncPublishedCallCount() == 2` and the two `IncPublished` calls recorded `("error", "daily")` and `("error", "monthly")`.

j. **Context cancelled between per-task Publish calls — loop exits early.** Use a 5-entry scheduleFn and a `pub.PublishStub` that cancels the context on the first call:
   ```go
   ctx, cancel := context.WithCancel(context.Background())
   defer cancel()
   pub.PublishStub = func(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error {
       cancel()
       return nil
   }
   ```
   Run `t.Run(ctx)` synchronously (or in a goroutine with `Eventually`). Assert `Expect(pub.PublishCallCount()).To(Equal(1))`. Then assert `Expect(metrics.SetLastTickTimestampCallCount()).To(Equal(1))` (gauge updated for the initial tick before the loop's body returned).

k. **Context cancelled between ticks — Run returns nil cleanly.** Cancel the context after 50 ms while `Run` is blocked on the ticker. Assert `Run` returns within 100 ms with `Expect(err).NotTo(HaveOccurred())` and `Expect(pub.PublishCallCount() >= 1)`. (The initial tick fires synchronously; cancellation during the ticker wait causes `Run` to return via `<-ctx.Done()`.)

l. **Gauge value equals the clock's Unix seconds.** After one tick, assert `metrics.SetLastTickTimestampArgsForCall(0)` is `float64(clock.Now().Time().Unix())` (within 1 second — `time.Unix()` rounds).

m. **Prometheus counter pre-initialization — gather before any tick.** Capture the default registerer's output via `prometheus.DefaultGatherer.Gather()` and find the `recurring_tasks_published_total` metric family. Assert the family has exactly 10 series (one per label combination), all with value 0. (Implementation hint: iterate `m.GetMetric()`, count, assert label sets. Use `prometheus.GathererFunc` to call the gatherer inside a `BeforeEach`.)

n. **Per-task error log line contains the slug and ISO date.** Set `pub.PublishReturns(errors.New("kafka down"))`. Run one tick with a 1-entry scheduleFn (slug = `"weekly-review"`, date `2025-01-04`). The `glog.Errorf` line is fired (covered by the `glog` package's own tests for log emission; we do NOT capture glog output in unit tests). The behavior that IS testable: the call to `Publish` happens once with the expected `(def, date)`. Verify the wrap path in a separate "publisher wraps error with slug and ISO date" test, OR rely on the publisher's own test (in `pkg/publisher/publisher_test.go`) for the wrap contract.

o. **Recurrence table for the counter label.** `DescribeTable` over the five `RecurrenceKind` values; for each, build a 1-entry scheduleFn with that recurrence, run a tick, assert `metrics.IncPublished("success", string(kind))` was called once.

## 8. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased`:

```markdown
- feat: Add `pkg/tick` package that runs the hourly cron loop (initial tick at boot, then 1-hour ticker) calling `publisher.Publish` for every `schedule.TasksForDate` entry; per-task error isolation via glog + Prometheus counter; gauge for last-tick timestamp; `Europe/Berlin` civil date from injected clock
```

The bullet must use the `feat:` prefix (minor bump per `changelog-guide.md`).

## 9. Imports and conventions

- Every new `.go` file starts with the 2026 copyright header.
- Use `goimports-reviser` style: standard library first, then third-party (alphabetical: `github.com/bborbe/...`, `github.com/golang/...`, `github.com/onsi/...`, `github.com/prometheus/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use `github.com/bborbe/errors` for wrapping; `errors.Wrap(ctx, err, "load location Europe/Berlin failed")` for the constructor's load failure.
- Do NOT call `time.Now()` in any non-test file.
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Do NOT touch `main.go`, `pkg/factory/`, `pkg/schedule/`, `pkg/publisher/`, `pkg/handler/`, `pkg/mathutil/`, `Makefile`, or any K8s manifest.
- Do NOT add a runtime feature flag, a per-task opt-out, a tick-interval knob, a timezone knob, or any other configurability. Spec Non-goals forbid all of these.

## 10. Verify and wire-up

After all files are written and mocks generated:

1. Run `cd /workspace && go build ./pkg/tick/...` — must compile.
2. Run `cd /workspace && go test -mod=mod -cover -race ./pkg/tick/...` — all Ginkgo specs green, coverage ≥80%.
3. Run `cd /workspace && go test ./...` — all package tests still green.
4. Run `cd /workspace && make precommit` — must exit 0.
5. Run `cd /workspace && grep -E '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/tick/*.go` — must return no matches (production files only; the forbidden-imports Ginkgo test enforces this).
6. Run `cd /workspace && ls mocks/` — must list `mocks.go`, plus `tick-metrics.go` and (if generated) `tick-tick.go` and `publisher-publisher.go`.
7. Spot-check: open `pkg/tick/tick.go` and visually confirm the constructor signature, the initial-tick-before-loop structure, and the Europe/Berlin location load at struct init.

If `make precommit` flags an unused-variable, missing-license-header, or import-grouping issue, fix it locally; do NOT broaden the scope.

</requirements>

<constraints>
- The package MUST NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, or any `github.com/bborbe/jira-task-creator/...` package.
- The package MUST NOT call `time.Now()` directly anywhere (production files only). Wall-clock time comes from the injected `bborbe/time.CurrentDateTimeGetter`. The `time.NewTicker(time.Hour)` call is permitted.
- The package MUST NOT walk Kafka topics, KV stores, files, or env vars. Inputs are constructor-injected; outputs are the publisher and the metrics.
- The package MUST follow the Interface → Constructor → Struct → Method pattern: `Tick` (interface) → `NewTick(...) (Tick, error)` → `tick` (private struct) → `(t *tick) Run(ctx context.Context) error`.
- Context-cancellation discipline: every blocking `select` includes `<-ctx.Done()`; every per-task iteration body checks `ctx.Err()` before doing meaningful work.
- Error wrapping: use `github.com/bborbe/errors` for the constructor's `LoadLocation` failure (`errors.Wrap(ctx, err, "load location Europe/Berlin failed")`); `glog.Errorf` for per-task publish errors with the slug and ISO date in the message.
- Prometheus metrics: register in `init()` via `prometheus.MustRegister`; pre-initialize the counter for all 10 label combinations (`result ∈ {success, error}` × `recurrence ∈ {daily, weekly, monthly, quarterly, yearly}`) to `0`; gauge is registered once with no labels.
- The Europe/Berlin timezone MUST be loaded via `time.LoadLocation("Europe/Berlin")` ONCE at struct init (cached on the struct), NOT per-tick. If the load fails, the constructor returns a wrapped error.
- The first tick fires BEFORE the `for { select }` loop, not after. The initial tick is subject to the same per-task error isolation as every subsequent tick.
- Tests use Ginkgo v2 / Gomega; mocks are counterfeiter-generated; coverage ≥80% for new code; external test packages (`package tick_test`).
- Do NOT add a `SIGHUP` / config-reload pathway, a tick-interval knob, a timezone knob, a per-task opt-out flag, a Prometheus histogram of publish latency, or any other YAGNI knob from the spec's Non-goals.
- Do NOT touch `main.go`, `pkg/factory/`, `pkg/handler/`, `pkg/mathutil/`, `pkg/schedule/`, `pkg/publisher/`, the Makefile, or any K8s manifest. Spec 4 wires the factory + main.go; Spec 1 + 2 own their packages.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test -mod=mod -cover -race ./pkg/tick/...` — all specs green, coverage ≥80% for `pkg/tick`.
3. `grep -E '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/tick/*.go` — must return no matches (excluding `*_test.go` files; the forbidden-imports Ginkgo test enforces this for production files).
4. `grep -E '^(type Tick|func NewTick|type tick )' pkg/tick/*.go` — must list exactly one `Tick` interface declaration, one `NewTick` constructor, and one unexported `tick` struct.
5. `grep -nE 'func NewTick\(' pkg/tick/*.go` — must show a signature matching `func NewTick(ctx context.Context, <ScheduleLookup>, publisher.Publisher, libtime.CurrentDateTimeGetter, Metrics) (Tick, error)`.
6. `ls mocks/` — must list `mocks.go`, plus `tick-metrics.go` and (if generated) `tick-tick.go` and `publisher-publisher.go`.
7. Spot-check: open `pkg/tick/tick.go` and visually confirm the initial `t.tick(ctx)` is BEFORE `time.NewTicker(time.Hour)` and the `for { select }` loop.
8. Spot-check: open `pkg/tick/metrics.go` and visually confirm the 10-element pre-init loop covers `result ∈ {success, error}` × `recurrence ∈ {daily, weekly, monthly, quarterly, yearly}`.

</verification>
