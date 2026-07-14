---
status: verifying
approved: "2026-07-14T11:46:51Z"
generating: "2026-07-14T11:46:52Z"
prompted: "2026-07-14T11:57:25Z"
verifying: "2026-07-14T12:12:06Z"
branch: dark-factory/ondate-recurrence-kind
---

## Summary

- Add a sixth-plus recurrence kind, `OnDate`, that fires a Schedule on one fixed calendar date (`month` + `day`) every year — e.g. `03-15` for a birthday.
- Today the closed enum has no date-anchored annual kind: `Yearly` is always-fire (period token `YYYY`, publishes every day, dedup absorbs), so it cannot target "on 03-15". `OnDate` closes that gap with point-shaped match-fire, mirroring how `Weekday` matches a day-of-week.
- `OnDate` fires only when the civil date's month AND day equal the entry's `Month`/`Day`; its period token is `YYYY` (once per year, UUID5-dedup), so replays within a year are idempotent.
- Harden the firing switch: the current implicit `default: always-fire` becomes explicit enumerated always-fire cases, and `default:` becomes skip-with-warning — an unrecognized kind must NOT silently fire every day (the 2026-06-19 deploy-gotcha class).
- CRD gains `month` (1-12) + `day` (1-31) fields, required iff `recurrence == OnDate` and forbidden otherwise, enforced by a CEL rule alongside the existing weekday rules.

## Problem

`recurring-task-creator`'s recurrence enum covers spans (daily/weekly/monthly/quarterly/yearly) and day-of-week (`Weekday`), but not a fixed annual calendar date. A birthday reminder needs exactly that: one task materialized on `03-15` each year, no other day. `Yearly` is the closest kind but is always-fire — it publishes on every civil date and relies on the `YYYY` period token + downstream dedup to collapse to one file, which lands on Jan 1 (first tick of the year), not the birthday. There is no way to express "fire on this month-and-day". This blocks the [[Never Miss a Friend's Birthday]] goal, whose whole mechanism is one Schedule CR per birthday.

A second, latent problem sits in the firing switch: `filterInventoryByDate` uses `default: always-fire` for every non-`Weekday` kind. Adding `OnDate` under that structure would be safe only by luck — and an unrecognized future kind (or a data/code skew during deploy) silently falls through to fire-every-day, which is exactly the failure class observed 2026-06-19. This spec adds `OnDate` AND makes the default restrictive.

## Goal

After this work:

- `RecurrenceOnDate` is a member of the closed `AllRecurrenceKinds` set.
- A `TaskDefinition` carries `Month time.Month` + `Day int`, meaningful only for `RecurrenceOnDate` (mirroring how `Weekdays` is meaningful only for `RecurrenceWeekday`).
- `TasksForDate` fires an `OnDate` entry on, and only on, civil dates whose month and day equal the entry's `Month`/`Day`; the publisher's period token for `OnDate` is `YYYY` (the fire date's year).
- The firing switch enumerates the always-fire kinds explicitly; the `default:` branch logs a warning and does NOT fire, so an unknown kind is a no-op, not a daily storm.
- The Schedule CRD accepts `recurrence: OnDate` with integer `month` (1-12) + `day` (1-31); its CEL rules require both iff `OnDate` and forbid both otherwise.
- The store adapter maps the CR's `month`/`day` onto the `TaskDefinition`.

## Non-goals

- Do NOT author any birthday Schedule CRs — data authoring is downstream ([[Never Miss a Friend's Birthday]]), not this spec.
- Do NOT change the period-token format of any existing kind (`YYYY-MM-DD`, `YYYYWww`, `YYYYWww-<wd>`, `YYYY-MM`, `YYYYQq`, `YYYY` stay exactly as they are).
- Do NOT add day-of-month validity beyond a static 1-31 range check (e.g. reject `02-30`). An `OnDate` of `02-29` fires only in leap years; that is documented behavior, not a validation error (see Failure Modes).
- Do NOT add `PeriodOffset` support for `OnDate` — the CEL rule keeps `PeriodOffset` valid only for Monthly/Quarterly/Yearly; `OnDate` rejects non-zero offset.
- Do NOT rename or renumber existing slugs; existing UUID5 identifiers are untouched (no existing entry changes kind).
- Do NOT delete or alter the `Weekday`/`Weekdays` handling.

## Acceptance Criteria

- [ ] `grep -nE 'RecurrenceOnDate\s+RecurrenceKind = "ondate"' pkg/schedule/recurrence.go` returns ≥1 — evidence: matched const declaration line.
- [ ] `RecurrenceOnDate` appears in the `AllRecurrenceKinds` slice — evidence: `grep -nA20 'AllRecurrenceKinds = ' pkg/schedule/recurrence.go | grep -c 'RecurrenceOnDate'` returns ≥1.
- [ ] `TaskDefinition` carries `Month time.Month` and `Day int` fields with GoDoc stating they are consulted only for `RecurrenceOnDate` — evidence: `grep -nE 'Month\s+time\.Month' pkg/schedule/task_definition.go` and `grep -nE 'Day\s+int' pkg/schedule/task_definition.go` each return ≥1.
- [ ] A Ginkgo spec in `pkg/schedule` proves `TasksForDate` fires an `OnDate` entry (`Month=time.March, Day=15`) on `2027-03-15` and does NOT fire it on `2027-03-14` or `2027-07-15` — evidence: passing test names printed by `go test -v ./pkg/schedule/`.
- [ ] A Ginkgo spec in `pkg/schedule` proves an entry with an unrecognized `RecurrenceKind` (e.g. `"bogus"`) is NOT included by `TasksForDate` (default branch is skip, not fire) — evidence: passing test name printed by `go test -v ./pkg/schedule/`.
- [ ] The `default:` branch is skip-with-warning, and every existing always-fire kind still fires — evidence (both): (a) `grep -nA4 'default:' pkg/schedule/tasks_for_date.go | grep -c 'glog.Warning'` returns ≥1 (default logs a warning, does not fire); (b) a Ginkgo spec in `pkg/schedule` asserts each of `RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly` is included by `TasksForDate` on an arbitrary date — passing test name printed by `go test -v ./pkg/schedule/`. (Behavioral, not source-text-pinned — a gofmt reorder must not fail this.)
- [ ] A Ginkgo spec in `pkg/publisher` proves the period token for `(RecurrenceOnDate, fire date 2027-03-15)` is `"2027"` — evidence: passing test name printed by `go test -v ./pkg/publisher/`.
- [ ] The Go-built CRD schema accepts `recurrence: "OnDate"` and declares integer `month` (min 1, max 12) + `day` (min 1, max 31) properties — evidence: `grep -nE '"OnDate"|"month"|"day"' pkg/k8s_connector_schema.go` returns matches for all three tokens.
- [ ] A Ginkgo spec proves CRD validation REQUIRES `month`+`day` when `recurrence == OnDate` and REJECTS `month`/`day` when recurrence is any other kind (CEL rule) — evidence: passing test names printed by `go test -v` for the k8s_connector validation suite (`pkg/k8s_connector_validation_test.go`).
- [ ] The store adapter maps CR `month`/`day` onto `TaskDefinition.Month`/`.Day` for an `OnDate` CR — evidence: passing adapter test name printed by `go test -v` asserting a round-tripped `OnDate` CR yields `Month=<m>, Day=<d>`.
- [ ] `make generatek8s` leaves the tree clean (deepcopy regenerated + committed) — evidence: `make generatek8s && git diff --exit-code` exits 0.
- [ ] `make precommit` exits 0 from the repo root — evidence: exit code 0.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make generatek8s && git diff --exit-code
go test -v ./pkg/schedule/ ./pkg/publisher/
grep -nE 'RecurrenceOnDate' pkg/schedule/recurrence.go
grep -nE 'case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly' pkg/schedule/tasks_for_date.go
```

Expected: `make precommit` exits 0; `make generatek8s && git diff --exit-code` exits 0 (no uncommitted codegen); `go test -v` shows the new OnDate specs passing; the greps return ≥1 line each.

### Operator-executable (runs on the host, spec-verification ladder — Rung 2/3 after deploy)

- [ ] **Post-Deploy (Rung-2):** an `OnDate` Schedule CR fires on its MM-DD in dev — evidence: apply a throwaway `OnDate` CR (`month`/`day` set to today's dev date), then `curl "https://dev.quant.benjamin-borbe.de/admin/recurring-task-creator/trigger?date=<YYYY-MM-DD matching the CR>"` returns JSON with `"published"` ≥ 1, and the materialized task file appears under `~/Documents/Obsidian/Personal/24 Tasks/`.
  - `deploy_check:` `kubectlquant -n dev get statefulset/recurring-task-creator -o jsonpath='{.spec.template.spec.containers[0].image}' | awk -F: '{print $NF}'`
  - `deploy_target:` `$(git rev-parse --short HEAD)`
- [ ] **Post-Deploy (Rung-3):** the same throwaway `OnDate` CR check passes on prod — evidence: `curl "https://prod.quant.benjamin-borbe.de/admin/recurring-task-creator/trigger?date=<MM-DD>"` returns `"published"` ≥ 1.
  - `deploy_check:` `kubectlquant -n prod get statefulset/recurring-task-creator -o jsonpath='{.spec.template.spec.containers[0].image}' | awk -F: '{print $NF}'`
  - `deploy_target:` the mirrored prod image tag (semver released for this change)

## Desired Behavior

1. `pkg/schedule/recurrence.go` declares `RecurrenceOnDate RecurrenceKind = "ondate"` and appends it to `AllRecurrenceKinds` (declaration-order preserved; the set stays the single closed source of valid kinds).
2. `TaskDefinition` (`pkg/schedule/task_definition.go`) gains `Month time.Month` and `Day int`. GoDoc states both are consulted only when `Recurrence == RecurrenceOnDate`, are the zero value (`time.Month(0)` / `0`) and ignored otherwise, and are produced by the store adapter from the CR's `month`/`day`.
3. `filterInventoryByDate` (`pkg/schedule/tasks_for_date.go`) adds `case RecurrenceOnDate:` firing the entry iff `def.Month == date.Time().Month() && def.Day == date.Time().Day()`.
4. The same switch replaces the implicit `default: always-fire` with an explicit `case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceQuarterly, RecurrenceYearly:` (always-fire) and a `default:` that logs `glog.Warningf("unknown recurrence kind %q for slug %q — skipping", ...)` and does NOT append the entry. Existing kinds' firing behavior is byte-for-byte unchanged.
5. The publisher period-token builder (`pkg/publisher/period_token.go`) returns the fire date's 4-digit year (`"YYYY"`) for `RecurrenceOnDate`, matching `Yearly`'s token shape and using the same year-formatting helper the existing kinds use (once-per-year dedup). `PeriodOffset` is not applied to `OnDate`.
6. The Go-built CRD JSONSchema (`pkg/k8s_connector_schema.go`) adds `"OnDate"` to the `recurrence` enum and integer `month` (minimum 1, maximum 12) + `day` (minimum 1, maximum 31) properties, plus a CEL rule: `month` and `day` are required when `recurrence == "OnDate"` and must be absent otherwise; `PeriodOffset` non-zero remains invalid for `OnDate`.
7. The store adapter that builds `TaskDefinition` from a Schedule CR maps `spec.schedule.month` → `Month` (as `time.Month`) and `spec.schedule.day` → `Day` for `OnDate` entries; leaves them zero for other kinds.

## Constraints

- Project DoD applies: `docs/dod.md` (Ginkgo v2 / Gomega, `bborbe/errors` 3-arg `Wrap`, no `context.Background()` / `time.Now()` in business logic, GoDoc on exports, `make precommit` clean).
- `RecurrenceKind` remains a closed enum; `AllRecurrenceKinds` stays declaration-ordered and is the single validity source (no inline kind switch that bypasses it — `go-enum-type-pattern.md`).
- Existing period-token formats and existing kinds' firing behavior MUST NOT change — only additive.
- The UUID5 namespace and existing slugs are frozen — no existing entry changes kind or identifier.
- CRD schema is the hand-built Go `JSONSchemaProps` in `pkg/k8s_connector_schema.go` (single source of truth); `make generatek8s` regenerates deepcopy/clients and MUST leave the tree clean.
- Coding guides apply: `go-kubernetes-crd-controller-guide.md`, `go-enum-type-pattern.md`, `go-error-wrapping-guide.md`, `go-testing-guide.md`, `go-time-injection.md`, `go-glog-guide.md`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Schedule CR sets `recurrence: OnDate` but omits `month` or `day` | CRD admission rejects the apply (CEL rule); the CR never reaches the informer. | Operator adds both fields; re-apply. |
| Non-`OnDate` CR sets `month`/`day` | CRD admission rejects the apply (CEL rule). | Remove the fields; re-apply. |
| `OnDate` with `month: 2, day: 29` (leap day) | Fires only in leap years (Feb 29 exists only then); no error — documented behavior. | If the operator wants every-year firing, pick `02-28` or `03-01`. |
| `day` out of a month's real range (e.g. `month: 4, day: 31`) | Static 1-31 range passes admission, but the civil date `2027-04-31` never occurs, so the entry never fires. Not validated per-month in this spec (Non-goal). | Operator corrects the day; the entry begins firing next matching year. |
| An unrecognized `RecurrenceKind` reaches `filterInventoryByDate` (data/code skew) | Entry is skipped and a WARN is logged (`unknown recurrence kind ... — skipping`); no daily storm. | Deploy the code that recognizes the kind, or fix the CR; the entry resumes on the next matching tick. |
| Two `OnDate` entries share a slug | Existing slug-uniqueness build test fails (unchanged). | Rename one (separate spec, slug freeze). |

## Security / Abuse Cases

The only external surface is the existing `/trigger?date=` handler, unchanged in shape. Adding `OnDate` narrows rather than widens firing (match-fire, not always-fire), so it introduces no new publish volume. The handler remains cluster-internal, unauthenticated, idempotent under replay (period-anchored UUID5). No new abuse vector.

## Suggested Decomposition

Prompts should be generated in this order; each row is one prompt.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Schedule-layer core: add `RecurrenceOnDate` const + `AllRecurrenceKinds` entry, add `Month`/`Day` to `TaskDefinition`, add `OnDate` case + explicit always-fire cases + warn-on-default to `filterInventoryByDate`, with Ginkgo specs for firing + unknown-kind-skip. | 1, 2, 3, 4 | 1, 2, 3, 4, 5, 6 | — |
| 2 | Publisher token: `OnDate → "YYYY"` in the period-token builder + Ginkgo spec. | 5 | 7 | prompt 1 (uses `RecurrenceOnDate`) |
| 3 | CRD schema + adapter: add `OnDate` enum value + `month`/`day` properties + CEL rule in `pkg/k8s_connector_schema.go`, map CR `month`/`day` → `TaskDefinition` in the store adapter, `make generatek8s`, with validation + adapter Ginkgo specs. | 6, 7 | 8, 9, 10, 11 | prompt 1 (uses the new fields/kind) |

Rationale: prompt 1 establishes the kind + firing semantics the other two build on. Prompt 2 (publisher) and prompt 3 (CRD + adapter) are independent of each other but both depend on prompt 1's type additions.

## Do-Nothing Option

If we don't do this: there is no way to fire a Schedule on a fixed annual calendar date, so [[Never Miss a Friend's Birthday]] cannot use `recurring-task-creator` at all — it would have to fall back to a Google-Calendar-plus-daily-check mechanism outside the vault-task pipeline, splitting where reminders live. The latent `default: always-fire` risk also remains: the next new kind, or any data/code deploy skew, can silently fire an entry every day (the 2026-06-19 class of incident) with no warning. The cost of the change is one spec + three prompts; the cost of not doing it is a permanently date-blind scheduler and a standing always-fire footgun.
