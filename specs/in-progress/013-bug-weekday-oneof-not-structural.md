---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-06-25T08:04:39Z"
generating: "2026-06-25T08:04:40Z"
prompted: "2026-06-25T08:15:46Z"
branch: dark-factory/bug-weekday-oneof-not-structural
---

## Summary

- Spec 012 widened `Schedule.spec.schedule.weekday` to `oneOf{string, array}`. Kubernetes structural-schema validation forbids that shape: the top-level `type` is empty AND `type` is set inside `oneOf` branches, both of which structural-schema rules reject.
- `kubectl apply` of the new CRD is refused by the API server's structural-schema admission check; the dev `recurring-task-creator-0` StatefulSet has been in `CrashLoopBackOff` since 2026-06-25 ~07:45 UTC because `SetupCustomResourceDefinition` fails on every pod start.
- Fix: drop `oneOf`; replace with two sibling, type-pure fields — `weekday: string` (unchanged, byte-identical backward compatibility) and `weekdays: []string` (new). CEL enforces "exactly one of" on `Weekday` recurrence and "neither" on every other recurrence kind.
- A regression-lock unit test runs the assembled CRD schema through `k8s.io/apiextensions-apiserver/pkg/apiserver/schema.NewStructural(...)`. The test fails on the current code and passes after the fix — preventing future schemas that compile in Go but the API server rejects at admission.
- All spec-012 internals (adapter `WeekdayList`, matcher day-set, period-token rendering, UUID5 stability, normalization map) are correct and stay untouched. Only the wire shape changes.

## Problem

The dev cluster's `recurring-task-creator-0` pod is in `CrashLoopBackOff` and has been since 2026-06-25 ~07:45 UTC. Service is down on dev. Root cause: the CRD schema shipped in spec 012 uses `oneOf` to express "single string OR array of strings" for `spec.schedule.weekday`. Kubernetes structural-schema validation — enforced at CRD admission time since v1.16 and required by API server invariants — forbids exactly this shape. There is no way to express "string OR array" in a structural schema; the API server rejects the CRD update on every pod restart, so `SetupCustomResourceDefinition` errors out and the pod exits. The error from the API server, captured verbatim from pod logs:

```
spec.validation.openAPIV3Schema.properties[spec].properties[schedule].properties[weekday].oneOf[0].type: Forbidden: must be empty to be structural
spec.validation.openAPIV3Schema.properties[spec].properties[schedule].properties[weekday].type: Required value: must not be empty for specified object fields
```

This is a structural-schema invariant the API server enforces, not a transient bug. The shape cannot be made to work; it must be replaced.

## Reproduction

```bash
cd ~/Documents/workspaces/recurring-task-creator-weekday-list
git log --oneline -1   # HEAD: 1246391 (post-spec-012 merge)

# 1. Build and apply the controller's CRD-install path
kubectlquant -n dev rollout restart statefulset/recurring-task-creator

# 2. Observe the pod fails to start
kubectlquant -n dev get pod recurring-task-creator-0
# STATUS: CrashLoopBackOff

# 3. Inspect the failure
kubectlquant -n dev logs recurring-task-creator-0 --previous | grep -A2 'Forbidden\|must not be empty'
```

Observed evidence (verbatim, copy from a live `kubectl logs --previous`):

```
spec.validation.openAPIV3Schema.properties[spec].properties[schedule].properties[weekday].oneOf[0].type: Forbidden: must be empty to be structural
spec.validation.openAPIV3Schema.properties[spec].properties[schedule].properties[weekday].type: Required value: must not be empty for specified object fields
```

The same error appears every restart; the pod never reaches the leader-election or informer-sync phases.

Repository version: `recurring-task-creator` HEAD `1246391`, branch `master` post-merge of `dark-factory/weekday-list-and-short-forms` (spec 012).

## Expected vs Actual

**Expected** (per spec 012's Goal and per `docs/architecture.md` "the controller installs its own CRDs on startup"): the controller installs / updates the `schedules.task.benjamin-borbe.de` CRD without error; the pod reaches Running; the informer cache fills.

**Actual**: API server rejects the CRD update with the structural-schema violation above. `SetupCustomResourceDefinition` returns the wrapped error; the pod exits non-zero; StatefulSet restarts it; same failure repeats. Service is down on dev. The exact log line cited above is reproducible on every pod start.

## Why this is a bug

1. Kubernetes structural-schema rules (apiextensions-apiserver, since v1.16) require: (a) every field's `type` is set and non-empty at the top level; (b) `type`, `additionalProperties`, `default`, `nullable`, `description`, `example`, and several other keywords are forbidden inside `oneOf` / `anyOf` / `allOf` / `not` branches. See `k8s.io/apiextensions-apiserver/pkg/apiserver/schema/structural.go` and the upstream [structural schemas doc](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#specifying-a-structural-schema). The spec-012 schema violates both rules.
2. "String OR array of strings" is **not expressible** in a structural schema — there is no `type: [string, array]` polymorphism (that's plain OpenAPI 3.0 but explicitly forbidden by k8s structural rules). The only way to ship the spec-012 user-facing goal (one CR per multi-day task) is to use two separate, type-pure fields.
3. The auditor's report on spec 012 missed a Should-Fix: a `NewStructural()` round-trip unit test on the assembled CRD schema would have caught this in CI before merge. The fix MUST include that test as a regression lock — every future schema change runs through it.

## Goal

After this work:

- `recurring-task-creator-0` on dev reaches and stays in `Running`. `SetupCustomResourceDefinition` succeeds.
- A single `Schedule` CR can still declare an arbitrary non-empty set of weekdays — via the new `weekdays: []string` field (mirroring k8s plural conventions: `tolerations`, `containers`, `args`).
- The `weekday: string` field is back to its pre-spec-012 single-string form and is byte-identically backward-compatible: every existing single-day CR keeps working with the same UUID5, period token, title, and body.
- CEL enforces: on `Weekday` recurrence, exactly one of `weekday` or `weekdays` is set; on every other recurrence kind, neither is set.
- The CRD's assembled OpenAPI schema passes `k8s.io/apiextensions-apiserver/pkg/apiserver/schema.NewStructural(...)` in unit tests — failing before the fix, passing after.
- Spec 012's internals (adapter `WeekdayList`, matcher day-set, period-token rendering, UUID5 derivation, 14-element enum, normalization map) are unchanged. Only the wire surface — the CRD schema, the CEL rules, and the `ScheduleTrigger` Go struct's exposed fields — changes.

## Desired Behavior

1. `Schedule.spec.schedule.weekday` is a single string (no `oneOf`). Its OpenAPI shape and enum match the pre-spec-012 schema byte-for-byte (long-form day names only, 7 strings: `Monday`..`Sunday`).
2. `Schedule.spec.schedule.weekdays` is a new, optional `[]string` field. Item enum is the 14-element long+short set (`Monday`..`Sunday` AND `Mon`..`Sun`) — identical to spec 012's enum. `MinItems: 1` when present. No `maxItems` cap.
3. CEL rule on `Weekday` recurrence: **exactly one** of `weekday` or `weekdays` must be set (XOR). Both-set is rejected; neither-set is rejected.
4. CEL rule on every non-`Weekday` recurrence: **neither** `weekday` nor `weekdays` may be set. (This replaces the existing `weekday-iff-Weekday` rule with a two-field equivalent — same intent, two-field surface.)
5. CEL rule on `weekdays` (when present): no duplicates including cross-form (`[Mon, Monday]`, `[Tue, Tue]`, `[Wednesday, Wednesday]` all rejected). Unknown day strings rejected by the item enum.
6. The assembled CRD schema, when converted to internal `apiextensions.JSONSchemaProps` and run through `apiserver/pkg/apiserver/schema.NewStructural(...)`, returns no error. A regression-lock unit test asserts this and is permanently wired into `make precommit`.
7. The `ScheduleTrigger` Go struct in `k8s/apis/.../v1` gains a `Weekdays []string` field next to the existing `Weekday string` field. Both are `omitempty`. The adapter (per spec 012) reads either: if `Weekday` is non-empty, it produces a one-element `WeekdayList`; if `Weekdays` is non-empty, it normalizes and dedups into a `WeekdayList`. The publisher / matcher / period-token code is unchanged — it already consumes the canonical `WeekdayList` internally.
8. Single-string CR (`weekday: Monday`) continues to apply, produces byte-identical period token, UUID5, title, and body to the pre-spec-012 publisher. The 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` still passes.
9. New list CR (`weekdays: [Mon, Wed, Fri]`) is accepted at apply, produces 3 distinct task files per ISO week with per-day period token and per-day UUID5 — identical end-state to spec 012's user-facing promise.

## Constraints

- CRD group, version, kind, plural, singular, short name unchanged. Adding a new field (`weekdays`) is permitted; renaming or removing existing fields is not.
- The `weekday` field returns to its pre-spec-012 type (`string`) and pre-spec-012 enum (7 long-form days only). Operators using `weekday: Mon` (short form) — which spec 012 introduced — will be rejected. This is acceptable: no in-cluster CR uses the short form on the `weekday` field today (the OneOf schema never landed), and short-form usage migrates to the `weekdays` field naturally.
- The `RecurrenceKind` wire enum (`Daily`, `Weekly`, `Weekday`, `Monthly`, `Quarterly`, `Yearly`) is frozen by spec 9; unchanged.
- Period-token format (`YYYYWww-<abbrev>`) frozen by specs 6/8/9; unchanged.
- UUID5 derivation (`recurring-<slug>-<period-token>`) frozen by spec 6; single-string CRs MUST produce byte-identical UUID5 post-fix.
- Spec 012's internals: `WeekdayList` type, the 14-element normalization map in `pkg/k8s_connector_schema.go`, the day-set matcher in `pkg/publisher`, and the 21-entry stability block in `pkg/publisher/publisher_test.go` MUST remain unchanged on the inside. Only the wire surface (CRD schema, CEL rules, `ScheduleTrigger` Go struct) changes.
- The `weekday-iff-Weekday` CEL rule is replaced by a two-field equivalent: the same field-path semantics and a comparable message; tests on existing single-string-on-non-Weekday rejection must continue to pass.
- Project DoD applies (`docs/dod.md`): `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` on every error path; Ginkgo v2 / Gomega; no `time.Now()` / `context.Background()` in business logic.
- `make precommit` exits 0 from repo root after every prompt lands.
- The fix lands in `master`, merges to `dev`, and the dev pod returns to Running before the spec is marked `completed`.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility |
|---------|-------------------|-----------|----------|---------------|
| Operator applies `weekday: Monday` + `weekdays: [Mon]` on a `Weekday` CR (both set) | API server rejects via CEL "exactly one of weekday/weekdays" rule. | `kubectl apply` exits non-zero with the CEL message naming `spec.schedule`. | Operator drops one of the two fields. | Reversible (no state change). |
| Operator applies a `Weekday` CR with neither `weekday` nor `weekdays` set | API server rejects via the same CEL XOR rule. | `kubectl apply` exits non-zero. | Operator supplies one of the two. | Reversible. |
| Operator applies `weekdays: [Mon, Wed]` on a `Daily` recurrence (or any non-Weekday) | API server rejects via the "neither-on-non-Weekday" CEL rule. | `kubectl apply` exits non-zero. | Operator drops `weekdays` or switches to `recurrence: Weekday`. | Reversible. |
| Operator applies `weekdays: []` (empty list) | API server rejects via OpenAPI `MinItems: 1`. | `kubectl apply` exits non-zero with the OpenAPI message naming `weekdays`. | Operator supplies at least one day. | Reversible. |
| Operator applies `weekdays: [Mon, Monday]` (cross-form duplicate) | API server rejects via CEL "no duplicates including cross-form" rule. | `kubectl apply` exits non-zero. | Operator removes the duplicate. | Reversible. |
| Operator applies `weekdays: [Mon, FunDay]` (unknown day) | API server rejects via item enum. | `kubectl apply` exits non-zero. | Operator fixes the value. | Reversible. |
| Existing single-string `weekday: Monday` CR continues to apply post-fix | Adapter produces the same `WeekdayList{time.Monday}` as before; period token, UUID5, title, body byte-identical. | The 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` passes; vault file identifier unchanged. | None — invariant. | N/A. |
| Operator applies `weekday: Mon` (short form, accepted under spec 012's enum but no longer accepted) | API server rejects via the restored long-form-only `weekday` enum. | `kubectl apply` exits non-zero. | Operator switches to `weekday: Monday` or to `weekdays: [Mon]`. | Reversible — no in-cluster CR uses short form on `weekday` today; the OneOf schema never successfully installed. |
| CRD apply during pod start | `SetupCustomResourceDefinition` succeeds; pod reaches Running; informer cache fills. | `kubectlquant -n dev get pod recurring-task-creator-0` shows `Running`; `kubectl logs` shows informer sync. | None — invariant. | N/A. |
| Future schema change reintroduces `oneOf` / `anyOf` / nested `type` | The structural-schema round-trip unit test fails at `make precommit`. CI blocks the merge. | `go test ./pkg/...` prints a FAIL line naming the round-trip spec; `make precommit` exits non-zero. | Author of the change reshapes the schema to be type-pure. | Reversible — defensive lock. |

## Acceptance Criteria

- [ ] **Regression-lock test exists and passes after fix.** A Ginkgo `It` in `pkg/k8s_connector_schema_test.go` (or `pkg/k8s_connector_structural_test.go`) builds the full assembled CRD's OpenAPI schema, converts it to internal `apiextensions.JSONSchemaProps`, runs it through `k8s.io/apiextensions-apiserver/pkg/apiserver/schema.NewStructural(...)`, and asserts the returned error is nil. Evidence: `go test -v -run 'Structural' ./pkg/...` prints PASS for this spec; the same test, run against the spec-012 HEAD (`1246391`), prints FAIL with the same "must be empty to be structural" / "must not be empty for specified object fields" messages quoted in the Reproduction section.
- [ ] **Post-Deploy (Rung-2):** **CRD applies on dev without API-server rejection.** Evidence: `kubectlquant -n dev rollout restart statefulset/recurring-task-creator` followed by `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.phase}'` returns `Running` within 60 seconds; `kubectlquant -n dev logs recurring-task-creator-0 | grep -c 'must be empty to be structural'` returns `0`.
  - `deploy_check:` `kubectlquant -n dev get statefulset/recurring-task-creator -o jsonpath='{.spec.template.spec.containers[0].image}'` returns the post-fix image digest.
  - `deploy_target:` dev cluster, namespace `dev`, statefulset `recurring-task-creator`.
- [ ] **Single-string `weekday: Monday` CR still works.** Evidence: a Ginkgo `It` in `pkg/k8s_connector_validation_test.go` applies a CR with `recurrence: Weekday, weekday: Monday` and asserts the validator returns no error; PASSes. The existing 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` still passes — evidence: `go test -v -run 'UUID5 stability' ./pkg/publisher/...` prints PASS.
- [ ] **New list `weekdays: [Mon, Wed, Fri]` CR is accepted.** Evidence: validation test asserts no error for `recurrence: Weekday, weekdays: [Mon, Wed, Fri]`; PASSes. Publisher test seeds this CR and asserts 3 distinct `task.CreateCommand` messages emitted across one ISO week with period tokens `2026Www-mon`, `2026Www-wed`, `2026Www-fri` and 3 distinct UUID5s; PASSes.
- [ ] **Both fields set together is rejected.** Evidence: validation test asserts the validator returns an error whose message names "exactly one" (or equivalent) for `recurrence: Weekday, weekday: Monday, weekdays: [Mon]`; PASSes.
- [ ] **Neither field set on `Weekday` recurrence is rejected.** Evidence: validation test asserts error for `recurrence: Weekday` with no `weekday` and no `weekdays`; PASSes.
- [ ] **`weekday` or `weekdays` set on non-`Weekday` recurrence is rejected.** Evidence: validation test asserts error for each of: `recurrence: Daily, weekday: Monday`; `recurrence: Daily, weekdays: [Mon]`; `recurrence: Weekly, weekdays: [Mon, Wed]`; all PASS (rejected as expected).
- [ ] **Empty list, duplicate days, unknown day strings all rejected.** Evidence: validation test asserts errors for `weekdays: []` (empty), `weekdays: [Mon, Monday]` (cross-form dup), `weekdays: [Tue, Tue]` (same-form dup), `weekdays: [Mon, FunDay]` (unknown); all PASS.
- [ ] **Spec-012 internals untouched.** Evidence: `git diff <pre-fix>..HEAD -- pkg/publisher/ pkg/store/ pkg/schedule/` shows no changes to the `WeekdayList` type, the day-set matcher, the period-token rendering, or the normalization map. The adapter gains only the parse branch reading the new `Weekdays` wire field; the existing `Weekday` parse branch is unchanged.
- [ ] **`ScheduleTrigger` Go struct gains `Weekdays []string`.** Evidence: `grep -n 'Weekdays.*\[\]string' k8s/apis/task.benjamin-borbe.de/v1/*.go` returns line ≥1; the field has `json:"weekdays,omitempty"` tag mirroring the existing `Weekday` field.
- [ ] **CHANGELOG entry under `## Unreleased`.** Evidence: `grep -nE 'fix:.*weekday|fix:.*structural|fix:.*CRD' CHANGELOG.md` returns line ≥1 under the `## Unreleased` heading.
- [ ] **`make precommit` exits 0** from repo root. Evidence: exit code 0.
- [ ] **Post-Deploy (Rung-2):** **Bug reproduction no longer reproduces.** Evidence: replay the Reproduction section steps against the post-fix HEAD on dev; `kubectlquant -n dev logs recurring-task-creator-0 | grep -c 'must be empty to be structural'` returns `0`; pod stays `Running` for ≥5 minutes without restart.
  - `deploy_check:` `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.containerStatuses[0].restartCount}'` does not change over a 5-minute window after rollout completes.
  - `deploy_target:` dev cluster, namespace `dev`, pod `recurring-task-creator-0`.

## Workaround

None viable. The CRD failure happens at API-server admission on every pod start; there is no operator-side mitigation that avoids the CRD update. Manually applying an earlier CRD version via `kubectl apply -f <old.yaml>` and disabling `SetupCustomResourceDefinition` is not exposed via config and would freeze every future schema evolution. The fix must land.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-weekday-list
make precommit
go test -v -run 'Structural|Weekdays|WeekdayList|XOR|NeitherOnNonWeekday' ./pkg/...

# Post-deploy on dev:
kubectlquant -n dev rollout restart statefulset/recurring-task-creator
kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.phase}'
kubectlquant -n dev logs recurring-task-creator-0 | grep -c 'must be empty to be structural'
```

Expected:
- `make precommit` exits 0.
- The targeted `go test` invocation prints PASS for every matched spec, including the new structural-schema round-trip lock.
- `get pod` returns `Running` within 60 seconds of rollout restart.
- The `grep -c` returns `0`.

### Bug-spec verification (mandatory)

Per `docs/bug-workflow.md`, the Reproduction steps in this spec are replayed against the post-fix deployed pod. The bug is considered fixed only when:

1. `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.phase}'` returns `Running`.
2. `kubectlquant -n dev logs recurring-task-creator-0 | grep 'must be empty to be structural'` returns no lines (exit 1).
3. The pod stays Running for ≥5 minutes without restart (`kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.containerStatuses[0].restartCount}'` is stable).
4. Applying a fresh `weekdays: [Mon, Wed, Fri]` CR succeeds; `kubectlquant -n dev get schedules` lists it.

Tests-passing alone does NOT satisfy verification — the dev runtime must demonstrate the new schema installs and serves.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | CRD schema reshape + CEL rules + regression-lock test. Revert `spec.schedule.weekday` to pre-spec-012 single-string with 7-element long-form enum. Add new `weekdays: []string` field with the 14-element enum and `MinItems: 1`. Replace the `weekday-iff-Weekday` CEL rule with a two-field equivalent: XOR on `Weekday` recurrence, neither on non-`Weekday`. Keep the spec-012 no-duplicates CEL on the new `weekdays` field. Add the `NewStructural()` round-trip unit test in `pkg/k8s_connector_schema_test.go`. Update `pkg/k8s_connector_validation_test.go` with the full accept/reject table for the two-field shape. No Go-side adapter changes yet — the in-memory `WeekdayList` and matcher stay as-is. | 1, 2, 3, 4, 5, 6 | 1, 3, 4, 5, 6, 7, 8 (partial: schema/CEL side) | — |
| 2 | Go-side adapter widening + `ScheduleTrigger` struct + e2e. Add `Weekdays []string` to the `ScheduleTrigger` Go struct in `k8s/apis/task.benjamin-borbe.de/v1`. Update the store package's adapter to read either `Weekday` (single-string) or `Weekdays` (list); both paths converge on the existing internal `WeekdayList`. Add the parse-side table tests for `Weekdays` and the byte-identical-to-pre-spec-012 single-string regression check. Land CHANGELOG entry. Deploy to dev; verify pod returns to Running; replay the bug reproduction; mark verifying. | 7, 8, 9 | 2, 4 (publisher side), 8 (adapter side), 9, 10, 11, 12 | prompt 1 |

Rationale: prompt 1 lands the schema fix standalone — the API server accepts the new CRD shape immediately, the dev pod can install it, and the existing single-string adapter still works because spec 012's adapter reads `Weekday` and the field still exists. Prompt 2 then exposes `Weekdays` end-to-end so operators can actually use the new field. Splitting in the other order would leave the adapter exposing a field the CRD rejects — operators couldn't apply list CRs between prompts.

## Related

- **Predecessor (introduced the broken shape):** `specs/completed/012-weekday-list-and-short-forms.md`
- **Predecessor (CRD types + schema + connector):** `specs/in-progress/008-crd-scaffolding.md`
- **Predecessor (informer-backed inventory; store seam):** `specs/in-progress/010-informer-backed-inventory.md`
- **Predecessor (recurrence enum closed):** `specs/completed/009-weekday-kind-split.md`
- **Predecessor (period token + UUID5):** `specs/completed/006-period-anchored-uuid.md`
- **Parent goal (Personal Obsidian vault, informational):** `[[Migrate vault-cli Recurring Tasks to recurring-task-creator]]`

## Assumptions

- No in-cluster `Schedule` CR currently uses `weekday: Mon` (short form on the single field) or `weekday: [...]` (list shape) — the spec-012 CRD update never successfully installed, so the only shapes that ever reached etcd are pre-spec-012 single-string long-form. Confirmation: `kubectlquant -n dev get schedules -o yaml | grep -E 'weekday:\s*(\[|Mon$|Tue$|Wed$|Thu$|Fri$|Sat$|Sun$)'` returns no matches.
- `k8s.io/apiextensions-apiserver/pkg/apiserver/schema.NewStructural` is the canonical structural-schema validator the API server uses at CRD admission. Running the assembled schema through it in a unit test gives byte-equivalent verdict to what the API server will rule.
- The Go-built JSONSchemaProps in `pkg/k8s_connector_schema.go` remains the single source of CRD schema. The two-field shape (`weekday` string + `weekdays` array) lives there.
- Adding `Weekdays []string` to `ScheduleTrigger` does not break the v1 round-trip; the field is additive with `omitempty`.
- The dev StatefulSet's restart loop will pick up the fix on next rollout. No manual intervention to clean the failed CRD update — the API server will accept the new (structurally valid) shape on next attempt.
