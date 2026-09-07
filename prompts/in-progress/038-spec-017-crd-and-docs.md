---
status: approved
spec: [017-hourly-recurrence-kind]
created: "2026-09-07T21:08:00Z"
queued: "2026-09-07T21:31:23Z"
branch: dark-factory/hourly-recurrence-kind
---

<summary>
- The Go-built Schedule CRD schema's recurrence enum now accepts `"Hourly"`, and the field description's kind list names it.
- The `periodOffset` CEL rule is unchanged — hourly is not in its allowed list, so a non-zero `periodOffset` on an Hourly CR is still rejected at admission.
- The k8s Go type's GoDoc enumerates the new kind (and fills in the previously missing `OnDate`).
- Ginkgo validation specs prove `recurrence: "Hourly"` is accepted and `recurrence: "Hourly"` with `periodOffset: 1` is rejected.
- The architecture doc's recurrence-kind and period-token documentation is updated for the new kind and token.
- No new CRD fields and no schema structural changes — purely additive enum entry plus comments and docs.
</summary>

<objective>
Teach the Go-built CRD schema to admit `recurrence: "Hourly"` while keeping `periodOffset` forbidden for it (the existing CEL rule already does this — only comments change), and keep the docs that enumerate recurrence kinds / token formats accurate for the new kind and token.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 4 of 4 for spec 017. It DEPENDS ON prompt 1 having landed: `schedule.RecurrenceHourly` must exist. Guard: if `grep -qn 'RecurrenceHourly' /workspace/pkg/schedule/recurrence.go` returns no match (exit non-zero), prompt 1 has NOT landed — STOP and report `status: failed` with summary "schedule core (prompt 1) not yet deployed — RecurrenceHourly undefined". This prompt is INDEPENDENT of prompts 2 and 3.

Read these files fully before changing anything:
- `/workspace/pkg/k8s_connector_schema.go` — the hand-built Go `JSONSchemaProps` (single source of truth for the CRD; no YAML manifest). Note `recurrenceEnum` (the `[]string{"Daily","Weekly","Weekday","Monthly","Quarterly","Yearly","OnDate"}` slice), the `recurrence` property `Description` in `scheduleTriggerSchema()` ("One of: Daily, Weekly, Weekday, Monthly, Quarterly, Yearly, OnDate."), and the `periodOffsetOnlyForPeriodKindsRule` const + its comment. The rule string `"!has(self.periodOffset) || self.periodOffset == 0 || self.recurrence in ['Monthly', 'Quarterly', 'Yearly']"` already rejects non-zero offset on Hourly (Hourly is not in the allowed list) — do NOT change the rule or its message.
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` — the `ScheduleTrigger.Recurrence` field's GoDoc (lines ~53-59). Comment-only change — NO new fields, so NO `make generatek8s` regeneration is needed.
- `/workspace/pkg/k8s_connector_validation_test.go` — `package pkg_test`. Note the `validateSpec(spec)` helper (checks `recurrence` against `pkg.RecurrenceEnumForTest()` and evaluates the weekday XOR rule) and the `periodOffset CEL validation` DescribeTable using `pkg.PeriodOffsetOnlyForPeriodKindsRuleForTest()` (mirror the `Entry("Daily + offset=-1 → reject", ...)` rows). `RecurrenceEnumForTest()` derives from `recurrenceEnum` in the same package, so adding `"Hourly"` to the enum makes `validateSpec` accept it with no further test-helper change.
- `/workspace/docs/architecture.md` — the closed recurrence-kind enumeration (line ~77) and the Idempotency Contract section (lines ~45-57) that documents the period-token input.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` — CRD schema + CEL rules.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc enumerating API values.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format.
</context>

<requirements>

### 1. Add `"Hourly"` to the recurrence enum and its description

In `/workspace/pkg/k8s_connector_schema.go`:

a. Append `"Hourly"` to `recurrenceEnum` (matching the Go kind order, where `RecurrenceHourly` is the last entry appended by prompt 1):

```go
var recurrenceEnum = []string{
	"Daily",
	"Weekly",
	"Weekday",
	"Monthly",
	"Quarterly",
	"Yearly",
	"OnDate",
	"Hourly",
}
```

b. Update the `recurrence` property `Description` in `scheduleTriggerSchema()` to include Hourly in the "One of: ..." list:

```go
			"recurrence": {
				Type:        "string",
				Description: "One of: Daily, Weekly, Weekday, Monthly, Quarterly, Yearly, OnDate, Hourly.",
				Enum:        jsonEnumValues(recurrenceEnum),
			},
```

c. Update the `recurrenceEnum` var GoDoc so its enumeration of the set includes `Hourly` alongside the existing kinds (the current comment's "post-Spec-9 6-kind" phrasing is stale — fix any count/list wording that would mislead about the current 8-value set).

### 2. Clarify the `periodOffset` CEL rule comment — rule string unchanged

In `/workspace/pkg/k8s_connector_schema.go`, the `periodOffsetOnlyForPeriodKindsRule` STRING and its `periodOffsetOnlyForPeriodKindsMessage` MUST stay byte-identical — Hourly is not in the `['Monthly', 'Quarterly', 'Yearly']` allowed list, so a non-zero `periodOffset` on an Hourly CR is already rejected (spec AC 7(b)). Only update the comment ABOVE the rule const so it names Hourly as periodOffset-forbidden alongside Daily/Weekly/Weekday:

```go
// periodOffsetOnlyForPeriodKindsRule rejects non-zero periodOffset on
// date-anchored recurrence kinds (Daily/Weekly/Weekday/Hourly). Those kinds
// don't carry a period concept distinct from the fire date; a date
// shift is the user-visible knob there, not an offset. Only Monthly,
// Quarterly, Yearly accept a non-zero offset.
```

### 3. Mention Hourly in the k8s Go type GoDoc (comment-only, no codegen)

In `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go`, update the `ScheduleTrigger.Recurrence` GoDoc to include `"Hourly"` (and `"OnDate"`, which is currently missing from the list) and note that Hourly fires every civil hour:

```go
	// Recurrence is one of: "Daily", "Weekly", "Weekday", "Monthly", "Quarterly", "Yearly",
	// "OnDate", "Hourly" (capitalized, matching Go's time.Weekday.String() style and Spec
	// 6/9's period-token output). "Weekly" is always-fire (no weekday); "Weekday" fires
	// only on its target weekday; "OnDate" fires on a fixed month-and-day each year;
	// "Hourly" fires every civil hour in Europe/Berlin. Constrained by the OpenAPI enum
	// in scheduleSpecSchema.
	Recurrence string `json:"recurrence"`
```

This is a comment-only change — NO new struct fields, so do NOT run `make generatek8s` (the generated deepcopy/client files are unaffected; running it would produce no change but wastes time).

### 4. Add CRD validation specs

In `/workspace/pkg/k8s_connector_validation_test.go`:

a. Add a spec proving `recurrence: "Hourly"` is accepted through the full admission-mirroring `validateSpec` helper (mirror the existing "accepts Daily with neither weekday nor weekdays" spec — a plain Hourly CR has no weekday/weekdays/month/day fields, so the XOR and OnDate CEL rules pass):

```go
	It("accepts Hourly with neither weekday/weekdays nor month/day", func() {
		spec := map[string]interface{}{
			"vault": "personal",
			"title": "Hourly Build Check",
			"schedule": map[string]interface{}{
				"recurrence": "Hourly",
			},
			"template": map[string]interface{}{"body": "."},
		}
		Expect(validateSpec(spec)).To(Succeed())
	})
```

b. Add rows to the existing `periodOffset CEL validation` DescribeTable proving the unchanged rule still rejects non-zero offset on Hourly:

```go
		Entry("Hourly + offset=0 → accept", "Hourly", true, 0, true),
		Entry("Hourly + offset=-1 → reject", "Hourly", true, -1, false),
		Entry("Hourly + offset=1 → reject", "Hourly", true, 1, false),
```

The existing `Schedule CRD CEL cost-budget regression-lock` and structural-schema round-trip specs must still pass — no new CEL rule was added, so the cost budget is untouched.

### 5. Update `docs/architecture.md`

In `/workspace/docs/architecture.md`:

a. Update the closed recurrence-kind enumeration (line ~77, currently "Recurrence-kind enum is closed — `daily`, `weekly`, `monthly`, `quarterly`, `yearly`. New kinds = new spec.") to include `hourly` — and the already-landed `weekday`/`ondate`, which are currently missing from the list:

```
- **Recurrence-kind enum is closed** — `daily`, `hourly`, `weekly`, `weekday`, `monthly`, `quarterly`, `yearly`, `ondate`. New kinds = new spec.
```

b. Add the hour-granular token to the Idempotency Contract / period-token documentation (the section around line 47, currently documenting the day-granular `recurring-<slug>-<YYYY-MM-DD>` identifier). Add one sentence stating that an `hourly` entry's period token is the compact civil hour `YYYYMMDDHH` (e.g. `2026090713`), so each civil hour produces a distinct identifier and task file. Do NOT rewrite the rest of the section — keep the edit scoped to the new kind + token.

### 6. Changelog

In `/workspace/CHANGELOG.md`, append to the existing `## Unreleased` section (created by prompt 1 — if it is somehow absent, create it) one entry:

```
- feat: Schedule CRD accepts `recurrence: "Hourly"` — enum and field description extended; `periodOffset` stays rejected for Hourly (existing CEL rule unchanged, comment clarified); k8s Go type GoDoc and `docs/architecture.md` enumerate the new kind and its `YYYYMMDDHH` period token
```

### 7. Refresh README.md (docs drift)

In `/workspace/README.md`, the inline recurrence enum comment (~line 53: `recurrence: Weekday  # Daily | Weekly | Weekday | Monthly | Quarterly | Yearly`) gains `| OnDate | Hourly` so it matches the closed set. Also refresh the kind list (~line 8: `recurring tasks (daily / weekly / weekday / monthly / quarterly / yearly)`) with `hourly` (and `ondate`, already missing), and add `hourly` / `YYYYMMDDHH` to the period-token table (~line 37) if one exists. Keep the edits to the kind/token enumerations only.

</requirements>

<constraints>
- The CRD schema is the hand-built Go `JSONSchemaProps` in `pkg/k8s_connector_schema.go` — the single source of truth. Do NOT hand-edit any generated deepcopy/client file.
- The `periodOffsetOnlyForPeriodKindsRule` STRING and `periodOffsetOnlyForPeriodKindsMessage` MUST remain byte-identical — only the comment above the rule may change. A non-zero `periodOffset` on an Hourly CR must remain rejected (spec Non-goal).
- Do NOT add Hourly to the allowed-offset set of any rule, do NOT add a new CEL rule, and do NOT touch the weekday XOR, weekday no-duplicate, or OnDate month/day rules.
- Do NOT add any new CRD field (no hour-of-day selector, no timezone field) — Hourly fires every civil hour in Europe/Berlin (spec Non-goals).
- Do NOT change `pkg/schedule`, `pkg/tick`, `pkg/publisher`, or `pkg/handler` — this prompt is CRD-schema + validation-test + docs only. In particular, do NOT change the `/trigger?date=` handler.
- Do NOT add any config knob, env var, tunable threshold, or opt-out flag (spec Non-goals).
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc updates stay accurate and start with the field/value name.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (this prompt adds none); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg` package; the Hourly accept path and the Hourly+offset reject path must be exercised.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the enum entry is present:

```bash
cd /workspace && grep -nE '"Hourly"' pkg/k8s_connector_schema.go
cd /workspace && grep -n 'Recurrence is one of' k8s/apis/task.benjamin-borbe.de/v1/types.go
```

Each must return ≥1 line.

Run the validation specs verbosely and confirm the new Hourly-accept and Hourly+periodOffset-reject specs pass alongside all pre-existing specs:

```bash
cd /workspace && go test -v ./pkg/
```

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0 (this includes the CEL cost-budget regression-lock and structural-schema round-trip specs — they must still pass since no CEL rule changed). If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.
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
