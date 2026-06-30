---
status: completed
spec: [015-auto-abort-prior-field]
summary: Added optional spec.schedule.autoAbortPrior *bool field to Schedule CRD (Go type, hand-rolled deepcopy, apply-configuration builder, boolean OpenAPI property) plus Ginkgo schema specs
execution_id: recurring-task-controller-auto-abort-exec-027-spec-015-crd-types-and-schema
dark-factory-version: dev
created: "2026-06-30T18:00:00Z"
queued: "2026-06-30T17:48:54Z"
started: "2026-06-30T19:24:11Z"
completed: "2026-06-30T19:29:20Z"
---

<summary>
- A `Schedule` CR can now carry an optional `spec.autoAbortPrior` boolean. When omitted, it behaves as `false`.
- The CRD's self-installing OpenAPI schema validates the field as a boolean, so the API server rejects a non-boolean value (e.g. the string `"yes"`) at `kubectl apply` time.
- The Go API type distinguishes "unset" from "explicit false" by carrying the field as a pointer-to-bool.
- Deep-copy and apply-configuration helpers carry the new field nil-safely, so existing client tooling keeps working.
- This is the API-contract layer: no behavior change to publishing yet — that arrives in later prompts.
- Existing Schedules with no `autoAbortPrior` are completely unaffected.
</summary>

<objective>
Add an opt-in `spec.autoAbortPrior` boolean to the `Schedule` CRD: a `*bool` field on the `ScheduleTrigger` Go struct (so unset is distinguishable from explicit `false`), the matching nil-safe deep-copy + apply-configuration plumbing, and a `boolean`-typed OpenAPI property on `scheduleTriggerSchema()` (no enum, not required) so the API server rejects non-boolean values at admission. This is the API contract every downstream layer reads from.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 1 of 4 for spec 015 (auto-abort-prior-field). It has no dependencies on the other prompts — it establishes the CRD/API contract the later prompts consume.

Read these files fully before changing anything:
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` — the `ScheduleTrigger` struct. Note the existing `PeriodOffset int` field with `json:"periodOffset,omitempty"` and its GoDoc style; `AutoAbortPrior` is added alongside it.
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go` — `ScheduleTrigger.DeepCopyInto` (around line 125). NOTE: per `hack/update-codegen.sh`, `ScheduleTrigger`'s deepcopy is HAND-ROLLED (deepcopy-gen cannot emit it), so you edit this file by hand; `make generatek8s` does NOT overwrite it.
- `/workspace/k8s/client/applyconfiguration/task.benjamin-borbe.de/v1/scheduletrigger.go` — the apply-config struct + `WithX` builders. Mirror the existing `Weekdays *[]string` / `PeriodOffset *int` pattern for the new field.
- `/workspace/pkg/k8s_connector_schema.go` — `scheduleTriggerSchema()` (around line 131). Note the existing `"periodOffset"` property block (`Type: "integer"`, no Enum, not in `Required`). The new property mirrors that shape with `Type: "boolean"`.
- `/workspace/pkg/k8s_connector_export_test.go` — test accessors (e.g. `ScheduleCRSchemaPtrForTest()` returns the full CR schema; `DesiredScheduleCRDForTest()` returns the assembled `*apiextensionsv1.CustomResourceDefinition`).
- `/workspace/pkg/k8s_connector_validation_test.go` — the package's CEL/structural-schema test patterns. Note the existing "Schedule CRD structural schema round-trip" `It` (around line 528) using `structuralschema.NewStructural` + `ValidateStructural`, and the imports `apiextensions`, `apiextensionsv1`, `structuralschema` already in scope (`package pkg_test`).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` — CRD schema + OpenAPI property conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc starts with the field name, full sentences.
</context>

<requirements>

### 1. Add `AutoAbortPrior *bool` to the `ScheduleTrigger` struct

In `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go`, add this field to `ScheduleTrigger` (place it after `PeriodOffset`), with GoDoc:

```go
	// AutoAbortPrior is an opt-in flag (default false when unset) marking
	// this Schedule as one whose prior-period instance MAY be auto-aborted
	// by the downstream task-controller when the next instance materializes.
	// A pointer so an unset field (nil → effective false) is distinguishable
	// from an explicit false. The publisher resolves the pointer to a plain
	// bool and stamps it as the `auto_abort_prior` frontmatter key on every
	// materialized task; the controller reads that key as its eligibility
	// gate (controller-side gate flip ships in a separate PR). Optional —
	// never required by the CRD schema.
	AutoAbortPrior *bool `json:"autoAbortPrior,omitempty"`
```

### 2. Update the hand-rolled deepcopy

In `/workspace/k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go`, extend `ScheduleTrigger.DeepCopyInto` to nil-safely copy the new pointer. The existing function copies the `Weekdays` slice; add a pointer copy for `AutoAbortPrior` after it:

```go
func (in *ScheduleTrigger) DeepCopyInto(out *ScheduleTrigger) {
	*out = *in
	if in.Weekdays != nil {
		in, out := &in.Weekdays, &out.Weekdays
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AutoAbortPrior != nil {
		in, out := &in.AutoAbortPrior, &out.AutoAbortPrior
		*out = new(bool)
		**out = **in
	}
}
```

NOTE: do NOT rely on `make generatek8s` to produce this — `hack/update-codegen.sh` documents that `ScheduleTrigger`'s deepcopy is hand-rolled. Edit it by hand. After your edits, `make generatek8s && git diff --exit-code` must still exit 0 (the regenerator does not touch the hand-rolled `ScheduleTrigger`/`ScheduleTemplate` deepcopy).

### 3. Update the apply-configuration

In `/workspace/k8s/client/applyconfiguration/task.benjamin-borbe.de/v1/scheduletrigger.go`:

- Add an `AutoAbortPrior *bool json:"autoAbortPrior,omitempty"` field to `ScheduleTriggerApplyConfiguration`, after `PeriodOffset`, mirroring the existing field GoDoc style (a one-line summary referencing the apis type doc).
- Add a `WithAutoAbortPrior(value bool)` builder mirroring the existing `WithPeriodOffset`/`WithWeekday` pattern:

```go
// WithAutoAbortPrior sets the AutoAbortPrior field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the AutoAbortPrior field is set to the value of the last call.
func (b *ScheduleTriggerApplyConfiguration) WithAutoAbortPrior(
	value bool,
) *ScheduleTriggerApplyConfiguration {
	b.AutoAbortPrior = &value
	return b
}
```

This file is marked `// Code generated ... DO NOT EDIT.` but is committed and hand-maintained in this repo (no applyconfiguration-gen step is wired into `make`); editing it by hand to mirror the type change is correct and expected.

### 4. Add the `autoAbortPrior` boolean property to the CRD schema

In `/workspace/pkg/k8s_connector_schema.go`, inside `scheduleTriggerSchema()`'s `Properties` map (after the `"periodOffset"` block), add:

```go
			"autoAbortPrior": {
				Type: "boolean",
				Description: "Opt-in flag (default false when omitted) marking this Schedule " +
					"as one whose prior-period instance may be auto-aborted by the task-controller " +
					"when the next instance materializes. Mirrored onto every materialized task's " +
					"frontmatter as auto_abort_prior. Optional; new Schedules are safe by default.",
			},
```

Do NOT add `"autoAbortPrior"` to the schema's `Required` list (it stays `[]string{"recurrence"}`). Do NOT add an `Enum`. Do NOT add any `x-kubernetes-validations` CEL rule for this field — a plain `Type: "boolean"` already makes the API server reject non-boolean values at admission.

### 5. Add a test accessor (only if needed by the test in step 6)

The test in step 6 reads the boolean property directly from `pkg.ScheduleCRSchemaPtrForTest()` (already exported in `pkg/k8s_connector_export_test.go`), so no new accessor is required. Do NOT add an unused accessor.

### 6. Add schema tests in `pkg/k8s_connector_validation_test.go`

Add a new `Describe("autoAbortPrior schema", ...)` block (`package pkg_test`) with these specs:

a. **Property shape** — the `autoAbortPrior` property exists under `spec.schedule`, is `Type: "boolean"`, has no `Enum`, and is absent from the schedule schema's `Required` list:

```go
var _ = Describe("autoAbortPrior schema", func() {
	It("declares autoAbortPrior as a boolean property, no enum, not required", func() {
		schema := pkg.ScheduleCRSchemaPtrForTest()
		trigger := schema.Properties["spec"].Properties["schedule"]
		prop, ok := trigger.Properties["autoAbortPrior"]
		Expect(ok).To(BeTrue(), "schedule schema must declare autoAbortPrior")
		Expect(prop.Type).To(Equal("boolean"))
		Expect(prop.Enum).To(BeEmpty(), "autoAbortPrior must not be an enum")
		Expect(trigger.Required).NotTo(ContainElement("autoAbortPrior"),
			"autoAbortPrior must remain optional")
	})
```

b. **Structural-schema round-trip still passes with the new property** — convert the full CR schema to a structural schema and assert `ValidateStructural` returns no errors (this is the same path the existing round-trip `It` uses; mirror its conversion code with `apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps`, `structuralschema.NewStructural`, `structuralschema.ValidateStructural`). This is the level-2 admission-path test for the new property — it exercises the same conversion the API server runs at CRD install:

```go
	It("the schema with autoAbortPrior round-trips through structural-schema validation", func() {
		v1Schema := pkg.ScheduleCRSchemaPtrForTest()
		var internalSchema apiextensions.JSONSchemaProps
		err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v1Schema, &internalSchema, nil,
		)
		Expect(err).NotTo(HaveOccurred())
		ss, err := structuralschema.NewStructural(&internalSchema)
		Expect(err).NotTo(HaveOccurred())
		Expect(structuralschema.ValidateStructural(nil, ss)).To(BeEmpty())
	})
})
```

The imports `apiextensions`, `apiextensionsv1`, and `structuralschema` are already present in `k8s_connector_validation_test.go` — reuse them; do not re-import.

NOTE for the reviewer: the spec's AC 10 ("schema rejects a non-boolean `autoAbortPrior` at apply time") is a property of the API server's value-validation against a `Type: "boolean"` schema, not a unit-testable seam in this repo (there is no in-repo CR-value-against-schema validator wired up — the existing value tests validate CEL rules via `cel-go`, not OpenAPI type validation). The structural-schema round-trip in (b) plus the property-shape assertion in (a) together lock that the schema declares the field as a boolean, which is what makes the API server reject `"yes"`/`1` at admission. If, while implementing, you find an existing in-repo helper that validates a CR value map against the OpenAPI schema (grep for `ValidateCustomResource` usage on a value, not a CRD), add a direct rejection test feeding `autoAbortPrior: "yes"`. Do NOT invent or wire up a new apiserver value-validator just for this test — that is out of scope and the structural round-trip is sufficient coverage for this prompt.

</requirements>

<constraints>
- `autoAbortPrior` is OPTIONAL on the CRD and defaults to `false` when unset — it must NEVER be added to any `Required` list. (Spec Non-goal: do not make it required.)
- Do NOT add an `Enum`, a CEL `x-kubernetes-validations` rule, an env var, or a tunable threshold for this field — `Type: "boolean"` is the entire validation. (Spec Non-goals.)
- Do NOT add a per-Schedule opt-out flag — the field IS the opt-in. (Spec Non-goal.)
- The CRD remains self-installing via `SetupCustomResourceDefinition` on every boot; no separate CRD YAML manifest is committed.
- The Go field MUST be `*bool` (pointer), not `bool`, so unset is distinguishable from explicit `false`.
- Do NOT change the `recurrence`/`weekday`/`weekdays`/`periodOffset` properties, the existing CEL rules, or the UUID5 contract.
- License headers (BSD-2-Clause) on every new or modified `.go` file. GoDoc on the new exported field.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the new field is present and codegen is a no-op:

```bash
cd /workspace && grep -n 'AutoAbortPrior \*bool' k8s/apis/task.benjamin-borbe.de/v1/types.go
cd /workspace && grep -n '"autoAbortPrior"' pkg/k8s_connector_schema.go
cd /workspace && grep -n 'AutoAbortPrior' k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go
cd /workspace && make generatek8s && git diff --exit-code
```

The last command must exit 0 (regenerated deepcopy matches committed).

Targeted schema tests:

```bash
cd /workspace && go test -v -run 'autoAbortPrior|structural' ./pkg/...
```

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
