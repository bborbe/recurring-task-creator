---
status: completed
spec: ["011"]
summary: Stripped 8 period placeholders from inventory TitleTemplate values, added periodTitlePlaceholders test var + 2 new validation specs, added BuildPeriodTokenForTest accessor, added full-inventory render test (45 cases), appended changelog entry; make precommit exits 0
container: recurring-task-creator-title-period-exec-016-inventory-cleanup-and-invariant-tests
dark-factory-version: v0.177.1
created: "2026-06-16T07:50:00Z"
queued: "2026-06-16T08:02:12Z"
started: "2026-06-16T08:02:13Z"
completed: "2026-06-16T08:12:02Z"
branch: dark-factory/title-period-tokens-and-drop-recurring-frontmatter
---

<summary>

- The 8 inventory entries whose `TitleTemplate` previously carried a period placeholder (`{{iso-week}}` / `{{next-iso-week}}` / `{{month}}` / `{{last-month}}` / `{{quarter}}` / `{{last-quarter}}` / `{{year}}` / `{{last-year}}`) are stripped to bare titles in `/workspace/pkg/schedule/inventory.go`. After this prompt, no `TitleTemplate` in the inventory contains any of those eight placeholders; the publisher's automatic suffix (added in Prompt 1) replaces them.
- The 8 affected entries are: `weekly-review` (`"Weekly Review {{iso-week}}"` → `"Weekly Review"`), `plan-next-week` (`"Plan Week {{next-iso-week}}"` → `"Plan Week"`), `monthly-review` (`"Review Month {{last-month}}"` → `"Review Month"`), `plan-month` (`"Plan Month {{month}}"` → `"Plan Month"`), `quarter-review` (`"Review Quarter {{last-quarter}}"` → `"Review Quarter"`), `quarter-plan` (`"Plan Quarter {{quarter}}"` → `"Plan Quarter"`), `yearly-review` (`"Review Year {{last-year}}"` → `"Review Year"`), `plan-year` (`"Plan Year {{year}}"` → `"Plan Year"`). The 37 unaffected entries are NOT touched.
- The 8 `BodyTemplate` values that still reference the same placeholders (e.g. `weekly-review`'s body `"... /complete-week - Bot performance...\n2. /weekly-trading-review {{iso-week}} - Portfolio balances"`) are NOT modified — placeholders remain valid in `BodyTemplate` per the spec's Desired Behavior 5 and the DoD's `pkg/schedule` placeholder-support contract.
- A new Ginkgo spec in `pkg/schedule/inventory_validation_test.go` iterates every entry and asserts `strings.TrimSpace(def.TitleTemplate)` does NOT contain any of the eight period placeholders. A second spec asserts `strings.TrimSpace(def.TitleTemplate)` is non-empty for every entry. Both follow the existing `Describe("inventory", ...)` block's style and use `schedule.AllDefinitionsForTest()` as the iterator.
- A new Ginkgo spec in `pkg/publisher/publisher_test.go` iterates every entry in `schedule.Inventory()` with a fixed reference date (2026-06-15) and asserts each rendered title ends with ` - ` + the expected period token for the entry's `(Recurrence, Weekday)` and the reference date. This is the full-inventory cross-check that proves Prompt 1's publisher suffix and Prompt 2's inventory cleanup are mutually consistent — a single test, 45 cases.
- A `feat:` bullet is appended to `CHANGELOG.md` `## Unreleased` describing the inventory cleanup. (The publisher's title-suffix change and the frontmatter drop already have their own bullets from Prompt 1; this bullet covers the data-shape change.)
- `make precommit` exits 0 at the end. The two spec-level `grep` evidence commands from the spec's `## Verification` section both return nothing. The new full-inventory render test exercises every entry end-to-end through the publisher, catching any inventory data shape that would break the publisher's suffix logic.

</summary>

<objective>

Strip the eight period placeholders from the 8 affected inventory `TitleTemplate` values so the inventory's title strings are bare (no `{{...}}` tokens) and the publisher's automatic period-token suffix (added in Prompt 1) takes over. Lock the new inventory invariant down with Ginkgo specs in `pkg/schedule` (no period placeholders in `TitleTemplate`; every `TitleTemplate` non-empty after trim) and add a Ginkgo spec in `pkg/publisher` that iterates the full inventory and asserts every rendered title carries the expected ` - <period-token>` suffix. Append a changelog entry. The build remains green; the publisher and the inventory are now mutually consistent end-to-end.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before making changes:

- `/workspace/pkg/schedule/inventory.go` — the 45-entry inventory slice. The 8 entries that carry period placeholders in `TitleTemplate` are, with their current values (verified by reading the file end-to-end): `weekly-review` (`"Weekly Review {{iso-week}}"`), `plan-next-week` (`"Plan Week {{next-iso-week}}"`), `monthly-review` (`"Review Month {{last-month}}"`), `plan-month` (`"Plan Month {{month}}"`), `quarter-review` (`"Review Quarter {{last-quarter}}"`), `quarter-plan` (`"Plan Quarter {{quarter}}"`), `yearly-review` (`"Review Year {{last-year}}"`), `plan-year` (`"Plan Year {{year}}"`). The 37 other entries have no `{{...}}` in `TitleTemplate`. This prompt strips the placeholders from the 8 affected entries only; the 37 others are not touched.
- `/workspace/pkg/schedule/inventory_validation_test.go` — the existing `Describe("inventory", ...)` block. It currently has six `It` cases: `has unique slugs`, `uses only supported placeholders in TitleTemplate and BodyTemplate`, `uses recurrence kinds from the closed set`, `has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList`, `every weekly entry has Weekday in {time.Saturday, time.Sunday}`, `every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList`. The first three were pre-Spec-7; the last three were added by Prompt 014. The `sundayWeeklyAllowList` package-level var sits at the top of the file. This prompt adds two new `It` cases AFTER the existing six and adds a new `periodTitlePlaceholders` package-level var (similar in style to `sundayWeeklyAllowList`) listing the eight stripped placeholders.
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition` struct. Unchanged; this prompt does not add or remove fields.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` closed enum (5 values) and `AllRecurrenceKinds` slice.
- `/workspace/pkg/schedule/date.go` — `Date` civil-date type with `NewDate(year, month, day)`.
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest()` accessor. Unchanged; this prompt uses it as the iterator in the new specs.
- `/workspace/pkg/schedule/canonical_slugs_test.go` — the 45-slug canonical list. Unchanged; this prompt does not add or remove any slug.
- `/workspace/pkg/schedule/schedule_suite_test.go` — the Ginkgo suite. Unchanged.
- `/workspace/pkg/publisher/publisher.go` — `Publisher.Publish` (as modified by Prompt 1) appends `strings.TrimSpace(renderTemplate(def.TitleTemplate, def.Slug, date)) + " - " + periodToken` to the `Title` field. The new full-inventory render test in `pkg/publisher/publisher_test.go` exercises this path for every entry; the test must use the post-Prompt-1 `Publisher.Publish` shape (i.e. a `Title` ending in ` - <period-token>`). Reading `publisher.go` end-to-end before writing the test confirms the contract.
- `/workspace/pkg/publisher/uuid_namespace.go` — `buildPeriodToken(ctx, recurrence, date, weekday)` returns the period token. The new full-inventory test in `pkg/publisher/publisher_test.go` MUST reuse `buildPeriodToken` (via the `Publisher.Publish` path) to derive the expected suffix for each entry — the test does NOT compute the expected suffix by hand. This guarantees the test asserts the publisher's output matches its own period-token derivation, not a hand-rolled copy.
- `/workspace/pkg/publisher/publisher_test.go` — the existing test file uses Ginkgo v2 / Gomega with dot-imports, external test package `package publisher_test`, counterfeiter `taskmocks.TaskCreateCommandSender` for the sender, and the per-iteration `localSender` / `localPub` pattern (used by the existing `Describe("period anchoring", ...)` block) for tests that observe multiple `Publish` calls. The new full-inventory spec follows the `localSender` / `localPub` pattern because the iteration body calls `Publish` 45 times on independent senders.
- `/workspace/pkg/publisher/export_test.go` — exposes `UuidNamespaceForTest() uuid.UUID`. Unchanged.
- `/workspace/CHANGELOG.md` — append ONE `feat:` bullet to `## Unreleased` describing the inventory cleanup. (The publisher's title-suffix change and the frontmatter drop are described in their own bullets from Prompt 1; this prompt's bullet is a third bullet.)

Coding-guideline references (read inside the YOLO container):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; external test package (`package schedule_test` / `package publisher_test`); `Describe` block with multiple `It` cases; the `sundayWeeklyAllowList` pattern from Spec 7 (named package-level var, length-asserted, used by multiple `It` cases) is the template for the new `periodTitlePlaceholders` var.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — `RecurrenceKind` is a closed string enum; the new full-inventory render test uses the existing constants via `def.Recurrence` (no need to import `schedule` redundantly).
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `- <prefix>: <what> [context]` format; `feat:` for new behavior.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% on changed packages; new behavior has new specs.

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/inventory.go (BEFORE this prompt — exact 8 affected entries)
//
// 1. weekly-review (lines 31-39)
//    TitleTemplate: "Weekly Review {{iso-week}}"
//    BodyTemplate:  "Complete weekly review.\n\n" +
//                   "In Obsidian run (in order):\n\n" +
//                   "1. /complete-week - Bot performance, fills weekly note\n" +
//                   "2. /weekly-trading-review {{iso-week}} - Portfolio balances"
//    -> Strip the placeholder from TitleTemplate; BodyTemplate unchanged.
//
// 2. plan-next-week (lines 96-103)
//    TitleTemplate: "Plan Week {{next-iso-week}}"
//    -> "Plan Week"
//
// 3. monthly-review (lines 253-258)
//    TitleTemplate: "Review Month {{last-month}}"
//    -> "Review Month"
//
// 4. plan-month (lines 261-266)
//    TitleTemplate: "Plan Month {{month}}"
//    -> "Plan Month"
//
// 5. quarter-review (lines 369-374)
//    TitleTemplate: "Review Quarter {{last-quarter}}"
//    -> "Review Quarter"
//
// 6. quarter-plan (lines 377-382)
//    TitleTemplate: "Plan Quarter {{quarter}}"
//    -> "Plan Quarter"
//
// 7. yearly-review (lines 386-391)
//    TitleTemplate: "Review Year {{last-year}}"
//    -> "Review Year"
//
// 8. plan-year (lines 394-399)
//    TitleTemplate: "Plan Year {{year}}"
//    -> "Plan Year"
```

```go
// pkg/publisher/uuid_namespace.go (FROZEN — do not edit)
// The new full-inventory test in pkg/publisher/publisher_test.go MUST reuse
// buildPeriodToken (via Publisher.Publish) to derive the expected suffix for
// each entry. The signature is:
func buildPeriodToken(
    ctx context.Context,
    recurrence schedule.RecurrenceKind,
    date schedule.Date,
    weekday time.Weekday,
) (string, error)
```

```go
// pkg/publisher/publisher.go (AFTER Prompt 1 — the Publish body the new test exercises)
//
// The relevant field on the captured CreateCommand is:
//     Title: strings.TrimSpace(renderTemplate(def.TitleTemplate, def.Slug, date)) + " - " + periodToken
// where periodToken is the return value of buildPeriodToken(ctx, def.Recurrence, date, def.Weekday).
//
// The new full-inventory test asserts:
//     capture().Title ends with " - " + buildPeriodToken(ctx, def.Recurrence, date, def.Weekday)
```

</context>

<requirements>

## 1. Strip the eight period placeholders from the 8 affected `TitleTemplate` values

In `/workspace/pkg/schedule/inventory.go`, change ONLY the `TitleTemplate` field of each of the 8 entries listed below. Do NOT change any other field (Slug, BodyTemplate, Recurrence, Weekday) on any entry. Do NOT change the 37 other entries' `TitleTemplate` values.

The 8 edits:

| # | Slug | Line (file as it stands) | Old `TitleTemplate` | New `TitleTemplate` |
|---|------|---------------------------|---------------------|---------------------|
| 1 | `weekly-review` | 32 | `"Weekly Review {{iso-week}}"` | `"Weekly Review"` |
| 2 | `plan-next-week` | 97 | `"Plan Week {{next-iso-week}}"` | `"Plan Week"` |
| 3 | `monthly-review` | 254 | `"Review Month {{last-month}}"` | `"Review Month"` |
| 4 | `plan-month` | 262 | `"Plan Month {{month}}"` | `"Plan Month"` |
| 5 | `quarter-review` | 370 | `"Review Quarter {{last-quarter}}"` | `"Review Quarter"` |
| 6 | `quarter-plan` | 378 | `"Plan Quarter {{quarter}}"` | `"Plan Quarter"` |
| 7 | `yearly-review` | 387 | `"Review Year {{last-year}}"` | `"Review Year"` |
| 8 | `plan-year` | 395 | `"Plan Year {{year}}"` | `"Plan Year"` |

Notes that are load-bearing for the executor:

- The line numbers above are the lines of the `TitleTemplate:` declaration in the file as it stands at the time of this prompt. The structural edit — find the slug, find the `TitleTemplate:` line on the same struct literal, change only its value — does not depend on the line numbers being exactly right; they are hints. Verify by reading the file end-to-end before editing.
- The `BodyTemplate` for `weekly-review` (lines 33-36) still contains `{{iso-week}}` references in the body text. The body is NOT modified — placeholders remain valid in `BodyTemplate` per the spec's Desired Behavior 5 and the `pkg/schedule` placeholder-support contract. Likewise, the `BodyTemplate` values of the other 7 affected entries may still reference the stripped placeholders (e.g. `monthly-review`'s body still uses `{{last-month}}`); those body references are unchanged.
- The `Slug`, `Recurrence`, `Weekday` fields on all 45 entries are NOT modified. Slugs are FROZEN; the `Weekday` field is FROZEN by Spec 7; the `Recurrence` field is FROZEN by the closed-enum contract.
- The `strings` package is already imported in `pkg/schedule/inventory.go` (it is imported as part of the standard-library block for the `time` import — verify with `grep -n '^import\|"strings"' pkg/schedule/inventory.go`; if it is NOT imported, add it to the import block in goimports-reviser order).
- The file's `Copyright (c) 2026` BSD header is preserved.

## 2. Add the `periodTitlePlaceholders` var and the two new inventory validation specs

In `/workspace/pkg/schedule/inventory_validation_test.go`, add the following content.

First, AFTER the existing `sundayWeeklyAllowList` package-level var (at lines 17-37 of the file as it stands) and BEFORE the existing `var _ = Describe("inventory", func() {` line, add the new `periodTitlePlaceholders` package-level var:

```go
// periodTitlePlaceholders is the exact set of placeholders that spec 008
// stripped from TitleTemplate values. The publisher's automatic
// `<bare> - <period-token>` suffix (added in Prompt 1) replaces them.
// TitleTemplate entries MUST NOT contain any of these placeholders
// (failure mode row 1 of spec 008); BodyTemplate entries MAY still contain
// them per the spec's Desired Behavior 5 and the schedule placeholder-
// support contract. The list is closed: adding a new period-style
// placeholder is a new spec.
var periodTitlePlaceholders = []string{
    "{{iso-week}}",
    "{{next-iso-week}}",
    "{{month}}",
    "{{last-month}}",
    "{{quarter}}",
    "{{last-quarter}}",
    "{{year}}",
    "{{last-year}}",
}
```

Second, INSIDE the existing `Describe("inventory", func() { ... })` block, AFTER the existing `It("every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList", ...)` block (lines 103-118 of the file as it stands; the block ends with the closing `})` of the `It` body) and BEFORE the closing `})` of the outer `Describe("inventory", ...)` block, add the two new `It` cases:

```go
It("has no period placeholders in any TitleTemplate", func() {
    // After spec 008, the eight period-style placeholders (periodTitlePlaceholders)
    // are replaced by the publisher's automatic title-suffix. A TitleTemplate that
    // still contains one of them would render as "Foo 2026W01 - 2026W01-sat" — a
    // double-token shape that no inventory entry intends. The publisher's
    // strings.TrimSpace hides the visible bug at render time, but the data invariant
    // is broken. This spec catches it at build time.
    for _, def := range schedule.AllDefinitionsForTest() {
        trimmed := strings.TrimSpace(def.TitleTemplate)
        for _, ph := range periodTitlePlaceholders {
            Expect(strings.Contains(trimmed, ph)).To(BeFalse(),
                "entry %q TitleTemplate %q still contains period placeholder %q; "+
                    "spec 008 strips these from TitleTemplate (the publisher's suffix replaces them)",
                def.Slug, def.TitleTemplate, ph)
        }
    }
})

It("has a non-empty TitleTemplate for every entry", func() {
    // After spec 008's placeholder stripping, a sloppy edit could empty an
    // entry's TitleTemplate. The publisher's strings.TrimSpace + " - " + suffix
    // logic would render such an entry as just " - 2026-06" — useless to the
    // user. Catch it at build time.
    for _, def := range schedule.AllDefinitionsForTest() {
        Expect(strings.TrimSpace(def.TitleTemplate)).NotTo(BeEmpty(),
            "entry %q has empty TitleTemplate; spec 008 requires a non-empty bare title", def.Slug)
    }
})
```

Notes that are load-bearing for the executor:

- The new `periodTitlePlaceholders` var follows the same style as the existing `sundayWeeklyAllowList` var: package-level (NOT inside the `Describe` block), placed before the `Describe` declaration, with a GoDoc-style comment that names the contract and explains why the list exists. Do NOT add a length assertion for `periodTitlePlaceholders` — unlike `sundayWeeklyAllowList` (where 9 is the structural invariant), the count of 8 is incidental; the contract is "no entry contains any of these". The 8-element shape is fixed by the spec's Desired Behavior 3.
- The new specs use `strings.TrimSpace(def.TitleTemplate)` (mirroring the publisher's render-time trim from Prompt 1) so a TitleTemplate of `" "` is caught as empty by the second spec. This matches the spec's Failure Mode row 3.
- Imports required for the new content:
  - `"strings"` — add to the import block of `pkg/schedule/inventory_validation_test.go`. The file currently imports `"regexp"`, `"time"`, `ginkgo`, `gomega`, and the internal `schedule` package. `strings` slots in alphabetically with the other standard-library imports (before `time` per goimports-reviser order).
  - `schedule.AllDefinitionsForTest()` is already used by the existing specs; no new import.
  - The `periodTitlePlaceholders` and `sundayWeeklyAllowList` vars are package-scoped; no new import.
- The new specs ADD coverage; they do not remove any existing test. The pre-existing six `It` cases continue to pass. The new `It("has no period placeholders in any TitleTemplate", ...)` catches the spec's Failure Mode row 1; the new `It("has a non-empty TitleTemplate for every entry", ...)` catches the spec's Failure Mode row 3.

## 3. Add the full-inventory render spec in `pkg/publisher/publisher_test.go`

In `/workspace/pkg/publisher/publisher_test.go`, add the following content INSIDE the existing `var _ = Describe("Publisher", func() { ... })` block, AFTER the new `Describe("title suffix", func() { ... })` block added by Prompt 1 and BEFORE the existing `Describe("frontmatter", func() { ... })` block. (The exact location is: after the closing `})` of the new `Describe("title suffix", ...)` block and before the `Describe("frontmatter", func() {` line.) The new content is a new `Describe("full-inventory render", func() { ... })` block.

```go
Describe("full-inventory render", func() {
    It("every inventory entry renders to a title ending in ' - <period-token>'", func() {
        // The full-inventory cross-check: prove that Prompt 1's publisher
        // suffix and Prompt 2's inventory cleanup are mutually consistent.
        // For each entry in schedule.Inventory() and the fixed reference
        // date 2026-06-15, the rendered Title must end with " - " followed
        // by the period token buildPeriodToken returns for the same input.
        refDate := schedule.NewDate(2026, time.June, 15)
        for _, def := range schedule.Inventory() {
            // Use a fresh sender per entry so SendCommandArgsForCall(0)
            // always points at the most recent Publish.
            localSender := &taskmocks.TaskCreateCommandSender{}
            localSender.SendCommandReturns(nil)
            localPub := publisher.NewPublisher(localSender, false)
            Expect(localPub.Publish(context.Background(), def, refDate)).To(Succeed())
            _, cmd := localSender.SendCommandArgsForCall(0)
            expectedToken, err := buildPeriodToken(
                context.Background(),
                def.Recurrence,
                refDate,
                def.Weekday,
            )
            Expect(err).NotTo(HaveOccurred(), def.Slug)
            expectedSuffix := " - " + expectedToken
            Expect(cmd.Title).To(HaveSuffix(expectedSuffix),
                "entry %q rendered title %q does not end with %q",
                def.Slug, cmd.Title, expectedSuffix)
        }
    })
})
```

Notes that are load-bearing for the executor:

- The new spec iterates ALL 45 entries in `schedule.Inventory()` (the production accessor) and calls `Publish` for each. For a weekly entry whose `Weekday` field is `time.Saturday`, the period token is `"<iso-week>-sat"`; for a Sunday weekly entry, `"<iso-week>-sun"`; for non-weekly entries, the period token is the bare period shape. The spec's `HaveSuffix(" - <expectedToken>")` assertion catches BOTH halves of the contract: (a) the suffix is present (Prompt 1's render logic ran); (b) the suffix matches the entry's own data (Prompt 2's stripped inventory is in sync with Prompt 1's buildPeriodToken). A failure in either prompt surfaces here.
- The spec calls `buildPeriodToken` directly (not via `buildTaskIdentifier` or `uuid.NewSHA1`) to compute the expected token. The function is in `package publisher`, but the test file is `package publisher_test` (external test package), so the function is NOT directly accessible. There are two ways to expose it:
  - (a) Add a new `BuildPeriodTokenForTest(ctx, recurrence, date, weekday) (string, error)` accessor to `pkg/publisher/export_test.go` (mirroring the existing `UuidNamespaceForTest` pattern). The accessor is the canonical way to call into the package's internal helpers from external tests.
  - (b) Compute the expected token by re-running the publisher path and extracting the suffix from the captured `Title` — but that is circular (the test cannot assert "the title ends with the token derived from the same logic that produced the title" if the only way to derive the token is from the title).

  Use option (a). The new `BuildPeriodTokenForTest` accessor in `pkg/publisher/export_test.go` is:

  ```go
  // BuildPeriodTokenForTest exposes buildPeriodToken to external tests so
  // they can compute the expected title suffix for a (def, date) pair
  // without re-implementing the period-token derivation. The test asserts
  // the publisher's rendered title ends with " - " + the result of this
  // function for the same input — guaranteeing the render and the
  // identifier pipeline use the same period token.
  func BuildPeriodTokenForTest(
      ctx context.Context,
      recurrence schedule.RecurrenceKind,
      date schedule.Date,
      weekday time.Weekday,
  ) (string, error) {
      return buildPeriodToken(ctx, recurrence, date, weekday)
  }
  ```

  Add the `context`, `schedule` imports to `pkg/publisher/export_test.go` (the file currently only imports `"github.com/google/uuid"`).

  After the accessor is added, update the test snippet above to call `publisher.BuildPeriodTokenForTest(...)` instead of `buildPeriodToken(...)`. The snippet above uses `buildPeriodToken` as a shorthand for clarity; the actual call is `publisher.BuildPeriodTokenForTest(context.Background(), def.Recurrence, refDate, def.Weekday)`.

- The spec uses `context.Background()` for the per-entry `Publish` and the per-entry `BuildPeriodTokenForTest` calls. This matches the pattern used by the existing per-iteration tests in `pkg/publisher/publisher_test.go` (lines 322-325, etc.). The test does not exercise cancellation; the `context.Background()` is acceptable per project convention for unit tests.
- The per-iteration `localSender` / `localPub` pattern (from the existing `Describe("period anchoring", ...)` block, lines 70-72) is REQUIRED — 45 entries means 45 `Publish` calls on a single shared `sender` would have call indices 0..44 and the `capture()` helper would only read index 0. Use the local pattern.
- Imports required for the new content in `pkg/publisher/publisher_test.go`:
  - `schedule.Inventory()` is already used by the existing tests; no new import.
  - `publisher.BuildPeriodTokenForTest` (after the export_test.go change) is in the same `publisher` package; no new import.
  - The `localPub.Publish` call uses `context.Background()` which is already imported.
  - `time` is already imported (the existing tests use `time.Saturday`, `time.June`, etc.).
- The new spec ADD coverage; it does not remove any existing test. The 45 cases all pass when the data and the publisher are in sync — that is the contract.

## 4. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (ONE new `feat:` bullet; the title-suffix and frontmatter-drop bullets were added by Prompt 1):

```markdown
- feat: Strip the eight period placeholders (`{{iso-week}}` / `{{next-iso-week}}` / {{month}} / {{last-month}} / {{quarter}} / {{last-quarter}} / {{year}} / {{last-year}}`) from the 8 affected inventory `TitleTemplate` values; the publisher's automatic `<bare> - <period-token>` suffix (added in the previous bullet) replaces them. `BodyTemplate` values that still reference the stripped placeholders are unchanged — placeholders remain valid in `BodyTemplate`
```

Notes that are load-bearing for the executor:

- This is the THIRD `feat:` bullet under `## Unreleased` for spec 008 (after the two added by Prompt 1). Append it to the end of the existing bullets, do not modify the earlier bullets.
- The bullet names the eight placeholders verbatim — this lets `grep -nE 'feat:.*(title suffix|period token)' CHANGELOG.md` (AC #11 first pattern) match the Prompt 1 bullet AND lets a future reviewer `grep` for the placeholders to see which spec stripped them.
- The `BodyTemplate` caveat is included so a future reviewer does not try to strip the placeholders from bodies too.

## 5. Imports and conventions

- The modified `/workspace/pkg/schedule/inventory.go` retains the `time` import and (if it was not already present) gains a `"strings"` import in the import block in goimports-reviser order. Verify with `head -10 pkg/schedule/inventory.go` before saving.
- The modified `/workspace/pkg/schedule/inventory_validation_test.go` adds `"strings"` to its import block. Keep goimports-reviser order: standard library first (alphabetical: `regexp`, `strings`, `time`), then third-party (alphabetical: `ginkgo`, `gomega`), then internal (`schedule`).
- The modified `/workspace/pkg/publisher/publisher_test.go` retains its existing imports. The new content uses `time`, `context`, `schedule.NewDate`, `schedule.Inventory`, `taskmocks.TaskCreateCommandSender`, `publisher.NewPublisher`, `publisher.BuildPeriodTokenForTest` — all from existing imports except the new `BuildPeriodTokenForTest`, which is in the `publisher` package and requires no new import.
- The modified `/workspace/pkg/publisher/export_test.go` adds `context` and the `schedule` package import to its import block. Keep goimports-reviser order: standard library first (`context`), then third-party (`github.com/google/uuid`), then internal (`github.com/bborbe/recurring-task-creator/pkg/schedule`).
- The 2026 copyright header is preserved on all four modified files.
- Use Ginkgo v2 / Gomega style with dot-imports (matches the existing tests).
- Do NOT touch `pkg/handler/`, `pkg/factory/`, `pkg/tick/`, `pkg/mathutil/`, `main.go`, the Makefile, k8s manifests, or the Prometheus metric surface.
- Do NOT add a new Prometheus metric, opt-out flag, runtime config knob, or per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT regenerate any counterfeiter mock. The `Publisher` interface signature is unchanged; the `task.CreateCommandSender` interface is unchanged.
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- Slugs are FROZEN. The 8 entries' slugs do not change in this prompt. The `periodTitlePlaceholders` var is a fixed set of 8 placeholder strings (not slugs).
- The 8 affected `TitleTemplate` values are stripped to bare titles. No other field on the 8 entries is modified. The 37 unaffected entries' `TitleTemplate` values are unchanged.
- The 8 entries' `BodyTemplate` values are unchanged. The `BodyTemplate` may still reference the stripped placeholders (e.g. `weekly-review`'s body still uses `{{iso-week}}` in the `/weekly-trading-review {{iso-week}}` reference). This is per the spec's Desired Behavior 5 and the `pkg/schedule` placeholder-support contract.
- The `Weekday` field on all 45 entries is FROZEN by Spec 7. This prompt does not touch it.
- The `RecurrenceKind` enum stays a closed set of exactly 5 values. The new full-inventory render spec uses the existing constants via `def.Recurrence`.
- The `uuidNamespace` constant in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN byte-identical — do NOT touch it.
- The `recurring-<slug>-<period-token>` UUID5 input string format is FROZEN — do NOT touch it.
- The `Publisher` interface signature is FROZEN — do NOT touch it.
- The `buildPeriodToken` function signature in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN — do NOT touch it. The new `BuildPeriodTokenForTest` accessor in `pkg/publisher/export_test.go` is a thin pass-through that exposes the FROZEN function to external tests.
- The `periodTitlePlaceholders` var lives in the TEST package only (`package schedule_test`), not in the production `schedule` package. It is a test-only constant. It is declared in `pkg/schedule/inventory_validation_test.go` and is referenced by the same file's specs.
- Existing tests must still pass after all edits. The new specs ADD to coverage; they do not remove any existing test. The pre-existing six `It` cases in `inventory_validation_test.go` continue to pass; the pre-existing publisher tests continue to pass.
- Coverage on the changed packages stays at or above 80%. The two new `It` cases in `pkg/schedule` and the one new `It` case in `pkg/publisher` (with 45 inner iterations) add significant coverage to both packages; the packages' coverage should stay well above 80%.
- Project DoD (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` for error wrapping (the new `BuildPeriodTokenForTest` accessor returns the function's error directly — no wrapping needed in the test accessor); no `context.Background()` in business logic (the new test code uses `context.Background()` in test code, which is acceptable per project convention; the production `buildPeriodToken` is unchanged and uses its `ctx` parameter); no `time.Time` / `time.Now()` in business logic (no new business logic in this prompt); GoDoc on the new `BuildPeriodTokenForTest` accessor (existing GoDoc-style comment in the snippet above); `make precommit` clean.
- Do NOT commit — dark-factory handles git.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/schedule/...` — all Ginkgo specs green. In particular:
   - `It("has no period placeholders in any TitleTemplate", ...)` passes for all 45 entries.
   - `It("has a non-empty TitleTemplate for every entry", ...)` passes for all 45 entries.
   - The pre-existing six `It` cases (`has unique slugs`, `uses only supported placeholders in TitleTemplate and BodyTemplate`, `uses recurrence kinds from the closed set`, `has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList`, `every weekly entry has Weekday in {time.Saturday, time.Sunday}`, `every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList`) continue to pass.
3. `go test ./pkg/publisher/...` — all Ginkgo specs green. In particular:
   - `It("every inventory entry renders to a title ending in ' - <period-token>'", ...)` passes for all 45 entries.
   - The pre-existing publisher specs (identifier byte-equality, period anchoring, placeholder rendering, the new Prompt-1 title-suffix specs, the new frontmatter specs, sender interaction, boundary contract, package surface) continue to pass.
4. `grep -nE '\{\{(iso-week|next-iso-week|month|last-month|quarter|last-quarter|year|last-year)\}\}' pkg/schedule/inventory.go | grep TitleTemplate` — must return no matches (AC #6). Expected: exit code 1, empty output.
5. `grep -nE '\{\{(iso-week|next-iso-week|month|last-month|quarter|last-quarter|year|last-year)\}\}' pkg/schedule/inventory.go | grep BodyTemplate` — must return matches for the bodies that legitimately still reference the placeholders. The bodies of `weekly-review`, `plan-next-week`, `monthly-review`, `plan-month`, `quarter-review`, `quarter-plan`, `yearly-review`, `plan-year` all retain the placeholders. This is expected per the spec's Desired Behavior 5.
6. `grep -nE 'periodTitlePlaceholders' pkg/schedule/inventory_validation_test.go` — must return at least 3 matches: the var declaration + the 2 spec references (`strings.Contains(trimmed, ph)` × 8, plus the loop var). The expected value is 3 (one declaration + one spec use for the loop over `periodTitlePlaceholders` in the "no period placeholders" test).
7. `grep -nE 'periodTitlePlaceholders' pkg/schedule/inventory.go` — must return no matches (the allow-list is a test-only artifact; it must not leak into production code).
8. `grep -nE 'BuildPeriodTokenForTest' pkg/publisher/export_test.go` — must return at least 1 match (the new accessor). The new full-inventory test in `pkg/publisher/publisher_test.go` calls `publisher.BuildPeriodTokenForTest(...)` — verify with `grep -nE 'BuildPeriodTokenForTest' pkg/publisher/publisher_test.go` returning at least 1 match.
9. `grep -nE 'recurring-<slug>-<period-token>|recurring-" \+ slug' pkg/publisher/uuid_namespace.go` — must return at least 1 match. The identifier input string format is byte-identical (Prompt 1's contract, preserved by this prompt).
10. `grep -nE 'Weekly Review|Plan Week|Review Month|Plan Month|Review Quarter|Plan Quarter|Review Year|Plan Year' pkg/schedule/inventory.go` — must return exactly 8 matches (one per stripped entry, each match being the `TitleTemplate:` line). The 8 new bare titles are: `Weekly Review`, `Plan Week`, `Review Month`, `Plan Month`, `Review Quarter`, `Plan Quarter`, `Review Year`, `Plan Year`.
11. Spot-check: open `pkg/schedule/inventory.go` and visually confirm (a) the 8 `TitleTemplate:` lines have the new bare values; (b) the `Slug`, `BodyTemplate`, `Recurrence`, `Weekday` fields on the 8 entries are UNCHANGED; (c) the 37 unaffected entries are completely unchanged.
12. Spot-check: open `pkg/schedule/inventory_validation_test.go` and visually confirm (a) the new `periodTitlePlaceholders` var sits at package level, after the `sundayWeeklyAllowList` var and before the `Describe` block; (b) the two new `It` cases are inside the existing `Describe("inventory", ...)` block, after the `sundayWeeklyAllowList`-related `It` cases; (c) the existing six `It` cases are unchanged; (d) the `"strings"` import is added.
13. Spot-check: open `pkg/publisher/publisher_test.go` and visually confirm (a) the new `Describe("full-inventory render", ...)` block is between the new `Describe("title suffix", ...)` block and the existing `Describe("frontmatter", ...)` block; (b) the new `It` uses `schedule.Inventory()` (not `AllDefinitionsForTest()`) and a fixed `refDate := schedule.NewDate(2026, time.June, 15)`; (c) the per-iteration `localSender` / `localPub` pattern is in use.
14. Spot-check: open `pkg/publisher/export_test.go` and visually confirm (a) the new `BuildPeriodTokenForTest` accessor exists; (b) it is a thin pass-through to `buildPeriodToken`; (c) the `context` and `schedule` imports are added.
15. Coverage check on the changed packages:
    - `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/schedule/...`
    - `go test -coverprofile=/tmp/cover.publisher.out -mod=vendor ./pkg/publisher/...`
    - `go tool cover -func=/tmp/cover.out | tail -1` — total coverage ≥ 80%.
    - `go tool cover -func=/tmp/cover.publisher.out | tail -1` — total coverage ≥ 80%.
16. Changelog:
    - `grep -nE 'feat:.*period placeholders' CHANGELOG.md` — must return at least 1 line (this prompt's new bullet).
    - The two earlier `feat:` bullets from Prompt 1 (`feat: Publisher renders every task title as...` and `feat: Drop the `recurring: <kind>` key...`) are still present (verify with `grep -nE 'feat:.*title suffix|feat:.*period token' CHANGELOG.md` returning at least 1 line AND `grep -nE 'feat:.*recurring' CHANGELOG.md` returning at least 1 line).
17. End-to-end smoke: with the inventory cleanup in this prompt, every inventory entry's `TitleTemplate` is a bare string and every rendered title ends with ` - <period-token>`. The full-inventory test in `pkg/publisher/publisher_test.go` exercises the full chain end-to-end. Run `go test ./...` to confirm the full test suite is green.

## Open Questions (for the human reviewer)

- **A. The `BuildPeriodTokenForTest` accessor location.** The new test accessor in `pkg/publisher/export_test.go` follows the existing `UuidNamespaceForTest` pattern. The alternative — making `buildPeriodToken` exported (rename to `BuildPeriodToken`) — would leak the production helper into the public surface. The `_test.go` export pattern keeps the production API clean and matches the existing convention. The accessor is purely a test seam.
- **B. The `periodTitlePlaceholders` var declaration order.** The var is declared after `sundayWeeklyAllowList` (both at package level, both before the `Describe` block). The order is incidental — Ginkgo does not care about declaration order of package-level vars. A future reviewer may sort the vars alphabetically for readability; that is a no-op for the tests.
- **C. The 8 `BodyTemplate` values that still reference the stripped placeholders.** The bodies of `weekly-review`, `plan-next-week`, `monthly-review`, `plan-month`, `quarter-review`, `quarter-plan`, `yearly-review`, `plan-year` all retain the placeholders (e.g. `weekly-review`'s body still has `"/weekly-trading-review {{iso-week}} - Portfolio balances"`). This is per the spec's Desired Behavior 5. The `uses only supported placeholders in TitleTemplate and BodyTemplate` validation spec (pre-existing) continues to pass because the placeholders are still in `schedule.SupportedPlaceholders`. No further cleanup is in scope for this spec.
- **D. The full-inventory test's per-entry error message.** The test uses `HaveSuffix(expectedSuffix, ...)` with a custom failure message `"entry %q rendered title %q does not end with %q"`. When this test fails, the failure message names the specific entry and the specific mismatch — critical for debugging when the inventory or the publisher diverges. The test runs 45 cases; without a per-entry failure message, a single failure would print a generic `HaveSuffix` mismatch and the executor would have to add logging to identify the entry.
- **E. The `BuildPeriodTokenForTest` call uses `context.Background()`.** Per project convention, `context.Background()` is acceptable in test code (the production `buildPeriodToken` uses its `ctx` parameter; the test accessor passes through whatever ctx the caller provides, and the test caller uses `Background`). The test does not exercise cancellation.
- **F. No scenario file.** The spec's Acceptance Criteria are all reachable from Ginkgo unit tests in the schedule / publisher packages. No real Kafka, no real vault, no real clock. No `scenarios/` work is part of this spec or this prompt.
- **G. The `import "strings"` placement in `pkg/schedule/inventory.go`.** The file's existing imports are `"time"`. The new code in this prompt does NOT add any `strings.X` call to `inventory.go` (the inventory has no Go code calling `strings.TrimSpace`; that happens at render time in the publisher). The `strings` import is therefore NOT needed in `inventory.go` itself. The note in §1 of `<requirements>` ("The `strings` package is already imported... if it is NOT imported, add it") is a guardrail — verify before adding. If the file's existing imports do not include `"strings"`, do NOT add it: the inventory is data, not code.

</verification>
