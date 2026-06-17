---
status: completed
spec: [008-title-period-tokens-and-drop-recurring-frontmatter]
summary: Publisher now appends ' - <period-token>' to every Title and emits a six-key frontmatter (no `recurring`); new specs lock the new shape; existing placeholder-rendering tests updated to include the suffix; `make precommit` exits 0 with pkg/publisher coverage 90.0%.
container: recurring-task-creator-title-period-exec-015-publisher-title-suffix-and-drop-recurring
dark-factory-version: v0.177.1
created: "2026-06-16T07:50:00Z"
queued: "2026-06-16T07:54:20Z"
started: "2026-06-16T07:54:20Z"
completed: "2026-06-16T08:02:12Z"
branch: dark-factory/title-period-tokens-and-drop-recurring-frontmatter
---

<summary>

- Every rendered `Title` field on a `task.CreateCommand` is now `<bare-title-after-placeholder-substitution> - <period-token>` where the period token is the same string `buildPeriodToken` returns for the entry's `(Recurrence, date, Weekday)` tuple — for example `Update K3s - 2026-06`, `Shutdown K3s - 2026W25-sat`, `Plan Year - 2026`. The suffix is computed in `Publisher.Publish`, NOT in the inventory.
- The separator is the three-character string ` - ` (space, hyphen, space). The publisher `strings.TrimSpace`'s the rendered title BEFORE appending the separator and suffix — so an inventory `TitleTemplate` whose placeholder has just been stripped (Prompt 2) cannot leak a stray space into the rendered output, and the 37 unaffected bare-title entries are unaffected.
- The publisher REUSES the existing `buildPeriodToken` helper from `pkg/publisher/uuid_namespace.go` — it does NOT re-derive the token. The frozen `uuidNamespace`, the `recurring-<slug>-<period-token>` UUID5 input string, and the `buildPeriodToken` switch arms are all unchanged; only the rendered title carries the new shape.
- The materialized frontmatter no longer carries a `recurring:` field. The output `lib.TaskFrontmatter` has exactly six keys: `assignee`, `status`, `page_type`, `goals`, `priority`, `created_by`. The `buildFrontmatter` function's signature changes from `buildFrontmatter(recurrence schedule.RecurrenceKind)` to `buildFrontmatter()` (no parameters) because `recurring` was its only consumer of the parameter; the sole caller (`Publisher.Publish` in `pkg/publisher/publisher.go`) is updated in lock-step.
- The `Publisher` interface signature is unchanged. The `task.CreateCommandSender` contract is unchanged. The `Publisher.Publish` method's external error path (slug/date validation, buildTaskIdentifier, sender wrap) is unchanged. The new title shape and the dropped frontmatter key are the only behavioral deltas.
- `pkg/publisher/publisher_test.go` gains: (a) an absolute-shape test asserting `Update K3s - 2026-06` for `RecurrenceMonthly` + 2026-06-15; (b) a weekly-suffix test asserting `Shutdown K3s - 2026W25-sat` for `RecurrenceWeekly` + `Weekday=Saturday` + 2026-06-17; (c) a `DescribeTable` covering all five `RecurrenceKind` values asserting the rendered title ends with ` - ` + `buildPeriodToken`'s output for the same input; (d) a frontmatter shape test asserting `HaveLen(6)` AND `Not(HaveKey("recurring"))`; (e) the existing `recurring matches RecurrenceKind` table-driven test is removed (the field no longer exists).
- `make precommit` exits 0 at the end. `grep -nE '"recurring"' pkg/publisher/frontmatter.go` returns nothing. All existing publisher and schedule tests continue to pass; this prompt only changes publisher-side shape + tests, NOT the inventory data (Prompt 2's job).

</summary>

<objective>

Extend the publisher's render path to append a period-token suffix (derived from the same `buildPeriodToken` helper that anchors the UUID5 identifier) to every rendered title, and drop the `recurring` key from the frontmatter map — both in `pkg/publisher/frontmatter.go` and at the call site in `pkg/publisher/publisher.go`. Lock the new shape down with `pkg/publisher/publisher_test.go` specs that exercise the suffix for representative cases per kind and assert the six-key frontmatter shape with explicit absence of `recurring`. The build remains green; Prompt 2 then strips the eight `{{...}}` placeholders from the inventory `TitleTemplate` values and adds invariant tests that prove prompts 1 and 2 are mutually consistent.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before making changes:

- `/workspace/pkg/publisher/publisher.go` — the `Publisher` interface, the `publisher` struct, and the `Publish` method. The `cmd.CreateCommand` literal at lines 68-73 is the only place that constructs a command from a render path. `Title` is set by `renderTemplate(def.TitleTemplate, def.Slug, date)` (line 70) and `Frontmatter` is set by `buildFrontmatter(def.Recurrence)` (line 71). This prompt changes BOTH of those values and changes the call site of `buildFrontmatter` to drop the now-unused argument.
- `/workspace/pkg/publisher/render.go` — `renderTemplate` is unchanged. The function still produces the placeholder-substituted string. The new title-suffix logic lives in `Publisher.Publish`, not in `renderTemplate`, so the body-template and title-template render paths remain symmetrical.
- `/workspace/pkg/publisher/uuid_namespace.go` — `buildPeriodToken(ctx, recurrence, date, weekday)` returns the period token. FROZEN — do not edit the switch, the `weekdayAbbrev` helper, or the `uuidNamespace` constant. The title-suffix logic REUSES this helper verbatim.
- `/workspace/pkg/publisher/frontmatter.go` — `buildFrontmatter(recurrence schedule.RecurrenceKind) lib.TaskFrontmatter` is the entire file body. After this prompt, the function takes no arguments and emits exactly six keys; the `"recurring": string(recurrence)` line is removed.
- `/workspace/pkg/publisher/publisher_test.go` — the existing test file uses Ginkgo v2 / Gomega with dot-imports, external test package `package publisher_test`, counterfeiter `taskmocks.TaskCreateCommandSender` for the sender, and `publisher.UuidNamespaceForTest()` (from `pkg/publisher/export_test.go`) for the namespace accessor. New specs follow the existing patterns: the `capture()` helper at the suite's top captures the last `SendCommand` call's `CreateCommand` value; the `localSender` / `localPub` pattern at lines 70-72 is used when the test must observe multiple `Publish` calls on independent senders.
- `/workspace/pkg/publisher/export_test.go` — exposes `UuidNamespaceForTest() uuid.UUID`. Unchanged.
- `/workspace/pkg/publisher/no_forbidden_imports_test.go` — guards the package against `net/http`, `kafka-go`, `sarama`, `jira-task-creator`, and `time.Now()`. The new code in this prompt does not add any of those; no impact.
- `/workspace/pkg/schedule/inventory.go` — read this for the `TitleTemplate` values that Prompt 2 will strip. The 8 entries that currently carry period placeholders in `TitleTemplate` are: `weekly-review` (`"Weekly Review {{iso-week}}"`), `plan-next-week` (`"Plan Week {{next-iso-week}}"`), `monthly-review` (`"Review Month {{last-month}}"`), `plan-month` (`"Plan Month {{month}}"`), `quarter-review` (`"Review Quarter {{last-quarter}}"`), `quarter-plan` (`"Plan Quarter {{quarter}}"`), `yearly-review` (`"Review Year {{last-year}}"`), `plan-year` (`"Plan Year {{year}}"`). For this prompt, the inventory is NOT modified; the new suffix logic must work for both the bare-title entries (37 unaffected) and the placeholder-carrying entries (8 affected, until Prompt 2 strips the placeholders).
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition` struct. Unchanged; this prompt does not add or remove fields.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` closed enum (5 values) and `AllRecurrenceKinds` slice.
- `/workspace/pkg/schedule/date.go` — `Date` civil-date type with `NewDate(year, month, day)`.
- `/workspace/CHANGELOG.md` — append `feat:` bullet(s) to `## Unreleased`. Per the spec's AC #11 the title-suffix change and the frontmatter drop are described in either one or two bullets; this prompt writes two separate bullets (one for the title-suffix, one for the frontmatter drop) for clarity.

Coding-guideline references (read inside the YOLO container):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` API; never `fmt.Errorf` in `pkg/`; `errors.Wrapf(ctx, err, "msg")` style with key=value args.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; external test package (`package publisher_test`); `DescribeTable` for parameterized coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public interface + private struct + `New*` constructor; counterfeiter annotations on the `Publisher` interface (already present, unchanged).
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `- <prefix>: <what> [context]` format; `feat:` for new feature behavior.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% on changed packages; new behavior has new specs.

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/publisher/uuid_namespace.go (FROZEN — do not edit)
// buildPeriodToken returns the period-anchored token for the given
// (recurrence, date) pair. The token is the same string the corresponding
// title-rendering formatter produces — "YYYY-MM-DD" for daily,
// "YYYYWNN-<3-letter-lowercase-weekday>" for weekly (the suffix is taken
// from the entry's Weekday field, NOT the date's weekday), "YYYY-MM" for
// monthly, "YYYYQN" for quarterly, "YYYY" for yearly.
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

// pkg/publisher/uuid_namespace.go (FROZEN)
var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")

// pkg/publisher/uuid_namespace.go (FROZEN)
// buildTaskIdentifier input string format: "recurring-" + slug + "-" + token
// The publisher's new title-suffix logic MUST NOT change this format.
```

```go
// pkg/publisher/publisher.go (BEFORE this prompt — exact source at the time of writing)
func (p *publisher) Publish(
    ctx context.Context,
    def schedule.TaskDefinition,
    date schedule.Date,
) error {
    if def.Slug == "" {
        return errors.Errorf(ctx, "publish failed: empty slug")
    }
    if date.IsZero() {
        return errors.Errorf(ctx, "publish failed: zero date for slug %q", def.Slug)
    }
    token, err := buildTaskIdentifier(ctx, def.Slug, def.Recurrence, date, def.Weekday)
    if err != nil {
        return errors.Wrapf(
            ctx,
            err,
            "publish failed: build identifier for slug %q",
            def.Slug,
        )
    }
    cmd := task.CreateCommand{
        TaskIdentifier: token,
        Title:          renderTemplate(def.TitleTemplate, def.Slug, date),
        Frontmatter:    buildFrontmatter(def.Recurrence), // <-- the call that changes
        Body:           renderTemplate(def.BodyTemplate, def.Slug, date),
    }
    if p.dryRun { /* unchanged */ }
    if err := p.sender.SendCommand(ctx, cmd); err != nil { /* unchanged */ }
    return nil
}
```

```go
// pkg/publisher/frontmatter.go (BEFORE this prompt — exact source at the time of writing)
package publisher

import (
    lib "github.com/bborbe/agent/lib"

    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// buildFrontmatter returns the exact frontmatter shape for every recurring
// task. The shape is FROZEN: changing any of these keys or values is a
// breaking change to the migration's vault-file layout.
func buildFrontmatter(recurrence schedule.RecurrenceKind) lib.TaskFrontmatter {
    return lib.TaskFrontmatter{
        "assignee":   "bborbe",
        "status":     "in_progress",
        "page_type":  "task",
        "goals":      []interface{}{goalsLink},
        "priority":   2,
        "recurring":  string(recurrence), // <-- the line that is removed
        "created_by": "recurring-task-creator",
    }
}
```

</context>

<requirements>

## 1. Update `Publisher.Publish` to append the period-token suffix to the title and to drop the `recurring` frontmatter argument

In `/workspace/pkg/publisher/publisher.go`, change the `Publish` method's `cmd` construction (the `task.CreateCommand{...}` literal at lines 68-73 of the file as it stands) so that:

- The `Title` field is `<strings.TrimSpace(renderTemplate(def.TitleTemplate, def.Slug, date))> - <period-token>`, where `<period-token>` is the string returned by `buildPeriodToken` for the entry's `(def.Recurrence, date, def.Weekday)` tuple. The token is the SAME string the UUID5 identifier is derived from; do NOT re-derive it.
- The `Frontmatter` field is the return value of `buildFrontmatter()` — the new no-arg `buildFrontmatter` (see §2).

The exact shape of the new `cmd` literal is:

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
periodToken, err := buildPeriodToken(ctx, def.Recurrence, date, def.Weekday)
if err != nil {
    return errors.Wrapf(
        ctx,
        err,
        "publish failed: build period token for slug %q",
        def.Slug,
    )
}
cmd := task.CreateCommand{
    TaskIdentifier: token,
    Title:          strings.TrimSpace(renderTemplate(def.TitleTemplate, def.Slug, date)) + " - " + periodToken,
    Frontmatter:    buildFrontmatter(),
    Body:           renderTemplate(def.BodyTemplate, def.Slug, date),
}
```

Notes that are load-bearing for the executor:

- The existing `buildTaskIdentifier` call ALREADY calls `buildPeriodToken` internally (see `pkg/publisher/uuid_namespace.go` lines 92-98). Re-calling `buildPeriodToken` here is a deliberate, single-source-of-truth read of the period token; the alternative — threading the token out of `buildTaskIdentifier` — would change the latter's signature. The spec's Non-goal "Do NOT change the period-token derivation" is satisfied because the second call reuses the same function; the token is byte-identical to what `buildTaskIdentifier` saw.
- Both error returns are wrapped with the slug in the wrap message (matches the existing wrap idiom at line 64). Use `bborbe/errors` `errors.Wrapf(ctx, err, "publish failed: build period token for slug %q", def.Slug)` — never `fmt.Errorf`, never `context.Background()`.
- The `strings.TrimSpace` is applied to the rendered title. The rendered title is `renderTemplate(def.TitleTemplate, def.Slug, date)`. For the 37 inventory entries whose `TitleTemplate` is already a bare string, `renderTemplate` returns the bare string (no placeholders to substitute) and `TrimSpace` is a no-op. For the 8 entries that still carry placeholders (until Prompt 2 strips them), `renderTemplate` substitutes the placeholder — `TrimSpace` ensures no leading/trailing space from a stripped placeholder leaks into the rendered output. Either way the title is well-formed.
- The `cmd` literal's other fields (`TaskIdentifier`, `Body`) are unchanged. The `dryRun` branch below the literal and the sender `SendCommand` call below that are unchanged. The early `def.Slug == ""` and `date.IsZero()` guards above the literal are unchanged.
- Add `"strings"` to the `import` block of `pkg/publisher/publisher.go` (the file does not currently import `strings`). Keep the import block goimports-reviser-ordered: standard library first, then third-party, then internal.
- The function's signature is unchanged: `func (p *publisher) Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error`. The `Publisher` interface is unchanged.

## 2. Update `buildFrontmatter` to take no arguments and emit exactly six keys

In `/workspace/pkg/publisher/frontmatter.go`, change the file to:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
    lib "github.com/bborbe/agent/lib"
)

// buildFrontmatter returns the exact frontmatter shape for every task
// published by this service. The shape is FROZEN: changing any of these
// keys or values is a breaking change to the migration's vault-file
// layout. The shape was reduced from seven keys to six by spec 008:
// the `recurring` key is gone — downstream vault tooling treats every
// published task as a normal one-shot task, regardless of cadence.
func buildFrontmatter() lib.TaskFrontmatter {
    return lib.TaskFrontmatter{
        "assignee":   "bborbe",
        "status":     "in_progress",
        "page_type":  "task",
        "goals":      []interface{}{goalsLink},
        "priority":   2,
        "created_by": "recurring-task-creator",
    }
}
```

Notes that are load-bearing for the executor:

- The function signature changes from `buildFrontmatter(recurrence schedule.RecurrenceKind) lib.TaskFrontmatter` to `buildFrontmatter() lib.TaskFrontmatter`. Do NOT keep a `recurrence schedule.RecurrenceKind` parameter "for future use" — the spec's Non-goal "Do NOT add a per-entry opt-out flag for the suffix or the frontmatter drop" forbids the kind from being reintroduced. After this prompt, `def.Recurrence` is consulted by the title-suffix logic (§1) and by `buildTaskIdentifier` / `buildPeriodToken` (FROZEN, unchanged); it is no longer needed by `buildFrontmatter`.
- The `import "github.com/bborbe/recurring-task-creator/pkg/schedule"` line is REMOVED — `schedule.RecurrenceKind` is no longer used in this file. Keep the `lib "github.com/bborbe/agent/lib"` import.
- The GoDoc comment block is updated to mention the six-key shape and the rationale (downstream vault tooling treats every task as one-shot). Do not move or rename the function; do not change the package.
- The `goalsLink` constant (defined in `pkg/publisher/publisher.go` line 20) is referenced by the new `buildFrontmatter`. It is in the same package, so no qualifier is needed. The constant is unchanged.
- The file's `Copyright (c) 2026` BSD header is preserved.

## 3. Add the new title-suffix specs in `pkg/publisher/publisher_test.go`

Add the following content INSIDE the existing `var _ = Describe("Publisher", func() { ... })` block, after the closing `})` of the existing `Describe("placeholder rendering", func() { ... })` block and before the opening of the existing `Describe("frontmatter", func() { ... })` block. (The exact location is: between the line `})` that closes the placeholder-rendering `Describe` and the `Describe("frontmatter", func() {` line.) The new content is a new `Describe("title suffix", func() { ... })` block.

```go
Describe("title suffix", func() {
    It("appends the period token to a monthly title", func() {
        def := schedule.TaskDefinition{
            Slug:          "update-k3s",
            TitleTemplate: "Update K3s",
            Recurrence:    schedule.RecurrenceMonthly,
        }
        Expect(pub.Publish(
            context.Background(),
            def,
            schedule.NewDate(2026, time.June, 15),
        )).To(Succeed())
        Expect(capture().Title).To(Equal("Update K3s - 2026-06"))
    })

    It("appends the period token to a weekly title (with weekday suffix)", func() {
        def := schedule.TaskDefinition{
            Slug:          "shutdown-k3s",
            TitleTemplate: "Shutdown K3s",
            Recurrence:    schedule.RecurrenceWeekly,
            Weekday:       time.Saturday,
        }
        // 2026-06-17 is a Wednesday in ISO 2026W25.
        Expect(pub.Publish(
            context.Background(),
            def,
            schedule.NewDate(2026, time.June, 17),
        )).To(Succeed())
        Expect(capture().Title).To(Equal("Shutdown K3s - 2026W25-sat"))
    })

    It("trims whitespace from the rendered title before appending the suffix", func() {
        def := schedule.TaskDefinition{
            Slug:          "trailing-space",
            TitleTemplate: "Trailing Space ", // bare string, no placeholders
            Recurrence:    schedule.RecurrenceMonthly,
        }
        Expect(pub.Publish(
            context.Background(),
            def,
            schedule.NewDate(2026, time.June, 15),
        )).To(Succeed())
        Expect(capture().Title).To(Equal("Trailing Space - 2026-06"))
    })

    It("renders bare placeholders-only templates as '<token>' after substitution and suffix", func() {
        // For a TitleTemplate that is JUST a placeholder (e.g. "{{month}}"),
        // the rendered output is the bare token, then a separator, then the
        // suffix token. The two tokens must not collapse to a single value.
        def := schedule.TaskDefinition{
            Slug:          "placeholder-only",
            TitleTemplate: "{{month}}",
            Recurrence:    schedule.RecurrenceMonthly,
        }
        Expect(pub.Publish(
            context.Background(),
            def,
            schedule.NewDate(2026, time.June, 15),
        )).To(Succeed())
        Expect(capture().Title).To(Equal("2026-06 - 2026-06"))
    })

    DescribeTable(
        "appends '<bare> - <period-token>' for every RecurrenceKind",
        func(rec schedule.RecurrenceKind, date schedule.Date, expectedToken string) {
            // Use a fresh sender per Entry so SendCommandArgsForCall(0)
            // always points at the most recent Publish.
            localSender := &taskmocks.TaskCreateCommandSender{}
            localSender.SendCommandReturns(nil)
            localPub := publisher.NewPublisher(localSender, false)
            def := schedule.TaskDefinition{
                Slug:          "kind-" + string(rec),
                TitleTemplate: "Bare",
                Recurrence:    rec,
                Weekday:       time.Saturday, // ignored for non-weekly kinds
            }
            Expect(localPub.Publish(context.Background(), def, date)).To(Succeed())
            _, cmd := localSender.SendCommandArgsForCall(0)
            Expect(cmd.Title).To(Equal("Bare - " + expectedToken))
        },
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
            schedule.NewDate(2026, time.April, 1), // April = Q2
            "2026Q2",
        ),
        Entry(
            "yearly",
            schedule.RecurrenceYearly,
            schedule.NewDate(2026, time.January, 1),
            "2026",
        ),
    )
})
```

Notes that are load-bearing for the executor:

- The new `It` tests at the top of the `Describe("title suffix", ...)` block use the parent suite's `pub` and `capture()` helper (defined at lines 28-37 of the existing test file). The `DescribeTable` uses the per-iteration `localSender` / `localPub` pattern from the existing `Describe("period anchoring", ...)` block (lines 70-72) — required because Ginkgo's `DescribeTable` runs all entries in the same `BeforeEach`, so a single shared `sender` would have multiple call indices to read. This is a pre-existing pattern; mirror it.
- The new specs do not assert on the UUID5 identifier or the frontmatter; those are covered by the existing identifier / frontmatter specs and the new frontmatter-shape spec (§4). The new specs are tight: they assert ONLY the title shape, which is the new contract.
- The `"trims whitespace from the rendered title before appending the suffix"` test exercises the `strings.TrimSpace` step explicitly. It uses a bare-string `TitleTemplate` (no placeholders) with a trailing space — so the only thing under test is the publisher's own trim, not the placeholder-stripping that Prompt 2 will add.
- The `"renders bare placeholders-only templates..."` test exercises the edge case where the rendered title is just the placeholder value (e.g. `"{{month}}"` renders to `"2026-06"`) and the suffix is the same value (e.g. `"2026-06"`). The result is `"2026-06 - 2026-06"` — a string that LOOKS like a duplicate but is correct. This is a degenerate case; no inventory entry uses it, but the test pins the behavior down so a future refactor cannot accidentally collapse the two halves.

## 4. Replace the existing frontmatter test block with the new six-key shape test

The existing `Describe("frontmatter", func() { ... })` block in `/workspace/pkg/publisher/publisher_test.go` (lines 472-518 of the file as it stands) contains:

- An `It("has the full frozen shape", ...)` that asserts seven keys including `"recurring"`. After this prompt, `recurring` is gone; the test is replaced.
- A `DescribeTable("recurring matches RecurrenceKind", ...)` that asserts `Frontmatter["recurring"]` is the kind. After this prompt, `recurring` is gone; the test is deleted.

Replace the entire `Describe("frontmatter", func() { ... })` block with:

```go
Describe("frontmatter", func() {
    It("has the six-key shape (assignee, status, page_type, goals, priority, created_by)", func() {
        def := schedule.TaskDefinition{
            Slug:          "test-slug",
            TitleTemplate: "t",
            Recurrence:    schedule.RecurrenceWeekly,
        }
        Expect(pub.Publish(
            context.Background(),
            def,
            schedule.NewDate(2025, time.January, 4),
        )).To(Succeed())
        fm := capture().Frontmatter
        Expect(fm).To(HaveKeyWithValue("assignee", "bborbe"))
        Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
        Expect(fm).To(HaveKeyWithValue("page_type", "task"))
        Expect(fm).To(HaveKeyWithValue("priority", 2))
        Expect(fm).To(HaveKeyWithValue(
            "goals",
            []interface{}{"[[Example Goal]]"},
        ))
        Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
        Expect(fm).To(HaveLen(6))
        // AC #4 explicit absence: the `recurring` key was removed by spec 008.
        Expect(fm).NotTo(HaveKey("recurring"))
    })

    It("does not depend on the entry's RecurrenceKind (no kind-specific keys)", func() {
        // After spec 008 the frontmatter shape is identical for every
        // RecurrenceKind — there is no kind-encoded field anymore. Two
        // entries with different kinds and otherwise identical definitions
        // produce the same Frontmatter.
        def1 := schedule.TaskDefinition{
            Slug:          "kind-a",
            TitleTemplate: "t",
            Recurrence:    schedule.RecurrenceDaily,
        }
        def2 := schedule.TaskDefinition{
            Slug:          "kind-b",
            TitleTemplate: "t",
            Recurrence:    schedule.RecurrenceYearly,
        }
        Expect(pub.Publish(
            context.Background(),
            def1,
            schedule.NewDate(2025, time.January, 4),
        )).To(Succeed())
        fm1 := capture().Frontmatter
        Expect(pub.Publish(
            context.Background(),
            def2,
            schedule.NewDate(2025, time.January, 4),
        )).To(Succeed())
        fm2 := capture().Frontmatter
        Expect(fm1).To(Equal(fm2))
    })
})
```

Notes that are load-bearing for the executor:

- The `HaveLen(6)` assertion locks the exact number of keys. Adding a seventh key in the future is a build-time test failure.
- The `Not(HaveKey("recurring"))` assertion is the explicit absence check required by AC #4. It is in addition to `HaveLen(6)` because the two are independent guarantees: a future refactor could keep the count at 6 by dropping a different key while keeping `recurring` — `HaveLen(6)` alone would not catch that.
- The second `It` test pins the invariant that the frontmatter shape is kind-independent. This is a corollary of dropping `recurring`; the test catches a future refactor that re-introduces a kind-dependent field.
- The two removed tests (the original "has the full frozen shape" seven-key test and the `DescribeTable("recurring matches RecurrenceKind", ...)`) are GONE — they reference a field that no longer exists and would fail to compile. They are not "temporarily disabled" or "commented out"; they are removed. The replaced block contains exactly the two `It` cases above.

## 5. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (two separate `feat:` bullets per the spec's AC #11; one bullet per logical change):

```markdown
- feat: Publisher renders every task title as `<bare-title> - <period-token>` (reusing `buildPeriodToken` for the token; separator is the three-character ` - `); the eight `TitleTemplate` placeholders `{{iso-week}} / {{next-iso-week}} / {{month}} / {{last-month}} / {{quarter}} / {{last-quarter}} / {{year}} / {{last-year}}` remain valid in `BodyTemplate` and are not stripped from the inventory in this prompt (Prompt 2 strips them from `TitleTemplate` only)
- feat: Drop the `recurring: <kind>` key from the published task frontmatter; `buildFrontmatter` is now no-arg and emits exactly six keys (`assignee`, `status`, `page_type`, `goals`, `priority`, `created_by`); downstream vault tooling treats every published task as a normal one-shot task
```

The second bullet is the ONLY place the frontmatter drop is described. The first bullet mentions that the placeholder stripping in inventory is Prompt 2's job — this is for the human reviewer's benefit so the changelog does not lie about what this prompt ships.

## 6. Imports and conventions

- The modified `/workspace/pkg/publisher/publisher.go` adds `"strings"` to its import block. Keep the block goimports-reviser-ordered: standard library first (alphabetical), then third-party (alphabetical), then internal (`github.com/bborbe/recurring-task-creator/...`). The existing imports are `context`, `github.com/bborbe/agent/lib/command/task`, `github.com/bborbe/errors`, `github.com/golang/glog`, `github.com/bborbe/recurring-task-creator/pkg/schedule`; `strings` slots in alphabetically at the top.
- The modified `/workspace/pkg/publisher/frontmatter.go` drops the `schedule` import. The remaining import is `lib "github.com/bborbe/agent/lib"`. Keep the `lib` alias (matches the rest of the package).
- The modified `/workspace/pkg/publisher/publisher_test.go` retains the existing imports. The new code uses `time` (already imported), `context` (already imported), `schedule.NewDate` (already imported), `taskmocks.TaskCreateCommandSender` (already imported), `publisher.NewPublisher` (already imported), and `publisher.Publisher` (already imported). No new imports.
- The 2026 copyright header is preserved on all three modified files.
- Use Ginkgo v2 / Gomega style with dot-imports (matches the existing tests).
- Do NOT touch `pkg/schedule/`, `pkg/handler/`, `pkg/factory/`, `pkg/tick/`, `main.go`, the Makefile, k8s manifests, or the Prometheus metric surface. The inventory data update is Prompt 2's job.
- Do NOT add a new Prometheus metric, opt-out flag, runtime config knob, or per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT regenerate any counterfeiter mock. The `Publisher` interface signature is unchanged; the `task.CreateCommandSender` interface is unchanged.
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- The period-token format is FROZEN by Specs 6 and 7. The new title-suffix logic MUST reuse `buildPeriodToken` verbatim — do NOT re-derive the token inline, do NOT re-implement `weekdayAbbrev`, do NOT re-implement the `fmt*` helpers, do NOT change the `uuidNamespace` constant.
- The `Publisher` interface signature is FROZEN — do NOT add a new method, do NOT change parameter order, do NOT add a new return value.
- The `task.CreateCommand` shape (the struct fields `TaskIdentifier`, `Title`, `Frontmatter`, `Body`) is FROZEN — do NOT add a new field, do NOT remove a field, do NOT rename a field. The new `Title` value carries a suffix; the new `Frontmatter` value has one fewer key; the other two fields are unchanged.
- The `buildFrontmatter` signature change is one-way: from `buildFrontmatter(recurrence schedule.RecurrenceKind)` to `buildFrontmatter()`. Do NOT keep the `recurrence` parameter "for future use" — the spec Non-goals forbid reintroducing kind-dependent behavior in the frontmatter.
- The 3-letter weekday abbreviation in the weekly suffix is lowercase (`mon` / `tue` / `wed` / `thu` / `fri` / `sat` / `sun`); the `W` in the week prefix stays uppercase. Matches Spec 7.
- The separator between the bare title and the suffix is exactly the three-character string ` - ` (space, hyphen, space). No other separator is allowed. Spec Non-goal.
- The `strings.TrimSpace` call is applied to the RENDERED title (post-`renderTemplate`), not to the original `TitleTemplate`. The order in the publisher is: render placeholders → trim → append separator → append token. This order is fixed.
- Existing tests must still pass after all edits. The only tests that are intentionally modified are the two removed tests in the existing `Describe("frontmatter", ...)` block (§4). The new specs ADD coverage; they do not remove any other test.
- Coverage on `pkg/publisher` stays at or above 80%. The new title-suffix specs add 5+ passing test cases (3 `It` + 1 `DescribeTable` × 5 entries = 8 cases) to the publisher test suite; the new frontmatter specs add 2 more. The package's coverage should stay well above 80%.
- Project DoD (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` for error wrapping (the new `buildPeriodToken` error path uses it); no `context.Background()` in business logic (the new code uses the function's `ctx` parameter); no `time.Time` / `time.Now()` in business logic (the new code uses `time.Saturday` / `time.Monday` constants and `schedule.NewDate`, NOT `time.Time`); GoDoc on the new `buildFrontmatter` (existing GoDoc, updated); `make precommit` clean.
- The `uuidNamespace` constant in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN byte-identical — do NOT touch it.
- The `recurring-<slug>-<period-token>` UUID5 input string format is FROZEN — do NOT touch it.
- The `RecurrenceKind` enum stays a closed set of exactly 5 values.
- The `/trigger?date=` HTTP response shape is unchanged.
- Do NOT commit — dark-factory handles git.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/publisher/...` — all Ginkgo specs green. In particular:
   - `It("appends the period token to a monthly title", ...)` passes — asserts `Title == "Update K3s - 2026-06"` for `RecurrenceMonthly` + 2026-06-15.
   - `It("appends the period token to a weekly title (with weekday suffix)", ...)` passes — asserts `Title == "Shutdown K3s - 2026W25-sat"` for `RecurrenceWeekly` + `Weekday=Saturday` + 2026-06-17.
   - `It("trims whitespace from the rendered title before appending the suffix", ...)` passes.
   - `It("renders bare placeholders-only templates as '<token>' after substitution and suffix", ...)` passes — asserts `Title == "2026-06 - 2026-06"`.
   - `DescribeTable("appends '<bare> - <period-token>' for every RecurrenceKind", ...)` passes for all 5 entries (daily / weekly / monthly / quarterly / yearly).
   - `It("has the six-key shape (assignee, status, page_type, goals, priority, created_by)", ...)` passes — asserts `HaveLen(6)` AND `Not(HaveKey("recurring"))`.
   - `It("does not depend on the entry's RecurrenceKind (no kind-specific keys)", ...)` passes.
   - The pre-existing tests (identifier byte-equality, period anchoring, placeholder rendering, sender interaction, boundary contract, package surface) continue to pass. The two removed tests (the seven-key shape test and the `recurring matches RecurrenceKind` table) are GONE — they are not "skipped", not "commented out", and not present in the file.
3. `grep -nE '"recurring"' pkg/publisher/frontmatter.go` — must return no matches (AC #5). Expected: exit code 1, empty output.
4. `grep -nE '\bbuildFrontmatter\(' pkg/publisher/` — must return exactly 1 match (the `Publish` call site, no-arg). The previous call site `buildFrontmatter(def.Recurrence)` is gone.
5. `grep -nE 'recurring.*RecurrenceKind|RecurrenceKind.*recurring' pkg/publisher/frontmatter.go` — must return no matches.
6. `grep -nE 'string\(recurrence\)' pkg/publisher/` — must return no matches (the kind-to-string conversion is gone from the frontmatter path).
7. `grep -n 'periodToken' pkg/publisher/publisher.go` — must return at least 2 matches: the variable declaration and the use in the `Title` field of the `cmd` literal.
8. `grep -n 'strings.TrimSpace' pkg/publisher/publisher.go` — must return exactly 1 match (the new trim around `renderTemplate`).
9. `grep -nE '"f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a"' pkg/publisher/uuid_namespace.go` — must return exactly 1 match. The frozen namespace constant is byte-identical.
10. `grep -nE 'recurring-<slug>-<period-token>|recurring-" \+ slug' pkg/publisher/uuid_namespace.go` — must return at least 1 match. The identifier input string format is byte-identical.
11. `grep -nE 'func buildPeriodToken' pkg/publisher/uuid_namespace.go` — must return exactly 1 match. The function is unchanged.
12. `grep -n 'buildFrontmatter' pkg/publisher/publisher.go pkg/publisher/frontmatter.go pkg/publisher/publisher_test.go` — must show: 1 declaration in `frontmatter.go` (the new no-arg signature), 1 call site in `publisher.go` (the no-arg call), and 0 references in `publisher_test.go` (the new frontmatter spec asserts on `capture().Frontmatter`, not on `buildFrontmatter` directly).
13. Spot-check: open `pkg/publisher/publisher.go` and visually confirm (a) the new `periodToken` variable is computed via `buildPeriodToken` (NOT re-derived inline); (b) the `Title` field concatenates `strings.TrimSpace(renderTemplate(...))` + `" - "` + `periodToken`; (c) the `Frontmatter` field calls `buildFrontmatter()` with no arguments; (d) the `"strings"` import is in the import block in goimports-reviser order.
14. Spot-check: open `pkg/publisher/frontmatter.go` and visually confirm (a) the function signature is `func buildFrontmatter() lib.TaskFrontmatter` (no parameters); (b) the returned map has exactly 6 entries; (c) the `"github.com/bborbe/recurring-task-creator/pkg/schedule"` import is REMOVED.
15. Spot-check: open `pkg/publisher/publisher_test.go` and visually confirm (a) the new `Describe("title suffix", ...)` block is between the existing `Describe("placeholder rendering", ...)` and `Describe("frontmatter", ...)` blocks; (b) the existing `Describe("frontmatter", ...)` block contains exactly the two new `It` cases; (c) the old `DescribeTable("recurring matches RecurrenceKind", ...)` is gone; (d) the `HaveLen(6)` and `Not(HaveKey("recurring"))` assertions are present.
16. Coverage check on the changed package:
    - `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/publisher/...`
    - `go tool cover -func=/tmp/cover.out | tail -1` — total coverage ≥ 80%.
17. Changelog:
    - `grep -nE 'feat:.*title suffix|feat:.*period token' CHANGELOG.md` — must return at least 1 line (AC #11, first bullet).
    - `grep -nE 'feat:.*recurring' CHANGELOG.md` — must return at least 1 line (AC #11, second bullet — note: this matches the word `recurring` in the bullet text, which describes the dropped key; it is NOT a `recurring` key in the frontmatter map, just a word in the changelog entry).
18. End-to-end smoke: with the new code, every published command has a title that ends with ` - <period-token>` and a frontmatter map with exactly 6 keys. The `/trigger?date=` HTTP handler in `pkg/handler/trigger` and the tick loop in `pkg/tick` both call `pub.Publish` (not `buildFrontmatter` or `buildPeriodToken` directly), so they pick up the new behavior automatically. Run `go test ./...` to confirm the full test suite is green.

## Open Questions (for the human reviewer)

- **A. The two-pass `buildTaskIdentifier` + `buildPeriodToken` in `Publisher.Publish`.** The new code calls `buildTaskIdentifier` (which internally calls `buildPeriodToken` to produce the token for the UUID5 input string) and then calls `buildPeriodToken` again to read the same token for the title suffix. This is a deliberate trade-off: it keeps `buildTaskIdentifier`'s signature unchanged (a frozen public helper, used elsewhere) at the cost of a single extra switch on `RecurrenceKind`. The alternative — modifying `buildTaskIdentifier` to return `(lib.TaskIdentifier, string, error)` — would touch the public helper and require updating the mock surface. The two-pass approach is preferred because (a) `buildPeriodToken` is cheap (one switch, no I/O), and (b) the public helper's signature is a contract. If you prefer the single-pass variant, swap to a return-tuple signature; the prompt's content remains valid modulo the call-site adjustment.
- **B. The `goalsLink` constant location.** `goalsLink` is defined in `pkg/publisher/publisher.go` (line 20) and referenced by the new no-arg `buildFrontmatter` in `pkg/publisher/frontmatter.go`. The constant is package-scoped, so no qualifier is needed. No change in this prompt.
- **C. The placeholders-only template edge case.** The new test `"renders bare placeholders-only templates..."` exercises a `TitleTemplate` of `"{{month}}"` (renders to `"2026-06"`) with a monthly `RecurrenceKind` and a June date; the result is `"2026-06 - 2026-06"`. No inventory entry uses this template; the test pins the behavior down so a future refactor cannot accidentally collapse the two halves. The test is intentionally included for safety; it does not exercise a real inventory entry.
- **D. The `frontmatter` `Describe` block relocation.** The original `Describe("frontmatter", ...)` block sits between `Describe("sender interaction", ...)` and `Describe("boundary contract", ...)` (lines 472-518 of the file as it stands). The replacement block is in the same location; the only change is the body. The new `Describe("title suffix", ...)` block is inserted BEFORE the `Describe("frontmatter", ...)` block — between the closing `})` of `Describe("placeholder rendering", ...)` and the opening `Describe("frontmatter", ...)` line.
- **E. The `Recurrence` field on the publisher's def literal in the placeholder-only test.** The `Recurrence: rec,` line in the `DescribeTable` body sets the recurrence explicitly. For the `RecurrenceWeekly` entry, `Weekday: time.Saturday` is set explicitly (the helper `buildPeriodToken` reads `def.Weekday` for the weekly token); for the other four kinds, the `Weekday` field is at the zero value `time.Sunday` (which `buildPeriodToken` ignores per its switch). This is the same pattern as the existing `DescribeTable("period-token byte-equality with the formatter output", ...)` (lines 212-251) and `It("non-weekly kinds ignore the Weekday field (token is identical to Spec 6)", ...)` (lines 302-333) — both of which pre-date this prompt and pass after it. Mirror their style.
- **F. No inventory data change in this prompt.** The 8 inventory entries that carry period placeholders in `TitleTemplate` are NOT modified in this prompt. After this prompt, a `Publish` call for `weekly-review` on 2026-06-17 produces `Title == "Weekly Review 2026W25 - 2026W25-sat"` — a temporary double-token shape that Prompt 2 fixes by stripping the `{{iso-week}}` from the inventory `TitleTemplate`. The temporary shape is harmless: it is a non-existent combination (no `Publish` call hits it in production) and is removed by the end of Prompt 2. The new `frontmatter` and `title suffix` tests do NOT exercise the 8 affected entries — they use bare strings like `"Update K3s"`, `"Shutdown K3s"`, `"Bare"` — so this prompt is test-green without the inventory fix.

</verification>
