---
status: approved
spec: [007-recurrence-kind-cleanup]
created: "2026-06-15T21:05:00Z"
queued: "2026-06-15T21:15:49Z"
branch: dark-factory/recurrence-kind-cleanup
---

<summary>
- Sets `Weekday: time.Saturday` on the 12 Saturday weekly inventory entries and `Weekday: time.Sunday` on the 9 Sunday weekly inventory entries, so the publisher's weekly period token carries the correct lowercase 3-letter weekday suffix (`sat` for Saturday, `sun` for Sunday) for every current entry.
- Adds a new Ginkgo validation spec that enumerates every weekly inventory entry and asserts its `Weekday` field is in the closed set `{time.Saturday, time.Sunday}` — catches any future weekly entry added without an explicit weekday.
- Adds a second Ginkgo validation spec that asserts every non-weekly entry's `Weekday` field is the zero value (`time.Sunday`) AND its slug is NOT in the new `sundayWeeklyAllowList` — catches the ambiguity where a Sunday-weekly entry at the zero value would be indistinguishable from a non-weekly entry left at the zero value.
- Declares the `sundayWeeklyAllowList` as a named package-level var in `inventory_validation_test.go` (not in production code) containing the exact 9 Sunday weekly slugs, with a Ginkgo `It` asserting `len(sundayWeeklyAllowList) == 9` so an accidental addition or removal of a Sunday slug fails the test before the data invariant breaks.
- Updates the per-slug weekly token expectations implicitly (the publisher's `weekdayAbbrev` helper now receives the explicit `time.Saturday` / `time.Sunday` from the data, producing `2026W25-sat` or `2026W25-sun` tokens).
- The build is green at the end of this prompt. Existing tests pass; the new validation specs cover the new `Weekday` invariants. `make precommit` exits 0.

</summary>

<objective>

Populate the new `Weekday` field on every weekly inventory entry with the correct `time.Weekday` value (Saturday for the 12 Saturday entries, Sunday for the 9 Sunday entries) and add the validation specs that lock the new `Weekday` data invariant down. The 9-Sunday-slug allow-list is declared as a named package-level test variable so the disambiguation rule (Sunday weekly entries are the only weekly entries whose `Weekday` may equal the zero value) is enforced by a test, not by a comment. The build remains green; Prompt 1's shape change and the publisher's weekly branch are now exercised end-to-end with real data.

</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before making changes:

- `/workspace/pkg/schedule/inventory.go` — the 45-entry inventory slice. After Prompt 1: (a) every entry has the fields `Slug`, `TitleTemplate`, `BodyTemplate`, `Recurrence` only (no `Fires`); (b) the 21 weekly entries' `Weekday` field is NOT set (zero value `time.Sunday`); (c) the 24 non-weekly entries' `Weekday` field is also the zero value (correct — non-weekly entries must leave `Weekday` unset per the spec). This prompt adds `Weekday: time.Saturday` to the 12 Saturday weekly entries and `Weekday: time.Sunday` to the 9 Sunday weekly entries. The 24 non-weekly entries are NOT touched.
- `/workspace/pkg/schedule/inventory_validation_test.go` — the existing validation test file (`unique slugs`, `supported placeholders`, `recurrence kinds from the closed set`). It currently has no `Weekday`-related specs. This prompt adds two new `It` blocks and the `sundayWeeklyAllowList` package-level var. The existing three `It` blocks are unchanged.
- `/workspace/pkg/schedule/canonical_slugs_test.go` — the 45-slug canonical list. The list is unchanged; the per-slug weekday is encoded in the new validation spec, not in the canonical list.
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest()` accessor. Unchanged; this prompt uses it to enumerate entries.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` closed enum (5 values). Unchanged.
- `/workspace/pkg/publisher/uuid_namespace.go` — the publisher's weekly branch now reads `def.Weekday` and appends `-"<3-letter-lowercase-weekday>"`. No change in this prompt; the helper is exercised through the inventory data.
- `/workspace/pkg/publisher/publisher_test.go` — the new `It("buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday", ...)` test added by Prompt 1 sets `Weekday: time.Saturday` directly on a test `def` literal; it is unaffected by the inventory data update. The other tests are unaffected.
- `/workspace/pkg/handler/trigger.go` — the handler iterates `schedule.Inventory()` (slug-sorted) and publishes every entry. After this prompt, the 21 weekly entries' identifiers are the new shape (`2026W25-sat` / `2026W25-sun`); the tick path (post-Spec-6) and the trigger path produce the same identifiers for the same `(def, date)`. No change in this prompt.
- `/workspace/CHANGELOG.md` — append a `feat:` bullet to `## Unreleased`.

Coding-guideline references (read inside the YOLO container):
- `go-testing-guide.md` — Ginkgo v2 / Gomega; dot-imports; external test package (`package schedule_test`). The new specs follow the existing pattern in `inventory_validation_test.go` (one `Describe` block, multiple `It` blocks, no `BeforeEach` needed).
- `go-enum-type-pattern.md` — `RecurrenceKind` is a closed string enum; the new validation spec uses the same map-lookup pattern the existing `recurrence kinds from the closed set` spec uses for membership testing.
- `go-licensing-guide.md` — the new content lives in an existing file that already has the 2026 BSD header. Do NOT add a new file; add the new specs to `inventory_validation_test.go` in place.
- `definition-of-done.md` — coverage ≥80% for the package (the new specs ADD to coverage, they do not remove it; the package's coverage stays at or above 80% after the change).

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/recurrence.go (FROZEN, lines 10-16)
const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)

// pkg/schedule/inventory.go (POST-PROMPT-1, weekly entry shape)
//
// A weekly entry after Prompt 1 looks like:
//
//     {
//         Slug:          "shutdown-k3s",
//         TitleTemplate: "Shutdown K3s",
//         BodyTemplate:  "...",
//         Recurrence:    RecurrenceWeekly,
//         // Weekday is at the zero value (time.Sunday). This prompt sets
//         // it explicitly: time.Saturday for the 12 Saturday entries,
//         // time.Sunday for the 9 Sunday entries.
//     }
//
// The 9 Sunday weekly slugs (in inventory declaration order):
//   1. complete-rsync-backups
//   2. complete-longhorn-backups
//   3. turn-off-hell
//   4. turn-off-sun
//   5. turn-off-fire
//   6. docker-registry-gc
//   7. rebuild-trading-dev-prod
//   8. check-bot-is-healthy
//   9. run-update-all
```

The 12 Saturday weekly slugs (Prompt 1 left them at the zero value; this prompt sets `Weekday: time.Saturday` on each — for the executor's reference, the slugs are `shutdown-k3s`, `turn-on-hell`, `weekly-review`, `check-ftmo-demo-accounts`, `lexoffice-invoices`, `moneymoney-review`, `opnsense-update`, `home-assistant-update-backup`, `renew-gmail-oauth-tokens`, `plan-next-week`, `run-update-all-saturday`, `topic-backup-saturday`).

</context>

<requirements>

## 1. Set `Weekday: time.Saturday` on the 12 Saturday weekly entries

In `/workspace/pkg/schedule/inventory.go`, add the field `Weekday: time.Saturday,` to each of the 12 Saturday weekly entries. The entries are listed in declaration order; the slug of each is one of: `shutdown-k3s`, `turn-on-hell`, `weekly-review`, `check-ftmo-demo-accounts`, `lexoffice-invoices`, `moneymoney-review`, `opnsense-update`, `home-assistant-update-backup`, `renew-gmail-oauth-tokens`, `plan-next-week`, `run-update-all-saturday`, `topic-backup-saturday`.

The new field line is added immediately after the `Recurrence: RecurrenceWeekly,` line in each entry. After the change, a representative entry (e.g. `shutdown-k3s`) reads:

```go
{
    Slug:          "shutdown-k3s",
    TitleTemplate: "Shutdown K3s",
    BodyTemplate: "Shutdown K3s cleanly so BoltDB files are not corrupt during backups.\n\n" +
        "~/Documents/workspaces/scripts/remote-k3s-shutdown-nuke.sh\n\n" +
        "[K3s Cluster Weekly Reboot Procedure](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FK3s%20Cluster%20Weekly%20Reboot%20Procedure)\n\n" +
        "[jira-task-creator](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2Fjira-task-creator)",
    Recurrence: RecurrenceWeekly,
    Weekday:    time.Saturday,
},
```

The `goimports-reviser` alignment of the colon is preserved — the `Recurrence` and `Weekday` lines may need their colon alignment updated if the existing entries' colons in their `Slug`/`TitleTemplate`/`BodyTemplate`/`Recurrence` lines are visually aligned. Match the alignment style of the existing struct literal in the same file.

Do not change the `Slug`, `TitleTemplate`, `BodyTemplate`, or `Recurrence` field on any entry. Only the `Weekday` field is added.

## 2. Set `Weekday: time.Sunday` on the 9 Sunday weekly entries

In `/workspace/pkg/schedule/inventory.go`, add the field `Weekday: time.Sunday,` to each of the 9 Sunday weekly entries. The entries are listed in declaration order; the slug of each is one of: `complete-rsync-backups`, `complete-longhorn-backups`, `turn-off-hell`, `turn-off-sun`, `turn-off-fire`, `docker-registry-gc`, `rebuild-trading-dev-prod`, `check-bot-is-healthy`, `run-update-all`.

The new field line is added immediately after the `Recurrence: RecurrenceWeekly,` line in each entry. After the change, a representative entry (e.g. `complete-rsync-backups`) reads:

```go
{
    Slug:          "complete-rsync-backups",
    TitleTemplate: "Complete Rsync Backups",
    BodyTemplate: "* check backup status\n" +
        "** [Backup Status](https://backup.hell.hm.benjamin-borbe.de/status)",
    Recurrence: RecurrenceWeekly,
    Weekday:    time.Sunday,
},
```

Do not change the `Slug`, `TitleTemplate`, `BodyTemplate`, or `Recurrence` field on any entry. Only the `Weekday` field is added.

## 3. Verify the 21 `Weekday` assignments and 24 untouched entries

After steps 1 and 2, the inventory has:

- 12 entries with `Weekday: time.Saturday,` (the 12 Saturday weekly entries listed in §1).
- 9 entries with `Weekday: time.Sunday,` (the 9 Sunday weekly entries listed in §2).
- 24 entries with no `Weekday` field line (the 24 non-weekly entries: 1 day-5 monthly, 2 yearly, 17 monthly-day-1, 2 quarterly, 2 yearly). The Go zero value of `time.Weekday` is `time.Sunday` (0), so the non-weekly entries' effective `Weekday` is `time.Sunday` — same as a Sunday weekly entry. The disambiguation is the Sunday-slug allow-list in the validation spec (§5).

Verify by reading the file end-to-end before saving. Run the AC #4 and AC #5 grep evidence (see `<verification>`) to confirm the counts.

## 4. `import "time"` MUST remain in `inventory.go` (guardrail, not an instruction to act)

After the 21 `Weekday:` lines are added, `time.Saturday` and `time.Sunday` are the only `time.X` references in `pkg/schedule/inventory.go`, but they still require the `time` import to compile. **DO NOT remove or modify the `import "time"` line.** This section is a guardrail against accidental removal — there is no edit to make here.

(The `inventory.go` file's `time` import was originally added for the deleted `onWeekdayDay5OfMonth` function in Prompt 1, but it now serves the new `Weekday: time.Saturday` / `Weekday: time.Sunday` field values. Keep the import.)

## 5. Add the new validation specs and the `sundayWeeklyAllowList` allow-list

In `/workspace/pkg/schedule/inventory_validation_test.go`, append the following content AFTER the existing `It("uses recurrence kinds from the closed set", ...)` block (the existing closing `})` of the outer `var _ = Describe("inventory", ...)` is the last line of the file; the new specs are added inside the existing `Describe` block, before its closing `})`).

First, add the `sundayWeeklyAllowList` package-level var at the top of the file (after the imports, before the existing `var _ = Describe(...)` line). The var is a sorted-or-declaration-order slice of the 9 Sunday weekly slugs. The convention is declaration order, matching the order in `inventory.go`:

```go
// sundayWeeklyAllowList is the exact set of inventory slugs whose Recurrence
// is RecurrenceWeekly AND whose intended Weekday is time.Sunday. The list
// is the disambiguation key for the new "non-weekly entries must leave
// Weekday at the zero value" validation: because time.Sunday is BOTH the
// zero value of time.Weekday AND the intended value of a Sunday weekly
// entry, the only way to tell a "Sunday weekly entry" apart from a
// "non-weekly entry that forgot to set Weekday" is to enumerate the
// Sunday slugs. Length is asserted to be exactly 9 — adding or removing
// a Sunday weekly slug is a data-shape change that requires updating
// this list and the inventory together.
var sundayWeeklyAllowList = []string{
    "complete-rsync-backups",
    "complete-longhorn-backups",
    "turn-off-hell",
    "turn-off-sun",
    "turn-off-fire",
    "docker-registry-gc",
    "rebuild-trading-dev-prod",
    "check-bot-is-healthy",
    "run-update-all",
}
```

Then, INSIDE the existing `Describe("inventory", ...)` block, after the `It("uses recurrence kinds from the closed set", ...)` test, add the three new specs. The new code:

```go
It("has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList", func() {
    // Adding or removing a Sunday weekly slug is a data-shape change
    // that must be reflected here. This assertion catches accidental
    // list drift.
    Expect(sundayWeeklyAllowList).To(HaveLen(9))
})

It("every weekly entry has Weekday in {time.Saturday, time.Sunday}", func() {
    allowed := map[time.Weekday]bool{
        time.Saturday: true,
        time.Sunday:   true,
    }
    for _, def := range schedule.AllDefinitionsForTest() {
        if def.Recurrence != schedule.RecurrenceWeekly {
            continue
        }
        Expect(allowed).To(HaveKey(def.Weekday),
            "weekly entry %q has Weekday %v; expected time.Saturday or time.Sunday", def.Slug, def.Weekday)
    }
})

It("every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList", func() {
    for _, def := range schedule.AllDefinitionsForTest() {
        if def.Recurrence == schedule.RecurrenceWeekly {
            continue
        }
        Expect(def.Weekday).To(Equal(time.Sunday),
            "non-weekly entry %q has non-zero Weekday %v; non-weekly entries must leave Weekday unset",
            def.Slug, def.Weekday)
        Expect(sundayWeeklyAllowList).NotTo(ContainElement(def.Slug),
            "non-weekly entry %q is in sundayWeeklyAllowList; the allow-list must contain only weekly slugs",
            def.Slug)
    }
})
```

The third `It` does double duty: (a) it catches a non-weekly entry that was given a non-zero `Weekday` (the spec's Failure Mode row 2); (b) it catches the symmetric error of accidentally listing a non-weekly slug in the Sunday allow-list (which would silently make the test for Failure Mode row 1 produce a false positive — a non-weekly slug at `Weekday=time.Sunday` would be allowed by the allow-list even though it isn't a weekly entry). Both directions of the disambiguation are covered by this single test.

Imports required for the new content:
- `"time"` — add to the import block (currently the file only imports `"regexp"`, `"github.com/onsi/ginkgo/v2"`, `"github.com/onsi/gomega"`, and the internal `schedule` package). The `time.Saturday`, `time.Sunday` constants require it.
- `schedule.AllDefinitionsForTest()` is already used by the existing specs; no new import needed.

After the change, the file's full contents are: imports (regexp, time, ginkgo, gomega, schedule), the `sundayWeeklyAllowList` var, and the single `Describe("inventory", ...)` block with six `It` cases (the original three + the new three).

## 6. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (one bullet, `feat:` prefix per `changelog-guide.md`):

```markdown
- feat: Set `Weekday: time.Saturday` on the 12 Saturday weekly inventory entries and `Weekday: time.Sunday` on the 9 Sunday weekly entries; add inventory-`Weekday` validation specs (weekly entries in `{Sat, Sun}`, non-weekly entries at zero value) with the `sundayWeeklyAllowList` test allow-list (length asserted to be exactly 9)
```

## 7. Imports and conventions

- The modified `pkg/schedule/inventory.go` file retains the 2026 copyright header.
- The modified `pkg/schedule/inventory_validation_test.go` file retains the 2026 copyright header.
- Use `goimports-reviser` style: standard library first (alphabetical), then third-party (alphabetical: `github.com/onsi/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use Ginkgo v2 / Gomega style with dot-imports (matches the existing tests in the file).
- Do NOT touch `pkg/publisher/`, `pkg/handler/`, `pkg/factory/`, `pkg/tick/`, `main.go`, the Makefile, k8s manifests, or the Prometheus metric surface. The publisher's weekly branch is already wired (Prompt 1); this prompt only feeds the data into it.
- Do NOT add a new Prometheus metric, opt-out flag, runtime config knob, or per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT regenerate any counterfeiter mock.
- Do NOT commit — dark-factory handles git.

</requirements>

<constraints>

- Slugs are FROZEN. The 45 entries' slugs do not change in this prompt. The `sundayWeeklyAllowList` is a strict subset of the 45 slugs, exactly 9 entries long.
- The 21 weekly entries' `Weekday` field MUST be set explicitly in this prompt: `time.Saturday` for the 12 Saturday entries, `time.Sunday` for the 9 Sunday entries. Leaving any weekly entry's `Weekday` at the zero value is a build-time test failure (the new "every weekly entry has Weekday in {Sat, Sun}" spec catches it).
- The 24 non-weekly entries MUST leave the `Weekday` field unset (no `Weekday:` line in the struct literal). The zero value `time.Sunday` is correct for them. The new "every non-weekly entry leaves Weekday at the zero value" spec catches any accidental non-zero `Weekday` on a non-weekly entry.
- The `sundayWeeklyAllowList` var lives in the TEST package only (`package schedule_test`), not in the production `schedule` package. It is a test-only constant. It is declared in `pkg/schedule/inventory_validation_test.go` and is referenced by the same file's specs.
- The 3-letter weekday abbreviation is lowercase (`mon` / `tue` / `wed` / `thu` / `fri` / `sat` / `sun`). The publisher's `weekdayAbbrev` helper is unchanged in this prompt; the lowercase mapping was set in Prompt 1.
- The `RecurrenceKind` enum stays a closed set of exactly 5 values. The new validation specs use the existing constants.
- The `uuidNamespace` constant in `/workspace/pkg/publisher/uuid_namespace.go` is FROZEN byte-identical — do NOT touch it.
- The `Publisher` interface signature is FROZEN — do NOT touch it.
- The handler's iteration order (full inventory, slug-sorted) is set in Prompt 1; the data update in this prompt does not change the iteration order.
- Existing tests must still pass after all edits. The new specs ADD to coverage; they do not remove any existing test.
- Coverage on the changed package stays at or above 80%. The two new `It` blocks and the `sundayWeeklyAllowList` length assertion add three passes through the `AllDefinitionsForTest()` enumeration; coverage of the schedule package's exported surface increases.
- Project DoD (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` for error wrapping (no error paths in this prompt's new code); no `context.Background()` in business logic (no business logic in this prompt's new code); no `time.Time` / `time.Now()` in business logic (the new code uses `time.Saturday` / `time.Sunday` constants, NOT `time.Time`); GoDoc on the new `sundayWeeklyAllowList` var (the var is a package-level test helper, so the convention is a `//` comment, not a full GoDoc); `make precommit` clean.
- Do NOT commit — dark-factory handles git.

</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/schedule/...` — all Ginkgo specs green. In particular:
   - `It("has exactly 9 Sunday weekly slugs in sundayWeeklyAllowList", ...)` passes (asserts `len == 9`).
   - `It("every weekly entry has Weekday in {time.Saturday, time.Sunday}", ...)` passes.
   - `It("every non-weekly entry leaves Weekday at the zero value AND its slug is NOT in sundayWeeklyAllowList", ...)` passes.
   - The pre-existing three `It` blocks (`has unique slugs`, `uses only supported placeholders in TitleTemplate and BodyTemplate`, `uses recurrence kinds from the closed set`) continue to pass.
3. `grep -nE 'Weekday:\s+time\.Saturday' pkg/schedule/inventory.go | wc -l` — must report exactly `12` (AC #4).
4. `grep -nE 'Weekday:\s+time\.Sunday' pkg/schedule/inventory.go | wc -l` — must report exactly `9` (AC #5).
5. `grep -c 'sundayWeeklyAllowList' pkg/schedule/inventory_validation_test.go` — must return ≥ 2 (the var declaration + at least one spec use). The expected value is exactly 3 (one declaration + two spec uses: the length assertion references it via `Expect(sundayWeeklyAllowList).To(HaveLen(9))`; the non-weekly spec references it via `Expect(sundayWeeklyAllowList).NotTo(ContainElement(def.Slug))`).
6. `grep -n 'sundayWeeklyAllowList' pkg/schedule/inventory.go` — must return no matches (the allow-list is a test-only artifact; it must not leak into production code).
7. `grep -n 'AllDefinitionsForTest' pkg/schedule/inventory_export_test.go` — must return exactly 1 match (the test accessor itself, unchanged).
8. `grep -nE '"f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a"' pkg/publisher/uuid_namespace.go` — must return exactly 1 match. The frozen namespace constant is byte-identical.
9. `grep -nE 'Weekday time\.Weekday' pkg/schedule/task_definition.go` — must return exactly 1 match. The `TaskDefinition` struct field is unchanged from Prompt 1.
10. Spot-check: open `pkg/schedule/inventory.go` and visually confirm the 12 `Weekday: time.Saturday,` lines and 9 `Weekday: time.Sunday,` lines, in declaration order, with the `goimports-reviser` colon alignment preserved.
11. Spot-check: open `pkg/schedule/inventory_validation_test.go` and visually confirm (a) the `sundayWeeklyAllowList` var is at package level, before the `Describe` block, with the 9 slugs in declaration order; (b) the three new `It` blocks are inside the existing `Describe` block; (c) the existing three `It` blocks are unchanged.
12. Coverage check on the changed package:
    - `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/schedule/...`
    - `go tool cover -func=/tmp/cover.out | tail -1` — total coverage ≥ 80%.
13. End-to-end smoke: with the data updates in this prompt, the publisher's `weekdayAbbrev` helper now produces `sat` for `time.Saturday` and `sun` for `time.Sunday` for every weekly entry. The `pkg/publisher/publisher_test.go` test `It("buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday", ...)` (added in Prompt 1) continues to pass. Run `go test ./pkg/publisher/...` to confirm.

## Open Questions (for the human reviewer)

- **A. The `sundayWeeklyAllowList` declaration location.** The list is declared in the test file (`package schedule_test`), not in the production package. An alternative is to declare it as a `var` in `pkg/schedule/inventory.go` (production code), with a comment that it is "test-facing" — but that pollutes the production surface with a test artifact. The current placement keeps the test-only data test-only. Review this if you prefer the production-side declaration.
- **B. The order of the 9 slugs in `sundayWeeklyAllowList`.** The list is in inventory-declaration order (matching the order the slugs appear in `inventory.go`). The order does not affect the test's correctness — the validation specs iterate over the inventory and look up each slug in the allow-list, not vice versa. A future reviewer may sort the list alphabetically for readability; that is a no-op for the tests. No action needed.
- **C. The `import "time"` line in `pkg/schedule/inventory.go`.** The `time` import was originally added for the deleted `onWeekdayDay5OfMonth` function (removed in Prompt 1). After this prompt, the only `time.X` references in the file are the new `time.Saturday` and `time.Sunday` constants in the 21 `Weekday:` field lines. The import stays. Do NOT remove it.
- **D. The `time` import in `pkg/schedule/inventory_validation_test.go`.** The test file's existing imports are `"regexp"`, `"github.com/onsi/ginkgo/v2"`, `"github.com/onsi/gomega"`, and the internal `schedule` package. The new specs use `time.Saturday` and `time.Sunday` constants, so `"time"` must be added to the import block. The `goimports-reviser` linter enforces the standard-library-first ordering.
- **E. No scenario file.** The spec's Acceptance Criteria are all reachable from Ginkgo unit tests in the schedule / publisher / handler packages. No real Kafka, no real vault, no real clock. No `scenarios/` work is part of this spec or this prompt.
- **F. The deterministic-identifier guarantee for the updated data.** Two weekly entries with the same slug and the same `Weekday` (the spec's Failure Mode row 5) collapse to the existing canonical-slugs uniqueness test (`It("has unique slugs", ...)`) which is preserved. No new check is added. The existing failure mode catches this case.

</verification>
