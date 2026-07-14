---
status: completed
spec: [016-ondate-recurrence-kind]
summary: Added RecurrenceOnDate period token case (YYYY via fmtYear, no PeriodOffset) to publisher's periodTokenBuilder.Build and added Ginkgo specs covering the token value and boundary contract Validate path
execution_id: recurring-task-creator-ondate-recurrence-exec-031-spec-016-publisher-token
dark-factory-version: dev
created: "2026-07-14T12:00:00Z"
queued: "2026-07-14T12:27:27Z"
started: "2026-07-14T12:50:39Z"
completed: "2026-07-14T12:56:00Z"
branch: dark-factory/ondate-recurrence-kind
---

<summary>
- Teaches the publisher's period-token builder about the new `OnDate` kind: its token is the fire date's 4-digit year, exactly like the yearly kind.
- Because the token is once-per-year, replaying an `OnDate` schedule within the same year produces the same task identifier — so downstream dedup collapses it to a single materialized file.
- No `PeriodOffset` shifting is applied to `OnDate` (unlike monthly/quarterly/yearly), matching the spec's decision that offset is not supported for this kind.
- No existing kind's token format changes — this is a single additional case in the token switch.
- A Ginkgo spec proves the `OnDate` token for a March-15-2027 fire date is `"2027"`.
</summary>

<objective>
Make the publisher's period-token builder return the fire date's 4-digit year (`"YYYY"`) for `RecurrenceOnDate`, matching the `Yearly` token shape and using the existing year-formatting helper, so `OnDate` schedules dedup once per year. `PeriodOffset` is not applied to `OnDate`.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 2 of 3 for spec 016. It DEPENDS ON prompt 1 having landed: `schedule.RecurrenceOnDate` must exist. Guard: if `grep -qn 'RecurrenceOnDate' /workspace/pkg/schedule/recurrence.go` returns no match (exit non-zero), prompt 1 has NOT landed — STOP and report `status: failed` with summary "schedule core (prompt 1) not yet deployed — RecurrenceOnDate undefined".

Read these files fully before changing anything:
- `/workspace/pkg/publisher/period_token.go` — the `periodTokenBuilder.Build(ctx, def, date)` method with the `switch def.Recurrence` you extend. Note the existing `case schedule.RecurrenceYearly:` which does `shifted := base.AddDate(def.PeriodOffset, 0, 0); return PeriodToken(fmtYear(shifted.Year())), nil`. The `default:` returns an "unknown recurrence kind" error. `OnDate` must be handled BEFORE the default so it does NOT fall into the error branch.
- `/workspace/pkg/publisher/render.go` — the `fmtYear(year int) string` helper (renders `"%04d"`). This is the year-formatting helper the Yearly case uses; OnDate reuses it.
- `/workspace/pkg/publisher/publisher_test.go` — `package publisher_test`. Note the `It("non-weekly kinds ignore the Weekday field ...")` table (around the `RecurrenceYearly, NewDate(2025, time.January, 1), "2025"` entry) and the many `publisher.NewPeriodTokenBuilder().Build(context.Background(), def, date)` call sites. Add the OnDate token spec here. Test code may use `context.Background()` (exempt from the business-logic ban).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` (only relevant if you touch an error path; OnDate does not add one).
</context>

<requirements>

### 1. Add the `RecurrenceOnDate` case to `Build`

In `/workspace/pkg/publisher/period_token.go`, in `periodTokenBuilder.Build`, add a case for `schedule.RecurrenceOnDate` that returns the fire date's 4-digit year via `fmtYear`. Place it after the `case schedule.RecurrenceYearly:` and before `default:`:

```go
	case schedule.RecurrenceYearly:
		shifted := base.AddDate(def.PeriodOffset, 0, 0)
		return PeriodToken(fmtYear(shifted.Year())), nil
	case schedule.RecurrenceOnDate:
		// OnDate fires on one fixed month-and-day per year; its period token
		// is that year — once-per-year dedup, matching Yearly's token shape.
		// PeriodOffset is NOT applied to OnDate (the CRD CEL rule keeps
		// periodOffset valid only for Monthly/Quarterly/Yearly), so the raw
		// fire date's year is used directly.
		return PeriodToken(fmtYear(date.Year)), nil
	default:
		return "", errors.Errorf(
			ctx,
			"PeriodTokenBuilder.Build: unknown recurrence kind %q",
			def.Recurrence,
		)
```

Use `date.Year` (the civil `schedule.Date`'s year field) directly — do NOT apply `def.PeriodOffset`, and do NOT go through `base.AddDate(...)`. This keeps `OnDate` offset-free per the spec. `date.Year` is the exported field on `schedule.Date` (see `/workspace/pkg/schedule/date.go`); `fmtYear` formats it as `"%04d"`.

Do NOT change the `RecurrenceYearly` case or any other existing case. Do NOT change any period-token format.

### 2. Add a Ginkgo spec proving the OnDate token is the year

In `/workspace/pkg/publisher/publisher_test.go`, add a spec proving the period token for `(RecurrenceOnDate, fire date 2027-03-15)` is `"2027"`. Call the builder directly (mirror the existing direct-`Build` call sites):

```go
It("builds the OnDate period token as the fire date's 4-digit year", func() {
	def := schedule.TaskDefinition{
		Slug:          "birthday-alice",
		TitleTemplate: "t",
		Recurrence:    schedule.RecurrenceOnDate,
		Month:         time.March,
		Day:           15,
	}
	tok, err := publisher.NewPeriodTokenBuilder().
		Build(context.Background(), def, schedule.NewDate(2027, time.March, 15))
	Expect(err).NotTo(HaveOccurred())
	Expect(string(tok)).To(Equal("2027"))
})
```

Place it inside the existing top-level `Describe("Publisher", ...)` block (or alongside the other direct-`Build` specs). Keep every existing spec unchanged and passing.

Also add OnDate to the existing exhaustive-over-kinds boundary table so the OnDate publish path traverses `task.CreateCommand.Validate` like every other kind: in `/workspace/pkg/publisher/publisher_test.go`, find the `Describe("boundary contract")` → `DescribeTable("produced command passes task.CreateCommand.Validate")` block (it has one `Entry(...)` per kind, daily…yearly) and add `Entry("ondate", schedule.RecurrenceOnDate)` (match the exact arg shape of the existing entries — if they pass extra fixture fields like Month/Day for date-anchored kinds, supply `Month: time.March, Day: 15` so the OnDate row is valid). This closes the only coverage gap: without it, the OnDate path is the one kind absent from the table meant to be exhaustive.

</requirements>

<constraints>
- Do NOT change any existing period-token format (`YYYY-MM-DD`, `YYYYWww`, `YYYYWww-<wd>`, `YYYY-MM`, `YYYYQq`, `YYYY` stay exactly as they are). This prompt adds ONE new case only.
- Do NOT apply `PeriodOffset` to `OnDate` — use `date.Year` directly, no `AddDate` shift. (Spec Non-goal.)
- The OnDate token MUST equal the fire date's year formatted `"%04d"` via the existing `fmtYear` helper — do NOT introduce a new formatting helper.
- Do NOT add any config knob, env var, or tunable threshold. (Spec Non-goals.)
- Do NOT modify the `RecurrenceYearly` case or the `default:` error branch (beyond inserting the new case before it).
- License headers (BSD-2-Clause) on every modified `.go` file.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (this prompt adds none); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg/publisher` package; the new OnDate case must be exercised by the added spec.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the OnDate case is present in the token builder:

```bash
cd /workspace && grep -n 'case schedule.RecurrenceOnDate' pkg/publisher/period_token.go
cd /workspace && grep -n 'fmtYear(date.Year)' pkg/publisher/period_token.go
```

Run the publisher specs verbosely and confirm the OnDate token spec passes (token `"2027"`):

```bash
cd /workspace && go test -v ./pkg/publisher/
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
