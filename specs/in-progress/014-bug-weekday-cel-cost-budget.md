---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-06-25T09:53:48Z"
generating: "2026-06-25T09:53:48Z"
prompted: "2026-06-25T09:53:48Z"
branch: dark-factory/bug-weekday-cel-cost-budget
---

## Summary

- Spec 013 reshaped `weekday` / `weekdays` to a structural-schema-valid two-field form and added a CEL no-duplicates rule on `weekdays`. The rule uses nested `weekdays.map(d, NORM[d]).filter(c2, c2 == c)` — a per-element O(n) traversal inside an O(n) `map`, scored O(n²).
- The Kubernetes CEL cost estimator runs at CRD admission. Because `weekdays` has no `maxItems` upper bound, the estimator assumes N up to `int32 max` and reports the rule's estimated cost as exceeding the per-rule budget "by factor of more than 100x." The API server refuses the CRD update.
- Dev `recurring-task-creator-0` is in `CrashLoopBackOff` (restartCount=18+, rising) since 2026-06-25 ~08:46 UTC. Same user-facing outcome as spec 013's original failure — pod never starts, service is down on dev — but a different API-server admission validator is the rejecting layer.
- Fix is two combined changes: (a) cap `weekdays` with `MaxItems: 7` (the true upper bound — only 7 distinct weekdays exist), and (b) replace the nested `map().filter()` with a bounded pair-wise CEL form whose cost the estimator agrees is in-budget at n=7.
- Spec 013's structural-schema regression-lock stays — useful, but a different validator. This spec adds a CEL cost-budget regression-lock that walks every `x-kubernetes-validations` rule on the assembled CRD through the k8s CEL cost estimator and asserts each is under per-rule budget.

## Problem

The dev cluster's `recurring-task-creator-0` pod is in `CrashLoopBackOff` and has been since 2026-06-25 ~08:46 UTC. Service is down on dev. Root cause: the CEL rule added in spec 013 to detect cross-form duplicates in `spec.schedule.weekdays` uses a nested `map().filter()` pattern. The Kubernetes API server's CEL cost estimator (run at CRD admission) computes the worst-case rule cost using the array's `maxItems` bound; because `weekdays` has no `maxItems`, the estimator assumes the upper limit of `int32 max` and rejects the CRD. The error from the API server, captured verbatim from pod logs:

```
spec.validation.openAPIV3Schema.properties[spec].properties[schedule].x-kubernetes-validations[2].rule: Forbidden: estimated rule cost exceeds budget by factor of more than 100x
```

`SetupCustomResourceDefinition` returns the wrapped error; the pod exits non-zero; StatefulSet restarts it; same failure repeats every restart.

## Reproduction

```bash
cd ~/Documents/workspaces/recurring-task-creator-weekday-list
git log --oneline -1   # HEAD: ac982b8 (post-spec-013 merge)

# 1. Build and apply the controller's CRD-install path
kubectlquant -n dev rollout restart statefulset/recurring-task-creator

# 2. Observe the pod fails to start
kubectlquant -n dev get pod recurring-task-creator-0
# STATUS: CrashLoopBackOff, restartCount rising

# 3. Inspect the failure
kubectlquant -n dev logs recurring-task-creator-0 --previous | grep -A1 'exceeds budget'
```

Observed evidence (verbatim, copy from a live `kubectl logs --previous`):

```
application failed: setup CRD failed: update CRD schedules.task.benjamin-borbe.de: CustomResourceDefinition.apiextensions.k8s.io "schedules.task.benjamin-borbe.de" is invalid: spec.validation.openAPIV3Schema.properties[spec].properties[schedule].x-kubernetes-validations[2].rule: Forbidden: estimated rule cost exceeds budget by factor of more than 100x
```

The same error appears every restart; the pod never reaches the leader-election or informer-sync phases.

Repository version: `recurring-task-creator` HEAD `ac982b8`, branch `master` post-merge of spec 013.

## Expected vs Actual

**Expected** (per spec 013's Goal and per `docs/architecture.md` "the controller installs its own CRDs on startup"): the controller installs / updates the `schedules.task.benjamin-borbe.de` CRD without error; the pod reaches Running.

**Actual**: API server rejects the CRD update with the CEL cost-budget violation above. `SetupCustomResourceDefinition` returns the wrapped error; the pod exits non-zero; StatefulSet restarts it; restartCount rises (18+ as of filing). Service is down on dev.

## Why this is a bug

1. Kubernetes CRD admission runs every `x-kubernetes-validations` rule through a CEL cost estimator. The estimator computes worst-case cost using the array's `maxItems`. With no `maxItems` on `weekdays`, the estimator assumes `int32 max` and any non-trivial nested traversal blows the per-rule budget. See `k8s.io/apiserver/pkg/cel` cost helpers and `k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel` for the runtime validator.
2. The CEL cost-budget validator is a **different** validator from the structural-schema validator that spec 013's regression-lock test exercises. `apiserver/schema.NewStructural(...)` checks structural shape; CEL cost is computed by `cel.NewValidator(...)` (or the cost helpers under `apiserver/pkg/cel`). Spec 013's lock passes the new schema but the API server still rejects it.
3. The duplicate-detection user-facing behavior (reject `[Mon, Monday]`) is still required — operators must see the error at apply time, not silently dedup. The fix replaces the implementation form, not the semantics.
4. The fix MUST add a CEL-cost regression-lock test so any future CEL rule whose cost exceeds the per-rule budget is caught in CI before merge.

## Goal

After this work:

- `recurring-task-creator-0` on dev reaches and stays in `Running`. `SetupCustomResourceDefinition` succeeds.
- `weekdays` carries `MinItems: 1, MaxItems: 7` on the CRD schema. The MaxItems bound matches the true domain (only 7 distinct weekdays exist after dedup).
- The cross-form duplicate-detection CEL rule on `weekdays` is rewritten to a form whose estimated cost the k8s CEL cost estimator agrees is under the per-rule budget at the new `maxItems: 7` cap. The user-facing behavior is unchanged: `[Mon, Monday]`, `[Tue, Tue]`, `[Wednesday, Wednesday]` continue to be rejected at admission.
- A regression-lock unit test walks every `x-kubernetes-validations[i].rule` on `spec.schedule` through the k8s CEL cost estimator and asserts each rule's estimated cost is under the per-rule budget. The test fails on pre-fix HEAD `ac982b8` with a message naming "exceeds budget" / "cost" for the dup rule, and passes after the fix.
- Spec 013's structural-schema regression-lock test stays untouched — it still validates structural shape, just doesn't catch this bug class.
- Spec 013's two-field shape, spec 012's `WeekdayList` adapter, the day-set matcher, period-token rendering, UUID5 derivation, normalization map, and the 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` are unchanged on the inside.

## Desired Behavior

1. `Schedule.spec.schedule.weekdays` carries `MinItems: 1` (unchanged from spec 013) AND `MaxItems: 7` (new). Lists with >7 items are rejected by OpenAPI validation before any CEL rule runs.
2. The cross-form no-duplicates CEL rule on `weekdays` is rewritten to a form whose estimated cost the k8s CEL cost estimator agrees is under the per-rule budget at `maxItems: 7`. Two acceptable shapes (agent picks one at impl time based on what the CEL env supports):
   - **Index pair** (preferred): `weekdays.all(i, weekdays.all(j, i >= j || NORM[weekdays[i]] != NORM[weekdays[j]]))` — explicit O(n²) at known-bounded n=7.
   - **Set-size**: `size(weekdays.map(d, NORM[d]).toSet()) == size(weekdays)` — requires `toSet()` in the CEL env; verify before choosing.
3. User-facing behavior is unchanged: `[Mon, Monday]`, `[Monday, Mon]`, `[Tue, Tue]`, `[Wednesday, Wednesday]` are rejected at admission; `[Mon, Wed, Fri]` and `[Mon, Tue, Wednesday, Thu, Fri]` are accepted.
4. A new regression-lock unit test (Ginkgo `It` in `pkg/k8s_connector_validation_test.go` or a sibling `pkg/k8s_connector_cost_test.go`) builds the full CRD's assembled OpenAPI schema, walks every `x-kubernetes-validations[i].rule` on `spec.schedule`, runs each through the k8s CEL cost estimator (import path resolved at impl time — likely `k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel.NewValidator` or `k8s.io/apiserver/pkg/cel` cost helpers), and asserts each rule's estimated cost is under the per-rule budget.
5. The single-string `weekday: Monday` path is unchanged. Spec 013's two-field shape stays.
6. `docs/architecture.md` notes the 7-day max on `weekdays` (one-line addition to the line describing the field).
7. CHANGELOG gains a `fix:` entry under `## Unreleased`.

## Constraints

- CRD group, version, kind, plural, singular, short name unchanged.
- `weekdays` enum is unchanged from spec 013 (14-element long+short set).
- The cross-form duplicate-detection user-facing behavior is unchanged. Operators MUST still see `[Mon, Monday]` rejected at admission, not silently deduped. Only the rule's CEL implementation form changes.
- Spec 012's internals (`WeekdayList`, normalization map, day-set matcher, period-token rendering, UUID5 derivation) untouched. Spec 013's two-field wire shape and structural-schema regression-lock test untouched.
- The 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` must continue to pass byte-identically.
- Project DoD applies (`docs/dod.md`): `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` on every error path; Ginkgo v2 / Gomega; no `time.Now()` / `context.Background()` in business logic.
- `make precommit` exits 0 from repo root after every prompt lands.
- The fix lands in `master`, merges to `dev`, and the dev pod returns to Running before the spec is marked `completed`.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility |
|---------|-------------------|-----------|----------|---------------|
| Operator applies `weekdays: [Mon, Tue, Wed, Thu, Fri, Sat, Sun, Mon]` (8 items) | API server rejects via OpenAPI `MaxItems: 7` before CEL runs. | `kubectl apply` exits non-zero with the OpenAPI message naming `weekdays` and `maxItems`. | Operator drops to ≤7 items. | Reversible. |
| Operator applies `weekdays: [Mon, Monday]` (cross-form dup) | API server rejects via the rewritten CEL no-duplicates rule. | `kubectl apply` exits non-zero with the CEL message. | Operator removes the duplicate. | Reversible. |
| Operator applies `weekdays: [Tue, Tue]` (same-form dup) | API server rejects via the same rule. | `kubectl apply` exits non-zero. | Operator removes the duplicate. | Reversible. |
| Operator applies `weekdays: [Mon, Wed, Fri]` (no dups, ≤7 items) | API server accepts; CR persists; publisher emits 3 task files per ISO week. | `kubectl get schedules` lists the CR; publisher logs show 3 emissions. | None — invariant. | N/A. |
| CRD apply during pod start | `SetupCustomResourceDefinition` succeeds; pod reaches Running; informer cache fills. | `kubectlquant -n dev get pod recurring-task-creator-0` shows `Running`. | None — invariant. | N/A. |
| Future CEL rule on an unbounded array reintroduces an out-of-budget cost | The CEL cost-budget round-trip unit test fails at `make precommit`. CI blocks the merge. | `go test ./pkg/...` prints FAIL naming the rule's index and "exceeds budget" / "cost"; `make precommit` exits non-zero. | Author bounds the array with `maxItems` or rewrites the rule. | Reversible — defensive lock. |
| Single-string `weekday: Monday` CR (unchanged path) | Adapter produces `WeekdayList{time.Monday}`; period token, UUID5, title, body byte-identical. | The 21-entry UUID5 stability block passes; vault file identifier unchanged. | None — invariant. | N/A. |

## Acceptance Criteria

- [ ] **CEL cost-budget regression-lock test exists, FAILs on pre-fix HEAD, PASSes after fix.** A new Ginkgo `It` in `pkg/k8s_connector_validation_test.go` (or a new `pkg/k8s_connector_cost_test.go`) builds the full CRD's assembled OpenAPI schema, walks every `x-kubernetes-validations[i].rule` on `spec.schedule`, runs each through the k8s CEL cost estimator (canonical import resolved at impl time — likely `k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel.NewValidator` or `k8s.io/apiserver/pkg/cel` cost helpers), and asserts each rule's estimated cost is under the per-rule budget. Evidence: `go test -v -run 'CostBudget|CELCost' ./pkg/...` prints PASS post-fix. **Pre-fix FAIL anchor** (capture before any source edit):
  ```bash
  git stash push -u -- pkg/k8s_connector_schema.go && \
    git checkout ac982b8 -- pkg/k8s_connector_schema.go && \
    go test -v -run 'CostBudget|CELCost' ./pkg/... ; \
    rc=$? ; \
    git checkout HEAD -- pkg/k8s_connector_schema.go && \
    git stash pop ; \
    test $rc -ne 0
  ```
  Must print a FAIL line naming "exceeds budget" (or "cost") and the dup rule's index. Capture the failure text in the completion report's `summary`. If the pre-fix run does NOT FAIL, STOP and report `status: failed` — the regression-lock invariant is broken.
- [ ] **`weekdays` schema gains `MinItems: 1, MaxItems: 7`.** Evidence: `grep -nE 'MaxItems.*7' pkg/k8s_connector_schema.go` returns line ≥1.
- [ ] **Cross-form duplicates still rejected at admission.** Evidence: the existing accept/reject DescribeTable in `pkg/k8s_connector_validation_test.go` is **extended in this spec** with rows asserting rejection for each of `weekdays: [Mon, Monday]`, `[Monday, Mon]`, `[Tue, Tue]`, `[Wednesday, Wednesday]`; all four rows PASS. Acceptance rows for `[Mon, Wed, Fri]` and `[Mon, Tue, Wednesday, Thu, Fri]` PASS. Anti-laziness check: a CEL rule replaced with the literal `true` would PASS the existing thin coverage but FAIL all four new rejection rows — so adding them locks the dup-detection invariant to this spec, not inherited from prior tests.
- [ ] **>7 items rejected by `MaxItems` before CEL.** Evidence: new DescribeTable row in `pkg/k8s_connector_validation_test.go` asserts error for `weekdays: [Mon, Tue, Wed, Thu, Fri, Sat, Sun, Mon]` (8 items); the rejection message names `maxItems` (i.e. the OpenAPI validator fires before the CEL rule, proving the bound is engaged); PASSes.
- [ ] **Single-string `weekday: Monday` regression unchanged.** Evidence: existing validation test for `recurrence: Weekday, weekday: Monday` PASSes; the 21-entry UUID5 stability block in `pkg/publisher/publisher_test.go` PASSes — `go test -v -run 'UUID5 stability' ./pkg/publisher/...` prints PASS.
- [ ] **Spec 013 structural-schema regression-lock test still PASSes.** Evidence: `go test -v -run 'Structural' ./pkg/...` prints PASS — the structural-shape lock is preserved (the change is additive: `MaxItems` + CEL rule body).
- [ ] **`docs/architecture.md` notes the 7-day max on `weekdays`.** Evidence: `grep -nE 'weekdays.*(7|maxItems|max items)' docs/architecture.md` returns line ≥1.
- [ ] **CHANGELOG entry under `## Unreleased`.** Evidence: `awk '/^## Unreleased/{u=1;next} /^## /{u=0} u' CHANGELOG.md | grep -nE 'fix:.*(CEL|cost|weekdays|MaxItems)'` returns ≥1 line — restricts the grep to bullets between `## Unreleased` and the next `## ` header, ensuring the new bullet lives in the unreleased section, not a stale released one.
- [ ] **`make precommit` exits 0** from repo root. Evidence: exit code 0.
- [ ] **Post-Deploy (Rung-2):** **CRD installs on dev without API-server rejection.** Evidence: `kubectlquant -n dev rollout restart statefulset/recurring-task-creator` followed by `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.phase}'` returns `Running` within 60 seconds AND `kubectlquant -n dev logs recurring-task-creator-0 | grep -c 'exceeds budget'` returns `0` AND `kubectlquant -n dev logs recurring-task-creator-0 | grep -c 'must be empty to be structural'` returns `0`.
  - `deploy_check:` `kubectlquant -n dev get statefulset/recurring-task-creator -o jsonpath='{.spec.template.spec.containers[0].image}'` returns the post-fix image digest.
  - `deploy_target:` dev cluster, namespace `dev`, statefulset `recurring-task-creator`.
- [ ] **Post-Deploy (Rung-2):** **Bug reproduction no longer reproduces.** Evidence: replay the Reproduction section steps against the post-fix HEAD on dev; pod stays `Running` ≥5 minutes; `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.containerStatuses[0].restartCount}'` does not increase over a 5-minute window after rollout completes; `kubectlquant -n dev logs recurring-task-creator-0 | grep -cE 'exceeds budget|must be empty to be structural'` returns `0`.
  - `deploy_check:` `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.containerStatuses[0].restartCount}'` is stable over 5 minutes.
  - `deploy_target:` dev cluster, namespace `dev`, pod `recurring-task-creator-0`.

## Workaround

None viable. The CRD failure happens at API-server admission on every pod start; there is no operator-side mitigation that avoids the CRD update. Disabling `SetupCustomResourceDefinition` is not exposed via config. The fix must land.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-weekday-list
make precommit
go test -v -run 'CostBudget|CELCost|Structural|Weekdays' ./pkg/...

# Post-deploy on dev:
kubectlquant -n dev rollout restart statefulset/recurring-task-creator
kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.phase}'
kubectlquant -n dev logs recurring-task-creator-0 | grep -cE 'exceeds budget|must be empty to be structural'
```

Expected:
- `make precommit` exits 0.
- The targeted `go test` invocation prints PASS for every matched spec, including the new CEL cost-budget round-trip lock AND the spec-013 structural-schema lock.
- `get pod` returns `Running` within 60 seconds of rollout restart.
- The `grep -cE` returns `0`.

### Bug-spec verification (mandatory)

Per `docs/bug-workflow.md`, the Reproduction steps in this spec are replayed against the post-fix deployed pod. The bug is considered fixed only when:

1. `kubectlquant -n dev get pod recurring-task-creator-0 -o jsonpath='{.status.phase}'` returns `Running`.
2. `kubectlquant -n dev logs recurring-task-creator-0 | grep -cE 'exceeds budget|must be empty to be structural'` returns `0`.
3. The pod stays Running for ≥5 minutes without restart (restartCount stable).
4. Applying a fresh `weekdays: [Mon, Wed, Fri]` CR succeeds; `kubectlquant -n dev get schedules` lists it.
5. Applying `weekdays: [Mon, Monday]` is rejected at admission with a CEL message; applying `weekdays: [Mon, Tue, Wed, Thu, Fri, Sat, Sun, Mon]` is rejected at admission with a `maxItems` message.

Tests-passing alone does NOT satisfy verification — the dev runtime must demonstrate the new CRD installs and admission still rejects duplicates and >7 items.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | CRD schema + CEL rule + cost-budget regression-lock. Add `MaxItems: 7` (alongside the existing `MinItems: 1`) to `weekdays` in `pkg/k8s_connector_schema.go`. Rewrite the cross-form no-duplicates CEL rule from the nested `map().filter()` form to a pair-wise form (e.g. `weekdays.all(i, weekdays.all(j, i >= j || NORM[weekdays[i]] != NORM[weekdays[j]]))`). Add the CEL cost-budget round-trip Ginkgo `It` that walks every rule on `spec.schedule` through the k8s CEL cost estimator and asserts each is under per-rule budget. Extend the validation DescribeTable with the 8-item MaxItems-rejection row and confirm all existing accept/reject rows still pass. Update `docs/architecture.md` and add the CHANGELOG `fix:` entry. Verify on dev — pod returns to Running, replay the bug reproduction, mark verifying. | 1, 2, 3, 4, 5, 6, 7 | all |  — |

Rationale: single prompt — the change is one cohesive surface (CRD schema bound + one CEL rule rewrite + one new regression-lock test + doc/changelog), all in the same package, no cross-layer seam. Splitting would create a window where one half is merged and admission is still broken.

## Related

- **Predecessor (introduced the cost-budget-violating CEL rule):** `specs/in-progress/013-bug-weekday-oneof-not-structural.md`
- **Predecessor (introduced the list-of-weekdays user-facing surface):** `specs/completed/012-weekday-list-and-short-forms.md`
- **Parent goal (Personal Obsidian vault, informational):** `[[Migrate vault-cli Recurring Tasks to recurring-task-creator]]`

## Assumptions

- The k8s CEL cost estimator's per-rule budget is the same value the API server enforces at CRD admission; running the assembled CRD's rules through the estimator in a unit test gives a byte-equivalent verdict to what the API server will rule.
- The canonical import for invoking the estimator is reachable from the project's existing `k8s.io/apiextensions-apiserver` / `k8s.io/apiserver` dependency closure. Agent resolves the exact import path at impl time; the test build must compile against the project's current go.mod without adding new top-level deps beyond what is already transitively available.
- 7 is the true upper bound on `weekdays` after dedup — there are exactly 7 days in a week, and the cross-form dedup CEL rule guarantees one CR cannot list more than 7 distinct days. `MaxItems: 7` is correct, not arbitrary.
- The pair-wise CEL form is supported by the CEL environment the apiextensions-apiserver uses. If a chosen form turns out to be unsupported at impl time, the agent falls back to one of the other listed shapes; the AC is that the cost-budget test passes, not which form is used.
- No in-cluster `Schedule` CR currently lists more than 7 weekdays. Confirmation: `kubectlquant -n dev get schedules -o yaml` shows no `weekdays` lists at all today (the spec-013 CRD never successfully installed).
