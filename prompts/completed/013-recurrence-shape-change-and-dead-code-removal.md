---
status: completed
spec: [007-recurrence-kind-cleanup]
summary: Reshaped TaskDefinition (drop Fires predicate, add Weekday time.Weekday), deleted predicate/lookup/tasks_for_date files, added weekday suffix to publisher's weekly period token, switched trigger handler to iterate full Inventory() slug-sorted, updated factory and main.go, added new tests for weekday-takes-precedence and non-weekly-Weekday-ignored, and appended CHANGELOG entry
container: recurring-task-creator-sat-sun-weekly-exec-013-recurrence-shape-change-and-dead-code-removal
dark-factory-version: v0.177.1
created: "2026-06-15T21:00:00Z"
queued: "2026-06-15T21:15:49Z"
started: "2026-06-15T21:15:50Z"
completed: "2026-06-15T21:32:08Z"
branch: dark-factory/recurrence-kind-cleanup
---

<summary>
- Removes the dead `Fires predicate` field from `TaskDefinition` and adds a `Weekday time.Weekday` field in its place, with GoDoc stating it is consulted only for `RecurrenceWeekly` entries and ignored for the other four recurrence kinds.
- Changes the publisher's weekly period token from `YYYYWww` to `YYYYWww-<3-letter-lowercase-weekday>` (for example `2026W25-sat` or `2026W25-sun`), where the suffix is read from the entry's new `Weekday` field — not from the date passed in.
- Deletes the seven closed predicate constructors (`OnWeekdays`, `OnDaysOfMonth`, `OnMonthAndDay`, `EveryDay`, `OnFirstDayOfQuarter`, `OnFirstDayOfYear`, `OnFirstDayOfMonth`), the `predicate` type, the `onWeekdayDay5OfMonth` helper, the `ScheduleLookup` type, and the `TasksForDate` function plus its cache — none of them have any remaining production consumer after the handler switch.
- Switches the `GET /trigger?date=YYYY-MM-DD` handler to iterate the full `schedule.Inventory()` (slug-sorted) and removes the `ScheduleLookup` parameter from `NewTriggerHandler` and `CreateTriggerHandler`; the factory and `main.go` wire the new single-argument signature.
- All 21 weekly inventory entries' `Weekday` field is left at the zero value (`time.Sunday`) in this prompt — the per-slug Sat/Sun assignment is deferred to Prompt 2, which is the only place that knows the intended weekday for each entry.
- The build will not compile cleanly until all three layers (struct shape, publisher weekly branch, handler signature) are reshaped in lock-step — that is intentional and matches the spec's decomposition rationale.

</summary>

<objective>

Reshape the schedule / publisher / handler stack in lock-step so that `TaskDefinition` carries a `Weekday` field instead of a `Fires` predicate, the publisher's weekly period token encodes the weekday, and the `/trigger` handler iterates the same set of entries the tick iterates (full inventory, slug-sorted). Delete every piece of code that only existed to feed the removed `Fires` field. End the prompt in a state where the binary compiles, every test passes, and the weekly period token's new shape is covered by tests — but the `Weekday` field on every weekly inventory entry is still the zero value (Prompt 2 fills in `time.Saturday` / `time.Sunday` per entry).

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before making changes:

- `/workspace/pkg/schedule/task_definition.go` — the `TaskDefinition` struct. Replace the `Fires predicate` field (line 25) with a new `Weekday time.Weekday` field. Update GoDoc; the `Slug`, `TitleTemplate`, `BodyTemplate`, `Recurrence` fields are unchanged.
- `/workspace/pkg/schedule/predicate.go` — seven closed predicate constructors (`OnWeekdays`, `OnDaysOfMonth`, `OnMonthAndDay`, `EveryDay`, `OnFirstDayOfQuarter`, `OnFirstDayOfYear`, `OnFirstDayOfMonth`) plus the `predicate` type. The ENTIRE FILE is deleted in this prompt. Verify no production caller outside the package still imports these symbols (they are unexported, so the blast radius is the same package only — covered by the `make precommit` run).
- `/workspace/pkg/schedule/lookup.go` — the `ScheduleLookup` type alias. The ENTIRE FILE is deleted. It is used by `NewTriggerHandler`, `CreateTriggerHandler`, and the `/trigger` handler tests; all of those are updated in this prompt.
- `/workspace/pkg/schedule/tasks_for_date.go` — the `TasksForDate` function and `tasksForDateCache`. The ENTIRE FILE is deleted. Callers (`pkg/handler/trigger.go`, `pkg/handler/trigger_test.go`, `pkg/factory/factory.go`, `pkg/factory/factory_test.go`, `main.go`) are all updated in this prompt.
- `/workspace/pkg/schedule/tasks_for_date_test.go` and `/workspace/pkg/schedule/predicate_test.go` — the corresponding test files. Both are deleted outright (Prompt 2 adds new validation tests in their place).
- `/workspace/pkg/schedule/inventory.go` — the 45-entry inventory slice. In this prompt: (a) delete the `onWeekdayDay5OfMonth` function (lines 9-24) and the `import "time"` if no other code in the file uses `time` after the cleanup; (b) delete every `Fires: <predicate>` field assignment on every entry (45 occurrences). Inventory entries' `Weekday` field is NOT set in this prompt (Prompt 2's job).
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest()` accessor. Unchanged.
- `/workspace/pkg/schedule/inventory_validation_test.go` — existing validation tests (unique slugs, supported placeholders, closed RecurrenceKind set). Unchanged in this prompt; Prompt 2 adds the new `Weekday` invariants alongside.
- `/workspace/pkg/schedule/canonical_slugs_test.go` — the 45-slug canonical list. Unchanged; slugs are frozen.
- `/workspace/pkg/schedule/date.go` — `Date` type and `weekday()` helper. Unchanged.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` enum (5 closed values) and `AllRecurrenceKinds`. Unchanged.
- `/workspace/pkg/schedule/no_forbidden_imports_test.go` — package-level forbidden-imports guard. Unchanged; the schedule package still must not import `net/http`, `kafka-go`, `sarama`, `jira-task-creator`, etc.
- `/workspace/pkg/schedule/schedule_suite_test.go` — Ginkgo test-suite boilerplate. Unchanged.
- `/workspace/pkg/publisher/uuid_namespace.go` — the frozen `uuidNamespace` constant (line 25: `var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")`) is BYTE-IDENTICAL FOREVER. The `buildPeriodToken` weekly branch (lines 52-54) is updated: it now takes a `Weekday time.Weekday` argument and appends `-"<3-letter-lowercase-weekday>"` to the existing `YYYYWww` token. The `default` branch with `errors.Errorf` is preserved unchanged (Spec 6 behavior).
- `/workspace/pkg/publisher/publisher.go` — the `Publisher` interface and the `Publish` method. The interface signature (`Publish(ctx, def, date) error`) is FROZEN. The `Publish` body updates its call to `buildTaskIdentifier` to pass the new `(ctx, slug, recurrence, date, weekday)` tuple, and the new `buildPeriodToken` signature matches.
- `/workspace/pkg/publisher/publisher_test.go` — update the `period-token byte-equality with the formatter output` table test (around lines 210-256). The `weekly` entry's `expectedToken` changes from `"2025W24"` to `"2025W24-<weekday>"`. The `captureIdentifier` closure inside `Describe("period anchoring")` (around line 64) now sets `Weekday: time.<day>` on the `TaskDefinition` literal. None of the other tests in the file reference `Fires` (only the spec's known file touched it via `def.Fires(d)` in `TasksForDate`).
- `/workspace/pkg/publisher/export_test.go` — `UuidNamespaceForTest()` accessor. Unchanged.
- `/workspace/pkg/publisher/render.go` — `fmtIsoWeek` (emits `YYYYWww`), `fmtMonthYear`, `fmtQuarter`, `fmtYear`, `fmtDate`, `quarterOf`. Unchanged. The publisher's weekly branch calls `fmtIsoWeek(isoYear, isoWeek)` to get the prefix, then appends `-"<weekday-abbrev>"` itself.
- `/workspace/pkg/handler/trigger.go` — the `NewTriggerHandler` function. Update signature: drop the `lookup schedule.ScheduleLookup` parameter. The handler body replaces `tasks := lookup(date)` with `tasks := schedule.Inventory()` (Inventory already returns a defensive copy sorted by insertion order — the spec requires slug-sorted order, so the handler sorts the result by `Slug` ascending after calling `Inventory()`).
- `/workspace/pkg/handler/trigger_test.go` — every `schedule.TasksForDate(...)` call site (lines 31, 79, 81, 93, 131, 171) becomes `schedule.Inventory()`. The expected `len(tasks)` values change for the date 2025-01-04 (was a Saturday arm with 12 entries; now full inventory of 45 entries). One new spec is added: `It("publishes every entry in the inventory on /trigger?date=", ...)` — see §6.
- `/workspace/pkg/factory/factory.go` — `CreateTriggerHandler` signature changes: drop the `lookup schedule.ScheduleLookup` parameter. Update the GoDoc to reflect that the handler now reads `schedule.Inventory()` directly.
- `/workspace/pkg/factory/factory_test.go` — every `schedule.TasksForDate(...)` call site (lines 87, 103) becomes `schedule.Inventory()`. The expected `len(...)` value changes to 45.
- `/workspace/main.go` — line 99 changes from `factory.CreateTriggerHandler(pub, schedule.TasksForDate)` to `factory.CreateTriggerHandler(pub)`.
- `/workspace/CHANGELOG.md` — append a `feat:` bullet to `## Unreleased`.

Coding-guideline references (read inside the YOLO container; the agent reads these at `/home/node/.claude/plugins/marketplaces/coding/docs/`):
- `go-architecture-patterns.md` — Interface → Constructor → Struct → Method. The `Publisher` interface stays put; only its `Publish` body and the buildTaskIdentifier helper change. The handler's public API (`NewTriggerHandler(publisher http.Handler) http.Handler`) is preserved as a single-argument constructor.
- `go-error-wrapping-guide.md` — `errors.Errorf(ctx, ...)` for the unknown-recurrence-kind branch (unchanged). `errors.Wrapf(ctx, err, ...)` for the new slug-wrapped call site in `Publish`.
- `go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; `DescribeTable` for the period-token byte-equality table; external test package (`package publisher_test`, `package handler_test`, `package factory_test`, `package schedule_test`).
- `go-factory-pattern.md` — `Create*` prefix, zero business logic. The rewritten `CreateTriggerHandler` still has zero logic; the call becomes a one-argument delegation.
- `definition-of-done.md` — coverage ≥80% for changed packages; every error path tested; boundary contract test preserved (the `cmd.Validate(ctx)` test in publisher_test.go is unaffected).

Verified external symbols (grep'd at `/home/node/go/pkg/mod/` on 2026-06-15; no new deps are needed by this prompt):
- `github.com/google/uuid` (already in `go.mod`): `func NewSHA1(space UUID, data []byte) UUID` — unchanged usage.
- `github.com/bborbe/agent/lib` (already in `go.mod`): `type TaskIdentifier string` — unchanged usage.
- `github.com/bborbe/errors` (already in `go.mod`): `Wrapf(ctx, err, format, args...)`, `Errorf(ctx, format, args...)` — unchanged usage.
- `github.com/golang/glog` (already in `go.mod`): `glog.V(2).Infof(...)` for the trigger's per-request log line — unchanged.

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/task_definition.go (CURRENT, lines 7-26)
type TaskDefinition struct {
    Slug          string
    TitleTemplate string
    BodyTemplate  string
    Recurrence    RecurrenceKind
    Fires         predicate
}

// pkg/publisher/uuid_namespace.go (CURRENT weekly branch, lines 52-54)
case schedule.RecurrenceWeekly:
    isoYear, isoWeek := base.ISOWeek()
    return fmtIsoWeek(isoYear, isoWeek), nil

// pkg/schedule/recurrence.go (FROZEN, lines 8-16)
type RecurrenceKind string
const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)

// pkg/publisher/uuid_namespace.go line 25 (FROZEN, do not edit)
var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")
```

</context>

<requirements>

## 1. Reshape `TaskDefinition`: drop `Fires`, add `Weekday`

In `/workspace/pkg/schedule/task_definition.go`:

- Delete the field `Fires predicate` (line 25 of the current file). The GoDoc line above it (line 24, `// Fires reports whether this definition fires on the given civil date.`) is also removed.
- Add a new field `Weekday time.Weekday` in its place, with GoDoc that says exactly:

  ```go
  // Weekday is the day of the week the entry is intended for. It is
  // consulted ONLY when Recurrence == RecurrenceWeekly: the publisher
  // appends the lowercase 3-letter weekday abbreviation to the weekly
  // period token (e.g. "2026W25-sat"). For the other four RecurrenceKinds
  // (daily / monthly / quarterly / yearly) Weekday is ignored and MUST
  // remain the zero value (time.Sunday).
  Weekday time.Weekday
  ```

- Add `import "time"` to the file. The struct field type `time.Weekday` requires it; verify the import block uses the project's `goimports-reviser` style (stdlib first, then third-party alphabetical, then internal).
- The `SupportedPlaceholders` var at the bottom of the file is unchanged.

The new struct shape, in order, is:

```go
type TaskDefinition struct {
    Slug          string
    TitleTemplate string
    BodyTemplate  string
    Recurrence    RecurrenceKind
    Weekday       time.Weekday
}
```

## 2. Delete the dead predicate / lookup / tasks_for_date files

Delete these files in their entirety:

- `/workspace/pkg/schedule/predicate.go` (7 closed predicate constructors + the `predicate` type alias)
- `/workspace/pkg/schedule/lookup.go` (`ScheduleLookup` type alias)
- `/workspace/pkg/schedule/tasks_for_date.go` (`TasksForDate` function + `tasksForDateCache`)
- `/workspace/pkg/schedule/predicate_test.go` (tests for the deleted predicates)
- `/workspace/pkg/schedule/tasks_for_date_test.go` (tests for the deleted `TasksForDate`)

After deletion, the `schedule` package retains only: `task_definition.go`, `date.go`, `recurrence.go`, `inventory.go`, `inventory_export_test.go`, `inventory_validation_test.go`, `canonical_slugs_test.go`, `no_forbidden_imports_test.go`, `schedule_suite_test.go`.

## 3. Strip `inventory.go` of the dead `Fires` and `onWeekdayDay5OfMonth`

In `/workspace/pkg/schedule/inventory.go`:

- Delete the `onWeekdayDay5OfMonth` function (lines 9-24 of the current file) outright.
- Delete every `Fires: <predicate-call>` field assignment on every one of the 45 inventory entries. There are exactly 45 such assignments today: 12 `OnWeekdays(time.Saturday)`, 9 `OnWeekdays(time.Sunday)`, 1 `onWeekdayDay5OfMonth`, 2 `OnMonthAndDay(time.May, 1)`, 17 `OnFirstDayOfMonth()`, 2 `OnFirstDayOfQuarter()`, 2 `OnFirstDayOfYear()`. After deletion, every entry has the same 4 fields: `Slug`, `TitleTemplate`, `BodyTemplate`, `Recurrence`.
- Do NOT add a `Weekday: ...` field to any entry in this prompt. Every weekly entry's `Weekday` is left at the zero value (`time.Sunday`); Prompt 2 sets it explicitly per slug.
- After the `Fires` assignments are gone, the `import "time"` at the top of the file is no longer needed by the inventory slice itself — but `Inventory()` (line 454) and the `Weekday` field type (when Prompt 2 lands) are still in the same package, and `time` is now imported by `task_definition.go`. Leave the import statement in `inventory.go` for now (its presence is harmless; Prompt 2 may remove it if unused after both prompts are merged).
- The GoDoc on the `inventory` var (lines 26-27) and the `Inventory()` function (lines 444-458) is updated: drop the line "TasksForDate sorts by Slug before returning" from the `inventory` var GoDoc, and update the `Inventory()` GoDoc to remove the parenthetical "(pkg/handler/trigger) still uses schedule.TasksForDate for per-day ?date= replay" — the handler now uses `Inventory()` too. The rest of the GoDoc is preserved.
- The 45-entry slice order is preserved exactly. Slugs are frozen.

## 4. Update the publisher's weekly period token

In `/workspace/pkg/publisher/uuid_namespace.go`:

- Update the `buildPeriodToken` signature. It now takes a `weekday time.Weekday` argument and returns the suffix-aware token. The new signature is:

  ```go
  func buildPeriodToken(
      ctx context.Context,
      recurrence schedule.RecurrenceKind,
      date schedule.Date,
      weekday time.Weekday,
  ) (string, error) {
      base := date.Time()
      switch recurrence {
      case schedule.RecurrenceDaily:
          return fmtDate(date.Year, int(date.Month), date.Day), nil
      case schedule.RecurrenceWeekly:
          isoYear, isoWeek := base.ISOWeek()
          return fmtIsoWeek(isoYear, isoWeek) + "-" + weekdayAbbrev(weekday), nil
      case schedule.RecurrenceMonthly:
          return fmtMonthYear(base.Year(), int(base.Month())), nil
      case schedule.RecurrenceQuarterly:
          return fmtQuarter(base.Year(), quarterOf(base.Month())), nil
      case schedule.RecurrenceYearly:
          return fmtYear(base.Year()), nil
      default:
          return "", errors.Errorf(
              ctx,
              "buildPeriodToken: unknown recurrence kind %q",
              recurrence,
          )
      }
  }
  ```

- Update the `buildTaskIdentifier` signature to match: it now takes `(ctx, slug, recurrence, date, weekday)` and threads `weekday` through to `buildPeriodToken`. The new body is:

  ```go
  func buildTaskIdentifier(
      ctx context.Context,
      slug string,
      recurrence schedule.RecurrenceKind,
      date schedule.Date,
      weekday time.Weekday,
  ) (lib.TaskIdentifier, error) {
      token, err := buildPeriodToken(ctx, recurrence, date, weekday)
      if err != nil {
          return "", errors.Wrapf(ctx, err, "buildTaskIdentifier: slug %q", slug)
      }
      name := "recurring-" + slug + "-" + token
      return lib.TaskIdentifier(uuid.NewSHA1(uuidNamespace, []byte(name)).String()), nil
  }
  ```

- Add a new unexported helper `weekdayAbbrev` in the same file (below `buildTaskIdentifier`). The helper maps `time.Weekday` to its 3-letter lowercase abbreviation, as defined in this exact table:

  ```go
  // weekdayAbbrev returns the lowercase 3-letter abbreviation of w
  // (e.g. "mon" for Monday, "sun" for Sunday). Used by buildPeriodToken
  // to encode the weekday suffix in the weekly period token. All seven
  // values are spelled per the conventional time package abbreviations.
  func weekdayAbbrev(w time.Weekday) string {
      switch w {
      case time.Monday:
          return "mon"
      case time.Tuesday:
          return "tue"
      case time.Wednesday:
          return "wed"
      case time.Thursday:
          return "thu"
      case time.Friday:
          return "fri"
      case time.Saturday:
          return "sat"
      case time.Sunday:
          return "sun"
      }
      return ""
  }
  ```

  Place this helper after the existing `buildTaskIdentifier` block. The `time` import is required by `weekdayAbbrev`'s `time.Weekday` parameter type; ensure it is present in the import block.

- Update the GoDoc on `buildPeriodToken` to reflect the new shape: it now documents the suffix format `YYYYWNN-<3-letter-lowercase-weekday>` for weekly and notes that the suffix is taken from the entry's `Weekday` field, not the date.

- The frozen `uuidNamespace` constant at line 25 is UNTOUCHED. The line must read exactly `var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")` after the change.

- The `default` branch with `errors.Errorf` for unknown `RecurrenceKind` is preserved verbatim. It is the only path that returns an error.

## 5. Update the publisher's `Publish` call site

In `/workspace/pkg/publisher/publisher.go`:

- Update the call to `buildTaskIdentifier` at line 59 of the current file. The new call signature is `(ctx, slug, recurrence, date, weekday)`. The replacement is:

  ```go
  token, err := buildTaskIdentifier(ctx, def.Slug, def.Recurrence, date, def.Weekday)
  if err != nil {
      return errors.Wrapf(
          ctx,
          err,
          "publish failed: build identifier for slug %q",
          def.Slug,
      )
  }
  ```

- The rest of `Publish` is unchanged. The interface signature stays `(ctx, def, date) error`. The `def.Weekday` field is read here for the first time.

- The `Publisher` interface declaration (line 22 directive, lines 24-31 interface body) and the `NewPublisher` constructor are unchanged.

## 6. Update publisher tests for the new weekly token

In `/workspace/pkg/publisher/publisher_test.go`:

- Update the `period-token byte-equality with the formatter output` `DescribeTable` (lines 210-256). The `weekly` entry's `expectedToken` changes from `"2025W24"` to `"2025W24-sat"` (the test's `TaskDefinition` literal must set `Weekday: time.Saturday`). All other entries (daily, monthly, quarterly, yearly) are unchanged. The new table entry is:

  ```go
  Entry(
      "weekly",
      schedule.RecurrenceWeekly,
      schedule.NewDate(2025, time.June, 9),
      "2025W24-sat",
  ),
  ```

  The `TaskDefinition` literal in the table's `func` parameter is updated to include `Weekday: time.Saturday` for the weekly row only. One way to do this: split the weekly row into a separate `It` block (the simplest patch is to keep the table for the four non-weekly kinds and add a one-off `It("weekly: byte-equality with the formatter output (with weekday suffix)", ...)` next to the table — either approach is acceptable, but the new `It` form is preferred because it lets the `def` literal carry `Weekday` cleanly).

- The `Describe("identifier")` block (the single `It("is the UUID5 of the canonical key")` test, lines 39-58) needs its `def` literal updated to set `Weekday: time.Saturday` (the slug `weekly-review` is a Saturday entry) and its expected input string updated from `"recurring-weekly-review-2025W01"` to `"recurring-weekly-review-2026W01-sat"`. **WAIT** — recompute: the test calls `Publish` on `schedule.NewDate(2025, time.January, 4)`. `2025-01-04` is a Saturday in ISO week 2025W01. So the new expected token is `"2025W01-sat"`, and the new expected input string is `"recurring-weekly-review-2025W01-sat"`. The test's `def` literal adds `Weekday: time.Saturday`. This is the canonical evidence for AC #6.

- The `Describe("period anchoring")` block (lines 60-256) has a `captureIdentifier` closure (lines 61-80) that constructs a `TaskDefinition` literal. The closure's `def` literal needs `Weekday: time.Saturday` for weekly rows (or pass it as a parameter — the simplest patch is hard-coding `Weekday: time.Saturday` inside the closure since every weekly test in this block uses the same Saturday case). The equality-within-period tests (lines 82-95: "weekly: same ISO week, different civil dates produce the same identifier") and the inequality-across-boundaries test (lines 139-152: "weekly: adjacent ISO weeks produce different identifiers") need this fix; the monthly / quarterly / yearly / daily tests are unchanged.

- Add a new `It` block right after the `Describe("period anchoring")` block, as evidence for AC #6. The test exercises `buildPeriodToken` directly through `Publish` for a Wednesday in 2025W25 with `Weekday: time.Saturday`, asserting the period token is `"2026W25-sat"`:

  ```go
  It("buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday", func() {
      // 2026-06-17 is a Wednesday, in ISO 2026W25. With Weekday=time.Saturday
      // on the def, the period token must be "2026W25-sat" (NOT "2026W25-wed").
      def := schedule.TaskDefinition{
          Slug:          "weekday-takes-precedence",
          TitleTemplate: "t",
          Recurrence:    schedule.RecurrenceWeekly,
          Weekday:       time.Saturday,
      }
      Expect(pub.Publish(
          context.Background(),
          def,
          schedule.NewDate(2026, time.June, 17),
      )).To(Succeed())
      captured := capture()
      expected := uuid.NewSHA1(
          publisher.UuidNamespaceForTest(),
          []byte("recurring-weekday-takes-precedence-2026W25-sat"),
      ).String()
      Expect(string(captured.TaskIdentifier)).To(Equal(expected))
  })
  ```

- Add a new `It` block (AC #7 evidence): confirm the non-weekly period tokens are unchanged. This is a quick smoke test that asserts each of the four non-weekly `RecurrenceKind`s produces the unchanged token, regardless of `Weekday`:

  ```go
  It("non-weekly kinds ignore the Weekday field (token is identical to Spec 6)", func() {
      for _, c := range []struct {
          rec schedule.RecurrenceKind
          d   schedule.Date
          tok string
      }{
          {schedule.RecurrenceDaily, schedule.NewDate(2025, time.June, 14), "2025-06-14"},
          {schedule.RecurrenceMonthly, schedule.NewDate(2025, time.June, 1), "2025-06"},
          {schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.April, 1), "2025Q2"},
          {schedule.RecurrenceYearly, schedule.NewDate(2025, time.January, 1), "2025"},
      } {
          // Weekday deliberately non-zero to prove it is ignored for non-weekly kinds.
          def := schedule.TaskDefinition{
              Slug:          "non-weekly-" + string(c.rec),
              TitleTemplate: "t",
              Recurrence:    c.rec,
              Weekday:       time.Wednesday,
          }
          // Use a fresh sender per iteration so SendCommandArgsForCall(0)
          // always points at the most recent Publish.
          localSender := &taskmocks.TaskCreateCommandSender{}
          localSender.SendCommandReturns(nil)
          localPub := publisher.NewPublisher(localSender, false)
          Expect(localPub.Publish(context.Background(), def, c.d)).To(Succeed())
          _, cmd := localSender.SendCommandArgsForCall(0)
          want := uuid.NewSHA1(
              publisher.UuidNamespaceForTest(),
              []byte("recurring-non-weekly-"+string(c.rec)+"-"+c.tok),
          ).String()
          Expect(string(cmd.TaskIdentifier)).To(Equal(want))
      }
  })
  ```

- No other test in `publisher_test.go` constructs a `TaskDefinition` with a `Fires` field. The `Describe("placeholder rendering")`, `Describe("frontmatter")`, `Describe("sender interaction")`, and `Describe("boundary contract")` blocks all create `TaskDefinition` literals without `Fires` (they only set `Slug`, `TitleTemplate`, `BodyTemplate`, `Recurrence`). The `Describe("errors")` block's `It("returns a wrapped error for an unknown recurrence kind")` is also unaffected. The `Describe("determinism")` block's `TaskDefinition` literal (line 275-280) is unaffected.

- Add `"time"` to the file's import block if not already present (it is — the file already imports `"time"` for the `time.January` etc. constants).

## 7. Update the trigger handler signature and body

In `/workspace/pkg/handler/trigger.go`:

- Change the `NewTriggerHandler` signature. Drop the `lookup schedule.ScheduleLookup` parameter. The new signature is:

  ```go
  func NewTriggerHandler(publisher publisher.Publisher) http.Handler
  ```

- Update the GoDoc above the function. The new GoDoc reads (replace lines 34-54 of the current file):

  ```go
  // NewTriggerHandler returns an HTTP handler that replays the recurring-task
  // publishes for one civil date. The date is supplied as the `date` query
  // parameter in YYYY-MM-DD format. For each entry in the full inventory
  // (schedule.Inventory(), slug-sorted), the handler calls
  // publisher.Publish(req.Context(), def, date). Per-task errors are
  // accumulated in the response's `errors` array — the iteration does
  // NOT short-circuit on error. The response is always HTTP 200 on a
  // successfully parsed date, regardless of whether any individual
  // publish failed.
  //
  // The handler iterates the same set of entries the hourly tick iterates
  // (the full inventory); per-day filtering is gone (Spec 7). Malformed or
  // missing `date` parameter returns HTTP 400 with a JSON body of the form
  // {"error":"<message>"}. The handler holds no per-request state and is
  // safe to call concurrently for the same date (the controller dedups
  // by deterministic UUID5).
  //
  // Security: this handler intentionally has no authentication. The
  // service is deployed cluster-internal-only (no k8s Ingress); all
  // external access is brokered by ~/Documents/workspaces/trading/frontend/
  // gateway, which owns auth. The /trigger surface is reachable only
  // inside the cluster. Idempotency via deterministic UUID5 also makes
  // accidental replay safe.
  ```

- Update the handler body. Replace the line `tasks := lookup(date)` with the two-line slug-sorted full-inventory read:

  ```go
  defs := schedule.Inventory()
  sort.Slice(defs, func(i, j int) bool { return defs[i].Slug < defs[j].Slug })
  tasks := defs
  ```

  The `sort` package is from stdlib; add `"sort"` to the file's import block. The rest of the handler body (date parsing, error response, per-entry Publish loop, JSON encode) is unchanged.

- The function still uses `req.Context()` for the publish context (DoD: no `context.Background()` in business logic). The `time.Parse` call is input parsing, not a clock read — its presence is preserved.

- The `writeTriggerError` helper is unchanged.

## 8. Update trigger handler tests

In `/workspace/pkg/handler/trigger_test.go`:

- Update the `BeforeEach` at lines 29-32. The new call is:

  ```go
  httpHandler = handler.NewTriggerHandler(fakePublisher)
  ```

- Update the five `schedule.TasksForDate(...)` call sites (lines 31 [removed], 79, 81, 93, 131, 171) to `schedule.Inventory()`. After the change, the helper variable `tasks` holds the full inventory (45 entries), and the assertions `len(tasks)` evaluate to 45 for the date 2025-01-04 (and any other date — the iteration set is the full inventory regardless of the parsed date, by design).

- The `It("calls publisher.Publish once for every entry returned by schedule.TasksForDate", ...)` test (lines 79-89) is renamed to `It("publishes every entry in the inventory on /trigger?date=", ...)` (this is AC #11). The body asserts `len(tasks) == 45` and `fakePublisher.PublishCallCount() == 45`. The renamed test reads:

  ```go
  It("publishes every entry in the inventory on /trigger?date=", func() {
      tasks := schedule.Inventory()
      Expect(tasks).To(HaveLen(45))

      req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
      resp := httptest.NewRecorder()
      httpHandler.ServeHTTP(resp, req)

      Expect(resp.Code).To(Equal(http.StatusOK))
      Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
  })
  ```

- The `It("responds 200 with date, published=N, errors=[] when all publishes succeed", ...)` test (lines 91-115) is updated: `tasks := schedule.Inventory()`; `Expect(body.Published).To(Equal(len(tasks)))` → `Equal(45)`.

- The `It("returns 200 with errors[] populated and published=len(tasks)-1 when one publish fails", ...)` test (lines 127-165) is updated: `tasks := schedule.Inventory()`; `Expect(body.Published).To(Equal(len(tasks) - 1))` → `Equal(44)`.

- The `It("returns 200 (not 5xx) with published=0 and full errors array when every publish fails", ...)` test (lines 167-191) is updated: `tasks := schedule.Inventory()`; `Expect(body.Errors).To(HaveLen(len(tasks)))` → `HaveLen(45)`.

- The `It("returns 400 with 'missing date parameter' when no date query is set", ...)`, `It("returns 400 with 'missing date parameter' when date query is empty", ...)`, `It("returns 400 with 'invalid date format' for non-date input", ...)`, and `It("returns 400 with 'invalid date format' for day-of-month=32 (parse-fail)", ...)` tests are unchanged — the missing/invalid `date` path is unaffected by the signature change.

- The `It("serializes errors as [] (not null) when no errors occurred", ...)` test (lines 117-125) is unchanged.

- The `It("propagates the request context to publisher.Publish", ...)` test (lines 193-205) is unchanged.

- The `mocks` import (line 18) is unchanged. The `time` import (line 12) is unchanged.

## 9. Update the factory signature and wiring

In `/workspace/pkg/factory/factory.go`:

- Update the `CreateTriggerHandler` signature. Drop the `lookup schedule.ScheduleLookup` parameter. The new signature is:

  ```go
  func CreateTriggerHandler(publisher publisher.Publisher) http.Handler
  ```

- Update the GoDoc above the function. The new GoDoc reads (replace lines 61-66 of the current file):

  ```go
  // CreateTriggerHandler returns the operator-replay HTTP handler. The
  // handler iterates schedule.Inventory() (full inventory, slug-sorted)
  // and calls publisher.Publish for each entry against the parsed date.
  // Pure plumbing: no business logic, no closure capture, no state.
  ```

- Update the function body: the call becomes `return handler.NewTriggerHandler(publisher)`.

- The other factory functions (`CreatePublisher`, `CreateTick`, `CreateHealthzHandler`, `CreateTickLoop`, `CreateCommandSender`) are unchanged.

- Remove the now-unused `schedule` import if the file no longer references any other `schedule.X` symbol. Verify: the file's other call sites to `schedule` are `schedule.Inventory()` in `CreateTick` (line 49), so the `schedule` import stays.

In `/workspace/pkg/factory/factory_test.go`:

- Update the `BeforeEach` at lines 84-88. The new call is:

  ```go
  httpHndl = factory.CreateTriggerHandler(pubFake)
  ```

- Update the two `schedule.TasksForDate(...)` call sites (lines 87 [removed], 103) to `schedule.Inventory()`. The assertion `Expect(pubFake.PublishCallCount()).To(Equal(len(schedule.Inventory())))` evaluates to `Equal(45)`.

- The `Describe("CreatePublisher")`, `Describe("CreateTick")`, and `Describe("CreateHealthzHandler")` blocks are unchanged.

## 10. Update `main.go`

In `/workspace/main.go`:

- Update line 99. The new call is:

  ```go
  a.TriggerHandler = factory.CreateTriggerHandler(pub)
  ```

- The `schedule` import is still required (the file uses `schedule.NewDate` elsewhere — verify by re-reading the file). If the file's only use of `schedule` was the deleted `schedule.TasksForDate`, remove the import. In the current state of the file, `schedule` is imported but never used outside line 99. Keep the import; it may be needed by future specs. (Do not silently remove it — leaving it in is harmless and the `goimports-reviser` linter will not flag an unused import at the file level for stdlib-aliases; the import is referenced as `schedule.X` somewhere — if not, the linter will flag it and you can remove it then.)

- The `application` struct and the `Run` method body are otherwise unchanged.

## 11. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (one bullet, `feat:` prefix per `changelog-guide.md`):

```markdown
- feat: Drop the dead `Fires` predicate from `schedule.TaskDefinition` and add a `Weekday time.Weekday` field; the publisher's weekly period token now appends the lowercase 3-letter weekday abbreviation (e.g. `2026W25-sat`); the `GET /trigger?date=` handler iterates the full `schedule.Inventory()` instead of `schedule.TasksForDate`; delete the seven closed predicate constructors, the `ScheduleLookup` and `TasksForDate` plumbing, and the `onWeekdayDay5OfMonth` helper
```

## 12. Imports and conventions

- Every modified `.go` file retains the 2026 copyright header.
- Use `goimports-reviser` style: standard library first, then third-party (alphabetical: `github.com/bborbe/...`, `github.com/google/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use `github.com/bborbe/errors` for error wrapping. Never `fmt.Errorf`.
- Do NOT touch `pkg/tick/`, the Makefile, k8s manifests, or the Prometheus metric surface.
- Do NOT add a new Prometheus metric, opt-out flag, runtime config knob, or per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT regenerate the counterfeiter mock for `Publisher` — the existing `/workspace/mocks/publisher-publisher.go` is for the same interface (signature is unchanged).
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- The `uuidNamespace` constant in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN byte-identical. The literal `f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a` is the contract; the line `var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")` is unchanged.
- The `Publisher` interface signature (`Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error`) is FROZEN. The counterfeiter directive on the interface stays as-is.
- The `RecurrenceKind` enum stays a closed set of exactly 5 values: `RecurrenceDaily`, `RecurrenceWeekly`, `RecurrenceMonthly`, `RecurrenceQuarterly`, `RecurrenceYearly`. The `AllRecurrenceKinds` slice stays in declaration order.
- Slugs are FROZEN. The 45 entries' slugs do not change in this prompt. The change to weekly UUID5 identifiers is the accepted one-time deploy cost (matches Spec 6's accepted cost). No backward-compat layer for the old token format.
- The publisher's period-token derivation MUST reuse the existing `fmtIsoWeek`, `fmtMonthYear`, `fmtQuarter`, `fmtYear`, `fmtDate`, and `quarterOf` helpers in `pkg/publisher/render.go`. The new `weekdayAbbrev` helper is a separate small helper in `uuid_namespace.go`; the 7-case `time.Weekday → string` mapping is NOT a duplicate of any existing helper.
- The 3-letter weekday abbreviation is lowercase (`mon` / `tue` / `wed` / `thu` / `fri` / `sat` / `sun`). The uppercase `W` in `YYYYWww` is preserved.
- Non-weekly period tokens (`YYYY-MM-DD`, `YYYY-MM`, `YYYYQq`, `YYYY`) are unchanged from Spec 6. The `Weekday` field is consulted only for `RecurrenceWeekly`.
- The `/trigger?date=` response JSON shape (`date`, `published`, `errors`) is unchanged from Spec 5. Only the iteration set changes from "entries firing on the parsed date" to "every entry in the inventory".
- `time` is imported by `task_definition.go` (for the `Weekday` field type) and by `uuid_namespace.go` (for the `weekdayAbbrev` helper and the new `buildPeriodToken` parameter type). The `inventory.go` file's existing `import "time"` is left in place even after the `Fires` assignments are removed; it will be removed by Prompt 2 (or by the executor if Prompt 2 confirms it is unused after both prompts land).
- The deleted predicate primitives (`OnWeekdays`, `OnDaysOfMonth`, `OnMonthAndDay`, `EveryDay`, `OnFirstDayOfQuarter`, `OnFirstDayOfYear`, `OnFirstDayOfMonth`), the `predicate` type, the `onWeekdayDay5OfMonth` helper, the `ScheduleLookup` type, and the `TasksForDate` function are NOT preserved behind an alias or compatibility shim. They are deleted outright.
- This prompt does NOT set the `Weekday` field on any inventory entry. Every weekly entry's `Weekday` is the zero value (`time.Sunday`) after this prompt. Prompt 2 sets it explicitly per slug. The build still passes because the publisher's weekly branch appends the suffix unconditionally, and the validation tests added in Prompt 2 will catch any entry that has a non-Saturday `Weekday` for a slug that should be Saturday (or vice versa).
- Out of scope (documented in the spec, not addressed in code): orphan vault files keyed by the previous `2026Www` identifier that remain after deploy. They are NOT deleted by this service (no vault writes from this service per DoD). The user removes them manually.
- Project DoD (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg `Wrap`; no `context.Background()` in business logic; no `time.Time` / `time.Now()` in business logic; GoDoc on exports; `make precommit` clean.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass after all edits. (The `predicate_test.go` and `tasks_for_date_test.go` files are deleted as part of the dead-code cleanup; that is the only test-file deletion in this prompt. All other test files are updated in place and continue to pass.)

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/schedule/... ./pkg/publisher/... ./pkg/handler/... ./pkg/factory/...` — all Ginkgo specs green. In particular:
   - `It("publishes every entry in the inventory on /trigger?date=", ...)` in `pkg/handler/trigger_test.go` passes.
   - `It("buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday", ...)` in `pkg/publisher/publisher_test.go` passes.
   - `It("non-weekly kinds ignore the Weekday field (token is identical to Spec 6)", ...)` in `pkg/publisher/publisher_test.go` passes.
   - The renamed `It("is the UUID5 of the canonical key")` test in `pkg/publisher/publisher_test.go` passes with the new expected input string `"recurring-weekly-review-2025W01-sat"`.
3. `grep -RnE "\bFires\b" pkg/ main.go` — must return no matches (AC #1). The only acceptable match is in GoDoc / comments referencing historical decisions — there are none left after this prompt because `task_definition.go`'s Fires GoDoc is removed and the `uuid_namespace.go` comment that references "def.Fires" is updated to "def.Weekday" in §4.
4. `grep -RnE "OnWeekdays|OnDaysOfMonth|OnMonthAndDay|EveryDay|OnFirstDayOfQuarter|OnFirstDayOfYear|OnFirstDayOfMonth|onWeekdayDay5OfMonth" pkg/ main.go` — must return no matches (AC #2).
5. `grep -RnE "TasksForDate|ScheduleLookup|tasksForDateCache" pkg/ main.go` — must return no matches (additional evidence; the spec does not require this grep but it is the binary check that the lookup plumbing is fully removed).
6. `grep -n "Weekday time.Weekday" pkg/schedule/task_definition.go` — must return exactly one line, the field declaration (AC #3). The GoDoc above it must mention "consulted ONLY when Recurrence == RecurrenceWeekly".
7. `grep -nE '"f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a"' pkg/publisher/uuid_namespace.go` — must return exactly one match (line 25 of the file). The frozen namespace constant is byte-identical.
8. `grep -n 'buildPeriodToken' pkg/publisher/*.go` — must return at least 2 matches (the definition and the call site in `buildTaskIdentifier`).
9. `grep -n 'weekdayAbbrev' pkg/publisher/uuid_namespace.go` — must return at least 2 matches (the definition and the call site in `buildPeriodToken`).
10. `grep -nE 'Weekday:' pkg/schedule/inventory.go` — must return no matches in this prompt. Every weekly entry's `Weekday` is at the zero value. Prompt 2 adds the 21 `Weekday: time.Saturday` / `Weekday: time.Sunday` lines.
11. Spot-check: open `pkg/schedule/task_definition.go` and visually confirm the field order is `Slug, TitleTemplate, BodyTemplate, Recurrence, Weekday` and the GoDoc above `Weekday` is the exact text from §1.
12. Spot-check: open `pkg/publisher/uuid_namespace.go` and visually confirm the namespace constant line is byte-identical, `buildPeriodToken` has 5 cases plus the `default` branch, the weekly case appends `"-" + weekdayAbbrev(weekday)`, and `weekdayAbbrev` covers all 7 `time.Weekday` values.
13. Spot-check: open `pkg/handler/trigger.go` and visually confirm the `NewTriggerHandler` signature has one parameter, the body uses `schedule.Inventory()` + a `sort.Slice` call, and the `time.Parse` + `req.Context()` paths are preserved.
14. Spot-check: open `main.go` line 99 and visually confirm the call is `factory.CreateTriggerHandler(pub)` with no second argument.
15. Coverage check on changed packages:
    - `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/schedule/... ./pkg/publisher/... ./pkg/handler/... ./pkg/factory/...`
    - `go tool cover -func=/tmp/cover.out | tail -1` — total coverage ≥ 80%.

## Open Questions (for the human reviewer)

- **A. The trigger handler sorts the inventory after `Inventory()`.** The current `Inventory()` function returns entries in the order they appear in the package-level `inventory` slice (which is grouped by category, not by slug). The new handler calls `sort.Slice(defs, ...)` on the returned slice to enforce slug-sorted iteration. This matches the spec's "full inventory, slug-sorted" wording. An alternative would be to push the sort into `Inventory()` itself, but that would change the tick path too (currently the tick does not depend on iteration order — see `CreateTick` in `pkg/factory/factory.go` which just hands the slice to `tick.NewTick`). The current patch localizes the change to the handler; review this if you prefer the sort at the accessor.
- **B. The non-weekly-`Weekday`-must-be-zero validation is added in Prompt 2, not here.** This prompt leaves every non-weekly entry's `Weekday` at the zero value (which is correct), and the new validation test in `pkg/schedule/inventory_validation_test.go` (added by Prompt 2) asserts it. There is no validation in this prompt; the new fields are written, the tests for the new fields are added alongside, in Prompt 2. The two prompts are intentionally sequenced so the build passes at the end of each one.
- **C. The `Weekday: time.Saturday` zero-value reset for non-weekly entries.** Every non-weekly entry's `Weekday` is the zero value (`time.Sunday`) in this prompt. That value is also the value of a Sunday weekly entry. Prompt 2 disambiguates by setting the 12 Saturday entries' `Weekday` to `time.Saturday` explicitly, so the 9 Sunday weekly entries remain the only weekly entries at `time.Sunday`. The disambiguation is the Sunday-slug allow-list (also added in Prompt 2). The build is green at the end of this prompt; the disambiguation is added on top of it in Prompt 2.
- **D. `sort` import in `pkg/handler/trigger.go`.** The handler's import block gains `"sort"` (stdlib). The `goimports-reviser` linter sorts stdlib imports first. Verify the order after the change.
- **E. The `Publisher` interface directive.** The counterfeiter directive `//counterfeiter:generate -o ../../mocks/publisher-publisher.go --fake-name PublisherPublisher . Publisher` stays. The interface signature is unchanged. The `Publish` body changes; the mocks do not need regeneration. The `<verification>` step 3 + the `mocks/publisher-publisher.go` file's contents (read it before writing) confirm this.
- **F. The orphan-vault-file cost.** Per the spec, orphan files keyed by the previous `2026Www` identifier are NOT deleted by this service. The user removes them manually. This is a one-time deploy cost acknowledged by the spec and is not addressed by any prompt in this spec.
- **G. No scenario file.** The spec's Acceptance Criteria all reduce to grep evidence and Ginkgo test names — every behavior is reachable from a YOLO container with the counterfeiter mock for `Publisher` and the in-process `httptest.NewRecorder` for the trigger handler. No real Kafka, no real vault, no real clock. No `scenarios/` work is part of this spec or this prompt.

</verification>
