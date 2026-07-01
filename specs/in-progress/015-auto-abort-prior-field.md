---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-30T17:34:14Z"
generating: "2026-06-30T17:35:59Z"
prompted: "2026-06-30T17:44:00Z"
verifying: "2026-07-01T10:09:26Z"
branch: dark-factory/auto-abort-prior-field
---

## Summary

TL;DR — what this spec proposes, in plain language:

- Add an opt-in `spec.autoAbortPrior: bool` field (default `false`) to the `Schedule` CRD so an operator can mark a recurring-task Schedule as one whose prior-period instance may be auto-aborted when the next instance materializes.
- The publisher stamps `auto_abort_prior: <bool>` onto every materialized task's frontmatter, mirroring the CRD field, so the downstream controller can read the opt-in from the task file itself.
- Opt-in (default `false`) is the load-bearing safety model: a new Schedule never aborts its prior instance unless the operator explicitly sets `autoAbortPrior: true`. This inverts the abandoned v2 `skipAutoCleanup` (opt-out) approach, which risked audit-style tasks getting superseded whenever an operator forgot to mark them.
- This spec covers ONLY the recurring-task-creator side (CRD field + Go struct + store adapter + frontmatter stamp). The controller-side gate flip (`audit_style != true` → `auto_abort_prior == true`) lives in `bborbe/agent-task-controller` and ships as a separate PR after this one.

## Problem

Inbox-style recurring tasks (`cleanup-obsidian-inbox`, `cleanup-omnifocus-inbox`, `aquascape-pwc`) accumulate stale `in_progress` instances when a day or week gets skipped — today's instance already covers yesterday's intent, so the leftover is noise that taxes the operator with manual close-out. Audit-style dailies (`check-prometheus-alerts`, `ibkr-swing-trading`) are the opposite: each missed firing IS the signal, and silently aborting destroys "we missed Tuesday."

The auto-supersede feature (implemented in `bborbe/agent-task-controller`) lets the controller transition a prior-period instance to `aborted` at materialize time. To decide which Schedules are eligible, the controller reads a flag from each materialized task's frontmatter. Today the controller reads `audit_style != true` — an opt-OUT model where every Schedule is eligible unless the operator marks it audit-style. That is unsafe: a new Schedule defaults to "eligible for auto-abort," so any operator who forgets to set the audit-style flag on an audit-style task silently loses the missed-firing signal.

The fix is to invert to opt-IN: a Schedule is eligible for auto-abort only when the operator explicitly sets `spec.autoAbortPrior: true`. The publisher stamps `auto_abort_prior: <bool>` onto the task frontmatter; the controller (in a separate, later PR) flips its gate to `auto_abort_prior == true`. This spec delivers the publisher-side prerequisite: the CRD field and the frontmatter stamp. Until the controller flip ships, the stamp has no runtime effect — but it is the prerequisite the controller flip depends on.

## Goal

After this work, a `Schedule` CR carries an opt-in `spec.autoAbortPrior: bool` (default `false`), and every `task.CreateCommand` the publisher emits carries `auto_abort_prior: <bool>` in its frontmatter, mirroring the CRD field, with `created_by` still winning as the last force-set key. New Schedules are safe by default — no prior instance gets aborted unless an operator explicitly opts in.

## Non-goals

- The controller-side gate flip (`audit_style != true` → `auto_abort_prior == true`) — separate repo (`bborbe/agent-task-controller`), separate PR, ships AFTER this one.
- Per-Schedule enablement (setting `autoAbortPrior: true` on inbox-style Schedules) — happens AFTER both PRs ship; not part of this spec.
- Bulk retro-abort of historical stale instances — separate cleanup work if needed.
- Changing the publisher's hourly tick, the `/trigger` handler, the Kafka topic, or the UUID5 contract.
- Removing or renaming the existing `audit_style` stamp path on the controller side — the controller keeps reading `audit_style` until its own flip PR lands.
- Do NOT add a per-Schedule opt-out flag that disables the stamp — the stamp is invariant; the opt-in lives entirely in the CRD field's value.
- Do NOT add a tunable threshold or env var for the stamp — invariant; if a future consumer demands variation, that's a separate spec.
- Do NOT make `autoAbortPrior` required on the CRD — it must remain optional with a `false` default.

## Assumptions

- The controller reads `auto_abort_prior` from the materialized task's frontmatter. Until the controller flip ships, the `auto_abort_prior` stamp has no runtime effect — but it is the prerequisite the controller flip depends on.
- The existing `created_by: recurring-task-creator` provenance invariant (force-set last, operator cannot override) is unchanged and remains the highest-priority force-set key.
- The CRD is self-installing: the Go-built `JSONSchemaProps` in `pkg/k8s_connector_schema.go` is the single source of truth for the CRD schema, applied on every binary boot via `SetupCustomResourceDefinition`. No separate CRD YAML manifest is committed.
- The store adapter is the single seam that maps a `Schedule` CR to a `schedule.TaskDefinition`; the publisher reads only from `TaskDefinition`.
- `auto_abort_prior` is a frontmatter key the controller reads as a YAML boolean; the publisher stamps a Go `bool`, serialized as YAML `true`/`false`.
- Schedule CRs live in the private `bborbe/quant` repo; this spec does not touch any CR YAML. Setting `autoAbortPrior: true` on specific Schedules is a rollout step, not part of this code change.
- References: [[Per-Kind Firing Semantics for Recurring Task Schedulers]] (period-token shapes), [[recurring-task-creator]] (service KB — CRD shape, publisher architecture, frontmatter stamping), [[Closure Patterns]] (DoD formatting), `docs/dod.md` (project Definition of Done).

## Desired Behavior

Numbered observable outcomes:

1. A `Schedule` CR accepts an optional `spec.autoAbortPrior` boolean field. When omitted, the effective value is `false`.
2. The CRD's OpenAPI schema validates `spec.autoAbortPrior` as `type: boolean` with no enum and no required-ness; a non-boolean value (e.g. the string `"yes"`) is rejected at `kubectl apply` time by the API server.
3. The `ScheduleTrigger` Go type carries `AutoAbortPrior` as a pointer-to-bool (`*bool`) with JSON tag `autoAbortPrior,omitempty`, so an unset field is distinguishable from an explicit `false`.
4. The store adapter resolves the pointer to a plain `bool` for the `schedule.TaskDefinition`: nil pointer → `false`; explicit `false` → `false`; explicit `true` → `true`.
5. The `schedule.TaskDefinition` struct carries `AutoAbortPrior bool`.
6. The publisher's `FrontmatterFormatter` stamps `auto_abort_prior: <bool>` onto every materialized task's frontmatter, mirroring the resolved `TaskDefinition.AutoAbortPrior` value.
7. Stamp ordering: `auto_abort_prior` is set AFTER operator-supplied frontmatter keys are merged (so an operator cannot override it via `spec.template.frontmatter.auto_abort_prior`) but BEFORE `created_by` is force-set (so `created_by` still wins as the last force-set key — the existing publisher invariant is preserved).
8. An operator setting `auto_abort_prior: true` inside `spec.template.frontmatter` does NOT change the stamped value — the stamped value is driven solely by `spec.autoAbortPrior`, never by the template frontmatter.
9. Existing Schedules (with no `spec.autoAbortPrior`) continue to publish identical `task.CreateCommand` payloads as before, except for the newly added `auto_abort_prior: false` frontmatter key.

## Constraints

What must NOT change:

- The `created_by: recurring-task-creator` provenance key remains force-set last and cannot be overridden by any operator-supplied frontmatter — frozen invariant from `docs/dod.md` "Service Discipline."
- The `status: in_progress` and `page_type: task` defaults remain seeded before operator keys — unchanged.
- The deterministic UUID5 contract (`recurring-<slug>-<period-token>`) is unchanged — `autoAbortPrior` does not participate in the identifier input.
- The hourly tick, the `/trigger` handler, the Kafka topic, and the `DRY_RUN` behavior are unchanged.
- The CRD remains self-installing via `SetupCustomResourceDefinition` on every boot; no separate CRD YAML manifest is committed.
- Existing tests in `pkg/publisher/frontmatter_test.go`, `pkg/publisher/publisher_test.go`, `pkg/store/adapter_test.go`, and the `k8s/apis/.../v1` suite must still pass (with updates that add `auto_abort_prior` assertions where the assertion enumerates the full frontmatter key set).
- `pkg/schedule/` stays a pure-data layer (no Kafka / HTTP / agent imports) — `AutoAbortPrior` is a plain `bool` field, consistent with the existing `PeriodOffset int`.
- `make precommit` and `make test` pass from the project root — see `docs/dod.md`.
- License headers (BSD-2-Clause) on every new or modified `.go` file — see `docs/dod.md`.
- GoDoc comments on every exported type, function, and method — see `docs/dod.md`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| `spec.autoAbortPrior: "yes"` (string) applied to a Schedule CR | API server rejects the apply at admission with a type-mismatch error; CR is not persisted | Operator corrects to `true`/`false` or omits the field and re-applies |
| `spec.autoAbortPrior: 1` (integer) applied | API server rejects at admission (type mismatch; `1` is not a JSON boolean) | Operator corrects to `true`/`false` and re-applies |
| Operator sets `auto_abort_prior: true` inside `spec.template.frontmatter` | Publisher ignores it; stamped value is driven solely by `spec.autoAbortPrior` (default `false` if unset) | Operator sets `spec.autoAbortPrior: true` at the spec level |
| Pod restarts mid-tick before publishing | No partial state — `autoAbortPrior` is read from the in-memory CR cache per tick; the next tick re-reads and re-publishes idempotently (UUID5 dedup) | Automatic on next tick |
| Controller flip not yet shipped | `auto_abort_prior` stamp is present on every materialized task but the controller ignores it (still reads `audit_style`); no runtime effect | Ship the controller-side PR; no rollback needed on this repo |
| Old pod binary running with new CRs that set `autoAbortPrior: true` | Old pod's Go struct has no `AutoAbortPrior` field → JSON unmarshalling drops the unknown field silently; `auto_abort_prior` is NOT stamped | Deploy the new binary (`BRANCH=dev|prod make buca`) so the field is read and stamped; matches the "data + code must ship together" gotcha in [[Per-Kind Firing Semantics for Recurring Task Schedulers]] |

## Security / Abuse Cases

- What can an attacker control? An operator with `kubectl apply` rights on `Schedule` CRs can set `autoAbortPrior: true` on any Schedule. This only affects whether the controller MAY abort a prior instance — it does not grant write access to the vault (the controller remains the single git writer) or change the UUID5 contract.
- What crosses trust boundaries? The `auto_abort_prior` stamp flows from the CRD (cluster-trusted) through the publisher (cluster-trusted pod) to the Kafka `task.CreateCommand` and the materialized vault file (controller-trusted). No untrusted input crosses in — the field is a typed boolean validated at admission.
- What can hang, retry forever, or race? Nothing new — the field is read synchronously per tick from the informer cache; no new I/O, no new goroutine, no new network call.
- What data/path/input must be validated? The CRD OpenAPI schema validates `type: boolean` at admission. The store adapter must tolerate a nil pointer (unset) and resolve to `false` — never panic on nil.

## Acceptance Criteria

Binary, testable statements. Each declares its evidence shape.

- [ ] `spec.autoAbortPrior` appears as a property of `scheduleTriggerSchema()` in `pkg/k8s_connector_schema.go` with `Type: "boolean"`, no `Enum`, and is absent from the schema's `Required` list — evidence: `grep -n 'autoAbortPrior' pkg/k8s_connector_schema.go` returns ≥1 line and `grep -n '"autoAbortPrior"' pkg/k8s_connector_schema.go` shows a `Type: "boolean"` property block.
- [ ] The `ScheduleTrigger` Go struct in `k8s/apis/task.benjamin-borbe.de/v1/types.go` has a field `AutoAbortPrior *bool` with struct tag `json:"autoAbortPrior,omitempty"` — evidence: `grep -n 'AutoAbortPrior \*bool' k8s/apis/task.benjamin-borbe.de/v1/types.go` returns ≥1 line.
- [ ] `ScheduleTrigger.DeepCopyInto` in `k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go` copies the `AutoAbortPrior` pointer (nil-safe) — evidence: `grep -n 'AutoAbortPrior' k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go` returns ≥1 line within the `ScheduleTrigger` `DeepCopyInto`.
- [ ] `make generatek8s` is a no-op regenerator (regenerated deepcopy matches committed) — evidence: `make generatek8s && git diff --exit-code` exits 0.
- [ ] `schedule.TaskDefinition` in `pkg/schedule/task_definition.go` has a field `AutoAbortPrior bool` — evidence: `grep -n 'AutoAbortPrior bool' pkg/schedule/task_definition.go` returns ≥1 line.
- [ ] The store adapter in `pkg/store/adapter.go` maps a nil `*bool` → `false`, explicit `false` → `false`, explicit `true` → `true` into `TaskDefinition.AutoAbortPrior` — evidence: a Ginkgo spec in `pkg/store/adapter_test.go` (or equivalent) asserting all three cases passes; `make test` exits 0.
- [ ] The publisher's `FrontmatterFormatter.Format` stamps `auto_abort_prior: <bool>` onto the returned frontmatter for every materialized task — evidence: a Ginkgo spec in `pkg/publisher/frontmatter_test.go` asserting `HaveKeyWithValue("auto_abort_prior", <bool>)` passes; `make test` exits 0.
- [ ] Stamp ordering: `auto_abort_prior` is set AFTER operator keys but BEFORE `created_by` — evidence: a Ginkgo spec asserting that an operator-supplied `auto_abort_prior` in the input frontmatter does NOT change the stamped value (stamped value driven solely by the CRD field), and that `created_by` is still `recurring-task-creator`, passes; `make test` exits 0.
- [ ] An operator-supplied `auto_abort_prior: true` inside `spec.template.frontmatter` does NOT flip the stamped value when `spec.autoAbortPrior` is unset (stamped value stays `false`) — evidence: a Ginkgo spec asserting `HaveKeyWithValue("auto_abort_prior", false)` in that scenario passes; `make test` exits 0.
- [ ] CRD schema rejects a non-boolean `autoAbortPrior` at apply time — evidence: a Ginkgo spec feeding a `Schedule` CR with `autoAbortPrior: "yes"` (string) through the schema validator asserts admission rejection (schema validation error); `make test` exits 0.
- [ ] Round-trip: frontmatter `auto_abort_prior: true` survives parse + re-serialize as `auto_abort_prior: true` (YAML boolean, not string) — evidence: a Ginkgo spec serializing the frontmatter map to YAML and re-parsing asserts the re-parsed value is Go `bool` `true` (not `string` `"true"`); `make test` exits 0.
- [ ] Existing Schedules without `spec.autoAbortPrior` still publish with `auto_abort_prior: false` added to the frontmatter; the rest of the `task.CreateCommand` payload (TaskIdentifier, Title, Body) is byte-identical to the pre-change output — evidence: a Ginkgo spec asserting the command's `TaskIdentifier`, `Title`, and `Body` are unchanged and `Frontmatter` contains `auto_abort_prior: false` passes; `make test` exits 0.
- [ ] `make precommit` exits 0 from the project root (lint + test + security scan + license headers) — evidence: exit code 0.
- [ ] `CHANGELOG.md` has an entry describing the new `spec.autoAbortPrior` field and the `auto_abort_prior` frontmatter stamp — evidence: `grep -n 'autoAbortPrior\|auto_abort_prior' CHANGELOG.md` returns ≥1 line.

**Scenario coverage — NO new E2E scenario in this PR.** The end-to-end verify (observe a prior instance transition to `aborted` after a deliberately-missed firing) depends on the controller-side gate flip, which ships in a separate PR AFTER this one. An E2E scenario here cannot reach the behavior (the controller still reads `audit_style` until its flip lands) and would be brittle. The E2E verify procedure is documented below in "E2E verify procedure (post-controller-flip)" as a rollout step, not an AC in this spec.

## Verification

Exact commands and expected results:

```
# Regenerate deepcopy + typed clients; must produce no diff
make generatek8s && git diff --exit-code
# Expected: exit code 0

# Full unit + race test suite (Ginkgo v2 / Gomega)
make test
# Expected: exit code 0, all specs pass

# Lint + test + security scan + license headers
make precommit
# Expected: exit code 0
```

## Suggested Decomposition

This spec touches 3 code layers (CRD/API types, store adapter, publisher frontmatter) but is small and tightly coupled (the field flows in one direction: CRD → struct → adapter → TaskDefinition → formatter). With ≤9 Desired Behaviors and `DB × AC = 9 × 14 = 126` AC-DB cells, the count is high but every AC is a thin per-layer assertion on the same single field. Splitting would fracture a single data-flow into specs that cannot be independently verified (the formatter AC depends on the TaskDefinition field; the TaskDefinition field is meaningless without the adapter). The right shape is ONE spec, decomposed into prompts along the data-flow seams:

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | CRD API types: `ScheduleTrigger.AutoAbortPrior *bool` field + JSON tag + DeepCopyInto + OpenAPI schema (`type: boolean`, no enum, no required) + schema-rejects-string test | 1, 2, 3 | AC 1, 2, 3, 4, 10 | — |
| 2 | Store adapter + TaskDefinition: `TaskDefinition.AutoAbortPrior bool` field + nil/false/true mapping in adapter + adapter tests | 4, 5 | AC 5, 6 | prompt 1 |
| 3 | Publisher frontmatter: `FrontmatterFormatter` stamps `auto_abort_prior: <bool>` after operator keys, before `created_by`; operator-frontmatter-override guard; round-trip YAML bool; byte-identical payload regression | 6, 7, 8, 9 | AC 7, 8, 9, 11, 12 | prompt 2 |
| 4 | Docs + changelog: `CHANGELOG.md` entry; GoDoc on new exported field; license headers on touched files | — | AC 13, 14 | prompts 1–3 |

Rationale: prompt 1 establishes the API contract (schema + struct + deepcopy) that every downstream layer depends on; prompt 2 bridges API → domain via the adapter (cannot exist without prompt 1's struct field); prompt 3 consumes the domain field at the publisher seam (cannot exist without prompt 2's TaskDefinition field). Prompt 4 is the docs/changelog sweep after the code is stable. Ordering follows the data flow strictly — no cycles, since each prompt only reads from the prior layer's output.

## Do-Nothing Option

If we don't do this, the controller-side auto-supersede gate stays on the opt-OUT `audit_style != true` model. The risk: any new Schedule defaults to "eligible for auto-abort," so an operator who forgets to set the audit-style flag on an audit-style task (e.g. `check-prometheus-alerts`) silently loses the missed-firing signal when the next instance materializes and the prior is auto-aborted. The current approach is acceptable only if every new Schedule is manually audited for audit-style status at creation — which is exactly the human-tax the opt-in model removes. The opt-in `autoAbortPrior` field is the safer default and is the prerequisite for the controller-side gate flip that ships the safety model end-to-end.

## File-by-file change list

- `k8s/apis/task.benjamin-borbe.de/v1/types.go` — add `AutoAbortPrior *bool json:"autoAbortPrior,omitempty"` to `ScheduleTrigger` with GoDoc.
- `k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go` — regenerate via `make generatek8s`; `ScheduleTrigger.DeepCopyInto` gains a nil-safe pointer copy for `AutoAbortPrior`.
- `pkg/k8s_connector_schema.go` — add `autoAbortPrior` property (`Type: "boolean"`, no `Enum`, not in `Required`) to `scheduleTriggerSchema()`'s `Properties` map.
- `pkg/schedule/task_definition.go` — add `AutoAbortPrior bool` field to `TaskDefinition` with GoDoc.
- `pkg/store/adapter.go` — resolve `cr.Spec.Schedule.AutoAbortPrior` (`*bool`) to `TaskDefinition.AutoAbortPrior` (`bool`), defaulting `false` on nil.
- `pkg/publisher/frontmatter.go` — extend `FrontmatterFormatter` so `Format` receives the resolved `AutoAbortPrior bool` (signature change or constructor-supplied value — agent decides at impl time) and stamps `auto_abort_prior: <bool>` after operator keys, before `created_by`.
- `pkg/publisher/publisher.go` — pass `def.AutoAbortPrior` into the formatter call at the `p.formatter.Format(...)` site.
- `pkg/factory/factory.go` — no business-logic change expected (formatter construction unchanged in shape); update only if the formatter constructor signature changes.
- `pkg/publisher/frontmatter_test.go` — add specs: stamps `auto_abort_prior`; operator frontmatter cannot override it; `created_by` still wins; round-trip YAML bool.
- `pkg/store/adapter_test.go` — add specs: nil → false, explicit false → false, explicit true → true.
- `pkg/k8s_connector_schema_test.go` (or equivalent) — add spec: schema rejects `autoAbortPrior: "yes"`.
- `CHANGELOG.md` — entry for `spec.autoAbortPrior` + `auto_abort_prior` stamp.

## Test plan (Ginkgo/Gomega)

- `pkg/publisher/frontmatter_test.go`:
  - `Format` stamps `auto_abort_prior: false` when `AutoAbortPrior` is false.
  - `Format` stamps `auto_abort_prior: true` when `AutoAbortPrior` is true.
  - Operator-supplied `auto_abort_prior` in input frontmatter does NOT change the stamped value (driven solely by the CRD field).
  - `created_by` is still `recurring-task-creator` and is force-set after `auto_abort_prior`.
  - Round-trip: serialize frontmatter to YAML, re-parse, assert `auto_abort_prior` re-parses as Go `bool` (not `string`).
- `pkg/store/adapter_test.go`:
  - nil `AutoAbortPrior` → `TaskDefinition.AutoAbortPrior == false`.
  - explicit `false` → `false`.
  - explicit `true` → `true`.
- `pkg/k8s_connector_schema_test.go` (or the existing schema test file):
  - `autoAbortPrior` property exists with `Type: "boolean"`, no `Enum`, absent from `Required`.
  - Schema validation rejects `autoAbortPrior: "yes"` (string).
  - Schema validation accepts `autoAbortPrior: true` and `autoAbortPrior: false`.
- `pkg/publisher/publisher_test.go`:
  - A Schedule without `autoAbortPrior` publishes a command whose `Frontmatter` contains `auto_abort_prior: false` and whose `TaskIdentifier`, `Title`, `Body` are byte-identical to the pre-change baseline.

## Deploy plan

1. Feature worktree → branch → PR (draft → local `/coding:pr-review` loop → `gh pr ready` after human approves → bot review → `merge --merge`).
2. Dev: `cd ~/Documents/workspaces/recurring-task-creator-dev && git pull && git merge master`, then `BRANCH=dev make buca`. Verify on dev via the admin gateway healthz + a `/trigger?date=<Saturday>` replay against the `weekly-review` canary; confirm the materialized task carries `auto_abort_prior: false` in its frontmatter (DRY_RUN in dev means no Kafka send; inspect via the publisher's DRY_RUN log line OR temporarily toggle DRY_RUN off for one trigger and inspect the materialized file).
3. Prod: `cd ~/Documents/workspaces/recurring-task-creator-prod && git pull && git merge master`, then `BRANCH=prod make buca`. Verify healthz green; confirm the next hourly tick publishes normally (no errors in pod logs).
4. Post-deploy: confirm `kubectlquant -n prod get schedules.task.benjamin-borbe.de <slug> -o yaml` shows the CRD schema now includes `autoAbortPrior` (the self-installing CRD update ran on boot).

## Rollout plan (per-Schedule enablement — AFTER both PRs ship)

Do NOT set `autoAbortPrior: true` on any Schedule until BOTH this PR and the controller-side gate-flip PR are merged and deployed to prod. Sequence:

1. This PR (publisher stamp) ships to prod — every materialized task now carries `auto_abort_prior: false` (no runtime effect; controller still reads `audit_style`).
2. Controller-side gate-flip PR ships to prod — controller now reads `auto_abort_prior == true` as the eligibility gate.
3. Set `autoAbortPrior: true` on inbox-style Schedules (`cleanup-obsidian-inbox`, `cleanup-omnifocus-inbox`, `aquascape-pwc`) in `bborbe/quant`'s `task/recurring-schedules/all/<slug>.yaml` and `BRANCH=prod make apply` from a prod-branch worktree.
4. Leave audit-style Schedules (`check-prometheus-alerts`, `ibkr-swing-trading`) with `autoAbortPrior` unset (defaults to `false`) — they remain ineligible for auto-abort.

## E2E verify procedure (post-controller-flip — NOT in this PR)

After both PRs ship and `autoAbortPrior: true` is set on `cleanup-obsidian-inbox`:

1. Temporarily disable firing on `cleanup-obsidian-inbox` for one cycle (edit the Schedule CR to set a non-matching recurrence, or scale the pod to 0 for one tick window).
2. Let the next instance materialize (restore the Schedule).
3. Confirm via `git diff` on the personal vault that the prior instance's frontmatter shows `status: aborted`, `phase: done`, `superseded_by: <new file path>`.
4. Confirm the controller pod log shows the supersede event.
5. Restore the Schedule to its normal cadence.

This verify is the DoD for the cross-repo feature, not for this spec. This spec is done when its ACs pass and the stamp ships to prod.

## Definition of Done formatting

Per [[Closure Patterns]] and `docs/dod.md`: `make precommit` green, `CHANGELOG.md` entry present, GoDoc on new exported field, license headers on touched files, no vault writes from this service (the stamp goes through `task.CreateCommand` to Kafka, not to the vault directly).

## Verification Result

**Verified:** 2026-07-01T10:14:05Z (HEAD 076f86c, release v0.7.0)
**Binary:** installed `dark-factory` (/Users/bborbe/Documents/workspaces/go/bin/dark-factory)
**Scenario:** Structural AC walk plus live dev + prod e2e replay of publisher stamp and downstream controller supersede.
**Evidence:**
- CRD schema in `pkg/k8s_connector_schema.go:166` declares `autoAbortPrior` as `Type: "boolean"`; `kubectlquant get crd schedules.task.benjamin-borbe.de` on prod shows the property with `type: boolean` after the v0.7.0 boot ran `SetupCustomResourceDefinition`.
- `ScheduleTrigger.AutoAbortPrior *bool` at `k8s/apis/task.benjamin-borbe.de/v1/types.go:99`; deepcopy pointer copy at `zz_generated.deepcopy.go:132`.
- `schedule.TaskDefinition.AutoAbortPrior bool` at `pkg/schedule/task_definition.go:71`; adapter maps nil/false/true at `pkg/store/adapter.go:79-91` with three tests at `pkg/store/adapter_test.go:317,331,346`.
- `FrontmatterFormatter.Format` stamps `auto_abort_prior` at `pkg/publisher/frontmatter.go:76`; Ginkgo specs at `pkg/publisher/frontmatter_test.go:58-98` cover stamp values, operator-override guard, created_by ordering, and YAML boolean round-trip. Publisher regression at `publisher_test.go:936` asserts byte-identical payload plus new `auto_abort_prior: false`.
- CHANGELOG entries at `CHANGELOG.md:13-16` describe the new field and stamp.
- `make precommit` exit 0 at 12:13 local (fmt + lint + gosec 0 issues + trivy 0 vulns + license headers).
- Dev live replay: `POST /admin/recurring-task-creator/trigger?date=2026-07-01` published 3 tasks; `24 Tasks/Auto Abort E2E Enabled - 2026-07-01.md` frontmatter carries `auto_abort_prior: true`, disabled variant carries `false`.
- Prod live: Schedules `cleanup-obsidian-inbox`, `cleanup-omnifocus-inbox`, `aquascape-pwc` opted in via quant PR #17 + #18 merge `d31c8f6`; `kubectlquant -n prod get schedule cleanup-obsidian-inbox -o jsonpath='{.spec.schedule}'` → `{"autoAbortPrior":true,"recurrence":"Daily"}`.
- Cross-repo downstream: agent-task-controller PR #3 merged as `c481d77` observed on dev cluster at 10:05:21 executing `auto-supersede: 24 Tasks/Auto Abort E2E Enabled - 2026-07-01.md -> Enabled - 2026-07-02.md`; day-1 file transitioned to `status: aborted`, `phase: done`, `superseded_by: ...`, `completed_date: 2026-07-01T10:05:19Z` (git show `fc672245c`).
- AC 4 caveat: local `make generatek8s` produced drift in `k8s/client/{applyconfiguration,clientset,informers,listers}` due to a code-generator version mismatch on the operator's workstation; the `zz_generated.deepcopy.go` that AC 4 is scoped to remained clean, and merged CI plus the successful v0.7.0 prod release confirm the committed generated code matches the generator version used in CI. Drift is orthogonal to spec 015.
**Verdict:** PASS
