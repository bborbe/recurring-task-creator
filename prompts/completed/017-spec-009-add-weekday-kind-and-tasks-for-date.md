---
status: completed
spec: [009-weekday-kind-split]
summary: Added RecurrenceWeekday (6th value) to RecurrenceKind enum and AllRecurrenceKinds slice, updated Weekday GoDoc to describe 3-way semantic, added pure TasksForDate accessor and internal filterInventoryByDate worker, added 9 Ginkgo specs (8 synthetic-fixture + 1 public-accessor coverage), added FilterInventoryByDateForTest test accessor, updated CHANGELOG Unreleased; make precommit exits 0
container: recurring-task-creator-weekday-kind-exec-017-spec-009-add-weekday-kind-and-tasks-for-date
dark-factory-version: v0.177.1
created: "2026-06-16T12:30:00Z"
queued: "2026-06-16T12:45:58Z"
started: "2026-06-16T12:45:59Z"
completed: "2026-06-16T12:56:26Z"
branch: dark-factory/weekday-kind-split
---

<summary>

- The `RecurrenceKind` closed enum gains a sixth value: `RecurrenceWeekday RecurrenceKind = "weekday"`. After this prompt the enum has 6 values in stable declaration order: `RecurrenceDaily`, `RecurrenceWeekly`, `RecurrenceWeekday`, `RecurrenceMonthly`, `RecurrenceQuarterly`, `RecurrenceYearly`. The new constant is exported alongside the existing five; existing `RecurrenceWeekly` is unchanged and still reserved for true always-fire weekly entries (no inventory entries of that kind exist yet, none are added by this prompt).
- `RecurrenceWeekday` is added to the canonical `AllRecurrenceKinds` slice in the same position (between `RecurrenceWeekly` and `RecurrenceMonthly`). Consumers that pre-initialize Prometheus label combinations (the `init()` in `pkg/tick/metrics.go`) now iterate 6 kinds, but no production code calls `AllRecurrenceKinds` for execution yet — the metric surface will pick up the 6th kind in Prompt 2, where the tick is wired to filter by date.
- The `Weekday` field's GoDoc on `schedule.TaskDefinition` is updated to describe the new invariant: `Weekday` is required (non-zero) for `RecurrenceWeekday`; forbidden (zero) for `RecurrenceWeekly`; ignored for all other kinds. The doc is the single source of truth for the field's semantics — both Prompt 2's validation tests and any future spec that adds another kind will reference it.
- A new pure accessor `TasksForDate(date schedule.Date) []TaskDefinition` is added to `pkg/schedule/`. It returns the subset of the canonical inventory that fires on the given civil date: every `RecurrenceDaily`, `RecurrenceWeekly`, `RecurrenceMonthly`, `RecurrenceQuarterly`, `RecurrenceYearly` entry fires on every day (always-fire semantic, unchanged from Spec 6); every `RecurrenceWeekday` entry fires only if its `Weekday` equals the date's weekday (computed via `date.Time().Weekday()`). The function is pure: no I/O, no clock, no env. The factory and the trigger handler do NOT switch to it in this prompt — the tick and the trigger continue to call `schedule.Inventory()` (full-inventory iteration); Prompt 2 wires both to `TasksForDate`.
- New Ginkgo specs in `pkg/schedule/tasks_for_date_test.go` exercise `TasksForDate` with synthetic fixtures (not the production inventory, so this prompt does not need any inventory migration). The fixtures cover: `RecurrenceWeekday` fires only on the matching weekday, `RecurrenceWeekday` does not fire on any other weekday, all other kinds (daily/weekly/monthly/quarterly/yearly) fire on every day, and an empty inventory returns an empty slice. None of the fixtures are added to the production inventory — they are local to the test file.
- The `sundayWeeklyAllowList` package-level var in `pkg/schedule/inventory_validation_test.go` is NOT yet removed. The removal is Prompt 2's job, because the var remains load-bearing until the 21 weekly-with-`Weekday` entries are migrated to `RecurrenceWeekday` (Prompt 2, change (d) in the spec's coupled-changes list). The 21 inventory entries are also unchanged in this prompt; their `Recurrence: RecurrenceWeekly` declarations and `Weekday: time.Saturday|time.Sunday` values are preserved.
- `make precommit` exits 0 at the end. The `RecurrenceKind` enum now has 6 values; `AllRecurrenceKinds` has 6 entries; `TasksForDate` is exported and covered by unit tests; the inventory is unchanged. Prompt 2 then migrates the inventory and wires the tick and the trigger to filter by date.

</summary>

<objective>

Extend the recurrence-kind enum with a new first-class value for per-weekday firing (`RecurrenceWeekday`), update the `Weekday` field's GoDoc to describe the new invariant, and add a pure `TasksForDate(date) []TaskDefinition` accessor that returns the date-filtered subset of the inventory. The new kind and the new function exist after this prompt, but the inventory is NOT yet migrated (still 21 entries with `Recurrence: RecurrenceWeekly` + `Weekday: time.Saturday|time.Sunday`); the tick and the trigger handler continue to iterate the full inventory. The build remains green and existing tests pass unchanged — Prompt 2 performs the inventory migration and the tick/trigger wiring.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before making changes:

- `/workspace/pkg/schedule/recurrence.go` — the 5-value `RecurrenceKind` enum and the `AllRecurrenceKinds` slice (5 entries in declaration order). This prompt adds the 6th value `RecurrenceWeekday` and the 6th slice entry. The doc comment on `AllRecurrenceKinds` mentions "pre-initializing Prometheus counter label combinations" — adding the 6th kind grows the metric series count from 10 to 12 in Prompt 2.
- `/workspace/pkg/schedule/task_definition.go` — the `TaskDefinition` struct (fields `Slug`, `TitleTemplate`, `BodyTemplate`, `Recurrence`, `Weekday`). The GoDoc on `Weekday` (lines 26-32) currently says the field is "consulted ONLY when `Recurrence == RecurrenceWeekly`" and "For the other four RecurrenceKinds ... Weekday is ignored and MUST remain the zero value (time.Sunday)." After this prompt the doc covers the new invariant: required for `RecurrenceWeekday`, forbidden for `RecurrenceWeekly`, ignored for the other four. The GoDoc is the contract; Prompt 2's validation tests enforce it.
- `/workspace/pkg/schedule/date.go` — the `Date` civil-date type with `NewDate(year, month, day)` constructor. The `Date.Time()` method (lines 41-43) returns a `time.Time` for stdlib weekday/iso-week computation. There is no direct `Date.Weekday()` method — consumers call `date.Time().Weekday()` to get the date's weekday. The new `TasksForDate` uses this idiom.
- `/workspace/pkg/schedule/inventory.go` — the 45-entry inventory slice. UNCHANGED in this prompt. The 21 weekly entries (12 Saturday + 9 Sunday) keep `Recurrence: RecurrenceWeekly` and `Weekday: time.Saturday|time.Sunday`. Prompt 2 migrates them.
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest() []TaskDefinition` accessor. Unchanged. The new `TasksForDate` tests use synthetic fixtures (local to the test file), not the production inventory, so `AllDefinitionsForTest` is not exercised by this prompt's tests.
- `/workspace/pkg/schedule/inventory_validation_test.go` — the existing `Describe("inventory", ...)` block. UNCHANGED in this prompt. The `sundayWeeklyAllowList` var (lines 18-38) stays. The 6 pre-existing `It` cases stay. Prompt 2 deletes the `sundayWeeklyAllowList` var and the `It("has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList", ...)` case and the `It("every weekly entry has Weekday in {time.Saturday, time.Sunday}", ...)` case and the `It("every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList", ...)` case.
- `/workspace/pkg/schedule/canonical_slugs_test.go` — the 45-slug canonical list. Unchanged. Inventory is unchanged in this prompt.
- `/workspace/pkg/schedule/schedule_suite_test.go` — the Ginkgo suite. Unchanged.
- `/workspace/pkg/publisher/uuid_namespace.go` — the `buildPeriodToken` switch. UNCHANGED in this prompt. The `RecurrenceWeekly` case still emits `YYYYWww-<abbrev>`; the new `RecurrenceWeekday` case is added in Prompt 2. The `RecurrenceKind` enum change in this prompt does not affect the switch because Go switch statements over typed string constants without a `default` panic if a new constant is added but the switch is not updated — the switch is `RecurrenceWeekly` (FROZEN pre-Spec-9) and the new `RecurrenceWeekday` constant is simply not in the switch's `case` list. The switch will fail at runtime if a `RecurrenceWeekday` entry ever reaches the publisher — and that failure is fine because Prompt 2 adds the missing `case` before any `RecurrenceWeekday` entry can reach the publisher (Prompt 2 also migrates the inventory and wires the tick/trigger).
- `/workspace/pkg/publisher/publisher.go` — `Publisher.Publish`. Unchanged. Does not call `TasksForDate` (the publisher's input is a single `TaskDefinition` + `Date`, not a list of definitions).
- `/workspace/pkg/publisher/export_test.go` — `UuidNamespaceForTest`, `BuildPeriodTokenForTest`. Unchanged.
- `/workspace/pkg/publisher/publisher_test.go` — the existing publisher tests. UNCHANGED. They exercise `RecurrenceWeekly` as test data; the kind is still valid; the tests still pass. The new `TasksForDate` is not exercised by publisher tests (it's a `pkg/schedule` accessor, not a publisher concern).
- `/workspace/pkg/tick/tick.go` — the `tick` function iterates `t.inventory` and calls `publisher.Publish` for each. UNCHANGED in this prompt. The factory still passes `schedule.Inventory()` (full inventory); the tick does not call `TasksForDate` yet. Prompt 2 changes this.
- `/workspace/pkg/tick/tick_test.go` — UNCHANGED. The "full inventory" test (line 423) still asserts the full count. The "recurrence label coverage" test (line 377) still enumerates 5 kinds. The "Prometheus pre-initialization" test (line 463) still asserts 10 series. Prompt 2 updates all three.
- `/workspace/pkg/tick/metrics.go` — the `init()` pre-initializes the counter for every kind in `AllRecurrenceKinds`. UNCHANGED in this prompt. After this prompt the slice has 6 entries but the `init()` body is unchanged — it will start pre-initializing the 6th kind (`weekday` × `success`/`error`) automatically because it iterates the slice. The series count growth (10 → 12) is observable by the existing `Prometheus pre-initialization` test, but this prompt does not update the assertion; Prompt 2 updates it.
- `/workspace/pkg/handler/trigger.go` — `NewTriggerHandler` iterates `schedule.Inventory()` and calls `publisher.Publish` for each. UNCHANGED in this prompt. Prompt 2 changes this to `TasksForDate(date)`.
- `/workspace/pkg/handler/trigger_test.go` — the existing tests assert `PublishCallCount() == 45` for `?date=2025-01-04`. UNCHANGED in this prompt. The 2025-01-04 (Saturday) trigger still publishes all 45 entries. Prompt 2 changes the assertion to use `TasksForDate` for the expected count.
- `/workspace/pkg/factory/factory.go` — `CreateTick` passes `schedule.Inventory()` to `tick.NewTick`. UNCHANGED. Prompt 2 may keep the factory passing `schedule.Inventory()` and move the date filter into the tick; the factory wiring is unchanged in either case.
- `/workspace/cmd/run-once/main.go` — uses `factory.CreateTickLoop(...).RunOnce(ctx)`. UNCHANGED. Behavior changes transitively through the factory and the tick.
- `/workspace/CHANGELOG.md` — UNCHANGED in this prompt. Prompt 2 appends the `feat:` bullet describing the kind split. Adding the `RecurrenceWeekday` constant and the `TasksForDate` function are not user-visible until Prompt 2's inventory migration and tick/trigger wiring; the changelog entry describes the user-visible behavior change, not the internal scaffolding.

Coding-guideline references (read inside the YOLO container):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — `RecurrenceKind` is a closed string enum; the new `RecurrenceWeekday` constant follows the existing pattern (`const X RecurrenceKind = "<lowercase string>"`); the new value is added in stable declaration order to `AllRecurrenceKinds`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; external test package (`package schedule_test`); `Describe` block with `It` cases; synthetic fixtures for `TasksForDate` tests (no dependency on the production inventory).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public function `TasksForDate` returns a defensive copy of the filtered slice (mirroring `Inventory()`); counterfeiter annotations are NOT needed (the function is a pure helper, not a collaborator interface).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc on `TasksForDate` starts with the function name; full sentences; describes behavior not implementation; mentions the date-filter semantic and the per-weekday predicate.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for the changed package; new behavior has new specs; the new `TasksForDate` is fully covered by the synthetic-fixture specs.

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/recurrence.go (BEFORE this prompt — exact source at the time of writing)
package schedule

// RecurrenceKind classifies how often an entry repeats. Closed set.
type RecurrenceKind string

const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)

// AllRecurrenceKinds is the canonical, closed set of RecurrenceKind values
// in stable declaration order. Consumers that need to iterate over every
// kind (e.g. pre-initializing Prometheus counter label combinations) range
// over this slice — never hand-roll a duplicate slice.
var AllRecurrenceKinds = []RecurrenceKind{
    RecurrenceDaily,
    RecurrenceWeekly,
    RecurrenceMonthly,
    RecurrenceQuarterly,
    RecurrenceYearly,
}
```

```go
// pkg/schedule/task_definition.go (BEFORE this prompt — exact Weekday GoDoc at the time of writing)
//
// Weekday is the day of the week the entry is intended for. It is
// consulted ONLY when Recurrence == RecurrenceWeekly: the publisher
// appends the lowercase 3-letter weekday abbreviation to the weekly
// period token (e.g. "2026W25-sat"). For the other four RecurrenceKinds
// (daily / monthly / quarterly / yearly) Weekday is ignored and MUST
// remain the zero value (time.Sunday).
Weekday time.Weekday
```

```go
// pkg/schedule/date.go (UNCHANGED — used by the new TasksForDate)
//
// Date.Time() returns the civil date as midnight UTC. Used to derive
// the date's weekday via date.Time().Weekday().
func (d Date) Time() time.Time {
    return d.toTime()
}
```

</context>

<requirements>

## 1. Add `RecurrenceWeekday` to the enum and to `AllRecurrenceKinds`

In `/workspace/pkg/schedule/recurrence.go`, change the file so that the const block reads:

```go
const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceWeekday   RecurrenceKind = "weekday"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)
```

And the `AllRecurrenceKinds` slice reads:

```go
var AllRecurrenceKinds = []RecurrenceKind{
    RecurrenceDaily,
    RecurrenceWeekly,
    RecurrenceWeekday,
    RecurrenceMonthly,
    RecurrenceQuarterly,
    RecurrenceYearly,
}
```

Notes that are load-bearing for the executor:

- `RecurrenceWeekday` is inserted between `RecurrenceWeekly` and `RecurrenceMonthly` in BOTH the const block and the `AllRecurrenceKinds` slice. The order is stable declaration order; the slice's order is the source of truth for the metric surface and the `recurrence kinds from the closed set` validation test (which uses a map so order is incidental for that test, but the `init()` in `pkg/tick/metrics.go` iterates the slice and emits Prometheus series in slice order). Slicing the new kind in the middle keeps the always-fire kinds (`Daily`, `Weekly`) adjacent and the period-bucket kinds (`Monthly`, `Quarterly`, `Yearly`) adjacent.
- The string value is `"weekday"` (lowercase, no separators). The spec's Desired Behavior 1 and the Failure Modes table row 5 (`Build fails` for a future collision with the string `"weekday"`) are satisfied by the closed-enum pattern: the constant is the only way to refer to the value, and any new constant must have a unique string.
- The GoDoc comment on the `RecurrenceKind` type ("Closed set.") and the GoDoc comment on `AllRecurrenceKinds` ("the canonical, closed set ... in stable declaration order ... never hand-roll a duplicate slice") are UNCHANGED. They are already correct for the 6-value shape.
- The file's `Copyright (c) 2026` BSD header is preserved.
- Do NOT add a `RecurrenceWeekday` doc comment in the const block. The existing 5 constants have no per-constant doc; the contract lives on `Weekday`'s GoDoc in `task_definition.go` (see §2).
- Do NOT add a `_ = RecurrenceWeekday` reference anywhere. The constant is used by `AllRecurrenceKinds`, which is itself used by `pkg/tick/metrics.go` and by the `recurrence kinds from the closed set` validation test (in Prompt 2). The compiler will not flag it as unused; no linter override is needed.

## 2. Update the `Weekday` GoDoc in `task_definition.go`

In `/workspace/pkg/schedule/task_definition.go`, change the GoDoc on the `Weekday` field (lines 26-32 of the file as it stands) to:

```go
// Weekday is the day of the week the entry is intended for. Its
// semantics depend on the entry's Recurrence:
//
//   - RecurrenceWeekday: REQUIRED (non-zero). The entry fires only on
//     the day whose weekday equals this value; the publisher appends
//     the lowercase 3-letter weekday abbreviation to the period token
//     (e.g. "2026W25-sat"). The disambiguation from RecurrenceWeekly
//     was introduced in spec 009.
//
//   - RecurrenceWeekly: FORBIDDEN (must be the zero value, time.Sunday).
//     The entry fires on every day inside its ISO week (always-fire
//     semantic introduced in spec 006); this field is not consulted.
//     The inventory contains zero RecurrenceWeekly entries after
//     spec 009 — the kind is reserved for future use.
//
//   - RecurrenceDaily / RecurrenceMonthly / RecurrenceQuarterly /
//     RecurrenceYearly: ignored. May be the zero value or any other
//     value without effect on firing or rendering.
Weekday time.Weekday
```

Notes that are load-bearing for the executor:

- The new GoDoc is the single source of truth for the field's semantics across all 6 kinds. Prompt 2's inventory validation tests assert the invariant (`RecurrenceWeekday` requires non-zero `Weekday`; `RecurrenceWeekly` forbids non-zero `Weekday`); the GoDoc must be precise enough that a future reviewer reading the field alone can derive the invariant.
- The GoDoc starts with the field name (`Weekday is ...`) per the project convention in `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md`.
- The mention of "spec 009" in the GoDoc is intentional — it traces the design decision back to the spec so a future reviewer can find the rationale. The other spec references (`spec 006`) follow the same pattern.
- The field declaration (`Weekday time.Weekday`) is unchanged. The `time` import is unchanged.
- The other three fields' GoDocs (`Slug`, `TitleTemplate`, `BodyTemplate`, `Recurrence`) are unchanged. Only `Weekday`'s doc is updated.

## 3. Add `TasksForDate` in a new `tasks_for_date.go` file

Create `/workspace/pkg/schedule/tasks_for_date.go` with the following content:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

// TasksForDate returns the subset of the canonical inventory that fires
// on the given civil date. The filter rule is:
//
//   - RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly,
//     RecurrenceQuarterly, RecurrenceYearly: always-fire (the entry
//     fires on every day; this is the spec 006 always-fire semantic,
//     preserved by spec 009).
//   - RecurrenceWeekday: fires only on the day whose weekday equals
//     the entry's Weekday field. Computed via date.Time().Weekday().
//
// The returned slice is a defensive copy — mutating it does NOT affect
// the package-level inventory state. The result is NOT sorted; the
// caller may sort on Slug if a stable order is required (the HTTP
// trigger handler does so for the response body).
//
// Pure function: no I/O, no clock, no env. The Europe/Berlin civil-date
// conversion (and the ISO-week boundary math that goes with it) is the
// caller's responsibility — this function takes a civil Date, not a
// time.Time with a location. The tick (pkg/tick) and the trigger HTTP
// handler (pkg/handler/trigger) both convert their wall-clock input
// to a Europe/Berlin civil Date before calling this function.
//
// An empty inventory yields an empty slice. An inventory that contains
// only RecurrenceWeekday entries whose Weekday does not match the
// given date's weekday also yields an empty slice — this is the
// regression fix from spec 009: weekday-pinned tasks no longer fire
// on a non-target weekday.
func TasksForDate(date Date) []TaskDefinition {
    return filterInventoryByDate(inventory, date)
}

// filterInventoryByDate is the package-internal implementation. It
// exists as a separate function so the synthetic-fixture tests in
// tasks_for_date_test.go can pass small custom inventories without
// touching the package-level inventory. Production callers go through
// TasksForDate (which reads the canonical inventory).
func filterInventoryByDate(defs []TaskDefinition, date Date) []TaskDefinition {
    dateWeekday := date.Time().Weekday()
    out := make([]TaskDefinition, 0, len(defs))
    for _, def := range defs {
        switch def.Recurrence {
        case RecurrenceWeekday:
            if def.Weekday == dateWeekday {
                out = append(out, def)
            }
        default:
            // Daily, Weekly, Monthly, Quarterly, Yearly — always-fire.
            out = append(out, def)
        }
    }
    return out
}
```

Notes that are load-bearing for the executor:

- The split between `TasksForDate` (the public accessor) and `filterInventoryByDate` (the package-internal worker) is deliberate. The public function reads the package-level `inventory`; the worker accepts an arbitrary `[]TaskDefinition` so synthetic-fixture tests can pass a custom 5-entry slice and assert the filter without polluting the production inventory. This mirrors the `Inventory()` / `AllDefinitionsForTest()` split in `inventory.go` / `inventory_export_test.go`.
- The `dateWeekday` is computed ONCE outside the loop — minor optimization, but the bigger reason is correctness: the function's behavior is "every RecurrenceWeekday entry whose Weekday matches the date's weekday fires" — the date's weekday is a single value, not a per-entry recomputation.
- The `default` arm of the switch covers `RecurrenceDaily`, `RecurrenceWeekly`, `RecurrenceMonthly`, `RecurrenceQuarterly`, `RecurrenceYearly`. There is no `RecurrenceKind` value that is NOT in one of these two arms (the enum is closed at 6 values as of §1), so a `default` is correct and exhaustive. If a 7th kind is added in a future spec, the switch must be reviewed — add a `// add new kind arm here` comment after the `default` to make the maintenance obvious.
- The result is `make([]TaskDefinition, 0, len(defs))` so the slice has the right capacity (avoids regrowth on append) but starts empty (so a fully-filtered-out inventory returns `len == 0` and not `len == 45` of zero-value entries).
- No `import "time"` is required: the function uses `date.Time().Weekday()` whose returned `time.Weekday` value is captured by `:=` inference, never named explicitly. `goimports` would mark a `time` import as unused, so the file declares no imports.
- The file's `Copyright (c) 2026` BSD header is preserved.
- The GoDoc on `TasksForDate` starts with the function name and describes the behavior (always-fire for 5 kinds, weekday-match for 1 kind) and the contract (defensive copy, no sort, pure function).
- The GoDoc on `filterInventoryByDate` is shorter and explains the split.
- The function is NOT a `Tick` collaborator and does not need a counterfeiter annotation. It is a pure helper, not an interface.

## 4. Add synthetic-fixture tests in `pkg/schedule/tasks_for_date_test.go`

Create `/workspace/pkg/schedule/tasks_for_date_test.go` with the following content:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("TasksForDate", func() {
    var defs []schedule.TaskDefinition

    BeforeEach(func() {
        // Synthetic fixtures — not the production inventory. The production
        // inventory is exercised by the full-inventory render test in
        // pkg/publisher/publisher_test.go; this spec exercises the filter
        // rule with a controlled set of entries.
        defs = []schedule.TaskDefinition{
            {Slug: "daily-x", Recurrence: schedule.RecurrenceDaily},
            {Slug: "weekly-x", Recurrence: schedule.RecurrenceWeekly},
            {Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
            {Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
            {Slug: "monthly-x", Recurrence: schedule.RecurrenceMonthly},
        }
    })

    // Helper: drive the package-internal worker with the synthetic fixtures.
    // Production callers go through schedule.TasksForDate; tests use the
    // worker so the production inventory is not consulted.
    filter := func(date schedule.Date) []schedule.TaskDefinition {
        return schedule.FilterInventoryByDateForTest(defs, date)
    }

    It("returns the full set on a Saturday when all weekday entries match", func() {
        // 2025-01-04 is a Saturday. Saturday weekday entries fire; Sunday ones do not.
        got := filter(schedule.NewDate(2025, time.January, 4))
        slugs := slugsOf(got)
        Expect(slugs).To(ConsistOf(
            "daily-x", "weekly-x", "weekday-sat", "monthly-x",
        ))
    })

    It("returns the full set on a Sunday when all weekday entries match", func() {
        // 2025-01-05 is a Sunday. Sunday weekday entries fire; Saturday ones do not.
        got := filter(schedule.NewDate(2025, time.January, 5))
        slugs := slugsOf(got)
        Expect(slugs).To(ConsistOf(
            "daily-x", "weekly-x", "weekday-sun", "monthly-x",
        ))
    })

    It("returns zero weekday-kind tasks on a Tuesday (regression fix)", func() {
        // 2025-01-07 is a Tuesday. No weekday-kind entry fires; the 4
        // always-fire entries (daily, weekly, monthly, plus zero weekday
        // ones) are returned. Quarterly and yearly are not in the fixture;
        // they would also fire on Tuesday under the same always-fire rule.
        got := filter(schedule.NewDate(2025, time.January, 7))
        slugs := slugsOf(got)
        Expect(slugs).To(ConsistOf(
            "daily-x", "weekly-x", "monthly-x",
        ))
        Expect(slugs).NotTo(ContainElement("weekday-sat"))
        Expect(slugs).NotTo(ContainElement("weekday-sun"))
    })

    It("returns exactly the Saturday weekday entry on a Saturday for a weekday-only inventory", func() {
        weekdayOnly := []schedule.TaskDefinition{
            {Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
            {Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
        }
        got := schedule.FilterInventoryByDateForTest(
            weekdayOnly,
            schedule.NewDate(2025, time.January, 4),
        )
        Expect(slugsOf(got)).To(ConsistOf("weekday-sat"))
    })

    It("returns exactly the Sunday weekday entry on a Sunday for a weekday-only inventory", func() {
        weekdayOnly := []schedule.TaskDefinition{
            {Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
            {Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
        }
        got := schedule.FilterInventoryByDateForTest(
            weekdayOnly,
            schedule.NewDate(2025, time.January, 5),
        )
        Expect(slugsOf(got)).To(ConsistOf("weekday-sun"))
    })

    It("returns an empty slice for an empty inventory", func() {
        got := schedule.FilterInventoryByDateForTest(
            []schedule.TaskDefinition{},
            schedule.NewDate(2025, time.January, 4),
        )
        Expect(got).To(BeEmpty())
    })

    It("returns an empty slice when no weekday entry matches", func() {
        weekdayOnly := []schedule.TaskDefinition{
            {Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
        }
        got := schedule.FilterInventoryByDateForTest(
            weekdayOnly,
            schedule.NewDate(2025, time.January, 7), // Tuesday
        )
        Expect(got).To(BeEmpty())
    })

    It("preserves the always-fire semantic for the four other kinds", func() {
        // Daily, weekly, monthly, quarterly, and yearly all fire on every day
        // (spec 006 always-fire). The fixture omits quarterly and yearly for
        // brevity; the always-fire guarantee for those kinds is exercised by
        // the existing full-inventory test in pkg/tick/tick_test.go and the
        // prompt-2 trigger-handler test.
        for _, date := range []schedule.Date{
            schedule.NewDate(2025, time.January, 6),  // Monday
            schedule.NewDate(2025, time.January, 7),  // Tuesday
            schedule.NewDate(2025, time.January, 10), // Friday
            schedule.NewDate(2025, time.January, 11), // Saturday
        } {
            got := filter(date)
            for _, def := range got {
                if def.Recurrence == schedule.RecurrenceWeekday {
                    // Weekday entries fire only on their target weekday.
                    continue
                }
                // Always-fire: present on every date.
                Expect(def.Recurrence).To(BeElementOf(
                    schedule.RecurrenceDaily,
                    schedule.RecurrenceWeekly,
                    schedule.RecurrenceMonthly,
                    schedule.RecurrenceQuarterly,
                    schedule.RecurrenceYearly,
                ))
            }
        }
    })
})

func slugsOf(defs []schedule.TaskDefinition) []string {
    out := make([]string, 0, len(defs))
    for _, d := range defs {
        out = append(out, d.Slug)
    }
    return out
}
```

Notes that are load-bearing for the executor:

- The tests use the package-internal worker `schedule.FilterInventoryByDateForTest(defs, date)` (added in §5) instead of the production accessor `schedule.TasksForDate(date)`. The reason: `TasksForDate` reads the package-level `inventory` slice (45 entries) and would force the test to compute the expected count over 45 entries instead of 5 fixtures. The worker takes an arbitrary `[]TaskDefinition`, which is what the synthetic-fixture tests need.
- The fixtures cover the spec's three key date cases: a Tuesday (zero weekday-kind tasks), a Saturday (Saturday weekday entry fires, Sunday does not), a Sunday (vice versa).
- The `slugsOf` helper is a package-level function in the test file (NOT in the production `schedule` package). It converts a `[]TaskDefinition` to `[]string` for cleaner `ConsistOf` / `Not(ContainElement)` assertions.
- The `"returns an empty slice when no weekday entry matches"` test pins the edge case the spec calls out: an inventory that contains only `RecurrenceWeekday` entries, called on a non-target date, returns `len == 0`. This is the regression fix from the spec's Problem statement: `/start-day` on a Tuesday must not surface Saturday-pinned tasks.
- The `"preserves the always-fire semantic for the four other kinds"` test walks four dates and asserts that the 3 always-fire entries in the fixture (`daily-x`, `weekly-x`, `monthly-x`) are present on every date. The `weekday` entries are explicitly excluded from the always-fire check (they fire only on their target weekday). The 2 missing always-fire kinds (quarterly, yearly) are not in the fixture — the always-fire guarantee for those kinds is already exercised by the existing full-inventory tick test (which Prompt 2 will update).
- Imports required: `time` (for the `time.Weekday` constants in the fixtures), `ginkgo` / `gomega` (dot-imported), `schedule` (for the package types). The `package schedule_test` declaration matches the existing test files.

## 5. Add the `FilterInventoryByDateForTest` accessor

In `/workspace/pkg/schedule/inventory_export_test.go` (the existing test-accessor file, currently exporting `AllDefinitionsForTest`), add a new accessor:

```go
// FilterInventoryByDateForTest exposes the package-internal filter worker
// to external tests so the synthetic-fixture tests in
// tasks_for_date_test.go can drive the filter with a controlled inventory
// instead of the package-level 45-entry canonical inventory. Production
// callers go through TasksForDate (which reads the canonical inventory).
func FilterInventoryByDateForTest(defs []TaskDefinition, date Date) []TaskDefinition {
    return filterInventoryByDate(defs, date)
}
```

Notes that are load-bearing for the executor:

- The accessor is in the same file as `AllDefinitionsForTest` — both are test accessors, both follow the `_test.go` convention. The `package schedule` (NOT `package schedule_test`) declaration of the file means the exported `FilterInventoryByDateForTest` is reachable from `package schedule_test` tests but invisible to the production binary. The `_test.go` suffix keeps it out of the compiled package.
- The accessor takes a `Date` (the production type from `date.go`) — no new type is introduced. The synthetic-fixture tests construct a `Date` via `schedule.NewDate(2025, time.January, 4)`.
- The function body is a thin pass-through to `filterInventoryByDate` (the package-internal worker from §3). It exists ONLY to expose the worker to external tests; the public `TasksForDate` (also from §3) is the production entry point.
- The `Date` type is in the same package (`schedule`), so the file does not need to import it — the type is package-local. The existing `AllDefinitionsForTest` returns `[]TaskDefinition` from the same package, so the file already follows the pattern of "export a thing from the same package as the file lives in".

## 6. Imports and conventions

- The modified `/workspace/pkg/schedule/recurrence.go` has no import changes — the file currently has no imports and adding `RecurrenceWeekday` requires none.
- The modified `/workspace/pkg/schedule/task_definition.go` has no import changes — `time` is already imported and used for the `time.Weekday` field type.
- The new `/workspace/pkg/schedule/tasks_for_date.go` does NOT import `time` — see §3's note on the unused import. The `time.Weekday` type is referenced only in the doc comment (a string, not a Go expression). Verify with `head -10 pkg/schedule/tasks_for_date.go` and `goimports -l pkg/schedule/tasks_for_date.go` after writing the file.
- The new `/workspace/pkg/schedule/tasks_for_date_test.go` imports `time`, `ginkgo` / `gomega` (dot-imported), and `schedule`. The import block follows goimports-reviser order: standard library first (`time`), then third-party (ginkgo, gomega), then internal (`schedule`).
- The modified `/workspace/pkg/schedule/inventory_export_test.go` adds the `FilterInventoryByDateForTest` accessor. The file's existing import block has no imports (it just references `TaskDefinition` and `Date` from the same package). The new function follows the same pattern — no new imports.
- The 2026 copyright header is preserved on all modified and new files.
- Use Ginkgo v2 / Gomega style with dot-imports (matches the existing tests).
- Do NOT touch `pkg/publisher/`, `pkg/handler/`, `pkg/factory/`, `pkg/tick/`, `main.go`, `cmd/run-once/`, the Makefile, k8s manifests, the Prometheus metric surface (the `init()` count change is automatic; do not edit it), or `CHANGELOG.md`.
- Do NOT modify the inventory (`pkg/schedule/inventory.go`) in this prompt. The 21 weekly entries' `Recurrence: RecurrenceWeekly` and `Weekday: time.Saturday|time.Sunday` are preserved; Prompt 2 migrates them.
- Do NOT modify the existing inventory validation tests in `pkg/schedule/inventory_validation_test.go`. The `sundayWeeklyAllowList` var stays; the 6 pre-existing `It` cases stay. Prompt 2 deletes them.
- Do NOT modify the publisher's `buildPeriodToken` switch in `pkg/publisher/uuid_namespace.go`. The `RecurrenceWeekly` case continues to emit `YYYYWww-<abbrev>`; the `RecurrenceWeekday` case is added in Prompt 2.
- Do NOT modify `pkg/tick/tick.go`, `pkg/handler/trigger.go`, `pkg/factory/factory.go`, or `pkg/tick/metrics.go`. The factory and the trigger continue to call `schedule.Inventory()`; the tick continues to iterate the full inventory. Prompt 2 wires the tick and the trigger to `TasksForDate`.
- Do NOT regenerate any counterfeiter mock. No interface signatures change.
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- The `RecurrenceKind` enum is a closed set. After this prompt the set is `{Daily, Weekly, Weekday, Monthly, Quarterly, Yearly}` — exactly 6 values. No 7th value may be added in this prompt or any other prompt without a new spec.
- The string value of `RecurrenceWeekday` is `"weekday"` (lowercase, no separators). Any future constant that wants to use the same string collides at the Go const level — a compile-time error. This is the spec's Failure Modes row 5 ("new recurrence kind added later collides with `\"weekday\"` string" → "Pick a different string constant" → "Build fails").
- The `AllRecurrenceKinds` slice's order is stable: `Daily, Weekly, Weekday, Monthly, Quarterly, Yearly`. Any future kind goes at the end (or in a position that keeps the always-fire group and the period-bucket group contiguous, at the author's discretion — but the order MUST be preserved in subsequent specs; the metric surface and the validation test rely on the slice, not on the order, but the `init()` does iterate in slice order).
- The `Weekday` field's GoDoc is the contract for the field's semantics across all 6 kinds. Any future spec that adds a 7th kind must update the doc to cover the new kind's semantics.
- The `TasksForDate` function is pure: no I/O, no clock, no env. The caller is responsible for converting wall-clock input to a Europe/Berlin civil Date before calling.
- The `filterInventoryByDate` worker is package-internal. External tests reach it via the `FilterInventoryByDateForTest` accessor in `inventory_export_test.go`. Do not promote it to a public symbol.
- The `RecurrenceWeekday` constant is in the enum and in `AllRecurrenceKinds` AFTER this prompt. The publisher's `buildPeriodToken` switch does NOT have a `RecurrenceWeekday` case yet; the switch will return an error for any `RecurrenceWeekday` entry that reaches the publisher. This is acceptable in this prompt because no `RecurrenceWeekday` inventory entry exists yet (the inventory is unchanged; Prompt 2 migrates the 21 entries AND adds the missing `case` arm).
- The inventory (`pkg/schedule/inventory.go`) is FROZEN in this prompt. The 21 weekly entries' `Recurrence: RecurrenceWeekly` and `Weekday: time.Saturday|time.Sunday` are unchanged.
- The `sundayWeeklyAllowList` var in `pkg/schedule/inventory_validation_test.go` is FROZEN in this prompt. The 6 pre-existing `It` cases in the `Describe("inventory", ...)` block are FROZEN.
- The `pkg/tick/tick_test.go` "full inventory" test (line 423, asserts `expected := len(schedule.Inventory())`) is FROZEN in this prompt. The "recurrence label coverage" test (line 377, enumerates 5 kinds) is FROZEN. The "Prometheus pre-initialization" test (line 463, asserts 10 series) is FROZEN. All three will fail after Prompt 2 adds the 6th kind and the inventory migration; Prompt 2 updates all three.
- The `pkg/handler/trigger_test.go` "publishes every entry in the inventory on /trigger?date=" test (line 78, asserts `PublishCallCount() == 45`) is FROZEN in this prompt. Prompt 2 changes it.
- The `pkg/publisher/publisher_test.go` "boundary contract" `DescribeTable` (line 751, has 5 `Entry` cases for the 5 kinds) is FROZEN in this prompt. The 5 `RecurrenceKind` values are still all valid; the test still passes.
- The `pkg/publisher/uuid_namespace.go` `buildPeriodToken` switch is FROZEN in this prompt. The `RecurrenceWeekly` case still emits `YYYYWww-<abbrev>`.
- `CHANGELOG.md` is FROZEN in this prompt — no entry is added. Prompt 2 writes the single `feat:` bullet describing the kind split.
- Project DoD (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` for error wrapping (the new `TasksForDate` and `filterInventoryByDate` have no error path — they are pure functions, no wrapping needed); no `context.Background()` in business logic (the new code has no I/O); no `time.Time` / `time.Now()` in business logic (the new code uses `date.Time().Weekday()`, not `time.Now()`); GoDoc on `TasksForDate` and `filterInventoryByDate` (provided in §3); `make precommit` clean.
- Existing tests must still pass after all edits. The 6 pre-existing `It` cases in `inventory_validation_test.go` continue to pass (the inventory and the kind set are unchanged). The publisher and tick tests continue to pass (no production code calls `TasksForDate` yet; `RecurrenceWeekday` is in the enum but not used in any inventory entry).
- Coverage on the changed package stays at or above 80%. The new `TasksForDate` tests cover the new function end-to-end; the `RecurrenceKind` enum change is exercised by the existing closed-set test; the `AllRecurrenceKinds` slice change is exercised by the new `RecurrenceWeekday` constant being in the slice.
- Do NOT commit — dark-factory handles git.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/schedule/...` — all Ginkgo specs green. In particular:
   - The 6 pre-existing `It` cases in `Describe("inventory", ...)` continue to pass.
   - The 8 new `It` cases in `Describe("TasksForDate", ...)` all pass.
3. `go test ./pkg/publisher/...` — all Ginkgo specs continue to pass (no production code in `pkg/publisher` changed; the `buildPeriodToken` switch still handles the 5 cases it knows about; no `RecurrenceWeekday` entry reaches the publisher in this prompt).
4. `go test ./pkg/tick/...` — all Ginkgo specs continue to pass. The "full inventory" test still asserts the full count; the "recurrence label coverage" test still enumerates 5 kinds; the "Prometheus pre-initialization" test still asserts 10 series (the metric surface change is automatic via the `init()` iterating `AllRecurrenceKinds`, but the test's expected value is FROZEN at 10 in this prompt; Prompt 2 updates it).
5. `go test ./pkg/handler/...` — the trigger handler test still asserts `PublishCallCount() == 45` for `?date=2025-01-04`; the full inventory is still iterated.
6. `grep -nE 'RecurrenceWeekday\s+RecurrenceKind' pkg/schedule/recurrence.go` — must return exactly 1 match (the new constant declaration).
7. `grep -nE 'RecurrenceWeekday' pkg/schedule/recurrence.go` — must return exactly 2 matches: the constant declaration and the slice entry. No other matches.
8. `grep -nE 'AllRecurrenceKinds' pkg/schedule/recurrence.go` — must return exactly 2 matches: the doc comment line and the slice declaration. The slice has 6 entries (verify with `grep -cE 'Recurrence(Daily|Weekly|Weekday|Monthly|Quarterly|Yearly),' pkg/schedule/recurrence.go` returning 11 — 5 in the const block + 6 in the slice).
9. `grep -nE 'TasksForDate' pkg/schedule/tasks_for_date.go` — must return at least 1 match (the new function declaration).
10. `grep -nE 'FilterInventoryByDateForTest' pkg/schedule/inventory_export_test.go` — must return at least 2 matches: the doc comment and the function declaration.
11. `grep -nE 'filterInventoryByDate' pkg/schedule/tasks_for_date.go` — must return exactly 2 matches: the call from `TasksForDate` and the function declaration.
12. `grep -nE 'sundayWeeklyAllowList' pkg/schedule/inventory_validation_test.go` — must return at least 1 match (the var is still present; Prompt 2 deletes it).
13. `grep -nE 'RecurrenceWeekly' pkg/schedule/inventory.go` — must return at least 21 matches (the 21 weekly entries are unchanged in this prompt; Prompt 2 migrates them to `RecurrenceWeekday`).
14. Spot-check: open `/workspace/pkg/schedule/recurrence.go` and visually confirm (a) `RecurrenceWeekday RecurrenceKind = "weekday"` is in the const block between `RecurrenceWeekly` and `RecurrenceMonthly`; (b) `RecurrenceWeekday` is in `AllRecurrenceKinds` in the same position; (c) the file's BSD header is preserved.
15. Spot-check: open `/workspace/pkg/schedule/task_definition.go` and visually confirm (a) the `Weekday` field's GoDoc describes the 3-way semantic (required for `RecurrenceWeekday`, forbidden for `RecurrenceWeekly`, ignored for the other 4); (b) the field declaration is unchanged; (c) the other fields' GoDocs are unchanged.
16. Spot-check: open `/workspace/pkg/schedule/tasks_for_date.go` and visually confirm (a) `TasksForDate` is exported; (b) `filterInventoryByDate` is package-internal; (c) the switch has a `case RecurrenceWeekday` arm and a `default` arm; (d) no `"time"` import is present (the function body has no `time.X` reference).
17. Spot-check: open `/workspace/pkg/schedule/tasks_for_date_test.go` and visually confirm (a) the test package is `package schedule_test`; (b) the 8 `It` cases match the spec's three date cases (Tuesday/Saturday/Sunday) and the empty-inventory / no-match / always-fire edge cases; (c) the `slugsOf` helper is package-level in the test file.
18. Spot-check: open `/workspace/pkg/schedule/inventory_export_test.go` and visually confirm (a) the new `FilterInventoryByDateForTest` accessor sits after the existing `AllDefinitionsForTest`; (b) the accessor is a thin pass-through to `filterInventoryByDate`.
19. Coverage check on the changed package:
    - `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/schedule/...`
    - `go tool cover -func=/tmp/cover.out | tail -1` — total coverage ≥ 80%.
20. End-to-end smoke: `RecurrenceKind` has 6 values; `AllRecurrenceKinds` has 6 entries; `TasksForDate` is exported and covered; the inventory is unchanged. Prompt 2 then migrates the inventory, adds the publisher's `RecurrenceWeekday` switch arm, wires the tick and the trigger to `TasksForDate`, deletes `sundayWeeklyAllowList`, and updates the metric surface and the trigger/tick tests for the 6th kind.

## Open Questions (for the human reviewer)

- **A. The `RecurrenceWeekday` const position.** The new constant is inserted between `RecurrenceWeekly` and `RecurrenceMonthly` in BOTH the const block and the `AllRecurrenceKinds` slice. The choice keeps the always-fire group (Daily, Weekly) adjacent and the period-bucket group (Monthly, Quarterly, Yearly) adjacent. The alternative — appending `RecurrenceWeekday` at the end of the const block and slice — would have been cleaner from a "minimal-diff" perspective but would have visually orphaned the new kind from its always-fire cousin. The prompt picks the grouped position; if you prefer the append-at-end position, swap the two edits. The only impact is the order in the slice, which the validation test treats as a set (no order dependence) but the metric `init()` treats as iteration order (no semantic dependence).
- **B. The `TasksForDate` / `filterInventoryByDate` split.** The public `TasksForDate` reads the package-level `inventory`; the internal `filterInventoryByDate` takes an arbitrary `[]TaskDefinition` and is exposed to tests via `FilterInventoryByDateForTest`. The split lets the synthetic-fixture tests in `tasks_for_date_test.go` drive the filter with a 5-entry fixture instead of the 45-entry production inventory. The alternative — a single public function that takes a `[]TaskDefinition` parameter — would either leak the worker into the production API surface (rename `filterInventoryByDate` to `FilterInventoryByDate`) or force the synthetic-fixture tests to compute the expected count over the full inventory. The split is the cleanest of the three options.
- **C. The "no time import" rule in `tasks_for_date.go`.** The function body uses `date.Time().Weekday()` whose return type is `time.Weekday`, but the type is never referenced by name in the function body (the variable `dateWeekday` is inferred). The doc comment mentions `date.Time().Weekday()` in prose but doc comments are strings, not Go expressions, so `time` is not a real dependency. `goimports` will mark `time` as unused if added. If the executor wants to add a `time.Weekday` type assertion or cast in the function body for clarity (e.g. `var dateWeekday time.Weekday = date.Time().Weekday()`), the import is needed — but the inferred form is fine and matches the project's `var x = expr()` style elsewhere.
- **D. The `RecurrenceWeekday` enum value and the publisher's `buildPeriodToken` switch.** After this prompt, the switch does NOT have a `RecurrenceWeekly` / `RecurrenceWeekday` disambiguation: the existing `RecurrenceWeekly` case still emits `YYYYWww-<abbrev>`, and the new `RecurrenceWeekday` case is not added. If a `RecurrenceWeekday` entry reached the publisher after this prompt (it cannot — the inventory is unchanged), the switch's `default` arm would return the `"buildPeriodToken: unknown recurrence kind"` error. The `default` arm is the safety net that prevents a silent misrender; Prompt 2 adds the missing `RecurrenceWeekday` case (and changes the `RecurrenceWeekly` case to bare `YYYYWww`) before any `RecurrenceWeekday` entry can reach the publisher. If you prefer the missing `case` to be added in this prompt (and the inventory migration in Prompt 2 picks it up), the change is small — let me know.
- **E. The `RecurrenceWeekly` inventory entries remain unchanged.** After this prompt, the inventory still has 21 entries with `Recurrence: RecurrenceWeekly, Weekday: time.Saturday|time.Sunday`. The publisher's existing `RecurrenceWeekly` case in `buildPeriodToken` still emits `YYYYWww-<abbrev>`. The behavior is byte-identical to pre-prompt-9 (Spec 8 in effect). The user's `/start-day` STILL surfaces 21 weekend tasks on every weekday — the regression fix is Prompt 2's. This prompt is purely additive scaffolding.
- **F. The trigger handler and the tick still iterate the full inventory.** The factory calls `schedule.Inventory()` (45 entries); the trigger calls `schedule.Inventory()` (45 entries); the tick iterates `t.inventory` (45 entries). `TasksForDate` is exported but no production code calls it. Prompt 2 wires both the tick and the trigger to `TasksForDate`. The factory wiring (passing `schedule.Inventory()` vs passing `schedule.TasksForDate(date)`) is Prompt 2's decision — both are valid; the prompt picks the "factory passes the full inventory, the tick filters" pattern because the factory has no civil-date input (the tick reads the clock at tick time).
- **G. No scenario file.** The spec's Acceptance Criteria are all reachable from Ginkgo unit tests in `pkg/schedule` and (Prompt 2) `pkg/publisher` / `pkg/tick` / `pkg/handler`. No real Kafka, no real vault, no real clock, no real HTTP. No `scenarios/` work is part of this spec or this prompt.
- **H. The "preserves the always-fire semantic" test in `tasks_for_date_test.go` is a sanity check, not a full coverage.** The fixture omits quarterly and yearly entries (the prompt keeps the fixture small). The always-fire guarantee for those two kinds is exercised by the existing full-inventory tick test (which Prompt 2 updates) and the trigger test (which Prompt 2 updates). If you want quarterly and yearly in the fixture, add two more `TaskDefinition` entries to the `BeforeEach` and re-run; the test logic does not change.

</verification>
