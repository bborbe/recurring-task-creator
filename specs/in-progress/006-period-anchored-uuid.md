---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-06-14T19:58:07Z"
generating: "2026-06-14T19:59:05Z"
prompted: "2026-06-14T20:17:47Z"
branch: dark-factory/period-anchored-uuid
---

## Summary

- Switch every recurring task's deterministic identifier from a date-anchored shape to a period-anchored shape, so weekly/monthly/quarterly/yearly tasks collapse to one identifier per period regardless of which day inside that period the publisher runs.
- Make the hourly tick publish the FULL inventory every hour. Drop the predicate-driven filter that selected a per-day subset.
- Daily, day-of-month, and yearly-specific-date entries remain date-anchored (their identifier is the date they fire on — one fire per specific date).
- One-time controlled break: identifiers for already-existing weekly/monthly/quarterly/yearly tasks change shape on the first tick after deploy. Vault files keyed by old identifiers become orphans and must be closed manually.
- Net effect: a missed tick no longer means a missed period. The vault file path (derived from the rendered title containing the period token, e.g. "Plan Month 2026-06") provides the second layer of dedup.

## Problem

Today the publisher derives each task's identifier from the calendar date the tick fires on, and the hourly tick only publishes the subset of inventory entries whose recurrence predicate matches that date. The two concerns — "which day inside the period do we fire?" and "what identifier dedups this fire across retries?" — are conflated in the same date string. If the pod is unavailable on the single day a monthly task was scheduled to fire, the monthly task is never published for that month at all; the hourly idempotent retry only deduplicates within the same calendar day, not within the same period. The cron predicate (e.g. "OnWeekdays(Saturday)") was originally only meant as a scheduling hint, but it ended up also driving identifier uniqueness — a hidden coupling that turns a one-hour outage on the wrong day into a one-month gap in the user's vault.

## Goal

After this work, the publisher's deterministic identifier for any recurring task is anchored on the **period** the task belongs to, not the day the publisher happens to run. The hourly tick publishes every inventory entry every hour; downstream UUID-based dedup in the controller plus title-based dedup in the vault collapse the duplicates back to one task per period. A missed tick within a period is fully recovered by the next tick in that same period.

## Non-goals

- Do NOT rename existing slugs — slugs remain frozen (separate spec governs slug stability).
- Do NOT change the `RecurrenceKind` enum or its members.
- Do NOT change inventory contents — no edits to titles, body templates, or recurrence assignments.
- Do NOT add a `--date` flag for run-once — that is a separate spec.
- Do NOT migrate or rewrite already-existing vault files keyed by old identifiers — orphans are accepted as the one-time cost.
- Do NOT keep the recurrence predicate as a runtime toggle — removing the predicate-driven filter is the point of this spec; if a future consumer needs predicate-driven filtering, that's a separate spec.

## Desired Behavior

1. The deterministic identifier for a weekly entry on any day inside ISO week `2026W24` is identical to the identifier for the same slug on any other day inside `2026W24`, and differs from `2026W23` and `2026W25`.
2. The deterministic identifier for a monthly entry on any day inside `2026-06` is identical for every day in that month, and differs from `2026-05` and `2026-07`.
3. The deterministic identifier for a quarterly entry on any day inside `2026Q2` is identical for every day in that quarter, and differs from `2026Q1` and `2026Q3`.
4. The deterministic identifier for a yearly entry on any day inside `2026` is identical for every day in that year, and differs from `2025` and `2027`.
5. The deterministic identifier for a daily entry, a day-of-month entry, or a yearly-specific-date entry remains anchored on the firing date (`YYYY-MM-DD`) — one identifier per specific date.
6. The hourly tick iterates the full inventory (currently 45 entries) every hour and calls the publisher once per entry. The previous predicate-driven per-day filter is removed.
7. The identifier-construction input string follows the shape `recurring-<slug>-<period-token>`, where `<period-token>` is the period-anchored token for non-date-anchored kinds and the existing `YYYY-MM-DD` for date-anchored kinds. The UUID5 namespace constant is unchanged.

## Constraints

- The UUID5 namespace constant (`uuidNamespace`) stays byte-identical to its current value — never read from env, never regenerated.
- The period-token format matches the format already used by the rendered title placeholders (see `pkg/publisher/render.go`): `fmtIsoWeek` → `YYYYWNN`, `fmtMonthYear` → `YYYY-MM`, `fmtQuarter` → `YYYYQN`, `fmtYear` → `YYYY`. Reuse those formatters — never introduce a second formatter for the same period.
- Berlin local time governs the period boundary, same as today's tick (the date passed into the publisher is already Berlin-local).
- The publisher's `Publish(ctx, def, date)` external signature is unchanged — period anchoring is computed internally from `def.Recurrence` and `date`.
- Existing tests in `pkg/schedule` that validate inventory contents (canonical slugs, placeholder coverage, no-forbidden-imports) must still pass with no changes to inventory data.
- `pkg/schedule/predicate.go` and `pkg/schedule/tasks_for_date.go` may stay in the tree as library code, but `tick` must no longer call `TasksForDate` for filtering — the tick consumes the full inventory directly. Whether the predicate code is deleted or left as dead code is the implementer's call at impl time (agent decides at impl time).
- Coding guides apply: `~/Documents/workspaces/coding/docs/go-factory-pattern.md`, `~/Documents/workspaces/coding/docs/go-error-wrapping-guide.md`, `~/Documents/workspaces/coding/docs/go-testing-guide.md`.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---------|-------------------|----------|-----------|---------------|-------------|
| Tick is missed for several hours mid-period (pod restart, network blip) | Next tick inside the same period republishes every entry with the same period-anchored identifier; controller dedup collapses duplicates | Automatic on next tick | `recurring_task_creator_last_tick_timestamp` gauge stops advancing; resumes on recovery | Reversible (idempotent) | Two ticks firing concurrently produce identical identifiers — downstream dedup absorbs both |
| First deploy of this change finds vault files keyed by old date-anchored identifiers | New identifiers diverge from the old ones; new vault files are created with the new identifiers; old files remain in the vault as orphans until manually closed | Manual close of orphaned vault files by the operator; no automated migration | Operator observes both old-format and new-format files for the same period after first post-deploy tick | Irreversible without manual cleanup | Not applicable — one-time event |
| Inventory contains a recurrence kind not handled by the period-anchor mapping | Publisher returns a wrapped error for that entry; tick logs and increments the error metric; other entries continue | Fix inventory or extend the mapping in a follow-up spec | `recurring_task_creator_published_total{result="error"}` increments for the affected recurrence label | Reversible (no side effect on failed entry) | Not applicable |
| Clock skew places the host slightly before/after a period boundary | The first tick inside the new period emits new identifiers; the last tick of the old period emits old identifiers; both are correct for their respective periods | None needed — boundary handling is exact per Berlin-local civil date | None required | Reversible (idempotent) | Two ticks straddling the boundary publish two distinct period identifiers — both correct |

## Security / Abuse Cases

Not applicable. This spec touches no HTTP surface, no user-controlled input, no file paths, no trust boundary. Inventory is compiled in; the tick is a closed loop with no external input beyond the wall clock.

## Acceptance Criteria

- [ ] For every entry in the inventory with recurrence `weekly`, the identifier returned by the publisher for any two civil dates `d1` and `d2` inside the same ISO week is equal — evidence: Go table test in `pkg/publisher` asserting equality across at least two distinct dates per period, exit code 0
- [ ] For every entry with recurrence `monthly`, the identifier is equal for any two civil dates inside the same calendar month and differs across adjacent months — evidence: Go table test, exit code 0
- [ ] For every entry with recurrence `quarterly`, the identifier is equal for any two civil dates inside the same quarter and differs across adjacent quarters — evidence: Go table test, exit code 0
- [ ] For every entry with recurrence `yearly`, the identifier is equal for any two civil dates inside the same calendar year and differs across adjacent years — evidence: Go table test, exit code 0
- [ ] For daily entries and any entry whose firing rule is keyed to a specific date (day-of-month, yearly-specific-date), the identifier remains `YYYY-MM-DD` anchored — evidence: Go test asserting two distinct civil dates produce two distinct identifiers, exit code 0
- [ ] The period-token format produced inside the identifier-input string is byte-identical to the corresponding `fmtIsoWeek`/`fmtMonthYear`/`fmtQuarter`/`fmtYear` formatter output — evidence: Go test comparing the substring after `recurring-<slug>-` against the formatter's return value, exit code 0
- [ ] The hourly tick publishes the full inventory every tick. A single `RunOnce` call results in `len(schedule.Inventory())` invocations of `publisher.Publish` regardless of the civil date supplied — evidence: counterfeiter mock `PublishCallCount()` equals the inventory length (derived at test time from `len(schedule.Inventory())`, NOT a hardcoded literal; at spec authoring there are 45 entries but this AC must not regress if the inventory grows) for at least three distinct test dates spanning different weekdays/months, exit code 0
- [ ] `pkg/schedule` inventory-level tests (canonical slugs, inventory validation, no-forbidden-imports) pass with no changes to inventory data — evidence: `go test ./pkg/schedule/...` exit code 0
- [ ] `uuidNamespace` constant value is unchanged — evidence: `grep -n 'f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a' pkg/publisher/uuid_namespace.go` returns line ≥1
- [ ] `make precommit` in the repo root exits 0 — evidence: exit code 0

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-always-fire
make precommit
go test ./pkg/publisher/... ./pkg/tick/... ./pkg/schedule/...
grep -n 'f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a' pkg/publisher/uuid_namespace.go
```

Expected: all `go test` invocations and `make precommit` exit 0; the `grep` returns the namespace constant line.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Period-anchor mapping + identifier builder rewrite in `pkg/publisher` (period-token derivation per `RecurrenceKind`, identifier shape change, publisher-level tests) | 1, 2, 3, 4, 5, 7 | weekly/monthly/quarterly/yearly equality, date-anchored kinds unchanged, format-byte-equality, namespace-unchanged | — |
| 2 | Tick simplification in `pkg/tick` (drop predicate filter, iterate full inventory every tick, tick-level tests) | 6 | full-inventory-published, schedule tests still pass, precommit | prompt 1 |

Rationale: the publisher change is self-contained and must land first because the tick change relies on the publisher's new dedup contract to remain idempotent across the full inventory. Splitting also keeps each PR's diff scoped to a single package's behavior change.

## Do-Nothing Option

If we don't do this, a missed tick on the single calendar day a monthly/quarterly/yearly task was scheduled to fire results in that task never being created for that period. Operators currently work around this by manually creating vault files after outages — acceptable for a one-person system but increasingly expensive as the inventory grows. The hourly idempotent retry inside a single day is not a substitute, because the predicate's per-day window is exactly one day. Doing nothing also leaves the cron predicate doing double duty (scheduling hint AND identifier anchor) — a latent coupling that will resurface every time we add a new recurrence kind.
