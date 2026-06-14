---
status: generating
tags:
    - dark-factory
    - spec
approved: "2026-06-14T12:12:07Z"
generating: "2026-06-14T12:25:32Z"
branch: dark-factory/trigger-http-handler
---

## Summary

- Add a new HTTP handler `GET /trigger?date=YYYY-MM-DD` that lets an operator replay the recurring-task publishes for any single civil date on demand.
- Add a JSON-shaped `GET /healthz` handler (`{"status":"ok"}`) that the Kubernetes liveness/readiness probes (Spec 5) target. Replaces Spec 3's interim `libhttp.NewPrintHandler("OK")` for `/healthz` only.
- Re-uses `pkg/schedule.TasksForDate` + the existing `pkg/publisher.Publisher` from Spec 2 — no new scheduling logic, no new Kafka I/O code. The handler is a thin HTTP-to-publisher adapter.
- The trigger handler is intentionally unauthenticated and safe to replay: every publish in the day's set is keyed by the deterministic UUID5 from Spec 2, so the controller dedups duplicates server-side. Replays are idempotent at the vault layer.
- Out of scope: `/readiness`, `/metrics`, `/setloglevel/{level}` (already wired by Spec 3); multi-date bulk replay (`from=`/`to=`); auth on `/trigger` (the k8s `Service` is cluster-internal — Spec 5 does not expose `/trigger` via Ingress until a follow-up).

## Problem

After Spec 3 lands, the binary publishes today's tasks every hour starting from boot — but if a tick is missed (pod down, broker hiccup that exceeded one hour, operator-introduced bug), there is no way to replay a specific day without restarting the pod and waiting for the next civil-date roll-over. The Spec 5 manifests reference `/healthz` for the liveness probe and `/trigger` is named in the binary's `--help` env doc, but neither handler exists in any Go package today — Spec 3 only wired the four admin endpoints (`/healthz` as a text `OK`, `/readiness`, `/metrics`, `/setloglevel/{level}`).

Without a `/trigger` handler, the operational story is "wait an hour, hope the next tick succeeds." Without a structured `/healthz` JSON response, the probe contract is implicit — a future reviewer cannot grep for the response shape and a future client (e.g. a Prometheus blackbox probe, an external uptime monitor) cannot rely on a stable field to parse.

The `/healthz` change is small but load-bearing: Spec 5's STS declares the liveness probe against `/healthz`, and the JSON response shape is the artifact the probe consumers can depend on. Folding `/trigger` and `/healthz` into one spec keeps the new `pkg/handler/` package landing with both of its inhabitants in one diff.

## Goal

After this work, a fresh `pkg/handler/` package owns two HTTP handlers — one for `/healthz` returning JSON `{"status":"ok"}` with HTTP 200, one for `/trigger?date=YYYY-MM-DD` that parses the date param, iterates `schedule.TasksForDate(date)`, calls `publisher.Publish(ctx, def, date)` for each entry, and returns HTTP 200 with a JSON summary `{"date":"YYYY-MM-DD","published":N,"errors":[{"slug":"...","error":"..."}, ...]}`. Per-task publish errors are collected into the `errors` array but never cause the request to fail with a 5xx — the response always carries the partial result. Malformed `date` params return HTTP 400 with a JSON error body. Missing `date` param returns HTTP 400. The handlers are constructed via the existing `pkg/factory` pattern (one `Create<Name>Handler` constructor per handler), wired into `main.go`'s admin router at the corresponding paths, replacing Spec 3's interim `/healthz` print-handler stub.

## Non-goals

- Do NOT add auth on `/trigger` — invariant; the worst case is a replay of an already-deterministic-UUID5 publish, which the controller dedups. If a future consumer demands variation (e.g. exposing `/trigger` via public Ingress), that is a separate spec.
- Do NOT add a multi-date bulk replay endpoint (`/trigger?from=&to=`) — single-day only. If a future consumer demands variation, that is a separate spec.
- Do NOT add a web UI, static assets, HTML response shape, or any non-JSON content type for the handler responses.
- Do NOT change `/readiness`, `/metrics`, or `/setloglevel/{level}` — Spec 3 owns those. This spec only adds `/trigger` and replaces `/healthz`'s handler.
- Do NOT add an HTTP server, listener, or any `net.Listen` call — the existing `libhttp.NewServer(a.Listen, router).Run(ctx)` from Spec 3 keeps owning the server lifecycle.
- Do NOT make the response field names configurable — the JSON shape is the contract. If a future consumer demands variation, that is a separate spec.
- Do NOT make HTTP method routing flexible (e.g. accept both `GET` and `POST` for `/trigger`) — `GET` only. The operation is idempotent at the controller layer; `GET` is the simplest invocation (`curl <url>` with no body).
- Do NOT add a per-task error-count limit, response-size cap, or pagination — the day's task set is ~45 entries (Spec 1's inventory bound); the response stays small.
- Do NOT short-circuit the iteration on first error — every task gets its `Publish` call, errors are collected, and the response carries the full picture.
- Do NOT add new tests under `pkg/schedule/` or `pkg/publisher/` — they are frozen interfaces. Tests live in `pkg/handler/` and use counterfeiter mocks of `Publisher`.
- Do NOT introduce a new metric in this spec — Spec 3 owns `recurring_tasks_published_total{result, recurrence}`; the `/trigger` handler benefits from it transitively (every call to `publisher.Publish` increments the same counter Spec 3 registered), but does not add a new metric of its own. If a future consumer demands a per-trigger histogram, that is a separate spec.
- Do NOT delete or rename any package, file, or symbol outside `pkg/handler/`, `pkg/factory/factory.go`, and `main.go`'s admin-router section.
- Do NOT block on `ctx.Done()` between per-task publishes inside the trigger handler — the iteration is bounded (~45 tasks per day) and each publish is bounded by `publisher.Publish`'s own context discipline. If `ctx.Done()` fires, the in-flight `publisher.Publish` propagates it via the context and returns; the handler captures that as an error in the response.

## Desired Behavior

1. A new Go package `pkg/handler` owns two exported constructors: one returns the `/healthz` JSON handler, one returns the `/trigger` handler. Each follows the Interface → Constructor → Struct pattern where applicable; for stateless handlers a single `http.Handler` returned from `New<Name>Handler` is sufficient (matches Spec 2-removed `NewSentryAlertHandler` shape).
2. `GET /healthz` returns HTTP 200 with `Content-Type: application/json` and body `{"status":"ok"}` (exact bytes, no trailing newline required). It never blocks, never reads from any dependency, and never returns 5xx.
3. `GET /trigger` reads the `date` query parameter, parses it strictly as `YYYY-MM-DD` (Go layout `2006-01-02`), and on parse failure or missing param returns HTTP 400 with `Content-Type: application/json` and body `{"error":"<message>"}` where `<message>` is one of `missing date parameter` (param absent or empty) or `invalid date format, expected YYYY-MM-DD` (param malformed).
4. On valid date, the `/trigger` handler constructs a `schedule.Date` via `schedule.NewDate(t.Year(), t.Month(), t.Day())` from the parsed `time.Time`, calls `schedule.TasksForDate(date)`, then iterates the returned slice and calls `publisher.Publish(ctx, def, date)` for each entry, where `ctx` is the request's `r.Context()`.
5. The handler accumulates per-task results: every successful publish increments the `published` counter; every error appends `{"slug": "<def.Slug>", "error": "<err.Error()>"}` to the `errors` array. The iteration does NOT stop on error.
6. After the iteration completes (or after `ctx.Done()` propagates through a `publisher.Publish` call), the handler responds with HTTP 200, `Content-Type: application/json`, and body `{"date":"YYYY-MM-DD","published":N,"errors":[...]}`. The `date` field echoes the parsed date in `YYYY-MM-DD` format. The `errors` field is always present (never omitted); when empty it serializes as `[]`, never `null`.
7. The `/trigger` handler returns HTTP 200 even when every publish in the day failed — the response body carries the truth (`published: 0`, full `errors` array). The handler returns HTTP 5xx only if it fails to write the response body itself (which the standard library handles implicitly).
8. The handler logs each error at `glog.Errorf` with the slug and ISO date in the message (matches Spec 3's per-task error logging convention; Prometheus counter increments come transitively from `publisher.Publish`).
9. The handler logs a single `glog.V(2).Infof` line per request with the date and the count of tasks processed (entry point trace; default verbosity emits nothing).
10. `pkg/factory/factory.go` exposes two new constructors: `CreateHealthzHandler() http.Handler` (no parameters) and `CreateTriggerHandler(publisher publisher.Publisher) http.Handler`. Both compile and are wired into `main.go`'s admin router, replacing the Spec 3 `libhttp.NewPrintHandler("OK")` call for `/healthz` and adding `/trigger`.
11. `main.go`'s admin router after this spec contains exactly five routes: `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`, `/trigger` — Spec 3's first four plus this spec's `/trigger`. `/healthz` now uses `factory.CreateHealthzHandler()` instead of `libhttp.NewPrintHandler("OK")`.

## Constraints

- Module pre-check: this spec adds no new direct dependencies. `net/http`, `encoding/json`, `time`, and `github.com/golang/glog` are already imported elsewhere in the module. The `gorilla/mux` router (used in Spec 3) is the project's standard; no router swap.
- The handler MUST live in `pkg/handler/` (recreated after Spec 3 deletes the skeleton handler package). The package name is `handler`.
- The handler MUST NOT import `github.com/bborbe/recurring-task-creator/pkg/publisher` directly to take a concrete type; it depends only on the `publisher.Publisher` interface for `/trigger`. Direct import of the package for the interface symbol is required and expected.
- The handler MUST NOT call `time.Now()` directly — the date for `/trigger` comes from the parsed query parameter. (No clock injection needed because no wall-clock read happens.)
- The handler MUST NOT call `schedule.TasksForDate` from a goroutine or buffer the result for later — synchronous in-handler call only. Spec 1's inventory traversal is in-memory and bounded; goroutine fan-out would buy nothing and risks goroutine leaks.
- The handler MUST NOT short-circuit on `ctx.Err()` between per-task publishes — `publisher.Publish` already takes the context and will return promptly when cancelled; the handler captures the cancellation as a per-task error in the response. If the cancellation happens before the response is written, the standard library aborts the write — this is acceptable.
- Date parsing MUST use `time.Parse("2006-01-02", param)`; the parsed `time.Time` MUST be converted to `schedule.NewDate(t.Year(), t.Month(), t.Day())` — civil-date semantics, no timezone read, no `time.Local`, no `time.UTC` involvement in the comparison logic.
- JSON serialization MUST use `encoding/json.Marshal` (or `json.NewEncoder(w).Encode(...)`). The response body for `/trigger` MUST include an empty `errors: []` when no errors occurred — use `json:"errors"` (not `json:",omitempty"`).
- Errors written to the response body MUST be the result of `err.Error()` on the wrapped error from `publisher.Publish`. The handler does not redact or transform the message; if `publisher.Publish` chooses to expose the slug+date in its message (Spec 2 does), that message reaches the response.
- The handler follows the project's standard architecture: Interface → Constructor → Struct → Method (see `~/Documents/workspaces/coding/docs/go-architecture-patterns.md`); stateless handlers may use the `http.HandlerFunc` shorthand returned directly from `New<Name>Handler`, mirroring the now-deleted `NewSentryAlertHandler` precedent.
- Tests follow Ginkgo v2 / Gomega per `~/Documents/workspaces/coding/docs/go-testing-guide.md`. Trigger handler tests use a counterfeiter mock of `publisher.Publisher` from `mocks/` (Spec 2 already generated it).
- Tests use `httptest.NewRecorder()` and `httptest.NewRequest()`; no real HTTP server in tests.
- Logging follows `~/Documents/workspaces/coding/docs/go-logging-guide.md`: never use `glog.Info*` without V-gating; `glog.Errorf` is fine for actual errors.
- Error wrapping follows `~/Documents/workspaces/coding/docs/go-error-wrapping-guide.md`: any error returned by an internal helper (not just exposed through the JSON body) is wrapped via `github.com/bborbe/errors.Wrap`/`Wrapf`.
- HTTP handler organization follows `~/Documents/workspaces/coding/docs/go-http-handler-refactoring-guide.md`: handlers in `pkg/handler/`, factory wiring in `pkg/factory/`, never inline in `main.go`.
- License headers MUST be present on every new `.go` file per `~/Documents/workspaces/coding/docs/go-licensing-guide.md`.
- `make precommit` MUST pass in the changed module.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---|---|---|---|---|---|
| `GET /trigger` with no `date` param | HTTP 400, JSON body `{"error":"missing date parameter"}` | Operator retries with `?date=YYYY-MM-DD` | Response status 400; body contains `missing date parameter` | Reversible (no side effect) | N/A — request is rejected before any publish |
| `GET /trigger?date=2025-13-99` (parse-fail malformed) | HTTP 400, JSON body `{"error":"invalid date format, expected YYYY-MM-DD"}` | Operator retries with a valid date | Response status 400; body contains `invalid date format` | Reversible | N/A |
| `GET /trigger?date=2025-01-04` with empty `TasksForDate` result (no tasks fire that day) | HTTP 200, JSON body `{"date":"2025-01-04","published":0,"errors":[]}` | None needed — this is normal for civil dates that match no predicate | Response status 200; `published: 0`; `errors: []` | N/A | N/A — no publishes occurred |
| `publisher.Publish` returns an error for one task in the iteration | Error appended to `errors[]` with the task's slug; iteration continues; HTTP 200 with the partial summary | Operator inspects `errors[]` and decides whether to retry the request (safe — UUID5 idempotency) | Response body's `errors` array contains an object with the failing slug; `glog.Errorf` line appears in pod logs | Reversible — retry the trigger; controller dedups successful re-publishes | Concurrent triggers for the same date produce identical commands per task; controller dedups |
| `publisher.Publish` returns an error for every task in the iteration | All errors appended to `errors[]`; HTTP 200 with `published: 0` and full `errors` array | Operator triages (e.g. broker down); retries when upstream is healthy | Response status 200 (not 5xx); body's `errors` array length equals the day's task count | Reversible | Same as single-error row |
| Two operators trigger the same date concurrently | Both requests iterate the same task set; each calls `publisher.Publish` for every entry; controller dedups on the deterministic UUID5; one create, one no-op per task per request | None needed — idempotent by design | Both requests return HTTP 200 with `published: N`; controller log shows N creates plus N skips | Idempotent at the vault layer | Safe — `publisher.Publish` holds no state between calls (Spec 2 contract) |
| Client cancels mid-request (TCP reset, browser close) | The in-flight `publisher.Publish` receives the cancelled context and returns promptly; the handler appends the cancellation error to `errors[]`; standard library aborts the response write; no further state side effect | None needed | Standard library logs the write failure at the server level (transparent to handler) | Reversible — operator retries | Safe — request context cancellation is the only shared signal |
| `GET /healthz` while the binary is healthy | HTTP 200, JSON body `{"status":"ok"}` | N/A | Response status 200; body matches the literal string | N/A — read-only handler | N/A — stateless |
| `GET /healthz` during pod startup before `main.go` finishes wiring | Pod's HTTP server not yet listening; client receives connection refused | Wait — pod liveness probe handles this with `initialDelaySeconds: 10` (Spec 5) | `kubectl describe pod` shows probe failures during startup window | Reversible — server eventually starts | N/A |
| `POST /trigger?date=...` (wrong method) | gorilla/mux default: HTTP 405 Method Not Allowed (router-level rejection, no handler invoked) | Operator uses `GET` | Response status 405 | Reversible | N/A |

External unavailability: Kafka broker outage is covered via the per-task error row (response carries `errors[]`, request still returns 200). Schema drift is irrelevant at this layer (no schemas owned). Partial-progress crash: mid-request pod crash drops the connection; the operator's retry replays the day safely (UUID5 idempotency). Rate limiting: not anticipated — operator-driven cadence, not user traffic. Resource exhaustion: response body bounded by `~45 tasks × ~200 bytes` ≈ 10 KiB worst case, no risk. Clock skew: irrelevant — date is operator-supplied, no wall-clock read.

## Security / Abuse Cases

- `/trigger` is intentionally unauthenticated. The Spec 5 `Service` is cluster-internal (ClusterIP) and the Spec 5 Ingress, per its host pattern, may expose `/trigger` to the public internet behind the `quant.benjamin-borbe.de` TLS gateway. The trust boundary is therefore the cluster perimeter plus the TLS gateway's coarse-grained authentication. Even if an external actor reaches `/trigger`, the worst case is a replay of the day's already-deterministic-UUID5 publishes — the controller dedups successful re-creates. Pathological abuse (1000 concurrent requests for the same date) costs Kafka producer capacity but produces zero duplicate vault files.
- The `date` query parameter is the only user-controlled input. Strict `time.Parse("2006-01-02", ...)` rejects everything except a literal `YYYY-MM-DD` token; no SQL, no path traversal, no template injection vector reaches downstream.
- Per-task error messages are exposed verbatim in the response body. Spec 2's `publisher.Publish` does not include secrets in its error messages (it wraps the slug and ISO date plus the underlying Kafka error), so this exposure is bounded to operational metadata. No credential or token can appear in `errors[]`.
- The handler holds no per-request state, opens no files, reads no env vars, makes no outbound network calls except via the injected `publisher.Publisher`. Resource exhaustion via large response bodies is bounded by the inventory size from Spec 1.
- `/healthz` has no input, no state, and a fixed response — zero attack surface.

## Acceptance Criteria

- [ ] `make precommit` exits 0 in the recurring-task-creator module — evidence: exit code 0.
- [ ] `pkg/handler/healthz.go` exists and exports `NewHealthzHandler() http.Handler` — evidence: `grep -nE 'func NewHealthzHandler' pkg/handler/healthz.go` returns exactly one match.
- [ ] `pkg/handler/trigger.go` exists and exports `NewTriggerHandler(publisher.Publisher) http.Handler` — evidence: `grep -nE 'func NewTriggerHandler' pkg/handler/trigger.go` returns exactly one match.
- [ ] `GET /healthz` returns HTTP 200 with `Content-Type: application/json` and body equal to the literal string `{"status":"ok"}` — evidence: Ginkgo test with `httptest.NewRecorder` asserts `rec.Code == 200`, `rec.Header().Get("Content-Type") == "application/json"`, `rec.Body.String() == "{\"status\":\"ok\"}"` (or equivalent after trimming trailing newline if `json.Encoder` emits one — pick one and assert it consistently).
- [ ] `GET /trigger` with no `date` query param returns HTTP 400 with body containing `missing date parameter` — evidence: Ginkgo `Expect(rec.Code).To(Equal(400))` and `Expect(rec.Body.String()).To(ContainSubstring("missing date parameter"))`.
- [ ] `GET /trigger?date=` (empty value) returns HTTP 400 with body containing `missing date parameter` — evidence: same as above with an empty `date` value.
- [ ] `GET /trigger?date=not-a-date` returns HTTP 400 with body containing `invalid date format, expected YYYY-MM-DD` — evidence: Ginkgo `Expect(rec.Code).To(Equal(400))` and `Expect(rec.Body.String()).To(ContainSubstring("invalid date format"))`.
- [ ] `GET /trigger?date=2025-01-32` (day-of-month=32, rejected by `time.Parse("2006-01-02", ...)`) returns HTTP 400 with body containing `invalid date format, expected YYYY-MM-DD` — evidence: Ginkgo `Expect(rec.Code).To(Equal(400))` and `Expect(rec.Body.String()).To(ContainSubstring("invalid date format"))`.
- [ ] `GET /trigger?date=2025-01-04` calls `publisher.Publish` exactly once for each entry returned by `schedule.TasksForDate(schedule.NewDate(2025, time.January, 4))` — evidence: Ginkgo test with a counterfeiter `Publisher` and the real `schedule.TasksForDate` asserts `fakePublisher.PublishCallCount() == len(schedule.TasksForDate(expectedDate))`.
- [ ] `GET /trigger?date=2025-01-04` with all publishes succeeding returns HTTP 200 with JSON body whose `date` field is `"2025-01-04"`, `published` equals the number of tasks, and `errors` is `[]` — evidence: Ginkgo decodes the response body into a map and asserts each field.
- [ ] `GET /trigger?date=2025-01-04` when `publisher.Publish` returns an error for one specific slug returns HTTP 200 with body containing that slug in the `errors` array and `published` equal to `len(tasks) - 1` — evidence: Ginkgo test where `fakePublisher.PublishStub` returns an error for one chosen `def.Slug`; assertion on `errors[0].slug` and `published` count.
- [ ] When `publisher.Publish` returns errors for every task, the response is HTTP 200 (not 5xx) with `published: 0` and `errors` length equal to the day's task count — evidence: Ginkgo `Expect(rec.Code).To(Equal(200))` and `Expect(decoded.Published).To(Equal(0))` and `Expect(decoded.Errors).To(HaveLen(len(tasks)))`.
- [ ] The `/trigger` response always serializes `errors` as `[]` (never `null`) when empty — evidence: Ginkgo asserts the raw response body contains the substring `"errors":[]` (literal, with no whitespace tolerance issues — use `json.Marshal` reference output to compare).
- [ ] The `/trigger` handler logs `glog.Errorf` for each per-task error with the slug substring in the message — evidence: Ginkgo test captures glog output (via `glog.SetOutputBySeverity` or buffer redirection per project test convention) and asserts the slug appears in the error log line. Alternative evidence shape: `grep -nE 'glog\.Errorf' pkg/handler/trigger.go` returns at least one match AND the format string contains `%s` paired with the slug variable name (source inspection).
- [ ] `pkg/factory/factory.go` exposes `CreateHealthzHandler() http.Handler` and `CreateTriggerHandler(publisher.Publisher) http.Handler` — evidence: `grep -nE 'func CreateHealthzHandler|func CreateTriggerHandler' pkg/factory/factory.go` returns exactly two matches.
- [ ] `main.go`'s admin router contains exactly five `router.Path` calls for `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`, `/trigger` — evidence: `grep -nE 'router\.Path' main.go` returns exactly five lines whose path literals match this set (any order).
- [ ] `main.go`'s `/healthz` route uses `factory.CreateHealthzHandler()` (not `libhttp.NewPrintHandler("OK")`) — evidence: `grep -nE '/healthz' main.go` shows the line; the next line (or same line) references `CreateHealthzHandler`; `grep -nE 'NewPrintHandler\("OK"\)' main.go` returns no matches for `/healthz`. (`/readiness` may still use `NewPrintHandler("OK")` — Spec 3 owns that.)
- [ ] `main.go`'s `/trigger` route uses `factory.CreateTriggerHandler(...)` with the publisher built from Spec 3's wire graph — evidence: `grep -nE '/trigger' main.go` shows the line; `grep -nE 'CreateTriggerHandler' main.go` returns at least one match.
- [ ] `pkg/handler/` package does NOT import `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/bborbe/jira-task-creator/...`, and does NOT call `time.Now()` directly — evidence: `grep -E '"(github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/handler/*.go` returns no matches.
- [ ] `pkg/handler/trigger.go` imports `github.com/bborbe/recurring-task-creator/pkg/schedule` and `github.com/bborbe/recurring-task-creator/pkg/publisher` — evidence: `grep -nE '"github\.com/bborbe/recurring-task-creator/pkg/(schedule|publisher)"' pkg/handler/trigger.go` returns exactly two matches.
- [ ] Trigger handler tests use the existing counterfeiter mock at `mocks/publisher-publisher.go` (generated by Spec 2) — evidence: `grep -nE 'PublisherPublisher' pkg/handler/trigger_test.go` returns at least one match.
- [ ] No scenario test added — covered by unit tests; see scenario rule below.

Scenario coverage: NO new scenario. The handlers are pure in-process HTTP-to-publisher adapters. Unit tests with `httptest.NewRecorder` + counterfeiter `Publisher` + real `schedule.TasksForDate` reach every behavior: status codes, response body shapes, error accumulation, per-task call count, JSON serialization of empty arrays, logging side effects. No real Kafka, no real cluster, no real HTTP server. The integration test that exercises a real `/trigger` request against a running pod with real Kafka is deferred to the Spec 5 post-merge runbook smoke check, where the operator runs `curl https://recurring-task-creator.dev.quant.benjamin-borbe.de/trigger?date=$(date -I)` and observes the JSON summary plus the resulting vault files.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-mvp
make precommit
```

Expected: exit code 0, all tests green, lint clean, license headers present, no forbidden imports.

Manual smoke check (NOT part of this spec's verification; documented for the prompt-writer):

```
cd ~/Documents/workspaces/recurring-task-creator-mvp
go run . -v=2 -listen=:9090 -kafka-brokers=... &
curl -s http://localhost:9090/healthz
# Expected: {"status":"ok"}
curl -s 'http://localhost:9090/trigger?date=2025-01-04' | jq .
# Expected: {"date":"2025-01-04","published":N,"errors":[]}
```

## Suggested Decomposition

Single-layer spec (one new package with two handlers, one factory edit, one main.go router edit). DB count = 11, AC count = 22; DB × AC = 242 by raw count, but every AC except the factory and main.go wiring lives on the same two handler functions. The `/healthz` handler is three lines of code; the `/trigger` handler is the substantive change, and its tests share the same Ginkgo bootstrap as the `/healthz` tests in the same package. Splitting along `/healthz` vs `/trigger` would create artificial seams that re-converge in `main.go` on the next prompt.

Recommend a single prompt. If the executor insists on splitting:

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `pkg/handler` package: `NewHealthzHandler` + `NewTriggerHandler` + tests with counterfeiter `Publisher` and real `schedule.TasksForDate`; covers all handler-level behavior | 1, 2, 3, 4, 5, 6, 7, 8, 9 | healthz status/body, trigger param parsing, missing/invalid date, fan-out publish, error accumulation, JSON shape (`errors:[]`), logging, no-forbidden-imports, schedule/publisher imports, mock-used | — |
| 2 | `pkg/factory` constructors + `main.go` router rewiring: add `CreateHealthzHandler`, `CreateTriggerHandler`, swap `/healthz` to use the new handler, add `/trigger` route; factory test | 10, 11 | factory constructors exist, router has five paths, `/healthz` uses new handler, `/trigger` route wired, `make precommit` passes | prompt 1 |

Rationale: prompt 1 lands the handlers' contracts in isolation; prompt 2 plugs them into the binary's wire graph. No cycles. Default is still single-prompt.

## Do-Nothing Option

Without this spec, the binary's only publish pathway is the hourly tick from Spec 3. Operationally that means "we know how to make tasks fire, but we cannot replay a missed day." Spec 5's STS liveness probe targets `/healthz` and accepts the Spec 3 text response (`OK`) — strictly speaking, the cluster works without the JSON change. But the moment an external monitor or a future blackbox-exporter probe wants a parseable response, the lack of a JSON field becomes a refactor with downstream consumers. And without `/trigger`, the operator's only recourse when an hourly tick fails for an entire day (e.g. a six-hour Kafka outage that happened to straddle every tick) is to wait for the next civil-date roll-over and lose that day's task creations forever — the controller's de-dup only protects against duplicate creates, not against missed creates. Folding this into Spec 5 (manifests) would mean shipping Go code in a YAML diff, which obscures both reviews. Folding it into Spec 3 (tick) would re-tangle the cron loop's contract with the HTTP surface. Recommendation: do this spec immediately after Spec 3 lands, before Spec 5's deploy.
