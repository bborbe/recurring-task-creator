---
status: completed
spec: [016-ondate-recurrence-kind]
summary: Extended Schedule CRD with OnDate recurrence, month/day fields, CEL rule, store adapter mapping, and tests
execution_id: recurring-task-creator-ondate-recurrence-exec-032-spec-016-crd-and-adapter
dark-factory-version: dev
created: "2026-07-14T12:00:00Z"
queued: "2026-07-14T12:27:27Z"
started: "2026-07-14T12:56:01Z"
completed: "2026-07-14T13:04:10Z"
branch: dark-factory/ondate-recurrence-kind
---

<summary>
- Extends the Schedule custom resource so operators can write `recurrence: OnDate` with an integer `month` (1-12) and `day` (1-31).
- Adds a validation rule that requires both `month` and `day` when the recurrence is `OnDate`, and forbids both on every other recurrence kind — enforced at `kubectl apply` time so a malformed CR never reaches the service.
- Keeps the existing rule that `periodOffset` is only valid for monthly/quarterly/yearly — `OnDate` rejects a non-zero offset.
- Wires the store adapter — the single seam that turns a Schedule CR into the internal record — to copy the CR's `month`/`day` onto the internal task record for `OnDate` entries.
- Regenerates Kubernetes codegen so the tree stays clean, and adds Ginkgo specs for the new validation rule and the adapter mapping.
- No existing kind, field, or CEL rule changes behavior — this is purely additive on the CRD + adapter side.
</summary>

<objective>
Extend the Schedule CRD to accept `recurrence: OnDate` with integer `month` (1-12) and `day` (1-31), guarded by a CEL rule that requires both iff `OnDate` and forbids both otherwise; and map the CR's `month`/`day` onto `TaskDefinition.Month`/`.Day` in the store adapter. Regenerate k8s codegen and add validation + adapter specs.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 3 of 3 for spec 016. It DEPENDS ON prompt 1 having landed: `schedule.RecurrenceOnDate` and `TaskDefinition.Month`/`.Day` must exist. Guard: if `grep -qn 'Month time.Month' /workspace/pkg/schedule/task_definition.go` returns no match (exit non-zero), prompt 1 has NOT landed — STOP and report `status: failed` with summary "schedule core (prompt 1) not yet deployed — TaskDefinition.Month/.Day undefined". This prompt is INDEPENDENT of prompt 2 (publisher).

Read these files fully before changing anything:
- `/workspace/pkg/k8s_connector_schema.go` — the hand-built Go `JSONSchemaProps` (single source of truth for the CRD; no YAML manifest). Note: `recurrenceEnum` (the `[]string{"Daily",...,"Yearly"}` slice you extend), `scheduleTriggerSchema()` (adds the `month`/`day` properties and the new CEL rule to `XValidations`), the existing CEL rule consts (`periodOffsetOnlyForPeriodKindsRule`, `weekdayXorRule`, `weekdayNoDuplicateRule`) for the style of a rule const + message const, and the `ptrInt64` / `jsonEnumValues` helpers. `periodOffsetOnlyForPeriodKindsRule` already keeps `periodOffset` valid only for Monthly/Quarterly/Yearly — do NOT add `OnDate` to it (OnDate must reject non-zero offset, which this rule already does since OnDate is not in the allowed list).
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` — the `ScheduleTrigger` struct. Note the existing `Recurrence string`, `Weekday string`, `Weekdays []string`, `PeriodOffset int` fields and their `json:"...,omitempty"` tags + GoDoc style. You add `Month int` and `Day int` here.
- `/workspace/pkg/store/adapter.go` — `adaptSchedule(ctx, cr)`. Note the final `return schedule.TaskDefinition{...}` literal that already maps `Recurrence`, `Weekdays`, `PeriodOffset`, `AutoAbortPrior`. You add `Month`/`Day` mapping. `time` is already imported.
- `/workspace/pkg/store/adapter_test.go` — the adapter tests (`package store_test`). Note `store.AdaptScheduleForTest(ctx, cr)`, the `v1.Schedule{ObjectMeta:..., Spec: v1.ScheduleSpec{...}}` fixture shape, and the existing recurrence-mapping DescribeTable. Add an OnDate mapping spec here. Test code may use `context.Background()`.
- `/workspace/pkg/k8s_connector_validation_test.go` — the CEL validation suite (`package pkg_test`). Note `evalXorRule` / `buildSelfForXor` pattern, the `periodOffset CEL validation` DescribeTable using `pkg.PeriodOffsetOnlyForPeriodKindsRuleForTest()`, and the `Schedule CRD CEL cost-budget regression-lock` `Describe` at the bottom (runs the assembled CRD through `apiextensionsvalidation.ValidateCustomResourceDefinition` — your new CEL rule MUST NOT exceed the per-rule cost budget). Add month/day validation specs mirroring the existing rule-eval helper style.
- `/workspace/pkg/k8s_connector_export_test.go` — the `*ForTest` accessors. You add a `OnDateMonthDayRuleForTest()` (and message) accessor so the validation test can reach the new rule const.
- `/workspace/Makefile.precommit` — note `generatek8s: bash hack/update-codegen.sh`. `make precommit` runs `generate` (mocks) but NOT `generatek8s`; you run `make generatek8s` explicitly and commit its output so the tree is clean.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` — CRD schema + CEL rules, codegen.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` (if you touch an adapter error path).
</context>

<requirements>

### 1. Add `"OnDate"` to the recurrence enum

In `/workspace/pkg/k8s_connector_schema.go`, append `"OnDate"` to `recurrenceEnum`:

```go
var recurrenceEnum = []string{"Daily", "Weekly", "Weekday", "Monthly", "Quarterly", "Yearly", "OnDate"}
```

Update the `recurrence` property `Description` in `scheduleTriggerSchema()` to include `OnDate` in the "One of: ..." list.

### 2. Add integer `month` (1-12) and `day` (1-31) properties

In `scheduleTriggerSchema()`, add two properties to the `Properties` map (alongside `weekday`/`weekdays`/`periodOffset`). Use `ptrInt64` for the numeric bounds (mirror how MinItems/MaxItems use it):

```go
	"month": {
		Type:        "integer",
		Description: "Calendar month (1-12) an OnDate schedule fires in. Required when recurrence is 'OnDate'; forbidden otherwise (CEL rule below).",
		Minimum:     ptrFloat64(1),
		Maximum:     ptrFloat64(12),
	},
	"day": {
		Type:        "integer",
		Description: "Day-of-month (1-31) an OnDate schedule fires on. Required when recurrence is 'OnDate'; forbidden otherwise (CEL rule below). A static 1-31 range only — e.g. 02-30 is not rejected here; such a date simply never occurs, so the entry never fires.",
		Minimum:     ptrFloat64(1),
		Maximum:     ptrFloat64(31),
	},
```

Note: `JSONSchemaProps.Minimum` and `.Maximum` are `*float64`, NOT `*int64`. Add a `ptrFloat64(n float64) *float64` helper next to the existing `ptrInt64` helper in this file:

```go
// ptrFloat64 returns a pointer to the given float64; the k8s OpenAPI schema
// represents Minimum/Maximum numeric bounds as *float64.
func ptrFloat64(n float64) *float64 {
	return &n
}
```

VERIFY the field type before writing: grep the struct — `grep -n 'Minimum\|Maximum' $(go env GOPATH)/pkg/mod/k8s.io/apiextensions-apiserver@*/pkg/apis/apiextensions/v1/types_jsonschema.go` — and confirm both are `*float64`. If they are a different type, use the matching pointer helper instead. Do NOT guess.

### 3. Add the OnDate month/day CEL rule

In `/workspace/pkg/k8s_connector_schema.go`, add a rule const + message const (mirror the `periodOffsetOnlyForPeriodKindsRule` / `...Message` pair):

```go
// onDateMonthDayRule requires both month and day when recurrence is
// 'OnDate', and forbids both on every other recurrence kind. Mirrors the
// weekday XOR intent for the date-anchored OnDate kind. Kept as a flat
// has()-only expression (no list traversal) so its estimated CEL cost stays
// well under the API server's per-rule budget (the cost-budget regression
// lock test asserts this).
const onDateMonthDayRule = "self.recurrence == 'OnDate' ? " +
	"(has(self.month) && has(self.day)) : " +
	"(!has(self.month) && !has(self.day))"

// onDateMonthDayMessage is the operator-facing error the API server emits
// when the rule fails.
const onDateMonthDayMessage = "month and day are both required when recurrence is 'OnDate', and both are forbidden otherwise"
```

Add the rule to the `XValidations` slice in `scheduleTriggerSchema()` (after the existing three rules):

```go
		XValidations: apiextensionsv1.ValidationRules{
			{Rule: weekdayXorRule, Message: weekdayXorMessage},
			{
				Rule:    periodOffsetOnlyForPeriodKindsRule,
				Message: periodOffsetOnlyForPeriodKindsMessage,
			},
			{Rule: weekdayNoDuplicateRule, Message: weekdayNoDuplicateMessage},
			{Rule: onDateMonthDayRule, Message: onDateMonthDayMessage},
		},
```

Do NOT modify `periodOffsetOnlyForPeriodKindsRule` — it already excludes `OnDate` from the allowed-offset set, so a non-zero `periodOffset` on an `OnDate` CR is already rejected. That is the desired behavior (Spec Non-goal: no PeriodOffset for OnDate).

### 4. Expose the new rule/message via `*ForTest` accessors

In `/workspace/pkg/k8s_connector_export_test.go`, add:

```go
// OnDateMonthDayRuleForTest returns the CEL rule requiring month+day iff OnDate.
func OnDateMonthDayRuleForTest() string { return onDateMonthDayRule }

// OnDateMonthDayMessageForTest returns the operator-facing OnDate month/day error message.
func OnDateMonthDayMessageForTest() string { return onDateMonthDayMessage }
```

### 5. Add `Month int` and `Day int` to the CR type

In `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go`, add two fields to `ScheduleTrigger` (place after `PeriodOffset`, before `AutoAbortPrior`), with `json:"...,omitempty"` tags and GoDoc:

```go
	// Month is the calendar month (1-12) an OnDate schedule fires in. Set
	// when Recurrence == "OnDate"; forbidden on every other kind (the CEL
	// rule in scheduleTriggerSchema enforces month+day required-iff-OnDate).
	// Mapped to schedule.TaskDefinition.Month (as time.Month) by the store
	// adapter.
	Month int `json:"month,omitempty"`

	// Day is the day-of-month (1-31) an OnDate schedule fires on. Set when
	// Recurrence == "OnDate"; forbidden on every other kind. A static 1-31
	// range only — 02-30 is admitted but never fires. Mapped to
	// schedule.TaskDefinition.Day by the store adapter.
	Day int `json:"day,omitempty"`
```

### 6. Map `month`/`day` in the store adapter

In `/workspace/pkg/store/adapter.go`, in `adaptSchedule`, add `Month` and `Day` to the returned `schedule.TaskDefinition{...}` literal, converting the CR's `int` month to `time.Month`:

```go
	return schedule.TaskDefinition{
		Slug:           cr.Name,
		TitleTemplate:  cr.Spec.Title,
		BodyTemplate:   cr.Spec.Template.Body,
		Recurrence:     kind,
		Weekdays:       weekdays,
		Month:          time.Month(cr.Spec.Schedule.Month),
		Day:            cr.Spec.Schedule.Day,
		Frontmatter:    cr.Spec.Template.Frontmatter,
		PeriodOffset:   cr.Spec.Schedule.PeriodOffset,
		AutoAbortPrior: autoAbortPrior,
	}, nil
```

`time` is already imported in this file. For a non-`OnDate` CR the CEL rule forbids `month`/`day`, so they arrive as the zero value (`0`) and map to `Month=time.Month(0)`, `Day=0` — the schedule layer ignores them for non-OnDate kinds. Do NOT branch on the kind here; the CEL rule already guarantees the fields are zero for non-OnDate. Do NOT change any other mapping.

### 7. Add adapter test — OnDate CR maps to Month/Day

In `/workspace/pkg/store/adapter_test.go`, add a spec asserting a round-tripped OnDate CR yields the expected `Month`/`Day`:

```go
It("maps month/day from an OnDate CR onto TaskDefinition.Month/.Day", func() {
	cr := &v1.Schedule{
		ObjectMeta: metav1.ObjectMeta{Name: "birthday-alice"},
		Spec: v1.ScheduleSpec{
			Title:    "Birthday",
			Schedule: v1.ScheduleTrigger{Recurrence: "OnDate", Month: 3, Day: 15},
			Template: v1.ScheduleTemplate{Body: "B"},
		},
	}
	def, err := store.AdaptScheduleForTest(ctx, cr)
	Expect(err).NotTo(HaveOccurred())
	Expect(def.Recurrence).To(Equal(schedule.RecurrenceOnDate))
	Expect(def.Month).To(Equal(time.March))
	Expect(def.Day).To(Equal(15))
})
```

Also add an `Entry("ondate", "OnDate", schedule.RecurrenceOnDate)` to the existing `DescribeTable("recurrence mapping", ...)` so the enum lowercasing (`"OnDate"` → `"ondate"`) is covered.

### 8. Add CRD validation tests for the month/day CEL rule

In `/workspace/pkg/k8s_connector_validation_test.go`, add a `Describe`/`DescribeTable` that evaluates `pkg.OnDateMonthDayRuleForTest()` (mirror the `periodOffset CEL validation` block's `evalRule` helper — build a `cel.NewEnv(cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)))`, compile, eval, return "" on pass else the message). Cover:
- `OnDate` with both `month` and `day` set → accept.
- `OnDate` with `month` only (no `day`) → reject.
- `OnDate` with `day` only (no `month`) → reject.
- `OnDate` with neither → reject.
- `Daily` with `month` set → reject (field on non-OnDate).
- `Daily` with `day` set → reject.
- `Yearly` with neither `month` nor `day` → accept.

Build the `self` map by including/omitting the `month`/`day` keys (an absent key models a field the operator did not set — `has(self.month)` is false). Example helper:

```go
Describe("OnDate month/day CEL validation", func() {
	evalRule := func(self map[string]interface{}) string {
		rule := pkg.OnDateMonthDayRuleForTest()
		env, err := cel.NewEnv(cel.Variable("self", cel.MapType(cel.StringType, cel.DynType)))
		Expect(err).NotTo(HaveOccurred())
		ast, issues := env.Compile(rule)
		Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
		program, err := env.Program(ast)
		Expect(err).NotTo(HaveOccurred())
		out, _, err := program.Eval(map[string]interface{}{"self": self})
		Expect(err).NotTo(HaveOccurred())
		if b, ok := out.(types.Bool); ok && bool(b) {
			return ""
		}
		return pkg.OnDateMonthDayMessageForTest()
	}
	DescribeTable("accepts/rejects (recurrence, month?, day?)",
		func(recurrence string, hasMonth, hasDay, expectOK bool) {
			self := map[string]interface{}{"recurrence": recurrence}
			if hasMonth {
				self["month"] = 3
			}
			if hasDay {
				self["day"] = 15
			}
			if expectOK {
				Expect(evalRule(self)).To(Equal(""))
			} else {
				Expect(evalRule(self)).NotTo(Equal(""))
			}
		},
		Entry("OnDate + month + day → accept", "OnDate", true, true, true),
		Entry("OnDate + month only → reject", "OnDate", true, false, false),
		Entry("OnDate + day only → reject", "OnDate", false, true, false),
		Entry("OnDate + neither → reject", "OnDate", false, false, false),
		Entry("Daily + month → reject", "Daily", true, false, false),
		Entry("Daily + day → reject", "Daily", false, true, false),
		Entry("Yearly + neither → accept", "Yearly", false, false, true),
	)
})
```

### 9. Regenerate k8s codegen and keep the tree clean

Run `make generatek8s` (it invokes `bash hack/update-codegen.sh`, which regenerates deepcopy + clients). Since you added two plain `int` fields to `ScheduleTrigger`, deepcopy for those fields is trivial (ints copied by value), but run codegen anyway so the generated files stay authoritative. Note: you are working in the container with uncommitted edits and MUST NOT commit (dark-factory handles git), so a whole-tree `git diff --exit-code` is NOT a valid drift check here — it would always be non-zero from your own source edits. Instead, prove codegen is **idempotent/up-to-date** with the git-free double-run check in `<verification>` below (run `generatek8s` twice; the second run must produce byte-identical generated files). The whole-tree `git diff --exit-code` drift check belongs to the spec's operator/CI ladder, which runs post-commit.

</requirements>

<constraints>
- The CRD schema is the hand-built Go `JSONSchemaProps` in `pkg/k8s_connector_schema.go` — the single source of truth. Do NOT hand-edit any generated deepcopy/client file; regenerate via `make generatek8s`.
- The new `onDateMonthDayRule` MUST be a flat `has()`-only CEL expression (no list traversal, no `map()/all()/filter()`) so its estimated cost stays under the API-server per-rule budget. The `Schedule CRD CEL cost-budget regression-lock` test MUST still pass.
- `month`/`day` are `integer` with a STATIC range only: month 1-12, day 1-31. Do NOT add per-month day validity (no rejecting 02-30) — that is a Spec Non-goal; 02-30 is admitted but simply never fires.
- Do NOT add `OnDate` to `periodOffsetOnlyForPeriodKindsRule` — a non-zero `periodOffset` on an `OnDate` CR must remain rejected (Spec Non-goal: no PeriodOffset for OnDate).
- Do NOT change any existing enum value, property, or CEL rule (weekday XOR, weekday no-duplicate, periodOffset) — this prompt is additive.
- The adapter maps `time.Month(cr.Spec.Schedule.Month)` and `cr.Spec.Schedule.Day` unconditionally; the CEL rule guarantees they are zero for non-OnDate kinds, and the schedule layer ignores them there. Do NOT branch on kind in the adapter.
- Do NOT add any config knob, env var, or tunable threshold. (Spec Non-goals.)
- Do NOT author any birthday/OnDate Schedule CRs — data authoring is a downstream concern, not this spec (Spec Non-goal).
- The UUID5 namespace and existing slugs are frozen — no existing entry changes kind or identifier.
- Before writing any k8s library field (`Minimum`/`Maximum`/etc.), grep the module source to confirm the field name and pointer type — do NOT guess. Module source: `$(go env GOPATH)/pkg/mod/k8s.io/apiextensions-apiserver@*/...`.
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc on the new exported CR fields and schema consts.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path; no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg` and `pkg/store` packages; the CEL rule accept/reject paths and the adapter month/day mapping must be exercised.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the enum value, properties, and CEL rule are present:

```bash
cd /workspace && grep -nE '"OnDate"|"month"|"day"' pkg/k8s_connector_schema.go
cd /workspace && grep -n 'onDateMonthDayRule' pkg/k8s_connector_schema.go
cd /workspace && grep -nE 'Month int|Day int' k8s/apis/task.benjamin-borbe.de/v1/types.go
cd /workspace && grep -n 'time.Month(cr.Spec.Schedule.Month)' pkg/store/adapter.go
```

Each grep must return matches.

Run the validation and adapter specs verbosely and confirm the new OnDate specs pass:

```bash
cd /workspace && go test -v ./pkg/ ./pkg/store/
```

Confirm codegen is idempotent/up-to-date — git-free double-run (the container tree is uncommitted, so `git diff` is not usable here). Run `make generatek8s` twice and confirm the generated files are byte-identical between the two runs:

```bash
cd /workspace && make generatek8s
find k8s -name '*.go' | sort | xargs sha1sum > /tmp/gen1.sha
cd /workspace && make generatek8s
find k8s -name '*.go' | sort | xargs sha1sum > /tmp/gen2.sha
diff /tmp/gen1.sha /tmp/gen2.sha && echo CODEGEN_STABLE
```

Must print `CODEGEN_STABLE` and `diff` must exit 0 (the second `generatek8s` run changed nothing — codegen is deterministic and matches your source edits). The whole-tree `git diff --exit-code` drift check is the operator/CI ladder's job post-commit, NOT this in-container step.

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0 (this includes the CEL cost-budget regression-lock test — confirm the new rule does not exceed the per-rule budget). If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.
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
