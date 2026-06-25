---
status: completed
spec: [014-bug-weekday-cel-cost-budget]
container: recurring-task-creator-weekday-list-exec-025-spec-014-weekday-cel-cost-budget
dark-factory-version: v0.182.0
created: "2026-06-25T11:05:00Z"
queued: "2026-06-25T10:56:01Z"
started: "2026-06-25T10:56:02Z"
completed: "2026-06-25T11:08:41Z"
branch: dark-factory/bug-weekday-cel-cost-budget
---

<summary>
- Fixes the dev-pod CrashLoopBackOff: the Kubernetes API server stops rejecting the `Schedule` CRD with "estimated rule cost exceeds budget by factor of more than 100x".
- Caps the `weekdays` list at a maximum of 7 entries on the CRD schema (the true upper bound — there are only 7 distinct weekdays), keeping the existing minimum of 1.
- Rewrites the cross-form duplicate-day CEL rule from the expensive nested form to a bounded form whose estimated cost the API server accepts once the list is capped at 7 items.
- Operator-facing behavior is unchanged: `[Mon, Monday]`, `[Monday, Mon]`, `[Tue, Tue]`, `[Wednesday, Wednesday]` are still rejected at `kubectl apply` time; `[Mon, Wed, Fri]` and `[Mon, Tue, Wednesday, Thu, Fri]` are still accepted.
- Lists with more than 7 entries are now rejected by OpenAPI `maxItems` before any CEL rule runs.
- Adds a regression-lock unit test that runs the fully-assembled CRD through the API server's own admission validator and asserts no rule exceeds the cost budget — this test fails on the broken pre-fix schema and passes after the fix.
- Adds the matching accept/reject rows to the existing validation test tables, updates the architecture doc note, and adds a CHANGELOG entry.
- Touches only the Go-built CRD schema, its test-export accessors, and its validation tests — no adapter, struct, publisher, or runtime behavior changes.
</summary>

<objective>
Cap `spec.schedule.weekdays` at `MaxItems: 7` and rewrite the cross-form no-duplicates CEL rule to a bounded form so the Kubernetes API server's CEL cost estimator accepts the `Schedule` CRD at admission, ending the dev-pod CrashLoopBackOff. Add a regression-lock unit test that walks every CEL rule on the assembled CRD through the API server's admission validator and asserts each is under the per-rule cost budget.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:
- `/workspace/pkg/k8s_connector_schema.go` — the canonical Go-built CRD schema. Note the `weekdayNoDuplicateRule`/`weekdayNoDuplicateMessage` constants (the expensive nested `map().all().exists_one()` form — this is the rule the API server rejects on cost), the `weekdays` property in `scheduleTriggerSchema()` (currently has `MinItems: ptrInt64(1)` and no `MaxItems`), the `XValidations` list (the no-duplicate rule is index `[2]`, matching the production error), `jsonEnumValues()`, `ptrInt64()`, `scheduleCRSchemaPtr()`.
- `/workspace/pkg/k8s_connector_export_test.go` — the `*ForTest` accessors that expose unexported schema symbols to the external `pkg_test` package. Note `WeekdayNoDuplicateRuleForTest()`, `WeekdayNoDuplicateMessageForTest()`, `ScheduleCRSchemaPtrForTest()`.
- `/workspace/pkg/k8s_connector.go` — the connector. Note the private `desiredCRDSpec()` method that assembles the full `apiextensionsv1.CustomResourceDefinitionSpec` from the `v1` package constants and `scheduleCRSchemaPtr()`. The regression-lock test needs the assembled CRD; you will add a test-only export for it (Step 3).
- `/workspace/pkg/k8s_connector_validation_test.go` — the existing Ginkgo validation tests. Note the `evalListRule` helper (builds `cel.NewEnv(cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)))` — bare cel-go, NOT the full apiextensions CEL library) and the `DescribeTable("no-duplicate rule rejects same logical day in any form", ...)` block (the dup-rule table you will verify/extend), and the final `Describe("Schedule CRD structural schema round-trip", ...)` block (spec 013's structural lock — leave it untouched).
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/register.go` — `GroupName`, `Version`, `Kind`, `ListKind`, `Plural`, `Singular`, `ShortNames`. The CRD's `ObjectMeta.Name` must be `Plural + "." + GroupName` for the admission validator to accept the name.
- `/workspace/docs/architecture.md` — the "Schedule CR Weekday Field" section (~line 81) describing the `weekdays` field; you will add the 7-day max note there.
- `/workspace/CHANGELOG.md` — the `## Unreleased` section at the top; you will append a `fix:` bullet.

Coding guides (read the ones relevant to CRD schema + CEL + testing):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` — CRD schema + CEL `x-kubernetes-validations` patterns.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.

Why the current schema is rejected (from the spec): Kubernetes CRD admission runs every `x-kubernetes-validations` rule through a CEL cost estimator that computes worst-case cost using the array's `maxItems` bound. Because `weekdays` has no `maxItems`, the estimator assumes the array can hold up to `int32 max` elements, and the nested `map().all().exists_one()` traversal blows the per-rule budget — the API server reports "estimated rule cost exceeds budget by factor of more than 100x" on `x-kubernetes-validations[2].rule`. This is a DIFFERENT validator from spec 013's structural-schema validator, which is why spec 013's round-trip test passes but the API server still rejects the CRD. The fix is two combined changes: bound the array with `MaxItems: 7` AND keep the rule body within the cost budget at the new n=7 bound.

Verified facts (grepped from `k8s.io/apiextensions-apiserver@v0.36.2`, the version in `/workspace/go.mod`):
- The admission-time cost check lives in `apiextensionsvalidation.ValidateCustomResourceDefinition(ctx context.Context, obj *apiextensions.CustomResourceDefinition) field.ErrorList` at `k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation`. This is the exact public function the API server runs at CRD admission; it internally compiles every CEL rule, estimates cost, and appends a `field.Forbidden(...)` error whose detail contains `"exceeds budget"` when a rule's estimated cost exceeds `StaticEstimatedCostLimit` (10000000). Calling it on the assembled CRD gives a byte-equivalent verdict to what the API server rules.
- The internal (non-`v1`) CRD type is `apiextensions.CustomResourceDefinition` at `k8s.io/apiextensions-apiserver/pkg/apis/apiextensions`. Convert a `v1` CRD to it with `apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(in *v1.CustomResourceDefinition, out *apiextensions.CustomResourceDefinition, s conversion.Scope) error` (pass `nil` for the scope, matching how `Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps` is already called in the structural round-trip test).
- CEL form choice was probed in this repo's cel-go (`v0.28.1`, the bare env the test harness uses): the spec's "index pair" form using `0.until(size(...))` FAILS to compile — `undeclared reference to 'until'` — because the bare `cel.NewEnv` the tests build does not load the Kubernetes Lists CEL library. The set-size canonicalize-and-count form (Step 2) compiles and evaluates correctly (`[Mon,Monday]`→reject, `[Tue,Tue]`→reject, `[Mon,Wed,Fri]`→accept, mixed→accept). The set-size form is therefore the ONLY viable form — it works in both the bare test env and the apiextensions admission env, and is bounded by `MaxItems: 7`.
</context>

<requirements>

### 1. Cap `weekdays` with `MaxItems: 7` in `scheduleTriggerSchema()`

In `/workspace/pkg/k8s_connector_schema.go`, in `scheduleTriggerSchema()`, add `MaxItems: ptrInt64(7)` to the `"weekdays"` array property, alongside the existing `MinItems: ptrInt64(1)`. Update the property's `Description` to note the 7-entry cap. The resulting property:

```go
"weekdays": {
	Type:        "array",
	Description: "A non-empty list of weekdays, at most 7 (the number of distinct days in a week). Each entry is one of the 14 accepted day strings (long form Monday..Sunday or short form Mon..Sun); the two forms may be mixed in one list. Required-XOR with weekday when recurrence is 'Weekday'; both fields forbidden otherwise. Normalized to canonical time.Weekday values Go-side at parse time.",
	MinItems:    ptrInt64(1),
	MaxItems:    ptrInt64(7),
	Items: &apiextensionsv1.JSONSchemaPropsOrArray{
		Schema: &apiextensionsv1.JSONSchemaProps{
			Type: "string",
			Enum: jsonEnumValues(weekdayAllEnum),
		},
	},
},
```

Do not change `MinItems`, `Items`, `Type`, or any other property. `ptrInt64` already exists in this file — do not redefine it.

### 2. Rewrite the cross-form no-duplicates CEL rule to the bounded set-size form

In `/workspace/pkg/k8s_connector_schema.go`, replace the BODY of the `weekdayNoDuplicateRule` constant (the nested `self.weekdays.map(d, NORM).all(c, self.weekdays.map(d2, NORM).exists_one(c2, c2 == c))` form) with the set-size form below. This is the ONLY form to ship — do not introduce alternatives, do not leave the old body. Keep the leading `!has(self.weekdays) || type(self.weekdays) != list ||` short-circuit so the rule is a no-op when `weekdays` is absent. Keep `weekdayNoDuplicateMessage` byte-unchanged. The inline canonicalization map literal must be the byte-identical 14-entry map currently in the file (pasted in both `map` calls).

```go
// weekdayNoDuplicateRule rejects a weekdays list that names the same
// logical day twice, including cross-form duplicates ([Mon, Monday]).
// Each entry is canonicalized to its long form via a literal map, then
// the rule asserts every canonical value appears exactly once: for each
// canonical day c, the count of list entries that canonicalize to c must
// be 1. Bounded by the weekdays MaxItems:7 cap (Step 1), this form's
// estimated cost stays under the API server's per-rule CEL cost budget.
// (The prior map().all().exists_one() form blew the budget because, with
// no maxItems, the cost estimator assumed n up to int32 max.) The bare
// cel.NewEnv the tests build does not load the Kubernetes Lists library,
// so index-range forms like 0.until(size(...)) do NOT compile here; this
// map().all().filter().size() form does. Only applies when weekdays is
// present.
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
	"'Sunday':'Sunday'}[d2]).filter(x, x == c).size() == 1)"
```

This form was verified in this repo's cel-go to compile and produce the correct verdicts. If the Step 5 cost-budget test reports this form STILL over budget at `MaxItems: 7` (it should not — the bound is what brings it in budget), STOP and report `status: failed` with the estimated cost figure from the test output, rather than inventing a different rule form; the next iteration of the prompt will pick a cheaper form with the apiextensions Lists library loaded.

### 3. Add a test-only export for the assembled CRD object

The regression-lock test (Step 5) needs the FULL assembled CRD, not just the schema, because `ValidateCustomResourceDefinition` validates the whole object (names, versions, ObjectMeta). The connector's `desiredCRDSpec()` is unexported and is a method on `*k8sConnector`. Add a package-level test export.

In `/workspace/pkg/k8s_connector_export_test.go`, add:
```go
// DesiredScheduleCRDForTest returns the fully-assembled v1 CustomResourceDefinition
// the connector installs — ObjectMeta.Name, group, names, versions, and the
// OpenAPIV3Schema with all x-kubernetes-validations rules. Exposed so the CEL
// cost-budget regression-lock test can run the exact object through the API
// server's admission validator.
func DesiredScheduleCRDForTest() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
		Spec:       (&k8sConnector{}).desiredCRDSpec(),
	}
}
```
This requires adding imports to `k8s_connector_export_test.go`: `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` and `v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"`. The `apiextensionsv1` import is already present. A zero-value `&k8sConnector{}` is safe here: `desiredCRDSpec()` reads only the `v1` constants and `scheduleCRSchemaPtr()` — it does not touch `configBuilder`/`clientBuilder`. Confirm by reading `desiredCRDSpec()` in `k8s_connector.go` before relying on this.

### 4. Keep the XValidations order and the messages unchanged

In `scheduleTriggerSchema()`, the `XValidations` list stays exactly three entries in this order — the no-duplicate rule remains the third entry (index `[2]`, matching the production error path):
```go
XValidations: apiextensionsv1.ValidationRules{
	{Rule: weekdayXorRule, Message: weekdayXorMessage},
	{Rule: periodOffsetOnlyForPeriodKindsRule, Message: periodOffsetOnlyForPeriodKindsMessage},
	{Rule: weekdayNoDuplicateRule, Message: weekdayNoDuplicateMessage},
},
```
`weekdayXorRule`, `weekdayXorMessage`, `periodOffsetOnlyForPeriodKindsRule`, `periodOffsetOnlyForPeriodKindsMessage`, and `weekdayNoDuplicateMessage` are all byte-unchanged — do not touch them.

### 5. Add the CEL cost-budget regression-lock test

In `/workspace/pkg/k8s_connector_validation_test.go`, add a new `Describe` block that runs the assembled CRD through the API server's admission validator and asserts no rule exceeds the cost budget.

Add these imports to the test file's import block (the file is `package pkg_test`). The `apiextensions` and `apiextensionsv1` aliases are ALREADY imported by the structural round-trip test — do NOT add duplicates; reuse them. Add only what is missing (`context`, `strings`, and the validation package):
```go
"context"
"strings"

apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
```

Add the block:
```go
var _ = Describe("Schedule CRD CEL cost-budget regression-lock", func() {
	It("no x-kubernetes-validations rule on the assembled CRD exceeds the API-server per-rule cost budget", func() {
		// Convert the assembled v1 CRD to the internal apiextensions type, then
		// run it through the exact public function the API server uses at CRD
		// admission. ValidateCustomResourceDefinition compiles every CEL rule,
		// estimates worst-case cost using each array's maxItems, and emits a
		// field.Forbidden error whose detail contains "exceeds budget" when a
		// rule's estimated cost exceeds StaticEstimatedCostLimit. This is a
		// byte-equivalent verdict to production admission. Regression lock for
		// the spec-014 CrashLoopBackOff: this It FAILS on the pre-fix schema
		// (unbounded weekdays + nested map().all().exists_one()) and PASSES
		// after MaxItems:7 + the rewritten dup rule.
		v1CRD := pkg.DesiredScheduleCRDForTest()
		Expect(v1CRD).NotTo(BeNil())

		var internalCRD apiextensions.CustomResourceDefinition
		err := apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(
			v1CRD, &internalCRD, nil,
		)
		Expect(err).NotTo(HaveOccurred(), "v1->internal CRD conversion must succeed")

		errs := apiextensionsvalidation.ValidateCustomResourceDefinition(context.Background(), &internalCRD)

		var costErrs []string
		for _, e := range errs {
			if strings.Contains(e.Detail, "exceeds budget") {
				costErrs = append(costErrs, e.Error())
			}
		}
		Expect(costErrs).To(BeEmpty(), "no CEL rule may exceed the per-rule cost budget; offending rules: %v", costErrs)
	})
})
```
Notes:
- The filter matches only `"exceeds budget"` — the exact phrase the production error uses — so any unrelated admission nits (CABundle, conversion-review versions, name format) returned by `ValidateCustomResourceDefinition` do NOT make this test flake. The test asserts only on the cost-budget error class.
- `context.Background()` here is in TEST code (the validator's API requires a context); the no-`context.Background()`-in-business-logic DoD rule applies to `pkg/` production code, not test files. This matches how the apiextensions test suite itself calls the function.

### 5a. Anchor the FAIL-before / PASS-after invariant

The cost test you are adding lives in the test file and references `DesiredScheduleCRDForTest()` (Step 3) — both must exist for the test to compile. So run the anchor AFTER you have written Steps 3 and 5 (the test + export), but with the pre-fix SCHEMA swapped in:

**Atomic stash of all three modified files** so the FAIL-anchor restore is reversible and the `stash pop` cannot conflict:

```bash
cd /workspace && \
  git stash push -- pkg/k8s_connector_schema.go pkg/k8s_connector_validation_test.go pkg/k8s_connector_export_test.go && \
  git checkout ac982b8 -- pkg/k8s_connector_schema.go && \
  # restore JUST the test + export from the stash so the pre-fix schema compiles against the new test
  git checkout stash@{0} -- pkg/k8s_connector_validation_test.go pkg/k8s_connector_export_test.go && \
  go test -v -run 'cost-budget' ./pkg/... ; \
  rc=$? ; \
  git checkout HEAD -- pkg/k8s_connector_schema.go pkg/k8s_connector_validation_test.go pkg/k8s_connector_export_test.go && \
  git stash pop ; \
  echo "pre-fix-rc=$rc"
```
The pre-fix run MUST FAIL with a message containing `"exceeds budget"`. Capture that failure text in your completion report's `summary`. If `ac982b8` does not exist on this checkout (the branch may be a fresh worktree — verify with `git cat-file -e ac982b8^{commit}`), substitute: temporarily hand-edit `pkg/k8s_connector_schema.go` to remove `MaxItems: ptrInt64(7)` AND restore the old nested `map().all().exists_one()` dup rule body, run `go test -v -run 'cost-budget' ./pkg/...` (must FAIL with "exceeds budget"), then restore your fix. If you CANNOT reproduce the FAIL pre-fix, STOP and report `status: failed` — the regression-lock invariant is broken.

### 6. Verify and extend the dup-rule DescribeTable rows

In `/workspace/pkg/k8s_connector_validation_test.go`, in the existing `DescribeTable("no-duplicate rule rejects same logical day in any form", ...)` block, confirm these rejection rows are present and still PASS against the rewritten set-size rule (they already exist from spec 013 — do NOT delete them; the rewrite must keep their verdicts):
- `[Mon, Monday]` → reject
- `[Monday, Mon]` → reject
- `[Tue, Tue]` → reject
- `[Wednesday, Wednesday]` → reject

And these acceptance rows PASS:
- `[Mon, Wed, Fri]` → accept
- `[Mon, Tue, Wednesday, Thu, Fri]` → accept (mixed forms) — present already; keep it
- absent weekdays → accept

These rows drive the CEL rule directly through `WeekdayNoDuplicateRuleForTest()` via `evalListRule` (bare cel-go). The rewritten set-size form was verified to produce exactly these verdicts. Anti-laziness anchor: a rule replaced with the literal `true` would pass the acceptance rows but FAIL all four rejection rows — keeping them locks the dup-detection semantics to this spec.

### 7. Add a >7-items MaxItems bound assertion

The `MaxItems: 7` bound is an OpenAPI declarative constraint enforced by the API server BEFORE any CEL rule runs — it is not expressible through the `evalListRule` CEL-only helper. Add an assertion that proves the bound is engaged on the schema. In `/workspace/pkg/k8s_connector_validation_test.go`, add:

```go
var _ = Describe("weekdays MaxItems bound", func() {
	It("weekdays property carries MaxItems == 7 and keeps MinItems == 1", func() {
		schema := pkg.ScheduleCRSchemaPtrForTest()
		weekdays := schema.Properties["spec"].Properties["schedule"].Properties["weekdays"]
		Expect(weekdays.MaxItems).NotTo(BeNil(), "weekdays must declare MaxItems")
		Expect(*weekdays.MaxItems).To(Equal(int64(7)))
		Expect(weekdays.MinItems).NotTo(BeNil(), "weekdays must keep MinItems")
		Expect(*weekdays.MinItems).To(Equal(int64(1)))
	})
})
```
This proves the 8-item list `[Mon, Tue, Wed, Thu, Fri, Sat, Sun, Mon]` is rejected by `maxItems` at admission before the CEL rule fires (8 > 7). Verify the navigation path (`Properties["spec"].Properties["schedule"].Properties["weekdays"]`) against `scheduleCRSchemaPtr()` / `scheduleSpecSchema()` / `scheduleTriggerSchema()` in `k8s_connector_schema.go` before writing — the top-level schema nests `spec` then `schedule` then `weekdays`.

### 8. Update the architecture doc

In `/workspace/docs/architecture.md`, in the "Schedule CR Weekday Field" section (~line 81), amend the sentence describing `weekdays`: change the parenthetical "(non-empty list of long-or-short day names)" to "(non-empty list of at most 7 long-or-short day names)". One-line edit; do not restructure the section.

### 9. Add the CHANGELOG entry

In `/workspace/CHANGELOG.md`, under the existing `## Unreleased` header (append to it — do not create a new section), add one bullet:
```
- fix: cap `spec.schedule.weekdays` at `maxItems: 7` and rewrite the cross-form no-duplicates CEL rule to a bounded set-size form, so the Kubernetes API server's CEL cost estimator stops rejecting the `Schedule` CRD ("estimated rule cost exceeds budget"); resolves the dev-pod CrashLoopBackOff. Duplicate-day rejection (`[Mon, Monday]`, `[Tue, Tue]`) is unchanged.
```

### 10. Do NOT touch Go-side adapter, struct, or runtime code

Do NOT modify any of:
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` (the `WeekdayList` type and `ScheduleTrigger` struct — untouched per spec 012/013 internals constraint).
- `/workspace/pkg/store/adapter.go`.
- `/workspace/pkg/publisher/` (the 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` must pass byte-identically — do not touch it).
- `/workspace/pkg/schedule/`, `/workspace/pkg/tick/`, handler/trigger code.
- The `recurrenceEnum`, `weekdayLongEnum`, `weekdayAllEnum` vars, the `weekdayXorRule`/`Message`, and `periodOffsetOnlyForPeriodKindsRule`/`Message` constants — all byte-unchanged.
- The `Describe("Schedule CRD structural schema round-trip", ...)` block (spec 013's structural lock) — leave it exactly as-is; it must still PASS.

### 11. Error paths

This prompt adds no new Go error-returning production code path (the schema is declarative data). The only new error handling is in test code: the v1->internal conversion in Step 5 returns an `error` that the test asserts is nil via `Expect(err).NotTo(HaveOccurred())`. No `bborbe/errors` calls are introduced — CEL/admission failures surface as the validator's `field.ErrorList`, which the test inspects directly.
</requirements>

<constraints>
- CRD group, version, kind, plural, singular, short name are frozen — do not touch the `v1` package constants.
- `weekdays` enum is unchanged from spec 013 (14-element long+short set) — do not touch `weekdayAllEnum`.
- The cross-form duplicate-detection user-facing behavior is unchanged. Operators MUST still see `[Mon, Monday]` rejected at admission, not silently deduped. Only the rule's CEL implementation form changes.
- `weekdays` keeps `MinItems: 1`; the only schema addition is `MaxItems: 7`.
- Spec 012's internals (`WeekdayList`, normalization map, day-set matcher, period-token rendering, UUID5 derivation) untouched. Spec 013's two-field wire shape and structural-schema regression-lock test untouched.
- The 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` must continue to pass byte-identically — do not touch `pkg/publisher/`.
- Ship exactly ONE CEL form for the no-duplicate rule (the set-size form in Step 2) — never leave the old body or an alternative form in the source.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega for tests; `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` on any production error path (none new here); no `time.Now()` / `context.Background()` in business logic (the `context.Background()` in Step 5 is TEST code, which is permitted).
- Do NOT add any config knob to disable the bound or switch rule forms — the fix is unconditional.
- Do NOT add a CRD `Status` subresource or status writeback.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Then targeted — the new cost-budget lock, the spec-013 structural lock, the dup-rule table, and the MaxItems assertion:

```bash
cd /workspace && go test -v -run 'cost-budget|Structural|no-duplicate|MaxItems|Weekday|weekday' ./pkg/...
```

Confirm:
- The new `Schedule CRD CEL cost-budget regression-lock` `It` PASSES (it FAILS on the pre-fix schema — anchor captured per Step 5a).
- The `Schedule CRD structural schema round-trip` `It` still PASSES (spec 013's lock preserved).
- All four duplicate rejection rows (`[Mon, Monday]`, `[Monday, Mon]`, `[Tue, Tue]`, `[Wednesday, Wednesday]`) PASS, and `[Mon, Wed, Fri]` / `[Mon, Tue, Wednesday, Thu, Fri]` are accepted.
- `weekdays.MaxItems == 7` and `weekdays.MinItems == 1`.

Confirm the UUID5 stability block is untouched and passes:
```bash
cd /workspace && go test -v -run 'UUID5 stability' ./pkg/publisher/...
```

Confirm the doc and changelog edits:
```bash
cd /workspace && grep -nE 'weekdays.*(7|maxItems|at most 7)' docs/architecture.md
cd /workspace && awk '/^## Unreleased/{u=1;next} /^## /{u=0} u' CHANGELOG.md | grep -nE 'fix:.*(CEL|cost|weekdays|maxItems|MaxItems)'
```
Both must return ≥1 line.

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
  "summary": "<one line — include the pre-fix FAIL text containing 'exceeds budget' captured in Step 5a>",
  "verification": {"command": "make precommit", "exitCode": 0}
}
```

`"status":"success"` ONLY if `make precommit` exited 0.

## Improvements

- (fill in per the reflection rules; write `- None` if nothing)
</completion>
