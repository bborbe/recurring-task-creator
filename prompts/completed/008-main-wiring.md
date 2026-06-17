---
status: completed
spec: [003-tick]
summary: Wired main.go for tick loop + HTTP server via run.CancelOnFirstFinish; trimmed HTTP router to 4 endpoints; deleted pkg/handler/ and pkg/mathutil/; added CreateTick factory + tests; make precommit exits 0
container: recurring-task-creator-mvp-exec-008-main-wiring
dark-factory-version: v0.177.1
created: "2026-06-14T12:01:16Z"
queued: "2026-06-14T12:24:33Z"
started: "2026-06-14T12:35:51Z"
completed: "2026-06-14T12:45:50Z"
branch: dark-factory/tick
---

<summary>
- Adds `pkg/factory.CreateTick(scheduleFn, pub, clock, metrics) tick.Tick` — a one-line pass-through that wraps `pkg/tick.NewTick` for the binary's composition layer. A factory test injects fakes and confirms the constructor returns a non-nil `tick.Tick`.
- Rewrites `main.go` so the binary's wire graph is: env config → `service.Main` → sentry init → Kafka `SyncProducer` (existing) → `task.CreateCommandSender` via `pkg/factory.CreatePublisher` → `tick.Tick` via the new `pkg/factory.CreateTick` → `service.Run` with two `run.Func`s in parallel (the tick loop and the HTTP admin server) using `run.CancelOnFirstFinish` semantics.
- Trims the HTTP admin router to exactly `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}` (per the spec — `/gc`, `/resetdb`, `/resetbucket/{BucketName}`, `/testloglevel`, `/sentryalert` are removed). Removes `BATCH_SIZE` and `DATADIR` env vars and the `libboltkv.OpenDir` call from `main.go`.
- Deletes `pkg/handler/` and `pkg/mathutil/` directories in their entirety.
- Rewrites `pkg/factory/factory.go` to keep only `CreatePublisher` (from Spec 2) and the new `CreateTick`; the `CreateTestLoglevelHandler` and `CreateSentryAlertHandler` factories and their `pkg/handler` imports are removed. The `pkg/factory/factory_suite_test.go` Ginkgo bootstrap stays.
- `make precommit` exits 0 after the change; no per-task opt-out, no tick-interval knob, no per-task Prometheus histogram, no SIGHUP reload pathway.

</summary>

<objective>
Wire the binary's startup so it boots the tick loop in parallel with the HTTP admin server, delete the inherited go-skeleton example packages (`pkg/handler/`, `pkg/mathutil/`) and the `CreateTestLoglevelHandler` / `CreateSentryAlertHandler` factories in `pkg/factory/factory.go`, and add the `CreateTick` factory constructor alongside `CreatePublisher`. The HTTP router trims to four endpoints; the boltkv `DataDir` and `BATCH_SIZE` env vars and the `libboltkv.OpenDir` call disappear from `main.go`. `make precommit` must exit 0.
</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26.4, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before writing code:
- `/workspace/main.go` — current `application` struct (with `SentryDSN`, `SentryProxy`, `Listen`, `KafkaBrokers`, `BatchSize`, `DataDir`, `BuildGitVersion`, `BuildGitCommit`, `BuildDate` fields) and `application.Run(ctx, sentryClient)` flow. The `Run` body currently opens a `libboltkv.OpenDir(a.DataDir)` and calls `service.Run` with a single `run.Func` (the HTTP server). The HTTP router currently has 9 paths: `/healthz`, `/readiness`, `/metrics`, `/resetdb`, `/resetbucket/{BucketName}`, `/setloglevel/{level}`, `/gc`, `/testloglevel`, `/sentryalert`.
- `/workspace/pkg/factory/factory.go` — current shape: `CreateTestLoglevelHandler`, `CreateSentryAlertHandler`, `CreatePublisher` (Spec 2). After this prompt: keep only `CreatePublisher` and the new `CreateTick`.
- `/workspace/pkg/factory/factory_suite_test.go` — Ginkgo suite bootstrap; PRESERVE as-is.
- `/workspace/pkg/factory/factory_test.go` — currently has a single `Describe("CreatePublisher", ...)` block. After this prompt: add a new `Describe("CreateTick", ...)` block. The existing `CreatePublisher` block stays.
- `/workspace/pkg/handler/sentry-alert.go` and `/workspace/pkg/handler/test-loglevel.go` — DELETE both files; the entire `pkg/handler/` directory is removed.
- `/workspace/pkg/handler/handler_suite_test.go`, `sentry-alert_test.go`, `test-loglevel_test.go` — DELETE; the entire `pkg/handler/` directory is removed.
- `/workspace/pkg/mathutil/clamp.go`, `clamp_test.go`, `mathutil_suite_test.go` — DELETE; the entire `pkg/mathutil/` directory is removed.
- `/workspace/pkg/publisher/publisher.go` — `Publisher` interface; `NewPublisher(sender task.CreateCommandSender) Publisher`. UNCHANGED (frozen by Spec 2).
- `/workspace/pkg/tick/tick.go` — `Tick` interface; `NewTick(scheduleFn, pub, clock, metrics) (Tick, error)`. The constructor returns `error` (the `Europe/Berlin` load can fail), but `CreateTick` MUST NOT return `error` per the factory pattern — call `NewTick` and panic-or-wrap on failure is NOT acceptable. The factory pattern disallows returning `error` from `Create*` functions. So either: (a) the factory panics on the `NewTick` failure (acceptable because the spec says "impossible in standard Go runtimes" — the tzdata failure mode is a container-build bug, not a runtime fault), OR (b) the factory does not perform the `NewTick` call and lets the caller (i.e. `main.go`) handle it. Pick (a): `CreateTick` calls `NewTick` and `panic`s with the wrapped error if it fails. Document this in the `CreateTick` doc-comment.
- `/workspace/pkg/schedule/tasks_for_date.go` — `schedule.TasksForDate(d Date) []TaskDefinition`. This is the `ScheduleLookup` the factory's `CreateTick` will inject.
- `/workspace/mocks/mocks.go` — single-line `package mocks` file.
- `/workspace/CHANGELOG.md` — append a `feat:` bullet under `## Unreleased` for this work.
- `/workspace/go.mod` — no new direct deps. `bborbe/time v1.27.1`, `bborbe/run v1.9.28`, `bborbe/agent/lib v0.65.0`, `github.com/prometheus/client_golang v1.23.2`, `bborbe/kafka v1.23.2`, `github.com/IBM/sarama` (indirect), `bborbe/service v1.10.1`, `bborbe/boltkv` (direct dep, no longer imported by `main.go` after this spec — `go mod tidy` will move it to indirect), `bborbe/kv` (direct dep, no longer imported by `main.go` — same).

Verified external symbols (read at `/home/node/go/pkg/mod/` via the YOLO container's Go module proxy on 2026-06-14):

`github.com/bborbe/service` (direct dep, v1.10.1) — verified `service.Run` signature in `service.go`:
```go
// service.Run is verified to exist with this signature (cancellable concurrent runner).
func Run(ctx context.Context, fns ...run.Func) error
```
`run.Func = func(ctx context.Context) error` (from `bborbe/run` v1.9.28).

`github.com/bborbe/kafka` (direct dep, v1.23.2) — verified surface for the existing `main.go`:
```go
func NewSyncProducerWithName(ctx context.Context, brokers []string, name string) (SyncProducer, error)
func ParseBrokersFromString(s string) []string
func CreateSaramaClient(ctx context.Context, brokers []string) (sarama.Client, error)
type SyncProducer interface { Close() error; ... }
```
The `NewSyncProducerWithName` returns the `SyncProducer` interface that wraps `cdb.CommandObjectSender` (see below).

`github.com/bborbe/agent/lib/command/task` (transitive, v0.65.0) — verified at the publisher's `<context>`:
```go
func NewCreateCommandSender(commandObjectSender cdb.CommandObjectSender) CreateCommandSender
type CreateCommandSender interface {
    SendCommand(ctx context.Context, cmd CreateCommand) error
}
```

`github.com/bborbe/cqrs/cdb` — verified signature (canonical wiring lives at `~/workspaces/agent/task/executor/pkg/factory/factory.go:149`):
```go
func NewCommandObjectSender(
    syncProducer libkafka.SyncProducer,
    branch base.Branch,
    logSamplerFactory log.SamplerFactory,
) CommandObjectSender
```
Three args — NO topic name. Topic is derived from branch + schema by the CQRS layer.

`github.com/bborbe/cqrs/base` — `type Branch string`. Use `base.Branch("master")` (the default tracking branch for the bborbe CQRS stack).

`github.com/bborbe/log` — `log.DefaultSamplerFactory` is the canonical `log.SamplerFactory` value used by the agent's executor (`cdb.NewCommandObjectSender(syncProducer, branch, log.DefaultSamplerFactory)`).

The publisher wire in `main.go` is:
```go
sender := task.NewCreateCommandSender(cdb.NewCommandObjectSender(syncProducer, base.Branch("master"), log.DefaultSamplerFactory))
```

`github.com/bborbe/run` (direct dep, v1.9.28) — verified:
```go
func CancelOnFirstFinish(ctx context.Context, fns ...Func) error
type Func func(ctx context.Context) error
```
The spec's "run.CancelOnFirstFinish semantics: if one exits, the other is cancelled" matches this function's contract: when one of the `fns` returns, the others are cancelled and `CancelOnFirstFinish` returns. The clean-shutdown path is: SIGTERM cancels the parent `ctx`; both `fns` (HTTP server + tick loop) return; `CancelOnFirstFinish` returns nil. The error path is: HTTP server errors out → `run.Func` returns error → `CancelOnFirstFinish` cancels the tick → both return → first error is propagated.

`github.com/bborbe/time` (direct dep, v1.27.1) — verified:
```go
type DateTime time.Time // named type — stdlib time.Time methods NOT promoted
func NewCurrentDateTime() CurrentDateTime
type CurrentDateTime interface { CurrentDateTimeGetter; SetNow(DateTime) }
type CurrentDateTimeGetter interface { Now() DateTime }
```
`NewCurrentDateTime` is in the main package, NOT in the `test` subpackage.

`github.com/golang/glog` (direct dep, v1.2.5) — `glog.V(2).Infof(...)` for the existing HTTP-server startup log.

`github.com/gorilla/mux` (direct dep, v1.8.1) — `mux.NewRouter()`, `router.Path(...)`.

`github.com/prometheus/client_golang/prometheus/promhttp` (direct dep, v1.23.2) — `promhttp.Handler()`.

`github.com/bborbe/recurring-task-creator/pkg/tick` (NEW from prompt 1) — verified in that prompt's `<context>`:
```go
type Tick interface { Run(ctx context.Context) error }
type ScheduleLookup func(date schedule.Date) []schedule.TaskDefinition
```

`github.com/bborbe/recurring-task-creator/pkg/publisher` — frozen by Spec 2:
```go
type Publisher interface { Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error }
```

`github.com/bborbe/recurring-task-creator/pkg/schedule` — frozen by Spec 1:
```go
func TasksForDate(d Date) []TaskDefinition
```

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-k8s-binary-conventions.md` — `application` struct, `service.Main(ctx, app, &SentryDSN, &SentryProxy)` entry, `/healthz + /readiness + /metrics` HTTP triple, `run.CancelOnFirstFinish(ctx, work..., httpServer)` for the wire-up, secret fields carry `display:"length"`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `Create*` prefix in `pkg/factory`, zero business logic, returns interface type, never returns `error`. The `CreateTick` factory in this spec is unusual because `pkg/tick.NewTick` returns `error` (LoadLocation failure); the factory calls `NewTick` and panics on error. Document the rationale in the doc-comment.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-http-service-guide.md` — the canonical admin endpoints are five: `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`, `/gc`. The SPEC for this work lists only the first four as kept; `/gc` is in the explicit removal list. Follow the SPEC (which is the source of truth), not the doc. The audit will read `grep -nE 'router\.Path' main.go` and confirm exactly four matches.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — use `github.com/bborbe/errors` for the `NewTick` panic wrap; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method. The factory's `CreateTick` is a one-line pass-through.

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/publisher/publisher.go (verbatim — frozen)
type Publisher interface {
    Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
}
func NewPublisher(sender task.CreateCommandSender) Publisher

// pkg/tick/tick.go (verbatim — from prompt 1)
type Tick interface {
    Run(ctx context.Context) error
}
type ScheduleLookup func(date schedule.Date) []schedule.TaskDefinition
func NewTick(
    scheduleFn ScheduleLookup,
    pub publisher.Publisher,
    clock libtime.CurrentDateTimeGetter,
    metrics Metrics,
) (Tick, error)
```

</context>

<requirements>

## 1. The factory — `CreateTick`

Rewrite `/workspace/pkg/factory/factory.go` to keep only `CreatePublisher` (from Spec 2) and add `CreateTick`. Drop `CreateTestLoglevelHandler`, `CreateSentryAlertHandler`, and the `pkg/handler` import. Keep the 2026 copyright header.

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
    "context"

    "github.com/bborbe/agent/lib/command/task"
    "github.com/bborbe/errors"
    libtime "github.com/bborbe/time"

    "github.com/bborbe/recurring-task-creator/pkg/publisher"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
    "github.com/bborbe/recurring-task-creator/pkg/tick"
)

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. Pure plumbing: no business logic.
func CreatePublisher(sender task.CreateCommandSender) publisher.Publisher {
    return publisher.NewPublisher(sender)
}

// CreateTick builds the hourly cron loop. schedule.TasksForDate is
// injected as the lookup so the caller never imports the inventory
// directly. pub sends one CreateCommand per task; clock is the wall-clock
// source; metrics records per-publish outcomes and the tick-start
// timestamp.
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
    t, err := tick.NewTick(ctx, schedule.TasksForDate, pub, clock, metrics)
    if err != nil {
        panic(errors.Wrap(ctx, err, "create tick failed"))
    }
    return t
}
```

Imports required:
- `"context"` for the `CreateTick(ctx, ...)` signature.
- `"github.com/bborbe/errors"`.
- `"github.com/bborbe/agent/lib/command/task"`.
- `libtime "github.com/bborbe/time"`.
- The three internal `pkg/` paths.

**YAGNI note:** The factory uses `schedule.TasksForDate` directly. Do NOT add a parameter for the schedule function — the spec explicitly says `schedule.TasksForDate` is the only lookup the tick needs.

## 2. The factory test

Add a `Describe("CreateTick", ...)` block to `/workspace/pkg/factory/factory_test.go`. The existing `Describe("CreatePublisher", ...)` block stays unchanged.

```go
var _ = Describe("CreateTick", func() {
    var (
        pubFake     *projmocks.PublisherPublisher
        clock       libtime.CurrentDateTime
        metricsFake *projmocks.TickMetrics
        t           tick.Tick
    )
    BeforeEach(func() {
        pubFake = &projmocks.PublisherPublisher{}
        clock = libtime.NewCurrentDateTime()
        metricsFake = &projmocks.TickMetrics{}
        t = factory.CreateTick(context.Background(), pubFake, clock, metricsFake)
    })
    It("returns a Tick that wires the publisher, clock, and metrics", func() {
        Expect(t).NotTo(BeNil())
    })
    It("does not panic on the happy path (Europe/Berlin loadable)", func() {
        // Implicit: if CreateTick panicked, the BeforeEach would have
        // failed this test. The presence of a non-nil Tick IS the proof.
        Expect(t).NotTo(BeNil())
    })
})
```

Required imports (in addition to the existing `factory_test.go` imports):
- `libtime "github.com/bborbe/time"`
- `projmocks "github.com/bborbe/recurring-task-creator/mocks"` — ONE alias for the project mocks package. Reference both fake types (`projmocks.PublisherPublisher`, `projmocks.TickMetrics`) through this single alias. Do NOT add a second alias for the same import path — Go will reject duplicate aliases for the same package.
- `"github.com/bborbe/recurring-task-creator/pkg/tick"`.

Note: the existing `factory_test.go` on disk uses `taskmocks "github.com/bborbe/agent/lib/command/task/mocks"` (a DIFFERENT package). Add `projmocks` for `github.com/bborbe/recurring-task-creator/mocks` as a new, separate import — there is no collision.

The publisher fake type is `PublisherPublisher` (from Prompt 2's publisher spec). The tick metrics fake type is `TickMetrics` (from Prompt 1 §3 counterfeiter directive). Both live under `github.com/bborbe/recurring-task-creator/mocks`.

## 3. `main.go` — full rewrite

Rewrite `/workspace/main.go` so the wire graph is the spec's contract. The 2026 copyright header stays. Imports drop `libboltkv`, `libkv`, `libmetrics` (replaced by the spec's `libmetrics.NewBuildInfoMetrics().SetBuildInfo(...)` if still needed — see below), and `pkg/handler` (no longer exists).

Final `main.go` shape (sketch; the executor writes the file end-to-end):

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "context"
    "os"
    "time"

    "github.com/bborbe/agent/lib/command/task"
    cqrsbase "github.com/bborbe/cqrs/base"
    cdb "github.com/bborbe/cqrs/cdb"
    "github.com/bborbe/errors"
    libhttp "github.com/bborbe/http"
    libkafka "github.com/bborbe/kafka"
    liblog "github.com/bborbe/log"
    libmetrics "github.com/bborbe/metrics"
    "github.com/bborbe/run"
    libsentry "github.com/bborbe/sentry"
    "github.com/bborbe/service"
    libtime "github.com/bborbe/time"
    "github.com/golang/glog"
    "github.com/gorilla/mux"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/bborbe/recurring-task-creator/pkg/factory"
    "github.com/bborbe/recurring-task-creator/pkg/tick"
)

const serviceName = "recurring-task-creator"

func main() {
    app := &application{}
    os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
    SentryDSN       string            `required:"true"  arg:"sentry-dsn"        env:"SENTRY_DSN"        usage:"SentryDSN"                             display:"length"`
    SentryProxy     string            `required:"false" arg:"sentry-proxy"      env:"SENTRY_PROXY"      usage:"Sentry Proxy"`
    Listen          string            `required:"true"  arg:"listen"            env:"LISTEN"            usage:"address to listen to"`
    KafkaBrokers    string            `required:"true"  arg:"kafka-brokers"     env:"KAFKA_BROKERS"     usage:"Comma separated list of Kafka brokers"`
    BuildGitVersion string            `required:"false" arg:"build-git-version" env:"BUILD_GIT_VERSION" usage:"Build Git version"                       default:"dev"`
    BuildGitCommit  string            `required:"false" arg:"build-git-commit"  env:"BUILD_GIT_COMMIT"  usage:"Build Git commit hash"                   default:"none"`
    BuildDate       *libtime.DateTime `required:"false" arg:"build-date"        env:"BUILD_DATE"        usage:"Build timestamp (RFC3339)"`
}

func (a *application) Run(ctx context.Context, _ libsentry.Client) error {
    libmetrics.NewBuildInfoMetrics().SetBuildInfo(a.BuildGitVersion, a.BuildGitCommit, a.BuildDate)

    saramaClient, err := libkafka.CreateSaramaClient(
        ctx,
        libkafka.ParseBrokersFromString(a.KafkaBrokers),
    )
    if err != nil {
        return errors.Wrap(ctx, err, "create sarama client failed")
    }
    defer saramaClient.Close()

    syncProducer, err := libkafka.NewSyncProducerWithName(
        ctx,
        libkafka.ParseBrokersFromString(a.KafkaBrokers),
        serviceName,
    )
    if err != nil {
        return errors.Wrap(ctx, err, "create sync producer failed")
    }
    defer syncProducer.Close()

    sender := task.NewCreateCommandSender(cdb.NewCommandObjectSender(
        syncProducer,
        cqrsbase.Branch("master"),
        liblog.DefaultSamplerFactory,
    ))
    pub := factory.CreatePublisher(sender)

    clock := libtime.NewCurrentDateTime()
    metrics := tick.NewPrometheusMetrics()
    tickLoop := factory.CreateTick(ctx, pub, clock, metrics)

    return run.CancelOnFirstFinish(
        ctx,
        a.runHTTPServer(),
        tickLoop.Run,
    )
}

func (a *application) runHTTPServer() run.Func {
    return func(ctx context.Context) error {
        ctx, cancel := context.WithCancel(ctx)
        defer cancel()

        router := mux.NewRouter()
        router.Path("/healthz").Handler(libhttp.NewPrintHandler("OK"))
        router.Path("/readiness").Handler(libhttp.NewPrintHandler("OK"))
        router.Path("/metrics").Handler(promhttp.Handler())
        router.Path("/setloglevel/{level}").
            Handler(log.NewSetLoglevelHandler(ctx, log.NewLogLevelSetter(2, 5*time.Minute)))

        glog.V(2).Infof("starting http server listen on %s", a.Listen)
        return libhttp.NewServer(a.Listen, router).Run(ctx)
    }
}
```

Notes on the above:
- `cdb.NewCommandObjectSender` is `github.com/bborbe/cqrs/cdb.NewCommandObjectSender(syncProducer, branch, logSamplerFactory)` — three args, NO topic name. Topic is derived from `branch` + schema by the CQRS layer. The canonical wiring lives at `~/workspaces/agent/task/executor/pkg/factory/factory.go:149`. Use `cqrsbase.Branch("master")` and `liblog.DefaultSamplerFactory`.
- `run.CancelOnFirstFinish(ctx, a.runHTTPServer(), tickLoop.Run)` matches the spec's "run.CancelOnFirstFinish semantics: if one exits, the other is cancelled". The HTTP server `run.Func` and the tick loop's `Run` method both satisfy `func(ctx context.Context) error`. `tickLoop.Run` is a method value bound to the concrete `*tick.tick` value; `run.Func` accepts it directly.
- `log.NewSetLoglevelHandler` and `log.NewLogLevelSetter` come from `github.com/bborbe/log` — the import was previously `github.com/bborbe/log` and is preserved.
- `libmetrics.NewBuildInfoMetrics().SetBuildInfo(...)` is the existing build-info metrics call; preserved.
- The `sentryClient` parameter to `Run` is unused inside `Run` (it was previously used by the `/sentryalert` handler). The signature must stay `Run(ctx context.Context, _ libsentry.Client) error` per the `service.Main` contract. Use the blank identifier `_` — this matches the canonical pattern at `~/workspaces/maintainer/watcher/github-build/main.go:73`.
- The HTTP router has EXACTLY four `router.Path(...)` lines: `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`. The spec AC says "grep -nE 'router\.Path' main.go returns exactly four lines matching these four paths". Do NOT add `/gc` even though the canonical k8s-binary-conventions doc says it is always required — the spec explicitly removes it.
- `Listen` keeps its existing tag (no default added). The spec does not specify a default; do not add one unilaterally.
- The order of `defer` calls in `Run` matters: `defer saramaClient.Close()` and `defer syncProducer.Close()` execute in LIFO order. Close `syncProducer` first (most recent), then `saramaClient`. (This matches the existing main.go.) The `run.CancelOnFirstFinish` is the LAST return value — defer cleanup happens after `Run` returns, so the producer+client close after the run.Funcs are done. Good.

## 4. Delete the skeleton packages

Run from the repo root:

```bash
rm -rf /workspace/pkg/handler
rm -rf /workspace/pkg/mathutil
```

Verify with:
```bash
ls /workspace/pkg/handler /workspace/pkg/mathutil 2>&1
# Both must return "No such file or directory".
```

The `pkg/factory/factory.go` import list no longer references `pkg/handler` (already dropped in §1).

## 5. Run `go mod tidy` and re-verify

`cd /workspace && go mod tidy` — at this point `bborbe/boltkv` and `bborbe/kv` are no longer imported by any file in the module. Tidy will move them from direct to indirect (or remove them if they have no other consumer). If they remain as direct deps after `go mod tidy`, the executor MUST manually move them to the `// indirect` require block in `go.mod` (and run `go mod tidy` again to confirm). The resulting `go.mod` should NOT have any new direct deps; the indirect set may shift.

Note: `IBM/sarama` is already indirect and stays indirect — it's a transitive dep of `bborbe/kafka`.

## 6. Verify that no source file references the deleted symbols

```bash
grep -rE 'pkg/handler|pkg/mathutil|CreateTestLoglevelHandler|CreateSentryAlertHandler' /workspace/main.go /workspace/pkg/
# Must return no matches.

grep -rE 'BATCH_SIZE|DATADIR|boltkv' /workspace/main.go
# Must return no matches.

grep -rE 'libboltkv|"github.com/bborbe/boltkv"|"github.com/bborbe/kv"|libkv\.' /workspace/main.go
# Must return no matches.
```

If any of these still match, fix the source — typically the rewrite in §3 already handles all of them.

## 7. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased`:

```markdown
- feat: Wire `main.go` for the hourly tick loop (initial tick at boot, then 1-hour ticker in parallel with HTTP admin server via `run.CancelOnFirstFinish`); trim HTTP router to `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`; drop `BATCH_SIZE` and `DATADIR` env vars; delete `pkg/handler/` and `pkg/mathutil/` skeleton packages; add `pkg/factory.CreateTick`
```

## 8. Verify and wire-up

After all files are written:

1. Run `cd /workspace && go mod tidy`.
2. Run `cd /workspace && go build ./...` — must compile.
3. Run `cd /workspace && go test ./...` — all Ginkgo specs green across all packages.
4. Run `cd /workspace && make precommit` — must exit 0.
5. Run `cd /workspace && grep -nE 'router\.Path' main.go` — exactly four matches: `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`.
6. Run `cd /workspace && grep -nE 'BATCH_SIZE|DATADIR|boltkv' main.go` — no matches.
7. Run `cd /workspace && grep -nE 'CreateTestLoglevelHandler|CreateSentryAlertHandler' pkg/factory/*.go` — no matches.
8. Run `cd /workspace && ls pkg/handler pkg/mathutil 2>&1` — both return "No such file or directory".
9. Run `cd /workspace && grep -nE 'func CreateTick|func CreatePublisher' pkg/factory/*.go` — exactly two matches: `CreatePublisher` (from Spec 2) and `CreateTick` (this spec).
10. Smoke check (manually, not part of `make precommit`): `cd /workspace && go build -o /tmp/recurring-task-creator . && /tmp/recurring-task-creator --help 2>&1 | grep -E 'kafka-brokers|sentry-dsn|listen'` — output must list `SENTRY_DSN`, `SENTRY_PROXY`, `LISTEN`, `KAFKA_BROKERS`. It must NOT list `BATCH_SIZE` or `DATADIR`.

If `make precommit` flags an unused-variable, missing-license-header, or import-grouping issue, fix it locally; do NOT broaden the scope.

</requirements>

<constraints>
- `main.go` MUST NOT import `github.com/bborbe/boltkv` (aliased as `libboltkv` or otherwise), `github.com/bborbe/kv` (aliased as `libkv` or otherwise), or `github.com/bborbe/handler` / `github.com/bborbe/mathutil` (those packages are deleted and not external).
- `main.go` MUST NOT reference `BATCH_SIZE` or `DATADIR` env vars. The `application` struct drops the `BatchSize` and `DataDir` fields.
- `main.go` MUST NOT open a `libboltkv.OpenDir(...)` call or any other KV-store open.
- `main.go`'s HTTP router MUST contain exactly four `router.Path(...)` lines: `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`. The `/gc`, `/resetdb`, `/resetbucket/{BucketName}`, `/testloglevel`, `/sentryalert` paths are removed.
- `main.go` MUST use `run.CancelOnFirstFinish(ctx, a.runHTTPServer(), tickLoop.Run)` to compose the two long-running goroutines. The spec's "if one exits, the other is cancelled" semantic is what `run.CancelOnFirstFinish` provides.
- `pkg/factory/factory.go` MUST keep only `CreatePublisher` (from Spec 2) and the new `CreateTick`. `CreateTestLoglevelHandler` and `CreateSentryAlertHandler` are removed; the `pkg/handler` import is removed.
- `pkg/factory.CreateTick` MUST call `tick.NewTick(schedule.TasksForDate, pub, clock, metrics)` and panic with a wrapped error on `NewTick` failure (the spec's `LoadLocation` failure is a container-build bug, not a runtime fault — panicking at boot is the right response; `service.Main` will report non-zero exit and the pod will CrashLoopBackOff with the tzdata error visible).
- `pkg/handler/` and `pkg/mathutil/` directories MUST be deleted entirely (no orphan files; verify with `ls pkg/handler pkg/mathutil 2>&1`).
- `pkg/factory/factory_suite_test.go` Ginkgo bootstrap MUST be preserved as-is.
- `pkg/factory/factory_test.go` MUST keep the existing `Describe("CreatePublisher", ...)` block AND add a new `Describe("CreateTick", ...)` block. Each `Describe` has its own `var` block (block-scoped), so variable names like `pub` may safely repeat across blocks — but the suggested pattern in §2 uses `pubFake` / `metricsFake` to avoid reader confusion.
- `make precommit` MUST pass in the changed module.
- License headers MUST be present on every new `.go` file (year `2026`).
- Do NOT add a per-task opt-out flag, a tick-interval knob, a timezone knob, a Prometheus histogram of publish latency, a SIGHUP reload pathway, or any other YAGNI knob from the spec's Non-goals.
- Do NOT touch `pkg/schedule/`, `pkg/publisher/`, `pkg/tick/` (other than importing its new `tick` package), `Makefile`, or any K8s manifest.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./...` — all Ginkgo specs green across all packages.
3. `grep -nE 'router\.Path' main.go` — exactly four matches.
4. `grep -nE 'BATCH_SIZE|DATADIR|boltkv' main.go` — no matches.
5. `grep -rE 'pkg/handler|pkg/mathutil|CreateTestLoglevelHandler|CreateSentryAlertHandler' main.go pkg/` — no matches.
6. `ls pkg/handler pkg/mathutil 2>&1` — both "No such file or directory".
7. `grep -nE 'func CreateTick|func CreatePublisher' pkg/factory/*.go` — exactly two matches.
8. `grep -nE 'run\.CancelOnFirstFinish|CreateTick|service\.Run|libkafka\.NewSyncProducer' main.go` — at least one match per symbol.
9. `/tmp/recurring-task-creator --help 2>&1 | grep -E 'kafka-brokers|sentry-dsn|listen'` — output lists `SENTRY_DSN`, `SENTRY_PROXY`, `LISTEN`, `KAFKA_BROKERS`. Does NOT list `BATCH_SIZE` or `DATADIR`.

</verification>
