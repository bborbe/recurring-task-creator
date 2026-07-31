---
status: completed
summary: Fix periodTokenBuilder.Build month-end normalization bug by anchoring RecurrenceMonthly/Quarterly/Yearly offset shifts to day-1 of the fire month before calling AddDate
execution_id: recurring-task-creator-period-token-monthend-exec-033-fix-period-token-month-end-offset
dark-factory-version: v0.192.9
created: "2026-07-31T00:00:00Z"
queued: "2026-07-31T19:56:13Z"
started: "2026-07-31T20:02:40Z"
completed: "2026-07-31T20:08:06Z"
---

<summary>
- Fixes a bug where a recurring monthly, quarterly, or yearly task scheduled with a "prior period" offset skips or duplicates the intended month/quarter/year when it fires on a day near month-end (29th, 30th, 31st).
- Example: a "review last month" task that fires every day of July, when it fires on July 31, currently names itself "2026-07" (the current month) instead of "2026-06" (the intended prior month) — the same failure mode can also occur on other day-29/30/31 fire dates, but only when the shifted target month happens to have fewer days than the fire month (e.g. March 31 → Feb, May 31 → Apr), not on every such date.
- Real-world impact: this already produced a duplicate "Review Month" task and, worse, silently blocked the real next month's review task from ever being created because both months' reviews collide on the same deduplication identifier.
- After the fix, the named token always reflects the correct target period regardless of which day of the month the task happens to fire on.
- Daily, weekly, weekday, and fixed-calendar-date recurring tasks are completely unaffected — this only touches the month/quarter/year offset math.
- New automated tests lock in the correct behavior for month-end dates, year-boundary crossings, and a leap-year edge case, so this class of bug cannot silently reappear.
</summary>

<objective>
Fix `periodTokenBuilder.Build` in `pkg/publisher/period_token.go` so that the `PeriodOffset` shift for `RecurrenceMonthly`, `RecurrenceQuarterly`, and `RecurrenceYearly` produces the correct target period regardless of the fire date's day-of-month, by anchoring to the first of the fire date's month before calling `AddDate`. Today, `AddDate` is called directly on the full fire date (e.g. `2026-07-31`), and Go's `AddDate` normalizes an overflowing intermediate day (e.g. "June 31") forward into the next month, producing a wrong period token on day-29/30/31 fire dates. This has already caused a duplicate + a suppressed real task in production via `bborbe/quant`'s `monthly-review` schedule.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:
- `/workspace/pkg/publisher/period_token.go` — the `periodTokenBuilder.Build(ctx, def, date)` method you are fixing. The three cases to change are `case schedule.RecurrenceMonthly:`, `case schedule.RecurrenceQuarterly:`, and `case schedule.RecurrenceYearly:`. Do NOT touch `case schedule.RecurrenceDaily:`, `case schedule.RecurrenceWeekly:`, `case schedule.RecurrenceWeekday:`, `case schedule.RecurrenceOnDate:` (including its comment explaining why `PeriodOffset` is deliberately not applied there), or the `default:` error branch.
- `/workspace/pkg/publisher/render.go` — the formatting helpers used by `Build`: `fmtMonthYear(year, month int) string` (renders `YYYY-MM`), `fmtQuarter(year, quarter int) string` (renders `YYYYQN`), `fmtYear(year int) string` (renders `YYYY`), and `quarterOf(m time.Month) int` (returns 1..4). Do not change any of these helpers or their output format.
- `/workspace/pkg/schedule/date.go` — `type Date struct { Year int; Month time.Month; Day int }` and `func (d Date) Time() time.Time` (midnight-UTC carrier, no timezone ambiguity). `base := date.Time()` in `Build` uses this.
- `/workspace/pkg/publisher/publisher_test.go` — the existing `DescribeTable("applies PeriodOffset to period token (and to UUID5 input)", ...)` inside `Describe("title suffix", ...)` (search for that string). This is the existing offset test table; note that every existing `Entry` in it uses a fire date on the 1st or 15th of the month — none exercise a day-29/30/31 fire date, which is exactly why this bug shipped uncaught. Do not remove or change any existing `Entry` in this table; you may add new ones here or in a new dedicated test file (see requirement 3).
- `/workspace/pkg/publisher/task_identifier_test.go` and `/workspace/pkg/publisher/publisher_suite_test.go` — for the `package publisher_test` import style, the `projmocks "github.com/bborbe/recurring-task-creator/mocks"` alias pattern, and the Ginkgo suite bootstrap (`TestSuite` calls `RegisterFailHandler(Fail)` and `RunSpecs`); `pkg/publisher/period_token.go` currently has NO dedicated test file — you are creating the first one.
- `/workspace/pkg/schedule/task_definition.go` — the `PeriodOffset int` field on `TaskDefinition` (around the comment "PeriodOffset shifts the period-anchored token by N periods. Default 0").
- `/workspace/CHANGELOG.md` — append a `fix:` bullet under `## Unreleased` (create that section at the top, directly under the `# Changelog` preamble and above the current `## v0.10.1` entry, if it does not already exist).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable` / `Entry` table-test style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` wrapping style (only relevant if you touch an error path; this fix does not add one).
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — Keep a Changelog conventions.
</context>

<requirements>

### 1. Fix the month-end normalization bug in `Build`

In `/workspace/pkg/publisher/period_token.go`, in `periodTokenBuilder.Build`, change the `RecurrenceMonthly`, `RecurrenceQuarterly`, and `RecurrenceYearly` cases so the offset shift is anchored to the first of the fire date's month, in the fire date's original location, before calling `AddDate`. Day-of-month is irrelevant to all three of these tokens (`YYYY-MM`, `YYYYQN`, `YYYY`), so clamping to day 1 is always safe and removes every overflow case — including `RecurrenceYearly`'s Feb-29 edge, where shifting a leap day by a non-multiple-of-4 year offset can also land on a nonexistent Feb-29.

Current code:
```go
	case schedule.RecurrenceMonthly:
		shifted := base.AddDate(0, def.PeriodOffset, 0)
		return PeriodToken(fmtMonthYear(shifted.Year(), int(shifted.Month()))), nil
	case schedule.RecurrenceQuarterly:
		shifted := base.AddDate(0, def.PeriodOffset*3, 0)
		return PeriodToken(fmtQuarter(shifted.Year(), quarterOf(shifted.Month()))), nil
	case schedule.RecurrenceYearly:
		shifted := base.AddDate(def.PeriodOffset, 0, 0)
		return PeriodToken(fmtYear(shifted.Year())), nil
```

Replace with a version that first clamps `base` to the first of its month (preserving `base.Location()`), then shifts:
```go
	case schedule.RecurrenceMonthly:
		firstOfMonth := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location())
		shifted := firstOfMonth.AddDate(0, def.PeriodOffset, 0)
		return PeriodToken(fmtMonthYear(shifted.Year(), int(shifted.Month()))), nil
	case schedule.RecurrenceQuarterly:
		firstOfMonth := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location())
		shifted := firstOfMonth.AddDate(0, def.PeriodOffset*3, 0)
		return PeriodToken(fmtQuarter(shifted.Year(), quarterOf(shifted.Month()))), nil
	case schedule.RecurrenceYearly:
		firstOfMonth := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location())
		shifted := firstOfMonth.AddDate(def.PeriodOffset, 0, 0)
		return PeriodToken(fmtYear(shifted.Year())), nil
```

You may factor the repeated `time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location())` clamp into a small unexported helper (e.g. alongside the other leaf helpers in `/workspace/pkg/publisher/render.go`, following the style of the existing `firstOfPreviousMonth` helper already in that file) if that better matches the file's existing pattern — either inline per-case or a shared helper is acceptable, as long as all three cases anchor to day-1 before shifting and the token output is identical to the tables below. Note: unlike `firstOfPreviousMonth`, which hardcodes `time.UTC`, your new helper (or inline clamp) MUST preserve `base.Location()` — do not copy that part of the exemplar's style.

Do NOT change `case schedule.RecurrenceDaily:`, `case schedule.RecurrenceWeekly:`, `case schedule.RecurrenceWeekday:`, or `case schedule.RecurrenceOnDate:` (including its comment about `PeriodOffset` not applying to it). Do NOT change the `default:` error branch. Do NOT change any of the format helpers in `render.go`.

### 2. Verify against the confirmed-bug table

The fixed code must produce these exact tokens for `PeriodOffset: -1`, `RecurrenceMonthly` (this table documents the bug and its fix — use it to derive your test entries in requirement 3, not as inline requirements text to paste as-is):

| fire date | correct token |
|---|---|
| 2026-07-01 | `2026-06` |
| 2026-07-30 | `2026-06` |
| 2026-07-31 | `2026-06` |
| 2026-03-31 | `2026-02` |
| 2026-05-31 | `2026-04` |
| 2026-08-31 | `2026-07` |

### 3. Add a dedicated test file: `pkg/publisher/period_token_test.go`

`/workspace/pkg/publisher/period_token.go` currently has no dedicated test file (the offset logic today is only covered indirectly via `publisher_test.go` and `task_identifier_test.go`, and none of those existing entries use a day-29/30/31 fire date). Create `/workspace/pkg/publisher/period_token_test.go` in `package publisher_test`, following the Ginkgo v2 / Gomega `Describe` / `DescribeTable` / `Entry` style and import alias conventions used in `/workspace/pkg/publisher/task_identifier_test.go` (import `"context"`, `"time"`, `. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`, and `"github.com/bborbe/recurring-task-creator/pkg/publisher"` / `"github.com/bborbe/recurring-task-creator/pkg/schedule"`; call `publisher.NewPeriodTokenBuilder().Build(context.Background(), def, date)` directly, matching how `publisher_test.go`'s existing direct-`Build` specs and `task_identifier_test.go` call the builder — `context.Background()` is fine in test code per the project DoD).

Cover at minimum, via `DescribeTable` with one `Entry` per row (call `Build` directly and assert on the returned `PeriodToken` and `error`):

- Monthly `PeriodOffset: -1` for every fire date in the table in requirement 2, asserting the CORRECT token in each case: `2026-07-01`→`2026-06`, `2026-07-30`→`2026-06`, `2026-07-31`→`2026-06`, `2026-03-31`→`2026-02`, `2026-05-31`→`2026-04`, `2026-08-31`→`2026-07`. Note: `2026-07-01`, `2026-07-30`, and `2026-08-31` are control/regression entries — they already produce the correct token under the OLD unfixed code (the target month happens to have enough days); `2026-07-31`, `2026-03-31`, and `2026-05-31` are the entries that actually discriminate old vs. fixed behavior (target month shorter than fire month, so the old code overflows into the wrong month).
- Monthly `PeriodOffset: 0` on `2026-07-31` → token must equal `2026-07` (the fire month itself, proving the day-1 anchor doesn't change zero-offset behavior).
- Monthly `PeriodOffset: -1` crossing a year boundary on a 31st: `2026-01-31` → `2025-12` (control/regression entry — December always has 31 days so this does not discriminate old vs. fixed behavior, but it locks in correct year-rollover formatting).
- Quarterly `PeriodOffset: -1`, discriminating case: `2026-12-31` → `2026Q3` (old code shifts Dec 31 by -3 months to nonexistent "Sep 31", which Go normalizes forward to Oct 1 → wrong quarter `2026Q4`; fixed code anchors to Sep 1 → correct `2026Q3`). Also include control/regression entries `2026-07-31` → `2026Q2` and `2026-10-31` → `2026Q3` — both already produce the correct token under the OLD code too (April/May are both Q2; July has no overflow at all), so they do not by themselves prove the fix, but they lock in existing correct behavior.
- Yearly `PeriodOffset: -1` on a leap day: `2028-02-29` → `2027` — defensive-hygiene case, not a confirmed production bug: `fmtYear` only reads `.Year()`, and December (the only month whose overflow could push into a following year) always has 31 days, so this exact mechanism cannot produce a wrong year token either before or after the fix. Keep the day-1 anchor on `RecurrenceYearly` anyway (consistency with Monthly/Quarterly, and it avoids ever constructing a nonexistent Feb-29 intermediate value) — this test locks in "doesn't break," not "fixes a bug."
- At least one non-offset-affected kind (Daily or Weekly) on an arbitrary date, proving the untouched branches still produce their existing token shape unchanged.

Use `schedule.TaskDefinition{Slug: "...", Recurrence: schedule.RecurrenceMonthly, PeriodOffset: -1}` (or `Quarterly`/`Yearly` as appropriate) and `schedule.NewDate(year, month, day)` to build each `date` argument, matching the construction style already used in `publisher_test.go`'s offset table.

### 4. CHANGELOG entry

Append a bullet to `/workspace/CHANGELOG.md` under a `## Unreleased` section (create it at the top of the file, directly below the `# Changelog` preamble paragraph and above the current topmost version entry `## v0.10.1`, if `## Unreleased` does not already exist). The bullet must be patch-level and describe the fix in user-visible terms, e.g.:

```
## Unreleased

- fix: anchor `PeriodOffset` month/quarter/year shifting in `periodTokenBuilder.Build` to the first of the fire date's month before applying the offset — previously a day-29/30/31 fire date (e.g. `2026-07-31` with `periodOffset: -1`) could normalize past the intended prior period (Go's `AddDate` rolls a nonexistent "June 31" forward into July), producing the wrong period token, a duplicate task, and a suppressed subsequent task via the shared UUID5 identifier.
```

</requirements>

<constraints>
- Do NOT change any existing period-token format (`YYYY-MM-DD`, `YYYYWww`, `YYYYWww-<wd>`, `YYYY-MM`, `YYYYQN`, `YYYY` stay exactly as they are) — this is an arithmetic fix, not a format change.
- Do NOT change `RecurrenceDaily`, `RecurrenceWeekly`, `RecurrenceWeekday`, or `RecurrenceOnDate` cases, or the `default:` error branch, or any formatting helper in `render.go`.
- Preserve `base.Location()` when constructing the day-1 anchor — do not hardcode `time.UTC` (the existing `base := date.Time()` already carries a fixed location per `schedule.Date.Time()`, but the fix must not silently change it).
- Do NOT add any config knob, env var, feature flag, or tunable threshold. This is a pure bug fix.
- Do NOT remove or alter any existing `Entry` in `publisher_test.go`'s `DescribeTable("applies PeriodOffset to period token (and to UUID5 input)", ...)` table or any other existing passing test.
- License headers (BSD-2-Clause, matching the header already at the top of `period_token.go`) on every new or modified `.go` file.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (this fix touches none); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt); all exported types/functions/methods have GoDoc comments.
- Coverage for the changed `pkg/publisher` package must not regress; the new day-29/30/31 and leap-year cases must be exercised by the added test file.
- Do NOT commit — dark-factory handles git.
- Do NOT deploy, tag, release, or open a PR.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the day-1 anchor is present (3 matches if inlined per-case, or fewer in `period_token.go` plus a helper definition in `render.go` if factored into a shared helper — either is fine):

```bash
cd /workspace && grep -rn 'base.Year(), base.Month(), 1' pkg/publisher/period_token.go pkg/publisher/render.go
```

Confirm the new dedicated test file exists and run it verbosely:

```bash
cd /workspace && test -f pkg/publisher/period_token_test.go && go test -v ./pkg/publisher/ -run TestSuite
```

Confirm the CHANGELOG has an Unreleased entry referencing the fix:

```bash
cd /workspace && grep -n '## Unreleased' CHANGELOG.md
```

Finally, the single command that must exit 0:

```bash
cd /workspace && make precommit
```

If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.
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
