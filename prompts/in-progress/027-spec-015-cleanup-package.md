---
status: approved
spec: [015-recurring-task-cleanup-cron]
created: "2026-06-29T19:45:00Z"
queued: "2026-06-29T19:34:17Z"
branch: dark-factory/recurring-task-cleanup-cron
---

<summary>
- Introduces a new internal package that decides which prior recurring-task instances should be auto-closed and performs the close-out against the vault.
- Computes the prior period's token for any of the six recurrence kinds, reusing the existing period-token formula so the lookup is deterministic and matches what the publisher produced.
- Reads vault files and, only when the next period's instance already exists, rewrites the prior instance's frontmatter to mark it aborted/done with a supersede marker.
- Schedules flagged "preserve every missed instance" are skipped entirely — no vault read, no write.
- Re-running on the same data is a no-op: an already-aborted file is left alone.
- Concurrent vault edits that cause a write conflict are tolerated (logged, counted, retried next tick) — never an overwrite, never a crash.
- A single Prometheus counter records every close-out attempt by recurrence kind and outcome (success / conflict / error).
- No binary, no cron loop, no Kubernetes manifest in this prompt — only the package and its tests with generated mocks.
</summary>

<objective>
Build a new `pkg/cleanup` package: a `PriorPeriodToken` pure function (reusing `publisher.PeriodTokenBuilder`), `VaultReader` / `VaultWriter` interfaces, a `Metrics` interface with a Prometheus-backed implementation, and a `Supersedance` orchestrator whose `Run(ctx, date)` iterates Schedules and auto-aborts prior in-progress instances under the spec's safety and idempotency rules. Everything is tested with counterfeiter mocks; no binary and no manifest yet.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This prompt DEPENDS ON prompt 1 (`1-spec-015-crd-adapter-frontmatter.md`) having landed. Verify before starting:

```bash
grep -q 'SkipAutoCleanup bool' /workspace/pkg/schedule/task_definition.go
```

If that exits non-zero, prompt 1 has not landed — STOP and report `status: failed` with summary "prompt 1 (TaskDefinition.SkipAutoCleanup) not yet deployed".

Read these files fully before writing the new package:
- `/workspace/pkg/publisher/period_token.go` — the `PeriodToken` named string type, the `PeriodTokenBuilder` interface (`Build(ctx, def schedule.TaskDefinition, date schedule.Date) (PeriodToken, error)`), `NewPeriodTokenBuilder()`, and the per-kind formula switch. The decrementor REUSES this; it does not re-implement formatting. Study how each kind shifts the date: Monthly uses `base.AddDate(0, def.PeriodOffset, 0)`, Quarterly `base.AddDate(0, def.PeriodOffset*3, 0)`, Yearly `base.AddDate(def.PeriodOffset, 0, 0)`, and Weekday derives its suffix from the firing weekday (must be in `def.Weekdays`).
- `/workspace/pkg/schedule/date.go` — `schedule.Date{Year, Month, Day}`, `NewDate`, `Date.Time()` (midnight-UTC carrier for stdlib arithmetic). The decrementor shifts a `Date` and feeds it back into `Build`.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` closed enum and `AllRecurrenceKinds` (range over this for metric pre-init; never hand-roll a duplicate slice).
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition` (now with `SkipAutoCleanup bool` from prompt 1), `Slug`, `Recurrence`, `Weekdays`, `PeriodOffset`.
- `/workspace/pkg/store/store.go` — `ScheduleStore` interface (`List(ctx) ([]schedule.TaskDefinition, error)`). `Supersedance` consumes a `ScheduleStore`.
- `/workspace/pkg/tick/metrics.go` — the EXACT pattern for the Prometheus metrics package: a `Metrics` interface, `NewPrometheusMetrics()`, a package-level `prometheus.NewCounterVec`, an `init()` that `MustRegister`s and pre-initializes every label combination by ranging `schedule.AllRecurrenceKinds`, and the `//counterfeiter:generate -o ../../mocks/...` directive. Mirror this file structure exactly for `pkg/cleanup/metrics.go`.
- `/workspace/pkg/tick/tick.go` — for the orchestrator's overall shape (struct with injected deps, `Run` method, context-cancellation handling, glog usage). Read it to match the package's idioms.
- `/workspace/pkg/store/store_export_test.go` and `/workspace/pkg/publisher/export_test.go` — the `*ForTest` export pattern, if cross-package test access to an unexported symbol is needed.
- `/workspace/mocks/tick-metrics.go` and `/workspace/mocks/store-store.go` — examples of generated counterfeiter mocks and their naming (`--fake-name`, output to `../../mocks/`).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public interface + private struct + `New*` constructor; counterfeiter annotations on all interfaces.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-prometheus-metrics-guide.md` — interface-based metrics, init() registration, naming, bounded labels.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` 3-arg wrapping; sentinel errors with `stderrors` alias if needed.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable, counterfeiter mocks.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter generation.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — non-blocking select context check in the per-Schedule loop.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — CHANGELOG entry format.

YAML frontmatter parsing: this repo already depends on a YAML library transitively. Before writing a YAML import, grep the vendored/imported set: `grep -rn 'gopkg.in/yaml\|sigs.k8s.io/yaml' /workspace/go.mod /workspace/pkg`. Use whichever is already a direct dependency; if none is direct, prefer `sigs.k8s.io/yaml` (already present via k8s deps) for round-tripping frontmatter maps. Confirm the exact import path resolves with `go build ./pkg/cleanup/...` before finalizing.
</context>

<requirements>

Create a new package `pkg/cleanup`. Every new `.go` file starts with the standard 3-line BSD copyright header (copy it verbatim from any existing file, e.g. `pkg/tick/tick.go`).

### 1. `pkg/cleanup/period_token_decrementor.go` — `PriorPeriodToken`

A pure function (no clock, no I/O) that, given a `TaskDefinition` and a current `schedule.Date`, returns the prior period's `publisher.PeriodToken` using the existing `publisher.PeriodTokenBuilder`.

Signature:

```go
// PriorPeriodToken returns the period-anchored token of the period
// immediately before currentDate for def's recurrence kind, computed by
// shifting the date one period back and re-invoking the existing
// publisher.PeriodTokenBuilder. Deterministic: the same (def, currentDate)
// always yields the same token, matching the title/UUID5 the publisher
// produced for that prior instance.
func PriorPeriodToken(
	ctx context.Context,
	builder publisher.PeriodTokenBuilder,
	def schedule.TaskDefinition,
	currentDate schedule.Date,
) (publisher.PeriodToken, error)
```

Decrement rules (per the spec's Period-token decrement table) — shift the date, then call `builder.Build(ctx, def, priorDate)`:

| Kind | Date shift to compute priorDate |
|------|---------------------------------|
| `RecurrenceDaily` | `currentDate - 1 day` |
| `RecurrenceWeekday` | the most recent date on-or-before `currentDate - 1 day` whose weekday is in `def.Weekdays` (walk back day-by-day, max 7 iterations, until `weekdayInSet`; error if none found in 7 days — that means an empty `Weekdays`) |
| `RecurrenceWeekly` | `currentDate - 7 days` |
| `RecurrenceMonthly` | `currentDate.Time().AddDate(0, -1, 0)` |
| `RecurrenceQuarterly` | `currentDate.Time().AddDate(0, -3, 0)` |
| `RecurrenceYearly` | `currentDate.Time().AddDate(-1, 0, 0)` |
| unknown kind | wrapped error via `errors.Errorf(ctx, ...)` naming the kind |

Implementation notes:
- Convert `schedule.Date` to `time.Time` via `currentDate.Time()`, do the `AddDate`/day-walk, then convert back to a `schedule.Date` with `schedule.NewDate(t.Year(), t.Month(), t.Day())`.
- For Weekday, the prior firing date may be earlier in the same ISO week or a prior week — the builder's Weekday formula reads the weekday off the shifted date and produces `YYYYWNN-<abbrev>`, so feeding it the correct prior firing date is sufficient. Do NOT re-implement the abbreviation.
- `PeriodOffset` symmetry is automatic: the builder applies `def.PeriodOffset` to whatever date it receives, so a Monthly schedule with `periodOffset=-1` naturally gets the prior offset token when fed the prior-month date.
- All errors wrapped with `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` — never `fmt.Errorf`, never `context.Background()`.

### 2. `pkg/cleanup/vault.go` — `VaultReader` / `VaultWriter` interfaces

```go
//counterfeiter:generate -o ../../mocks/cleanup-vault-reader.go --fake-name CleanupVaultReader . VaultReader

// VaultReader reads vault files via git-rest (HTTP). Read-only.
type VaultReader interface {
	// GetFile returns the raw bytes of the file at path. Returns a
	// wrapped error on transport failure, a not-found sentinel/error on
	// 404. path is "<vault-relative-dir>/<title> - <token>.md".
	GetFile(ctx context.Context, path string) ([]byte, error)

	// ListFiles returns the relative paths of every vault file whose name
	// begins with prefix. prefix is the slug-derived directory or title
	// stem used to detect whether the next-period instance exists.
	ListFiles(ctx context.Context, prefix string) ([]string, error)
}

//counterfeiter:generate -o ../../mocks/cleanup-vault-writer.go --fake-name CleanupVaultWriter . VaultWriter

// VaultWriter performs merge-aware writes via git-rest. The mutator is
// invoked against the CURRENT file bytes (re-read inside the writer so a
// vault-cli mid-edit is not clobbered); the writer POSTs the mutated
// result back. A 409 from git-rest (file changed between read and write)
// is surfaced as an error the caller classifies as a conflict.
type VaultWriter interface {
	// UpdateFile reads the file at path, applies mutator to its current
	// bytes, and writes the result back. Returns a 409-classified error
	// on a write conflict, a generic wrapped error otherwise, nil on success.
	UpdateFile(ctx context.Context, path string, mutator func([]byte) ([]byte, error)) error
}
```

Add a way for the orchestrator to classify a 409. Define a sentinel error in this file and a classifier:

```go
// ErrVaultConflict is returned (wrapped) by a VaultWriter when git-rest
// responds 409 because the file changed between read and write. The
// orchestrator classifies it as result="conflict" and defers to the next
// tick. Use bborbe/errors with a stderrors alias per the error-wrapping guide.
var ErrVaultConflict = stderrors.New("vault write conflict (git-rest 409)")
```

(The git-rest HTTP implementation of these interfaces is built in prompt 3 — this prompt defines the interfaces and the sentinel only. Do NOT implement an HTTP client here.)

### 3. `pkg/cleanup/metrics.go` — `Metrics` interface + Prometheus impl

Mirror `/workspace/pkg/tick/metrics.go` exactly in structure:

```go
//counterfeiter:generate -o ../../mocks/cleanup-metrics.go --fake-name CleanupMetrics . Metrics

// Metrics records cleanup-cron supersede outcomes.
type Metrics interface {
	// IncSuperseded is called once per file the cron attempts to supersede.
	// result is one of "success", "conflict", "error"; recurrence is the
	// kind string.
	IncSuperseded(result string, recurrence string)
}
```

- Package-level `recurringTaskCleanupSupersededTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "recurring_task_cleanup_superseded_total", Help: "..."}, []string{"recurrence", "result"})`.
- `init()` runs `prometheus.MustRegister(...)` then pre-initializes EVERY combination of `schedule.AllRecurrenceKinds` × `{"success", "conflict", "error"}` with `.Add(0)` so scrapes see the series before the first event.
- `NewPrometheusMetrics() Metrics` returns the wrapper struct.
- Do NOT add a "last cleanup tick" gauge — the spec forbids new gauges (reuse the publisher's existing gauge if ever needed; not needed here). Exactly ONE new counter.

### 4. `pkg/cleanup/supersedance.go` — `Supersedance` orchestrator

```go
// Supersedance auto-aborts prior-period recurring-task instances left
// `status: in_progress` once the next period's instance has materialized.
type Supersedance struct {
	Store        store.ScheduleStore
	TokenBuilder publisher.PeriodTokenBuilder
	Reader       VaultReader
	Writer       VaultWriter
	Metrics      Metrics
	Clock        libtime.CurrentDateTimeGetter
}

// Run iterates every Schedule for the given Berlin civil date, skips those
// with SkipAutoCleanup == true, computes each Schedule's prior-period
// token, and supersedes the matching prior in-progress file when the
// next-period instance already exists. A per-Schedule failure is logged
// and counted (result="error"); it never aborts the whole tick.
func (s *Supersedance) Run(ctx context.Context, date schedule.Date) error
```

Algorithm per Schedule `def`:
1. Non-blocking context-cancellation check at the top of the loop (per `go-context-cancellation-in-loops.md`); return wrapped ctx error if cancelled.
2. If `def.SkipAutoCleanup`, log at V(3) `cleanup: skipping <slug>: skipAutoCleanup=true` and `continue` (no reader call, no counter increment).
3. Compute `currentToken, err := s.TokenBuilder.Build(ctx, def, date)`. On error: log, `s.Metrics.IncSuperseded("error", string(def.Recurrence))`, continue.
4. Compute `priorToken, err := PriorPeriodToken(ctx, s.TokenBuilder, def, date)`. On error: log, count error, continue.
5. Derive the prior file path and the next-period file path from the slug + tokens. The vault filename convention is `<title> - <token>.md` under the slug's vault directory; for the lookup, use the deterministic stem `<def.Slug> - <token>` (the spec's path-safety section pins this). List candidate files via `s.Reader.ListFiles(ctx, <prefix>)` where prefix is the slug stem; from the returned paths determine: (a) whether the prior-token file exists, (b) whether the current-token (next-period) file exists.
   - Read the spec's AC examples for the exact `ListFiles`/`GetFile` argument shapes the tests assert (the test for "next-period exists" stubs `ListFiles("24 Tasks")` returning `["<slug> - 2026-06-28.md", "<slug> - 2026-06-29.md"]` and `GetFile("<slug> - 2026-06-28.md")`). Match those argument shapes so the tests in requirement 6 pass.
6. If the prior-token file does NOT exist: skip (first-ever instance / pre-cutover). Log V(3), no counter, continue.
7. **Safety gate**: if the next-period (current-token) file does NOT exist AND the prior file's `planned_date` is within one cadence of `date` (i.e. the prior period IS effectively the current open period), skip — never abort the current period. Log V(3), no write, continue. (Compute "one cadence" from `def.Recurrence`: Daily=1 day, Weekly/Weekday=7 days, Monthly≈1 month, Quarterly≈3 months, Yearly≈1 year. Parse `planned_date` from the prior file's frontmatter; if absent or unparseable, treat the gate conservatively — see requirement 5.)
8. Read the prior file via `s.Reader.GetFile(ctx, priorPath)`. Parse its YAML frontmatter. If `status != "in_progress"` (e.g. already `aborted`): skip — idempotency (no write, no counter). If frontmatter parse fails: log, count `error`, continue (do NOT write).
9. Otherwise call `s.Writer.UpdateFile(ctx, priorPath, mutator)` where `mutator` re-parses the current bytes, sets `status: aborted`, `phase: done`, `completed_date: <now-ish marker>`, appends `superseded_by: auto-cleanup-<unix-ts>`, and re-serializes. Classify the result:
   - nil → `s.Metrics.IncSuperseded("success", kind)`.
   - error wrapping `ErrVaultConflict` (use `errors.Is(err, ErrVaultConflict)`) → log `git-rest conflict, will retry next tick`, `s.Metrics.IncSuperseded("conflict", kind)`, continue (no panic, no abort).
   - any other error → log, `s.Metrics.IncSuperseded("error", kind)`, continue.

### 5. Frontmatter parsing helper (in `supersedance.go` or a small `frontmatter.go` in the package)

A helper to extract YAML frontmatter (the `---`-delimited block at the top of a markdown file) into a `map[string]interface{}`, and a helper to re-serialize a mutated map back into the file bytes preserving the markdown body below the frontmatter. Use the YAML lib confirmed in `<context>`. The supersede mutator:
- sets `status: aborted`, `phase: done`,
- sets `completed_date` to a deterministic marker derived from the supersede timestamp (the timestamp comes from the writer/orchestrator — do NOT call `time.Now()` directly in business logic; thread a `libtime.CurrentDateTimeGetter` into `Supersedance` if a wall-clock is needed, mirroring how `tick.Tick` injects `clock`. Read `pkg/tick/tick.go` to confirm the injection pattern, and add a `Clock libtime.CurrentDateTimeGetter` field to `Supersedance` for the `superseded_by: auto-cleanup-<unix-ts>` suffix and `completed_date`).
- appends `superseded_by: auto-cleanup-<unix-ts>`.
On malformed YAML in the mutator, return a wrapped error so `UpdateFile`'s caller counts `error` and writes nothing.

### 6. `pkg/cleanup/supersedance_test.go` + `pkg/cleanup/period_token_decrementor_test.go` + `pkg/cleanup/metrics_test.go` + `pkg/cleanup/cleanup_suite_test.go`

Ginkgo v2 suite (external `cleanup_test` package; add a `pkg_export_test.go` for any unexported helper a test needs). Use the generated counterfeiter mocks (`mocks.CleanupVaultReader`, `mocks.CleanupVaultWriter`, `mocks.CleanupMetrics`) and the existing `mocks.FakeScheduleStore` for the store. Inject a fixed clock via `libtime` (`SetNow`) per `go-time-injection.md`.

Cover every spec Acceptance Criterion:
- **`PriorPeriodToken` table test** (`-run 'PriorPeriodToken'`): all 6 kinds × 3 representative `(today, def)` triples = 18 rows, asserting the prior token matches the spec's decrement table. Build the real `publisher.NewPeriodTokenBuilder()` (no mock — assert real formatted tokens like `2026-06-28`, `2026W26`, `2026W27-sat`, `2026-05`, `2026Q1`, `2025`).
- **skip on `SkipAutoCleanup == true`** (`-run 'Supersedance.*SkipAutoCleanup'`): store returns a def with `SkipAutoCleanup: true`; assert `reader.ListFilesCallCount() == 0` and `metrics.IncSupersededCallCount() == 0`.
- **supersede when next-period exists** (`-run 'Supersedance.*next-period-exists'`): Daily def, `ListFiles` returns prior+current files, `GetFile` returns `status: in_progress` bytes; assert `writer.UpdateFileCallCount() == 1`; invoke the captured mutator against the input bytes and assert the result has `status: aborted`, `phase: done`, `completed_date`, and a `superseded_by: auto-cleanup-` prefix; assert `metrics.IncSuperseded("success", "daily")` recorded.
- **skip when next-period absent AND within firing window** (`-run 'Supersedance.*firing-window'`): assert `writer.UpdateFileCallCount() == 0`.
- **idempotent on re-run** (`-run 'Supersedance.*idempotent'`): first run writes once; second run's `GetFile` returns the post-write `status: aborted` bytes; assert second-run `UpdateFileCallCount` delta == 0.
- **first-ever instance no-op** (`-run 'Supersedance.*first-ever'`): `ListFiles` does not contain the prior-token file; assert zero writes.
- **409 conflict tolerated** (`-run 'Supersedance.*conflict'`): writer returns an error wrapping `cleanup.ErrVaultConflict`; assert no panic, `metrics.IncSuperseded("conflict", ...)` recorded, tick completes.
- **generic writer error** (`-run 'Supersedance.*error'`): writer returns a plain error; assert `metrics.IncSuperseded("error", ...)` recorded and the loop continues to the next Schedule (use a 2-Schedule store and assert the second is still processed).
- **frontmatter parse failure**: malformed YAML from `GetFile`; assert `error` counted, no write.
- **metrics_test**: assert `IncSuperseded` increments the labelled counter (mirror `pkg/tick/metrics_test.go`).

### 7. Regenerate mocks

Run `go generate ./...` so the three new counterfeiter directives produce `mocks/cleanup-vault-reader.go`, `mocks/cleanup-vault-writer.go`, `mocks/cleanup-metrics.go`. Commit them (the agent does not commit; just leave them on disk for dark-factory). Confirm:

```bash
grep -l 'counterfeiter:generate' pkg/cleanup/*.go | wc -l   # >= 3
ls mocks/cleanup-*.go | wc -l                                 # >= 3
```

### 8. CHANGELOG

Append under `## Unreleased`:

```
- feat: Add `pkg/cleanup` package — `Supersedance` orchestrator auto-aborts prior in-progress recurring-task instances once the next period materializes; `PriorPeriodToken` decrementor; `recurring_task_cleanup_superseded_total` counter.
```

</requirements>

<constraints>
- **Reuse `publisher.PeriodTokenBuilder` verbatim** for token formatting — the decrementor only computes the prior `Date` and re-invokes `Build`. Do NOT re-implement token formatting, the weekday abbreviation, or the `PeriodOffset` math.
- **No Kafka, no direct git CLI, no `os.WriteFile`, no in-process git library** in this package. All vault access is through the `VaultReader` / `VaultWriter` interfaces (HTTP impl lands in prompt 3).
- **Exactly ONE new Prometheus counter** (`recurring_task_cleanup_superseded_total{recurrence, result}`). No gauges, no histograms.
- **No `time.Now()` / `context.Background()` in business logic** — inject `libtime.CurrentDateTimeGetter` for the supersede timestamp (mirror `tick.Tick`).
- **Safety property (never abort the current period)**: a prior file is never superseded when the next-period file does NOT exist AND the prior is within one cadence of `date`.
- **Idempotency**: an already-`aborted` file is skipped — no second write.
- **Closed label cardinality**: counter labels are bounded to `AllRecurrenceKinds` × `{success, conflict, error}` — never attacker-controlled.
- Project DoD applies (`/workspace/docs/dod.md`): `bborbe/errors` 3-arg wrapping; Ginkgo v2 / Gomega; GoDoc on exports; counterfeiter (never manual) mocks; coverage ≥80% for the new package; error paths tested.
- Do NOT build a binary or a STS manifest in this prompt (that is prompt 3). Do NOT wire anything into `cmd/` or `pkg/factory/`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && go generate ./...
cd /workspace && go build ./pkg/cleanup/...
cd /workspace && make test
```

Targeted:

```bash
cd /workspace && go test -v -run 'PriorPeriodToken|Supersedance' ./pkg/cleanup/...
cd /workspace && go test -cover ./pkg/cleanup/...
cd /workspace && grep -l 'counterfeiter:generate' pkg/cleanup/*.go | wc -l
cd /workspace && ls mocks/cleanup-*.go | wc -l
```

Confirm:
- All 18 `PriorPeriodToken` rows PASS.
- All `Supersedance` ACs PASS (skip / supersede / firing-window / idempotent / first-ever / conflict / error / parse-failure).
- Coverage ≥80% for `pkg/cleanup`.
- 3+ counterfeiter directives and 3+ generated mock files.

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.
</verification>

<completion>
Append after implementation:

```
DARK-FACTORY-REPORT
{
  "status": "success|partial|failed",
  "summary": "<one line>",
  "verification": {"command": "make precommit", "exitCode": 0}
}
```

`"status":"success"` ONLY if `make precommit` exited 0.

## Improvements

- (fill in per the reflection rules; write `- None` if nothing)
</completion>
