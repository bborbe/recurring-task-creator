---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-24T20:19:46Z"
generating: "2026-06-24T20:20:24Z"
prompted: "2026-06-24T20:56:15Z"
verifying: "2026-06-24T21:40:49Z"
branch: dark-factory/weekday-list-and-short-forms
---

## Summary

- Allow the `Weekday` recurrence kind's `weekday` field on the `Schedule` CRD to accept a **list** of weekdays — not just a single string. A list collapses N single-day CRs (e.g. IBKR Swing Trading Mon-Fri = 5 CRs today) into one.
- Each list entry may use either the long form (`Monday`..`Sunday`) or the short form (`Mon`..`Sun`); the two forms may mix freely within one list.
- Backward-compatible: existing single-string usage (`weekday: Monday`) keeps working byte-identically — no migration of in-cluster CRs is forced by this spec.
- Publisher fires once per matching weekday per ISO week — a list `[Mon, Wed, Fri]` produces 3 distinct task files per week, each with its own per-day period token and UUID5 (no dedup collision).
- Duplicates inside a list (including cross-form duplicates like `[Mon, Monday]`) and unknown day strings are rejected by CRD-level CEL validation; list on a non-Weekday recurrence is rejected by the existing `weekday-iff-Weekday` rule.

## Problem

The current `Schedule` CRD encodes the `Weekday` recurrence's target day as a **single** string. Real-world recurring tasks rarely land on exactly one weekday: IBKR Swing Trading runs Mo-Fr, Feed Worms runs Sun+Wed. Migrating these from vault-cli into `Schedule` CRs today requires duplicating the entire CR body 2-5 times — same title, same body template, same vault, only the weekday differs. The CR fan-out is pure noise: it bloats `kubectl get schedules`, multiplies the surface area for typos, and means every body edit has to be replicated across siblings. A list-of-days collapses each multi-day task back to one CR.

## Goal

After this work, a single `Schedule` CR can declare an arbitrary non-empty set of weekdays on which the `Weekday` recurrence fires. The publisher emits one task file per matching weekday per ISO week, with the existing per-day period-token (`YYYYWww-<abbrev>`) and UUID5 identity preserved. Single-string usage continues to work byte-identically (no forced migration). Short-form (`Mon`..`Sun`) and long-form (`Monday`..`Sunday`) day names are both accepted and may be mixed in the same list. The CRD-level CEL validation rejects empty lists, duplicate days (including cross-form), unknown day strings, and any `weekday` field (string or list) on a non-`Weekday` recurrence.

## Non-goals

- Do NOT add a new `Interval` recurrence kind (e.g. "every 3 days") — separate spec.
- Do NOT migrate existing single-day `Weekday` CRs in `bborbe/quant` (or anywhere else) to list form. This is a purely additive change; migration is optional and operator-driven.
- Do NOT change `Daily` / `Weekly` / `Monthly` / `Quarterly` / `Yearly` semantics, schema, or period-token rendering.
- Do NOT change the period-token format (`YYYYWww-<3-letter-lowercase-abbrev>`), the UUID5 derivation, or the CRD's group / version / kind / plural / singular / short name.
- Do NOT change the `RecurrenceKind` enum on the wire (closed at `Daily`, `Weekly`, `Weekday`, `Monthly`, `Quarterly`, `Yearly` per Spec 9). This spec only widens the `weekday` field's type, not the recurrence enum.
- Do NOT introduce a config knob to disable list support or fall back to single-string-only mode. List support is additive and unconditional; if a future consumer demands the old single-only shape, that's a separate spec.
- Do NOT add a CRD `Status` subresource or any status writeback (Spec 8 / Spec 10 boundary preserved).
- Do NOT touch the `/trigger?date=` HTTP request / response contract (Spec 5).

## Desired Behavior

1. The `Schedule` CRD's `spec.schedule.weekday` field accepts either a single string OR a non-empty list of strings. Both shapes are valid; existing single-string CRs continue to apply and serve traffic without re-edit.
2. The closed enum of valid day strings includes BOTH long form (`Monday`, `Tuesday`, `Wednesday`, `Thursday`, `Friday`, `Saturday`, `Sunday`) and short form (`Mon`, `Tue`, `Wed`, `Thu`, `Fri`, `Sat`, `Sun`) — 14 strings total. Any value outside this closed set is rejected at `kubectl apply` time.
3. The existing `weekday-iff-Weekday` CEL rule still holds for both shapes:
    - Present (non-empty list OR single string) iff `recurrence == 'Weekday'`.
    - Absent for every other recurrence kind. A list on a non-`Weekday` recurrence is rejected at apply time.
4. CEL validation additionally rejects:
    - An empty list (`weekday: []`).
    - A list containing the same logical day twice — including cross-form duplicates (`[Mon, Monday]` and `[Monday, Mon]` both error; so do `[Tue, Tue]` and `[Wednesday, Wednesday]`).
5. The Go-side adapter (currently in the store package per Spec 10) normalizes short-form to long-form (or to the canonical `time.Weekday` value) at parse time. Code downstream of the adapter sees a single canonical representation — never branches on form.
6. The adapter produces a set (slice) of `time.Weekday` values on the in-memory `TaskDefinition`. A `Weekday`-recurrence entry's day-matcher fires on any day in that set. Existing single-string CRs produce a one-element set; the matcher logic is unified — no separate "single-day vs multi-day" branch.
7. Period-token rendering per firing day is unchanged: `YYYYWww-<3-letter-lowercase-abbrev>` for the firing day (e.g. `2026W25-mon`, `2026W25-wed`, `2026W25-fri`). Each firing day produces its own period token → its own UUID5 → its own task file. A list `[Mon, Wed, Fri]` produces exactly 3 task files per ISO week, one per matching day, all sharing slug / title / body but each with its own period suffix.
8. Backward compatibility: a CR with `weekday: Monday` (string) produces byte-identical output (same UUID5, same period token, same title, same body, same Kafka message) to what the pre-this-spec publisher produced. No vault file regenerates; no Kafka dedup collision.

## Constraints

- CRD group, version, kind, plural, singular, short name, and every field name are frozen — this spec only widens the `weekday` field's *type*, not its name or position.
- The `RecurrenceKind` wire enum (closed: `Daily`, `Weekly`, `Weekday`, `Monthly`, `Quarterly`, `Yearly`) is frozen by Spec 9; this spec does not touch it.
- Period-token format (`YYYYWww-<abbrev>`) is frozen by Specs 6/8/9. The abbrev is the canonical 3-letter lowercase weekday (e.g. `mon`, `tue`) regardless of which form the operator typed in the list.
- UUID5 derivation (`recurring-<slug>-<period-token>`) is frozen by Spec 6. Single-string CRs MUST produce the same UUID5 byte-for-byte post-deploy.
- The Go-built JSONSchemaProps in `pkg/k8s_connector_schema.go` (or current equivalent) is the canonical source of CRD schema. The string-or-array shape, the 14-element enum, and the new CEL rules live there.
- The existing `weekday-iff-Weekday` CEL rule message and field-path semantics are preserved; this spec extends the rule body, it does not replace the rule. The new "empty list" and "duplicate day" rejections may be additional CEL rules or extensions of the existing one — agent decides at impl time.
- Project DoD applies (`docs/dod.md`): `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` on every error path; no `time.Now()` / `context.Background()` in business logic; Ginkgo v2 / Gomega for tests; Counterfeiter v6 for any new fakes.
- Coding guides apply: `go-k8s-crd-controller-guide.md` (for CRD schema and CEL), `go-validation-framework-guide.md`, `go-factory-pattern.md`, `go-testing-guide.md`, `go-error-wrapping-guide.md`.
- The inventory source of truth is the in-cluster `Schedule` CRD set (per Spec 10), not the deleted `pkg/schedule/inventory.go`. Tests inject fixed `TaskDefinition` slices through the store interface, not a hard-coded Go slice.
- `make precommit` exits 0 from the repo root after every prompt lands.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility |
|---------|-------------------|-----------|----------|---------------|
| Operator applies `weekday: []` (empty list) on a `Weekday` CR | API server rejects the `kubectl apply` with the CEL "non-empty list" message; no CR persists. | `kubectl apply` exits non-zero with the CEL violation message naming `spec.schedule.weekday`. | Operator supplies at least one day. | Reversible (no state change). |
| Operator applies `weekday: [Mon, Monday]` (cross-form duplicate) | API server rejects the apply with the CEL "duplicate day" message. | `kubectl apply` exits non-zero with the CEL violation message. | Operator removes the duplicate. | Reversible. |
| Operator applies `weekday: [Mon, FunDay]` (unknown day) | API server rejects the apply with the CEL/enum violation. | `kubectl apply` exits non-zero. | Operator fixes the value. | Reversible. |
| Operator applies `weekday: [Mon, Wed]` on a `Daily` (or other non-Weekday) recurrence | API server rejects the apply via the existing `weekday-iff-Weekday` CEL rule. | `kubectl apply` exits non-zero with that rule's message. | Operator either drops `weekday` or switches to `recurrence: Weekday`. | Reversible. |
| Operator applies a list-Weekday CR; the informer cache delivers it to the store; the adapter encounters an unknown day string at parse time (defensive — CEL should have rejected it) | Adapter returns a wrapped error for that single entry; the entry is dropped from the store's `List` output for the current cycle; an error is logged with the slug; other entries continue to flow. Tick does NOT abort. | `glog` error line names the slug; Prometheus error counter increments. | Operator fixes the CR; informer watch delivers the update; next tick uses the fixed value. | Reversible. |
| Tick fires on a day NOT in a CR's list | Zero `task.CreateCommand` messages emitted for that slug for that tick. No error. | Prometheus tick counter records the no-op; tick log shows `0 publishes` for that slug. | None needed — by design. | N/A. |
| Tick fires on a day IN a CR's list, and the same date has already been ticked (replay via `/trigger?date=`) | Publisher emits the `task.CreateCommand` for that day → Kafka consumer dedups on UUID5 → no duplicate vault file. Existing idempotency story from Spec 6 preserved. | Vault inspector sees exactly one file per (slug, period-token); Kafka log shows the redundant message accepted by the topic but ignored downstream. | None needed — by design. | Reversible (idempotent). |
| Existing single-string `weekday: Monday` CR continues to apply post-deploy | Adapter produces a one-element `time.Weekday` set containing `time.Monday`; period-token, UUID5, title, and body are byte-identical to pre-this-spec output. | Existing vault file's identifier and title unchanged; no second file appears with a different UUID5. | None needed — invariant. | N/A. |
| Operator applies the SAME slug under TWO different `Schedule` CRs with overlapping day lists | Cannot occur — Kubernetes enforces `metadata.name` uniqueness per namespace; `metadata.name` IS the slug per Spec 10's adapter. | API server rejects the second apply. | N/A — invariant. | N/A. |

## Security / Abuse Cases

- The CRD's CEL validation rejects unknown day strings, empty lists, and duplicate days at apply time — no injection surface from operator input. The closed 14-element enum bounds the set of strings the adapter ever sees.
- A malicious operator with `kubectl apply` on `schedules.task.benjamin-borbe.de` could already publish arbitrary task bodies (per Spec 10 threat model); this spec does not widen that surface. Title and body templating are unchanged.
- The list shape is bounded by the enum (14 distinct values) → max list length post-dedup is 7. Defensive cap: agent decides at impl time whether to add an explicit `maxItems: 14` (pre-dedup) or `maxItems: 7` (post-dedup) on the array schema, or to rely solely on the enum + dedup CEL rule to bound size. Either choice is safe — memory footprint is trivial.
- No new HTTP surface, no new file paths, no new env vars.

## Acceptance Criteria

- [ ] `spec.schedule.weekday` on the `Schedule` CRD accepts a single string OR a non-empty list of strings — evidence: a Ginkgo `It` in `pkg/k8s_connector_validation_test.go` (or sibling) drives the project's CRD validator with one CR using `weekday: Monday` (string) and one using `weekday: [Mon, Wed, Fri]` (list); both PASS; `go test -v ./pkg/...` prints both spec names with PASS.
- [ ] The closed enum of valid day strings includes all 14 forms (`Monday`..`Sunday` AND `Mon`..`Sun`) — evidence: `grep -nE '"Mon"|"Mond"|"Mo"' pkg/k8s_connector_schema.go` returns the enum constant; a table-driven test in `pkg/k8s_connector_validation_test.go` exercises all 14 strings as singletons and as list entries and asserts each is accepted; PASS.
- [ ] A CR with `weekday: []` (empty list) is rejected by CEL validation — evidence: validation test asserts the validator returns an error whose message names "non-empty" (or equivalent); test PASSes.
- [ ] A CR with `weekday: [Mon, Monday]` (cross-form duplicate) is rejected by CEL validation — evidence: validation test asserts the validator returns an error whose message names "duplicate" (or equivalent); test PASSes. The same test covers `[Tue, Tue]` (same-form duplicate) and `[Friday, Friday]`.
- [ ] A CR with `weekday: [Mon, FunDay]` (unknown day) is rejected by enum or CEL validation — evidence: validation test asserts the validator returns an error; test PASSes.
- [ ] A CR with `weekday: [Mon, Wed]` on `recurrence: Daily` (or any non-`Weekday` recurrence) is rejected by the existing `weekday-iff-Weekday` CEL rule — evidence: validation test asserts rejection with the rule's existing message; test PASSes.
- [ ] A CR with `weekday: Monday` (single-string, single-day) on `recurrence: Weekday` is ACCEPTED — evidence: validation test asserts no error; PASSes.
- [ ] The store's adapter normalizes short-form to long-form (or to canonical `time.Weekday`) — evidence: a Ginkgo `DescribeTable` in the store package covers all 14 strings as single-element lists and asserts each maps to the expected `time.Weekday` value; PASSes; `go test -v ./pkg/store/...` prints the table name with PASS.
- [ ] The store's adapter exposes a multi-day `TaskDefinition` whose day-set field (or equivalent) contains the parsed `time.Weekday` values — evidence: an `It` in the store package asserts that `weekday: [Mon, Wednesday, Fri]` (mixed form) maps to a `TaskDefinition` whose day-set equals `{time.Monday, time.Wednesday, time.Friday}` (order-independent); PASSes.
- [ ] The publisher's day-matcher fires on every day in the set — evidence: a table-driven test in `pkg/publisher` (or sibling) seeds a `TaskDefinition` with day-set `{Mon, Wed, Fri}` and asserts: `TasksForDate(2026-06-22 [Mon])` includes the slug, `2026-06-23 [Tue]` does not, `2026-06-24 [Wed]` includes, `2026-06-25 [Thu]` does not, `2026-06-26 [Fri]` includes, `2026-06-27 [Sat]` does not, `2026-06-28 [Sun]` does not. PASSes.
- [ ] Backward-compat: a `TaskDefinition` produced from `weekday: Monday` (single string) yields the byte-identical period token, UUID5 input string, and task title that the pre-this-spec adapter produced — evidence: a Ginkgo `It` asserts the UUID5 input string equals a hand-derived pre-spec value (`recurring-<slug>-2026W26-mon` for a known fixture); PASSes.
- [ ] Across one ISO week, a list `[Mon, Wed, Fri]` CR produces exactly 3 distinct Kafka `task.CreateCommand` messages with 3 distinct UUID5s and 3 distinct period tokens (`-mon`, `-wed`, `-fri`) — evidence: an integration-flavoured Ginkgo `It` runs the tick over each weekday of one ISO week with a fake Kafka publisher (Counterfeiter fake) and asserts the captured message slice has length 3 with the expected period tokens; PASSes.
- [ ] Mixed long+short list (`weekday: [Mon, Wednesday, Fri]`) produces the same in-memory `TaskDefinition` as `[Monday, Wednesday, Friday]` — evidence: a Ginkgo `It` constructs two `v1.Schedule` fixtures (mixed-form and all-long-form) and asserts the adapter outputs are equal on the day-set field; PASSes.
- [ ] Existing tests still pass: `pkg/publisher` (UUID5 stability, period-token rendering from Specs 6/7/8/9), `pkg/tick`, `pkg/handler` (trigger + healthz), `pkg/schedule` (`tasks_for_date`, `canonical_slugs`, `recurrence`, `date`, `no_forbidden_imports`, `task_definition`), `pkg/k8s_connector_*` (Spec 008's validation tests), `pkg/store` (Spec 010's adapter tests for the other five recurrence kinds), and the `k8s/apis/.../v1` round-trip — evidence: `make precommit` exits 0.
- [ ] `CHANGELOG.md` `## Unreleased` section contains one `feat:` bullet describing list-of-weekdays support and short-form acceptance — evidence: `grep -nE 'feat:.*weekday.*list|feat:.*list.*weekday|feat:.*short.*form' CHANGELOG.md` returns line ≥1 under the `## Unreleased` heading.
- [ ] `make precommit` exits 0 from the repo root — evidence: exit code 0.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-weekday-list
make precommit
go test -v -run 'WeekdayList|WeekdayShortForm|WeekdayAdapter|WeekdayMatcher|MixedForm|EmptyList|DuplicateDay|UnknownDay' ./...
grep -nE '"Mon"|"Tue"|"Wed"|"Thu"|"Fri"|"Sat"|"Sun"' pkg/k8s_connector_schema.go
grep -nE '"Monday"|"Tuesday"|"Wednesday"|"Thursday"|"Friday"|"Saturday"|"Sunday"' pkg/k8s_connector_schema.go
```

Expected:
- `make precommit` exits 0.
- The targeted `go test` invocation prints PASS for every matched spec.
- Each grep returns at least 7 matches (the 14 enum entries, split across the two greps).

### End-to-end smoke (spec-verification scenario)

The dark-factory spec-verification step exercises one list-Weekday CR end-to-end:

1. Apply a `Schedule` CR with `recurrence: Weekday` and `weekday: [Mon, Wed, Fri]` (mixed form: e.g. `[Mon, Wednesday, Fri]`).
2. Wait for the informer cache to deliver the CR (`cache.WaitForCacheSync` returns true).
3. Issue `GET /trigger?date=<a-Monday-in-the-list>` (e.g. `2026-06-22`). Expect exactly one Kafka `task.CreateCommand` published with period token `2026W26-mon` and UUID5 derived from `recurring-<slug>-2026W26-mon`.
4. Issue the same `/trigger?date=<same-Monday>` a second time. Expect the same Kafka message republished; downstream consumer dedups on UUID5 → vault gains zero new files (idempotency story from Spec 6).
5. Issue `GET /trigger?date=<a-Tuesday>` (e.g. `2026-06-23`, not in the list). Expect zero Kafka `task.CreateCommand` messages for this slug.

Evidence: each step's expected Kafka message count is asserted against the fake-Kafka publisher's captured messages (or against the real topic if the scenario runs in an integration cluster). Vault-side dedup is asserted via UUID5 equality across the two replay messages.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | CRD schema widening + CEL rules. Update `pkg/k8s_connector_schema.go` so `spec.schedule.weekday` accepts string-or-array. Extend the 14-element enum (long + short forms). Extend CEL: list non-empty, no cross-form or same-form duplicates, presence-iff-Weekday unchanged. Update `pkg/k8s_connector_validation_test.go` with the full table of accept/reject cases. No Go-side adapter changes yet — store still expects single-string. | 1, 2, 3, 4 | 1, 2, 3, 4, 5, 6, 7, 15 (partial), 16 | — |
| 2 | Go-side adapter + matcher widening. Update the store package's adapter (per Spec 10's seam) to parse string-or-list, normalize short→long form, and produce a `TaskDefinition` with a day-set field. Update the publisher / matcher to fire on any day in the set. Update `pkg/schedule/task_definition.go` (or current equivalent) to carry the set. Add the table-driven tests for short-form normalization, mixed-form equivalence, multi-day matching, and single-day backward-compat UUID5 stability. CHANGELOG entry lands here. | 5, 6, 7, 8 | 8, 9, 10, 11, 12, 13, 14, 15, 16 | prompt 1 |

Rationale: prompt 1 lands the CRD-side widening and CEL invariants standalone — the API server rejects bad apply attempts immediately, and the existing single-string adapter still works because the new shapes are not yet produced by any in-cluster CR. Prompt 2 then widens the Go-side type and unifies the matcher, with full table coverage. Splitting the other direction (adapter first) would leave the CRD blocking valid list-shaped CRs the adapter would accept — operators couldn't even test the new behavior end-to-end between prompts.

## Related

- Predecessor (recurrence enum closed): `specs/completed/009-weekday-kind-split.md`
- Predecessor (period token + UUID5 format): `specs/completed/006-period-anchored-uuid.md`, `specs/completed/011-title-period-tokens-and-drop-recurring-frontmatter.md`
- Predecessor (CRD types + schema + connector): `specs/in-progress/008-crd-scaffolding.md`
- Predecessor (informer-backed inventory; store seam this spec extends): `specs/in-progress/010-informer-backed-inventory.md`
- Parent goal (Personal Obsidian vault): `[[Migrate vault-cli Recurring Tasks to recurring-task-creator]]`

## Assumptions

- The store package's adapter from Spec 10 is the single chokepoint for translating `v1.Schedule` → `schedule.TaskDefinition`. Widening `TaskDefinition`'s weekday field to a set (or list of `time.Weekday`) is the minimal Go-side change; the publisher's day-matcher already takes a `TaskDefinition` and a date.
- The CRD schema is Go-built via `apiextensions/v1.JSONSchemaProps` in `pkg/k8s_connector_schema.go`. The string-or-array shape is expressible via `x-kubernetes-preserve-unknown-fields` or an `OneOf` / union approach — agent decides at impl time which idiom the controller-runtime CEL/OpenAPI version in this repo supports cleanly.
- The period-token rendering already takes a `time.Weekday` per Spec 9 (`buildPeriodToken(RecurrenceWeekday, date)` → `YYYYWww-<abbrev>`). Multi-day support is purely about how many times the matcher fires per ISO week, not about how each token is rendered.
- The existing `tasks_for_date.go` (or equivalent) iterates over `TaskDefinition`s and asks each whether it fires on the given date. Switching that question from "does `def.Weekday == date.Weekday()`" to "is `date.Weekday()` in `def.Weekdays`" is a local change.
- The CHANGELOG follows the `## Unreleased` + Conventional Commits style established by Specs 8/9/10.

## Do-Nothing Option

If we don't do this: every multi-day recurring task in the vault-cli inventory expands to N near-duplicate `Schedule` CRs at migration time (IBKR Swing Trading: 5 CRs; Feed Worms: 2 CRs; the long tail of weekday-pair tasks pushes the total well over 30). Body edits have to be replicated across siblings; `kubectl get schedules` becomes hard to scan; the `metadata.name` slug has to encode the weekday (`ibkr-swing-trading-monday`, `ibkr-swing-trading-tuesday`, ...) to avoid name collisions, which leaks weekday into the slug and re-couples the schedule to its day. Not acceptable: the parent goal is to migrate vault-cli's recurring-task inventory, and the duplication tax makes that migration measurably more expensive at every step (write, review, edit, audit) for the entire lifetime of the inventory.

## Verification Result

**Verified:** 2026-06-25T07:17:03Z (HEAD 1246391)
**Binary:** repository source — code-level ACs only; no deployed-binary or Post-Deploy ACs declared
**Scenario:** Ginkgo specs across pkg/, pkg/store, pkg/schedule, pkg/publisher exercise CRD CEL, adapter normalization, multi-day matcher, and per-day UUID5 derivation; `make precommit` covers the full suite
**Evidence:**
- `pkg/k8s_connector_schema.go:28-29` defines the 14-string weekday enum (long + short); 4 CEL rules (`weekdayRequiredIfWeekdayRule`, `weekdayListNonEmptyRule`, `weekdayNoDuplicateRule`, plus the existing `periodOffsetOnlyForPeriodKindsRule`) wired into `scheduleTriggerSchema.XValidations`
- `weekday list — non-empty CEL rule`: empty `[]` rejected; `[Mon]` / `[Mon,Wed]` / `"Monday"` accepted (PASS)
- `weekday list — no-duplicate CEL rule`: `[Mon,Monday]`, `[Monday,Mon]`, `[Tue,Tue]`, `[Wednesday,Wednesday]` rejected; `[Mon,Wed,Fri]` and `[Mon,Tue,Wednesday,Thu,Fri]` accepted (PASS)
- `weekday enum — FunDay rejected … rejects a list containing an unknown day value` (PASS)
- `adaptSchedule weekday normalization — all 14 day strings map to canonical time.Weekday`: 14 entries (Monday long..Sunday long, Mon short..Sun short) all map to expected `time.Weekday` (PASS)
- `adaptSchedule mixed-form list [Mon,Wednesday,Fri] produces {Monday,Wednesday,Friday}` + `long-form and mixed-form lists produce the same Weekdays (long==mixed equivalence)` (PASS)
- `TasksForDate multi-day set fires on matching days across the week of 2026-06-22..28`: Mon IN, Tue OUT, Wed IN, Thu OUT, Fri IN, Sat OUT, Sun OUT — 7/7 entries PASS
- `Publisher weekday token encodes the firing day's weekday (multi-day set fires per day)`: `[Mon,Wed,Fri]` over 2026W25 emits 3 SendCommand calls with byte-identical UUID5s derived from `recurring-swing-trade-2026W25-{mon,wed,fri}` (PASS)
- `Publisher UUID5 stability for the 21 migrated weekday entries`: every single-string CR yields the pre-spec-9 `recurring-<slug>-2026W25-<abbrev>` UUID5 input byte-identically (PASS) — backward-compat
- pkg suite 54/54 PASS · pkg/store 33/33 PASS · pkg/schedule 16/16 PASS · pkg/publisher PASS
- `make precommit` exits 0 ("ready to commit") — covers fmt/vet/lint/gosec/trivy/addlicense + full test suite
- `CHANGELOG.md:13-14` carries two `feat:` bullets describing the CRD widening and Go-side adapter + period-token rendering
**Verdict:** PASS
