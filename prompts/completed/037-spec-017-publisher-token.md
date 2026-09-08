---
status: completed
spec: [017-hourly-recurrence-kind]
summary: 'Added RecurrenceHourly to the publisher: fmtHourToken builds the compact civil-hour token YYYYMMDDHH (no PeriodOffset), deferDateFor names Hourly among point-shaped kinds, and six new specs cover token format, hour distinctness, identifier distinctness, the wire-validate contract, and defer_date'
execution_id: recurring-task-creator-hourly-exec-037-spec-017-publisher-token
dark-factory-version: dev
created: "2026-09-07T21:07:00Z"
queued: "2026-09-07T21:31:23Z"
started: "2026-09-07T21:41:58Z"
completed: "2026-09-07T21:47:56Z"
branch: dark-factory/hourly-recurrence-kind
---

<summary>
- The publisher's period-token builder now knows the `hourly` kind: its token is the compact civil hour `YYYYMMDDHH`.
- Each civil hour therefore yields a distinct token, a distinct UUID5 identifier, and a distinct materialized task file.
- `PeriodOffset` is not applied to hourly, matching the CRD rule that keeps offset valid only for monthly/quarterly/yearly.
- Hourly task frontmatter stamps `defer_date` as the fire date, consistent with the other point-shaped kinds (the closed-set switch names Hourly explicitly).
- New Ginkgo specs prove the token format, that hour 12 vs hour 13 on the same day differ (different identifiers), that a date-only construction yields the hour-00 token, and that the hourly command passes the wire validation contract like every other kind.
- No existing kind's period-token format or code path changes.
</summary>

<objective>
Make the publisher's period-token builder return an hour-granular `YYYYMMDDHH` token for `RecurrenceHourly` (no `PeriodOffset`), so each civil hour of the Europe/Berlin clock produces a distinct period token → distinct UUID5 identifier → distinct task file, and close the publish-path and frontmatter test gaps for the new kind.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 3 of 4 for spec 017. It DEPENDS ON prompt 1 having landed: `schedule.RecurrenceHourly` must exist. Guard: if `grep -qn 'RecurrenceHourly' /workspace/pkg/schedule/recurrence.go` returns no match (exit non-zero), prompt 1 has NOT landed — STOP and report `status: failed` with summary "schedule core (prompt 1) not yet deployed — RecurrenceHourly undefined". This prompt is INDEPENDENT of prompt 2 (tick).

Read these files fully before changing anything:
- `/workspace/pkg/publisher/period_token.go` — the `periodTokenBuilder.Build(ctx, def, date)` method with the `switch def.Recurrence` you extend, and the `PeriodToken` doc comment (lines ~50-56) that enumerates the token formats. The `default:` returns an "unknown recurrence kind" error — `hourly` must be handled BEFORE it. `date.Hour` is the exported field on `schedule.Date` (prompt 1).
- `/workspace/pkg/publisher/render.go` — the leaf format helpers (`fmtDate`, `fmtIsoWeek`, `fmtMonthYear`, `fmtQuarter`, `fmtYear`). You add one compact helper here.
- `/workspace/pkg/publisher/frontmatter.go` — `deferDateFor(recurrence, date)` (line ~107) with the point-shaped case group `case schedule.RecurrenceDaily, schedule.RecurrenceWeekday, schedule.RecurrenceOnDate:` and its GoDoc. You add `RecurrenceHourly` to that group and update the GoDoc's "point-shaped kinds" enumeration.
- `/workspace/pkg/publisher/period_token_test.go` — `package publisher_test`, the `build := func(def schedule.TaskDefinition, date schedule.Date) publisher.PeriodToken` helper at the top. Add the hourly specs here using `schedule.Date{...}` literals with `Hour` set.
- `/workspace/pkg/publisher/publisher_test.go` — `package publisher_test`. Note the `Describe("boundary contract")` → `DescribeTable("produced command passes task.CreateCommand.Validate", ...)` block (line ~1200, one `Entry(...)` per kind ending at `Entry("ondate", schedule.RecurrenceOnDate)`) and the `publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder())` usage (line ~370).
- `/workspace/pkg/publisher/frontmatter_test.go` — the `Describe("defer_date stamp")` → `DescribeTable("stamps the period-start date per recurrence kind", ...)` (line ~110, one `Entry(...)` per kind).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` (only if you touch an error path; this prompt does not add one).
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format.
</context>

<requirements>

### 1. Add the `RecurrenceHourly` case to `Build`

In `/workspace/pkg/publisher/period_token.go`, in `periodTokenBuilder.Build`, add a case for `schedule.RecurrenceHourly` after the `case schedule.RecurrenceDaily:` and before `case schedule.RecurrenceWeekly:` (cadence order), returning the compact hour-granular token via a new `fmtHourToken` helper:

```go
	case schedule.RecurrenceDaily:
		return PeriodToken(fmtDate(date.Year, int(date.Month), date.Day)), nil
	case schedule.RecurrenceHourly:
		// Hourly fires every civil hour; its period token is the compact
		// civil hour "YYYYMMDDHH", so each civil hour produces a distinct
		// UUID5 identifier and task file. PeriodOffset is NOT applied
		// (the CRD CEL rule keeps periodOffset valid only for
		// Monthly/Quarterly/Yearly). date.Hour is 0 for date-only
		// construction sites (e.g. the /trigger handler), which yields
		// the hour-00 token of that day.
		return PeriodToken(fmtHourToken(date.Year, int(date.Month), date.Day, date.Hour)), nil
```

Read `date.Year`, `int(date.Month)`, `date.Day`, and `date.Hour` directly from the civil `schedule.Date` — do NOT go through `date.Time()` (which is midnight UTC and ignores `Hour`), and do NOT apply `def.PeriodOffset`. Do NOT change any existing case.

### 2. Add the `fmtHourToken` helper

In `/workspace/pkg/publisher/render.go`, add the helper next to `fmtDate`:

```go
// fmtHourToken renders YYYYMMDDHH (compact, no separators) — the hourly
// period token. Each civil hour yields a distinct token, hence a distinct
// UUID5 identifier and task file.
func fmtHourToken(year, month, day, hour int) string {
	return fmt.Sprintf("%04d%02d%02d%02d", year, month, day, hour)
}
```

`fmt` is already imported in render.go.

### 3. Document the hourly format on the `PeriodToken` type (pure insertion — AC 8 safety)

In `/workspace/pkg/publisher/period_token.go`, the `PeriodToken` doc comment enumerates the token formats. Update it to mention the hourly format by INSERTING a new `//` line — do NOT modify or rewrite any existing line in this file, because spec AC 8 asserts `git diff pkg/publisher/period_token.go | grep -c '^-[^-]'` returns `0` (no removed lines anywhere in this file). The current comment reads:

```go
// PeriodToken is the period-anchored token string appended to a recurring
// task's title and fed into the UUID5 identifier — "YYYY-MM-DD" for daily,
// "YYYYWNN" for weekly, "YYYYWNN-<3-letter-weekday>" for weekday, "YYYY-MM"
// for monthly, "YYYYQN" for quarterly, "YYYY" for yearly. Wrapped in a
// named string type so calls that take both a slug and a token can't accept
// them in the wrong order without a compile error.
```

Insert these two new comment lines between the existing `// "YYYY" for yearly. Wrapped in a` line and the existing `// named string type ...` line, editing neither existing line:

```go
// "YYYYQN" for quarterly, "YYYY" for yearly, and "YYYYMMDDHH" for the
// hourly kind (one distinct token per civil hour).
```

(Adjust the exact line wrapping as gofmt requires, but the change must remain a pure insertion of new comment lines between the two untouched existing lines — never an edit-in-place of an existing line.)

### 4. Name Hourly in the point-shaped `deferDateFor` switch

In `/workspace/pkg/publisher/frontmatter.go`, add `schedule.RecurrenceHourly` to the point-shaped case group in `deferDateFor`:

```go
	case schedule.RecurrenceDaily, schedule.RecurrenceWeekday, schedule.RecurrenceOnDate, schedule.RecurrenceHourly:
		return fmtDate(date.Year, int(date.Month), date.Day)
```

The `default:` branch would have returned the same fire date, but Hourly is a point-shaped kind and must be named explicitly in this closed-set switch. Update the `deferDateFor` GoDoc (and the `FrontmatterFormatter.Format` interface GoDoc) where they enumerate "Point-shaped kinds (Daily, Weekday, OnDate)" to include Hourly. `defer_date` for an hourly task is the fire date (`YYYY-MM-DD`) — the hour itself is not part of the `defer_date` ISO string.

### 5. Add token-format and distinctness specs to `period_token_test.go`

In `/workspace/pkg/publisher/period_token_test.go`, add a `Describe("RecurrenceHourly", ...)` block using the existing `build` helper (it accepts a `schedule.Date`):

```go
	Describe("RecurrenceHourly", func() {
		It("produces the compact hour-granular token YYYYMMDDHH", func() {
			def := schedule.TaskDefinition{
				Slug:       "hourly-build-check",
				Recurrence: schedule.RecurrenceHourly,
			}
			tok := build(def, schedule.Date{Year: 2026, Month: time.September, Day: 7, Hour: 13})
			Expect(string(tok)).To(Equal("2026090713"))
		})

		It("yields different tokens for hour 12 and hour 13 on the same day", func() {
			def := schedule.TaskDefinition{
				Slug:       "hourly-build-check",
				Recurrence: schedule.RecurrenceHourly,
			}
			tok12 := build(def, schedule.Date{Year: 2026, Month: time.September, Day: 7, Hour: 12})
			tok13 := build(def, schedule.Date{Year: 2026, Month: time.September, Day: 7, Hour: 13})
			Expect(string(tok12)).To(Equal("2026090712"))
			Expect(string(tok13)).To(Equal("2026090713"))
			Expect(tok12).NotTo(Equal(tok13))
		})

		It("produces the hour-00 token for a date-only construction site", func() {
			def := schedule.TaskDefinition{
				Slug:       "hourly-build-check",
				Recurrence: schedule.RecurrenceHourly,
			}
			// NewDate leaves Hour=0 (the /trigger handler's shape) → hour-00 token.
			tok := build(def, schedule.NewDate(2026, time.September, 7))
			Expect(string(tok)).To(Equal("2026090700"))
		})
	})
```

Keep every existing spec in the file unchanged and passing.

### 6. Add an identifier-distinctness spec through the real boundary

In `/workspace/pkg/publisher/publisher_test.go`, add a spec proving different hours yield different UUID5 identifiers — traverse the real `TaskIdentifierCreator.Create` path (which feeds the token into the UUID5 input), mirroring the `publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder())` pattern already used in the file (line ~370). `github.com/google/uuid` is NOT needed here — `Create` returns `lib.TaskIdentifier` directly:

```go
	It("produces distinct UUID5 identifiers for hour 12 and hour 13 on the same day", func() {
		def := schedule.TaskDefinition{
			Slug:          "hourly-build-check",
			TitleTemplate: "Build Check",
			Recurrence:    schedule.RecurrenceHourly,
		}
		creator := publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder())
		id12, tok12, err := creator.Create(context.Background(), def,
			schedule.Date{Year: 2026, Month: time.September, Day: 7, Hour: 12})
		Expect(err).NotTo(HaveOccurred())
		id13, tok13, err := creator.Create(context.Background(), def,
			schedule.Date{Year: 2026, Month: time.September, Day: 7, Hour: 13})
		Expect(err).NotTo(HaveOccurred())

		Expect(string(tok12)).To(Equal("2026090712"))
		Expect(string(tok13)).To(Equal("2026090713"))
		Expect(id12).NotTo(Equal(id13),
			"different hours must produce different identifiers → different task files")
	})
```

Place it alongside the other `NewTaskIdentifierCreator` specs. Keep every existing spec unchanged and passing.

### 7. Add `hourly` to the boundary-contract table

In `/workspace/pkg/publisher/publisher_test.go`, in the `DescribeTable("produced command passes task.CreateCommand.Validate", ...)` block (line ~1200, currently one `Entry(...)` per kind ending with `Entry("ondate", schedule.RecurrenceOnDate)`), add `Entry("hourly", schedule.RecurrenceHourly)` so the hourly publish path traverses `task.CreateCommand.Validate` like every other kind. The table already publishes with `schedule.NewDate(2025, time.January, 4)` (Hour=0 → token `2025010400`), which is fine for the validation contract.

### 8. Add `hourly` to the defer_date table

In `/workspace/pkg/publisher/frontmatter_test.go`, in the `Describe("defer_date stamp")` → `DescribeTable("stamps the period-start date per recurrence kind", ...)`, add an entry next to the other point-shaped kinds:

```go
			Entry("hourly → fire date", schedule.RecurrenceHourly, "2026-06-20"),
```

The table's `date` fixture is `2026-06-20` — match the existing entries' shape exactly.

### 9. Changelog

In `/workspace/CHANGELOG.md`, ensure a `## Unreleased` section exists directly below the SemVer preamble (create it if absent — the last release renamed it to `## v0.11.6`, so at HEAD it does NOT exist), then append one entry under it:

```
- feat: add `hourly` period token to the publisher — `RecurrenceHourly` builds the compact civil-hour token `YYYYMMDDHH` via the new `fmtHourToken` helper (no `PeriodOffset`, matching the CRD rule that keeps offset valid only for Monthly/Quarterly/Yearly), so each civil hour yields a distinct UUID5 identifier and task file; `deferDateFor` names Hourly among the point-shaped kinds (defer_date = fire date)
```

</requirements>

<constraints>
- Do NOT change any existing period-token format (`YYYY-MM-DD`, `YYYYWww`, `YYYYWww-<wd>`, `YYYY-MM`, `YYYYQq`, `YYYY` stay exactly as they are). This prompt adds ONE new case only.
- Do NOT apply `PeriodOffset` to `Hourly` — read `date.Hour` directly, no `AddDate` shift (spec Non-goal).
- Do NOT modify the `RecurrenceYearly`, `RecurrenceOnDate`, or `default:` cases of `Build` (beyond inserting the new case).
- **AC 8 (negative): `pkg/publisher/period_token.go` must have ZERO removed lines** — `git diff pkg/publisher/period_token.go | grep -c '^-[^-]'` must return `0`. Every change to that file (new case + doc-comment insertion) must be additions-only; never edit an existing line in place.
- The hourly token MUST equal the compact `%04d%02d%02d%02d` of (year, month, day, hour) via the new `fmtHourToken` helper in `pkg/publisher/render.go` — do NOT reuse `fmtDate` (dash-separated) or build the token inline in `Build`.
- Do NOT add any placeholder to the closed placeholder set (`Date`, `ISOWeek`, `MonthYear`, `Quarter`, `Year` stay unchanged) and do NOT add any config knob, env var, or tunable threshold (spec Non-goals).
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc on the new exported nothing (the helper is unexported) — keep the helper GoDoc consistent with the file's style.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (this prompt adds none); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg/publisher` package; the hourly token path, the distinctness paths, the frontmatter defer_date path, and the wire-validate path must be exercised by the added specs.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the new case, helper, and point-shaped switch entry are present:

```bash
cd /workspace && grep -n 'case schedule.RecurrenceHourly' pkg/publisher/period_token.go
cd /workspace && grep -n 'fmtHourToken' pkg/publisher/render.go
cd /workspace && grep -n 'RecurrenceHourly' pkg/publisher/frontmatter.go
```

Each must return ≥1 line.

Run the publisher specs verbosely and confirm the new hourly specs pass (token `2026090713`, hour-12/13 distinctness, hour-00 token, identifier distinctness, wire-validate entry, defer_date entry):

```bash
cd /workspace && go test -v ./pkg/publisher/
```

Confirm AC 8 — no removed lines in the period-token file:

```bash
cd /workspace && git diff pkg/publisher/period_token.go | grep -c '^-[^-]'
```

Must print `0`. If it prints `1` or more, an existing line in `period_token.go` was edited — fix it so the diff is additions-only.

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
