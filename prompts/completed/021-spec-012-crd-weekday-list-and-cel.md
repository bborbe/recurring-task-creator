---
status: completed
spec: [012-weekday-list-and-short-forms]
summary: Widened Schedule CRD weekday field to string-or-array OneOf, added 14-value enum, two new CEL rules (non-empty list, no-duplicate), ForTest accessors, and comprehensive Ginkgo tests including structural schema round-trip
container: recurring-task-creator-weekday-list-exec-021-spec-012-crd-weekday-list-and-cel
dark-factory-version: v0.182.0
created: "2026-06-24T20:30:00Z"
queued: "2026-06-24T21:06:11Z"
started: "2026-06-24T21:06:12Z"
completed: "2026-06-24T21:21:17Z"
branch: dark-factory/weekday-list-and-short-forms
---

<summary>
- The `Schedule` CRD's `spec.schedule.weekday` field can now be a single day OR a list of days — one CR can target Mon-Fri instead of five near-identical CRs.
- Both long names (`Monday`..`Sunday`) and short names (`Mon`..`Sun`) are accepted, and the two forms may be mixed in one list.
- An empty list (`weekday: []`) is rejected at `kubectl apply` time.
- A list with the same logical day twice — including cross-form duplicates like `[Mon, Monday]` — is rejected at apply time.
- An unknown day string (e.g. `FunDay`) is still rejected by the enum.
- A `weekday` value (string or list) on any non-`Weekday` recurrence is still rejected by the existing presence rule; a `Weekday` recurrence without a `weekday` is still rejected.
- Existing single-string CRs (`weekday: Monday`) keep applying unchanged.
- This prompt touches ONLY the Go-built CRD schema and its validation tests — no adapter, publisher, or runtime behavior changes yet (that is Prompt 2).
</summary>

<objective>
Widen the Go-built `Schedule` CRD OpenAPI schema so `spec.schedule.weekday` accepts a single day string OR a non-empty list of day strings, extend the accepted day enum to 14 values (long + short forms), and add CEL validation rejecting empty lists and duplicate days (same-form and cross-form), while preserving the existing presence-iff-`Weekday` rule. No Go-side adapter or publisher changes in this prompt.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:
- `/workspace/pkg/k8s_connector_schema.go` — the canonical Go-built CRD schema. Note `weekdayEnum`, `weekdayRequiredIfWeekdayRule`, `weekdayRequiredIfWeekdayMessage`, `scheduleSpecSchema()`, `jsonEnumValues()`, `ptrTrue()`. The `weekday` property is currently `{Type: "string", Enum: jsonEnumValues(weekdayEnum)}`.
- `/workspace/pkg/k8s_connector_export_test.go` — the `*ForTest` accessors that expose unexported schema symbols to the external `pkg_test` package. You will add accessors here for any new unexported symbol referenced by tests.
- `/workspace/pkg/k8s_connector_validation_test.go` — the existing Ginkgo validation tests. Note `buildSelfForCEL`, `evalRule`, `validateSpec`, and the `DescribeTable` style. The CEL rule is evaluated in-process via `github.com/google/cel-go/cel`.
- `/workspace/pkg/k8s_connector.go` — `SetupCustomResourceDefinition` (line ~67) registers the schema; no change needed here, but confirm the schema flows through it.

Coding guides (read the ones relevant to CRD schema + CEL + testing):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` — CRD schema + CEL `x-kubernetes-validations` patterns.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` 3-arg `Wrap(ctx, err, msg)`.

Open question surfaced for the reviewer (resolved here; flagging for visibility): the spec leaves "additional CEL rule vs extended rule" to the agent. This prompt adds the empty-list and duplicate-day checks as SEPARATE CEL rules appended to `schedule.XValidations`, leaving `weekdayRequiredIfWeekdayRule` (and its message) byte-unchanged per the Constraints section. Do not edit the existing rule string or message.
</context>

<requirements>

### 1. Verify the apiextensions schema fields you will use actually exist

Before writing schema code, grep-verify these `JSONSchemaProps` fields exist in the apiextensions module so you do not invent field names:

```bash
cd /workspace
MODDIR=$(go list -m -f '{{.Dir}}' k8s.io/apiextensions-apiserver)
grep -n 'OneOf\|MinItems\|MaxItems\|UniqueItems\|Items \|XValidations\|XPreserveUnknownFields' "$MODDIR/pkg/apis/apiextensions/v1/types_jsonschema.go"
```

Confirm `OneOf []JSONSchemaProps`, `Items *JSONSchemaPropsOrArray`, `MinItems *int64`, `UniqueItems bool`, and `XValidations ValidationRules` are present. The existing `scheduleSpecSchema()` already uses `Type`, `Enum`, `XValidations`, `XPreserveUnknownFields`, `Pattern`, `Properties`, `Required` — those are proven. If `OneOf` or `Items` is spelled differently, use the actual spelling. Do NOT fabricate a field that does not exist.

### 2. Extend the weekday enum to 14 values (long + short forms)

In `/workspace/pkg/k8s_connector_schema.go`, replace the `weekdayEnum` slice so it carries BOTH long form and short form, 14 strings total. Keep the existing GoDoc tone; update it to mention short forms are accepted and normalized Go-side at parse time (Prompt 2).

```go
var weekdayEnum = []string{
	"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday",
	"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun",
}
```

### 3. Widen the `weekday` schema property to string-or-array

In `scheduleSpecSchema()`, replace the current `"weekday"` property:

```go
"weekday": {
	Type:        "string",
	Description: "time.Weekday string (Monday..Sunday). Required when recurrence is 'Weekday'; forbidden otherwise.",
	Enum:        jsonEnumValues(weekdayEnum),
},
```

with a property that accepts EITHER a single enum string OR a non-empty array of enum strings, using `OneOf`. The field itself carries no top-level `Type` (the two `OneOf` branches each declare their own type — this is the standard string-or-array CRD shape). Use this structure (adjust field spellings to match what you verified in step 1):

```go
"weekday": {
	Description: "A single weekday or a non-empty list of weekdays. Each entry is one of the 14 accepted day strings (long form Monday..Sunday or short form Mon..Sun); the two forms may be mixed in one list. Required when recurrence is 'Weekday'; forbidden otherwise. Normalized to canonical time.Weekday values Go-side at parse time.",
	OneOf: []apiextensionsv1.JSONSchemaProps{
		{
			Type: "string",
			Enum: jsonEnumValues(weekdayEnum),
		},
		{
			Type:     "array",
			MinItems: ptrInt64(1),
			Items: &apiextensionsv1.JSONSchemaPropsOrArray{
				Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "string",
					Enum: jsonEnumValues(weekdayEnum),
				},
			},
		},
	},
},
```

Add a helper `ptrInt64` next to `ptrTrue` (the schema field for `MinItems` is `*int64`):

```go
// ptrInt64 returns a pointer to the given int64; the k8s OpenAPI schema
// represents MinItems and similar numeric bounds as *int64.
func ptrInt64(n int64) *int64 {
	return &n
}
```

If `MinItems` enforcement via the array branch alone is insufficient to reject `weekday: []` in the apiserver semantics (an empty array could still satisfy the string branch's failure and fall to the array branch), ALSO add the explicit CEL non-empty rule in step 4 — that is the authoritative empty-list guard. Add BOTH; do not rely on `MinItems` alone.

### 4. Add CEL rules for non-empty list and duplicate-day rejection

Add two new package-level rule + message constant pairs in `/workspace/pkg/k8s_connector_schema.go`, mirroring the existing `weekdayRequiredIfWeekdayRule` / `weekdayRequiredIfWeekdayMessage` style. Then append them to the `schedule` node's `XValidations` AFTER the existing two rules (do not reorder or edit the existing entries).

`self` is bound to the `ScheduleTrigger` object. `self.weekday` may be a string OR a list. CEL must guard the type before list operations. Use `type(self.weekday) == list` to branch.

Non-empty rule — reject `weekday: []`:

```go
// weekdayListNonEmptyRule rejects an empty weekday list. Only applies
// when weekday is present AND is a list; a single string is never empty
// in this sense. self.weekday is the string-or-list union from the
// OpenAPI OneOf branch.
const weekdayListNonEmptyRule = "!has(self.weekday) || type(self.weekday) != list || size(self.weekday) > 0"

const weekdayListNonEmptyMessage = "weekday list must be non-empty"
```

Duplicate-day rule — reject same-form (`[Tue, Tue]`) AND cross-form (`[Mon, Monday]`) duplicates. Normalize each entry to its canonical long form inside CEL via a literal map, then assert the normalized set has no duplicates by comparing sizes of the mapped list vs its distinct set.

CEL list comprehensions do not offer a distinct() builtin in all apiserver versions; assert no-duplicates by checking that for the canonicalized list, every element's first index equals its last index is not expressible directly, so use the size-vs-toSet idiom CEL supports: `size(list) == size(list.map(x, x))` is a no-op. The reliable, apiserver-supported idiom is:

```go
// weekdayNoDuplicateRule rejects a weekday list that names the same
// logical day twice, including cross-form duplicates ([Mon, Monday]).
// Each entry is canonicalized to its long form via a literal map, then
// the rule asserts the canonical list has no element appearing more than
// once. Only applies when weekday is a list.
const weekdayNoDuplicateRule = "!has(self.weekday) || type(self.weekday) != list || " +
	"self.weekday.map(d, " +
	"{'Mon':'Monday','Tue':'Tuesday','Wed':'Wednesday','Thu':'Thursday'," +
	"'Fri':'Friday','Sat':'Saturday','Sun':'Sunday'," +
	"'Monday':'Monday','Tuesday':'Tuesday','Wednesday':'Wednesday'," +
	"'Thursday':'Thursday','Friday':'Friday','Saturday':'Saturday'," +
	"'Sunday':'Sunday'}[d])" +
	".all(c, self.weekday.map(d2, " +
	"{'Mon':'Monday','Tue':'Tuesday','Wed':'Wednesday','Thu':'Thursday'," +
	"'Fri':'Friday','Sat':'Saturday','Sun':'Sunday'," +
	"'Monday':'Monday','Tuesday':'Tuesday','Wednesday':'Wednesday'," +
	"'Thursday':'Thursday','Friday':'Friday','Saturday':'Saturday'," +
	"'Sunday':'Sunday'}[d2]).exists_one(c2, c2 == c))"

const weekdayNoDuplicateMessage = "weekday list must not contain the same day twice (including cross-form duplicates like [Mon, Monday])"
```

The `exists_one` idiom asserts each canonical value appears exactly once across the canonicalized list — a duplicate makes `exists_one` false for the offending element, failing `.all(...)`. Verify this compiles in the test (step 6) BEFORE relying on it; if `exists_one` or `.map(...)` is rejected by the cel-go env used in tests, fall back to the equivalent index-based form `... .all(i, ...)` using `size()` and indexed access, but keep the canonicalization map. Pick ONE working form and commit to it; do not leave both in the source.

Append to `XValidations` in `scheduleSpecSchema()`:

```go
XValidations: apiextensionsv1.ValidationRules{
	{Rule: weekdayRequiredIfWeekdayRule, Message: weekdayRequiredIfWeekdayMessage},
	{Rule: periodOffsetOnlyForPeriodKindsRule, Message: periodOffsetOnlyForPeriodKindsMessage},
	{Rule: weekdayListNonEmptyRule, Message: weekdayListNonEmptyMessage},
	{Rule: weekdayNoDuplicateRule, Message: weekdayNoDuplicateMessage},
},
```

### 5. Add `*ForTest` accessors for the new symbols

In `/workspace/pkg/k8s_connector_export_test.go`, add accessors mirroring the existing ones so the external `pkg_test` package can read the new rules/messages:

```go
// WeekdayListNonEmptyRuleForTest returns the CEL rule rejecting empty weekday lists.
func WeekdayListNonEmptyRuleForTest() string { return weekdayListNonEmptyRule }

// WeekdayListNonEmptyMessageForTest returns the operator-facing empty-list message.
func WeekdayListNonEmptyMessageForTest() string { return weekdayListNonEmptyMessage }

// WeekdayNoDuplicateRuleForTest returns the CEL rule rejecting duplicate weekday entries.
func WeekdayNoDuplicateRuleForTest() string { return weekdayNoDuplicateRule }

// WeekdayNoDuplicateMessageForTest returns the operator-facing duplicate-day message.
func WeekdayNoDuplicateMessageForTest() string { return weekdayNoDuplicateMessage }
```

`WeekdayEnumForTest()` already exists and now returns the 14-element slice — no signature change.

### 6. Extend the validation tests

In `/workspace/pkg/k8s_connector_validation_test.go`:

a. The existing `evalRule` helper binds `self` as `cel.MapType(cel.StringType, cel.StringType)`. The new list rules require `self.weekday` to be a list. Add a SECOND eval helper (e.g. `evalListRule`) that binds `self` as `cel.MapType(cel.StringType, cel.DynType)` (mirror the `periodOffset` test's env at line ~206 which already uses `cel.DynType`), compiles the given rule, and returns the message on failure / "" on pass. Use this for `weekdayListNonEmptyRule` and `weekdayNoDuplicateRule`. Keep the existing `evalRule` for `weekdayRequiredIfWeekdayRule` unchanged.

b. Add a `DescribeTable` asserting all 14 day strings are accepted as the SINGLE-STRING `weekday` on a `Weekday` recurrence (drive `validateSpec` or the enum directly). Cover all 14: `Monday`..`Sunday`, `Mon`..`Sun`.

c. Add a `DescribeTable` for the non-empty rule: `[]` rejected; `["Mon"]` accepted; `["Mon","Wed"]` accepted; a single string `"Monday"` accepted (rule must not fire for non-list).

d. Add a `DescribeTable` for the duplicate rule covering: `["Mon","Monday"]` rejected; `["Monday","Mon"]` rejected; `["Tue","Tue"]` rejected; `["Wednesday","Wednesday"]` rejected; `["Mon","Wed","Fri"]` accepted; `["Mon","Tue","Wednesday","Thu","Fri"]` accepted; single string `"Monday"` accepted (rule must not fire). Build the `self` map with `weekday` as a `[]interface{}` of strings for the list cases.

e. Add an enum-rejection case: `["Mon","FunDay"]` — assert `FunDay` is not in `WeekdayEnumForTest()` (the array-items enum rejects it). A `validateSpec`-style check that iterates list entries against the enum is acceptable; the existing `validateSpec` only handles the string shape, so add a small list-aware enum check in the test for this case.

f. Confirm (do not remove) the existing cases still pass: single-string `Saturday` on `Weekday` accepted; `Weekday` without weekday rejected; `weekday` on `Monthly`/`Weekly` rejected; typo `Satuday` rejected.

g. **Structural-schema round-trip test** — this prompt introduces the first use of `OneOf` in `pkg/k8s_connector_schema.go` (`grep -n OneOf pkg/k8s_connector_schema.go` returns zero hits today). The `OneOf{string, array}` shape with nested `Items.Schema.Enum` and `MinItems` is the highest-risk surface — silent misshape would be accepted by the in-process CEL env but rejected (or quietly ignored) by the apiserver's structural-schema validator at CRD-install time. Add ONE Ginkgo `It` that builds the full schema via `scheduleCRSchemaPtr()` (or the equivalent test helper), converts to `apiextensions.JSONSchemaProps` via `apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps`, and runs it through `k8s.io/apiextensions-apiserver/pkg/apiserver/schema.NewStructural(...)` — assert no error. Mirrors the path the apiserver uses when admitting a CRD. Cheap, auditable, and catches the one boundary the CEL tests cannot reach.

All new tests use Ginkgo v2 / Gomega and live in the external `pkg_test` package. Coverage for the new schema code paths must be ≥80% (the rules are data + the test exercises them).

### 7. No adapter / publisher / types changes in this prompt

Do NOT touch `pkg/store/adapter.go`, `pkg/publisher/`, `pkg/schedule/task_definition.go`, or `k8s/apis/.../types.go` here. The Go struct field stays `Weekday string` until Prompt 2 — this prompt's schema widening is additive and the apiserver accepts both shapes regardless of the Go decode type used by the controller (the controller does not run admission). If `make precommit` reveals the schema round-trip test in `k8s/apis/.../example_test.go` breaks, STOP and report — it should not, because `example.yaml` uses a single string.

### 8. Error paths

This prompt adds no new Go error-returning code paths (schema is declarative data). The CEL evaluation failures are surfaced by the apiserver as admission rejections — covered by the tests, not by Go error wrapping. No `bborbe/errors` calls are introduced here.
</requirements>

<constraints>
- CRD group, version, kind, plural, singular, short name, and every field NAME are frozen — this prompt only widens the `weekday` field's TYPE, not its name or position.
- The `RecurrenceKind` wire enum (`Daily`, `Weekly`, `Weekday`, `Monthly`, `Quarterly`, `Yearly`) is frozen — do not touch `recurrenceEnum`.
- Preserve `weekdayRequiredIfWeekdayRule` and `weekdayRequiredIfWeekdayMessage` byte-for-byte. The new empty-list and duplicate-day checks are ADDITIONAL `XValidations` entries appended after the existing two.
- Do NOT add any config knob to disable list support or fall back to single-string-only mode. List support is additive and unconditional (spec Non-goal).
- Do NOT add a CRD `Status` subresource or status writeback.
- Period-token format, UUID5 derivation, and the `/trigger` contract are out of scope for this prompt.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega for tests; Counterfeiter v6 for any new fakes (none expected here).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Then targeted:

```bash
cd /workspace && go test ./pkg/...
```

Specifically confirm the new DescribeTables pass (14-element enum, non-empty list, duplicate-day cross-form, enum rejection of `FunDay`).

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

## Improvements

- (fill in per the reflection rules; write `- None` if nothing)
</completion>
