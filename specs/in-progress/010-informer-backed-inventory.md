---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-16T18:55:40Z"
generating: "2026-06-16T18:58:57Z"
prompted: "2026-06-16T19:13:06Z"
verifying: "2026-06-16T19:50:47Z"
branch: dark-factory/informer-backed-inventory
---

## Summary

- Replace the hard-coded 45-entry static slice in `pkg/schedule/inventory.go` with a runtime store backed by a Kubernetes informer cache over the `Schedule` CRD that Spec 008 shipped. The tick and the `/trigger` handler read from the store; the static slice is deleted.
- Single-namespace scope: the binary watches `Schedule` resources only in its own pod-namespace (read from env), matching the existing one-StatefulSet-per-namespace deploy.
- Wire `K8sConnector.SetupCustomResourceDefinition(ctx)` (already shipped by Spec 008, untouched in `pkg/k8s_connector.go`) into both `application.Run` in `main.go` AND `cmd/run-once/main.go`. The CRD MUST be installed and the informer cache MUST be synced before the first tick fires.
- The on-disk CRD bootstrap (the 45 YAML files matching today's static inventory) is OUT OF SCOPE — it lands in a follow-up spec. Until that follow-up ships, the binary publishes zero tasks per tick (no `Schedule` CRs exist in-cluster yet). This is intentional and acceptable for a brief deploy window because the production cutover sequence is: ship this wiring → ship the YAML bootstrap → cut over.
- Net effect: the connector and CRD types Spec 008 shipped finally do something at runtime, and editing the recurring-task inventory becomes a `kubectl apply` instead of a Go PR + release + deploy.

## Problem

Spec 008 added the `Schedule` CRD types, the Go-built schema, a generated client, and the `K8sConnector.SetupCustomResourceDefinition` installer — but explicitly deferred the runtime wiring. The binary still reads its task inventory from a 45-entry compiled-in slice (`pkg/schedule/inventory.go`); the `K8sConnector` shipped in `pkg/k8s_connector.go` is never invoked from `main.go` or `cmd/run-once/main.go`; and the generated informer / lister tree under `k8s/client/` is dead code. Editing the recurring-task inventory still requires a Go PR, a release, and a deploy. Spec 008's foundation is in place; this spec is the wiring change that activates it.

## Goal

After this work, the running binary self-installs the CRD on boot, then maintains an informer-backed cache of every `Schedule` resource in its pod-namespace, and serves every read of the recurring-task inventory (hourly tick + `/trigger?date=` replay) from that cache via a single typed store interface. The static slice `pkg/schedule/inventory.go` no longer exists. `main.go` and `cmd/run-once/main.go` go through the same wiring code path — the tick loop is rebuilt only once, in `application.Run`, and both entry points exercise it.

## Non-goals

- Do NOT migrate the 45 entries to `Schedule` CRs in this spec — that is the follow-up bootstrap spec.
- Do NOT introduce a multi-namespace watch — single-namespace scope (the pod's namespace) is invariant for v1; if a future deploy demands a cluster-wide watch, that's a separate spec.
- Do NOT add a status-writeback loop on `Schedule.Status.LastTickedAt` / `LastPublishedTaskIdentifier`. Spec 008 declared status writes deferred (`+genclient:noStatus`); this spec respects that boundary. Tick observability stays in `glog` + Prometheus metrics.
- Do NOT add a leader-election lease. The binary runs as a single-replica StatefulSet (per existing k8s manifests); two pods will never read the same Kafka topic at the same wall-clock tick.
- Do NOT add a config knob to toggle between "informer-backed" and "static-slice" mode. The static slice is deleted; there is no fallback path.
- Do NOT touch `pkg/schedule/{recurrence,task_definition,date,tasks_for_date}.go`. Those are pure types and utilities still consumed by the publisher and tick.
- Do NOT change the `Schedule` CRD schema, the CRD names, the `RecurrenceKind` enum, or the `TaskDefinition` struct shape. The adapter bridges the two; neither side moves.
- Do NOT change `task.CreateCommand`, the UUID5 derivation, the period-token format, or anything downstream of the tick. The cache change is invisible to the publisher.
- Do NOT change the `/trigger?date=` HTTP request or response shape (Spec 5).

## Desired Behavior

1. A new package (informally: the store) exposes a typed interface that returns the current set of `TaskDefinition`s the publisher and tick consume. The interface is the only seam the rest of the codebase sees — neither `pkg/tick` nor `pkg/handler/trigger` imports `k8s.io/client-go` or the generated client tree. Consumers receive the store by constructor injection through the factory.
2. The store is backed by a `cache.SharedIndexInformer` (or the bborbe wrapper equivalent, per the project's CRD-controller guide) over the generated `Schedule` clientset, scoped to the binary's pod-namespace.
3. At boot, `application.Run` MUST call `K8sConnector.SetupCustomResourceDefinition(ctx)` and wait for it to return nil BEFORE constructing the informer factory or starting the tick loop. The CRD install is sequenced before the informer; otherwise the informer's first List/Watch fails with `NoKindMatchFor`. Same sequencing applies in `cmd/run-once/main.go` — both entry points reach the wiring through one shared code path.
4. After the informer is started, the boot path waits for `cache.WaitForCacheSync` with a bounded timeout of 30 seconds (mirroring `crdSetupTimeout` shipped in Spec 008) before declaring the binary ready. If the sync deadline expires, boot fails with a wrapped error; the pod CrashLoopBackOffs and the operator sees the timeout in pod logs.
5. The store's `List` returns a `[]schedule.TaskDefinition` adapted from the cached `Schedule` resources. The adapter maps each cached `v1.Schedule` to one `TaskDefinition`:
    - `Slug` = `metadata.name`
    - `TitleTemplate` = `spec.title`
    - `BodyTemplate` = `spec.template.body`
    - `Recurrence` = lowercase of `spec.schedule.recurrence` (`Daily`→`daily`, `Weekly`→`weekly`, `Weekday`→`weekday`, `Monthly`→`monthly`, `Quarterly`→`quarterly`, `Yearly`→`yearly`)
    - `Weekday` = `time.Weekday` parsed from `spec.schedule.weekday` when `Recurrence == "Weekday"`; zero-value otherwise
6. Tick and `/trigger` are rewired so each tick or trigger call queries the store at the moment of firing — never reads a snapshot captured at boot. A `Schedule` CR added at 14:32 is visible to the 15:00 tick once the informer's watch has delivered it; the controller does not need to be restarted.
7. The static slice file `pkg/schedule/inventory.go` is deleted. The `schedule.Inventory()` accessor is deleted along with it. Every consumer that previously called `schedule.Inventory()` is rewired to the store through the factory (no consumer imports the store package directly except the factory). Tests that previously called `schedule.Inventory()` are rewritten to inject a fixed test slice through the same store interface. Prompt-creator note: a literal `schedule.Inventory()` grep won't catch import-aliased calls (e.g. `sched "…/schedule"; sched.Inventory()`); the load-bearing check that no orphaned call survives is `make precommit`'s build step + the `ls` AC asserting the file is absent.
8. The factory exposes one `CreateScheduleStore(...)` constructor and rewires `CreateTick`, `CreateTriggerHandler`, and `CreateTickLoop` to receive the store (or a `List` callable) instead of the static slice. `main.go` and `cmd/run-once/main.go` each build the store exactly once and pass it through.

## Constraints

- The CRD name, group, version, kind, plural, singular, short name, and schema are frozen by Spec 008 — this spec does not touch them.
- The `RecurrenceKind` enum (lowercase: `daily`, `weekly`, `weekday`, `monthly`, `quarterly`, `yearly`) is frozen by Spec 009; the adapter normalises CRD-side capitalised values to it.
- The `TaskDefinition` struct shape is frozen; the adapter populates the exact same fields the static slice did.
- Project DoD applies (`docs/dod.md`): `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` for every error path (never `fmt.Errorf`); no `time.Now()` / `time.Time` in business logic — inject `libtime.CurrentDateTimeGetter`; no `context.Background()` in business logic; Ginkgo v2 / Gomega for tests; Counterfeiter v6 for the store mock under `mocks/`.
- Every hand-written Go file carries the project's BSD-style header (Copyright 2026 Benjamin Borbe).
- GoDoc on every exported identifier introduced by this spec.
- `make precommit` exits 0 from the repo root after every prompt lands.
- All existing tests must continue to pass (the publisher, the period-token tests from Specs 6/7/8, the `recurrence` invariants from Spec 9, the trigger handler's response-shape contract from Spec 5).
- The binary reads its watch-namespace from an environment variable (the same standard env the existing handlers already consume — confirm at impl time which one the existing healthz / StatefulSet template injects; do NOT invent a new env name).
- Coding guides apply: `go-architecture-patterns.md`, `go-testing-guide.md`, `go-mocking-guide.md`, `go-error-wrapping-guide.md`, `go-makefile-commands.md`. If the project has a `go-kubernetes-crd-controller-guide.md` in `~/Documents/workspaces/coding-guidelines/`, follow its informer / event-handler patterns (including any bborbe wrapper used elsewhere in the bborbe ecosystem); otherwise raw `client-go` `cache.SharedIndexInformer` + `cache.ResourceEventHandlerFuncs` is acceptable.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility |
|---------|-------------------|-----------|----------|---------------|
| `K8sConnector.SetupCustomResourceDefinition` returns an error at boot (API server unreachable, RBAC missing) | `application.Run` returns the wrapped error; the binary exits non-zero. The informer is NOT started. The tick loop is NOT started. | Pod CrashLoopBackOff with the wrap context (`build apiextensions clientset` / `create CRD` / `update CRD`) visible in pod logs. | Operator fixes RBAC / waits for API server; pod restarts and retries. | Reversible (pod restart is idempotent — `SetupCustomResourceDefinition` is get-or-create-or-update). |
| `cache.WaitForCacheSync` does not return true within 30s | `application.Run` returns a wrapped error naming the cache-sync timeout. Binary exits non-zero; no tick fires. | Pod CrashLoopBackOff with the timeout error in logs. | Operator inspects API server, RBAC, and the binary's NAMESPACE env. Restart retries. | Reversible. |
| Informer cache is empty (no `Schedule` CRs exist yet — the post-deploy, pre-bootstrap window) | Tick fires, store `List` returns empty slice, zero `task.CreateCommand` messages published, tick metric records 0 publishes for that hour. No error returned. | Prometheus metric for publishes per tick reads 0; tick log line at `glog.V(2)` shows `0 entries`. | Operator applies the bootstrap `Schedule` YAMLs (follow-up spec). | Reversible — next tick after CRs land publishes the full set. |
| A `Schedule` CR's `spec.schedule.recurrence` value is outside the closed enum (defensive — CRD CEL validation should have rejected it at apply time) | Adapter returns an error for that single entry, the entry is dropped from the store's `List` output for the current cycle, and an error is logged with the slug. The other entries in the cache continue to flow through. The tick does NOT abort. | `glog` error line naming the offending slug; Prometheus error counter incremented. | Operator fixes the CR; informer watch delivers the update; next tick uses the fixed value. | Reversible. |
| Two `Schedule` CRs in the same namespace share the same `metadata.name` | Cannot occur — Kubernetes enforces name uniqueness per namespace. | API server rejects the second `kubectl apply`. | N/A — invariant. | N/A. |
| A `Schedule` CR is deleted mid-tick | The deletion is delivered via watch to the cache; the next `List` reflects the delete. If the deletion lands between the tick's snapshot of the store and the publisher's Publish call, the published `task.CreateCommand` for that slug for that tick still goes through (snapshot semantics) — this is the same idempotency story as the static-slice version: the next tick simply omits the deleted slug. | Tick log diff between cycles. | Operator-intended deletion; no action needed. | Reversible (re-create the CR). |
| Informer watch connection drops mid-run | `client-go`'s default reconnect loop re-establishes the watch and resyncs from the last resource version. No manual intervention; no tick failure. | `client-go`-internal log lines at higher verbosity AND the existing `recurring_task_creator_last_tick_seconds` Prometheus gauge stops advancing — alert if it stays flat for > 2 expected ticks. | Automatic. | Reversible (transient). |
| The binary boots in a namespace where the `NAMESPACE` env is empty or missing | `application.Run` returns a wrapped configuration error before even attempting to start the informer. Binary exits non-zero. | Pod CrashLoopBackOff with the config error in logs. | Operator fixes the StatefulSet template. | Reversible. |

## Security / Abuse Cases

- The binary already runs with a ServiceAccount that has `get` / `create` / `update` on CRDs cluster-wide (per Spec 008's `SetupCustomResourceDefinition`). This spec additionally requires `list` / `watch` on `schedules.task.benjamin-borbe.de` in the pod's own namespace. The RBAC change is in the StatefulSet's bundled RoleBinding (cluster-internal; no external surface).
- The informer caches an entire namespace's `Schedule` resources in memory. Forty-five entries × small bodies ≈ kilobytes of RSS. Even a 10x growth is harmless. No DoS surface from CR volume.
- A malicious operator who can `kubectl apply` a `Schedule` CR can cause the binary to publish a Kafka `task.CreateCommand` with attacker-controlled `title` and `body`. This is by design: the trust boundary is the cluster's RBAC, not this binary. The CRD's CEL validation already constrains `vault` (slug regex) and `recurrence` (closed enum). Body is free-form by design (matches today's static-inventory body).
- The watch-namespace comes from a pod-injected env (StatefulSet template) — not from user input. No path-traversal or injection surface.
- The `/trigger?date=` handler is unchanged; its request/response shape, lack of authentication (cluster-internal-only), and idempotency are all preserved.

## Acceptance Criteria

- [ ] A new package (informally referred to as the store) exists with: a `ScheduleStore` interface declaring `List(ctx) ([]schedule.TaskDefinition, error)`, a constructor that takes the generated `Schedule` informer (or its lister) and a clock, and a Counterfeiter-generated fake under `mocks/` named per the project's convention — evidence: `ls` on the new package directory shows the implementation file, the test file, and the suite file; `ls mocks/` shows the new fake file; `grep -n "type ScheduleStore interface" <pkg>/<file>.go` returns line ≥1.
- [ ] `pkg/schedule/inventory.go` is deleted — evidence: `ls pkg/schedule/inventory.go` exits non-zero (`No such file or directory`).
- [ ] `grep -RnE 'schedule\.Inventory\(\)' --include='*.go' .` returns no matches in production code (test files inside the store's own package may still reference it during the migration — but only as deleted strings; the grep checks the live tree) — evidence: empty grep output (exit code 1).
- [ ] The factory's `CreateTick` and `CreateTriggerHandler` accept a `ScheduleStore` (or a `List` callable) instead of `[]schedule.TaskDefinition` — evidence: `grep -nE 'ScheduleStore|store\.List' pkg/factory/factory.go` returns line ≥1; `make precommit` exits 0 (signal that all call-sites updated).
- [ ] `application.Run` in `main.go` calls `connector.SetupCustomResourceDefinition(ctx)` and only proceeds to construct the informer and the tick AFTER that call returns nil — evidence: a unit/integration test of the boot sequence asserts the recorded call order (CRD install → informer factory → cache sync → tick start). For example, a Ginkgo `It` that injects a fake connector recording invocation timestamps and asserts the connector's `SetupCustomResourceDefinitionCallCount() == 1` before any cache-sync wait fires.
- [ ] `cmd/run-once/main.go` reaches the same wiring code path as `main.go` — evidence: both files call a shared constructor (e.g. `application.Run` is moved into a shared package, or both binaries reuse a single `factory.CreateTickLoop`-shaped helper that takes the store + connector). `grep -n "SetupCustomResourceDefinition\|CreateScheduleStore" cmd/run-once/main.go` returns line ≥1.
- [ ] A unit test in the store's package, given a fake informer / lister pre-loaded with exactly one `v1.Schedule` that exactly matches a known-good `Schedule` example (e.g. the `testdata/example.yaml` Spec 008 ships), returns a one-element slice whose single element has the expected Slug / TitleTemplate / BodyTemplate / Recurrence / Weekday — evidence: passing Ginkgo `It` block; `go test -v ./<store-pkg>/...` prints the spec name.
- [ ] A Ginkgo `DescribeTable` in the store's package covers the adapter for all six `RecurrenceKind` values (`Daily`→`daily`, `Weekly`→`weekly`, `Weekday`→`weekday`, `Monthly`→`monthly`, `Quarterly`→`quarterly`, `Yearly`→`yearly`) and asserts the resulting `TaskDefinition.Recurrence` equals the expected lowercase value — evidence: passing test name(s) printed by `go test -v ./<store-pkg>/...`.
- [ ] A Ginkgo `It` in the store's package asserts that `Recurrence=Weekday` with `Weekday=Saturday` maps to a `TaskDefinition` whose `Weekday == time.Saturday` — evidence: passing test name printed by `go test -v ./<store-pkg>/...`.
- [ ] An integration-flavoured test (in the store's package or a sibling) spins up the project-pinned fake `Schedule` clientset, injects two `Schedule` resources, drives a real `cache.SharedIndexInformer` through `WaitForCacheSync`, and asserts the store's `List(ctx)` returns both resources adapted to `TaskDefinition`s — evidence: passing Ginkgo It; `go test -v ./<store-pkg>/...`.
- [ ] An adapter-level test asserts that a `v1.Schedule` whose `spec.schedule.recurrence` is outside the closed enum produces a `Wrap`-wrapped error from the adapter and is dropped from the store's `List` output (the test asserts both the error log AND that the other valid entries still appear) — evidence: passing Ginkgo It with the wrap-context string asserted by `Equal`/`ContainSubstring`.
- [ ] Existing tests still pass: `pkg/publisher`, `pkg/tick`, `pkg/handler` (trigger + healthz), `pkg/schedule` (`tasks_for_date`, `canonical_slugs`, `recurrence`, `date`, `no_forbidden_imports`, `task_definition`), `pkg/k8s_connector_*` (Spec 008's tests are untouched), and the `k8s/apis/.../v1` round-trip test — evidence: `make precommit` exits 0.
- [ ] `CHANGELOG.md` gains a `feat:` bullet describing the informer-backed inventory migration AND a `chore:` (or `refactor:`) bullet describing the deletion of the static slice — evidence: `grep -nE 'feat:.*informer|feat:.*Schedule CRD|feat:.*store' CHANGELOG.md` returns line ≥1.
- [ ] `make precommit` exits 0 from the repo root — evidence: exit code 0.

## Verification

```
cd ~/workspaces/recurring-task-creator-informer-inventory
make precommit
ls pkg/schedule/inventory.go 2>&1 | grep -c 'No such file'
grep -RnE 'schedule\.Inventory\(\)' --include='*.go' . || echo "no consumers remain"
grep -n "SetupCustomResourceDefinition" cmd/run-once/main.go main.go
```

Expected:
- `make precommit` exits 0.
- `ls pkg/schedule/inventory.go` reports `No such file or directory` (the grep matches one line).
- The recursive grep prints `no consumers remain` (no production-tree match).
- The last grep returns at least two matches (one per file or via the shared wiring path they both reach).

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Store package + adapter (no wiring). Create the new package with the `ScheduleStore` interface, the informer-backed implementation, the adapter from `v1.Schedule` to `schedule.TaskDefinition`, the Counterfeiter mock under `mocks/`, and the unit + integration tests (fake clientset + real informer + cache sync). `pkg/schedule/inventory.go` remains in place this prompt — no consumers are rewired yet, so existing tests still pass. CHANGELOG entry stays as a TODO until prompt 2. | 1, 2, 5 | 1, 7, 8, 9, 10, 11 | — |
| 2 | Wire the store into `application.Run` + delete the static slice. `application.Run` now calls `SetupCustomResourceDefinition` first, then constructs the informer factory and the store, then `cache.WaitForCacheSync` with a 30s deadline, then constructs the tick loop with the store; `cmd/run-once/main.go` reaches the same shared wiring path. Factory's `CreateTick` / `CreateTriggerHandler` / `CreateTickLoop` are rewired to accept the store. `pkg/schedule/inventory.go` is deleted; `Inventory()` is removed; every test that previously called `schedule.Inventory()` is rewritten to inject a fixed test slice through the store interface (the publisher's full-inventory test from Spec 008 may need a deterministic test-store seed). CHANGELOG entry lands here. | 3, 4, 6, 7, 8 | 2, 3, 4, 5, 6, 12, 13, 14 | prompt 1 |

Rationale: prompt 1 lands the new package side by side with the static slice so its tests can be written and verified without touching the rest of the codebase — zero risk of breaking the publisher / tick / trigger contracts. Prompt 2 is then a focused wiring change: it sequences CRD install before informer, replaces every consumer's data source through the factory, deletes the static slice, and updates the tests that previously relied on `schedule.Inventory()`. Splitting differently (delete first, then wire) would leave the binary uncompilable between prompts.

The on-disk CRD bootstrap (the 45 `Schedule` YAMLs that re-introduce today's inventory in-cluster) is INTENTIONALLY out of scope for this spec. It is a separate follow-up spec — likely large enough to warrant its own spec and its own canary/staging strategy. Until that bootstrap ships, the binary publishes zero tasks per tick. This is named in `Failure Modes` and in `Non-goals` so the prompt-creator does not try to fold it into prompt 2.

## Related

- Predecessor: `[[Build Recurring Task Creator]]`
- Predecessor spec (CRD scaffolding): `specs/in-progress/008-crd-scaffolding.md` (the connector, the types, the schema, and the generated client live here)
- Recurrence enum closed by Spec 9: `specs/completed/009-weekday-kind-split.md`

## Do-Nothing Option

If we don't do this: editing the recurring-task inventory remains a Go PR + release + deploy. Spec 008's CRD types, schema, connector, and generated client tree (`k8s/client/...`) remain dead code in the binary — the `K8sConnector` is never invoked, the informer tree is never imported. The vault accumulates the cost of every "I want to add one more weekly check-in" round-trip through the full Go release pipeline. The wiring is the cheapest dependency in the chain; deferring it strands the foundation Spec 008 paid the cost to build.
