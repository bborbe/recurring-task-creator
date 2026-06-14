---
status: completed
spec: [005-trigger-http-handler]
summary: Added `GET /trigger?date=YYYY-MM-DD` and structured `GET /healthz` JSON handlers in fresh `pkg/handler/` package, wired via `pkg/factory.Create*` constructors and a 5-route `main.go` admin router; all tests pass with 100% handler coverage and `make precommit` exits 0.
container: recurring-task-creator-mvp-exec-010-trigger-http-handler
dark-factory-version: v0.177.1
created: "2026-06-14T12:40:27Z"
queued: "2026-06-14T12:46:41Z"
started: "2026-06-14T12:52:13Z"
completed: "2026-06-14T13:03:32Z"
branch: dark-factory/trigger-http-handler
---

<summary>
- Adds a fresh `pkg/handler/` package (the directory was deleted by prompt 008) that owns two HTTP handlers: `NewHealthzHandler() http.Handler` (returns `{"status":"ok"}` JSON, replaces the interim `libhttp.NewPrintHandler("OK")` on `/healthz`) and `NewTriggerHandler(publisher.Publisher) http.Handler` (replays the day's `schedule.TasksForDate` slice through `publisher.Publish` and returns a JSON summary).
- The `/trigger` handler parses the `date` query param as `time.Parse("2006-01-02", ...)`, returns HTTP 400 with `{"error":"missing date parameter"}` on absent/empty param or `{"error":"invalid date format, expected YYYY-MM-DD"}` on parse failure, otherwise iterates the day and accumulates per-task errors into the response without short-circuiting — the response is always HTTP 200 with `{"date","published","errors"}` and `errors: []` (never `null`) on a clean run.
- Recreates `pkg/handler/handler_suite_test.go` (Ginkgo v2 / Gomega bootstrap that prompt 008 deleted alongside the directory) so the new test files compile and run; no SentryAlert or TestLoglevel tests are reinstated.
- Appends two factory constructors in `pkg/factory/factory.go`: `CreateHealthzHandler() http.Handler` and `CreateTriggerHandler(publisher.Publisher) http.Handler` — one-line pass-throughs, zero business logic, no error return. Factory tests assert the constructors return non-nil handlers and that `CreateTriggerHandler` wires the publisher.
- Rewires `main.go`'s admin router: `/healthz` switches from `libhttp.NewPrintHandler("OK")` to `factory.CreateHealthzHandler()`; `/trigger` is added via `factory.CreateTriggerHandler(pub)`; the router after this prompt has exactly five `router.Path` calls (`/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`, `/trigger`).
- No new direct dependencies, no new Prometheus metric, no `time.Now()` reads, no Kafka/sarama/jira-task-creator imports inside `pkg/handler/`, no SIGHUP reload pathway, no multi-date bulk replay.
- `make precommit` exits 0 after the change; trigger handler tests use the existing counterfeiter `mocks.PublisherPublisher` from prompt 006 and the real `schedule.TasksForDate` (no shimming).

</summary>

<objective>
Add the operator-replay surface for `recurring-task-creator`: a `GET /trigger?date=YYYY-MM-DD` endpoint that re-issues today's recurring-task publishes on demand, plus a structured `GET /healthz` JSON endpoint for k8s liveness probes. The handlers live in a fresh `pkg/handler/` package (the directory was removed by prompt 008 and is being re-created here with only the two new handlers). They are constructed via the existing `pkg/factory.Create*` pattern and wired into `main.go`'s admin router, which after this prompt contains five routes total.
</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26.4, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

This prompt executes AFTER prompt 008 (main-wiring) has landed. The expected post-008 state (read these files fully to confirm):

- `/workspace/main.go` — admin router has exactly FOUR routes: `/healthz` (currently `libhttp.NewPrintHandler("OK")`), `/readiness`, `/metrics`, `/setloglevel/{level}`. `application.Run` returns `service.Run(ctx, createTickFunc, createHTTPServerFunc)` (or equivalent parallel composition). The `pub` `publisher.Publisher` instance is built inside `Run` (or is the result of `factory.CreatePublisher(sender)`) and is in scope at the point where `createHTTPServer` is defined.
- `/workspace/pkg/factory/factory.go` — contains only `CreatePublisher(sender task.CreateCommandSender) publisher.Publisher` and `CreateTick(ctx, pub, clock, metrics) tick.Tick`. Does NOT import `pkg/handler` (the package was deleted by prompt 008).
- `/workspace/pkg/handler/` — does not exist on disk. The executor must `mkdir -p pkg/handler` and create `healthz.go`, `trigger.go`, and `handler_suite_test.go` (test bootstrap). Do NOT recreate `sentry-alert.go`, `test-loglevel.go`, or their test files — those handlers are gone for good per prompt 008.
- `/workspace/pkg/publisher/publisher.go` — frozen:
  ```go
  type Publisher interface {
      Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
  }
  ```
- `/workspace/pkg/schedule/date.go` — `Date{Year int, Month time.Month, Day int}`, `NewDate(y,m,d) Date`, `IsZero() bool`. Civil date, no timezone.
- `/workspace/pkg/schedule/tasks_for_date.go` — `TasksForDate(d Date) []TaskDefinition` (sorted by Slug ascending; returns `[]TaskDefinition{}` not `nil` for dates with no matches).
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition{Slug, TitleTemplate, BodyTemplate, Recurrence, Fires}`. The handler reads only `Slug` (for the `errors[]` entries) — the other fields are used by `publisher.Publish`.
- `/workspace/pkg/mocks/publisher-publisher.go` — counterfeiter-generated fake for the publisher interface (generated by prompt 006's `//counterfeiter:generate` directive). Type name: `PublisherPublisher`. Methods: `PublishStub`, `PublishCallCount()`, `PublishArgsForCall(i int) (context.Context, schedule.TaskDefinition, schedule.Date)`, `PublishReturns(err error)`, `PublishReturnsOnCall(i int, err error)`, `PublishCalls(stub func(...) error)`.
- `/workspace/CHANGELOG.md` — `## Unreleased` already has Spec 1 (schedule) and Spec 2 (publisher) bullets; append a new `feat:` bullet for this work.
- `/workspace/go.mod` — no new direct deps. `net/http`, `encoding/json`, `time`, `github.com/golang/glog` are already imported elsewhere. `github.com/gorilla/mux` is the project's router. `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` are already direct deps.

Verified external symbols (read from the project's own source; no new module fetches needed):

`github.com/bborbe/http` (direct dep, v1.26.13) — used at the import site `libhttp "github.com/bborbe/http"`. `libhttp.NewPrintHandler("OK")` is the interim stub being replaced for `/healthz` only; it is NOT being replaced for `/readiness` (Spec 3 owns that, and `/readiness` stays as `NewPrintHandler("OK")`).

`github.com/gorilla/mux` (direct dep, v1.8.1) — `mux.NewRouter()`, `router.Path("/healthz").Handler(handler)`, `router.Path("/trigger").Methods("GET").Handler(handler)`. **Critical**: a `.Path(...)`-only route matches ALL HTTP verbs; without `.Methods("GET")` on `/trigger`, `POST /trigger` hits the handler and returns 200 (contradicting spec row 9). Append `.Methods("GET")` to `/trigger` so a wrong-method request gets a 405 from gorilla/mux's built-in method-not-allowed handler. `/healthz` does NOT need `.Methods(...)` — POST /healthz returning 200 is harmless (the k8s probe is GET).

`github.com/golang/glog` (direct dep, v1.2.5) — `glog.V(2).Infof(...)` for the entry-trace per request, `glog.Errorf(...)` for per-task errors. Per `go-logging-guide.md` and `go-glog-guide.md`: V(2) is the heartbeat level (default verbosity emits nothing), V(3) is per-item. The handler emits one V(2) line per request and one Errorf line per failing task. The error-log line must include the slug (spec AC #13).

`github.com/bborbe/errors` (direct dep, v1.5.13) — used internally when wrapping the `time.Parse` error path. The trigger handler's date-parse failure path returns HTTP 400 with the JSON body — it does NOT propagate the wrapped error to the caller. If you wrap, do it on an internal helper that returns an error, then translate to JSON at the response-writing step. **However, the spec's simplest reading is: do not wrap, just write the JSON body — `time.Parse` is an `error` value and `errors.New` is not needed for a JSON-string body.** Pick the simpler path: do not wrap; write the JSON body directly.

`github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` — Ginkgo v2 dot-imported, Gomega `Expect`/`Equal`/`ContainSubstring`/`HaveLen`/`BeAssignableToTypeOf`. External test package `package handler_test`.

`github.com/maxbrunsfeld/counterfeiter/v6` (transitive, used in `//go:generate` directives) — the existing publisher mock is already generated; do not re-run counterfeiter in this prompt.

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method. The trigger handler depends on the `publisher.Publisher` INTERFACE (not the concrete `*publisher`), which is the canonical DI shape. For stateless handlers, returning `http.Handler` directly from `New<Name>Handler` is sufficient (matches the now-deleted `NewSentryAlertHandler` / `NewTestLoglevelHandler` shape).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-http-handler-refactoring-guide.md` — handlers in `pkg/handler/`, factory wiring in `pkg/factory/`, never inline in `main.go`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `Create*` prefix, zero business logic, returns interface type, never returns `error`. `CreateHealthzHandler` and `CreateTriggerHandler` are one-line pass-throughs.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, dot-imports, `BeforeEach`, `Expect`, external test package (`package handler_test`).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — internal helpers that return errors wrap with `github.com/bborbe/errors`; the handler's date-parse path does NOT propagate the wrapped error (it writes a JSON body), so no wrap needed.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-logging-guide.md` — `glog.V(2).Infof` for the request entry trace, `glog.Errorf` for per-task errors. Never `glog.Info*` without V-gating.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-licensing-guide.md` — BSD license header on every new `.go` file, year `2026`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/dod.md` — coverage ≥80% for new code.

Load-bearing snippets inlined for executor verification:

```go
// /workspace/pkg/publisher/publisher.go (verbatim, frozen)
type Publisher interface {
    Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
}

// /workspace/pkg/schedule/date.go (verbatim, frozen)
type Date struct {
    Year  int
    Month time.Month
    Day   int
}
func NewDate(year int, month time.Month, day int) Date
func (d Date) IsZero() bool

// /workspace/pkg/schedule/tasks_for_date.go (verbatim, frozen)
func TasksForDate(d Date) []TaskDefinition

// /workspace/pkg/schedule/task_definition.go (verbatim, frozen)
type TaskDefinition struct {
    Slug          string
    TitleTemplate string
    BodyTemplate  string
    Recurrence    RecurrenceKind
    Fires         predicate
}

// /workspace/pkg/mocks/publisher-publisher.go (verbatim, counterfeiter output)
type PublisherPublisher struct {
    PublishStub func(context.Context, schedule.TaskDefinition, schedule.Date) error
    // ...
}
func (fake *PublisherPublisher) Publish(arg1 context.Context, arg2 schedule.TaskDefinition, arg3 schedule.Date) error
func (fake *PublisherPublisher) PublishCallCount() int
func (fake *PublisherPublisher) PublishArgsForCall(i int) (context.Context, schedule.TaskDefinition, schedule.Date)
func (fake *PublisherPublisher) PublishReturns(result1 error)
func (fake *PublisherPublisher) PublishReturnsOnCall(i int, result1 error)
func (fake *PublisherPublisher) PublishCalls(stub func(context.Context, schedule.TaskDefinition, schedule.Date) error)
```

Pre-existing handler-package structure (was DELETED by prompt 008; this prompt RECREATES the directory):
- `pkg/handler/handler_suite_test.go` — recreate with the same shape as `/workspace/pkg/publisher/publisher_suite_test.go` (the Ginkgo suite test for the package). Use `package handler_test`, `RunSpecs(t, "Handler Suite", ...)`, 60-second timeout, `time.Local = time.UTC`, `format.TruncatedDiff = false`. The `//go:generate` line is OPTIONAL — the package has no `//counterfeiter:generate` directives of its own, so omitting it is fine. (Counterfeiter directive is only needed if the package owns an interface that needs a fake. The handlers consume `publisher.Publisher`; its fake is already in `pkg/mocks/`.)
- `pkg/handler/sentry-alert.go` and `pkg/handler/test-loglevel.go` — DO NOT recreate. They are gone.
- `pkg/handler/sentry-alert_test.go` and `pkg/handler/test-loglevel_test.go` — DO NOT recreate.

</context>

<requirements>

## 1. Recreate `pkg/handler/` and add the suite test

Create `/workspace/pkg/handler/handler_suite_test.go` (the directory will be auto-created by your editor). The file is the Ginkgo v2 / Gomega suite entry for the package. Mirror the structure of `/workspace/pkg/publisher/publisher_suite_test.go`:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler_test

import (
    "testing"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/onsi/gomega/format"
)

func TestSuite(t *testing.T) {
    time.Local = time.UTC
    format.TruncatedDiff = false
    RegisterFailHandler(Fail)
    suiteConfig, reporterConfig := GinkgoConfiguration()
    suiteConfig.Timeout = 60 * time.Second
    RunSpecs(t, "Handler Suite", suiteConfig, reporterConfig)
}
```

No `//go:generate` line is needed (the package owns no `//counterfeiter:generate` directives).

## 2. The `/healthz` handler

Create `/workspace/pkg/handler/healthz.go`:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
    "encoding/json"
    "net/http"
)

// NewHealthzHandler returns an HTTP handler that responds with HTTP 200 and
// a fixed JSON body {"status":"ok"}. It is the liveness-probe target for the
// Service in the Spec 5 manifests. No state, no I/O, no dependencies —
// safe to call at any cadence from any source.
func NewHealthzHandler() http.Handler {
    return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
        resp.Header().Set("Content-Type", "application/json")
        resp.WriteHeader(http.StatusOK)
        _ = json.NewEncoder(resp).Encode(map[string]string{"status": "ok"})
    })
}
```

Notes:
- Use `json.NewEncoder(resp).Encode(...)` (not `json.Marshal` + `Write`) — this matches the project style and handles the trailing newline (`json.Encoder.Encode` always appends `\n`; the spec's AC body assertion uses `ContainSubstring` style and the `{"status":"ok"}` prefix is what matters).
- The trailing newline from `json.Encoder.Encode` is acceptable per spec AC #4 ("exact bytes, no trailing newline required" — the spec text says "no trailing newline required" meaning the absence is not a hard contract; a trailing newline is fine).
- `WriteHeader(http.StatusOK)` is the default; the explicit call documents intent and is harmless.

## 3. The `/trigger` handler

Create `/workspace/pkg/handler/trigger.go`:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/golang/glog"

    "github.com/bborbe/recurring-task-creator/pkg/publisher"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// triggerErrorEntry is one per-task failure in the /trigger response.
// Always emitted, even when empty (use json:"errors" — no omitempty).
type triggerErrorEntry struct {
    Slug  string `json:"slug"`
    Error string `json:"error"`
}

// triggerResponse is the JSON shape of GET /trigger on a 2xx.
// `errors` is always present, never omitted.
type triggerResponse struct {
    Date      string             `json:"date"`
    Published int                `json:"published"`
    Errors    []triggerErrorEntry `json:"errors"`
}

// NewTriggerHandler returns an HTTP handler that replays the recurring-task
// publishes for one civil date. The date is supplied as the `date` query
// parameter in YYYY-MM-DD format. For each entry returned by
// schedule.TasksForDate for that date, the handler calls
// publisher.Publish(req.Context(), def, date). Per-task errors are
// accumulated in the response's `errors` array — the iteration does NOT
// short-circuit on error. The response is always HTTP 200 on a successfully
// parsed date, regardless of whether any individual publish failed.
//
// Malformed or missing `date` parameter returns HTTP 400 with a JSON body
// of the form {"error":"<message>"}. The handler holds no per-request state
// and is safe to call concurrently for the same date (the controller dedups
// by deterministic UUID5).
func NewTriggerHandler(publisher publisher.Publisher) http.Handler {
    return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
        param := req.URL.Query().Get("date")
        if param == "" {
            writeTriggerError(resp, http.StatusBadRequest, "missing date parameter")
            return
        }
        t, err := time.Parse("2006-01-02", param)
        if err != nil {
            writeTriggerError(resp, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
            return
        }
        date := schedule.NewDate(t.Year(), t.Month(), t.Day())
        tasks := schedule.TasksForDate(date)

        glog.V(2).Infof("trigger: processing %d task(s) for %04d-%02d-%02d", len(tasks), date.Year, date.Month, date.Day)

        out := triggerResponse{
            Date:      param,
            Published: 0,
            Errors:    []triggerErrorEntry{},
        }
        for _, def := range tasks {
            if pubErr := publisher.Publish(req.Context(), def, date); pubErr != nil {
                glog.Errorf("trigger: publish failed for slug %q on %s: %v", def.Slug, param, pubErr)
                out.Errors = append(out.Errors, triggerErrorEntry{
                    Slug:  def.Slug,
                    Error: pubErr.Error(),
                })
                continue
            }
            out.Published++
        }

        resp.Header().Set("Content-Type", "application/json")
        resp.WriteHeader(http.StatusOK)
        _ = json.NewEncoder(resp).Encode(out)
    })
}

// writeTriggerError writes a JSON error body with the given status and message.
// Used for the missing/invalid `date` parameter paths.
func writeTriggerError(resp http.ResponseWriter, status int, message string) {
    resp.Header().Set("Content-Type", "application/json")
    resp.WriteHeader(status)
    _ = json.NewEncoder(resp).Encode(map[string]string{"error": message})
}
```

Notes on the above:
- `out.Errors = []triggerErrorEntry{}` is set explicitly so the JSON serialization emits `[]` (not `null`) when no errors occurred. The `json:"errors"` tag (no `omitempty`) is also load-bearing — see AC #12. **Both** the explicit empty-slice initialization AND the absence of `omitempty` are required.
- The iteration does NOT check `req.Context().Done()` between tasks (spec non-goal #12). `publisher.Publish` itself takes the request context; if cancellation fires during a publish, the publish returns the cancellation error and the handler records it in `out.Errors`. The standard library then aborts the response write — acceptable per spec.
- The handler logs at V(2) once per request (entry trace) and at Errorf once per failing task. The per-task Errorf format string contains the slug literal `"%q"` (paired with `def.Slug`) — verifiable per AC #13.
- The response echoes `param` (the original `date` query string), not `date.Year-Month-Day` formatted via `fmt.Sprintf`. This guarantees round-trip identity (input "2025-01-04" → output `"date":"2025-01-04"` byte-identical). Alternative: format from the parsed `date` — pick the simpler of the two and document. Use `param` for simplicity.
- The `publisher` parameter is shadowed by the function-scope name; the call inside the loop is `publisher.Publish(req.Context(), def, date)`.
- The handler depends on the `publisher.Publisher` interface only. The trigger handler tests inject a `*mocks.PublisherPublisher` (counterfeiter fake).

## 4. The `/healthz` test

Create `/workspace/pkg/handler/healthz_test.go`:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler_test

import (
    "net/http"
    "net/http/httptest"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/bborbe/recurring-task-creator/pkg/handler"
)

var _ = Describe("HealthzHandler", func() {
    var httpHandler http.Handler
    BeforeEach(func() {
        httpHandler = handler.NewHealthzHandler()
    })

    It("returns 200 with application/json content type and the literal status body", func() {
        req := httptest.NewRequest("GET", "/healthz", nil)
        resp := httptest.NewRecorder()

        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(resp.Header().Get("Content-Type")).To(Equal("application/json"))
        Expect(resp.Body.String()).To(ContainSubstring(`"status":"ok"`))
    })

    It("does not depend on request body, query params, or method", func() {
        req := httptest.NewRequest("POST", "/healthz", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)
        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(resp.Body.String()).To(ContainSubstring(`"status":"ok"`))
    })
})
```

The first `It` covers AC #4. The second `It` is a robustness check (the handler ignores the method, query, and body — gorilla/mux's router-level method matching is what enforces `GET`-only at the wire level).

## 5. The `/trigger` test

Create `/workspace/pkg/handler/trigger_test.go`. The test suite covers every AC for the trigger handler (5-13 in the spec).

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler_test

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/bborbe/recurring-task-creator/pkg/handler"
    "github.com/bborbe/recurring-task-creator/pkg/mocks"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("TriggerHandler", func() {
    var (
        fakePublisher *mocks.PublisherPublisher
        httpHandler   http.Handler
    )

    BeforeEach(func() {
        fakePublisher = &mocks.PublisherPublisher{}
        httpHandler = handler.NewTriggerHandler(fakePublisher)
    })

    // ---------- 400 paths (missing/invalid date) ----------

    It("returns 400 with 'missing date parameter' when no date query is set", func() {
        req := httptest.NewRequest("GET", "/trigger", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusBadRequest))
        Expect(resp.Header().Get("Content-Type")).To(Equal("application/json"))
        Expect(resp.Body.String()).To(ContainSubstring("missing date parameter"))
        Expect(fakePublisher.PublishCallCount()).To(Equal(0))
    })

    It("returns 400 with 'missing date parameter' when date query is empty", func() {
        req := httptest.NewRequest("GET", "/trigger?date=", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusBadRequest))
        Expect(resp.Body.String()).To(ContainSubstring("missing date parameter"))
        Expect(fakePublisher.PublishCallCount()).To(Equal(0))
    })

    It("returns 400 with 'invalid date format' for non-date input", func() {
        req := httptest.NewRequest("GET", "/trigger?date=not-a-date", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusBadRequest))
        Expect(resp.Body.String()).To(ContainSubstring("invalid date format, expected YYYY-MM-DD"))
        Expect(fakePublisher.PublishCallCount()).To(Equal(0))
    })

    It("returns 400 with 'invalid date format' for day-of-month=32 (parse-fail)", func() {
        req := httptest.NewRequest("GET", "/trigger?date=2025-01-32", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusBadRequest))
        Expect(resp.Body.String()).To(ContainSubstring("invalid date format, expected YYYY-MM-DD"))
        Expect(fakePublisher.PublishCallCount()).To(Equal(0))
    })

    // ---------- happy path: real schedule, fake publisher ----------

    It("calls publisher.Publish once for every entry returned by schedule.TasksForDate", func() {
        date := schedule.NewDate(2025, time.January, 4)
        tasks := schedule.TasksForDate(date)

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
    })

    It("responds 200 with date, published=N, errors=[] when all publishes succeed", func() {
        date := schedule.NewDate(2025, time.January, 4)
        tasks := schedule.TasksForDate(date)
        fakePublisher.PublishReturns(nil)

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(resp.Header().Get("Content-Type")).To(Equal("application/json"))

        var body struct {
            Date      string `json:"date"`
            Published int    `json:"published"`
            Errors    []struct {
                Slug  string `json:"slug"`
                Error string `json:"error"`
            } `json:"errors"`
        }
        Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
        Expect(body.Date).To(Equal("2025-01-04"))
        Expect(body.Published).To(Equal(len(tasks)))
        Expect(body.Errors).To(BeEmpty())
    })

    It("serializes errors as [] (not null) when no errors occurred", func() {
        fakePublisher.PublishReturns(nil)

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Body.String()).To(ContainSubstring(`"errors":[]`))
    })

    It("returns 200 with errors[] populated and published=len(tasks)-1 when one publish fails", func() {
        date := schedule.NewDate(2025, time.January, 4)
        tasks := schedule.TasksForDate(date)
        target := tasks[0].Slug

        // Use PublishStub (not PublishReturns) so the fake returns nil for
        // every call EXCEPT the one matching the target slug.
        fakePublisher.PublishCalls(func(ctx context.Context, def schedule.TaskDefinition, d schedule.Date) error {
            if def.Slug == target {
                return errors.New("simulated publish failure")
            }
            return nil
        })

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))

        var body struct {
            Date      string `json:"date"`
            Published int    `json:"published"`
            Errors    []struct {
                Slug  string `json:"slug"`
                Error string `json:"error"`
            } `json:"errors"`
        }
        Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
        Expect(body.Published).To(Equal(len(tasks) - 1))
        Expect(body.Errors).To(HaveLen(1))
        Expect(body.Errors[0].Slug).To(Equal(target))
        Expect(body.Errors[0].Error).To(ContainSubstring("simulated publish failure"))
    })

    It("returns 200 (not 5xx) with published=0 and full errors array when every publish fails", func() {
        date := schedule.NewDate(2025, time.January, 4)
        tasks := schedule.TasksForDate(date)
        fakePublisher.PublishReturns(errors.New("all down"))

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))

        var body struct {
            Published int `json:"published"`
            Errors    []struct {
                Slug  string `json:"slug"`
                Error string `json:"error"`
            } `json:"errors"`
        }
        Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
        Expect(body.Published).To(Equal(0))
        Expect(body.Errors).To(HaveLen(len(tasks)))
    })

    It("propagates the request context to publisher.Publish", func() {
        type ctxKey struct{}
        ctx := context.WithValue(context.Background(), ctxKey{}, "marker")
        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil).WithContext(ctx)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(fakePublisher.PublishCallCount()).To(BeNumerically(">", 0))
        for i := 0; i < fakePublisher.PublishCallCount(); i++ {
            callCtx, _, _ := fakePublisher.PublishArgsForCall(i)
            Expect(callCtx.Value(ctxKey{})).To(Equal("marker"))
        }
    })

})
```

Note on AC #13 (per-task error log): the source-grep check in `<verification>` step 12 (`grep -nE 'glog\.Errorf' pkg/handler/trigger.go` — at least one match; format string includes the slug via `%q` paired with `def.Slug`) is the verification path. No glog import is needed in this test file.

Imports required:
```go
import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/bborbe/recurring-task-creator/pkg/handler"
    "github.com/bborbe/recurring-task-creator/pkg/mocks"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)
```

## 6. Factory wiring

Append to `/workspace/pkg/factory/factory.go`. The file after prompt 008 contains only `CreatePublisher` and `CreateTick`. **Add** two new constructors. Do NOT remove the existing ones.

The import block needs `net/http` (the handlers' return type) added; the `pkg/handler` import needs to be reintroduced; the `pkg/publisher` import is already present.

```go
// (file head, 2026 copyright header preserved)

package factory

import (
    "context"
    "net/http"

    "github.com/bborbe/agent/lib/command/task"
    "github.com/bborbe/errors"
    libtime "github.com/bborbe/time"

    "github.com/bborbe/recurring-task-creator/pkg/handler"
    "github.com/bborbe/recurring-task-creator/pkg/publisher"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
    "github.com/bborbe/recurring-task-creator/pkg/tick"
)

// (existing CreatePublisher and CreateTick — DO NOT MODIFY)

// CreateHealthzHandler returns the liveness-probe HTTP handler. Pure plumbing.
func CreateHealthzHandler() http.Handler {
    return handler.NewHealthzHandler()
}

// CreateTriggerHandler returns the operator-replay HTTP handler. Pure plumbing.
func CreateTriggerHandler(publisher publisher.Publisher) http.Handler {
    return handler.NewTriggerHandler(publisher)
}
```

**Note on parameter naming**: the spec text (DB #10) shows `CreateTriggerHandler(publisher publisher.Publisher) http.Handler` — the parameter name is `publisher` and the type is `publisher.Publisher`. The factory function body passes it through to `handler.NewTriggerHandler(publisher)`. The variable name shadows the package name in the function body; this is acceptable because we never access the `publisher` package symbol inside the body (we only use the parameter).

**`net/http` import**: needed for the `http.Handler` return type. Add to the stdlib import group.

## 7. Factory tests

Append two new `Describe` blocks to `/workspace/pkg/factory/factory_test.go`. The existing `CreatePublisher` and `CreateTick` blocks stay.

```go
var _ = Describe("CreateHealthzHandler", func() {
    It("returns a non-nil http.Handler", func() {
        Expect(factory.CreateHealthzHandler()).NotTo(BeNil())
    })
})

var _ = Describe("CreateTriggerHandler", func() {
    var (
        pubFake   *projmocks.PublisherPublisher
        httpHndl  http.Handler
    )
    BeforeEach(func() {
        pubFake = &projmocks.PublisherPublisher{}
        pubFake.PublishReturns(nil)
        httpHndl = factory.CreateTriggerHandler(pubFake)
    })
    It("returns a non-nil http.Handler", func() {
        Expect(httpHndl).NotTo(BeNil())
    })
    It("wires the supplied publisher into the handler (publish is reachable through the returned handler)", func() {
        // Smoke test: drive the handler with a known date and confirm the
        // fake publisher was called the expected number of times. Detailed
        // per-task behavior is covered in pkg/handler/trigger_test.go.
        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHndl.ServeHTTP(resp, req)
        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(pubFake.PublishCallCount()).To(Equal(
            len(schedule.TasksForDate(schedule.NewDate(2025, time.January, 4))),
        ))
    })
})
```

## 8. `main.go` router rewiring

Modify `/workspace/main.go` ONLY in the `createHTTPServer` function's `router.Path(...).Handler(...)` block. The rest of `main.go` (the `application` struct, the `Run` method, the `service.Run` composition) is UNCHANGED.

The pre-005 router has 4 paths. After this prompt, the router has 5 paths:

```go
router := mux.NewRouter()
router.Path("/healthz").Handler(factory.CreateHealthzHandler())
router.Path("/readiness").Handler(libhttp.NewPrintHandler("OK"))
router.Path("/metrics").Handler(promhttp.Handler())
router.Path("/setloglevel/{level}").
    Handler(log.NewSetLoglevelHandler(ctx, log.NewLogLevelSetter(2, 5*time.Minute)))
router.Path("/trigger").Methods("GET").Handler(factory.CreateTriggerHandler(pub))
```

Key changes:
- `/healthz` switches from `libhttp.NewPrintHandler("OK")` to `factory.CreateHealthzHandler()`. Spec 3's text response on `/readiness` stays as-is (it is NOT being replaced in this spec).
- `/trigger` is added. The handler is built via `factory.CreateTriggerHandler(pub)` where `pub` is the `publisher.Publisher` instance already constructed earlier in `application.Run` (the one passed to `factory.CreateTick`). The exact variable name in scope is whatever prompt 008 left — likely `pub` (a `publisher.Publisher`). If prompt 008 named it differently (e.g. `publisher`), match the local name. Read the current `main.go` to find the variable name and use it.

**Do NOT change any other path.** Do NOT add new env vars, new log lines, or new imports beyond what is needed for the new handler symbols.

**Variable scoping for the publisher**: thread the `publisher.Publisher` to `createHTTPServer` as an additional parameter. Read the current `main.go` (post-008 state) to determine the existing parameter list, then APPEND the publisher to that list (do not reorder existing parameters). At the `service.Run` call site in `Run`, pass the publisher in the same position. The variable name in scope is whatever prompt 008 used — likely `pub`; read the file to confirm.

After the rewiring:
- `grep -nE 'router\.Path' main.go` returns exactly 5 lines.
- `grep -nE '/healthz' main.go` shows the line; the next line (or same line) references `CreateHealthzHandler`.
- `grep -nE 'NewPrintHandler\("OK"\)' main.go` returns exactly ONE match (the `/readiness` line).
- `grep -nE 'CreateTriggerHandler' main.go` returns at least one match.

## 9. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (the section already exists from Spec 1 and Spec 2):

```markdown
- feat: Add `GET /trigger?date=YYYY-MM-DD` HTTP handler that replays the day's recurring-task publishes on demand and returns a JSON summary with per-task error accumulation; add structured `GET /healthz` JSON handler replacing the Spec 3 text stub
```

The bullet uses the `feat:` prefix (minor bump per `changelog-guide.md`).

## 10. Imports and conventions

- Every new `.go` file starts with the 2026 copyright header (3 lines):
  ```
  // Copyright (c) 2026 Benjamin Borbe All rights reserved.
  // Use of this source code is governed by a BSD-style
  // license that can be found in the LICENSE file.
  ```
- Use `goimports`-style grouping: stdlib first, third-party alphabetical, then internal `github.com/bborbe/recurring-task-creator/...`.
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Use `github.com/bborbe/errors` for any internal error wrapping. The trigger handler's date-parse path does NOT need wrapping (it writes a JSON body, not an error). Do not introduce `fmt.Errorf`.
- Do NOT touch `Makefile`, `Makefile.precommit`, `Makefile.variables`, `k8s/`, or any file under `pkg/schedule/`, `pkg/publisher/`, `pkg/tick/`, `pkg/mathutil/`, or `mocks/`.
- Do NOT add a Prometheus metric, an opt-out flag, a runtime config knob, or any per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT add `//counterfeiter:generate` directives — the package owns no interface that needs a fake (it consumes `publisher.Publisher`, whose fake is already in `pkg/mocks/`).

## 11. Verification

After all files are written, run from `/workspace`:

1. `cd /workspace && go build ./...` — must compile.
2. `cd /workspace && go test ./pkg/handler/... ./pkg/factory/...` — all Ginkgo specs green.
3. `cd /workspace && make precommit` — exits 0.
4. `cd /workspace && grep -nE 'glog\.Errorf' pkg/handler/trigger.go` — at least one match (per-task error log; AC #13).
5. `cd /workspace && grep -nE 'func NewHealthzHandler' pkg/handler/healthz.go` — exactly one match.
6. `cd /workspace && grep -nE 'func NewTriggerHandler' pkg/handler/trigger.go` — exactly one match.
7. `cd /workspace && grep -nE 'func CreateHealthzHandler|func CreateTriggerHandler' pkg/factory/factory.go` — exactly two matches.
8. `cd /workspace && grep -nE 'router\.Path' main.go` — exactly five matches.
9. `cd /workspace && grep -nE 'NewPrintHandler\("OK"\)' main.go` — exactly one match (the `/readiness` line).
10. `cd /workspace && grep -nE 'CreateHealthzHandler' main.go` — at least one match.
11. `cd /workspace && grep -nE 'CreateTriggerHandler' main.go` — at least one match.
12. `cd /workspace && grep -nE 'PublisherPublisher' pkg/handler/trigger_test.go` — at least one match (counterfeiter mock used).
13. `cd /workspace && grep -E '"(github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/handler/*.go` — no matches (forbidden-imports / no-clock-read guard).
14. `cd /workspace && grep -nE '"github\.com/bborbe/recurring-task-creator/pkg/(schedule|publisher)"' pkg/handler/trigger.go` — exactly two matches.
15. `cd /workspace && grep -nE '"errors":\[\]' pkg/handler/trigger.go` — no match in source (the test asserts the literal substring in the response body, not in source). For the BODY check, run the test: `go test ./pkg/handler/... -run TestSuite -v` and confirm the `It("serializes errors as [] (not null)...")` case passes.

If any check fails, fix the underlying code; do NOT silence the test.

</requirements>

<constraints>
- The `pkg/handler/` package MUST NOT import `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, or any `github.com/bborbe/jira-task-creator/...` package.
- The `pkg/handler/` package MUST NOT call `time.Now()` directly — the date for `/trigger` comes from the parsed query parameter.
- The `pkg/handler/` package MUST NOT import `github.com/bborbe/recurring-task-creator/pkg/publisher` to take a concrete type; it depends ONLY on the `publisher.Publisher` interface for `/trigger`. Direct import of the `pkg/publisher` package for the interface symbol is required and expected.
- The handler MUST follow the spec's date-parsing contract: `time.Parse("2006-01-02", param)`, then `schedule.NewDate(t.Year(), t.Month(), t.Day())`. No `time.Local`, no `time.UTC` involvement in the comparison logic.
- The handler MUST NOT short-circuit on `ctx.Err()` between per-task publishes — `publisher.Publish` owns context discipline. The handler captures context-cancellation errors in the per-task `errors[]` array.
- The handler MUST NOT call `schedule.TasksForDate` from a goroutine — synchronous in-handler call only.
- The response for `/trigger` MUST include `errors: []` (not `null`) on a clean run. Use `json:"errors"` (no `omitempty`) AND initialize the field with `[]triggerErrorEntry{}`. Both are required.
- The response for `/trigger` MUST be HTTP 200 even when every publish in the day failed (response body carries the truth).
- The response for missing/invalid `date` MUST be HTTP 400 with `Content-Type: application/json` and body `{"error":"<message>"}` where `<message>` is exactly `missing date parameter` (param absent or empty) or exactly `invalid date format, expected YYYY-MM-DD` (param malformed).
- The factory constructors `CreateHealthzHandler` and `CreateTriggerHandler` follow the project pattern: `Create*` prefix, zero business logic, no `error` return, return interface type (`http.Handler` is the interface here).
- The `/trigger` handler depends on the `publisher.Publisher` INTERFACE, not the concrete `*publisher.publisher` struct.
- Tests use Ginkgo v2 / Gomega. Trigger handler tests use the existing counterfeiter `mocks.PublisherPublisher` from `pkg/mocks/`. No hand-written mocks.
- The `pkg/handler/` directory was DELETED by prompt 008. This prompt RECREATES it with only `healthz.go`, `trigger.go`, `handler_suite_test.go`, and their tests. Do NOT recreate `sentry-alert.go`, `test-loglevel.go`, or their test files — they are gone for good.
- The `/readiness` endpoint stays as `libhttp.NewPrintHandler("OK")` — Spec 3 owns it and this spec only replaces `/healthz`.
- Do NOT add a Prometheus metric, an opt-out flag, a runtime config knob, a per-task disable mechanism, or any other configuration surface. Spec Non-goals forbid all of these.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- The `pkg/handler/` package does NOT need a `//go:generate counterfeiter` line in its suite test (no interfaces in the package require a fake).

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/handler/... ./pkg/factory/...` — all Ginkgo specs green; counterfeiter mock used in trigger test; ≥80% statement coverage on the new code (per `dod.md`).
3. `grep -nE 'func NewHealthzHandler' pkg/handler/healthz.go` — exactly one match.
4. `grep -nE 'func NewTriggerHandler' pkg/handler/trigger.go` — exactly one match.
5. `grep -nE 'func CreateHealthzHandler|func CreateTriggerHandler' pkg/factory/factory.go` — exactly two matches.
6. `grep -nE 'router\.Path' main.go` — exactly five matches (`/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`, `/trigger`).
7. `grep -nE 'NewPrintHandler\("OK"\)' main.go` — exactly one match (the `/readiness` line).
8. `grep -nE 'CreateHealthzHandler|CreateTriggerHandler' main.go` — at least one match for each.
9. `grep -nE 'PublisherPublisher' pkg/handler/trigger_test.go` — at least one match.
10. `grep -E '"(github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/handler/*.go` — no matches.
11. `grep -nE '"github\.com/bborbe/recurring-task-creator/pkg/(schedule|publisher)"' pkg/handler/trigger.go` — exactly two matches.
12. `grep -nE 'glog\.Errorf' pkg/handler/trigger.go` — at least one match; the format string includes the slug (`%q` paired with `def.Slug`).
13. `grep -nE 'json:"errors"' pkg/handler/trigger.go` — exactly one match (no `omitempty` on the field).
14. `go test ./pkg/handler/... -v -run TestSuite` — the `It("serializes errors as [] (not null)...")` case passes (this is the literal-substring body check; the source-level `json:"errors"` tag is necessary but not sufficient on its own).
15. `ls pkg/handler/` — contains exactly `healthz.go`, `trigger.go`, `healthz_test.go`, `trigger_test.go`, `handler_suite_test.go`. No `sentry-alert.go`, no `test-loglevel.go`, no `sentry-alert_test.go`, no `test-loglevel_test.go`.

Expected final `make precommit` output: exit code 0, all tests green, lint clean, license headers present, no forbidden imports.

## Open Questions (for the human reviewer)

- **`publisher` parameter name in `CreateTriggerHandler`.** The spec text (DB #10) shows `CreateTriggerHandler(publisher publisher.Publisher) http.Handler` — the parameter name shadows the package name. This is acceptable because the function body only forwards the parameter; it never references the `publisher` package symbol. If the audit rejects the shadow, rename the parameter to `pub` (and update the body to `handler.NewTriggerHandler(pub)`). Both compile.
- **`pub` variable name in `main.go`.** The exact name of the `publisher.Publisher` instance in `application.Run` (post-prompt-008) is determined by prompt 008's variable naming. The executor must READ the current `main.go` and use whatever name is in scope at the `createHTTPServer` call site. Likely names: `pub`, `publisher`. If the publisher is currently a parameter to `createHTTPServer`, the executor adds it; if it's a method-receiver field on `application`, the executor uses the field. Read the file first.
- **Glog test capture.** No precedent in this project for capturing glog stderr in tests. AC #13 is satisfied via source-level grep (step 12 above). If a future test ever needs to assert the log content, a `glog.SetOutputBySeverity` capture helper is the standard pattern — but that is out of scope for this spec.
- **No scenario file.** The spec explicitly says NO scenario. The handler is a pure in-process HTTP-to-publisher adapter; counterfeiter `Publisher` + real `schedule.TasksForDate` + `httptest.NewRecorder` covers every behavior the scenario rule would target.

</verification>
