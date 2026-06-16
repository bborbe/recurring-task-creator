---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-06-16T12:01:34Z"
generating: "2026-06-16T12:04:42Z"
prompted: "2026-06-16T12:31:22Z"
branch: dark-factory/weekday-kind-split
---

## Summary

- Split the recurrence-kind enum so per-weekday firing is first-class instead of riding on "weekly".
- Add `RecurrenceWeekday`; keep `RecurrenceWeekly` reserved for true ISO-week always-fire (currently no inventory entries).
- Migrate all 21 existing weekly-with-`Weekday` inventory entries (12 Saturday, 9 Sunday) to `RecurrenceWeekday`.
- Fix the regression where Saturday/Sunday tasks materialize on every weekday since Spec 6's always-fire weekly tick.
- Preserve UUID5 identity for every existing weekday entry so no duplicate vault files appear after deploy.

## Problem

Spec 6 made `RecurrenceWeekly` fire on every day inside the ISO week (parallel to monthly/quarterly/yearly always-fire). Spec 7 added a `Weekday` field to the inventory so weekly entries could carry their target weekday, and Spec 8 baked the per-weekday token (`YYYYWww-<abbrev>`) into titles. But the inventory's 21 "weekly" entries are not actually weekly — they are weekday-specific (12 Saturday, 9 Sunday). Combined with always-fire weekly, `/start-day` on a Tuesday now surfaces `Backup Kafka Topics - 2026W25-sat.md` and 20 other weekend tasks — wrong UX. The enum is overloaded: one kind name is silently doing two different jobs depending on whether `Weekday` is set.

## Goal

The recurrence-kind enum cleanly distinguishes "fires any day this ISO week" (`RecurrenceWeekly`, always-fire) from "fires only on its target weekday" (`RecurrenceWeekday`). The inventory uses each kind for exactly what its name says. `/start-day` on a non-target weekday no longer surfaces weekday-pinned tasks. All 21 existing vault files retain the same identifier and title — zero duplicates after deploy.

## Non-goals

- Adding any `RecurrenceWeekly` (no-weekday) inventory entry — kind is reserved; inventory stays empty until a future use case demands it.
- Deleting `RecurrenceWeekly` from the enum — kept deliberately as a meaningful semantic; do NOT remove.
- Renaming or restructuring any other recurrence kind.
- Changing the always-fire semantic for monthly/quarterly/yearly (Spec 6 behavior preserved).
- Per-vault `TaskDir` or routing changes.
- Vault cleanup of pre-Spec-9 files (none needed — same UUID5, same title).
- Touching frontmatter shape, title-suffix format, namespace, or sender.

## Desired Behavior

1. The enum exposes `RecurrenceWeekday` as a sibling of `RecurrenceWeekly`, `RecurrenceMonthly`, `RecurrenceQuarterly`, `RecurrenceYearly`, and the existing `RecurrenceDaily`.
2. `RecurrenceWeekday` entries fire on a given date if and only if the entry's `Weekday` equals the date's weekday.
3. `RecurrenceWeekly` entries fire on every day inside the entry's ISO week (Spec 6 always-fire semantic, unchanged); no `Weekday` field is consulted for this kind.
4. The inventory contains zero `RecurrenceWeekly` entries after this spec; all 21 prior weekly entries are `RecurrenceWeekday` with `Weekday` unchanged (12 `time.Saturday`, 9 `time.Sunday`).
5. Period-token rendering: `RecurrenceWeekday` produces `YYYYWww-<3-letter-lowercase-weekday-abbrev>` (e.g. `2026W25-sun`); `RecurrenceWeekly` produces bare `YYYYWww` (e.g. `2026W25`).
6. For every existing weekday entry, the UUID5 input string `recurring-<slug>-<period-token>` is byte-identical to what the pre-Spec-9 publisher produced — same identifier, same vault filename, no duplicates.
7. `RecurrenceDaily` remains in the enum, has no inventory entries, and is unchanged.

## Constraints

- Frontmatter shape, title-suffix format, namespace, sender, and per-task `TaskDir` routing are frozen — no changes.
- `Weekday` field semantics: required (non-zero) for `RecurrenceWeekday`; forbidden (zero) for `RecurrenceWeekly`. Other kinds: unchanged.
- The monthly/quarterly/yearly always-fire behavior introduced in Spec 6 must not regress.
- `RecurrenceWeekly` MUST remain in the enum even with zero inventory entries — agent decides at impl time whether to add a compile-time `_ = RecurrenceWeekly` reference to satisfy unused-export linters.
- Spec 8's title-suffix tokens for already-published Saturday/Sunday files must remain byte-identical.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection |
|---------|-------------------|----------|-----------|
| Inventory entry declares `RecurrenceWeekday` with zero/unset `Weekday` | Validation test fails at build time | Fix inventory entry | `make precommit` exits non-zero with failing test name |
| Inventory entry declares `RecurrenceWeekly` with non-zero `Weekday` | Validation test fails at build time | Fix inventory entry | `make precommit` exits non-zero with failing test name |
| Period-token rendering for a weekday entry diverges from `YYYYWww-<abbrev>` | UUID5 changes, duplicate vault file appears on next tick | Revert token logic; pre-existing files keep old UUID, new ones get new UUID until reverted | Vault gains a second file with same title-prefix and different period-token after deploy |
| Tick runs on Tuesday and emits Saturday-pinned tasks | Regression — Spec 9 not in effect | Verify deploy hit prod; rerun `make precommit` to confirm test coverage | `/start-day` output contains `-sat`/`-sun` suffix on a weekday |
| New recurrence kind added later collides with `"weekday"` string | Compile-time enum mismatch | Pick a different string constant | Build fails |

## Security / Abuse Cases

Not applicable — no HTTP surface, no user-controlled input, no new file paths. Inventory is compile-time data.

## Acceptance Criteria

- [ ] `RecurrenceWeekday RecurrenceKind = "weekday"` is exported alongside existing kinds — evidence: `grep -n 'RecurrenceWeekday' pkg/schedule/recurrence.go` returns the constant declaration line.
- [ ] All 12 Saturday inventory entries use `Recurrence: RecurrenceWeekday` with `Weekday: time.Saturday` — evidence: inventory validation test asserts count == 12 and passes under `go test ./pkg/schedule/...` (exit 0).
- [ ] All 9 Sunday inventory entries use `Recurrence: RecurrenceWeekday` with `Weekday: time.Sunday` — evidence: inventory validation test asserts count == 9 and passes under `go test ./pkg/schedule/...` (exit 0).
- [ ] Inventory contains zero `RecurrenceWeekly` entries — evidence: inventory validation test asserts count == 0 and passes; also `grep -c 'RecurrenceWeekly' pkg/schedule/inventory.go` returns 0.
- [ ] Validation test enforces invariant: every `RecurrenceWeekday` entry has non-zero `Weekday`; every `RecurrenceWeekly` entry has zero `Weekday` — evidence: test names appear in `go test -v ./pkg/schedule/...` output with PASS status.
- [ ] `TasksForDate` (or current entry point) on a Tuesday produces zero weekday-kind tasks while monthly/quarterly/yearly fire as before — evidence: unit test in `pkg/schedule` passes (exit 0); test asserts `len(weekdayTasks) == 0` and `len(monthlyTasks) > 0`.
- [ ] `TasksForDate` on a Saturday produces exactly the 12 Saturday weekday-kind tasks and zero Sunday ones — evidence: unit test asserts `len(satTasks) == 12 && len(sunTasks) == 0`, exit 0.
- [ ] `TasksForDate` on a Sunday produces exactly the 9 Sunday weekday-kind tasks and zero Saturday ones — evidence: unit test asserts `len(sunTasks) == 9 && len(satTasks) == 0`, exit 0.
- [ ] `buildPeriodToken(RecurrenceWeekday, 2026-06-21)` returns `"2026W25-sun"` — evidence: unit test in `pkg/publisher` passes (exit 0) with string equality assertion.
- [ ] `buildPeriodToken(RecurrenceWeekly, 2026-06-16)` returns `"2026W25"` (no weekday suffix) — evidence: unit test passes with string equality assertion.
- [ ] UUID5 stability check: for each of the 21 weekday entries, `recurring-<slug>-<period-token>` produced by post-Spec-9 code matches the pre-Spec-9 Spec-8 string byte-for-byte — evidence: table-driven test enumerates all 21 slugs with expected token strings (hand-derived from Spec 8 format) and asserts equality; exit 0.
- [ ] `RecurrenceDaily` remains in the enum, has zero inventory entries — evidence: `grep -n 'RecurrenceDaily' pkg/schedule/recurrence.go` returns declaration line; validation test asserts 0 inventory entries with `RecurrenceDaily`.
- [ ] CHANGELOG `## Unreleased` section contains one bullet describing the kind split — evidence: `grep -n 'weekday' CHANGELOG.md` returns a line under the `## Unreleased` heading.
- [ ] `make precommit` exits 0 — evidence: exit code 0.

## Verification

```
cd /Users/bborbe/Documents/workspaces/recurring-task-creator-weekday-kind
make precommit
go test -v -run 'WeekdayKind|RecurrenceWeekday|RecurrenceWeekly|PeriodToken|UUID5' ./...
grep -c 'RecurrenceWeekly' pkg/schedule/inventory.go    # expect 0
grep -c 'RecurrenceWeekday' pkg/schedule/inventory.go   # expect 21
```

Expected: `make precommit` exits 0; all matched tests PASS; grep counts as annotated.

## Assumptions

- The pre-Spec-9 publisher (Spec 8 in effect) already emits `YYYYWww-<abbrev>` for these 21 entries via the `RecurrenceWeekly`+`Weekday` combo — so migrating to `RecurrenceWeekday` with the same token shape is UUID5-preserving by construction.
- The existing 21 vault files (`Backup Kafka Topics - 2026W25-sat.md` and siblings) remain valid post-deploy without cleanup; their identifiers and titles are unchanged.
- `pkg/schedule/inventory.go`, `pkg/schedule/inventory_validation_test.go`, and `pkg/publisher/publisher.go` are the entry points; the impl agent confirms exact symbol names.
- Specs `006-period-anchored-uuid.md`, `007-recurrence-kind-cleanup.md`, `008-title-period-tokens-and-drop-recurring-frontmatter.md` in `specs/completed/` document the prior decisions this spec builds on.

## Do-Nothing Option

`/start-day` keeps surfacing 21 weekend tasks on every weekday of the ISO week. Users either ignore the noise or manually delete unwanted entries each morning. The enum stays overloaded and the next person reading `Recurrence: RecurrenceWeekly, Weekday: time.Saturday` has to reverse-engineer which semantic applies. Not acceptable — the regression is daily-visible and the fix is small.
