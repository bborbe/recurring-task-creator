---
status: approved
spec: [013-bug-weekday-oneof-not-structural]
created: "2026-06-25T08:11:00Z"
queued: "2026-06-25T08:15:54Z"
---

<summary>
- Exposes the new `weekdays` list field end-to-end in Go so operators can apply a single CR targeting multiple weekdays.
- The `weekday` field's Go type returns to a plain `string` (its pre-spec-012 form); the bespoke string-or-array `WeekdayList` type is removed.
- The store adapter now reads from either the single `weekday` string or the new `weekdays` list, both converging on the same internal canonical weekday set the publisher already consumes.
- A single-day CR (`weekday: Monday`) keeps producing byte-identical period token, UUID5, title, and body — no regression to existing schedules.
- A list CR (`weekdays: [Mon, Wed, Fri]`) produces the same multi-day end state spec 012 promised.
- CHANGELOG records the structural-schema bug fix.
- Depends on the schema-reshape prompt landing first.
</summary>

<objective>
Widen the Go side to match the reshaped CRD: change `ScheduleTrigger.Weekday` back to a plain `string`, add a new `Weekdays []string` field, remove the now-unused `WeekdayList` type, update the generated apply-configuration and deepcopy to carry the new field, and update the store adapter to read from either field — both converging on the existing internal `[]time.Weekday` output the publisher consumes. Single-string CRs stay byte-identically backward compatible.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This prompt DEPENDS ON the CRD-schema-reshape prompt (`1-spec-013-crd-schema-reshape-and-cel.md`) having already landed: the CRD schema now exposes `weekday: string` (7 long forms) and `weekdays: []string` (14-value enum). If `grep -qn '"weekdays"' /workspace/pkg/k8s_connector_schema.go` returns no match (exit non-zero), that prompt has not landed — STOP and report `status: failed` with summary "schema reshape (prompt 1) not yet deployed". (Do not rely on `grep -n 'weekdays'` without quotes — that matches Description strings in the current broken schema and gives a false positive.)

Read these files fully before changing anything:
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` — the `WeekdayList` type (with custom `UnmarshalJSON`/`MarshalJSON`) and the `ScheduleTrigger` struct. `ScheduleTrigger.Weekday` is currently `WeekdayList`.
- `/workspace/pkg/store/adapter.go` — `adaptSchedule(ctx, cr)`. Note the `weekdayByName` map (14 entries, long + short → `time.Weekday`) and the dedup loop over `cr.Spec.Schedule.Weekday`. The internal output is `schedule.TaskDefinition.Weekdays []time.Weekday`.
- `/workspace/pkg/store/adapter_test.go` — the adapter tests. They currently build fixtures with `Weekday: v1.WeekdayList{...}`. Note `store.AdaptScheduleForTest`.
- `/workspace/k8s/client/applyconfiguration/task.benjamin-borbe.de/v1/scheduletrigger.go` — generated apply-config with `Weekday *apisv1.WeekdayList` and `WithWeekday(...)`.
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go` — `ScheduleTrigger.DeepCopyInto` (lines ~125-132) currently copies the `WeekdayList` slice.

Coding guides:
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` / `Errorf(ctx, ...)`; never `fmt.Errorf`; never `context.Background()` in business logic.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — CHANGELOG entry format.
</context>

<requirements>

### 1. Change `ScheduleTrigger.Weekday` to `string` and add `Weekdays []string`

In `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go`, edit the `ScheduleTrigger` struct so:

- `Weekday` becomes a plain `string` (was `WeekdayList`):

```go
// Weekday is a single weekday (long form Monday..Sunday). Set when
// Recurrence == "Weekday" and the schedule targets exactly one day;
// mutually exclusive with Weekdays (the CEL XOR rule in
// scheduleTriggerSchema enforces exactly-one-of on Weekday recurrence,
// neither otherwise). Normalized to a canonical time.Weekday Go-side by
// the store adapter.
Weekday string `json:"weekday,omitempty"`

// Weekdays is a non-empty list of weekdays (long form Monday..Sunday or
// short form Mon..Sun, mixable). Set when Recurrence == "Weekday" and the
// schedule targets multiple days; mutually exclusive with Weekday.
// Normalized and deduplicated to a canonical time.Weekday set Go-side by
// the store adapter.
Weekdays []string `json:"weekdays,omitempty"`
```

### 2. Remove the `WeekdayList` type

In the same file, remove the `WeekdayList` type declaration and its `UnmarshalJSON` and `MarshalJSON` methods entirely. The `weekday` field is now a plain string, so the bespoke string-or-array union type is no longer needed. After removal, run `grep -rn 'WeekdayList' /workspace/k8s /workspace/pkg` and update EVERY remaining reference (apply-config, deepcopy, adapter, tests) per the steps below — there must be zero references to `WeekdayList` when this prompt is done.

If removing the `WeekdayList` type leaves the `bytes` and/or `encoding/json` imports unused in `types.go`, remove those imports.

### 3. Update the generated apply-configuration

In `/workspace/k8s/client/applyconfiguration/task.benjamin-borbe.de/v1/scheduletrigger.go`:

- Change the `Weekday` field from `*apisv1.WeekdayList` to `*string`.
- Change `WithWeekday`'s parameter from `apisv1.WeekdayList` to `string`.
- Add a `Weekdays *[]string` field with `json:"weekdays,omitempty"` and a `WithWeekdays(value []string)` builder method, mirroring the existing `Weekday`/`WithWeekday` pattern:

```go
// Weekdays is a non-empty list of weekdays ... (mirror the apis types.go doc).
Weekdays *[]string `json:"weekdays,omitempty"`
```

```go
// WithWeekdays sets the Weekdays field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Weekdays field is set to the value of the last call.
func (b *ScheduleTriggerApplyConfiguration) WithWeekdays(
	value []string,
) *ScheduleTriggerApplyConfiguration {
	b.Weekdays = &value
	return b
}
```

If, after changing `Weekday` to `*string`, the `apisv1` import becomes unused in this file, remove it.

This file is marked `// Code generated ... DO NOT EDIT.` but is committed and hand-maintained in this repo (there is no codegen step wired into `make`); editing it by hand to mirror the type change is correct and expected.

### 4. Update the deepcopy

In `/workspace/k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go`, update `ScheduleTrigger.DeepCopyInto`. `Weekday` is now a plain `string` value type (copied by `*out = *in`, no special handling). The new `Weekdays []string` slice needs a deep copy:

```go
// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScheduleTrigger) DeepCopyInto(out *ScheduleTrigger) {
	*out = *in
	if in.Weekdays != nil {
		in, out := &in.Weekdays, &out.Weekdays
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}
```

Remove the old `if in.Weekday != nil { ... make(WeekdayList, ...) ... }` block.

### 5. Update the store adapter to read either field

In `/workspace/pkg/store/adapter.go`, replace the weekday-resolution loop. The adapter must:

- If `cr.Spec.Schedule.Weekday` is non-empty (single string), treat it as a one-element list.
- Else if `cr.Spec.Schedule.Weekdays` is non-empty (list), use that list.
- Else (neither set — valid for non-Weekday kinds), produce a nil/empty `[]time.Weekday`.

Both paths feed the EXISTING `weekdayByName` map lookup + dedup loop, producing the same `[]time.Weekday` output. Keep `weekdayByName` byte-unchanged (14 entries). Keep the existing `errors.Errorf(ctx, "unknown weekday %q in schedule %q", name, cr.Name)` 3-arg wrapping on an unknown day. Example resolution:

```go
var names []string
switch {
case cr.Spec.Schedule.Weekday != "":
	names = []string{cr.Spec.Schedule.Weekday}
case len(cr.Spec.Schedule.Weekdays) > 0:
	names = cr.Spec.Schedule.Weekdays
}

var weekdays []time.Weekday
seen := map[time.Weekday]bool{}
for _, name := range names {
	wd, ok := weekdayByName[name]
	if !ok {
		return schedule.TaskDefinition{}, errors.Errorf(
			ctx,
			"unknown weekday %q in schedule %q",
			name, cr.Name,
		)
	}
	if seen[wd] {
		continue
	}
	seen[wd] = true
	weekdays = append(weekdays, wd)
}
```

The `schedule.TaskDefinition{...}` return literal is unchanged (it still sets `Weekdays: weekdays`). Do NOT change `pkg/publisher/`, `pkg/schedule/`, or the matcher — they consume `TaskDefinition.Weekdays []time.Weekday` and are unaffected.

### 6. Update the adapter tests

In `/workspace/pkg/store/adapter_test.go`:

- Replace every `Weekday: v1.WeekdayList{input}` fixture. For single-day cases use the plain string field: `Weekday: input` (where the existing 14-element normalization `DescribeTable` drives each of the 14 day strings). Note: a single short-form string like `Mon` is still a VALID adapter input here — the adapter's `weekdayByName` map accepts all 14 forms; the CRD schema's long-form-only restriction on `weekday` is an API-server concern, not an adapter concern, so keep all 14 single-string entries in the normalization table to lock the map.
- Replace multi-day list fixtures (e.g. `v1.WeekdayList{"Mon", "Wednesday", "Fri"}`) with the new `Weekdays` field: `Weekdays: []string{"Mon", "Wednesday", "Fri"}`.
- Replace the unknown-weekday fixture `Weekday: v1.WeekdayList{"Funday"}` — use `Weekday: "Funday"` (single-string path) AND add a sibling case `Weekdays: []string{"Funday"}` (list path), both asserting the `"unknown weekday"` error.
- Add a regression case asserting the single-string path and the equivalent one-element list path produce identical `def.Weekdays`: e.g. `Weekday: "Monday"` and `Weekdays: []string{"Monday"}` both yield `[]time.Weekday{time.Monday}`.
- Keep all other adapter tests (recurrence mapping, frontmatter, PeriodOffset, slug/title/body) unchanged in behavior.

The adapter tests are in the external `store_test` package and may use `context.Background()` (test code is exempt from the no-`context.Background()` rule).

### 6a. Sweep remaining `WeekdayList` references in other test files

After Step 2 deletes the `WeekdayList` type, several test files still reference it. Update each:

a. **`/workspace/k8s/apis/task.benjamin-borbe.de/v1/example_test.go`**:
   - Change line ~59 from `Equal(v1.WeekdayList{"Saturday"})` to `Equal("Saturday")` (the `Weekday` field is now a plain string).
   - Delete the entire `Describe("WeekdayList", ...)` block (covers ~lines 87-144) — it tests the bespoke `UnmarshalJSON`/`MarshalJSON` round-trip for a type that no longer exists.
   - If this leaves `bytes` or `encoding/json` imports unused, remove them.

b. **`/workspace/pkg/store/store_integration_test.go`**:
   - Change line ~56 from `Weekday: v1.WeekdayList{"Friday"}` to `Weekday: "Friday"` (single-string path).
   - If any other test in this file uses `v1.WeekdayList{...}` for a multi-day fixture, convert it to `Weekdays: []string{...}` per the adapter test rules above.

c. **Verification gate (already in Step 2):** after these edits run `grep -rn 'WeekdayList' /workspace/k8s /workspace/pkg` — output MUST be empty. If any reference remains, fix it; do NOT exit until the grep is clean.

### 6b. Update `docs/architecture.md` for the two-field schema

In `/workspace/docs/architecture.md`:
- Line ~54 (the CR snippet currently showing `weekday: Saturday  # required iff recurrence == Weekday (Monday..Sunday)`): update the comment to reflect the two-field rule, e.g. `weekday: Saturday  # one of {weekday, weekdays} required iff recurrence == Weekday`. Add an alternative form line below it showing the list shape, e.g. `# weekdays: [Mon, Wed, Fri]  # alternative: list form (short or long names mix)`.
- Line ~69 (the prose "`weekday` is required iff recurrence == `Weekday`"): rewrite to "exactly one of `weekday` (single long-form day) or `weekdays` (non-empty list of long-or-short day names) is required iff recurrence == `Weekday`; both fields rejected on other recurrences."
- Keep the rest of the doc untouched.

### 7. Add a CHANGELOG entry

In `/workspace/CHANGELOG.md`, under the `## Unreleased` heading (create it if absent, immediately below the top title), add:

```
- fix: Replace structurally-invalid oneOf weekday CRD schema with type-pure `weekday` string + new `weekdays` list field; CEL enforces exactly-one-of on Weekday recurrence. Fixes dev-pod CrashLoopBackOff on CRD install.
```

If `## Unreleased` already exists, append the bullet to it (do not replace existing entries).

### 8. Backward-compatibility lock

The 21-entry UUID5 stability block in `/workspace/pkg/publisher/publisher_test.go` MUST still pass unchanged — do NOT edit that file. A single-string `weekday: Monday` CR must produce a one-element `[]time.Weekday{time.Monday}` from the adapter, identical to pre-spec-012, which feeds the unchanged publisher to the same UUID5, period token, title, and body. If `go test ./pkg/publisher/...` shows any UUID5 stability failure, your adapter change altered the canonical weekday output — fix the adapter, not the publisher test.

### 9. Error paths

The only new error path is the unknown-weekday lookup in step 5, already wrapped with `errors.Errorf(ctx, ...)`. No `fmt.Errorf`, no `context.Background()` in `adaptSchedule`. The `ctx` parameter is already threaded into `adaptSchedule` — use it.
</requirements>

<constraints>
- The CRD field names are frozen except for the additive `weekdays` field. `Weekday` keeps its name; its Go TYPE changes from `WeekdayList` to `string`.
- `weekdayByName` (the 14-entry normalization map in `pkg/store/adapter.go`) is unchanged byte-for-byte.
- `pkg/publisher/`, `pkg/schedule/`, the day-set matcher, period-token rendering, and UUID5 derivation are unchanged. Single-string CRs MUST produce byte-identical UUID5 / period token / title / body.
- The 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` is not edited and must still pass.
- Do NOT add a config knob to disable the `weekdays` field. It is additive and unconditional.
- Do NOT add a CRD `Status` subresource or status writeback.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg `Wrap(ctx, err, msg)`/`Errorf(ctx, ...)` on every business-logic error path; no `fmt.Errorf`; no `time.Now()` / `context.Background()` in business logic.
- Coverage ≥80% for the changed adapter package; test both the single-string and list parse paths and the unknown-day error path for both.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm no remaining references to the removed type:

```bash
cd /workspace && grep -rn 'WeekdayList' k8s pkg
```

Must return zero matches.

Targeted tests — adapter both-paths and publisher stability:

```bash
cd /workspace && go test -v -run 'adaptSchedule|Weekdays|UUID5 stability' ./pkg/...
```

Confirm:
- Single-string `weekday: Monday` and one-element `weekdays: [Monday]` produce identical `def.Weekdays`.
- List `weekdays: [Mon, Wed, Fri]` normalizes/dedups correctly.
- Unknown day errors on both the single-string and list paths.
- The 21-entry UUID5 stability block PASSES.

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize.
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

Deploy note for the human reviewer (post-merge, out of container scope): after this lands on master and merges to dev, restart the dev statefulset and confirm the pod returns to Running and the new `weekdays` list CR applies — per the spec's Bug-spec verification section. The container cannot run `kubectlquant`; this is a host-side step.

## Improvements

- (fill in per the reflection rules; write `- None` if nothing)
</completion>
