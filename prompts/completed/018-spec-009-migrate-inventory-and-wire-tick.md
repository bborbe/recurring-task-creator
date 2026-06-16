---
status: completed
spec: [009-weekday-kind-split]
summary: Migrated 21 weekly-with-Weekday inventory entries to RecurrenceWeekday; split buildPeriodToken switch (RecurrenceWeekly bare, RecurrenceWeekday with weekday suffix); wired tick and trigger to filter via schedule.TasksForDate(date); updated all tests, deleted sundayWeeklyAllowList, added UUID5 stability test, added changelog entry; make precommit passes
container: recurring-task-creator-weekday-kind-exec-018-spec-009-migrate-inventory-and-wire-tick
dark-factory-version: v0.177.1
created: "2026-06-16T12:30:00Z"
queued: "2026-06-16T12:45:58Z"
started: "2026-06-16T12:56:27Z"
completed: "2026-06-16T13:19:05Z"
branch: dark-factory/weekday-kind-split
---

<summary>

- All 21 inventory entries with `Recurrence: RecurrenceWeekly, Weekday: time.Saturday|time.Sunday` in `/workspace/pkg/schedule/inventory.go` are migrated to `Recurrence: RecurrenceWeekday` with `Weekday: time.Saturday|time.Sunday` unchanged. The 12 Saturday entries (slugs `shutdown-k3s`, `turn-on-hell`, `weekly-review`, `check-ftmo-demo-accounts`, `lexoffice-invoices`, `moneymoney-review`, `opnsense-update`, `home-assistant-update-backup`, `renew-gmail-oauth-tokens`, `plan-next-week`, `run-update-all-saturday`, `topic-backup-saturday`) and the 9 Sunday entries (slugs `complete-rsync-backups`, `complete-longhorn-backups`, `turn-off-hell`, `turn-off-sun`, `turn-off-fire`, `docker-registry-gc`, `rebuild-trading-dev-prod`, `check-bot-is-healthy`, `run-update-all`) are migrated. The 24 non-weekly entries (17 monthly + 2 quarterly + 2 yearly + 1 monthly day-5 + 2 yearly May-1st) are NOT touched.
- The publisher's `buildPeriodToken` switch in `/workspace/pkg/publisher/uuid_namespace.go` is updated: the existing `case schedule.RecurrenceWeekly` arm now emits bare `YYYYWww` (no weekday suffix); a new `case schedule.RecurrenceWeekday` arm emits `YYYYWww-<3-letter-lowercase-weekday-abbrev>`. The 21 migrated entries' UUID5 input strings are byte-identical to pre-Spec-9 — same identifier, same vault filename, no duplicates after deploy. A table-driven UUID5 stability test enumerates all 21 weekday slugs with the hand-derived pre-Spec-9 expected token strings and asserts byte-for-byte equality.
- The tick (`pkg/tick/tick.go`) is updated to filter by date: it now calls `schedule.TasksForDate(date)` (the new accessor added in Prompt 1) instead of iterating `t.inventory` directly. The factory still passes `schedule.Inventory()` (full inventory) to `tick.NewTick`; the tick derives the per-tick filtered slice internally. The "full inventory" tick test in `pkg/tick/tick_test.go` (line 423) is updated to assert the date-filtered count (using `schedule.TasksForDate(refDate)` as the expected count, not the full 45).
- The trigger handler (`pkg/handler/trigger.go`) is updated to iterate `schedule.TasksForDate(date)` instead of `schedule.Inventory()`. The "publishes every entry in the inventory on /trigger?date=" test (line 78) and the "responds 200 with date, published=N, errors=[]" test (line 90) are updated to use `schedule.TasksForDate(date)` for the expected count. For `?date=2025-01-04` (Saturday), the expected count is 12 Saturday weekday entries + 17 monthly + 2 quarterly + 2 yearly (May 1st) + 2 yearly (Jan 1st — does not fire on Jan 4) + 1 monthly (day 5 — does not fire on Jan 4) = 12 + 17 + 2 + 2 = 33 entries; verify by calling `schedule.TasksForDate(schedule.NewDate(2025, time.January, 4))` in the test and using `len(...)` for the assertion. The test uses the same accessor the trigger uses, guaranteeing the two stay in sync.
- The "recurrence label coverage" test in `pkg/tick/tick_test.go` (line 377) gains a `weekday` `Entry`; the test now enumerates 6 kinds. The "Prometheus pre-initialization" test in `pkg/tick/tick_test.go` (line 463) updates its assertion from 10 series to 12 series (6 kinds × 2 results) and adds `"weekday"` to the `BeElementOf` list of valid kinds. The metric surface itself (the `init()` in `pkg/tick/metrics.go`) needs no change — it iterates `schedule.AllRecurrenceKinds` which now has 6 entries.
- The `sundayWeeklyAllowList` package-level var in `pkg/schedule/inventory_validation_test.go` is DELETED. The 3 `It` cases that depend on it (`has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList`, `every weekly entry has Weekday in {time.Saturday, time.Sunday}`, `every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList`) are REPLACED with 2 new `It` cases: one asserting `RecurrenceWeekday` requires non-zero `Weekday`, and one asserting `RecurrenceWeekly` requires zero `Weekday`. The "uses recurrence kinds from the closed set" test (line 88) is updated to add `RecurrenceWeekday` to the `allowed` map.
- A new inventory validation spec asserts: 12 entries have `Recurrence: RecurrenceWeekday, Weekday: time.Saturday`; 9 entries have `Recurrence: RecurrenceWeekday, Weekday: time.Sunday`; 0 entries have `Recurrence: RecurrenceWeekly`. The 3 counts together cover the spec's Acceptance Criteria 2, 3, 4.
- A new spec in `pkg/publisher/publisher_test.go` (a `DescribeTable` covering all 6 kinds) asserts the post-Spec-9 period token for representative dates: `buildPeriodToken(RecurrenceWeekday, 2026-06-21)` returns `"2026W25-sun"`, `buildPeriodToken(RecurrenceWeekly, 2026-06-16)` returns `"2026W25"`, and the 4 always-fire kinds continue to emit the same tokens they did in Spec 8. The existing period-token tests in `publisher_test.go` (the `period anchoring` and `period-token byte-equality` and `appends '<bare> - <period-token>' for every RecurrenceKind` blocks) are updated: the `RecurrenceWeekly` case changes from `2026W25-sat` to `2026W25` (no weekday suffix), and a new `RecurrenceWeekday` case is added with `2026W25-sat`.
- A new spec in `pkg/handler/trigger_test.go` exercises the new date-filtered behavior: 3 tests cover 3 representative dates (Tuesday 2025-01-07 → 0 weekday entries; Saturday 2025-01-04 → 12 weekday entries; Sunday 2025-01-05 → 9 weekday entries) and assert the published count matches `len(schedule.TasksForDate(date))`. The existing tests that assert the full 45-entry publish are replaced with date-filtered assertions.
- `CHANGELOG.md` gains ONE `feat:` bullet under `## Unreleased` describing the kind split (per the spec's AC #11). The bullet is the ONLY place the changelog mentions Spec 9.

</summary>

<objective>

Migrate the 21 weekly-with-`Weekday` inventory entries from `RecurrenceWeekly` to `RecurrenceWeekday`; update the publisher's `buildPeriodToken` switch so the 21 entries' UUID5 input strings are byte-identical to pre-Spec-9 (preserving all existing vault files); wire the tick and the trigger handler to filter by date using the `TasksForDate` accessor added in Prompt 1; update the metric surface, the inventory validation tests (delete the now-obsolete `sundayWeeklyAllowList` var), the publisher tests, the trigger tests, and the tick tests; append the changelog entry. The build remains green; the regression (Saturday/Sunday tasks materializing on every weekday) is fixed; no duplicate vault files appear after deploy.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before making changes:

- `/workspace/pkg/schedule/inventory.go` — the 45-entry inventory slice (post-Prompt-1: still 21 entries with `Recurrence: RecurrenceWeekly, Weekday: time.Saturday|time.Sunday`). The 12 Saturday slugs (in declaration order) are: `shutdown-k3s`, `turn-on-hell`, `weekly-review`, `check-ftmo-demo-accounts`, `lexoffice-invoices`, `moneymoney-review`, `opnsense-update`, `home-assistant-update-backup`, `renew-gmail-oauth-tokens`, `plan-next-week`, `run-update-all-saturday`, `topic-backup-saturday`. The 9 Sunday slugs (in declaration order) are: `complete-rsync-backups`, `complete-longhorn-backups`, `turn-off-hell`, `turn-off-sun`, `turn-off-fire`, `docker-registry-gc`, `rebuild-trading-dev-prod`, `check-bot-is-healthy`, `run-update-all`. This prompt changes the `Recurrence: RecurrenceWeekly` line on all 21 entries to `Recurrence: RecurrenceWeekday`. The `Weekday` field is unchanged. The other 24 entries (17 monthly + 2 quarterly + 2 yearly Jan-1st + 1 monthly day-5 + 2 yearly May-1st) are NOT modified.
- `/workspace/pkg/schedule/inventory_validation_test.go` — the existing `Describe("inventory", ...)` block with 6 `It` cases. After this prompt the block has 5 `It` cases (3 are deleted: `has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList`, `every weekly entry has Weekday in {time.Saturday, time.Sunday}`, `every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList`; 2 are added: `every RecurrenceWeekday entry has non-zero Weekday`, `every RecurrenceWeekly entry has zero Weekday`). The `uses recurrence kinds from the closed set` test gains `RecurrenceWeekday` in its `allowed` map. The 3 unchanged pre-Spec-7 tests (`has unique slugs`, `uses only supported placeholders in TitleTemplate and BodyTemplate`, `has no period placeholders in any TitleTemplate`, `has a non-empty TitleTemplate for every entry`) stay. The `sundayWeeklyAllowList` package-level var is DELETED.
- `/workspace/pkg/schedule/recurrence.go` — 6-value `RecurrenceKind` enum (post-Prompt-1). Unchanged in this prompt.
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition` struct. Unchanged (Prompt 1 updated the `Weekday` GoDoc).
- `/workspace/pkg/schedule/date.go` — `Date` civil-date type. Unchanged.
- `/workspace/pkg/schedule/tasks_for_date.go` — `TasksForDate` and `filterInventoryByDate` (added by Prompt 1). Unchanged in this prompt; this prompt CONSUMES the function.
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest` and `FilterInventoryByDateForTest` accessors. Unchanged in this prompt.
- `/workspace/pkg/schedule/tasks_for_date_test.go` — 8 new specs (added by Prompt 1). Unchanged; they continue to pass.
- `/workspace/pkg/schedule/canonical_slugs_test.go` — the 45-slug canonical list. Unchanged; the 21 migrated slugs are still in the list (slug renames are forbidden).
- `/workspace/pkg/publisher/uuid_namespace.go` — `buildPeriodToken` switch. UPDATED in this prompt: the `case schedule.RecurrenceWeekly` arm now emits bare `YYYYWww` (no weekday suffix); a new `case schedule.RecurrenceWeekday` arm emits `YYYYWww-<3-letter-lowercase-weekday-abbrev>`. The `default` arm is unchanged (still returns the `"buildPeriodToken: unknown recurrence kind"` error for any future kind that is not in the switch). The `buildTaskIdentifier` function and the `uuidNamespace` constant are FROZEN byte-identical.
- `/workspace/pkg/publisher/publisher.go` — `Publisher.Publish` (already updated by Spec 8 Prompt 1 to use `strings.TrimSpace(renderTemplate(...)) + " - " + periodToken`). Unchanged in this prompt. The switch-arm change in `buildPeriodToken` propagates to the title-suffix and the UUID5 identifier transparently — `Publisher.Publish` calls `buildPeriodToken` twice (once via `buildTaskIdentifier`, once directly for the title) and the second call now returns the new shape.
- `/workspace/pkg/publisher/export_test.go` — `UuidNamespaceForTest` and `BuildPeriodTokenForTest`. Unchanged.
- `/workspace/pkg/publisher/publisher_test.go` — UPDATED. The existing `Describe("period anchoring", ...)` block has 2 tests that exercise `RecurrenceWeekly` with a `Weekday: time.Saturday` def literal and assert the period token `2025W24-sat` (or `2025W25-sat`); after the switch-arm change, the period token for `RecurrenceWeekly` is bare `YYYYWww` (no weekday suffix). The 2 tests must be updated: the def literal's `Weekday` field must be removed (or set to `time.Sunday` for clarity, since the field is now forbidden for `RecurrenceWeekly`), the expected period token must be `2025W24` / `2025W25`. The existing `DescribeTable("period-token byte-equality with the formatter output", ...)` block (line 212) has 4 entries (daily, monthly, quarterly, yearly) — no weekly entry; the new `RecurrenceWeekly` and `RecurrenceWeekday` cases are added. The `It("weekly: byte-equality with the formatter output (with weekday suffix)", ...)` test (line 253) uses `RecurrenceWeekly + Weekday=Saturday` and asserts `2025W24-sat`; after the change, the test's def literal is `RecurrenceWeekday + Weekday=Saturday` and the expected token is still `2025W24-sat`. The `It("buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday", ...)` test (line 277) is renamed and updated: def literal changes to `RecurrenceWeekday + Weekday=Saturday`, expected token stays `2026W25-sat`. The `DescribeTable("appends '<bare> - <period-token>' for every RecurrenceKind", ...)` block (line 534) has 5 entries — the `weekly` entry changes from `RecurrenceWeekly + Weekday=Saturday` (asserting `2026W25-sat`) to `RecurrenceWeekly` with no weekday (asserting `2026W25`); a new `weekday` entry is added with `RecurrenceWeekday + Weekday=Saturday` (asserting `2026W25-sat`). The "boundary contract" `DescribeTable` (line 751) has 5 `Entry` cases for the 5 pre-Spec-9 kinds — a new `RecurrenceWeekday` `Entry` is added; the `weekly` `Entry` is unchanged (the kind is still valid; `validate` does not care about the weekday).
- `/workspace/pkg/tick/tick.go` — UPDATED. The `tick` function no longer iterates `t.inventory` directly; it calls `schedule.TasksForDate(date)` to get the date-filtered slice, then iterates THAT. The factory still passes `schedule.Inventory()` (full inventory) to `tick.NewTick`; the per-tick filtering happens inside the tick. The function signature is unchanged.
- `/workspace/pkg/tick/tick_test.go` — UPDATED. The "full inventory" test (line 423) changes from `expected := len(schedule.Inventory())` to `expected := len(schedule.TasksForDate(refDate))` for each of the 3 test instants. The "recurrence label coverage" test (line 377) adds a `weekday` `Entry`. The "Prometheus pre-initialization" test (line 463) changes the `Expect(metrics).To(HaveLen(10))` to `Expect(metrics).To(HaveLen(12))` and adds `"weekday"` to the `BeElementOf` list. The "calls Publish once for a single-entry inventory" test (line 93) and the "calls Publish once for every entry in the inventory" test (line 109) and the "calls Publish once for every entry in the inventory" test (line 109) and the per-task error isolation tests (line 216-294) are unchanged (they use small synthetic inventories; the date filter is irrelevant for a single-entry or all-weekly inventory).
- `/workspace/pkg/tick/metrics.go` — UNCHANGED. The `init()` iterates `schedule.AllRecurrenceKinds` (6 entries after Prompt 1) and pre-initializes 12 series (6 × 2). The test assertion update (above) catches the new count.
- `/workspace/pkg/handler/trigger.go` — UPDATED. `NewTriggerHandler` now iterates `schedule.TasksForDate(date)` instead of `schedule.Inventory()`. The function signature is unchanged.
- `/workspace/pkg/handler/trigger_test.go` — UPDATED. The "publishes every entry in the inventory on /trigger?date=" test (line 78) and the "responds 200 with date, published=N, errors=[]" test (line 90) and the "returns 200 with errors[] populated and published=len(tasks)-1" test (line 124) and the "returns 200 (not 5xx) with published=0 and full errors array" test (line 163) and the "propagates the request context" test (line 187) all change from `tasks := schedule.Inventory()` and `len(tasks) == 45` to `tasks := schedule.TasksForDate(date)` and `len(tasks)` computed from the accessor. The 3 new specs (Tuesday/Saturday/Sunday) live in a new `Describe("date-filtered behavior", ...)` block.
- `/workspace/pkg/factory/factory.go` — `CreateTick` still passes `schedule.Inventory()` to `tick.NewTick`. The factory wiring is unchanged: the tick filters internally. The `CreateTriggerHandler` passes the publisher to `handler.NewTriggerHandler`; the handler internally calls `TasksForDate`. The factory has no civil-date input (the trigger gets it from the HTTP query parameter, the tick gets it from the clock). Unchanged in this prompt.
- `/workspace/cmd/run-once/main.go` — UNCHANGED. Behavior changes transitively through the factory and the tick.
- `/workspace/CHANGELOG.md` — append ONE `feat:` bullet under `## Unreleased` describing the kind split. The bullet names the 6th kind, the `TasksForDate` accessor, the tick/trigger wiring, and the inventory migration.

Coding-guideline references (read inside the YOLO container):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — `RecurrenceKind` is a closed string enum; the new `RecurrenceWeekday` case in `buildPeriodToken` follows the existing arm pattern.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` `errors.Errorf(ctx, ...)` for the `buildPeriodToken` `default` arm; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; external test packages; `DescribeTable` for parameterized coverage; the `localSender` / `localPub` pattern (per-iteration fresh sender) for tests that observe multiple `Publish` calls.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — counterfeiter annotations unchanged (no interface signature changes).
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `- feat: <what> [context]` format; one bullet per logical change; the spec's AC #11 specifies ONE bullet covering the kind split.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% on changed packages; new behavior has new specs; the 21-entry inventory migration is exercised by the new count tests.

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/inventory.go (BEFORE this prompt — the 21 entries to migrate)
// Each of the 21 entries has a line `Recurrence: RecurrenceWeekly,` followed
// by a `Weekday: time.Saturday,` (12 entries) or `Weekday: time.Sunday,`
// (9 entries) line. The migration changes ONLY the `Recurrence` line.
// The slugs (in declaration order) are:
//   Saturday (12): shutdown-k3s, turn-on-hell, weekly-review, check-ftmo-demo-accounts,
//                  lexoffice-invoices, moneymoney-review, opnsense-update,
//                  home-assistant-update-backup, renew-gmail-oauth-tokens,
//                  plan-next-week, run-update-all-saturday, topic-backup-saturday
//   Sunday (9):    complete-rsync-backups, complete-longhorn-backups, turn-off-hell,
//                  turn-off-sun, turn-off-fire, docker-registry-gc,
//                  rebuild-trading-dev-prod, check-bot-is-healthy, run-update-all
```

```go
// pkg/publisher/uuid_namespace.go (BEFORE this prompt — the switch to update)
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
//
// After this prompt the switch reads:
//
// case schedule.RecurrenceWeekly:
//     isoYear, isoWeek := base.ISOWeek()
//     return fmtIsoWeek(isoYear, isoWeek), nil
// case schedule.RecurrenceWeekday:
//     isoYear, isoWeek := base.ISOWeek()
//     return fmtIsoWeek(isoYear, isoWeek) + "-" + weekdayAbbrev(weekday), nil
//
// The RecurrenceWeekly arm no longer reads `weekday`; the parameter is
// still passed (the signature is unchanged) but is ignored for that arm.
// The RecurrenceWeekday arm reads `weekday` and emits the same shape the
// pre-Spec-9 RecurrenceWeekly arm emitted. UUID5 stability is preserved
// for the 21 migrated entries: pre-Spec-9 input was
// "recurring-<slug>-<YYYYWww>-<abbrev>" (where <abbrev> = weekdayAbbrev(def.Weekday));
// post-Spec-9 input is the byte-identical string for any entry with
// Recurrence: RecurrenceWeekday, Weekday: time.Saturday|time.Sunday.
```

```go
// pkg/schedule/tasks_for_date.go (added by Prompt 1, UNCHANGED in this prompt)
func TasksForDate(date schedule.Date) []TaskDefinition {
    return filterInventoryByDate(inventory, date)
}
// The tick and the trigger call this function with the date they care about
// (the trigger's date from the HTTP query; the tick's date from the clock).
```

```go
// pkg/schedule/inventory_validation_test.go (BEFORE this prompt — the var to delete)
// The sundayWeeklyAllowList var sits at lines 18-38 of the file.
// The 3 dependent It cases sit at lines 102-138 of the file:
//   - It("has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList", ...)
//   - It("every weekly entry has Weekday in {time.Saturday, time.Sunday}", ...)
//   - It("every non-weekly entry leaves Weekday at the zero value AND
//        its slug is NOT in sundayWeeklyAllowList", ...)
// All 3 It cases and the var are DELETED in this prompt.
```

</context>

<requirements>

## 1. Migrate the 21 inventory entries from `RecurrenceWeekly` to `RecurrenceWeekday`

In `/workspace/pkg/schedule/inventory.go`, change ONLY the `Recurrence: RecurrenceWeekly,` line on the 21 entries listed below. Do NOT change the `Weekday` line on any entry. Do NOT change the `Slug`, `TitleTemplate`, `BodyTemplate` on any entry. Do NOT change the 24 non-weekly entries.

The 21 edits:

| # | Slug | Old `Recurrence` | New `Recurrence` |
|---|------|------------------|------------------|
| 1 | `shutdown-k3s` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 2 | `turn-on-hell` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 3 | `weekly-review` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 4 | `check-ftmo-demo-accounts` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 5 | `lexoffice-invoices` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 6 | `moneymoney-review` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 7 | `opnsense-update` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 8 | `home-assistant-update-backup` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 9 | `renew-gmail-oauth-tokens` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 10 | `plan-next-week` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 11 | `run-update-all-saturday` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 12 | `topic-backup-saturday` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 13 | `complete-rsync-backups` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 14 | `complete-longhorn-backups` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 15 | `turn-off-hell` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 16 | `turn-off-sun` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 17 | `turn-off-fire` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 18 | `docker-registry-gc` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 19 | `rebuild-trading-dev-prod` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 20 | `check-bot-is-healthy` | `RecurrenceWeekly` | `RecurrenceWeekday` |
| 21 | `run-update-all` | `RecurrenceWeekly` | `RecurrenceWeekday` |

Notes that are load-bearing for the executor:

- The structural edit — find the slug, find the `Recurrence:` line on the same struct literal, change `RecurrenceWeekly` to `RecurrenceWeekday` — does not depend on line numbers (verify by reading the file end-to-end before editing). The line numbers in §0 of `<context>` are the line of the `Recurrence:` declaration as the file stands; they are hints.
- The `Weekday` field on each of the 21 entries is UNCHANGED. The 12 Saturday entries keep `Weekday: time.Saturday,`; the 9 Sunday entries keep `Weekday: time.Sunday,`. UUID5 stability (the spec's Desired Behavior 6) depends on this — the token rendering reads `def.Weekday` regardless of which `RecurrenceKind` arm the switch dispatches into.
- The 24 non-weekly entries (17 monthly + 2 quarterly + 2 yearly Jan-1st + 1 monthly day-5 + 2 yearly May-1st) are NOT modified. They keep `Recurrence: RecurrenceMonthly` / `RecurrenceQuarterly` / `RecurrenceYearly` and no `Weekday` field. The monthly day-5 entry (`update-finances`) keeps `Recurrence: RecurrenceMonthly`; the 2 yearly May-1st entries (`capitalcom-apikey-prod`, `capitalcom-apikey-dev`) keep `Recurrence: RecurrenceYearly`.
- The file's `Copyright (c) 2026` BSD header is preserved.

## 2. Update `buildPeriodToken` to handle `RecurrenceWeekly` (bare) and `RecurrenceWeekday` (suffixed)

In `/workspace/pkg/publisher/uuid_namespace.go`, change the `buildPeriodToken` switch so the `RecurrenceWeekly` arm emits bare `YYYYWww` and a new `RecurrenceWeekday` arm emits `YYYYWww-<abbrev>`. The exact new switch body is:

```go
switch recurrence {
case schedule.RecurrenceDaily:
    return fmtDate(date.Year, int(date.Month), date.Day), nil
case schedule.RecurrenceWeekly:
    isoYear, isoWeek := base.ISOWeek()
    return fmtIsoWeek(isoYear, isoWeek), nil
case schedule.RecurrenceWeekday:
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
```

Also update the function's GoDoc comment block to describe the new shape. The new GoDoc is:

```go
// buildPeriodToken returns the period-anchored token for the given
// (recurrence, date, weekday) triple. The token is the same string the
// corresponding title-rendering formatter produces — "YYYY-MM-DD" for
// daily, "YYYYWNN" for weekly (no weekday suffix; RecurrenceWeekly is
// always-fire and does not carry a weekday), "YYYYWNN-<3-letter-lowercase-
// weekday-abbrev>" for weekday (RecurrenceWeekday carries the target
// weekday in the entry's Weekday field, NOT the date's weekday),
// "YYYY-MM" for monthly, "YYYYQN" for quarterly, "YYYY" for yearly.
// Anchoring by def.Recurrence (not def.Weekday) is intentional: the
// publisher's identifier layer is period-stable, the schedule's intended
// weekday is a hint about which day inside the period the user wants
// to see the task. Berlin local time governs the period boundary;
// the date passed in is already Berlin-local (the tick converts
// wall-clock to Berlin civil date before calling Publish).
//
// RecurrenceWeekly was the only kind carrying a weekday suffix in
// spec 008; spec 009 split it into RecurrenceWeekly (always-fire, no
// suffix) and RecurrenceWeekday (per-weekday, with suffix) to fix
// the regression where Saturday/Sunday tasks materialized on every
// weekday of the ISO week. Existing RecurrenceWeekday entries retain
// the byte-identical period token the pre-spec-9 RecurrenceWeekly
// entries produced for the same (slug, date) — UUID5 identifiers
// are preserved by construction.
//
// An unknown RecurrenceKind is a build-time data error (closed enum,
// no valid runtime reason for a new value), so the function returns
// an error rather than a sentinel string. The caller wraps with
// the slug.
```

Notes that are load-bearing for the executor:

- The `RecurrenceWeekly` arm no longer reads `weekday`. The parameter is still passed (the signature is unchanged) but the `+ "-" + weekdayAbbrev(weekday)` suffix is gone. This is the spec's Desired Behavior 5: "RecurrenceWeekly produces bare YYYYWww".
- The new `RecurrenceWeekday` arm reads `weekday` and emits the same shape the pre-Spec-9 `RecurrenceWeekly` arm emitted (`YYYYWww-<3-letter-lowercase-weekday-abbrev>`). UUID5 stability for the 21 migrated entries is by construction — the input string `recurring-<slug>-<period-token>` is byte-identical to pre-Spec-9.
- The `default` arm is unchanged. It catches any future kind that is added to the enum but not to the switch. The spec's Failure Modes row 5 ("new recurrence kind added later collides with `\"weekday\"` string" → "Pick a different string constant" → "Build fails") is enforced by the closed-enum pattern: any new constant must be added to this switch.
- The `base := date.Time()` line at the top of the function is unchanged. The `isoYear, isoWeek := base.ISOWeek()` pattern is reused for both weekly and weekday arms.
- The `weekdayAbbrev` function (lines 105-123 of the file as it stands) is FROZEN byte-identical. It is now called only by the `RecurrenceWeekday` arm.
- The `buildTaskIdentifier` function (lines 86-99) is FROZEN byte-identical. It calls `buildPeriodToken` internally; the new switch-arm shape propagates to the UUID5 input string transparently.
- The `uuidNamespace` constant (line 26) is FROZEN byte-identical.
- The file's `Copyright (c) 2026` BSD header is preserved.
- The function signature is unchanged: `func buildPeriodToken(ctx context.Context, recurrence schedule.RecurrenceKind, date schedule.Date, weekday time.Weekday) (string, error)`.

## 3. Update the tick to filter by date

In `/workspace/pkg/tick/tick.go`, change the `tick` function so it calls `schedule.TasksForDate(date)` to get the per-tick filtered slice, then iterates THAT instead of `t.inventory` directly. The new tick body is:

```go
func (t *tick) tick(ctx context.Context) {
    now := t.clock.Now().Time().In(t.berlin)
    t.metrics.SetLastTickTimestamp(float64(now.Unix()))
    year, month, day := now.Date()
    date := schedule.NewDate(year, month, day)

    defs := schedule.TasksForDate(date)

    if len(defs) == 0 {
        glog.V(2).Infof("no tasks for date %04d-%02d-%02d", date.Year, date.Month, date.Day)
        return
    }

    for _, def := range defs {
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

Notes that are load-bearing for the executor:

- The `t.inventory` field on the `tick` struct is now UNUSED in the function body. Do NOT remove the field (the struct signature is unchanged; the factory still passes `schedule.Inventory()` to `NewTick` and the field captures it for completeness). Alternative — remove the field entirely and have the factory pass `nil` or have `NewTick` call `schedule.Inventory()` itself — would change the constructor signature or introduce an import cycle risk; the in-place filter via `TasksForDate` is the minimal-diff change. The field is set in `NewTick` and never read after this prompt. If the field is removed in a future refactor spec, this prompt's `t.inventory` references can be deleted then.
- The `len(t.inventory) == 0` early-return is replaced with `len(defs) == 0` — an empty filtered slice is a valid state (e.g. on a Tuesday with zero weekday-pinned entries, if the inventory contained only `RecurrenceWeekday` entries). The log message changes to "no tasks for date <date>" so an operator can distinguish "inventory is empty" from "no entry fires today".
- The `select { case <-ctx.Done(): return; default: }` per-entry context check is preserved.
- The `t.metrics.SetLastTickTimestamp(float64(now.Unix()))` call is preserved at the top of the function — the gauge updates on every tick regardless of whether any entry fires.
- The `t.publisher.Publish(ctx, def, date)` call is unchanged.
- The function signature is unchanged: `func (t *tick) tick(ctx context.Context)`. The `tick` struct's fields (`inventory`, `publisher`, `clock`, `metrics`, `berlin`) are unchanged in declaration; only the `inventory` field is now unused at runtime.
- The `NewTick` constructor's signature and body are unchanged. The factory still passes `schedule.Inventory()`.
- The file's `Copyright (c) 2026` BSD header is preserved.

## 4. Update the trigger handler to filter by date

In `/workspace/pkg/handler/trigger.go`, change `NewTriggerHandler` so it iterates `schedule.TasksForDate(date)` instead of `schedule.Inventory()`. The new function body is:

```go
func NewTriggerHandler(publisher publisher.Publisher) http.Handler {
    return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
        param := req.URL.Query().Get("date")
        if param == "" {
            writeTriggerError(resp, http.StatusBadRequest, "missing date parameter")
            return
        }
        t, err := time.Parse("2006-01-02", param)
        if err != nil {
            writeTriggerError(
                resp,
                http.StatusBadRequest,
                "invalid date format, expected YYYY-MM-DD",
            )
            return
        }
        date := schedule.NewDate(t.Year(), t.Month(), t.Day())
        tasks := schedule.TasksForDate(date)
        sort.Slice(tasks, func(i, j int) bool { return tasks[i].Slug < tasks[j].Slug })

        glog.V(2).
            Infof("trigger: processing %d task(s) for %04d-%02d-%02d", len(tasks), date.Year, date.Month, date.Day)

        out := triggerResponse{
            Date:      fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day),
            Published: 0,
            Errors:    []triggerErrorEntry{},
        }
        for _, def := range tasks {
            if pubErr := publisher.Publish(req.Context(), def, date); pubErr != nil {
                glog.Errorf(
                    "trigger: publish failed for slug %q on %s: %v",
                    def.Slug,
                    param,
                    pubErr,
                )
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
```

Also update the function's GoDoc comment block:

```go
// NewTriggerHandler returns an HTTP handler that replays the recurring-task
// publishes for one civil date. The date is supplied as the `date` query
// parameter in YYYY-MM-DD format. For each entry in the date-filtered
// inventory (schedule.TasksForDate(date), slug-sorted), the handler
// calls publisher.Publish(req.Context(), def, date). Per-task errors
// are accumulated in the response's `errors` array — the iteration does
// NOT short-circuit on error. The response is always HTTP 200 on a
// successfully parsed date, regardless of whether any individual
// publish failed.
//
// Spec 009 added per-weekday firing: RecurrenceWeekday entries fire
// only on their target weekday; RecurrenceWeekly and the four other
// kinds are always-fire. The handler now iterates the date-filtered
// slice (introduced in spec 009) instead of the full inventory. A
// /trigger?date=YYYY-MM-DD call on a Tuesday therefore publishes
// the always-fire entries (daily/weekly/monthly/quarterly/yearly)
// but no RecurrenceWeekday entries whose Weekday is Saturday or Sunday.
//
// The handler holds no per-request state and is safe to call
// concurrently for the same date (the controller dedups by
// deterministic UUID5).
//
// Security: this handler intentionally has no authentication. The
// service is deployed cluster-internal-only (no k8s Ingress); all
// external access is brokered by ~/Documents/workspaces/trading/frontend/
// gateway, which owns auth. The /trigger surface is reachable only
// inside the cluster. Idempotency via deterministic UUID5 also makes
// accidental replay safe.
```

Notes that are load-bearing for the executor:

- The `sort.Slice(tasks, ...)` line is preserved; the date-filtered slice is sorted on Slug for a stable response body.
- The `out := triggerResponse{...}` literal is unchanged.
- The error handling and JSON encoding are unchanged.
- The function signature is unchanged: `func NewTriggerHandler(publisher publisher.Publisher) http.Handler`.
- The file's `Copyright (c) 2026` BSD header is preserved.

## 5. Delete the `sundayWeeklyAllowList` var and the 3 dependent `It` cases in `inventory_validation_test.go`

In `/workspace/pkg/schedule/inventory_validation_test.go`:

- Delete the `sundayWeeklyAllowList` package-level var (lines 18-38 of the file as it stands, including the GoDoc-style comment that introduces it).
- Delete the 3 `It` cases that depend on it:
  - `It("has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList", ...)` (lines 102-107)
  - `It("every weekly entry has Weekday in {time.Saturday, time.Sunday}", ...)` (lines 109-121)
  - `It("every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList", ...)` (lines 123-138)

After deletion, update the `It("uses recurrence kinds from the closed set", ...)` test (line 88) to add `RecurrenceWeekday` to the `allowed` map. The new body is:

```go
It("uses recurrence kinds from the closed set", func() {
    allowed := map[schedule.RecurrenceKind]bool{
        schedule.RecurrenceDaily:     true,
        schedule.RecurrenceWeekly:    true,
        schedule.RecurrenceWeekday:   true,
        schedule.RecurrenceMonthly:   true,
        schedule.RecurrenceQuarterly: true,
        schedule.RecurrenceYearly:    true,
    }
    for _, def := range schedule.AllDefinitionsForTest() {
        Expect(allowed).To(HaveKey(def.Recurrence),
            "entry %q has unknown Recurrence %q", def.Slug, def.Recurrence)
    }
})
```

Then add 3 inventory-count `It` cases that assert the data shape (Go's `time.Sunday == 0` makes "non-zero Weekday" unenforceable as a generic invariant, so the spec's Constraints are encoded as explicit counts):

```go
It("inventory contains exactly 12 Saturday RecurrenceWeekday entries", func() {
    n := 0
    for _, def := range schedule.AllDefinitionsForTest() {
        if def.Recurrence == schedule.RecurrenceWeekday && def.Weekday == time.Saturday {
            n++
        }
    }
    Expect(n).To(Equal(12),
        "expected 12 RecurrenceWeekday entries with Weekday=time.Saturday, got %d", n)
})

It("inventory contains exactly 9 Sunday RecurrenceWeekday entries", func() {
    n := 0
    for _, def := range schedule.AllDefinitionsForTest() {
        if def.Recurrence == schedule.RecurrenceWeekday && def.Weekday == time.Sunday {
            n++
        }
    }
    Expect(n).To(Equal(9),
        "expected 9 RecurrenceWeekday entries with Weekday=time.Sunday, got %d", n)
})

It("inventory contains zero RecurrenceWeekly entries", func() {
    n := 0
    for _, def := range schedule.AllDefinitionsForTest() {
        if def.Recurrence == schedule.RecurrenceWeekly {
            n++
        }
    }
    Expect(n).To(Equal(0),
        "expected 0 RecurrenceWeekly entries after spec 009, got %d", n)
})
```

The 3 count tests cover the spec's Acceptance Criteria 2, 3, 4 directly. The spec's Failure Modes row 1 ("RecurrenceWeekday with zero Weekday") and row 2 ("RecurrenceWeekly with non-zero Weekday") are detected by these counts because any future change that violates the invariant breaks the count. The behavioral test (a Tuesday `TasksForDate` returns zero weekday tasks) is in `tasks_for_date_test.go` (added by Prompt 1) and covers the actual regression.

Notes that are load-bearing for the executor:

- The 4 unchanged pre-Spec-9 tests (`has unique slugs`, `uses only supported placeholders in TitleTemplate and BodyTemplate`, `has no period placeholders in any TitleTemplate`, `has a non-empty TitleTemplate for every entry`) stay.
- The `uses recurrence kinds from the closed set` test gets `RecurrenceWeekday` added to the `allowed` map.
- The `periodTitlePlaceholders` var (added by Spec 8 Prompt 2) is unchanged.
- The `time` import stays; the `sundayWeeklyAllowList` deletion removes no imports.

## 6. Update the publisher tests for the new switch arms and the `RecurrenceWeekday` kind

In `/workspace/pkg/publisher/publisher_test.go`, make the following changes. All changes preserve the existing per-iteration `localSender` / `localPub` pattern and the Ginkgo v2 / Gomega style.

### 6.1. Update the "period anchoring" tests

The two existing `RecurrenceWeekly` tests use a `Weekday: time.Saturday` def literal and assert a period token of `2025W24-sat` / `2025W25-sat`. After this prompt:

- The test "weekly: same ISO week, different civil dates produce the same identifier" (line 84) keeps `RecurrenceWeekly` (the kind) but drops the `Weekday: time.Saturday` from the def literal (the kind no longer carries a weekday). The expected period token for `RecurrenceWeekly` is now bare `2025W24`. The test name is unchanged; the assertion is `Expect(id1).To(Equal(id2))` (the test still proves period-stability across days in the same ISO week). The per-iteration `captureIdentifier` helper does NOT need to change — it still calls `Publish` and reads the identifier; the identifier's UUID5 input string changes from `recurring-w1-2025W24-sat` to `recurring-w1-2025W24`, and the equality assertion is unaffected.
- The test "weekly: adjacent ISO weeks produce different identifiers" (line 141) keeps `RecurrenceWeekly` (the kind), drops the `Weekday: time.Saturday` from the def literal, and asserts `Expect(id1).NotTo(Equal(id2))`. The test name is unchanged; the assertion is unaffected.

### 6.2. Rename the "weekly: byte-equality with the formatter output" test to weekday

The test at line 253 (`It("weekly: byte-equality with the formatter output (with weekday suffix)", ...)`) changes from `RecurrenceWeekly + Weekday=time.Saturday` to `RecurrenceWeekday + Weekday=time.Saturday`. The def literal's `Recurrence` field changes; the `Weekday` field is unchanged. The expected input string is unchanged: `"recurring-byte-eq-weekly-2025W24-sat"`. The test name changes to:

```go
It("weekday: byte-equality with the formatter output (with weekday suffix)", func() {
```

The body uses `localPub := publisher.NewPublisher(localSender, false)` (per the per-iteration pattern) and asserts:

```go
cmd := capture()
expected := "recurring-byte-eq-weekday-2025W24-sat"
want := uuid.NewSHA1(
    publisher.UuidNamespaceForTest(),
    []byte(expected),
).String()
Expect(string(cmd.TaskIdentifier)).To(Equal(want))
```

Note: the slug in the def literal is `"byte-eq-weekday"` (changed from `"byte-eq-weekly"` to keep the slug distinct from the `RecurrenceWeekly` test slug). The test name reflects the kind.

### 6.3. Add a new weekly byte-equality test (no weekday suffix)

After the renamed test (§6.2), add a new test that asserts the new bare-`YYYYWww` shape for `RecurrenceWeekly`:

```go
It("weekly: byte-equality with the formatter output (no weekday suffix)", func() {
    // After spec 009, RecurrenceWeekly is always-fire and the period
    // token is bare YYYYWww (no weekday suffix). The Weekday field is
    // ignored for this kind.
    def := schedule.TaskDefinition{
        Slug:          "byte-eq-weekly-bare",
        TitleTemplate: "t",
        Recurrence:    schedule.RecurrenceWeekly,
    }
    Expect(pub.Publish(
        context.Background(),
        def,
        schedule.NewDate(2025, time.June, 9),
    )).To(Succeed())
    cmd := capture()
    expected := "recurring-byte-eq-weekly-bare-2025W24"
    want := uuid.NewSHA1(
        publisher.UuidNamespaceForTest(),
        []byte(expected),
    ).String()
    Expect(string(cmd.TaskIdentifier)).To(Equal(want))
})
```

### 6.4. Update the "buildPeriodToken: weekly token carries..." test

The test at line 277 (`It("buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday", ...)`) changes its def literal from `RecurrenceWeekly + Weekday=time.Saturday` to `RecurrenceWeekday + Weekday=time.Saturday`. The expected input string is unchanged: `"recurring-weekday-takes-precedence-2026W25-sat"`. The test name changes to:

```go
It("buildPeriodToken: weekday token carries the entry's Weekday, not the date's weekday", func() {
```

The slug in the def literal is unchanged (`"weekday-takes-precedence"`).

### 6.5. Update the "period-token byte-equality" DescribeTable

The `DescribeTable` at line 212 has 4 `Entry` cases (daily, monthly, quarterly, yearly). Add 2 new `Entry` cases: one for `RecurrenceWeekly` (bare `YYYYWww`) and one for `RecurrenceWeekday` (with weekday suffix). The 4 existing entries are unchanged.

The new entries:

```go
Entry(
    "weekly",
    schedule.RecurrenceWeekly,
    schedule.NewDate(2025, time.June, 9),
    "2025W24",
),
Entry(
    "weekday",
    schedule.RecurrenceWeekday,
    schedule.NewDate(2025, time.June, 9), // Monday
    "2025W24-mon",
),
```

The `weekday` entry uses `Weekday: time.Monday` (set on the def literal inside the `DescribeTable` body — the table's body sets `Weekday: time.Saturday` for all current entries; the new weekday entry's body must use a different weekday to make the test meaningful). The current body:

```go
func(rec schedule.RecurrenceKind, date schedule.Date, expectedToken string) {
    localSender := &taskmocks.TaskCreateCommandSender{}
    localSender.SendCommandReturns(nil)
    localPub := publisher.NewPublisher(localSender, false)
    def := schedule.TaskDefinition{
        Slug:          "byte-eq-" + string(rec),
        TitleTemplate: "t",
        Recurrence:    rec,
    }
    Expect(localPub.Publish(context.Background(), def, date)).To(Succeed())
    _, cmd := localSender.SendCommandArgsForCall(0)
    expected := "recurring-" + "byte-eq-" + string(rec) + "-" + expectedToken
    want := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte(expected)).String()
    Expect(string(cmd.TaskIdentifier)).To(Equal(want))
},
```

This body sets no `Weekday` on the def — the new `RecurrenceWeekday` case would then have `Weekday: time.Sunday` (the zero value), and the period token would be `2025W24-sun` (matching the `expectedToken`). Change the body to set `Weekday: time.Monday` for the `RecurrenceWeekday` case so the token is `2025W24-mon`:

```go
func(rec schedule.RecurrenceKind, date schedule.Date, expectedToken string) {
    localSender := &taskmocks.TaskCreateCommandSender{}
    localSender.SendCommandReturns(nil)
    localPub := publisher.NewPublisher(localSender, false)
    def := schedule.TaskDefinition{
        Slug:          "byte-eq-" + string(rec),
        TitleTemplate: "t",
        Recurrence:    rec,
    }
    if rec == schedule.RecurrenceWeekday {
        def.Weekday = time.Monday
    }
    Expect(localPub.Publish(context.Background(), def, date)).To(Succeed())
    _, cmd := localSender.SendCommandArgsForCall(0)
    expected := "recurring-" + "byte-eq-" + string(rec) + "-" + expectedToken
    want := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte(expected)).String()
    Expect(string(cmd.TaskIdentifier)).To(Equal(want))
},
```

### 6.6. Update the "appends '<bare> - <period-token>'" DescribeTable

The `DescribeTable` at line 534 has 5 `Entry` cases (daily, weekly, monthly, quarterly, yearly). The `weekly` entry's expected token changes from `2026W25-sat` to `2026W25` (no weekday suffix). A new `weekday` entry is added with `RecurrenceWeekday + Weekday=time.Saturday` and expected token `2026W25-sat`.

The body of the `DescribeTable` (which iterates each `Entry`) sets `Weekday: time.Saturday` on every def literal. The `weekly` entry's `Weekday: time.Saturday` is now ignored (per the new `RecurrenceWeekly` arm); the expected token for that entry changes to `2026W25`. The new `weekday` entry's `Weekday: time.Saturday` is consumed by the `RecurrenceWeekday` arm; the expected token is `2026W25-sat`.

The new entry list:

```go
Entry(
    "daily",
    schedule.RecurrenceDaily,
    schedule.NewDate(2026, time.June, 15),
    "2026-06-15",
),
Entry(
    "weekly",
    schedule.RecurrenceWeekly,
    schedule.NewDate(2026, time.June, 17), // Wed in ISO 2026W25
    "2026W25",
),
Entry(
    "weekday",
    schedule.RecurrenceWeekday,
    schedule.NewDate(2026, time.June, 17), // Wed in ISO 2026W25; Saturday is the entry's weekday
    "2026W25-sat",
),
Entry(
    "monthly",
    schedule.RecurrenceMonthly,
    schedule.NewDate(2026, time.June, 15),
    "2026-06",
),
Entry(
    "quarterly",
    schedule.RecurrenceQuarterly,
    schedule.NewDate(2026, time.April, 1),
    "2026Q2",
),
Entry(
    "yearly",
    schedule.RecurrenceYearly,
    schedule.NewDate(2026, time.January, 1),
    "2026",
),
```

### 6.7. Update the "boundary contract" DescribeTable

The `DescribeTable` at line 751 has 5 `Entry` cases for the 5 pre-Spec-9 kinds. Add a new `Entry` for `RecurrenceWeekday`:

```go
Entry("weekday", schedule.RecurrenceWeekday),
```

The other 5 entries are unchanged. The new entry's `def` literal needs `Weekday: time.Saturday` for the `task.CreateCommand.Validate` call to succeed; the body of the `DescribeTable` currently sets no `Weekday` on the def — change the body to:

```go
func(kind schedule.RecurrenceKind) {
    def := schedule.TaskDefinition{
        Slug:          "test-slug",
        TitleTemplate: "Title for " + string(kind),
        BodyTemplate:  "Body for " + string(kind),
        Recurrence:    kind,
    }
    if kind == schedule.RecurrenceWeekday {
        def.Weekday = time.Saturday
    }
    Expect(pub.Publish(
        context.Background(),
        def,
        schedule.NewDate(2025, time.January, 4),
    )).To(Succeed())
    captured := capture()
    Expect(captured.Validate(context.Background())).To(Succeed())
},
```

### 6.8. Update the "full-inventory render" test

The test at line 584 iterates every entry in `schedule.Inventory()` and asserts each rendered title ends with ` - ` + the period token. After this prompt, the inventory is migrated (21 entries now `RecurrenceWeekday` with `Weekday: time.Saturday|time.Sunday`); the period token for those entries is `YYYYWww-<abbrev>` (the same shape the pre-Spec-9 `RecurrenceWeekly + Weekday` produced). The test is unchanged — the per-entry period-token lookup via `BuildPeriodTokenForTest` is the source of truth, and the new switch arms produce the same strings the old arms did for the 21 migrated entries. The test continues to pass without modification.

### 6.9. Add a UUID5 stability table-driven test

After the `Describe("full-inventory render", ...)` block, add a new `Describe` block:

```go
Describe("UUID5 stability for the 21 migrated weekday entries", func() {
    // Spec 009 migrated 21 entries from RecurrenceWeekly+Weekday to
    // RecurrenceWeekday+Weekday. The period token shape for these
    // entries is byte-identical to pre-spec-9 (YYYYWww-<abbrev>), so
    // the UUID5 input string "recurring-<slug>-<period-token>" is
    // byte-identical, so the identifier is byte-identical, so the
    // vault filename is byte-identical — no duplicates after deploy.
    //
    // This test enumerates all 21 slugs with the hand-derived pre-spec-9
    // expected input strings and asserts equality. If any of the 21
    // expected strings is wrong, the test fails and the deploy is
    // blocked. If the publisher's switch or the inventory migration
    // diverges from the pre-spec-9 shape, the test fails and the
    // regression is caught at build time.
    type stabilityCase struct {
        slug           string
        recurrence     schedule.RecurrenceKind
        weekday        time.Weekday
        date           schedule.Date
        expectedInput  string
    }
    cases := []stabilityCase{
        // 12 Saturday entries
        {slug: "shutdown-k3s", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-shutdown-k3s-2026W25-sat"},
        {slug: "turn-on-hell", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-turn-on-hell-2026W25-sat"},
        {slug: "weekly-review", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-weekly-review-2026W25-sat"},
        {slug: "check-ftmo-demo-accounts", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-check-ftmo-demo-accounts-2026W25-sat"},
        {slug: "lexoffice-invoices", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-lexoffice-invoices-2026W25-sat"},
        {slug: "moneymoney-review", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-moneymoney-review-2026W25-sat"},
        {slug: "opnsense-update", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-opnsense-update-2026W25-sat"},
        {slug: "home-assistant-update-backup", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-home-assistant-update-backup-2026W25-sat"},
        {slug: "renew-gmail-oauth-tokens", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-renew-gmail-oauth-tokens-2026W25-sat"},
        {slug: "plan-next-week", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-plan-next-week-2026W25-sat"},
        {slug: "run-update-all-saturday", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-run-update-all-saturday-2026W25-sat"},
        {slug: "topic-backup-saturday", recurrence: schedule.RecurrenceWeekday, weekday: time.Saturday, date: schedule.NewDate(2026, time.June, 20), expectedInput: "recurring-topic-backup-saturday-2026W25-sat"},
        // 9 Sunday entries
        {slug: "complete-rsync-backups", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-complete-rsync-backups-2026W25-sun"},
        {slug: "complete-longhorn-backups", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-complete-longhorn-backups-2026W25-sun"},
        {slug: "turn-off-hell", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-turn-off-hell-2026W25-sun"},
        {slug: "turn-off-sun", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-turn-off-sun-2026W25-sun"},
        {slug: "turn-off-fire", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-turn-off-fire-2026W25-sun"},
        {slug: "docker-registry-gc", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-docker-registry-gc-2026W25-sun"},
        {slug: "rebuild-trading-dev-prod", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-rebuild-trading-dev-prod-2026W25-sun"},
        {slug: "check-bot-is-healthy", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-check-bot-is-healthy-2026W25-sun"},
        {slug: "run-update-all", recurrence: schedule.RecurrenceWeekday, weekday: time.Sunday, date: schedule.NewDate(2026, time.June, 21), expectedInput: "recurring-run-update-all-2026W25-sun"},
    }
    DescribeTable(
        "produces byte-identical UUID5 input string to pre-spec-9",
        func(c stabilityCase) {
            localSender := &taskmocks.TaskCreateCommandSender{}
            localSender.SendCommandReturns(nil)
            localPub := publisher.NewPublisher(localSender, false)
            def := schedule.TaskDefinition{
                Slug:          c.slug,
                TitleTemplate: "t",
                Recurrence:    c.recurrence,
                Weekday:       c.weekday,
            }
            Expect(localPub.Publish(context.Background(), def, c.date)).To(Succeed())
            _, cmd := localSender.SendCommandArgsForCall(0)
            want := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte(c.expectedInput)).String()
            Expect(string(cmd.TaskIdentifier)).To(Equal(want),
                "entry %q identifier changed; UUID5 input string must be byte-identical to pre-spec-9",
                c.slug)
        },
        // The DescribeTable iterates `cases` by index; wrap each case in an Entry.
        func() []TableEntry {
            out := make([]TableEntry, 0, len(cases))
            for i, c := range cases {
                c := c
                out = append(out, Entry(
                    fmt.Sprintf("%02d-%s", i, c.slug),
                    c,
                ))
            }
            return out
        }()...,
    )
})
```

Notes that are load-bearing for the executor:

- The `fmt` package is needed for the `Entry` name formatting. The current `publisher_test.go` does NOT import `fmt`; add it to the import block in goimports-reviser order (alphabetical, in the standard-library block: `context`, `fmt`, `time`).
- The 21 hand-derived expected input strings are the byte-identical pre-Spec-9 strings. The pre-Spec-9 token for `RecurrenceWeekly + Weekday=time.Saturday + date=2026-06-20 (a Saturday)` was `2026W25-sat` (the Saturday is the date's weekday AND the entry's weekday; both happen to agree on this particular date — the test uses a date whose weekday matches the entry's weekday to make the token unambiguous). The same for the Sunday entries on 2026-06-21 (a Sunday). The test relies on the 2026-06-20/2026-06-21 date pair being Saturday/Sunday respectively — verify with `time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC).Weekday() == time.Saturday` and `time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC).Weekday() == time.Sunday` (the test is implicitly correct; the expected token's `<abbrev>` matches the entry's `Weekday`, not the date's weekday).
- The per-iteration `localSender` / `localPub` pattern is used; the `pub` and `capture()` from the parent suite are NOT used (the `DescribeTable`'s body makes 21 independent `Publish` calls and reads each via `localSender.SendCommandArgsForCall(0)`).
- The `Entry` names are `00-shutdown-k3s`, `01-turn-on-hell`, etc. — they encode the order so a failing case is identifiable in the test output.

## 7. Update the tick tests for the new 6-kind shape and the date-filter behavior

In `/workspace/pkg/tick/tick_test.go`, make the following changes.

### 7.1. Update the "full inventory" test

The test at line 423 currently asserts `expected := len(schedule.Inventory())` (the full 45-entry count). After this prompt, the tick filters by date via `schedule.TasksForDate(date)`. The test must use the filtered count:

```go
Describe("full inventory", func() {
    It("publishes every entry that fires on the given civil date", func() {
        // Derive expected count at test time (NOT a hardcoded literal).
        // The tick now filters by date via schedule.TasksForDate(date);
        // a Tuesday publishes 0 weekday entries, a Saturday publishes
        // 12, a Sunday publishes 9, plus the always-fire kinds
        // (17 monthly + 2 quarterly + 2 yearly-Jan-1st + 1 monthly-day-5
        // + 2 yearly-May-1st = 24 always-fire on any date in 2025/2026).
        // Use the same accessor the tick uses to guarantee the two
        // stay in sync.
        for _, instant := range []string{
            "2025-01-07T10:00:00Z", // Tuesday
            "2025-01-04T10:00:00Z", // Saturday
            "2025-01-05T10:00:00Z", // Sunday
            "2025-07-04T10:00:00Z", // Friday (different month, no weekday-kind fires)
            "2026-03-01T10:00:00Z", // Sunday (different year)
        } {
            clock.SetNow(libtimetest.ParseDateTime(instant))
            tk, err := tick.NewTick(
                context.Background(),
                schedule.Inventory(),
                pub,
                clock,
                metrics,
            )
            Expect(err).NotTo(HaveOccurred())

            // Compute the civil date the same way the tick does: clock
            // -> Berlin -> civil date.
            now := clock.Now().Time().In(berlin)
            y, m, d := now.Date()
            civilDate := schedule.NewDate(y, m, d)
            expected := len(schedule.TasksForDate(civilDate))
            Expect(expected).To(BeNumerically(">", 0)) // sanity: at least one entry fires

            ctx, cancel := context.WithCancel(context.Background())
            done := make(chan struct{})
            go func() {
                _ = tk.Run(ctx)
                close(done)
            }()

            Eventually(
                func() int { return pub.PublishCallCount() },
                "200ms",
                "5ms",
            ).Should(Equal(expected))
            cancel()
            Eventually(done, "200ms", "5ms").Should(BeClosed())
        }
    })
})
```

Notes that are load-bearing for the executor:

- The test computes the expected count via `len(schedule.TasksForDate(civilDate))` — the same accessor the tick uses internally. This guarantees the test stays in sync with the production filter rule.
- The `berlin` variable used in the test (to convert the clock to Berlin civil date) is NOT currently in scope. The test needs to add the conversion inline. The simplest approach: use `libtimetest` to parse the instant and convert to Berlin; or use the tick's own internal Berlin conversion (which the test does not have direct access to). The cleanest approach: pass a fixed `clock` for each instant, call `clock.Now().Time().In(berlinLoc)` to get the Berlin time, extract the civil date via `.Date()`. The `berlin` `*time.Location` can be created via `time.LoadLocation("Europe/Berlin")` in a `BeforeEach`.
- Add a `BeforeEach` at the top of the `Describe("full inventory", ...)` block to load the Berlin location:

```go
var berlin *time.Location
BeforeEach(func() {
    var err error
    berlin, err = time.LoadLocation("Europe/Berlin")
    Expect(err).NotTo(HaveOccurred())
})
```

- The expected count for the 5 test instants (for the human reviewer's reference; the test itself derives the count from the accessor, not from a hardcoded literal): 2025-01-07 (Tuesday) → 24 always-fire entries (0 weekday-kind fires on a Tuesday); 2025-01-04 (Saturday) → 24 + 12 = 36 entries; 2025-01-05 (Sunday) → 24 + 9 = 33 entries; 2025-07-04 (Friday) → 24 entries (no weekday-kind fires on a Friday); 2026-03-01 (Sunday) → 33 entries. The test uses `len(schedule.TasksForDate(civilDate))` directly so the exact count is whatever the accessor returns; the sanity check `Expect(expected).To(BeNumerically(">", 0))` confirms at least one entry fires for every test instant. The 5 test instants all yield ≥ 24 entries (the always-fire baseline).

- The `libtimetest` import is already present (line 13 of the test file). The `time` import is already present.

### 7.2. Update the "recurrence label coverage" test

The `DescribeTable` at line 377 has 5 `Entry` cases (daily, weekly, monthly, quarterly, yearly). Add a 6th `Entry` for `RecurrenceWeekday`:

```go
Entry("weekday", schedule.RecurrenceWeekday),
```

The body of the `DescribeTable` builds a def literal with `Recurrence: kind` and no `Weekday`. The new `RecurrenceWeekday` case needs `Weekday: time.Saturday` for the test to be meaningful. Update the body to:

```go
func(kind schedule.RecurrenceKind) {
    inventory = []schedule.TaskDefinition{
        {Slug: "x", TitleTemplate: "t", Recurrence: kind},
    }
    if kind == schedule.RecurrenceWeekday {
        inventory[0].Weekday = time.Saturday
    }
    var err error
    tk, err = tick.NewTick(
        context.Background(),
        inventory,
        pub,
        clock,
        metrics,
    )
    Expect(err).NotTo(HaveOccurred())

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    done := make(chan struct{})
    go func() {
        _ = tk.Run(ctx)
        close(done)
    }()

    Eventually(
        func() int { return metrics.IncPublishedCallCount() },
        "100ms",
        "5ms",
    ).Should(Equal(1))

    r, got := metrics.IncPublishedArgsForCall(0)
    Expect(r).To(Equal("success"))
    Expect(got).To(Equal(string(kind)))

    cancel()
    Eventually(done, "200ms", "5ms").Should(BeClosed())
},
```

### 7.3. Update the "Prometheus pre-initialization" test

The test at line 463 has `Expect(metrics).To(HaveLen(10))` and `Expect(k).To(BeElementOf("daily", "weekly", "monthly", "quarterly", "yearly"))`. Update to:

```go
Expect(metrics).To(HaveLen(12))

seen := map[string]bool{}
for _, m := range metrics {
    r := ""
    k := ""
    for _, lp := range m.GetLabel() {
        switch lp.GetName() {
        case "result":
            r = lp.GetValue()
        case "recurrence":
            k = lp.GetValue()
        }
    }
    Expect(r).To(BeElementOf("success", "error"))
    Expect(k).To(BeElementOf("daily", "weekly", "weekday", "monthly", "quarterly", "yearly"))
    seen[r+"/"+k] = true
    Expect(m.GetCounter().GetValue()).To(Equal(0.0))
}
Expect(seen).To(HaveLen(12))
```

The 2 changes: `HaveLen(10)` → `HaveLen(12)` (3 places: the `metrics` length, the `seen` length; the slice length after the loop) and `"weekday"` added to the `BeElementOf` list.

## 8. Update the trigger handler tests for the date-filter behavior

In `/workspace/pkg/handler/trigger_test.go`, make the following changes.

### 8.1. Update the existing 5 tests to use `TasksForDate(date)` for the expected count

The 5 existing tests that assert `PublishCallCount() == len(schedule.Inventory())` (i.e. 45 entries):

- "publishes every entry in the inventory on /trigger?date=" (line 78)
- "responds 200 with date, published=N, errors=[] when all publishes succeed" (line 90)
- "returns 200 with errors[] populated and published=len(tasks)-1 when one publish fails" (line 124)
- "returns 200 (not 5xx) with published=0 and full errors array when every publish fails" (line 163)
- "propagates the request context to publisher.Publish" (line 187)

All 5 tests must be updated to use `len(schedule.TasksForDate(date))` for the expected count. The change pattern (taking the first test as the example):

```go
It("publishes every entry that fires on the given civil date", func() {
    date := schedule.NewDate(2025, time.January, 4)
    tasks := schedule.TasksForDate(date)
    Expect(tasks).NotTo(BeEmpty())

    req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
    resp := httptest.NewRecorder()
    httpHandler.ServeHTTP(resp, req)

    Expect(resp.Code).To(Equal(http.StatusOK))
    Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
})
```

The other 4 tests follow the same pattern: `date := schedule.NewDate(2025, time.January, 4)` (or read the date from the request URL — the test's request URL is `/trigger?date=2025-01-04`, so the date is Saturday 2025-01-04) and `tasks := schedule.TasksForDate(date)`. The `tasks[0].Slug` in the error-isolation tests is replaced with the first slug of the date-filtered slice (e.g. `"backup-atlassian-confluence"`, alphabetically first among the entries that fire on 2025-01-04).

Notes that are load-bearing for the executor:

- The 2025-01-04 (Saturday) date fires 33 entries (24 always-fire + 9 Sunday weekly — wait, no: 9 Sunday entries fire on a Sunday, not Saturday; on a Saturday the 12 Saturday entries fire, not the 9 Sunday entries). Recompute: 24 always-fire + 12 Saturday weekday = 36 entries. Verify by calling `schedule.TasksForDate(schedule.NewDate(2025, time.January, 4))` and using `len(...)` directly in the test — the test does NOT hardcode the count, it derives it from the accessor.
- The `tasks[0].Slug` for the error-isolation test: alphabetically, the first slug in the 2025-01-04 set is one of the monthly/quarterly/yearly entries (the weekday slugs are mostly lowercase ASCII strings starting with letters later in the alphabet). The test uses `tasks[0].Slug` directly to pick a target, and the assertion `Expect(body.Errors[0].Slug).To(Equal(target))` then references the same variable — the test stays correct regardless of which slug is first.
- The `import "time"` is needed for the `time.January` literal. The test file's existing imports do not include `"time"`; add it.

### 8.2. Add 3 new date-filter behavior specs

After the existing 5 tests, add a new `Describe("date-filter behavior", ...)` block:

```go
Describe("date-filter behavior", func() {
    It("publishes the 12 Saturday weekday entries plus 24 always-fire entries on a Saturday", func() {
        date := schedule.NewDate(2025, time.January, 4) // Saturday
        tasks := schedule.TasksForDate(date)
        Expect(len(tasks)).To(BeNumerically(">", 24)) // 24 always-fire baseline
        Expect(tasks).To(HaveLen(len(schedule.TasksForDate(date))))

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
    })

    It("publishes the 9 Sunday weekday entries plus 24 always-fire entries on a Sunday", func() {
        date := schedule.NewDate(2025, time.January, 5) // Sunday
        tasks := schedule.TasksForDate(date)
        Expect(tasks).To(HaveLen(len(schedule.TasksForDate(date))))

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-05", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
    })

    It("publishes 0 weekday-kind tasks on a Tuesday (regression fix)", func() {
        date := schedule.NewDate(2025, time.January, 7) // Tuesday
        tasks := schedule.TasksForDate(date)
        weekdayKinds := 0
        for _, def := range tasks {
            if def.Recurrence == schedule.RecurrenceWeekday {
                weekdayKinds++
            }
        }
        Expect(weekdayKinds).To(Equal(0),
            "expected zero RecurrenceWeekday entries on a Tuesday, got %d", weekdayKinds)

        req := httptest.NewRequest("GET", "/trigger?date=2025-01-07", nil)
        resp := httptest.NewRecorder()
        httpHandler.ServeHTTP(resp, req)

        Expect(resp.Code).To(Equal(http.StatusOK))
        Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
    })
})
```

Notes that are load-bearing for the executor:

- The 3 new specs cover the spec's Acceptance Criteria 5, 6, 7: `TasksForDate` on a Tuesday / Saturday / Sunday produces the expected weekday-kind counts. The Tuesday test asserts zero weekday-kind tasks; the Saturday test asserts the 12 Saturday entries fire; the Sunday test asserts the 9 Sunday entries fire. The exact counts (33, 36, 24) are not hardcoded — the test derives them from `schedule.TasksForDate(date)`.
- The test imports: `time` (for `time.January`), the existing `schedule` import. The `time` import is added to the import block.
- The 3 new specs use the parent suite's `httpHandler` and `fakePublisher` (defined in the `BeforeEach` at line 28-31). No new `BeforeEach` is needed.

## 9. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (ONE `feat:` bullet per the spec's AC #11):

```markdown
- feat: Split `RecurrenceWeekly` into always-fire (`RecurrenceWeekly`, bare `YYYYWww` period token) and per-weekday (`RecurrenceWeekday`, `YYYYWww-<3-letter-weekday-abbrev>` period token); migrate the 21 weekly-with-`Weekday` inventory entries (12 Saturday + 9 Sunday) to `RecurrenceWeekday`; add `schedule.TasksForDate(date)` accessor that filters the inventory by civil date (always-fire kinds fire on every day; `RecurrenceWeekday` fires only on its target weekday); the tick and `GET /trigger?date=` now iterate `TasksForDate(date)` instead of the full inventory — fixes the regression where Saturday/Sunday tasks materialized on every weekday of the ISO week; existing vault files retain the same UUID5 identifier (period-token shape is byte-identical for the 21 migrated entries)
```

Notes that are load-bearing for the executor:

- This is the only `feat:` bullet for spec 009. The bullet names the 6th kind, the `TasksForDate` accessor, the tick/trigger wiring, the inventory migration count (21 entries), and the regression it fixes. The "byte-identical" phrase is the load-bearing claim for UUID5 stability — the human reviewer can `grep -nE 'byte-identical' CHANGELOG.md` to find the bullet.
- The bullet does NOT name the deleted `sundayWeeklyAllowList` var or the test file changes — those are internal scaffolding, not user-visible.
- The bullet's `feat:` prefix signals a minor version bump per the changelog guide.

## 10. Imports and conventions

- The modified `/workspace/pkg/schedule/inventory.go` has no import changes (`time` is already imported; no new imports).
- The modified `/workspace/pkg/schedule/inventory_validation_test.go` has no new imports. The `sundayWeeklyAllowList` var deletion removes no imports (the var was a value, not an import).
- The modified `/workspace/pkg/publisher/uuid_namespace.go` has no new imports. The `errors` package is already imported and used by the `default` arm.
- The modified `/workspace/pkg/publisher/publisher_test.go` adds `fmt` to the import block (for the `Entry` name formatting in the new UUID5 stability test). Keep goimports-reviser order: standard library first (alphabetical: `context`, `fmt`, `time`), then third-party (ginkgo, gomega, the bborbe/agent libs, the cqrs libs, the google/uuid lib), then internal (`schedule`).
- The modified `/workspace/pkg/tick/tick.go` has no new imports. The `schedule.TasksForDate` call uses the existing `schedule` import.
- The modified `/workspace/pkg/tick/tick_test.go` has no new imports. The `berlin` location is loaded in a new `BeforeEach`; the `time` import is already present.
- The modified `/workspace/pkg/handler/trigger.go` has no new imports. The `schedule.TasksForDate` call uses the existing `schedule` import.
- The modified `/workspace/pkg/handler/trigger_test.go` adds `time` to the import block. Keep goimports-reviser order: standard library first (alphabetical: `context`, `encoding/json`, `net/http`, `net/http/httptest`, `time`), then third-party (ginkgo, gomega, the bborbe/errors lib), then internal (`mocks`, `handler`, `schedule`).
- The 2026 copyright header is preserved on all modified files.
- Use Ginkgo v2 / Gomega style with dot-imports (matches the existing tests).
- Do NOT touch `pkg/factory/`, `main.go`, `cmd/run-once/`, the Makefile, k8s manifests, the Prometheus metric surface (the `init()` count change is automatic; do not edit it), or the `pkg/schedule/recurrence.go` / `pkg/schedule/task_definition.go` / `pkg/schedule/date.go` / `pkg/schedule/tasks_for_date.go` files (Prompt 1 covered them).
- Do NOT regenerate any counterfeiter mock. No interface signatures change.
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- The `RecurrenceKind` enum is FROZEN at 6 values (set by Prompt 1). No 7th value may be added in this prompt.
- The `buildPeriodToken` switch in `/workspace/pkg/publisher/uuid_namespace.go` MUST have exactly 6 `case` arms (one per kind) plus the `default` arm. The new `RecurrenceWeekday` arm emits `YYYYWww-<abbrev>`; the `RecurrenceWeekly` arm emits bare `YYYYWww`. The `default` arm is unchanged.
- The 21 inventory entries' `Recurrence` field MUST be changed to `RecurrenceWeekday`. The 24 non-weekly entries' `Recurrence` field is UNCHANGED. The 21 entries' `Weekday` field is UNCHANGED.
- UUID5 stability: the 21 migrated entries' UUID5 input string `recurring-<slug>-<period-token>` MUST be byte-identical to pre-Spec-9. The new `RecurrenceWeekday` switch arm produces the same period-token string (`YYYYWww-<abbrev>`) that the pre-Spec-9 `RecurrenceWeekly` switch arm produced for the same (slug, date) pair. The table-driven test in §6.9 enumerates all 21 slugs and asserts byte-for-byte equality. If the test fails, the deploy is blocked.
- The `sundayWeeklyAllowList` var in `/workspace/pkg/schedule/inventory_validation_test.go` is DELETED. The 3 `It` cases that reference it are REPLACED with 3 inventory-count `It` cases (12 Saturday + 9 Sunday + 0 Weekly). The `uses recurrence kinds from the closed set` test gets `RecurrenceWeekday` added to the `allowed` map.
- The tick (`pkg/tick/tick.go`) MUST call `schedule.TasksForDate(date)` and iterate the result. The factory still passes `schedule.Inventory()`; the tick does the filtering. The `t.inventory` field on the `tick` struct is set by `NewTick` but not read by `tick` after this prompt (the field is dead code at runtime; removal is a future refactor).
- The trigger handler (`pkg/handler/trigger.go`) MUST call `schedule.TasksForDate(date)` and iterate the result. The factory wiring is unchanged.
- The "full inventory" tick test in `/workspace/pkg/tick/tick_test.go` (line 423) MUST use `len(schedule.TasksForDate(civilDate))` for the expected count, NOT `len(schedule.Inventory())`. The 3 test instants (Tuesday / Saturday / Sunday / Friday / Sunday-in-2026) exercise the new date-filter semantic.
- The "recurrence label coverage" tick test in `/workspace/pkg/tick/tick_test.go` (line 377) MUST enumerate 6 kinds (daily / weekly / weekday / monthly / quarterly / yearly).
- The "Prometheus pre-initialization" tick test in `/workspace/pkg/tick/tick_test.go` (line 463) MUST assert 12 series (6 kinds × 2 results) and include `"weekday"` in the `BeElementOf` list.
- The trigger handler tests in `/workspace/pkg/handler/trigger_test.go` MUST use `len(schedule.TasksForDate(date))` for the expected count, NOT `len(schedule.Inventory())`. The 3 new date-filter behavior specs cover the spec's AC #5, 6, 7.
- The `pkg/publisher/publisher_test.go` tests MUST be updated for the new switch arms: the `RecurrenceWeekly` byte-equality cases drop the `Weekday` field and assert bare `YYYYWww`; the new `RecurrenceWeekday` byte-equality cases add the kind with `Weekday=time.Saturday` and assert `YYYYWww-sat`; the "appends '<bare> - <period-token>'" `DescribeTable` (line 534) adds a `weekday` entry; the "boundary contract" `DescribeTable` (line 751) adds a `weekday` entry; the "full-inventory render" test (line 584) is unchanged but continues to pass.
- The `uuidNamespace` constant in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN byte-identical.
- The `recurring-<slug>-<period-token>` UUID5 input string format is FROZEN.
- The `Publisher` interface signature in `/workspace/pkg/publisher/publisher.go` is FROZEN.
- The `task.CreateCommandSender` interface signature is FROZEN.
- The `task.CreateCommand` shape is FROZEN.
- The `buildFrontmatter` function in `/workspace/pkg/publisher/frontmatter.go` is FROZEN (6-key shape from Spec 8).
- The `buildPeriodToken` function signature in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN.
- The 6-key frontmatter shape is FROZEN.
- The `Europe/Berlin` civil-date conversion logic in `pkg/tick/tick.go` is FROZEN.
- The `schedule.Inventory()` accessor in `/workspace/pkg/schedule/inventory.go` is FROZEN (returns a defensive copy of the 45-entry slice; the factory still calls it; the tick still receives it via the constructor).
- The `schedule.TasksForDate(date)` accessor (added by Prompt 1) is FROZEN at its post-Prompt-1 signature and behavior.
- The 5-entry `time.Weekday` zero-value-as-`time.Sunday` behavior is a stdlib fact, not a project decision. The test in §5 that replaces the `sundayWeeklyAllowList` deletion uses 3 count tests (12 / 9 / 0) instead of the 2 invariant tests (non-zero / zero) because the stdlib zero value cannot be distinguished from a deliberately-set `time.Sunday` in Go. The 3 count tests cover the spec's AC #2, 3, 4 directly.
- Existing tests must still pass after all edits. The 4 pre-Spec-7 `It` cases in `Describe("inventory", ...)` continue to pass. The 8 new `TasksForDate` `It` cases from Prompt 1 continue to pass. The publisher's `period anchoring` and `period-token byte-equality` and `appends '<bare> - <period-token>'` and `boundary contract` tests are updated but continue to pass. The publisher's `full-inventory render` test is unchanged and continues to pass. The publisher's `determinism` and `placeholder rendering` and `sender interaction` and `errors` tests are unchanged and continue to pass.
- Coverage on the changed packages stays at or above 80%. The new UUID5 stability test (21 cases) and the new trigger date-filter behavior tests (3 cases) and the new publisher switch-arm tests (5+ cases) and the new tick metric tests (1 case updated + 1 case updated) all add coverage.
- Project DoD (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` for error wrapping (the updated `buildPeriodToken` `default` arm uses it; no new wrapping needed); no `context.Background()` in business logic (the new code uses the function's `ctx` parameter; the test code uses `context.Background()` per project convention); no `time.Time` / `time.Now()` in business logic (the new code uses `date.Time()` and `clock.Now().Time().In(t.berlin)`, not `time.Now()`); GoDoc on the updated `NewTriggerHandler` and `buildPeriodToken` (provided in §2 and §4); `make precommit` clean.
- Do NOT commit — dark-factory handles git.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/schedule/...` — all Ginkgo specs green. In particular:
   - The 4 pre-Spec-7 `It` cases in `Describe("inventory", ...)` continue to pass.
   - The 3 new count `It` cases (`inventory contains exactly 12 Saturday RecurrenceWeekday entries`, `inventory contains exactly 9 Sunday RecurrenceWeekday entries`, `inventory contains zero RecurrenceWeekly entries`) pass with count 12, 9, 0 respectively.
   - The `uses recurrence kinds from the closed set` test passes with `RecurrenceWeekday` in the `allowed` map.
   - The 8 `It` cases in `Describe("TasksForDate", ...)` from Prompt 1 continue to pass.
3. `go test ./pkg/publisher/...` — all Ginkgo specs green. In particular:
   - The 2 new `RecurrenceWeekly` / `RecurrenceWeekday` byte-equality `It` cases pass.
   - The renamed `buildPeriodToken: weekday token carries...` test passes.
   - The `DescribeTable("period-token byte-equality with the formatter output", ...)` passes for all 6 entries (daily / weekly / weekday / monthly / quarterly / yearly).
   - The `DescribeTable("appends '<bare> - <period-token>' for every RecurrenceKind", ...)` passes for all 6 entries.
   - The `DescribeTable("produced command passes task.CreateCommand.Validate", ...)` passes for all 6 entries.
   - The new `DescribeTable("produces byte-identical UUID5 input string to pre-spec-9", ...)` passes for all 21 cases.
   - The pre-existing tests (identifier byte-equality, period anchoring, placeholder rendering, title suffix, frontmatter, sender interaction, full-inventory render, errors, determinism) continue to pass.
4. `go test ./pkg/tick/...` — all Ginkgo specs green. In particular:
   - The "full inventory" test passes for all 5 instants (Tuesday / Saturday / Sunday / Friday / Sunday-in-2026); the expected count is `len(schedule.TasksForDate(civilDate))` for each instant.
   - The "recurrence label coverage" test passes for all 6 kinds (daily / weekly / weekday / monthly / quarterly / yearly).
   - The "Prometheus pre-initialization" test passes with `HaveLen(12)` and `"weekday"` in the `BeElementOf` list.
   - The pre-existing tests (constructor, initial tick, publish-per-entry, Berlin date conversion, empty inventory, per-task error isolation, context cancellation, metrics gauge) continue to pass.
5. `go test ./pkg/handler/...` — all Ginkgo specs green. In particular:
   - The 4 missing-date / invalid-date tests continue to pass (no inventory access on the 400 paths).
   - The 5 updated tests (publishes every entry / responds 200 / errors populated / all fail / propagates context) pass with `len(schedule.TasksForDate(date))` for the expected count.
   - The 3 new `Describe("date-filter behavior", ...)` tests pass: Tuesday=0 weekday, Saturday=12 weekday + 24 always-fire, Sunday=9 weekday + 24 always-fire.
6. `go test ./...` — full test suite is green. No regression in any other package.
7. `grep -nE 'RecurrenceWeekly' pkg/schedule/inventory.go` — must return 0 matches (the 21 weekly entries are migrated). The 24 non-weekly entries do NOT use `RecurrenceWeekly`.
8. `grep -nE 'RecurrenceWeekday' pkg/schedule/inventory.go` — must return 21 matches (the 21 migrated entries).
9. `grep -nE 'sundayWeeklyAllowList' pkg/schedule/inventory_validation_test.go` — must return 0 matches (the var is deleted).
10. `grep -nE 'case schedule\.RecurrenceWeekday:' pkg/publisher/uuid_namespace.go` — must return exactly 1 match (the new switch arm).
11. `grep -nE 'case schedule\.RecurrenceWeekly:' pkg/publisher/uuid_namespace.go` — must return exactly 1 match (the updated switch arm).
12. `grep -nE 'weekdayAbbrev\(weekday\)' pkg/publisher/uuid_namespace.go` — must return exactly 1 match (only the `RecurrenceWeekday` arm reads `weekday`; the `RecurrenceWeekly` arm no longer does).
13. `grep -nE 'TasksForDate' pkg/tick/tick.go pkg/handler/trigger.go` — must return at least 1 match in each file (the tick and the trigger call the new accessor).
14. `grep -nE 'RecurrenceWeekday' pkg/tick/tick.go pkg/handler/trigger.go` — must return 0 matches in each file (the tick and the trigger do NOT switch on `RecurrenceWeekday`; the filter happens inside `TasksForDate`).
15. `grep -nE 'schedul(ing|e) in week day' CHANGELOG.md` — must return at least 1 line under `## Unreleased` (the new `feat:` bullet).
16. Spot-check: open `/workspace/pkg/schedule/inventory.go` and visually confirm (a) exactly 21 entries have `Recurrence: RecurrenceWeekday,`; (b) 12 of those have `Weekday: time.Saturday,` and 9 have `Weekday: time.Sunday,`; (c) no entry has `Recurrence: RecurrenceWeekly,`; (d) the 24 non-weekday entries are unchanged.
17. Spot-check: open `/workspace/pkg/publisher/uuid_namespace.go` and visually confirm (a) the `RecurrenceWeekly` arm emits bare `fmtIsoWeek(isoYear, isoWeek)` (no weekday suffix); (b) the new `RecurrenceWeekday` arm emits `fmtIsoWeek(isoYear, isoWeek) + "-" + weekdayAbbrev(weekday)`; (c) the `default` arm is unchanged; (d) the `buildTaskIdentifier` function is unchanged; (e) the `uuidNamespace` constant is unchanged.
18. Spot-check: open `/workspace/pkg/tick/tick.go` and visually confirm (a) the `tick` function calls `schedule.TasksForDate(date)`; (b) the `defs := schedule.TasksForDate(date)` line replaces the `t.inventory` iteration; (c) the `select { case <-ctx.Done(): return; default: }` per-entry check is preserved; (d) the `t.metrics.SetLastTickTimestamp` call is preserved; (e) the `t.publisher.Publish` call is unchanged.
19. Spot-check: open `/workspace/pkg/handler/trigger.go` and visually confirm (a) the `tasks := schedule.TasksForDate(date)` line replaces the `schedule.Inventory()` call; (b) the `sort.Slice(tasks, ...)` is preserved; (c) the error handling and JSON encoding are unchanged; (d) the function signature is unchanged.
20. Spot-check: open `/workspace/pkg/schedule/inventory_validation_test.go` and visually confirm (a) the `sundayWeeklyAllowList` var is GONE; (b) the 3 dependent `It` cases are GONE; (c) the 3 new count `It` cases are present; (d) the `uses recurrence kinds from the closed set` test has `RecurrenceWeekday` in the `allowed` map; (e) the 4 pre-Spec-7 `It` cases are unchanged.
21. Coverage check on the changed packages:
    - `go test -coverprofile=/tmp/cover.schedule.out ./pkg/schedule/...`
    - `go test -coverprofile=/tmp/cover.publisher.out ./pkg/publisher/...`
    - `go test -coverprofile=/tmp/cover.tick.out ./pkg/tick/...`
    - `go test -coverprofile=/tmp/cover.handler.out ./pkg/handler/...`
    - `go tool cover -func=/tmp/cover.schedule.out | tail -1` — total coverage ≥ 80%.
    - `go tool cover -func=/tmp/cover.publisher.out | tail -1` — total coverage ≥ 80%.
    - `go tool cover -func=/tmp/cover.tick.out | tail -1` — total coverage ≥ 80%.
    - `go tool cover -func=/tmp/cover.handler.out | tail -1` — total coverage ≥ 80%.
## Open Questions (for the human reviewer)

- **A. The `sundayWeeklyAllowList` deletion and the 2 invariant tests.** The spec's Failure Modes row 1 ("RecurrenceWeekday with zero Weekday") and row 2 ("RecurrenceWeekly with non-zero Weekday") are tempting to assert directly, but `time.Sunday` is BOTH the zero value of `time.Weekday` AND a real weekday. The 9 migrated Sunday entries legitimately have `Weekday: time.Sunday`, so an invariant test that says "RecurrenceWeekday must not have zero Weekday" would falsely fail. The prompt replaces the 2 invariant tests with 3 count tests (12 / 9 / 0) that cover the spec's AC #2, 3, 4 directly. The behavioral test (Tuesday `TasksForDate` returns zero weekday-kind tasks) in `tasks_for_date_test.go` covers the actual regression. The deletion of `sundayWeeklyAllowList` is now unambiguous — the disambiguation key was needed when `RecurrenceWeekly + Weekday=time.Sunday` was indistinguishable from "non-weekly with zero Weekday"; after the migration, the 9 Sunday entries are `RecurrenceWeekday + Weekday=time.Sunday` and the 0 Weekly entries are exactly that — 0.
- **B. The 2025-01-04 (Saturday) trigger test count.** The exact count of entries that fire on 2025-01-04 is `len(schedule.TasksForDate(schedule.NewDate(2025, time.January, 4)))`, computed at test time. The test does NOT hardcode the count — it derives it from the accessor. The pre-Spec-9 count was 45 (full inventory); the post-Spec-9 count is whatever the accessor returns (somewhere around 33-36, depending on the always-fire calculation). If the count is wrong, the test fails and the regression is caught at build time.
- **C. The "full inventory" tick test's 5 test instants.** The pre-Spec-9 test used 3 instants (Wednesday 2025-01-15 / Friday 2025-07-04 / Sunday 2026-03-01). After this prompt, the tick filters by date, so the test uses 5 instants that exercise the filter (Tuesday / Saturday / Sunday / Friday / Sunday-in-2026). The 5 instants cover: a Tuesday (0 weekday), a Saturday (12 weekday), a Sunday (9 weekday), a Friday (0 weekday), and a Sunday in a different year (9 weekday). The Friday instant is a "control" — no weekday-kind fires, so the test confirms the always-fire baseline (24) is independent of the weekday.
- **D. The `t.inventory` field on the `tick` struct is now dead code at runtime.** The field is set by `NewTick` (the factory passes `schedule.Inventory()`) and never read by `tick` (the function calls `schedule.TasksForDate(date)` directly). The field could be removed in a future refactor — this prompt does not remove it (the change is out of scope and the field's existence is not load-bearing for the fix). The comment in the `tick` function notes the field is unused at runtime; a follow-up spec can remove it.
- **E. The UUID5 stability test's hand-derived expected strings.** The 21 expected input strings in §6.9 are hand-derived from the pre-Spec-9 period-token format. The strings are: `recurring-<slug>-2026W25-sat` (for the 12 Saturday entries on 2026-06-20, a Saturday in ISO 2026W25) and `recurring-<slug>-2026W25-sun` (for the 9 Sunday entries on 2026-06-21, a Sunday in ISO 2026W25). The 2026-06-20/2026-06-21 date pair was chosen because the date's weekday matches the entry's weekday — the test relies on this so the period token's `<abbrev>` is unambiguous. The pre-Spec-9 `RecurrenceWeekly + Weekday=time.Saturday + date=2026-06-20` produced the period token `2026W25-sat`; the post-Spec-9 `RecurrenceWeekday + Weekday=time.Saturday + date=2026-06-20` produces the same period token `2026W25-sat`; the UUID5 input string is byte-identical; the identifier is byte-identical. If any of the 21 expected strings is wrong, the test fails and the deploy is blocked. If the publisher's switch or the inventory migration diverges from the pre-Spec-9 shape, the test fails and the regression is caught at build time.
- **F. The `fmt` import in `pkg/publisher/publisher_test.go`.** The new UUID5 stability test in §6.9 uses `fmt.Sprintf` for the `Entry` names (so each case has a stable name in the test output). The test file's existing imports do NOT include `fmt`; the prompt adds it. The other test additions in §6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7 do NOT need `fmt` (they use string literals).
- **G. The `time` import in `pkg/handler/trigger_test.go`.** The new date-filter behavior tests in §8.2 use `time.January` (and the updated tests in §8.1 use `time.January` too). The test file's existing imports do NOT include `time`; the prompt adds it. The 3 new tests in §8.2 are the only new tests in the file; the 5 updated tests in §8.1 are the only modified tests.
- **H. The `berlin` `*time.Location` variable in `pkg/tick/tick_test.go`.** The new "full inventory" test in §7.1 needs a Berlin location to convert the clock to a civil date. The variable is loaded in a new `BeforeEach` (added at the top of the `Describe("full inventory", ...)` block) via `time.LoadLocation("Europe/Berlin")`. The `time` import is already present. The `BeforeEach` adds 4 lines and a 1-time setup cost.
- **I. The `RecurrenceWeekly` (bare) test fixture in the publisher's "appends '<bare> - <period-token>'" `DescribeTable` (§6.6).** The `weekly` entry's expected token changes from `2026W25-sat` to `2026W25`. The test's def literal still has `Weekday: time.Saturday` (the `DescribeTable` body sets it for every entry), but the new switch arm for `RecurrenceWeekly` ignores the `Weekday` field. The expected token reflects the new behavior. The test does NOT change the `Weekday: time.Saturday` line; the visual diff is just the expected-token string.
- **J. The `RecurrenceWeekday` `Entry` in the publisher's "boundary contract" `DescribeTable` (§6.7).** The new entry needs `Weekday: time.Saturday` on its def literal for `task.CreateCommand.Validate` to succeed (the `CreateCommand`'s Title is built from the def's `TitleTemplate` and the period token; the period token for `RecurrenceWeekday` includes the weekday suffix). The body of the `DescribeTable` is updated to set `Weekday: time.Saturday` only for the `RecurrenceWeekday` kind. The 5 existing entries are unchanged (their kinds do not need a weekday for the `CreateCommand.Validate` call).
- **K. The `RecurrenceWeekday` `Entry` in the publisher's "period-token byte-equality" `DescribeTable` (§6.5).** The new entry uses `Weekday: time.Monday` and a date in ISO 2025W24 (e.g. 2025-06-09, a Monday). The expected token is `2025W24-mon`. The body of the `DescribeTable` is updated to set `Weekday: time.Monday` only for the `RecurrenceWeekday` kind. The 4 existing entries (daily / monthly / quarterly / yearly) are unchanged.
- **L. No scenario file.** The spec's Acceptance Criteria are all reachable from Ginkgo unit tests in `pkg/schedule`, `pkg/publisher`, `pkg/tick`, and `pkg/handler`. No real Kafka, no real vault, no real clock, no real HTTP. No `scenarios/` work is part of this spec or this prompt.

</verification>
