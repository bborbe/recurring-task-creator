---
status: executing
spec: [006-period-anchored-uuid]
container: recurring-task-creator-always-fire-exec-011-period-anchored-identifier
dark-factory-version: v0.177.1
created: "2026-06-14T20:30:00Z"
queued: "2026-06-14T20:10:24Z"
started: "2026-06-14T20:10:25Z"
branch: dark-factory/period-anchored-uuid
---

<summary>
- Switches the publisher's deterministic identifier from a date-anchored shape to a period-anchored shape, so weekly, monthly, quarterly, and yearly tasks collapse to one identifier per period regardless of which day inside that period the publisher runs.
- The identifier input string is now `recurring-<slug>-<period-token>`, where `<period-token>` is `YYYY-MM-DD` for daily entries and one of `YYYYWNN`, `YYYY-MM`, `YYYYQN`, `YYYY` for weekly, monthly, quarterly, and yearly entries respectively — the exact strings already produced by the formatters in `pkg/publisher/render.go`.
- Reuses the existing `fmtIsoWeek`, `fmtMonthYear`, `fmtQuarter`, and `fmtYear` helpers unchanged. No new formatter is introduced.
- The `uuidNamespace` constant remains byte-identical (`f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a`); the publisher never reads env, never regenerates it.
- The publisher's external `Publish(ctx, def, date)` signature is unchanged. Period anchoring is computed internally from `def.Recurrence` and `date` — the publisher never reads `def.Fires` for identifier derivation.
- The existing identifier-shape test in `pkg/publisher/publisher_test.go` is updated to assert the new period-anchored shape; a new table test suite covers equality-across-period-internals and inequality-across-period-boundaries for every `RecurrenceKind`.
- `make precommit` exits 0; coverage on `pkg/publisher` stays at or above 80%.

</summary>

<objective>

Change the publisher's deterministic identifier derivation in `pkg/publisher` from a date-anchored shape (`"recurring-<slug>-<YYYY-MM-DD>"`) to a period-anchored shape (`"recurring-<slug>-<period-token>"`), where the period token is computed from `def.Recurrence` and `date` and matches the format already used by the title-rendering formatters in `pkg/publisher/render.go`. This unblocks the hourly tick (Prompt 2) which will publish the full inventory every hour: with a period-anchored identifier, every retry within the same period produces the same UUID5 and the controller's de-dup absorbs the duplicate. The publisher's external `Publish` signature is unchanged; the `uuidNamespace` constant is byte-identical.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before writing code:

- `/workspace/pkg/publisher/uuid_namespace.go` — contains the frozen `uuidNamespace` constant (line 24: `var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")`), the existing `buildTaskIdentifier(slug, date)` helper, and `isoDate(date)`. This is the only file in the package that touches identifier construction. The frozen constant MUST remain byte-identical; the rest of the file is rewritten by this prompt.
- `/workspace/pkg/publisher/render.go` — contains `fmtIsoWeek`, `fmtMonthYear`, `fmtQuarter`, `fmtYear`, `fmtDate`, `quarterOf`, `firstOfPreviousMonth`, `previousQuarter`, and `dateToTime`. REUSE these formatters; do not duplicate them. The exact format strings they emit are load-bearing: `fmtIsoWeek` → `YYYYWNN` (e.g. `2025W24`), `fmtMonthYear` → `YYYY-MM` (e.g. `2025-06`), `fmtQuarter` → `YYYYQN` (e.g. `2025Q2`), `fmtYear` → `YYYY` (e.g. `2025`), `fmtDate` → `YYYY-MM-DD` (e.g. `2025-06-14`). The `fmtQuarter` format is the single-digit-quarter variant (`%dQ%d`, not the zero-padded variant the Spec 2 prompt originally proposed) — match what is in the file today.
- `/workspace/pkg/publisher/publisher.go` — `Publisher` interface (signature `Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error`) and the unexported `publisher` struct. The interface signature is FROZEN. The counterfeiter directive on the interface stays as-is (`-o ../../mocks/publisher-publisher.go --fake-name PublisherPublisher . Publisher`). The body of `Publish` is updated by this prompt only at the `buildTaskIdentifier` call site (one line, plus error handling); the rest of `Publish` is unchanged.
- `/workspace/pkg/publisher/publisher_test.go` — the existing test "is the UUID5 of the canonical key" (lines 38-57) hard-codes the input string `"recurring-weekly-review-2025-01-04"`. This test MUST be updated to assert the new period-anchored shape for a weekly entry on `2025-01-04` (which falls in ISO week 2025W01) — the new expected input string is `"recurring-weekly-review-2025W01"`. The other tests (placeholder rendering, determinism, frontmatter shape, sender interaction, boundary contract) are unaffected because they assert on `Title`, `Body`, `Frontmatter`, and `Validate()`, not on the raw identifier input.
- `/workspace/pkg/publisher/export_test.go` — contains the test-only accessor `UuidNamespaceForTest() uuid.UUID`. Reuse as-is; the new tests use it the same way.
- `/workspace/pkg/publisher/no_forbidden_imports_test.go` — walk-and-grep guard. No change.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` is a string alias with five constants: `RecurrenceDaily` (`"daily"`), `RecurrenceWeekly` (`"weekly"`), `RecurrenceMonthly` (`"monthly"`), `RecurrenceQuarterly` (`"quarterly"`), `RecurrenceYearly` (`"yearly"`). The closed set is the anchor of the new period-token mapping; the publisher derives the period token from `def.Recurrence` alone.
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition{Slug, TitleTemplate, BodyTemplate, Recurrence, Fires}`. The publisher reads `Slug` and `Recurrence`; it does NOT read `Fires` for identifier derivation (the firing predicate is a schedule-hint layer, not an identifier layer — see Open Question A in `<verification>` for the spec ambiguity this resolves).
- `/workspace/pkg/schedule/date.go` — `Date{Year int, Month time.Month, Day int}`, `NewDate(...)`, `IsZero()`, `Time() time.Time`. The publisher continues to receive a `Date` from the tick (Prompt 2); no API change.
- `/workspace/CHANGELOG.md` — `## Unreleased` already has bullets from Specs 1-5. Append a new `feat:` bullet for this change. Prefix `feat:` per `changelog-guide.md` (minor bump).

Verified external symbols (grep'd at `/home/node/go/pkg/mod/` on 2026-06-14; no new deps are needed by this prompt):

- `github.com/google/uuid` (already in `go.mod`): `func NewSHA1(space UUID, data []byte) UUID` in `hash.go`; `var uuidNamespace = uuid.MustParse(...)` produces a `uuid.UUID` value.
- `github.com/bborbe/agent/lib` (already in `go.mod`): `type TaskIdentifier string`; `func (TaskIdentifier) String() string` is implicit via the `string` underlying type.
- `github.com/bborbe/errors` (already in `go.mod`): `Wrapf(ctx, err, format, args...)`, `Errorf(ctx, format, args...)`.

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — the publisher's public API surface is unchanged; only the internals of `buildTaskIdentifier` change. Interface → Constructor → Struct → Method pattern is already in place.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf(ctx, ...)` and `errors.Wrapf(ctx, err, ...)` for the new error paths.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; `DescribeTable` / `Entry` for the period-anchor table; external test package (`package publisher_test`).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter mock for `task.CreateCommandSender` is reused from `github.com/bborbe/agent/lib/command/task/mocks` (path `TaskCreateCommandSender`); no new mocks are generated by this prompt.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for new code; every error path tested; boundary contract test (the `cmd.Validate(ctx)` test) is preserved.

Load-bearing snippets inlined for the executor's verification (read fresh before writing):

```go
// pkg/schedule/recurrence.go (verbatim, lines 8-16)
type RecurrenceKind string

const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)

// pkg/publisher/uuid_namespace.go line 24 (FROZEN, do not edit)
var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")

// pkg/publisher/render.go (verbatim signatures, lines 64-83)
func fmtIsoWeek(year, week int) string    // → "YYYYWNN"
func fmtMonthYear(year, month int) string // → "YYYY-MM"
func fmtQuarter(year, quarter int) string // → "YYYYQN" (single-digit quarter)
func fmtYear(year int) string             // → "YYYY"
func fmtDate(year, month, day int) string // → "YYYY-MM-DD"

// pkg/publisher/publisher.go (FROZEN, do not edit)
type Publisher interface {
    Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
}
```

</context>

<requirements>

## 1. Add `buildPeriodToken` to `pkg/publisher`

In `/workspace/pkg/publisher/uuid_namespace.go` (same file as the frozen namespace constant; do NOT create a second file), add the new helper BELOW the rewritten `buildTaskIdentifier` (see §2):

```go
// buildPeriodToken returns the period-anchored token for the given
// (recurrence, date) pair. The token is the same string the corresponding
// title-rendering formatter produces — "YYYY-MM-DD" for daily, "YYYYWNN"
// for weekly, "YYYY-MM" for monthly, "YYYYQN" for quarterly, "YYYY" for
// yearly. Anchoring by def.Recurrence (not def.Fires) is intentional: the
// publisher's identifier layer is period-stable, the schedule's firing
// predicate is a hint about which day inside the period the user wants
// to see the task. See Open Question A in the prompt's <verification>.
//
// Berlin local time governs the period boundary; the date passed in is
// already Berlin-local (the tick converts wall-clock to Berlin civil date
// before calling Publish).
//
// An unknown RecurrenceKind is a build-time data error (closed enum, no
// valid runtime reason for a new value), so the function returns an error
// rather than a sentinel string. The caller wraps with the slug.
func buildPeriodToken(ctx context.Context, recurrence schedule.RecurrenceKind, date schedule.Date) (string, error) {
    base := date.Time()
    switch recurrence {
    case schedule.RecurrenceDaily:
        return fmtDate(date.Year, int(date.Month), date.Day), nil
    case schedule.RecurrenceWeekly:
        isoYear, isoWeek := base.ISOWeek()
        return fmtIsoWeek(isoYear, isoWeek), nil
    case schedule.RecurrenceMonthly:
        return fmtMonthYear(base.Year(), int(base.Month())), nil
    case schedule.RecurrenceQuarterly:
        return fmtQuarter(base.Year(), quarterOf(base.Month())), nil
    case schedule.RecurrenceYearly:
        return fmtYear(base.Year()), nil
    default:
        return "", errors.Errorf(ctx, "buildPeriodToken: unknown recurrence kind %q", recurrence)
    }
}
```

Notes on the above:
- The `ctx` parameter threads through from `Publish` → `buildTaskIdentifier` → `buildPeriodToken`. Project DoD (`docs/dod.md`) forbids `context.Background()` in business logic; the caller's ctx is always available since `Publish(ctx, ...)` is the only entry point.
- The `errors.Errorf` import is `github.com/bborbe/errors`. Add it to the import block.
- The `time` import is required for the `base.ISOWeek()` and `base.Year()` calls. `date.Time()` already returns a `time.Time`; no new conversion is needed.
- The function consumes the existing `fmtDate`, `fmtIsoWeek`, `fmtMonthYear`, `fmtQuarter`, `fmtYear`, and `quarterOf` helpers from `render.go` (same package — no import qualifier). Do not redefine them.

## 2. Rewrite `buildTaskIdentifier` and remove `isoDate`

Replace the existing `buildTaskIdentifier` and `isoDate` functions in `/workspace/pkg/publisher/uuid_namespace.go` (lines 26-38 in the current file) with the new three-argument signature that consumes `Recurrence`:

```go
// buildTaskIdentifier returns the deterministic TaskIdentifier for the
// (slug, recurrence, date) triple. The identifier is
// UUID5(uuidNamespace, "recurring-<slug>-<period-token>"), where
// <period-token> is the period-anchored token derived from recurrence
// and date (see buildPeriodToken). Same input on a second call produces
// the same identifier across processes, redeploys, and replays — this is
// the contract the controller's de-dup relies on.
//
// For weekly / monthly / quarterly / yearly entries the identifier is
// stable across all days inside one period, so the hourly tick can
// publish the full inventory every hour without producing duplicate
// vault files. For daily entries (and any future entry that should
// remain date-anchored) the identifier is the civil date itself.
func buildTaskIdentifier(ctx context.Context, slug string, recurrence schedule.RecurrenceKind, date schedule.Date) (lib.TaskIdentifier, error) {
    token, err := buildPeriodToken(ctx, recurrence, date)
    if err != nil {
        return "", errors.Wrapf(ctx, err, "buildTaskIdentifier: slug %q", slug)
    }
    name := "recurring-" + slug + "-" + token
    return lib.TaskIdentifier(uuid.NewSHA1(uuidNamespace, []byte(name)).String()), nil
}
```

Delete the `isoDate` function (lines 36-38 of the current file) — it is no longer called.

Update the call site in `/workspace/pkg/publisher/publisher.go` (line 57: `TaskIdentifier: buildTaskIdentifier(def.Slug, date)`) to handle the new `(ctx, slug, recurrence, date) → (string, error)` signature. The exact replacement is:

```go
token, err := buildTaskIdentifier(ctx, def.Slug, def.Recurrence, date)
if err != nil {
    return errors.Wrapf(ctx, err, "publish failed: build identifier for slug %q", def.Slug)
}
cmd := task.CreateCommand{
    TaskIdentifier: token,
    Title:          renderTemplate(def.TitleTemplate, def.Slug, date),
    Frontmatter:    buildFrontmatter(def.Recurrence),
    Body:           renderTemplate(def.BodyTemplate, def.Slug, date),
}
```

The two structural requirements are: (1) the error from `buildTaskIdentifier` is propagated and wrapped with the slug and the word "publish failed" (matching the wrap style of the existing sender-error path at line 63); (2) `cmd.TaskIdentifier` is the returned `lib.TaskIdentifier`.

## 3. Update the existing identifier test

In `/workspace/pkg/publisher/publisher_test.go`, update the test at lines 38-57 (the `Describe("identifier")` block, the single `It("is the UUID5 of the canonical key")` test) to assert the new period-anchored shape. The change is in the expected input string only — from `"recurring-weekly-review-2025-01-04"` to `"recurring-weekly-review-2025W01"` (because `2025-01-04` falls in ISO week `2025W01`; the `isoYear` from `date.Time().ISOWeek()` for this date is `2025`):

```go
Describe("identifier", func() {
    It("is the UUID5 of the canonical key", func() {
        def := schedule.TaskDefinition{
            Slug:          "weekly-review",
            TitleTemplate: "Weekly Review {{iso-week}}",
            Recurrence:    schedule.RecurrenceWeekly,
        }
        Expect(pub.Publish(
            context.Background(),
            def,
            schedule.NewDate(2025, time.January, 4),
        )).To(Succeed())
        captured := capture()
        expected := uuid.NewSHA1(
            publisher.UuidNamespaceForTest(),
            []byte("recurring-weekly-review-2025W01"),
        ).String()
        Expect(string(captured.TaskIdentifier)).To(Equal(expected))
    })
})
```

No other test in `publisher_test.go` is affected: placeholder rendering, determinism, frontmatter shape, sender interaction, and boundary contract tests assert on `Title`, `Body`, `Frontmatter`, and `Validate()` — not on the raw identifier input. The boundary contract test (which asserts `captured.Validate(ctx)` succeeds) is preserved unchanged.

## 4. Add the period-anchor table tests

In `/workspace/pkg/publisher/publisher_test.go`, after the existing `Describe("identifier")` block, add the new period-anchor test suite. This is the AC #1-#4 and AC #6 evidence for the publisher. Use a single `Describe("period anchoring")` block with three sub-blocks: equality-within-period (`It`s), inequality-across-boundaries (`It`s), and byte-equality-with-formatter (`DescribeTable`):

```go
Describe("period anchoring", func() {
    // captureIdentifier runs Publish and returns the TaskIdentifier.
    captureIdentifier := func(slug string, rec schedule.RecurrenceKind, date schedule.Date) lib.TaskIdentifier {
        def := schedule.TaskDefinition{
            Slug:          slug,
            TitleTemplate: "t",
            Recurrence:    rec,
        }
        Expect(pub.Publish(context.Background(), def, date)).To(Succeed())
        return capture().TaskIdentifier
    }

    // Equality-within-period (4 cases, one per non-daily RecurrenceKind).
    It("weekly: same ISO week, different civil dates produce the same identifier", func() {
        // 2025-06-09 (Mon) and 2025-06-15 (Sun) are both in ISO 2025W24.
        id1 := captureIdentifier("w1", schedule.RecurrenceWeekly, schedule.NewDate(2025, time.June, 9))
        id2 := captureIdentifier("w1", schedule.RecurrenceWeekly, schedule.NewDate(2025, time.June, 15))
        Expect(id1).To(Equal(id2))
    })

    It("monthly: same month, different civil dates produce the same identifier", func() {
        id1 := captureIdentifier("m1", schedule.RecurrenceMonthly, schedule.NewDate(2025, time.June, 1))
        id2 := captureIdentifier("m1", schedule.RecurrenceMonthly, schedule.NewDate(2025, time.June, 30))
        Expect(id1).To(Equal(id2))
    })

    It("quarterly: same quarter, different civil dates produce the same identifier", func() {
        id1 := captureIdentifier("q1", schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.April, 1))
        id2 := captureIdentifier("q1", schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.June, 30))
        Expect(id1).To(Equal(id2))
    })

    It("yearly: same year, different civil dates produce the same identifier", func() {
        id1 := captureIdentifier("y1", schedule.RecurrenceYearly, schedule.NewDate(2025, time.January, 1))
        id2 := captureIdentifier("y1", schedule.RecurrenceYearly, schedule.NewDate(2025, time.December, 31))
        Expect(id1).To(Equal(id2))
    })

    // Inequality-across-period-boundaries (4 cases).
    It("weekly: adjacent ISO weeks produce different identifiers", func() {
        // 2025-06-15 (Sun) is 2025W24; 2025-06-16 (Mon) is 2025W25.
        id1 := captureIdentifier("w1", schedule.RecurrenceWeekly, schedule.NewDate(2025, time.June, 15))
        id2 := captureIdentifier("w1", schedule.RecurrenceWeekly, schedule.NewDate(2025, time.June, 16))
        Expect(id1).NotTo(Equal(id2))
    })

    It("monthly: adjacent months produce different identifiers", func() {
        id1 := captureIdentifier("m1", schedule.RecurrenceMonthly, schedule.NewDate(2025, time.May, 31))
        id2 := captureIdentifier("m1", schedule.RecurrenceMonthly, schedule.NewDate(2025, time.June, 1))
        Expect(id1).NotTo(Equal(id2))
    })

    It("quarterly: adjacent quarters produce different identifiers", func() {
        id1 := captureIdentifier("q1", schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.June, 30))
        id2 := captureIdentifier("q1", schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.July, 1))
        Expect(id1).NotTo(Equal(id2))
    })

    It("yearly: adjacent years produce different identifiers", func() {
        id1 := captureIdentifier("y1", schedule.RecurrenceYearly, schedule.NewDate(2025, time.December, 31))
        id2 := captureIdentifier("y1", schedule.RecurrenceYearly, schedule.NewDate(2026, time.January, 1))
        Expect(id1).NotTo(Equal(id2))
    })

    // Daily is date-anchored (AC #5 evidence).
    It("daily: distinct civil dates produce distinct identifiers", func() {
        id1 := captureIdentifier("d1", schedule.RecurrenceDaily, schedule.NewDate(2025, time.June, 14))
        id2 := captureIdentifier("d1", schedule.RecurrenceDaily, schedule.NewDate(2025, time.June, 15))
        Expect(id1).NotTo(Equal(id2))
    })

    // Byte-equality with the title-rendering formatter output (AC #6).
    // The period token in the identifier-input string must be byte-identical
    // to the literal string the corresponding fmt* helper produces.
    DescribeTable("period-token byte-equality with the formatter output",
        func(rec schedule.RecurrenceKind, date schedule.Date, expectedToken string) {
            slug := "byte-eq-" + string(rec)
            def := schedule.TaskDefinition{
                Slug:          slug,
                TitleTemplate: "t",
                Recurrence:    rec,
            }
            Expect(pub.Publish(context.Background(), def, date)).To(Succeed())
            cmd := capture()
            expected := "recurring-" + slug + "-" + expectedToken
            want := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte(expected)).String()
            Expect(string(cmd.TaskIdentifier)).To(Equal(want))
        },
        Entry("daily", schedule.RecurrenceDaily, schedule.NewDate(2025, time.June, 14), "2025-06-14"),
        Entry("weekly", schedule.RecurrenceWeekly, schedule.NewDate(2025, time.June, 9), "2025W24"),
        Entry("monthly", schedule.RecurrenceMonthly, schedule.NewDate(2025, time.June, 1), "2025-06"),
        Entry("quarterly", schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.April, 1), "2025Q2"),
        Entry("yearly", schedule.RecurrenceYearly, schedule.NewDate(2025, time.January, 1), "2025"),
    )
})
```

Imports required for the new tests: `lib "github.com/bborbe/agent/lib"` (for the `lib.TaskIdentifier` return-type annotation on the `captureIdentifier` helper). The `uuid` import is already present in the file.

Each `expectedToken` is the literal string the corresponding `fmtXxx` helper produces for the given date. The quarterly entry uses `2025Q2` (single-digit quarter, matching `fmtQuarter`'s `%dQ%d` format string in `render.go` line 77).

## 5. Add the unknown-recurrence-kind error test

In `/workspace/pkg/publisher/publisher_test.go`, add a new `It` block that asserts the unknown-recurrence-kind error path. The publisher's RecurrenceKind enum is closed and exhaustive, so the only way to exercise this path is to bypass the type system via a string literal cast:

```go
It("returns a wrapped error for an unknown recurrence kind", func() {
    def := schedule.TaskDefinition{
        Slug:          "unknown-rec",
        TitleTemplate: "t",
        Recurrence:    schedule.RecurrenceKind("unknown"),
    }
    err := pub.Publish(context.Background(), def, schedule.NewDate(2025, time.June, 14))
    Expect(err).To(HaveOccurred())
    Expect(err.Error()).To(ContainSubstring("unknown"))
    Expect(err.Error()).To(ContainSubstring("unknown-rec")) // slug is in the wrap
    Expect(sender.SendCommandCallCount()).To(Equal(0))
})
```

This test corresponds to failure-mode row 3 in the spec ("Inventory contains a recurrence kind not handled by the period-anchor mapping").

## 6. Verify the namespace constant is byte-identical

After all edits, the line in `/workspace/pkg/publisher/uuid_namespace.go` that declares the namespace constant MUST read exactly:

```go
var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")
```

The literal UUID, the variable name, the `uuid.UUID` type annotation, and the `uuid.MustParse` constructor are all frozen. The `<verification>` grep must return this exact line.

## 7. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (one bullet, `feat:` prefix per `changelog-guide.md`):

```markdown
- feat: Switch `pkg/publisher` deterministic identifier to period-anchored shape `recurring-<slug>-<period-token>` (token is `YYYY-MM-DD` for daily, `YYYYWNN` for weekly, `YYYY-MM` for monthly, `YYYYQN` for quarterly, `YYYY` for yearly; reused the title-rendering formatters in `pkg/publisher/render.go`); uuid namespace constant unchanged
```

## 8. Imports and conventions

- Every modified `.go` file retains the 2026 copyright header.
- Use `goimports-reviser` style: standard library first, then third-party (alphabetical: `github.com/bborbe/...`, `github.com/google/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use `github.com/bborbe/errors` for error wrapping. Never `fmt.Errorf`.
- Do NOT touch `main.go`, the Makefile, `pkg/tick/`, `pkg/handler/`, `pkg/factory/`, `pkg/schedule/`, K8s manifests, or the Prometheus metric surface.
- Do NOT add a new Prometheus metric, opt-out flag, runtime config knob, or per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT regenerate the counterfeiter mock for `Publisher` — the existing `/workspace/mocks/publisher-publisher.go` is for the same interface (signature is unchanged).
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- The `uuidNamespace` constant in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN byte-identical. Do NOT change its value, do NOT read it from env, do NOT regenerate it. The literal `f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a` is the contract.
- The `Publisher` interface signature (`Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error`) is FROZEN. The counterfeiter directive on the interface stays as-is.
- The publisher's period-token derivation MUST reuse the existing `fmtIsoWeek`, `fmtMonthYear`, `fmtQuarter`, `fmtYear`, `fmtDate`, and `quarterOf` helpers in `pkg/publisher/render.go` — never introduce a second formatter for the same period. The byte-equality with the title-rendering formatters is the AC #6 contract.
- Period anchoring is anchored on `def.Recurrence` alone. The publisher does NOT read `def.Fires` for identifier derivation. This is a deliberate design decision (see Open Question A in `<verification>`).
- The package MUST NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, or any `github.com/bborbe/jira-task-creator/...` package. The forbidden-imports Ginkgo test enforces this.
- The package MUST NOT call `time.Now()`. The date is supplied by the caller (the tick, Prompt 2).
- Error wrapping uses `github.com/bborbe/errors`. `context.Background()` is permitted only inside the new `buildPeriodToken` and `buildTaskIdentifier` (which have no ctx parameter); the `Publish` method always propagates the real `ctx` to all error-wrap calls and to the sender.
- Tests use Ginkgo v2 / Gomega; mocks are the pre-existing `taskmocks.TaskCreateCommandSender` from `github.com/bborbe/agent/lib/command/task/mocks`. Coverage ≥80% for the package.
- The `frontmatter.go` file is unchanged.
- Existing tests in `pkg/schedule` (canonical slugs, inventory validation, no-forbidden-imports) must still pass with no changes to inventory data. This prompt does not edit `pkg/schedule/`.
- Do NOT add a Prometheus metric, an opt-out flag, a runtime config knob, or any per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/publisher/...` — all Ginkgo specs green.
3. `grep -nE '"f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a"' pkg/publisher/uuid_namespace.go` — must return exactly one match (line 24 of the file).
4. `grep -nE '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/publisher/*.go` — must return no matches (excluding `*_test.go` files; the forbidden-imports Ginkgo test enforces this for production files).
5. `grep -n 'isoDate' pkg/publisher/uuid_namespace.go` — must return no matches (the `isoDate` helper was removed; if it still appears the rewrite is incomplete).
6. `grep -nE 'fmt(Quarter|Year|MonthYear|IsoWeek|Date)\(' pkg/publisher/uuid_namespace.go` — must return at least one match per helper inside the `buildPeriodToken` switch (the formatters themselves remain in `render.go`).
7. `grep -n 'buildPeriodToken' pkg/publisher/*.go` — must return at least two matches (the definition and the call site in `buildTaskIdentifier`).
8. Spot-check: open `pkg/publisher/uuid_namespace.go` and visually confirm the namespace constant line is byte-identical, `isoDate` is gone, and `buildPeriodToken` + the rewritten `buildTaskIdentifier` are present.
9. Spot-check: open `pkg/publisher/publisher_test.go` and visually confirm (a) the updated identifier test uses `"recurring-weekly-review-2025W01"`, (b) the period-anchor `Describe` block has nine `It` cases (4 same-period, 4 across-boundary, 1 daily) and the byte-equality `DescribeTable` has five entries (one per `RecurrenceKind`), (c) the unknown-recurrence-kind error test is present.

## Open Questions (for the human reviewer)

- **A. AC #5 wording vs. design intent.** The spec's AC #5 says "For daily entries and any entry whose firing rule is keyed to a specific date (day-of-month, yearly-specific-date), the identifier remains YYYY-MM-DD anchored." But the publisher only receives `def.Recurrence` and `def.Fires`; it never inspects whether a `RecurrenceMonthly` entry's `Fires` predicate is `OnDaysOfMonth(5)` vs. `OnFirstDayOfMonth()`. The design pinned in this prompt anchors purely on `def.Recurrence`: all `RecurrenceMonthly` entries (including day-5 ones like `update-finances`) get a `YYYY-MM` token. The `{{year}}` placeholder in the title continues to render the calendar year, so the vault file is anchored on year, and the controller + vault dedup collapses the multi-fire case inside a year to one task per year. The "yearly-specific-date" entries in the current inventory (e.g. `capitalcom-apikey-prod`, `Recurrence: RecurrenceYearly`, `Fires: OnMonthAndDay(time.May, 1)`) get a `YYYY` token under this design — one identifier per calendar year for that entry. This matches the spec's stated design ("Net effect: a missed tick no longer means a missed period. The vault file path provides the second layer of dedup"). **Reviewer action**: confirm this matches intent, or specify that a future spec should add a "specific-date" detector that branches on `Fires`. If the reviewer wants the latter, this prompt is the wrong shape.
- **B. fmtQuarter single-digit quarter.** `pkg/publisher/render.go` line 77 uses `%dQ%d` (single-digit quarter, e.g. `2025Q2`). The byte-equality test asserts `2025Q2` (not the zero-padded `2025Q02`). This is consistent with the existing formatter and with the spec's "vault file path" wording (existing vault files use the single-digit form). The original Spec 2 prompt proposed zero-padded; the executor of Spec 2 overrode it. This prompt follows the as-built file. No action needed unless the reviewer wants the zero-padded form.
- **C. No scenario file.** The spec explicitly says NO scenario is needed (unit + counterfeiter-mock surface is fully reachable from a YOLO container; no real Kafka, no real vault, no real clock). No `scenarios/` work is part of this prompt.
- **D. The `isoDate` removal.** The previous identifier-input was `"recurring-"+slug+"-"+isoDate(date)`. Removing `isoDate` is part of the rewrite. If any test or downstream file calls `isoDate` (it is unexported, so the blast radius is the same package only), the grep in `<verification>` step 5 catches it. The package's only caller of the identifier path is `publisher.Publish`, which is updated in §2.
- **E. `buildTaskIdentifier` error path testing.** The new error-returning signature is covered by the unknown-recurrence-kind test (§5). All other paths (slug-only, date-only, recurrence-only happy paths) are covered by the existing tests + the new period-anchor table. The boundary contract test (which calls `cmd.Validate(ctx)`) continues to assert the command is well-formed for the period-anchored identifier too — this is a free level-1 test that the upstream cqrs layer will accept the new identifier shape.

</verification>
