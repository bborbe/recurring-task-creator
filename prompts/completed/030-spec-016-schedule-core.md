---
status: completed
spec: [016-ondate-recurrence-kind]
summary: Added RecurrenceOnDate kind to pkg/schedule with Month/Day fields, match-fire switch case, glog warning for unknown kinds, and updated tick metrics for 14 series
execution_id: recurring-task-creator-ondate-recurrence-exec-030-spec-016-schedule-core
dark-factory-version: dev
created: "2026-07-14T12:00:00Z"
queued: "2026-07-14T12:04:51Z"
started: "2026-07-14T12:04:52Z"
completed: "2026-07-14T12:12:06Z"
branch: dark-factory/ondate-recurrence-kind
---

<summary>
- Adds a new recurrence kind, `OnDate`, that fires a schedule on one fixed calendar date (a specific month + day) every year — the building block for "on this exact date" reminders like birthdays.
- The internal task record now carries a month and a day, meaningful only for the new `OnDate` kind (mirroring how the weekday set is meaningful only for the weekday kind).
- The firing rule now fires an `OnDate` entry on, and only on, dates whose month and day match the entry's month and day.
- Hardens the firing switch: the always-fire kinds (daily/weekly/monthly/quarterly/yearly) are now listed explicitly, and any unrecognized kind is skipped with a logged warning instead of silently firing every single day.
- Existing kinds' firing behavior is byte-for-byte unchanged; this is purely additive plus a safety hardening of the default branch.
- New Ginkgo specs prove the match-fire behavior, prove an unknown kind is skipped (not fired), and prove all five always-fire kinds still fire.
</summary>

<objective>
Add the `OnDate` recurrence kind to the schedule layer: declare `RecurrenceOnDate` in the closed enum, carry `Month`/`Day` on `TaskDefinition`, make `filterInventoryByDate` fire `OnDate` entries only on the matching month-and-day, and replace the implicit `default: always-fire` with explicit always-fire cases plus a skip-with-warning default so an unknown kind is a no-op, not a daily storm.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 1 of 3 for spec 016. It has NO dependency on the other prompts — it establishes the type + firing semantics prompts 2 (publisher) and 3 (CRD + adapter) build on.

Read these files fully before changing anything:
- `/workspace/pkg/schedule/recurrence.go` — the closed `RecurrenceKind` enum and the `AllRecurrenceKinds` slice. You add one const and one slice entry. Note the current 6 kinds: `RecurrenceDaily`/`Weekly`/`Weekday`/`Monthly`/`Quarterly`/`Yearly`, all lowercase string values.
- `/workspace/pkg/schedule/task_definition.go` — the `TaskDefinition` struct. Note the existing `Weekdays []time.Weekday` field and its GoDoc style ("meaningful only for RecurrenceWeekday; ignored otherwise"); the new `Month time.Month` / `Day int` fields mirror that style. `time` is already imported.
- `/workspace/pkg/schedule/date.go` — the `Date` struct (`Year int`, `Month time.Month`, `Day int`) and its `Time() time.Time` method. `date.Time().Month()` and `date.Time().Day()` are the civil month/day you compare against.
- `/workspace/pkg/schedule/tasks_for_date.go` — `TasksForDate` (public) and `filterInventoryByDate` (the internal switch you modify). Note the current `switch def.Recurrence { case RecurrenceWeekday: ...; default: /* always-fire */ append }`.
- `/workspace/pkg/schedule/tasks_for_date_test.go` — the existing `package schedule_test` Ginkgo specs. Note the `slugsOf(defs)` helper (declared at the bottom) and the synthetic-fixtures style. Add new specs here.
- `/workspace/pkg/schedule/no_forbidden_imports_test.go` — this asserts `pkg/schedule` imports no Kafka/HTTP/agent packages. `glog` is a logging import — verify whether the test permits it before adding it (see requirement 4).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — closed enum, `AllRecurrenceKinds` is the single validity source; never hand-roll a duplicate kind list.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-glog-guide.md` — `glog.Warningf` usage and V-levels.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc starts with the field/const name.
</context>

<requirements>

### 1. Declare `RecurrenceOnDate` and append it to `AllRecurrenceKinds`

In `/workspace/pkg/schedule/recurrence.go`:

Add the const to the existing `const (...)` block, after `RecurrenceYearly`:

```go
	RecurrenceYearly    RecurrenceKind = "yearly"
	// RecurrenceOnDate fires on one fixed calendar date (Month + Day) every
	// year — e.g. 03-15 for a birthday. Point-shaped match-fire, mirroring
	// how RecurrenceWeekday matches a day-of-week. Its publisher period token
	// is the fire date's 4-digit year ("YYYY"), so replays within a year are
	// idempotent (UUID5 dedup collapses them to one task file).
	RecurrenceOnDate    RecurrenceKind = "ondate"
```

Append `RecurrenceOnDate` to the `AllRecurrenceKinds` slice as the last entry (declaration order preserved):

```go
var AllRecurrenceKinds = []RecurrenceKind{
	RecurrenceDaily,
	RecurrenceWeekly,
	RecurrenceWeekday,
	RecurrenceMonthly,
	RecurrenceQuarterly,
	RecurrenceYearly,
	RecurrenceOnDate,
}
```

The lowercase string value `"ondate"` matters: the store adapter (prompt 3) lowercases the CR's `"OnDate"` string and matches it against `AllRecurrenceKinds`, so `"OnDate"` → `"ondate"` must equal this const's value.

### 2. Add `Month time.Month` and `Day int` to `TaskDefinition`

In `/workspace/pkg/schedule/task_definition.go`, add these two fields to the `TaskDefinition` struct (place them after `Weekdays`, before `Frontmatter`), with GoDoc mirroring the `Weekdays` field's "meaningful only for kind X" style:

```go
	// Month is the calendar month a RecurrenceOnDate entry fires in
	// (time.January..time.December). Consulted only when
	// Recurrence == RecurrenceOnDate; is the zero value (time.Month(0)) and
	// ignored for every other kind. Produced by the store adapter from the
	// CR's spec.schedule.month field. Paired with Day: the matcher
	// (TasksForDate) fires the entry only when both this Month and Day equal
	// the civil date's month and day.
	Month time.Month

	// Day is the day-of-month a RecurrenceOnDate entry fires on (1-31).
	// Consulted only when Recurrence == RecurrenceOnDate; is the zero value (0)
	// and ignored for every other kind. Produced by the store adapter from
	// the CR's spec.schedule.day field. An OnDate of Month=February, Day=29
	// fires only in leap years (documented behavior, not an error).
	Day int
```

Do NOT import anything new — `time` is already imported in this file. The package stays pure data (no Kafka/HTTP/agent imports).

### 3. Add the `OnDate` match-fire case to `filterInventoryByDate`

In `/workspace/pkg/schedule/tasks_for_date.go`, in the `switch def.Recurrence` inside `filterInventoryByDate`, add a case that fires the entry only when the entry's `Month` AND `Day` equal the civil date's month and day. Precompute the date's month and day alongside the existing `dateWeekday`:

```go
func filterInventoryByDate(defs []TaskDefinition, date Date) []TaskDefinition {
	dateWeekday := date.Time().Weekday()
	dateMonth := date.Time().Month()
	dateDay := date.Time().Day()
	out := make([]TaskDefinition, 0, len(defs))
	for _, def := range defs {
		switch def.Recurrence {
		case RecurrenceWeekday:
			for _, wd := range def.Weekdays {
				if wd == dateWeekday {
					out = append(out, def)
					break
				}
			}
		case RecurrenceOnDate:
			if def.Month == dateMonth && def.Day == dateDay {
				out = append(out, def)
			}
		case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly:
			// Always-fire — the entry fires on every civil date.
			out = append(out, def)
		default:
			glog.Warningf(
				"filterInventoryByDate: unknown recurrence kind %q for slug %q — skipping",
				def.Recurrence, def.Slug,
			)
		}
	}
	return out
}
```

### 4. Make the `default:` branch skip-with-warning (not always-fire)

As shown in requirement 3, the implicit `default: always-fire` is REPLACED by:
- an explicit `case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly:` that always-fires (byte-for-byte the same firing behavior these five kinds had before), and
- a `default:` that calls `glog.Warningf("filterInventoryByDate: unknown recurrence kind %q for slug %q — skipping", def.Recurrence, def.Slug)` and does NOT append the entry.

Add the glog import to `tasks_for_date.go`:

```go
import "github.com/golang/glog"
```

The `glog` import path `github.com/golang/glog` is the same one already used at `/workspace/pkg/publisher/publisher.go`, `/workspace/pkg/k8s_connector.go`, and `/workspace/pkg/handler/trigger.go` — confirm it resolves. BEFORE adding it, read `/workspace/pkg/schedule/no_forbidden_imports_test.go`: if that test's forbidden-import list would reject `github.com/golang/glog`, DO NOT add the import — instead add `github.com/golang/glog` to that test's ALLOWED set (the test's intent is to bar Kafka/HTTP/agent imports, not logging), and document the one-line change in `## Improvements` (category: PROMPT). glog is a leaf logging dependency and does not violate the pure-data-plus-logging boundary. Do NOT introduce any Kafka/HTTP/agent import.

Update the GoDoc block above `filterInventoryByDate` (and the doc on `TasksForDate` if it enumerates the firing rule) to describe the `OnDate` match-fire case and the skip-with-warning default. Keep it accurate to the new switch.

### 5. Add Ginkgo specs to `tasks_for_date_test.go`

In `/workspace/pkg/schedule/tasks_for_date_test.go` (`package schedule_test`), add specs — reuse the existing `slugsOf` helper:

a. **OnDate match-fire** — an `OnDate` entry (`Month=time.March, Day=15`) fires on `2027-03-15` and does NOT fire on `2027-03-14` or `2027-07-15`:

```go
It("fires an OnDate entry only on its exact month-and-day", func() {
	onDate := []schedule.TaskDefinition{
		{Slug: "birthday-alice", Recurrence: schedule.RecurrenceOnDate, Month: time.March, Day: 15},
	}
	Expect(slugsOf(schedule.TasksForDate(onDate, schedule.NewDate(2027, time.March, 15)))).
		To(ConsistOf("birthday-alice"))
	Expect(schedule.TasksForDate(onDate, schedule.NewDate(2027, time.March, 14))).To(BeEmpty())
	Expect(schedule.TasksForDate(onDate, schedule.NewDate(2027, time.July, 15))).To(BeEmpty())
})
```

b. **Unknown-kind skip** — an entry with an unrecognized `RecurrenceKind` (`"bogus"`) is NOT included by `TasksForDate` (default branch is skip, not fire):

```go
It("skips (does not fire) an entry with an unrecognized recurrence kind", func() {
	bogus := []schedule.TaskDefinition{
		{Slug: "mystery", Recurrence: schedule.RecurrenceKind("bogus")},
	}
	Expect(schedule.TasksForDate(bogus, schedule.NewDate(2027, time.March, 15))).To(BeEmpty())
})
```

c. **Always-fire kinds still fire** — each of `RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly` is included by `TasksForDate` on an arbitrary date (behavioral assertion, not source-text-pinned — a gofmt reorder must not fail it):

```go
DescribeTable("each always-fire kind fires on an arbitrary date",
	func(kind schedule.RecurrenceKind) {
		defs := []schedule.TaskDefinition{{Slug: "af", Recurrence: kind}}
		Expect(slugsOf(schedule.TasksForDate(defs, schedule.NewDate(2027, time.March, 14)))).
			To(ConsistOf("af"))
	},
	Entry("Daily", schedule.RecurrenceDaily),
	Entry("Weekly", schedule.RecurrenceWeekly),
	Entry("Monthly", schedule.RecurrenceMonthly),
	Entry("Quarterly", schedule.RecurrenceQuarterly),
	Entry("Yearly", schedule.RecurrenceYearly),
)
```

Keep every existing spec in this file unchanged and passing.

</requirements>

<constraints>
- `pkg/schedule/` stays a pure-data-plus-logging layer — NO Kafka / HTTP / agent imports. `glog` (leaf logging dependency) is the only new import permitted, and only if the forbidden-imports test allows it (see requirement 4).
- `RecurrenceKind` remains a CLOSED enum; `AllRecurrenceKinds` stays declaration-ordered and is the single validity source — no inline kind switch that bypasses it (go-enum-type-pattern.md).
- Existing period-token formats and existing kinds' firing behavior MUST NOT change — this prompt is additive plus the default-branch hardening only. The five always-fire kinds fire byte-for-byte as before.
- The UUID5 namespace and existing slugs are frozen — no existing entry changes kind or identifier.
- Do NOT add any config knob, env var, tunable threshold, or opt-out flag. (Spec Non-goals.)
- Do NOT add per-month day validity (e.g. rejecting 02-30) or `PeriodOffset` handling for OnDate — those are Non-goals / belong to prompt 3's CEL rule.
- The `default:` branch must NOT fire the entry — it logs a warning and continues. An unknown kind is a no-op, never a daily storm (the 2026-06-19 incident class).
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc on the new exported const and fields.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path; no `fmt.Errorf`; no `context.Background()` / `time.Now()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg/schedule` package; test the OnDate match/no-match paths and the unknown-kind skip path.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the const, fields, and firing cases are present:

```bash
cd /workspace && grep -nE 'RecurrenceOnDate\s+RecurrenceKind = "ondate"' pkg/schedule/recurrence.go
cd /workspace && grep -nA20 'AllRecurrenceKinds = ' pkg/schedule/recurrence.go | grep -c 'RecurrenceOnDate'
cd /workspace && grep -nE 'Month\s+time\.Month' pkg/schedule/task_definition.go
cd /workspace && grep -nE 'Day\s+int' pkg/schedule/task_definition.go
cd /workspace && grep -nE 'case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly' pkg/schedule/tasks_for_date.go
cd /workspace && grep -nA4 'default:' pkg/schedule/tasks_for_date.go | grep -c 'glog.Warning'
```

Each grep must return ≥1 line (the second/last two return `1`).

Run the new specs verbosely and confirm they pass:

```bash
cd /workspace && go test -v ./pkg/schedule/
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
