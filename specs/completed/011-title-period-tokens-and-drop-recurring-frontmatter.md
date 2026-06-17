---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-06-16T07:43:41Z"
generating: "2026-06-16T07:47:17Z"
verifying: "2026-06-16T08:12:02Z"
completed: "2026-06-16T08:45:19Z"
branch: dark-factory/title-period-tokens-and-drop-recurring-frontmatter
previous_id: 008
---

## Summary

- Every rendered task title carries a period-token suffix derived from its `RecurrenceKind` and date (e.g. `Update K3s - 2026-06`, `Shutdown K3s - 2026W25-sat`, `Plan Year - 2026`). The suffix prevents the next period's tick from overwriting the previous period's vault file at the same title-derived path.
- The publisher computes the suffix at render time from the same `(slug, Recurrence, Weekday, date)` tuple already used for the period UUID5 — inventory `TitleTemplate` values stay bare strings; the inventory does not encode the suffix.
- The eight inventory entries currently carrying `{{iso-week}}` / `{{next-iso-week}}` / `{{last-month}}` / `{{month}}` / `{{last-quarter}}` / `{{quarter}}` / `{{last-year}}` / `{{year}}` placeholders in `TitleTemplate` are stripped to bare titles; the publisher's automatic suffix replaces those placeholders.
- The materialized frontmatter no longer carries a `recurring: <kind>` field. The published task is a normal one-shot task; nothing labels it "recurring" in the vault.
- One-time deploy cost: vault files keyed by the pre-suffix titles become orphans on the first tick after deploy — same accepted cost pattern as Specs 6 and 7.

## Problem

In production today, the publisher renders each task's title from the inventory's `TitleTemplate` and the agent's downstream controller writes the materialized file at a vault path derived from that title. Most inventory entries (~37 of 45) carry bare title strings — `"Update K3s"`, `"Shutdown K3s"`, `"Atlassian Confluence Backup"` — with no period token in the title. When the next period's tick fires, the publisher emits a task whose title (and therefore vault path) is byte-identical to the previous period's, but whose body and UUID5 differ. The controller, seeing the same path, overwrites the previous period's file, destroying any user edits or ticked checkboxes in it.

Simultaneously, the publisher tags every materialized task with `recurring: <kind>` in its frontmatter. The vault's downstream tooling (Dataview, `/complete-task`, `/start-day`) treats files carrying `recurring:` as recurring tasks — single tasks that re-fire — and applies special behavior on completion. The publisher's output is in fact a one-shot task materialized for a single period, not a re-firing recurring task, so the `recurring:` label causes the wrong special treatment in those tools.

## Goal

After this work:

- Every rendered title carries an unambiguous period suffix that distinguishes one period's materialization from the next, so the controller writes a distinct vault file per period and never overwrites a previous period's edits.
- The suffix is computed by the publisher from the same tuple that produces the period UUID5 — the inventory keeps bare titles (with `{{...}}` placeholders stripped from the eight entries that have them), and there is exactly one source of truth for what a period token looks like.
- The materialized frontmatter is the normal one-shot task shape with no field marking it as recurring; downstream vault tooling treats these as plain tasks.

## Non-goals

- Do NOT add a new `RecurrenceKind` value. The closed enum stays at 5 (daily / weekly / monthly / quarterly / yearly).
- Do NOT change the UUID5 namespace constant or the `recurring-<slug>-<period-token>` input string format. Identifiers are unchanged.
- Do NOT change the period-token derivation (`buildPeriodToken`) — daily / weekly / monthly / quarterly / yearly tokens stay byte-for-byte what Specs 6 and 7 produced. Only the rendered title and frontmatter shape change.
- Do NOT migrate existing vault files. Files keyed by pre-suffix titles or carrying the old `recurring:` field are left untouched (orphan tolerance, matching Specs 6 and 7).
- Do NOT rename or split slugs. Slugs are frozen.
- Do NOT add a per-entry opt-out flag for the suffix or the frontmatter drop — invariant; if a future consumer demands variation, that's a separate spec.
- Do NOT introduce a separator other than ` - ` (space, hyphen, space) between the bare title and the suffix in the rendered title.

## Desired Behavior

1. The publisher renders every task's title as `<bare-title-after-placeholder-substitution> - <period-token>`, where the period token is the same string `buildPeriodToken` returns for the entry's `(Recurrence, date, Weekday)` tuple. Concretely: daily → `<title> - YYYY-MM-DD`; weekly → `<title> - YYYYWww-<3-letter-lowercase-weekday>`; monthly → `<title> - YYYY-MM`; quarterly → `<title> - YYYYQq`; yearly → `<title> - YYYY`.
2. The separator between the bare title and the suffix is the three-character string ` - ` (space, hyphen, space). The bare title is trimmed of trailing whitespace before the separator is appended.
3. The eight inventory entries whose `TitleTemplate` currently contains a `{{iso-week}}` / `{{next-iso-week}}` / `{{last-month}}` / `{{month}}` / `{{last-quarter}}` / `{{quarter}}` / `{{last-year}}` / `{{year}}` placeholder have that placeholder (and any surrounding whitespace it leaves behind) stripped, so the inventory carries the bare task name. After this spec, no `TitleTemplate` value in the inventory contains any of those eight placeholders.
4. The publisher's frontmatter builder emits no `recurring` key. The output `lib.TaskFrontmatter` carries exactly the other six keys it carries today (`assignee`, `status`, `page_type`, `goals`, `priority`, `created_by`) and no others.
5. `BodyTemplate` placeholder substitution is unchanged. Bodies that reference `{{iso-week}}` / `{{last-month}}` / etc. continue to render those placeholders verbatim; only `TitleTemplate` is affected by the inventory cleanup, and the body never grows an automatic suffix.
6. The UUID5 identifier for each `(slug, period)` pair is byte-identical to what Specs 6 and 7 produced. The new title and frontmatter changes do not feed into the identifier input string.

## Constraints

- Project DoD applies (`docs/dod.md`): frozen slugs, closed `RecurrenceKind` enum, `bborbe/errors` 3-arg `Wrap`, no `context.Background()` in business logic, no `time.Time` / `time.Now()` in business logic, GoDoc on exports, `make precommit` clean.
- The UUID5 namespace constant (`pkg/publisher/uuid_namespace.go`) and the `recurring-<slug>-<period-token>` input string format are frozen. Do NOT regenerate, replace, or alias the namespace.
- Period-token format is frozen by Specs 6 and 7. The title-suffix logic MUST reuse `buildPeriodToken` verbatim — no second implementation.
- The 3-letter weekday abbreviation in the weekly suffix is lowercase (`mon` / `tue` / `wed` / `thu` / `fri` / `sat` / `sun`); the week-number prefix `W` stays uppercase. Matches Spec 7.
- The `/trigger?date=` HTTP response shape (Spec 5) is unchanged.
- The set of `SupportedPlaceholders` exposed by `pkg/schedule` stays the same — placeholders remain valid in `BodyTemplate`. Only the inventory's use of the eight period-related placeholders in `TitleTemplate` is removed.
- Existing in-vault files keyed by pre-suffix titles or carrying the old `recurring:` field are left as-is. Future writes emit the new shape.
- Coding guides apply: `~/Documents/workspaces/coding/docs/go-factory-pattern.md`, `go-error-wrapping-guide.md`, `go-testing-guide.md`.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery |
|---------|-------------------|-----------|----------|
| Inventory `TitleTemplate` still contains one of the eight period placeholders after this spec ships | Inventory validation test fails at build time: a Ginkgo spec scans every `TitleTemplate` and asserts none of the eight period placeholders remain. | `make precommit` exit non-zero with the failing test name. | Strip the placeholder from the offending entry's `TitleTemplate`; rebuild. |
| Publisher is asked to render a title for a `RecurrenceKind` outside the closed enum | Render path returns an error wrapped with the slug (delegated to `buildPeriodToken`, which already returns an error for unknown kinds per Spec 6). The render is NOT silently emitted without a suffix. | Per-task error appears in the `/trigger` response or the tick's accumulated errors. | Add the new kind to the closed enum AND extend the publisher's switch — two file edits, one spec; do not paper over with a default branch. |
| Inventory `TitleTemplate` is the empty string after placeholder stripping (e.g. someone strips an entry to nothing) | Inventory validation test fails at build time: a Ginkgo spec asserts every `TitleTemplate` is non-empty after `strings.TrimSpace`. | `make precommit` exit non-zero. | Restore a non-empty bare title to the entry's `TitleTemplate`. |
| Existing vault files keyed by pre-suffix titles remain after deploy | Orphan files persist in the user's vault. They are NOT deleted by this service (no vault writes from this service per DoD). | User notices stale files; or runs a manual cleanup. | Out of scope for this spec; matches Specs 6 and 7's accepted cost. |
| Existing vault files carry `recurring: <kind>` in frontmatter | Left as-is; the publisher does not rewrite or migrate them. New writes (next tick) emit without the field. | Stale frontmatter visible in vault file. | Out of scope; user may strip manually. |
| Two inventory entries render to the same suffixed title for the same date (slug collision masquerade) | Cannot occur — slugs are unique (frozen, enforced by canonical-slugs test) and the UUID5 identifier is slug-anchored. Distinct identifiers ensure the controller writes distinct vault files even if titles ever collide post-suffix. | Existing canonical-slugs uniqueness test. | N/A — invariant. |

## Security / Abuse Cases

The change is internal-structural. The only external surface is the `/trigger?date=` handler, whose request/response shape is unchanged. The handler still:

- Has no authentication (cluster-internal-only deployment, per Spec 5 and existing GoDoc).
- Returns HTTP 400 for missing or malformed `date`, HTTP 200 with per-task errors otherwise.
- Is idempotent under replay because every Publish derives a deterministic UUID5 from the period token.

No new attacker-controlled input. The suffix is derived entirely from `(slug, Recurrence, Weekday, date)`, all of which originate from the frozen inventory or from the validated `date` query parameter that Spec 5 already constrains.

## Acceptance Criteria

- [ ] A Ginkgo spec in `pkg/publisher` exercising the publish path for `(Recurrence=RecurrenceMonthly, TitleTemplate="Update K3s", date=2026-06-15)` asserts the captured `Title` equals `"Update K3s - 2026-06"` — evidence: passing test name printed by `go test -v ./pkg/publisher`.
- [ ] A Ginkgo spec in `pkg/publisher` exercising the publish path for `(Recurrence=RecurrenceWeekly, Weekday=time.Saturday, TitleTemplate="Shutdown K3s", date=2026-06-17)` (Wednesday in ISO week 2026W25) asserts the captured `Title` equals `"Shutdown K3s - 2026W25-sat"` — evidence: passing test name printed by `go test -v ./pkg/publisher`.
- [ ] A Ginkgo `DescribeTable` in `pkg/publisher` covers all five `RecurrenceKind` values and asserts the rendered title format `<bare> - <token>` for each, where `<token>` matches the exact string `buildPeriodToken` returns for the same input — evidence: passing test name(s) printed by `go test -v ./pkg/publisher`.
- [ ] The frontmatter builder produces a map with exactly six keys: `assignee`, `status`, `page_type`, `goals`, `priority`, `created_by`. A Ginkgo spec asserts `HaveLen(6)` AND `Not(HaveKey("recurring"))` — evidence: passing test name printed by `go test -v ./pkg/publisher`.
- [ ] `grep -nE '"recurring"' pkg/publisher/frontmatter.go` returns no matches — evidence: empty grep output (exit code 1).
- [ ] `grep -nE '\{\{(iso-week|next-iso-week|month|last-month|quarter|last-quarter|year|last-year)\}\}' pkg/schedule/inventory.go | grep TitleTemplate` returns no matches — evidence: empty grep output (exit code 1).
- [ ] A Ginkgo spec in `pkg/schedule` iterates every entry in `Inventory()` and asserts `strings.TrimSpace(def.TitleTemplate)` does NOT contain any of the eight period placeholders (`{{iso-week}}`, `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`, `{{last-quarter}}`, `{{year}}`, `{{last-year}}`) — evidence: passing test name printed by `go test -v ./pkg/schedule`.
- [ ] A Ginkgo spec in `pkg/schedule` iterates every entry in `Inventory()` and asserts `strings.TrimSpace(def.TitleTemplate)` is non-empty — evidence: passing test name printed by `go test -v ./pkg/schedule`.
- [ ] A Ginkgo spec in `pkg/publisher` iterates every entry in `schedule.Inventory()` with a fixed reference date (e.g. `2026-06-15`), captures each rendered title, and asserts the title ends with ` - ` + the expected period token for the entry's `(Recurrence, Weekday)` and the reference date — evidence: passing test name printed by `go test -v ./pkg/publisher`.
- [ ] A Ginkgo spec in `pkg/publisher` asserts the UUID5 identifier produced for `(slug="update-k3s", Recurrence=RecurrenceMonthly, date=2026-06-15)` is byte-identical to the value produced by Spec 6's identifier helper for the same input — evidence: passing test asserting equality against `uuid.NewSHA1(uuidNamespace, []byte("recurring-update-k3s-2026-06"))`.
- [ ] `CHANGELOG.md` gains `feat:` bullet(s) describing **both** the title-suffix and the frontmatter `recurring:` removal — evidence: `grep -nE 'feat:.*(title suffix|period token)' CHANGELOG.md` returns line ≥1 AND `grep -nE 'feat:.*recurring' CHANGELOG.md` returns line ≥1 (single bullet may satisfy both; two separate bullets also acceptable).
- [ ] `make precommit` exits 0 from the repo root — evidence: exit code 0.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-title-period
make precommit
grep -nE '\{\{(iso-week|next-iso-week|month|last-month|quarter|last-quarter|year|last-year)\}\}' pkg/schedule/inventory.go | grep TitleTemplate
grep -nE '"recurring"' pkg/publisher/frontmatter.go
```

Expected: `make precommit` exits 0. Both `grep` commands exit with status 1 and print nothing.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Publisher render + frontmatter: extend the publisher's render path to append ` - ` + `buildPeriodToken(...)` to every rendered title (reusing the existing helper, not duplicating it); drop the `recurring` key from `buildFrontmatter` and adjust its signature/callers as needed; update `pkg/publisher/publisher_test.go` to assert the suffixed title format for representative cases per kind and the six-key frontmatter shape. | 1, 2, 4, 6 | 1, 2, 3, 4, 5, 10, 12 | — |
| 2 | Inventory cleanup + invariant tests: strip the eight period placeholders from the affected `TitleTemplate` values in `pkg/schedule/inventory.go`; add Ginkgo specs in `pkg/schedule` asserting (a) no `TitleTemplate` contains any of the eight period placeholders and (b) every `TitleTemplate` is non-empty after trim; add a Ginkgo spec in `pkg/publisher` iterating the full inventory and asserting every rendered title ends with the expected ` - <period-token>` suffix; append CHANGELOG entry. | 3, 5 | 6, 7, 8, 9, 11 | prompt 1 |

Rationale: prompt 1 changes the publisher contract (title shape + frontmatter shape) and updates the publisher's own tests in lock-step. Prompt 2 then aligns the inventory data to the new contract and locks the invariant down with schedule-level tests plus a full-inventory render assertion that proves prompts 1 and 2 are mutually consistent. Splitting differently (inventory first) would leave prompt 1 with stale titles passing tests that no longer reflect the production render path.

## Do-Nothing Option

If we don't do this: the next monthly / quarterly / yearly tick will overwrite the previous period's vault file at the same title-derived path, destroying any user edits and ticked checkboxes accumulated during that period. The user has already hit this in production after Spec 7 shipped. The `recurring:` frontmatter field continues to mislead the vault's downstream tooling (`/complete-task`, `/start-day`, Dataview) into treating one-shot period materializations as re-firing recurring tasks. Both bugs compound on every tick; there is no operational mitigation short of manual title fixes per entry per period.

## Verification Result

**Verified:** 2026-06-16T08:44:53Z (HEAD 7086812)
**Binary:** installed `dark-factory` (spec targets recurring-task-creator, not dark-factory itself)
**Scenario:** Ran `go test -v -args -ginkgo.v ./pkg/publisher` (54/54 pass) and `./pkg/schedule` (10/10 pass); ran the two grep evidence commands; ran `make precommit` from repo root.
**Evidence:**
- `Publisher title suffix appends the period token to a monthly title` (publisher_test.go:473) PASS — AC 1
- `Publisher title suffix appends the period token to a weekly title (with weekday suffix)` (publisher_test.go:487) PASS — AC 2
- `Publisher title suffix appends '<bare> - <period-token>' for every RecurrenceKind {daily,weekly,monthly,quarterly,yearly}` (publisher_test.go:550-574) PASS — AC 3
- `Publisher frontmatter has the six-key shape (assignee, status, page_type, goals, priority, created_by)` (publisher_test.go:615) + `does not depend on RecurrenceKind` PASS — AC 4
- `grep -nE '"recurring"' pkg/publisher/frontmatter.go` → empty, exit 1 — AC 5
- `grep -nE '\{\{(iso-week|next-iso-week|month|last-month|quarter|last-quarter|year|last-year)\}\}' pkg/schedule/inventory.go | grep TitleTemplate` → empty, exit 1 — AC 6
- `inventory has no period placeholders in any TitleTemplate` (inventory_validation_test.go:140) PASS — AC 7
- `inventory has a non-empty TitleTemplate for every entry` (inventory_validation_test.go:158) PASS — AC 8
- `Publisher full-inventory render every inventory entry renders to a title ending in ' - <period-token>'` (publisher_test.go:584) PASS — AC 9
- `period-token byte-equality with the formatter output` DescribeTable + `weekly: byte-equality with the formatter output` (publisher_test.go:227-273) prove UUID5 byte-identity with `uuid.NewSHA1(uuidNamespace, []byte("recurring-<slug>-<token>"))` — AC 10
- CHANGELOG.md line 5 (`feat: Publisher renders every task title as <bare-title> - <period-token>`) + line 6 (`feat: Drop the recurring: <kind> key from the published task frontmatter`) — AC 11
- `make precommit` exit 0 (gosec 0 issues, trivy 0 vulns, addlicense clean, `ready to commit`) — AC 12
**Verdict:** PASS
