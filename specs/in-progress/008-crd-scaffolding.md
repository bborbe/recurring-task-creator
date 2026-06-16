---
status: verifying
approved: "2026-06-15T22:16:49Z"
generating: "2026-06-15T22:20:55Z"
prompted: "2026-06-15T22:39:12Z"
verifying: "2026-06-16T08:35:27Z"
branch: dark-factory/crd-scaffolding
---

## Summary

- Add Go types for the `Schedule` CRD (`apiVersion: task.benjamin-borbe.de/v1`, kind `Schedule`) under `k8s/apis/task.benjamin-borbe.de/v1/{doc.go,register.go,types.go}`.
- Wire controller-gen so `make generate` produces DeepCopy methods (`zz_generated.deepcopy.go`) + a generated client under `k8s/client/{clientset/versioned,informers/externalversions,listers}/...`. **No CRD YAML manifest** — the CRD shape is built as Go code.
- Add `pkg/k8s_connector.go` with `K8sConnector.SetupCustomResourceDefinition(ctx)` that builds the CRD spec via `desiredCRDSpec()` + `scheduleSpecSchema()` (returning `apiextensionsv1.JSONSchemaProps`) and applies it on every binary boot via `apiextensionsv1.CustomResourceDefinitions` get-or-create-or-update. Mirrors `~/Documents/workspaces/agent/task/executor/pkg/k8s_connector.go`.
- Encode validation directly in the Go schema: `vault` slug regex (`Pattern: "^[a-z][a-z0-9-]*$"`), `recurrence` enum (`Enum: []apiextensionsv1.JSON{...}`), `weekday` required-iff-weekly via `XValidations: apiextensionsv1.ValidationRules{...}` CEL rule.
- Add an example fixture under `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` (the `weekly-review` canonical example).
- Tests: assert the schema function output (good shapes accepted, bad shapes rejected by CEL evaluator); round-trip the example fixture through the generated Go types via `sigs.k8s.io/yaml`.
- DOES NOT introduce an informer / `Listen` wiring, does NOT delete `pkg/schedule/inventory.go`, does NOT add any CRs to `k8s/schedules/` (those are Spec B + bootstrap migration).

## Problem

The publisher's inventory is compiled in. Adding/editing/removing a task definition requires a Go PR, release, deploy. The first step toward fixing this is establishing the data structure as a Kubernetes-native type — types + validation + an example. No behavior change yet; this spec lays the foundation Spec B will wire into the tick.

## Goal

After this work, when the recurring-task-creator binary boots into any namespace, it self-installs the `schedules.task.benjamin-borbe.de` CRD (via `SetupCustomResourceDefinition`) using a Go-built schema; `kubectl explain schedule.spec` returns a documented schema; an example `Schedule` parses cleanly via the generated Go types and round-trips; `make generate` regenerates DeepCopy + client code from `k8s/apis/.../types.go` idempotently; validation rejects malformed schedules (missing weekday on weekly, bad vault slug) at the API-server boundary before any controller code sees them.

## Non-goals

- Do NOT add an informer or any controller-runtime wiring — that is Spec B.
- Do NOT delete or modify `pkg/schedule/inventory.go` — that is Spec B.
- Do NOT add any `Schedule` CRs under `k8s/schedules/` beyond the test fixture under `k8s/apis/task.benjamin-borbe.de/v1/testdata/` — those are the bootstrap migration prompt that follows Spec B.
- Do NOT change `task.CreateCommand` or any field on it — the CRD's `template.frontmatter` reuses `lib.TaskFrontmatter`.
- Do NOT introduce `v1alpha1` or any non-v1 version — frozen-from-day-one per design pin.
- Do NOT add a CRD validation admission webhook — OpenAPI + CEL `x-kubernetes-validations` are sufficient.
- Do NOT touch the existing Spec 6 / Spec 7 code paths or behaviors. This spec lands additively.

## Desired Behavior

1. `k8s/apis/task.benjamin-borbe.de/v1/types.go` declares four Go types with the following struct shapes:
    - `Schedule` — embeds `metav1.TypeMeta`, `metav1.ObjectMeta`; fields `Spec ScheduleSpec` and `Status ScheduleStatus`; implements `runtime.Object` via generated DeepCopy.
    - `ScheduleSpec` — fields `Vault string`, `Title string`, `Schedule ScheduleTrigger`, `Template ScheduleTemplate`.
    - `ScheduleTrigger` (the nested `spec.schedule` subnode) — fields `Recurrence string` (one of `Daily|Weekly|Monthly|Quarterly|Yearly`), `Weekday string` (one of `Monday`…`Sunday`, omitempty).
    - `ScheduleTemplate` — fields `Body string`, `Frontmatter lib.TaskFrontmatter` (where `lib` is `github.com/bborbe/agent/lib/command/task`).
    - `ScheduleStatus` — fields `LastTickedAt metav1.Time` (omitempty), `LastPublishedTaskIdentifier string` (omitempty).
    - `ScheduleList` — embeds `metav1.TypeMeta`, `metav1.ListMeta`; field `Items []Schedule`.
2. `k8s/apis/task.benjamin-borbe.de/v1/register.go` registers the types with a scheme builder (group `task.benjamin-borbe.de`, version `v1`); `k8s/apis/task.benjamin-borbe.de/v1/doc.go` carries `+kubebuilder:object:generate=true` and `+groupName=task.benjamin-borbe.de` markers so controller-gen generates DeepCopy.
3. `k8s/apis/task.benjamin-borbe.de/v1/types.go` declares the documented placeholder set as `var Placeholders = []string{"Date", "ISOWeek", "MonthYear", "Quarter", "Year"}` with a GoDoc block explaining each.
4. `make generate` produces `k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go` (idempotent — no diff on a clean second run) and the generated client tree under `k8s/client/{clientset/versioned,informers/externalversions,listers}/...`. **No CRD YAML manifest is generated or committed.**
5. `pkg/k8s_connector.go` defines `K8sConnector` interface + `k8sConnector` impl with `SetupCustomResourceDefinition(ctx)` that calls `apiextensionsv1.CustomResourceDefinitions.Get`-or-`Create`-or-`Update` for `schedules.task.benjamin-borbe.de`. The CRD spec is built by `desiredCRDSpec()` (returning `apiextensionsv1.CustomResourceDefinitionSpec`) which calls `scheduleSpecSchema()` (returning `apiextensionsv1.JSONSchemaProps` for `spec.*`). The package mirrors `~/Documents/workspaces/agent/task/executor/pkg/k8s_connector.go` structure verbatim, varying only group / kind / schema.
6. The Go-built schema in `scheduleSpecSchema()` includes: `spec.vault` (`Type: "string", Pattern: "^[a-z][a-z0-9-]*$"`), `spec.schedule.recurrence` (`Enum: []apiextensionsv1.JSON{{Raw: []byte(...)}, ...}` for the 5 values), and `XValidations: apiextensionsv1.ValidationRules{...}` on `spec.schedule` with the CEL rule `self.recurrence == 'Weekly' ? has(self.weekday) : !has(self.weekday)`.
7. `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` contains a valid `Schedule` matching the design-pin example (`weekly-review`, namespace `personal`, vault `personal`, etc.). `k8s/apis/task.benjamin-borbe.de/v1/example_test.go` reads the file, unmarshals it via `sigs.k8s.io/yaml` into a `Schedule` struct, and asserts every field round-trips (Ginkgo `It`).
8. `pkg/k8s_connector_test.go` covers the CRD installer with a fake `apiextensionsclient.Interface` (via the `CRDClientBuilder` seam): (a) CRD does not exist → Create call observed with the Go-built spec; (b) CRD exists with old spec → Update call observed with new spec; (c) clientset build error → wrapped error returned via `errors.Wrapf`.
9. Validation tests in `pkg/k8s_connector_validation_test.go` use a CEL evaluator (e.g. `k8s.io/apiserver/pkg/cel`) to assert on the Go-built `scheduleSpecSchema()` output: (a) the example fixture validates; (b) recurrence `Foo` rejects; (c) recurrence `Weekly` without `weekday` rejects; (d) recurrence `Monthly` with `weekday` rejects; (e) vault `Bad Vault` rejects.

## Constraints

- The CRD group is `task.benjamin-borbe.de`, version `v1`, kind `Schedule`, plural `schedules`, singular `schedule`, short name `ts`. These names are frozen for the life of v1 — once shipped, renames require a v2 + conversion webhook (out of scope).
- The CRD is `Namespaced`, not `Cluster`-scoped.
- DeepCopy generation goes via `sigs.k8s.io/controller-tools/cmd/controller-gen` (the canonical tool); pin it via `tools.go` like the other build deps.
- Generated client (clientset / informers / listers) goes via `k8s.io/code-generator` (`generate-internal-groups.sh`); pin the version via `tools.go`. Lift the bborbe pattern from agent's existing CRD setup verbatim.
- The Go types use `metav1.TypeMeta` + `metav1.ObjectMeta` + the `runtime.Object` interface (boilerplate exists in agent CRDs).
- **No separate CRD YAML manifest** — the CRD schema is constructed in Go via `apiextensionsv1.JSONSchemaProps`, applied at binary startup. Single source of truth: the Go code.
- The `CRDClientBuilder` seam (`func(*rest.Config) (apiextensionsclient.Interface, error)`) is injected into `NewK8sConnector` so tests can pass a fake clientset (mirrors agent's pattern exactly).
- Project DoD applies (`docs/dod.md`): `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` for every error path introduced (`SetupCustomResourceDefinition` has 4 wrap sites in the agent template); no `context.Background()` in business logic; Ginkgo v2 / Gomega for tests; Counterfeiter for the `K8sConnector` interface fake (`mocks/k8s_connector.go`, fake name `FakeK8sConnector` — matches agent convention).
- All CR YAML is under `k8s/schedules/` (per pinned Decision #2 — bborbe `k8s/` convention).

## Failure Modes

| Trigger | Expected behavior | Recovery | Reversibility |
|---------|-------------------|----------|---------------|
| `kubectl apply -f bad-schedule.yaml` with missing `spec.weekday` and `recurrence: Weekly` | API server rejects with the CEL validation error message naming the rule | Operator fixes the manifest | Reversible (no resource created) |
| Two pods boot simultaneously and both attempt `SetupCustomResourceDefinition` | First `Create` succeeds; second `Create` returns `AlreadyExists`, falls through to `Update` path (same Go-built spec → no-op semantically). Detection: the second pod logs `crd-already-exists: applying update` at `glog.V(2)` (one log line per boot). | None needed | Reversible (k8s API server serialises CRD writes) |
| Existing cluster has the CRD installed by a previous binary version with a different schema | `SetupCustomResourceDefinition` `Get`s then `Update`s with the new Go-built spec; resources are not affected (schema validation runs on next create / update, not on existing rows) | None needed for in-spec resources; resources that drift outside the new schema are flagged on next edit | Reversible (revert binary or hand-edit CRD) |
| `controller-gen` version pinned in `tools.go` falls behind upstream's DeepCopy generation defaults | Generated `zz_generated.deepcopy.go` diff shows up on the next `make generate`; precommit catches it | Bump `controller-gen` in a separate PR with the diff visible | Reversible |
| `example.yaml` parses but a field is misspelled (e.g. `weeday: Saturday`) | Unmarshal silently ignores the unknown field; downstream consumer sees empty `Weekday` | `k8s/apis/task.benjamin-borbe.de/v1/example_test.go` uses strict unmarshal (`yaml.UnmarshalStrict`) and fails on unknown fields | Reversible (test failure surfaces it) |

## Acceptance Criteria

- [ ] `ls k8s/apis/task.benjamin-borbe.de/v1/{doc.go,register.go,types.go}` shows all three files — evidence: files present.
- [ ] `grep -nE 'type Schedule(Spec|Status|List)? struct' k8s/apis/task.benjamin-borbe.de/v1/types.go | wc -l` reports `4` — evidence: line count.
- [ ] `grep -nE '\+groupName=task\.benjamin-borbe\.de' k8s/apis/task.benjamin-borbe.de/v1/doc.go` returns at least one hit — evidence: matched line.
- [ ] `grep -RE 'package versioned|package externalversions|package v1' k8s/client/ | wc -l` returns ≥ 3 — evidence: generated client tree exists.
- [ ] `make generate` produces no diff on a second run (idempotent) — evidence: `git diff --exit-code k8s/` exits 0 after a fresh `make generate`.
- [ ] `grep -n 'SetupCustomResourceDefinition' pkg/k8s_connector.go` returns at least 1 line — smoke indicator only; the load-bearing assertion is the AC below on Create/Update behavior.
- [ ] `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` exists and contains `kind: Schedule` — evidence: file exists and `grep -c 'kind: Schedule' ...` returns 1.
- [ ] `k8s/apis/task.benjamin-borbe.de/v1/example_test.go` round-trips `example.yaml` via `sigs.k8s.io/yaml` and asserts every field — evidence: passing Ginkgo It-block, `go test -v` shows `... is the canonical Schedule example`.
- [ ] `pkg/k8s_connector_test.go` declares three named Ginkgo `It` blocks under `Describe("SetupCustomResourceDefinition", ...)`, each invoking the impl with a fake `apiextensionsclient.Interface` via the `CRDClientBuilder` seam and asserting on the recorded call sequence:
    - `It("creates the CRD when none exists", ...)` — asserts `fakeCRDs.CreateCallCount() == 1` AND the CRD argument satisfies `crd.Spec.Group == "task.benjamin-borbe.de"` AND `crd.Spec.Names.Kind == "Schedule"` AND `crd.Spec.Names.Plural == "schedules"`.
    - `It("updates the CRD when an older spec exists", ...)` — pre-loads the fake with a CRD whose `Spec` differs from `desiredCRDSpec()`; asserts `fakeCRDs.UpdateCallCount() == 1` AND the updated argument's spec equals `desiredCRDSpec()`.
    - `It("wraps an error when the clientset builder fails", ...)` — injects a `CRDClientBuilder` returning `errors.Errorf(ctx, "boom")`; asserts the returned error message contains `"build apiextensions clientset"` (the wrap context).
- [ ] `pkg/k8s_connector_validation_test.go` declares five named Ginkgo `It` blocks under `Describe("scheduleSpecSchema CEL validation", ...)`, each running a CEL evaluator over the schema returned by `scheduleSpecSchema()`:
    - `It("accepts the canonical weekly-review example", ...)` — `eval(example) == nil`.
    - `It("rejects an unknown recurrence value", ...)` — recurrence `"Foo"` → error contains `"recurrence"`.
    - `It("rejects a weekly schedule without weekday", ...)` — error contains `"weekday"`.
    - `It("rejects a non-weekly schedule that sets weekday", ...)` — `recurrence: Monthly, weekday: Saturday` → error contains `"weekday"`.
    - `It("rejects a vault slug that does not match the regex", ...)` — vault `"Bad Vault"` → error contains `"vault"`.
- [ ] `make precommit` exits 0 — evidence: exit code 0.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-crd-scaffolding  # this spec's worktree
make generate
git diff --exit-code k8s/  # should be clean after generate
make precommit
go test -v ./k8s/apis/task.benjamin-borbe.de/v1/...
go test -v ./pkg/...
# Optional kind-cluster smoke (CRD self-install + apply example):
kind create cluster --name schedule-validation || true
KUBECONFIG=$(kind get kubeconfig-path --name=schedule-validation) go run . & sleep 5  # binary self-installs CRD on boot
kubectl --dry-run=server apply -f k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml
```

## Suggested Decomposition

Likely **2 prompts**:

1. **Prompt 1 — types + codegen**: Add `k8s/apis/task.benjamin-borbe.de/v1/{doc.go,register.go,types.go}` with `Schedule`, `ScheduleSpec`, `ScheduleStatus`, `ScheduleList` structs + `Placeholders` constant. Pin `controller-gen` + `code-generator` versions in `tools.go`. Wire `make generate` to produce `zz_generated.deepcopy.go` and the client tree under `k8s/client/`. Add the `testdata/example.yaml` fixture + `example_test.go` round-trip. AC 1, 2, 3, 4, 5, 10, 11, 14 covered.
2. **Prompt 2 — CRD installer + Go-built schema + validation tests**: Add `pkg/k8s_connector.go` (interface + impl + `CRDClientBuilder` seam, mirroring agent's executor pattern). Implement `desiredCRDSpec()` + `scheduleSpecSchema()` returning `apiextensionsv1.JSONSchemaProps` with the vault regex, recurrence enum, and weekday-required-iff-weekly CEL rule. Add Counterfeiter mock for `K8sConnector`. Add `pkg/k8s_connector_test.go` (Create/Update paths via fake clientset) + `pkg/k8s_connector_validation_test.go` (CEL evaluator over the schema). AC 6, 7, 8, 9, 12, 13 covered.

Splitting at the connector boundary keeps prompt 1 strictly type-system + codegen and prompt 2 the runtime contract (installer + validation surface). Prompt 2 depends on the types from prompt 1.

## Do-nothing Option

If we skip this spec: the inventory stays compiled-in forever, the open-source split (task page Phase 2) is blocked because the personal data and the generic code can never separate, and adding a 46th task requires the full Go-PR / release / deploy ceremony. The current pain is low (Spec 6 + 7 cover the operational gaps), so doing nothing remains tolerable — but each new task definition compounds the lock-in.
