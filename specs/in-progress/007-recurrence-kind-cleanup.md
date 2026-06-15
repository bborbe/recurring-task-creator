---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-15T20:53:12Z"
generating: "2026-06-15T20:55:44Z"
prompted: "2026-06-15T21:12:28Z"
verifying: "2026-06-15T21:39:14Z"
branch: dark-factory/recurrence-kind-cleanup
---

## Summary

- Drop the dead `Fires` predicate field from every inventory entry. Spec 6 (period-anchored fire) stopped consulting it; it now only misleads readers.
- Add a generic `Weekday time.Weekday` field on each entry. It is consulted ONLY for weekly entries; ignored (and conventionally zero) for the other four recurrence kinds.
- Change the weekly period token from `YYYYWww` to `YYYYWww-<lowercase-3-letter-weekday>` (e.g. `2026W25-sat`, `2026W25-sun`) so weekly entries that conceptually belong to different days of the same ISO week get distinct identifiers.
- Set `Weekday` on the 21 existing weekly inventory entries (12 Saturday + 9 Sunday). Slugs are frozen; weekly UUID5 identifiers change once at deploy time — same accepted cost as Spec 6.
- Delete the dead predicate primitives and the inventory-lookup-by-date plumbing that only existed to serve the dead `Fires` field, including switching the `/trigger` HTTP path to read the full inventory (mirroring the tick post-Spec-6).

## Problem

Spec 6 made the hourly tick publish the full inventory every tick and derive idempotency from a period-anchored token (`YYYYWww`, `YYYY-MM`, etc.) instead of from the per-entry `Fires` predicate. The predicate field is now read by exactly one production path — `schedule.TasksForDate`, invoked only by the `/trigger?date=` replay handler — and even that consumer wants full-inventory semantics now that the tick has them. Meanwhile, the weekly token collapses 21 entries (12 Saturday, 9 Sunday) onto one identifier-per-ISO-week, erasing the day-of-week distinction that the entries' titles and bodies still carry (`"Weekly Review 2026W25"` vs `"Bot is Healthy 2026W25"` are different work that happens on different days). The result: dead data that misleads readers about what controls firing, and a period token that under-distinguishes weekly entries.

## Goal

After this work:

- `TaskDefinition` carries no firing predicate. It carries a `Weekday` field that is meaningful only when `Recurrence == RecurrenceWeekly`.
- The weekly period token encodes the weekday, so two weekly entries with the same slug-free ISO week but different weekdays produce different identifiers.
- No dead predicate-builder helpers, no schedule-by-date lookup plumbing, and no inventory-iteration code survive that only existed to feed the now-removed `Fires` field.
- The `/trigger?date=` handler iterates the same set of entries the tick does (full inventory) and derives identifiers exactly the way the tick does.

## Non-goals

- Do NOT add a new `RecurrenceKind` value. The closed enum stays at 5: daily / weekly / monthly / quarterly / yearly.
- Do NOT rename or split slugs. Slugs are frozen per project convention.
- Do NOT introduce a per-entry "skip this entry" feature flag or any other opt-out from publication. The tick publishes the full inventory; that's the invariant.
- Do NOT add the weekday suffix to non-weekly tokens. Daily / monthly / quarterly / yearly tokens are unchanged.
- Do NOT add new weekdays beyond Saturday and Sunday to the existing inventory in this spec. Existing entries get their literal weekday; the `Weekday` field is generic for future use, but no new entries are added here.
- Do NOT preserve the deprecated predicate helpers behind an alias or compatibility shim — delete them outright.

## Desired Behavior

1. The `TaskDefinition` shape has no `Fires` field and has a `Weekday time.Weekday` field. The `Weekday` field's GoDoc states explicitly that it is consulted only for `RecurrenceWeekly` entries and is ignored for all other kinds.
2. For `Recurrence == RecurrenceWeekly`, the publisher's period token is `<ISO-year>W<2-digit-ISO-week>-<3-letter-lowercase-weekday>` where the weekday abbreviation is one of `mon` / `tue` / `wed` / `thu` / `fri` / `sat` / `sun`, taken from the entry's `Weekday` field (NOT from the date passed to the publisher).
3. For the four non-weekly kinds, the period token is exactly what Spec 6 produced: `YYYY-MM-DD` for daily, `YYYY-MM` for monthly, `YYYYQq` for quarterly, `YYYY` for yearly. No suffix, no change.
4. Every weekly entry in the inventory has `Weekday` set to a non-zero value (Saturday or Sunday for current entries). Every non-weekly entry has `Weekday` unset (zero value, `time.Sunday` — see Failure Modes for the disambiguation rule).
5. The 12 Saturday-conceptual weekly entries have `Weekday: time.Saturday`; the 9 Sunday-conceptual weekly entries have `Weekday: time.Sunday`. Slugs are unchanged.
6. The `/trigger?date=YYYY-MM-DD` HTTP path iterates the same entry set the hourly tick iterates (full inventory), in slug-sorted order, and calls `publisher.Publish` for each with the parsed civil date. Per-entry errors continue to accumulate in the response (no short-circuit), and the response shape is unchanged.
7. The schedule package exposes no `OnWeekdays`, `OnDaysOfMonth`, `OnMonthAndDay`, `EveryDay`, `OnFirstDayOfQuarter`, `OnFirstDayOfYear`, `OnFirstDayOfMonth`, or `onWeekdayDay5OfMonth` symbol. The `predicate` type, `ScheduleLookup` type, and `TasksForDate` function are removed if and only if no production code or test still depends on them; the verification step makes this binary.

## Constraints

- Project DoD applies: `docs/dod.md` (Ginkgo v2 / Gomega, `bborbe/errors` 3-arg `Wrap`, no `context.Background()` in business logic, no `time.Time` / `time.Now()` in business logic, GoDoc on exports, `make precommit` clean).
- `RecurrenceKind` stays a closed enum of exactly 5 values; `AllRecurrenceKinds` stays in declaration order.
- Slugs are frozen. The change to weekly UUID5 identifiers is the accepted one-time deploy cost (matches Spec 6's accepted cost). No backward-compat layer for the old token format.
- The UUID5 namespace `uuidNamespace` in `pkg/publisher/uuid_namespace.go` is frozen — do NOT regenerate, replace, or alias it.
- Non-weekly period tokens are frozen by Spec 6 (`specs/completed/006-period-anchored-uuid.md`). This spec changes only the weekly format.
- The 3-letter weekday abbreviation is lowercase to match the project's lowercase-suffix style. The uppercase `W` in `YYYYWww` is preserved.
- The `/trigger?date=` response JSON shape (`date`, `published`, `errors`) is frozen by Spec 5. Only the set of entries iterated changes; the shape does not.
- Coding guides apply: `~/Documents/workspaces/coding/docs/go-factory-pattern.md`, `go-error-wrapping-guide.md`, `go-testing-guide.md`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Inventory entry has `Recurrence == RecurrenceWeekly` and `Weekday` left at the zero value (`time.Sunday`) by mistake | Inventory validation test fails at build time. The validation test enumerates weekly entries and requires the `Weekday` field to match the entry's intended day; ambiguity between "intended Sunday" and "default zero" is resolved by an explicit allow-list of Sunday slugs in the validation test (the 9 listed Sunday slugs are the only ones whose `Weekday` may equal `time.Sunday`). | Set `Weekday` explicitly on the new entry; rebuild. |
| Inventory entry has `Recurrence != RecurrenceWeekly` and a non-zero `Weekday` value | Inventory validation test fails at build time: non-weekly entries must leave `Weekday` at the zero value (`time.Sunday`). | Remove the `Weekday` assignment from the entry; rebuild. |
| Publisher is asked to build a period token for a `RecurrenceKind` outside the closed enum | `buildPeriodToken` returns an error wrapped with the slug (Spec 6 behavior, unchanged). | Add the new kind to the closed enum AND extend the publisher's switch — two file edits, one spec; do not paper over with a default branch. |
| Existing vault files keyed by the previous `2026Www` identifier remain after deploy | Orphan files persist in the user's vault. They are NOT deleted by this service (no vault writes from this service per DoD). The user removes them manually or leaves them. | Out of scope for this spec; matches Spec 6's accepted cost. |
| Two weekly entries with the same slug and the same `Weekday` exist | Build-time test failure: existing canonical-slugs uniqueness test already enforces slug uniqueness, so this collapses to the existing failure mode. No new check needed. | Rename one entry (separate spec, given the slug freeze). |

## Security / Abuse Cases

The change is internal-structural; the only external surface touched is the `/trigger?date=` handler, whose behavior is broadened from "entries firing on the parsed date" to "every entry in the inventory, evaluated against the parsed date". The handler still:

- Has no authentication (cluster-internal-only deployment, per Spec 5 and existing GoDoc).
- Returns HTTP 400 for missing or malformed `date`, HTTP 200 with per-task errors in the body otherwise.
- Is idempotent under replay because every Publish derives a deterministic UUID5 from the period token.

Risk introduced by widening the iteration set: replaying `/trigger?date=2026-06-13` (a Saturday) will now produce publish attempts for both Saturday and Sunday weekly entries (and for every non-weekly entry), where previously only Saturday weeklies would publish. Because identifiers are period-anchored, this still produces the same identifiers the hourly tick would produce on that same date, so the controller's de-dup absorbs the redundancy. No new abuse vector; the surface remains as exposed as it was.

## Acceptance Criteria

- [ ] `grep -RnE "\\bFires\\b" pkg/ main.go` returns no matches — evidence: empty grep output (exit code 1).
- [ ] `grep -RnE "OnWeekdays|OnDaysOfMonth|OnMonthAndDay|EveryDay|OnFirstDayOfQuarter|OnFirstDayOfYear|OnFirstDayOfMonth|onWeekdayDay5OfMonth" pkg/ main.go` returns no matches — evidence: empty grep output (exit code 1).
- [ ] `grep -n "Weekday time.Weekday" pkg/schedule/task_definition.go` returns line >=1 — evidence: matched line containing the field declaration with GoDoc above it stating it is consulted only for `RecurrenceWeekly`.
- [ ] `grep -nE "Weekday:\\s+time\\.Saturday" pkg/schedule/inventory.go | wc -l` reports exactly `12` — evidence: line count.
- [ ] `grep -nE "Weekday:\\s+time\\.Sunday" pkg/schedule/inventory.go | wc -l` reports exactly `9` — evidence: line count.
- [ ] A Ginkgo spec in `pkg/publisher` exercising `buildPeriodToken` with `(RecurrenceWeekly, schedule.NewDate(2026, time.June, 17) [Wednesday in ISO week 2026W25], Weekday=time.Saturday)` returns `"2026W25-sat"` — evidence: passing test name printed by `go test -v` for that spec.
- [ ] A Ginkgo spec in `pkg/publisher` exercising `buildPeriodToken` for non-weekly kinds returns the unchanged tokens from Spec 6 (`YYYY-MM-DD`, `YYYY-MM`, `YYYYQq`, `YYYY`) — evidence: passing test name printed by `go test -v`.
- [ ] A Ginkgo spec in `pkg/schedule` enumerates every weekly inventory entry and asserts the `Weekday` field is in `{time.Saturday, time.Sunday}` — evidence: passing test name printed by `go test -v`.
- [ ] The Sunday-weekly allow-list is declared as a named test-file constant `sundayWeeklyAllowList` (e.g. `var sundayWeeklyAllowList = []string{"check-bot-is-healthy", "complete-longhorn-backups", ...}`) — evidence: `grep -c 'sundayWeeklyAllowList' pkg/schedule/inventory_validation_test.go` returns ≥ 2 (declaration + at least one use), AND a Ginkgo `It` asserts `len(sundayWeeklyAllowList) == 9` so an accidental addition/removal of a Sunday slug fails the test.
- [ ] A Ginkgo spec in `pkg/schedule` asserts that for every non-weekly inventory entry, `Weekday` equals the zero value (`time.Sunday`) AND the slug is NOT in `sundayWeeklyAllowList` — evidence: passing test name printed by `go test -v`.
- [ ] A Ginkgo spec in `pkg/handler` for `/trigger?date=2026-06-13` asserts `published` equals the size of `schedule.Inventory()` and the errors array is empty (with a fake publisher that never errors) — evidence: passing test name printed by `go test -v`.
- [ ] `make precommit` exits 0 from the repo root — evidence: exit code 0.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-sat-sun-weekly
make precommit
grep -RnE "\bFires\b|OnWeekdays|OnDaysOfMonth|OnMonthAndDay|EveryDay|OnFirstDayOfQuarter|OnFirstDayOfYear|OnFirstDayOfMonth|onWeekdayDay5OfMonth" pkg/ main.go
```

Expected: `make precommit` exits 0. The `grep` exits with status 1 and prints nothing.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Shape change + token derivation + dead code removal: drop `Fires` from `TaskDefinition`, add `Weekday`, update `buildPeriodToken` weekly branch to append the lowercase 3-letter suffix from the def's `Weekday`, delete `predicate.go`, delete `lookup.go`, delete `tasks_for_date.go`, delete `onWeekdayDay5OfMonth`, and switch the `/trigger` handler + factory wiring to use `schedule.Inventory()` instead of `schedule.TasksForDate` / `ScheduleLookup`. Update or delete the corresponding tests. | 1, 2, 3, 6, 7 | 1, 2, 3, 6, 7, 10, 11 | — |
| 2 | Inventory data + validation tests: set `Weekday: time.Saturday` on the 12 Saturday weekly entries and `Weekday: time.Sunday` on the 9 Sunday weekly entries, add the weekly-Weekday validation spec and the non-weekly-Weekday-must-be-zero spec (with the explicit Sunday-slug allow-list to disambiguate Sunday from the zero value). | 4, 5 | 4, 5, 8, 9, 11 | prompt 1 |

Rationale: prompt 1 reshapes the type, the publisher, and the handler in lock-step (all touch the `TaskDefinition` struct or its consumers, so they must compile together). Prompt 2 fills in the data and adds the inventory invariants that lock the new shape down. Splitting differently (e.g. handler in its own prompt) would leave prompt 1 with a build break because the handler still references the removed `ScheduleLookup` type.

## Do-Nothing Option

If we don't do this: `TaskDefinition` keeps a misleading `Fires` field that the hourly tick ignores, and seven dead predicate constructors stay in `predicate.go` confusing future readers about what controls firing. Worse, the weekly period token continues to collapse Saturday and Sunday weekly entries into a single ISO-week identifier, so a future spec that wants per-weekday semantics (e.g. surfacing only Sunday entries on Sundays in a downstream view) has no key to filter on. The current approach is not actively broken — the tick publishes every entry every hour and the controller de-dups — but it accumulates carry cost on every future change to the schedule package and locks out per-weekday addressing.

## Verification Result

**Verified:** 2026-06-15T21:43:12Z (HEAD 7c95026)
**Binary:** installed dark-factory (host repo: bborbe/recurring-task-creator)
**Scenario:** Walked the 11 ACs against the working tree on `feature/sat-sun-weekly`: ran the required greps, executed `go test -v` for `pkg/publisher`, `pkg/schedule`, `pkg/handler`, and ran `make precommit` from the repo root.
**Evidence:**
- `grep -RnE "\bFires\b" pkg/ main.go` → exit 1, no output (AC 1)
- `grep -RnE "OnWeekdays|OnDaysOfMonth|OnMonthAndDay|EveryDay|OnFirstDayOfQuarter|OnFirstDayOfYear|OnFirstDayOfMonth|onWeekdayDay5OfMonth" pkg/ main.go` → exit 1 (AC 2)
- `pkg/schedule/task_definition.go:32` `Weekday time.Weekday` with GoDoc "consulted ONLY when Recurrence == RecurrenceWeekly" (AC 3)
- `grep -nE "Weekday:\s+time\.Saturday" pkg/schedule/inventory.go | wc -l` = 12; Sunday = 9 (AC 4, 5)
- `pkg/publisher` suite: 48/48 specs PASS, including `buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday` (publisher_test.go:277, asserts `(RecurrenceWeekly, 2026-06-17, Weekday=time.Saturday) → "2026W25-sat"`) and `non-weekly kinds ignore the Weekday field (token is identical to Spec 6)` (publisher_test.go:302) (AC 6, 7)
- `pkg/schedule` suite: 8/8 specs PASS, including `every weekly entry has Weekday in {time.Saturday, time.Sunday}` (line 89), `has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList` with `Expect(sundayWeeklyAllowList).To(HaveLen(9))` (line 86), and `every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList` (line 103) (AC 8, 9, 10)
- `pkg/handler` suite: 12/12 specs PASS, including `responds 200 with date, published=N, errors=[] when all publishes succeed` asserting `body.Published == 45 == len(schedule.Inventory())` and `body.Errors` empty (AC 11)
- `make precommit` → exit 0 (AC 12)
**Verdict:** PASS
