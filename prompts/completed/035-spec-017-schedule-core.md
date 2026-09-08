---
status: completed
spec: [017-hourly-recurrence-kind]
execution_id: recurring-task-creator-hourly-exec-035-spec-017-schedule-core
dark-factory-version: dev
created: "2026-09-07T21:05:00Z"
queued: "2026-09-07T21:31:23Z"
started: "2026-09-07T21:31:25Z"
completed: "2026-09-07T21:37:09Z"
branch: dark-factory/hourly-recurrence-kind
---

<summary>
- Adds a new recurrence kind, `hourly`, to the schedule layer's closed set of recurrence kinds, which the store adapter and metrics label pre-initialization consume automatically.
- The civil-date record now carries an `Hour` field (0-23); date-only construction sites keep producing hour zero, and only the upcoming hourly period token will ever read it.
- An `hourly` schedule fires on every tick — it joins the explicit always-fire group — while an unrecognized kind still never fires (skip with a logged warning).
- Because metrics label pre-initialization ranges over `AllRecurrenceKinds`, the new kind is picked up automatically: Prometheus now pre-seeds 16 series (8 kinds x 2 outcomes).
- New Ginkgo specs prove the hourly entry always fires on an arbitrary civil date (including with a non-zero hour on the date), and that the unknown-kind skip-with-warning behavior still holds.
- The existing Prometheus pre-initialization test is updated for the new series count so the whole tree stays green.
</summary>

<objective>
Add `hourly` to the closed `RecurrenceKind` set, give `schedule.Date` an `Hour int` field that date-only construction sites leave at zero, and make `filterInventoryByDate` treat `hourly` as always-fire — the schedule-layer foundation that prompts 2 (tick), 3 (publisher), and 4 (CRD) build on.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 1 of 4 for spec 017. It has NO dependency on the other prompts — it establishes the kind + the `Date.Hour` field every other layer consumes.

Read these files fully before changing anything:
- `/workspace/pkg/schedule/recurrence.go` — the closed `RecurrenceKind` enum and the `AllRecurrenceKinds` slice. Current kinds: `RecurrenceDaily`/`Weekly`/`Weekday`/`Monthly`/`Quarterly`/`Yearly`/`OnDate`, all lowercase string values. You append one const and one slice entry.
- `/workspace/pkg/schedule/date.go` — the `Date` struct (`Year int`, `Month time.Month`, `Day int`) with `NewDate(year, month, day)` constructor, `IsZero()`, `Time()`. You add an `Hour int` field. Do NOT change `NewDate`, `IsZero`, `toTime`, or `Time()` — the field is purely additive and read only by the publisher's hourly period token (prompt 3).
- `/workspace/pkg/schedule/tasks_for_date.go` — `TasksForDate` (public) and `filterInventoryByDate` (the internal switch). Note the explicit always-fire case group `case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly:` and the `default:` skip-with-warning branch. You add `RecurrenceHourly` to the always-fire group.
- `/workspace/pkg/schedule/tasks_for_date_test.go` — `package schedule_test`, Ginkgo specs, `slugsOf(defs)` helper at the bottom. Note the `DescribeTable("each always-fire kind fires on an arbitrary date", ...)` (Entry list at the end) and the existing unknown-kind-skip `It` you must leave green.
- `/workspace/pkg/tick/tick_test.go` — the `Prometheus pre-initialization` Describe block at the bottom asserts the counter has 14 zero-valued series and that each series' recurrence label is one of the 7 kinds. Adding a kind to `AllRecurrenceKinds` makes `init()` in `/workspace/pkg/tick/metrics.go` pre-seed 16 series (8 kinds x 2 outcomes) — you MUST update this test in the same prompt that breaks it, or `make test` fails.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — closed enum; `AllRecurrenceKinds` is the single validity source; never hand-roll a duplicate kind list.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc starts with the field name, states behavior not implementation.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format.
</context>

<requirements>

### 1. Declare `RecurrenceHourly` and append it to `AllRecurrenceKinds`

In `/workspace/pkg/schedule/recurrence.go`, add the const to the existing `const (...)` block AFTER `RecurrenceOnDate` (append at the end — declaration order preserved, every existing kind keeps its position):

```go
	RecurrenceOnDate RecurrenceKind = "ondate"
	// RecurrenceHourly fires once per civil hour — a distinct task file for
	// each civil hour of the Europe/Berlin clock, materialized on every tick
	// (always-fire, like Daily). Its publisher period token is the compact
	// civil hour "YYYYMMDDHH", so each civil hour produces a distinct UUID5
	// identifier and thus a distinct task file.
	RecurrenceHourly RecurrenceKind = "hourly"
```

Append `RecurrenceHourly` to the `AllRecurrenceKinds` slice as the last entry (after `RecurrenceOnDate`), keeping the slice's order identical to the const declaration order:

```go
var AllRecurrenceKinds = []RecurrenceKind{
	RecurrenceDaily,
	RecurrenceWeekly,
	RecurrenceWeekday,
	RecurrenceMonthly,
	RecurrenceQuarterly,
	RecurrenceYearly,
	RecurrenceOnDate,
	RecurrenceHourly,
}
```

The lowercase string value `"hourly"` matters: the store adapter lowercases the CR's `"Hourly"` string and matches it against `AllRecurrenceKinds`, so `"Hourly"` → `"hourly"` must equal this const's value.

### 2. Add `Hour int` to `schedule.Date`

In `/workspace/pkg/schedule/date.go`, add an `Hour int` field to the `Date` struct after `Day`. The GoDoc MUST state BOTH the zero-for-date-only rule AND the hourly-only rule (spec AC 4):

```go
type Date struct {
	Year  int
	Month time.Month
	Day   int
	// Hour is the civil hour of day (0-23) in Europe/Berlin. It is the zero
	// value (0) for every date-only construction site — the /trigger handler,
	// existing tests, and any caller that knows only a civil date. It is
	// consulted only by the publisher's hourly period token; every other
	// recurrence kind ignores it.
	Hour int
}
```

Do NOT change `NewDate` (stays 3-arg — construction sites keep producing `Hour=0`), `IsZero`, `toTime`, or `Time()`. `Date.Time()` stays midnight UTC — the hourly period token (prompt 3) reads `date.Hour` directly, never through `Time()`. No new imports: `time` is already imported and `Hour` is a plain `int`.

### 3. Make `hourly` always-fire in `filterInventoryByDate`

In `/workspace/pkg/schedule/tasks_for_date.go`, add `RecurrenceHourly` to the explicit always-fire case group in `filterInventoryByDate`:

```go
		case RecurrenceDaily,
			RecurrenceWeekly,
			RecurrenceMonthly,
			RecurrenceQuarterly,
			RecurrenceYearly,
			RecurrenceHourly:
			// Always-fire — the entry fires on every civil date.
			out = append(out, def)
```

The `default:` branch stays skip-with-warning — an unrecognized kind still never fires (spec Failure Mode: `default:` always-fire regression). `Hour` is NOT consulted here: an `hourly` entry fires regardless of the date's Hour (always-fire, like Daily).

Update the GoDoc block on `TasksForDate` (and `filterInventoryByDate` if it enumerates the rule) so the always-fire bullet names `RecurrenceHourly` alongside Daily/Weekly/Monthly/Quarterly/Yearly.

### 4. Update the Prometheus pre-initialization test in `pkg/tick`

Adding `RecurrenceHourly` to `AllRecurrenceKinds` makes `init()` in `/workspace/pkg/tick/metrics.go` pre-seed the counter with 16 zero-valued series (8 kinds x 2 outcomes) instead of 14. The `Prometheus pre-initialization` Describe block in `/workspace/pkg/tick/tick_test.go` asserts the old count and the 7-kind label set — update it:
- `Expect(metrics).To(HaveLen(14))` → `Expect(metrics).To(HaveLen(16))` (both occurrences — the gatherer length and the `seen` map length, lines ~497 and ~518).
- Add `"hourly"` to the `BeElementOf(...)` allowed-kinds list (line ~514).
- Update the spec title `It("registers the counter with 14 zero-valued series", ...)` to `16 zero-valued series` (line ~484) — the title would otherwise contradict the assertion.
- Update the stale count comments in `/workspace/pkg/tick/metrics.go`: line ~37 (`Pre-initialized to zero for all 14 combinations in init()...`) and line ~70 (`...pre-initialized for all fourteen result/recurrence label combinations.`) to `16` / `sixteen`.
- Keep the assertion that every series is zero-valued.

This is not scope creep — it is the mechanical consequence of adding the kind, and `make test` fails without it.

### 5. Add Ginkgo specs to `tasks_for_date_test.go`

In `/workspace/pkg/schedule/tasks_for_date_test.go` (`package schedule_test`), reuse the existing `slugsOf` helper:

a. Add `Entry("Hourly", schedule.RecurrenceHourly)` to the existing `DescribeTable("each always-fire kind fires on an arbitrary date", ...)`.

b. Add a dedicated spec proving an `hourly` entry fires regardless of the date's Hour:

```go
	It("fires an hourly entry on any civil date, regardless of the hour", func() {
		hourly := []schedule.TaskDefinition{
			{Slug: "hourly-build-check", Recurrence: schedule.RecurrenceHourly},
		}
		// TasksForDate ignores the Hour field: an hourly entry always fires,
		// on any civil date at any hour (always-fire, like Daily).
		Expect(slugsOf(schedule.TasksForDate(hourly, schedule.NewDate(2027, time.March, 14)))).
			To(ConsistOf("hourly-build-check"))
		Expect(slugsOf(schedule.TasksForDate(hourly, schedule.Date{
			Year: 2027, Month: time.March, Day: 14, Hour: 13,
		}))).To(ConsistOf("hourly-build-check"))
	})
```

Also update the always-fire enumeration spec `It("preserves the always-fire semantic for the four other kinds", ...)` (lines ~194-200): add `schedule.RecurrenceHourly` to its `BeElementOf(...)` list and update the title to `five other kinds`.

The existing `It("skips (does not fire) an entry with an unrecognized recurrence kind", ...)` spec already proves the unknown-kind skip-with-warning still holds — leave it unchanged and passing.

### 6. Changelog

In `/workspace/CHANGELOG.md`, ensure a `## Unreleased` section exists at the top (after the intro, before `## v0.11.6`); if it already exists, append to it. Add one entry per the changelog-guide format:

```
- feat: add `hourly` recurrence kind to `pkg/schedule` — appended to `AllRecurrenceKinds` (metrics label pre-initialization and the store adapter pick it up automatically); `schedule.Date` gains an `Hour int` field (0-23, zero for date-only construction, consulted only by the hourly period token); `filterInventoryByDate` treats `hourly` as always-fire alongside Daily/Weekly/Monthly/Quarterly/Yearly
```

Do NOT write anything else into the changelog — prompts 2-4 add their own entries.

### 7. Add the hourly entry to the store adapter's recurrence-mapping table

In `/workspace/pkg/store/adapter_test.go`, the `DescribeTable("recurrence mapping", ...)` enumerates every kind's CR-string → `RecurrenceKind` mapping (the parse-time boundary that lowercases `"Hourly"` and matches `AllRecurrenceKinds`). Add the `"hourly"` entry after the ondate entry, and add the reminder comment above the table so future kind additions update it:

```go
	// One entry per RecurrenceKind — keep in sync with
	// schedule.AllRecurrenceKinds when kinds are added or removed.
	DescribeTable("recurrence mapping",
		func(input string, expected schedule.RecurrenceKind) {
		...
		Entry("ondate", "OnDate", schedule.RecurrenceOnDate),
		Entry("hourly", "Hourly", schedule.RecurrenceHourly),
	)
```

Do not add or change any `pkg/store` production code — the adapter already picks the kind up via `AllRecurrenceKinds`; this is a regression-lock test only.

</requirements>

<constraints>
- `pkg/schedule/` stays a pure-data-plus-logging layer — NO new imports at all (date.go already imports `time`; `Hour` is a plain `int`). No Kafka / HTTP / agent imports.
- `RecurrenceKind` remains a CLOSED enum; `AllRecurrenceKinds` stays declaration-ordered and is the single validity source — no inline kind switch that bypasses it (`go-enum-type-pattern.md`).
- Existing kinds' firing behavior MUST NOT change — this prompt is purely additive. The always-fire kinds fire byte-for-byte as before; only `hourly` joins the group.
- The `default:` branch in `filterInventoryByDate` must NOT fire — it logs a warning and continues. An unknown kind is a no-op, never a daily storm.
- `schedule.Date` stays the only input shape to `TasksForDate` and the publisher; the new `Hour` field is additive and every existing construction site keeps producing `Hour=0`. Do NOT change `NewDate`, `IsZero`, `Time()`, or `toTime()`. Do NOT make `Time()` include `Hour`.
- Do NOT add any config knob, env var, tunable threshold, or opt-out flag (spec Non-goals).
- Do NOT change the tick-loop cadence and do NOT touch `pkg/tick/tick.go` (that is prompt 2). The only `pkg/tick` change here is the pre-initialization test that prompt 1's kind addition breaks.
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc on the new exported const and field.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (this prompt adds none); no `fmt.Errorf`; no `context.Background()` / `time.Now()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg/schedule` package; the hourly always-fire path and the unknown-kind skip path must be exercised.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the const, slice entry, and Hour field are present:

```bash
cd /workspace && grep -nE 'RecurrenceHourly\s+RecurrenceKind = "hourly"' pkg/schedule/recurrence.go
cd /workspace && grep -nA12 'AllRecurrenceKinds = ' pkg/schedule/recurrence.go | grep -c 'RecurrenceHourly'
cd /workspace && grep -nE 'Hour\s+int' pkg/schedule/date.go
cd /workspace && grep -n 'RecurrenceHourly' pkg/schedule/tasks_for_date.go
```

The first must return ≥1 line; the second must print `1`; the third and fourth must return ≥1 line each.

Run the new specs verbosely and confirm the hourly always-fire + unknown-kind skip specs pass, and the tick pre-initialization test passes with 16 series:

```bash
cd /workspace && go test -v ./pkg/schedule/ ./pkg/tick/ ./pkg/store/
```

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
