---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-07T20:47:31Z"
generating: "2026-06-07T21:27:11Z"
prompted: "2026-06-07T21:27:11Z"
verifying: "2026-06-14T10:51:57Z"
branch: dark-factory/schedule-encoding
---

## Summary

- Encode the full recurring-task inventory (all ~45 entries currently emitted by `jira-task-creator`) as typed Go data inside this service.
- Provide a pure date-driven lookup that answers "which recurring task definitions fire on this date?" with no I/O.
- Translation fidelity is the bar: for any given date, the set of task slugs returned must equal the set of subtasks the current Jira providers would emit for that date.
- Out of scope here: Kafka publishing, deterministic UUID5 generation, hourly cron tick, HTTP trigger, K8s manifests — those are follow-up specs.
- This is the foundational data layer the publisher will sit on top of in a later spec.

## Problem

The service must replace the existing `jira-task-creator` recurring-task pipeline by emitting vault `CreateCommand` events instead of Jira subtasks. Today the inventory is scattered across four `trading_*-story-provider.go` files in `jira-task-creator`, mixing schedule logic, ADF rendering, and Jira-specific labels/issue-type IDs. Until the inventory is captured as plain typed data — independent of Jira, Kafka, and the clock — there is no neutral basis for the publisher, no way to unit-test the schedule without standing up a side-effecting pipeline, and no way to assert the new service stays bug-for-bug compatible with what currently fires. Encoding the schedule first separates "what fires when" from "how it ships," which is the only way to validate the migration before any side effect lands.

## Goal

After this work, a single in-process function can be called with a calendar date and returns the exact set of recurring task definitions that fire on that date — slug, title template, body template, recurrence kind, predicate. The answer matches, slug-for-slug, what the corresponding `jira-task-creator` provider would emit on that date. No network, no clock, no Kafka, no UUID involved.

## Non-goals

- Do NOT publish anything to Kafka — separate spec.
- Do NOT generate deterministic UUID5 identifiers — separate spec.
- Do NOT introduce an hourly cron tick or an HTTP `/trigger` endpoint — separate specs.
- Do NOT add per-task opt-out flags, runtime feature toggles, or any mechanism to disable individual recurring tasks from config — invariant; if a future consumer demands variation, that's a separate spec.
- Do NOT model timezones as configurable — local-Europe/Berlin civil date is the contract; if a future consumer needs another zone, that's a separate spec.
- Do NOT render markdown to ADF, HTML, or any other format inside this package — body templates ship as raw markdown.
- Do NOT carry over disabled tasks from the source (entries currently commented out in `jira-task-creator`, e.g. `CreateCheckSentryTask`, `CreateReviewTradesTask`, `CreateDarwinexInvestor`) — they are excluded from the inventory.
- Do NOT keep the skeleton example packages (`pkg/factory`, `pkg/handler`, `pkg/mathutil`) as part of this spec's success — they remain untouched here; deletion belongs to the publisher spec.

## Desired Behavior

1. A single Go package owns the recurring-task inventory as typed data.
2. Each entry carries: stable slug, title template, markdown body template, recurrence kind (`daily | weekly | monthly | quarterly | yearly`), and a predicate that decides "does this fire on date D?".
3. A pure function `TasksForDate(date) []TaskDefinition` returns every entry whose predicate is true for that date, in deterministic order (sorted by slug).
4. Slugs are stable, kebab-case, globally unique across the inventory, and chosen so the eventual UUID5 input `recurring-<slug>-<YYYY-MM-DD>` will be stable across reboots and refactors.
5. Title templates support exactly the placeholders observed in the source providers: `{{date}}` (YYYY-MM-DD), `{{iso-week}}` (YYYYWWW — uppercase `W`, matches source `dateToWeek`), `{{next-iso-week}}`, `{{month}}` (YYYY-MM), `{{last-month}}`, `{{quarter}}` (YYYYQQ — uppercase `Q`, matches source `dateToQuarter`), `{{last-quarter}}`, `{{year}}` (YYYY), `{{last-year}}`. No others; unknown placeholders are a build-time failure of the inventory test, not a runtime fallback.
6. Body templates are raw markdown strings; whatever ADF the Jira providers built up paragraph-by-paragraph is flattened to its markdown equivalent (links as `[text](url)`, paragraphs separated by blank lines, list items as `- ` / `1. `).
7. Predicates compose from primitives: weekday-in-set, day-of-month-in-set, ISO-week-parity, month-and-day, every-day, quarter-boundary (first day of Jan/Apr/Jul/Oct), year-boundary (first day of Jan). The set of primitives is closed; adding a new kind of predicate is a new spec.
8. The inventory is exhaustive: every non-disabled entry across the four `trading_*-story-provider.go` files appears in the inventory exactly once, with predicate matching the original `switch`/`if` arm that gated it.

## Constraints

- Package is self-contained: no imports of `kafka`, `uuid`, `net/http`, `time.Now`, `github.com/bborbe/jira-task-creator/...`, or anything that touches a clock or network.
- `TasksForDate` is referentially transparent: same input date always returns the same slice (order included).
- Date input is a civil date (year, month, day) in Europe/Berlin local time — no `time.Time` with location ambiguity in the public signature.
- Slugs once committed are frozen: a slug rename is a breaking change to the future Kafka stream and requires its own spec.
- Tests follow the project's standard Ginkgo v2 / Gomega style (see `~/Documents/workspaces/coding-guidelines/go-testing-guide.md`).
- `make precommit` must pass in the changed module.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Two inventory entries declared with the same slug | Package-level test fails listing the duplicate slug | Rename one slug before merge |
| Title template references an unknown placeholder (e.g. `{{week-of-year}}`) | Package-level test fails listing the offending entry and placeholder | Use a supported placeholder or extend the placeholder set in a new spec |
| Predicate primitive references a state outside the closed set | Compile error (predicate constructors are the only way to build one) | Add primitive in a new spec, not ad-hoc |
| `TasksForDate` called with the zero date value | Returns empty slice; no panic | N/A — defensive only |
| Inventory drift vs source (`jira-task-creator` adds/removes a task before migration completes) | Translation-fidelity test fails on the relevant representative date | Update inventory entry, re-run test |
| Predicate evaluation order swapped (non-deterministic slice order) | Translation-fidelity test fails because slug order differs from expected | Sort by slug before return |

## Security / Abuse Cases

Not applicable — no HTTP, no file I/O, no user input crosses a trust boundary in this layer. Templates are static, defined in code, not loaded from disk or env.

## Acceptance Criteria

- [ ] `make precommit` exits 0 in the recurring-task-creator module — evidence: exit code 0.
- [ ] A test asserting all inventory slugs are unique passes — evidence: Ginkgo test report shows the case green; failure mode reproduced by manually duplicating a slug locally yields a failed assertion naming both entries.
- [ ] A test asserting every title-template placeholder is in the supported set passes — evidence: Ginkgo test report shows the case green.
- [ ] For date `2025-01-04` (a Saturday in W01), `TasksForDate` returns exactly the set of slugs corresponding to the Saturday arm of `NewDailyProvider` (`weekday-sat-1`, `weekday-sat-2`, `weekly-review`, `weekday-sat-4`, `weekday-sat-5`, `weekday-sat-6`, `weekday-sat-7`, `weekday-sat-8`, `weekday-sat-9`, `weekday-sat-10`, `weekday-sat-11`, `weekday-sat-12`) — evidence: Ginkgo `Expect(slugs).To(ConsistOf(...))` passes; exact slug list lives in the test file as the canonical spelling.
- [ ] For date `2025-01-05` (a Sunday), `TasksForDate` returns exactly the Sunday arm (`weekday-sun-1`, `weekday-sun-2`, `weekday-sun-3`, `weekday-sun-4`, `weekday-sun-5`, `weekday-sun-6`, `weekday-sun-7`, `weekday-sun-8`, `run-update-all`) — evidence: Ginkgo `ConsistOf` passes.
- [ ] For date `2025-03-05` (a Wednesday, day-of-month=5), `TasksForDate` returns exactly `{monthly-day5-1}` — evidence: Ginkgo `ConsistOf` passes (note: not a Sat/Sun, so no weekly arm contributes).
- [ ] For date `2025-05-01` (a Thursday, month=5 day=1), `TasksForDate` returns the union of the monthly inventory (same 17 monthly slugs as listed in the 2025-04-01 AC below: `monthly-1`, `monthly-2`, `monthly-3`, `monthly-4`, `monthly-5`, `monthly-6`, `monthly-7`, `monthly-8`, `monthly-9`, `monthly-10`, `monthly-11`, `monthly-12`, `monthly-13`, `monthly-14`, `monthly-15`, `monthly-16`, `monthly-17`) and the year-1st pair `{yearly-may-1, yearly-may-2}` — exactly 19 slugs total — evidence: Ginkgo `ConsistOf` passes. (Monthly fires on day-of-month=1; this is consistent with the 2025-04-01 AC.)
- [ ] For date `2025-04-01` (quarter boundary, month=4 day=1), `TasksForDate` returns the union of the monthly inventory (`monthly-1`, `monthly-2`, `monthly-3`, `monthly-4`, `monthly-5`, `monthly-6`, `monthly-7`, `monthly-8`, `monthly-9`, `monthly-10`, `monthly-11`, `monthly-12`, `monthly-13`, `monthly-14`, `monthly-15`, `monthly-16`, `monthly-17`) and the quarterly inventory (`quarterly-1`, `quarterly-2`) — evidence: Ginkgo `ConsistOf` passes.
- [ ] For date `2025-01-01` (year boundary, also quarter boundary, also a Wednesday), `TasksForDate` returns the union of monthly + quarterly + yearly inventories (yearly adds `yearly-1`, `yearly-2`) — evidence: Ginkgo `ConsistOf` passes.
- [ ] Returned slice is sorted by slug ascending on every call — evidence: Ginkgo `Expect(slugs).To(Equal(sortedSlugs))` passes.
- [ ] Calling `TasksForDate` twice with the same date produces identical slices (deep equality on every field) — evidence: Ginkgo `Expect(a).To(Equal(b))` passes.
- [ ] The full set of inventory slugs (across all entries, regardless of date) equals a frozen canonical sorted list pinned in the test file — evidence: Ginkgo `Expect(allSlugs).To(Equal([]string{...full canonical list of all all ~45 slugs...}))` passes. This locks DB #8's exhaustiveness against silent omission: an entry missing from the registry but not firing on any of the six representative dates would still fail this assertion. The canonical list lives in the test, not in production code.
- [ ] Package contains zero imports of `github.com/segmentio/kafka-go`, `github.com/google/uuid`, `net/http`, or `github.com/bborbe/jira-task-creator/...` — evidence: `grep -E '"(github\.com/segmentio/kafka-go|github\.com/google/uuid|net/http|github\.com/bborbe/jira-task-creator)"' pkg/schedule/*.go` returns no matches.
- [ ] No scenario test added — covered by unit tests above; see scenario rule below.

Scenario coverage: NO new scenario. All behavior is pure date → slice lookup, fully reachable from unit tests. No external system, no real Docker, no real cluster required.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator
make precommit
```

Expected: exit code 0, all tests green, lint clean.

## Suggested Decomposition

Single-layer spec (one new package, one pure function, one inventory file, one test file). DB × AC ≈ 8 × 12 = 96 — over the threshold by count, but every AC except the four "representative date" cases is a one-line invariant test on the same surface. Splitting buys nothing because the inventory and the predicate engine are co-designed: you cannot land entries without the predicates, and you cannot land predicates without entries to exercise them. Recommend a single prompt.

If a future agent insists on splitting:

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Predicate primitives + `TaskDefinition` type + empty registry + `TasksForDate` returning sorted, deterministic empty slice; unit tests for predicate primitives | 1, 2, 3, 7, 9, 10, 11 | sorted-order, determinism, no-forbidden-imports, `make precommit` | — |
| 2 | Encode full inventory + translation-fidelity tests for the six representative dates + uniqueness/placeholder validation tests | 4, 5, 6, 8 | the six date ACs, uniqueness, placeholder validation | prompt 1 |

Rationale: prompt 1 lands the closed surface; prompt 2 fills it. No cycles. Default is still single-prompt unless the executor signals otherwise.

## Do-Nothing Option

If we skip this, the publisher spec inherits a tangled job: invent the data shape, encode 40 entries, and assert translation fidelity in the same prompt that also wires Kafka and UUID5. That prompt becomes too large to verify and too risky to land — translation drift would hide behind plumbing bugs. The current `jira-task-creator` continues to fire correctly, so there is no production pressure, but every week of delay is another week of the new vault task system going unbacked by the recurring inventory. Recommendation: do this spec first, exactly as scoped.
