---
status: completed
spec: [013-bug-weekday-oneof-not-structural]
summary: Replaced OneOf{string,array} weekday schema with two type-pure sibling fields (weekday:string 7-enum, weekdays:[]string 14-enum), rewrote CEL XOR rule, updated export accessors, rewrote validation tests for two-field shape, and added ValidateStructural call to structural round-trip test — all 50 pkg tests pass and make precommit exits 0.
container: recurring-task-creator-weekday-list-exec-023-spec-013-crd-schema-reshape-and-cel
dark-factory-version: v0.182.0
created: "2026-06-25T08:10:00Z"
queued: "2026-06-25T08:15:54Z"
started: "2026-06-25T08:16:24Z"
completed: "2026-06-25T08:28:52Z"
---

<summary>
- Fixes the dev-pod CrashLoopBackOff: the `Schedule` CRD no longer uses the `oneOf{string, array}` weekday shape the Kubernetes API server rejects as non-structural.
- `weekday` returns to a single string field accepting only the 7 long-form day names (`Monday`..`Sunday`), exactly as before spec 012.
- A new `weekdays` list field accepts a non-empty list of days (14-value enum: long + short forms), mirroring k8s plural conventions.
- On `Weekday` recurrence, exactly one of `weekday` or `weekdays` must be set; on every other recurrence kind, neither may be set — enforced by CEL.
- Empty lists, duplicate days (including cross-form like `[Mon, Monday]`), and unknown day strings are still rejected at `kubectl apply` time.
- A regression-lock unit test runs the assembled CRD schema through the API server's own structural-schema validator and asserts it is accepted — this test fails on the broken code and passes after this fix.
- This prompt changes ONLY the Go-built CRD schema and its validation tests — no adapter, struct, publisher, or runtime behavior changes (those land in the next prompt).
</summary>

<objective>
Reshape the Go-built `Schedule` CRD OpenAPI schema to drop the structurally-invalid `oneOf{string, array}` on `spec.schedule.weekday`, replacing it with two type-pure sibling fields — `weekday: string` (7 long-form days, pre-spec-012 backward compatible) and `weekdays: []string` (new, 14-value enum, `MinItems: 1`) — and rewrite the CEL rules to enforce exactly-one-of on `Weekday` recurrence and neither on every other kind. This unblocks the dev pod immediately.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:
- `/workspace/pkg/k8s_connector_schema.go` — the canonical Go-built CRD schema. Note `weekdayEnum` (currently 14 strings), `weekdayRequiredIfWeekdayRule`/`Message`, `weekdayListNonEmptyRule`/`Message`, `weekdayNoDuplicateRule`/`Message`, `periodOffsetOnlyForPeriodKindsRule`/`Message`, `scheduleTriggerSchema()`, `scheduleSpecSchema()`, `jsonEnumValues()`, `ptrInt64()`, `ptrTrue()`. The `weekday` property currently uses a `OneOf` block — that is the broken shape.
- `/workspace/pkg/k8s_connector_export_test.go` — the `*ForTest` accessors that expose unexported schema symbols to the external `pkg_test` package.
- `/workspace/pkg/k8s_connector_validation_test.go` — the existing Ginkgo validation tests. Note `buildSelfForCEL`, `evalRule`, `validateSpec`, the `evalListRule` helpers, and the existing structural-schema round-trip `Describe` block at the end (lines ~479-496) — that block ALREADY EXISTS and currently FAILS on the broken code; it will PASS after this fix.

Coding guides (read the ones relevant to CRD schema + CEL + testing):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` — CRD schema + CEL `x-kubernetes-validations` patterns.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.

Why the current shape is broken (from the spec): Kubernetes structural-schema validation forbids `oneOf` branches that each carry their own `type`, and forbids a property whose top-level `type` is empty. "String OR array of strings" is not expressible in a structural schema. The only valid shape is two separate, type-pure fields.
</context>

<requirements>

### 1. Replace the single 14-element enum with two purpose-specific enums

In `/workspace/pkg/k8s_connector_schema.go`, remove the existing `weekdayEnum` var and replace it with two vars:

```go
// weekdayLongEnum is the closed set of valid strings for the single
// `weekday` field — 7 long forms only (Monday..Sunday, matching
// time.Weekday.String()). This is the pre-Spec-012 enum, restored so a
// single-day CR keeps its exact backward-compatible shape. Short forms
// are NOT accepted on this field; multi-day or short-form usage moves to
// the `weekdays` list field below.
var weekdayLongEnum = []string{
	"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday",
}

// weekdayAllEnum is the closed set of valid item strings for the new
// `weekdays` list field — both long forms (Monday..Sunday) and short
// forms (Mon..Sun), 14 strings total, freely mixable in one list. Short
// forms are normalized to long form Go-side at parse time (Prompt 2).
// Locked in v1 — typos like "Satuday" or "FunDay" are rejected at the
// API-server boundary by the item enum.
var weekdayAllEnum = []string{
	"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday",
	"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun",
}
```

### 2. Reshape the `weekday` property and add the `weekdays` property in `scheduleTriggerSchema()`

In `scheduleTriggerSchema()`, replace the current `"weekday"` property (the `OneOf` block) with a type-pure single-string field, and add a new sibling `"weekdays"` array field:

```go
"weekday": {
	Type:        "string",
	Description: "A single weekday (long form Monday..Sunday). Required-XOR with weekdays when recurrence is 'Weekday'; both fields forbidden otherwise. Normalized to a canonical time.Weekday Go-side at parse time.",
	Enum:        jsonEnumValues(weekdayLongEnum),
},
"weekdays": {
	Type:        "array",
	Description: "A non-empty list of weekdays. Each entry is one of the 14 accepted day strings (long form Monday..Sunday or short form Mon..Sun); the two forms may be mixed in one list. Required-XOR with weekday when recurrence is 'Weekday'; both fields forbidden otherwise. Normalized to canonical time.Weekday values Go-side at parse time.",
	MinItems:    ptrInt64(1),
	Items: &apiextensionsv1.JSONSchemaPropsOrArray{
		Schema: &apiextensionsv1.JSONSchemaProps{
			Type: "string",
			Enum: jsonEnumValues(weekdayAllEnum),
		},
	},
},
```

Keep the `recurrence` and `periodOffset` properties exactly as they are.

### 3. Replace the presence rule with a two-field XOR rule

Remove the `weekdayRequiredIfWeekdayRule` and `weekdayRequiredIfWeekdayMessage` constants and replace them with a two-field XOR equivalent. `self` is bound to the `ScheduleTrigger` object; `has(self.weekday)`/`has(self.weekdays)` check field presence.

```go
// weekdayXorRule is the CEL rule encoded in schedule.XValidations. self
// is bound to the ScheduleTrigger object. On 'Weekday' recurrence,
// exactly one of weekday/weekdays must be set (XOR via inequality of the
// two has() booleans). On every other recurrence kind, neither may be
// set. This replaces the pre-Spec-012 single-field presence rule with a
// two-field equivalent of the same intent.
const weekdayXorRule = "self.recurrence == 'Weekday' ? " +
	"(has(self.weekday) != has(self.weekdays)) : " +
	"(!has(self.weekday) && !has(self.weekdays))"

// weekdayXorMessage is the human-readable error the API server emits
// when the rule fails. Surfaced to the operator via kubectl apply output.
const weekdayXorMessage = "exactly one of weekday or weekdays is required when recurrence is 'Weekday', and both are forbidden otherwise"
```

### 4. Remove the now-redundant non-empty list rule

Remove the `weekdayListNonEmptyRule` and `weekdayListNonEmptyMessage` constants entirely. The empty-list guard is now handled declaratively by `MinItems: ptrInt64(1)` on the `weekdays` array property (step 2) — the API server rejects an empty `weekdays` list via the OpenAPI `minItems` constraint, so the CEL rule is redundant.

### 5. Repoint the no-duplicate rule from `self.weekday` to `self.weekdays`

The duplicate-day check now applies to the `weekdays` list field. Update `weekdayNoDuplicateRule` so every occurrence of `self.weekday` becomes `self.weekdays`. The canonicalization map and the `.map(...).all(...).exists_one(...)` structure stay identical; only the field name changes. Keep `weekdayNoDuplicateMessage` byte-unchanged. Resulting rule:

```go
// weekdayNoDuplicateRule rejects a weekdays list that names the same
// logical day twice, including cross-form duplicates ([Mon, Monday]).
// Each entry is canonicalized to its long form via a literal map, then
// the rule asserts each canonical value appears exactly once. Only
// applies when weekdays is present.
const weekdayNoDuplicateRule = "!has(self.weekdays) || type(self.weekdays) != list || " +
	"self.weekdays.map(d, " +
	"{'Mon':'Monday','Tue':'Tuesday','Wed':'Wednesday','Thu':'Thursday'," +
	"'Fri':'Friday','Sat':'Saturday','Sun':'Sunday'," +
	"'Monday':'Monday','Tuesday':'Tuesday','Wednesday':'Wednesday'," +
	"'Thursday':'Thursday','Friday':'Friday','Saturday':'Saturday'," +
	"'Sunday':'Sunday'}[d])" +
	".all(c, self.weekdays.map(d2, " +
	"{'Mon':'Monday','Tue':'Tuesday','Wed':'Wednesday','Thu':'Thursday'," +
	"'Fri':'Friday','Sat':'Saturday','Sun':'Sunday'," +
	"'Monday':'Monday','Tuesday':'Tuesday','Wednesday':'Wednesday'," +
	"'Thursday':'Thursday','Friday':'Friday','Saturday':'Saturday'," +
	"'Sunday':'Sunday'}[d2]).exists_one(c2, c2 == c))"
```

If the cel-go env in the tests rejects `exists_one` or `.map(...)` for the `self.weekdays` binding, keep the same fallback discipline as Prompt 021 used — pick ONE working form and commit to it; do not leave two forms in the source.

### 6. Update the XValidations list order

In `scheduleTriggerSchema()`, set `XValidations` to exactly three entries in this order (the non-empty rule is gone):

```go
XValidations: apiextensionsv1.ValidationRules{
	{Rule: weekdayXorRule, Message: weekdayXorMessage},
	{
		Rule:    periodOffsetOnlyForPeriodKindsRule,
		Message: periodOffsetOnlyForPeriodKindsMessage,
	},
	{Rule: weekdayNoDuplicateRule, Message: weekdayNoDuplicateMessage},
},
```

`periodOffsetOnlyForPeriodKindsRule`/`Message` are unchanged — do not touch them.

### 7. Update the `*ForTest` accessors

In `/workspace/pkg/k8s_connector_export_test.go`:

a. Remove `WeekdayRequiredIfWeekdayRuleForTest()`, `WeekdayRequiredIfWeekdayMessageForTest()`, `WeekdayListNonEmptyRuleForTest()`, and `WeekdayListNonEmptyMessageForTest()` — the underlying constants no longer exist.

b. Replace `WeekdayEnumForTest()` with two accessors exposing the new enums:

```go
// WeekdayLongEnumForTest returns the closed set of valid strings for the
// single `weekday` field (7 long forms).
func WeekdayLongEnumForTest() []string { return weekdayLongEnum }

// WeekdayAllEnumForTest returns the closed set of valid item strings for
// the `weekdays` list field (14 long + short forms).
func WeekdayAllEnumForTest() []string { return weekdayAllEnum }
```

c. Add accessors for the new XOR rule/message:

```go
// WeekdayXorRuleForTest returns the CEL XOR rule from XValidations[0].
func WeekdayXorRuleForTest() string { return weekdayXorRule }

// WeekdayXorMessageForTest returns the operator-facing XOR error message.
func WeekdayXorMessageForTest() string { return weekdayXorMessage }
```

d. Keep `WeekdayNoDuplicateRuleForTest()`, `WeekdayNoDuplicateMessageForTest()`, `PeriodOffsetOnlyForPeriodKindsRuleForTest()`, `PeriodOffsetOnlyForPeriodKindsMessageForTest()`, `VaultPatternForTest()`, `RecurrenceEnumForTest()`, `VaultRegexForTest`, and `ScheduleCRSchemaPtrForTest()` unchanged.

### 8. Rewrite the validation tests for the two-field shape

In `/workspace/pkg/k8s_connector_validation_test.go`, every test currently references the removed `WeekdayRequiredIfWeekday*`, `WeekdayListNonEmpty*`, or `WeekdayEnumForTest()` accessors and the single-field `weekday` semantics. Rewrite them for the two-field shape. Use Ginkgo v2 / Gomega in the external `pkg_test` package. The CEL XOR rule references both `self.weekday` and `self.weekdays`, so build the CEL env with `cel.MapType(cel.StringType, cel.DynType)` (as the existing list-rule helpers already do) and omit a key entirely to make `has(...)` false for that field.

Cover this full accept/reject table (each as a `DescribeTable` entry or `It`):

Accept:
- `recurrence: Weekday, weekday: Monday` (single long-form string) — and assert each of the 7 long forms `Monday`..`Sunday` is accepted as `weekday`.
- `recurrence: Weekday, weekdays: [Mon, Wed, Fri]` (list).
- `recurrence: Weekday, weekdays: [Monday]` (single-element list).
- `recurrence: Weekly` with neither field set.
- `recurrence: Daily` with neither field set.

Reject:
- `recurrence: Weekday` with neither `weekday` nor `weekdays` set (XOR — error message contains "exactly one").
- `recurrence: Weekday, weekday: Monday, weekdays: [Mon]` (both set — error contains "exactly one").
- `recurrence: Daily, weekday: Monday` (field on non-Weekday).
- `recurrence: Daily, weekdays: [Mon]` (field on non-Weekday).
- `recurrence: Weekly, weekdays: [Mon, Wed]` (field on non-Weekday).
- `weekday: Mon` (short form on the single field — NOT in `weekdayLongEnum`; rejected by the enum check). Assert `Mon` is NOT in `WeekdayLongEnumForTest()`.
- `weekday: Satuday` (typo — not in `weekdayLongEnum`).
- `weekdays: [Mon, Monday]` (cross-form duplicate — via `weekdayNoDuplicateRule`).
- `weekdays: [Monday, Mon]` (cross-form duplicate reversed).
- `weekdays: [Tue, Tue]` (same-form duplicate).
- `weekdays: [Wednesday, Wednesday]` (same-form long duplicate).
- `weekdays: [Mon, FunDay]` (unknown day — assert `FunDay` is not in `WeekdayAllEnumForTest()`; the item enum rejects it).

Also keep:
- The vault-regex rejection test (`vault: "Bad Vault"`).
- The unknown-recurrence rejection test (`recurrence: weekly` lowercase).
- The `periodOffset` CEL `DescribeTable` (it references `PeriodOffsetOnlyForPeriodKinds*` which is unchanged — leave it as-is).
- The enum-length assertions: `WeekdayLongEnumForTest()` has 7 elements; `WeekdayAllEnumForTest()` has 14 elements.

You may delete or rewrite the now-obsolete `validateSpec`/`buildSelfForCEL`/`evalRule` helpers as needed to fit the two-field shape — there is no requirement to preserve their exact signatures, only the behaviors in the table above. Keep the no-duplicate `DescribeTable` but drive it through `WeekdayNoDuplicateRuleForTest()` with `self["weekdays"]` (not `weekday`) as the `[]interface{}` list.

### 9. The structural-schema round-trip test already exists — confirm it now passes

The `Describe("Schedule CRD structural schema round-trip", ...)` block (lines ~479-496) is already present and uses `pkg.ScheduleCRSchemaPtrForTest()`, `apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps`, and `structuralschema.NewStructural`. Do NOT delete it. It is the regression lock required by the spec.

**Pre-edit FAIL anchor (capture before changing the schema):** Before any edit in this prompt, run `go test -v -run 'Structural' ./pkg/...` once against the current HEAD. The test MUST FAIL with messages containing `"must be empty to be structural"` and `"must not be empty for specified object fields"` (the spec's Reproduction quotes). Capture the failing output in your completion report's `summary`. This anchors the FAIL-before / PASS-after invariant from the spec. The current branch HEAD (`git rev-parse HEAD`) should be `20ad08f` or a descendant; if `git log --oneline -5 -- pkg/k8s_connector_schema.go` shows no commit touching that file since `1246391`, the FAIL still reproduces. If you cannot reproduce the FAIL pre-edit, STOP and report `status: failed` — the spec's regression-lock invariant is broken and the bug may already be partially patched.

After your schema reshape the test must PASS. If it still fails after your changes, your schema is still non-structural — fix the schema, not the test.

### 9a. Sweep removed test-helper call sites

After Step 7 deletes `WeekdayEnumForTest()`, `WeekdayRequiredIfWeekdayRuleForTest()`, `WeekdayRequiredIfWeekdayMessageForTest()`, `WeekdayListNonEmptyRuleForTest()`, and `WeekdayListNonEmptyMessageForTest()`, run:

```bash
cd /workspace && grep -rn 'WeekdayEnumForTest\|WeekdayRequiredIfWeekday[A-Z]\|WeekdayListNonEmpty[A-Z]' pkg/ k8s/
```

The output MUST be empty (zero matches) before you exit. Every call site must be either replaced with `WeekdayLongEnumForTest()` (for `weekday` field tests), `WeekdayAllEnumForTest()` (for `weekdays` list-item tests), or deleted along with its enclosing test case if obsolete. Do not leave a single dangling reference — the build will fail and the verifier will reject the prompt as `status: failed`.

### 10. Do NOT touch Go-side adapter, struct, or runtime code in this prompt

Do NOT modify any of:
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` (the `WeekdayList` type and `ScheduleTrigger` struct stay as-is — Prompt 2 changes them).
- `/workspace/pkg/store/adapter.go`.
- `/workspace/k8s/client/applyconfiguration/...`.
- `/workspace/k8s/apis/.../zz_generated.deepcopy.go`.
- `/workspace/pkg/publisher/`, `/workspace/pkg/schedule/`, `/workspace/pkg/tick/`, handler/trigger code.

The controller does not run admission validation — it decodes CRs with the existing Go struct regardless of the wire schema. This prompt's schema reshape is what the API server accepts; the Go struct gains the new `weekdays` field in Prompt 2. The `adaptSchedule` loop still reads `cr.Spec.Schedule.Weekday` (a `WeekdayList`), which still compiles and still passes its tests because the `weekday` field still exists.

### 11. Error paths

This prompt adds no new Go error-returning code paths (the schema is declarative data). CEL evaluation failures surface as API-server admission rejections — covered by the tests, not by Go error wrapping. No `bborbe/errors` calls are introduced here.
</requirements>

<constraints>
- CRD group, version, kind, plural, singular, short name are frozen. Adding the `weekdays` field is permitted; the `weekday` field name and the `recurrence`/`periodOffset` fields are unchanged.
- The `weekday` field returns to a single `string` with the 7-element long-form enum only. Short forms (`Mon`..`Sun`) on the `weekday` field are intentionally rejected; short-form usage moves to `weekdays`.
- The `RecurrenceKind` wire enum (`Daily`, `Weekly`, `Weekday`, `Monthly`, `Quarterly`, `Yearly`) is frozen — do not touch `recurrenceEnum`.
- `periodOffsetOnlyForPeriodKindsRule` and its message are unchanged byte-for-byte.
- Do NOT add any config knob to disable the `weekdays` field or to switch shapes. The two-field shape is unconditional.
- Do NOT add a CRD `Status` subresource or status writeback.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega for tests; `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` on any error path (none new here); no `time.Now()` / `context.Background()` in business logic.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Then targeted — the structural round-trip and the new two-field tables:

```bash
cd /workspace && go test -v -run 'Structural|XOR|Weekday|weekday|periodOffset' ./pkg/...
```

Confirm:
- The structural-schema round-trip `It` PASSES (it FAILS on the current broken code).
- Both-fields-set and neither-field-set on `Weekday` recurrence are rejected with a message containing "exactly one".
- `weekday: Mon` (short form) is rejected; all 7 long forms are accepted on `weekday`.
- `weekdays: [Mon, Monday]`, `[Tue, Tue]`, `[]` (via MinItems), and `[Mon, FunDay]` are all rejected.

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
