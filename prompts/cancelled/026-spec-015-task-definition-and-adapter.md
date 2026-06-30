---
status: cancelled
spec: [015-auto-abort-prior-field]
execution_id: recurring-task-controller-auto-abort-exec-026-spec-015-task-definition-and-adapter
dark-factory-version: dev
created: "2026-06-30T18:00:00Z"
queued: "2026-06-30T17:47:55Z"
started: "2026-06-30T19:24:02Z"
completed: "2026-06-30T17:48:44Z"
branch: dark-factory/auto-abort-prior-field
lastFailReason: 'validate completion report: completion report status: failed'
cancelled: "2026-06-30T19:24:11Z"
---

<summary>
- The internal recurring-task inventory record now carries a plain `auto-abort-prior` boolean alongside the existing period-offset field.
- The store adapter — the single seam that turns a `Schedule` CR into the internal record — resolves the optional CRD pointer to a plain boolean: unset becomes `false`, explicit `false` stays `false`, explicit `true` becomes `true`.
- A nil pointer never panics; it cleanly resolves to the safe-by-default `false`.
- This bridges the API contract (prompt 1) to the domain layer the publisher consumes (prompt 3).
- The pure-data schedule layer stays free of Kafka/HTTP/agent imports — the new field is just a `bool`.
- Existing adapter behavior (recurrence, weekdays, frontmatter, period-offset mapping) is unchanged.
</summary>

<objective>
Carry the opt-in flag from the CRD into the domain layer: add a plain `AutoAbortPrior bool` field to `schedule.TaskDefinition`, and make the store adapter resolve the CR's `*bool` (`cr.Spec.Schedule.AutoAbortPrior`) to that plain bool — nil → `false`, explicit `false` → `false`, explicit `true` → `true`. The publisher (prompt 3) reads only from `TaskDefinition`.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 2 of 4 for spec 015. It DEPENDS ON prompt 1 having landed: the `ScheduleTrigger` struct must already carry `AutoAbortPrior *bool`. Guard: if `grep -qn 'AutoAbortPrior \*bool' /workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` returns no match (exit non-zero), prompt 1 has not landed — STOP and report `status: failed` with summary "CRD types (prompt 1) not yet deployed".

Read these files fully before changing anything:
- `/workspace/pkg/schedule/task_definition.go` — the `TaskDefinition` struct. Note the existing `PeriodOffset int` field and its GoDoc style; `AutoAbortPrior bool` is added alongside it. This package is PURE DATA — no Kafka/HTTP/agent imports.
- `/workspace/pkg/store/adapter.go` — `adaptSchedule(ctx, cr)`. Note the final `return schedule.TaskDefinition{...}` literal that already maps `PeriodOffset: cr.Spec.Schedule.PeriodOffset`. You add one field to that literal, resolving the `*bool`.
- `/workspace/pkg/store/adapter_test.go` — the adapter tests, `package store_test`. Note `store.AdaptScheduleForTest(ctx, cr)`, the existing `It("propagates PeriodOffset from CR to TaskDefinition", ...)` and `It("defaults PeriodOffset to 0 when omitted on the CR", ...)` (around lines 289–315) — mirror those for the new field. Tests may use `context.Background()` (test code is exempt).

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc starts with the field name.
</context>

<requirements>

### 1. Add `AutoAbortPrior bool` to `TaskDefinition`

In `/workspace/pkg/schedule/task_definition.go`, add this field to `TaskDefinition` (place it after `PeriodOffset`), with GoDoc:

```go
	// AutoAbortPrior is the opt-in flag resolved from the CR's
	// spec.autoAbortPrior pointer by the store adapter (nil → false). A plain
	// bool, consistent with PeriodOffset's plain-int style — the schedule
	// layer stays pure data. The publisher mirrors this value onto every
	// materialized task's frontmatter as the `auto_abort_prior` key; the
	// downstream task-controller reads that key as its auto-abort eligibility
	// gate. Default false means a Schedule never opts into auto-abort unless
	// the operator explicitly sets spec.autoAbortPrior: true.
	AutoAbortPrior bool
```

Do NOT import anything new into `pkg/schedule/` — `bool` is a builtin. The package must stay free of Kafka/HTTP/agent imports.

### 2. Resolve the pointer in the adapter

In `/workspace/pkg/store/adapter.go`, inside `adaptSchedule`, add the new field to the returned `schedule.TaskDefinition{...}` literal. Resolve the `*bool` to a plain `bool`, nil-safe:

```go
	autoAbortPrior := false
	if cr.Spec.Schedule.AutoAbortPrior != nil {
		autoAbortPrior = *cr.Spec.Schedule.AutoAbortPrior
	}

	return schedule.TaskDefinition{
		Slug:           cr.Name,
		TitleTemplate:  cr.Spec.Title,
		BodyTemplate:   cr.Spec.Template.Body,
		Recurrence:     kind,
		Weekdays:       weekdays,
		Frontmatter:    cr.Spec.Template.Frontmatter,
		PeriodOffset:   cr.Spec.Schedule.PeriodOffset,
		AutoAbortPrior: autoAbortPrior,
	}, nil
```

Place the `autoAbortPrior` resolution immediately before the `return`. Do NOT change any other mapping. The nil check must never dereference a nil pointer — a nil `*bool` resolves to `false` without panic.

### 3. Add adapter tests

In `/workspace/pkg/store/adapter_test.go`, add three specs (mirror the existing PeriodOffset tests' fixture shape). Use a small local helper to take the address of a bool literal, or declare the bools and take their address inline:

```go
	It("resolves a nil autoAbortPrior pointer to false", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "no-flag"},
			Spec: v1.ScheduleSpec{
				Title:    "No Flag",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.AutoAbortPrior).To(BeFalse())
	})

	It("resolves an explicit false autoAbortPrior to false", func() {
		f := false
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "explicit-false"},
			Spec: v1.ScheduleSpec{
				Title:    "Explicit False",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily", AutoAbortPrior: &f},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.AutoAbortPrior).To(BeFalse())
	})

	It("resolves an explicit true autoAbortPrior to true", func() {
		t := true
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "explicit-true"},
			Spec: v1.ScheduleSpec{
				Title:    "Explicit True",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily", AutoAbortPrior: &t},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.AutoAbortPrior).To(BeTrue())
	})
```

Place these inside the existing `Describe("adaptSchedule", ...)` block. Keep all other adapter tests unchanged.

</requirements>

<constraints>
- `pkg/schedule/` stays a pure-data layer — no Kafka / HTTP / agent imports. `AutoAbortPrior` is a plain `bool`, consistent with the existing `PeriodOffset int`.
- The adapter must tolerate a nil `*bool` (unset) and resolve to `false` — NEVER panic on nil.
- Do NOT change the recurrence mapping, weekday normalization, frontmatter copy, period-offset mapping, or the UUID5 contract.
- Do NOT add a config knob, env var, or tunable threshold. (Spec Non-goals.)
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc on the new exported field.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (no new error path is introduced here); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed adapter package; test the nil/false/true resolution paths.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the field is present:

```bash
cd /workspace && grep -n 'AutoAbortPrior bool' pkg/schedule/task_definition.go
cd /workspace && grep -n 'AutoAbortPrior' pkg/store/adapter.go
```

Targeted adapter tests:

```bash
cd /workspace && go test -v -run 'adaptSchedule' ./pkg/store/...
```

Confirm nil → false, explicit false → false, explicit true → true all pass.

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
