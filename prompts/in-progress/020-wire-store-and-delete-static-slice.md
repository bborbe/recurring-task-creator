---
status: approved
spec: [010-informer-backed-inventory]
created: "2026-06-16T00:00:00Z"
queued: "2026-06-16T19:26:22Z"
branch: dark-factory/informer-backed-inventory
---

<summary>
- The recurring-task inventory now comes from `Schedule` custom resources in the cluster instead of a hard-coded Go slice — editing the inventory becomes a `kubectl apply`, not a code change plus release plus deploy.
- On startup the binary installs the `Schedule` CRD, builds an informer watching only its own pod-namespace, and waits for the cache to fill (up to 30 seconds) before the first tick fires — so the very first tick sees the real inventory, never an empty one.
- The hourly tick and the on-demand `/trigger` handler both read today's tasks from the live store.
- The local one-shot smoke-test binary follows the same startup path so it behaves like production.
- The old 45-entry static slice and its accessor are deleted; every test that depended on them is rewritten to build its own task fixtures.
- The pod now learns its own namespace via the Kubernetes Downward API, injected through a new env var on the StatefulSet.
- A single CHANGELOG entry records the switch from static inventory to CRD-backed inventory.
</summary>

<objective>
Wire the `pkg/store.ScheduleStore` (shipped in prompt 1) into both startup paths, install the CRD and sync the informer cache before the first tick, make the tick and `/trigger` handler read from the store, delete the static inventory slice, rewrite all dependent tests, inject the pod namespace via the Downward API, and record the change in the CHANGELOG.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions.

Read these coding-plugin docs before implementing:
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` (factory has zero business logic, `Create*` prefix, returns interfaces)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-composition.md` (deps visible in constructor params, no package-function calls from business logic)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`

Read these source files fully before implementing (the wiring touches all of them):
- `/workspace/main.go` — `application` struct + `Run`. Currently builds `pub`, `clock`, `metrics`, calls `factory.CreateTick(ctx, pub, clock, metrics)`, sets `a.TriggerHandler = factory.CreateTriggerHandler(pub)`, runs `run.CancelOnFirstFinish(ctx, a.runHTTPServer(), tickLoop.Run)`. NO `Namespace` field yet.
- `/workspace/cmd/run-once/main.go` — `application` struct (`Stage cqrsbase.Branch`) + `Run`. Currently calls `factory.CreateTickLoop(ctx, syncProducer, a.Stage, a.DryRun).RunOnce(ctx)`. NO `Namespace` field yet.
- `/workspace/pkg/factory/factory.go` — `CreateTick`, `CreateTriggerHandler`, `CreateTickLoop`, `CreatePublisher`, `CreateCommandSender`, `CreateHealthzHandler`.
- `/workspace/pkg/tick/tick.go` — `NewTick(ctx, inventory []schedule.TaskDefinition, pub, clock, metrics)` and the unexported `tick()` method that today calls `schedule.TasksForDate(date)` (reading the package-level static var, NOT the constructor `inventory` field).
- `/workspace/pkg/handler/trigger.go` — `NewTriggerHandler(publisher publisher.Publisher) http.Handler`; handler body calls `schedule.TasksForDate(date)`.
- `/workspace/pkg/schedule/tasks_for_date.go` — `TasksForDate(date Date) []TaskDefinition` calls `filterInventoryByDate(inventory, date)` against the package-level static var.
- `/workspace/pkg/schedule/inventory.go` — the 45-entry static `var inventory` + `func Inventory()` (to be DELETED).
- `/workspace/pkg/schedule/inventory_export_test.go` — `AllDefinitionsForTest()` and `FilterInventoryByDateForTest()` (both reference the static var; to be removed/rewritten).
- `/workspace/pkg/k8s_connector.go` — `K8sConnector` interface with `SetupCustomResourceDefinition(ctx context.Context) error`; `NewK8sConnector(configBuilder ConfigBuilder, clientBuilder CRDClientBuilder) K8sConnector`; `ConfigBuilder func() (*rest.Config, error)`; `CRDClientBuilder func(*rest.Config) (apiextensionsclient.Interface, error)`.
- `/workspace/k8s/recurring-task-creator-sts.yaml` — the StatefulSet; `spec.template.spec.containers[0].env` is the env list to extend.
- `/workspace/pkg/store/store.go` — `NewScheduleStore(lister listersv1.ScheduleLister, namespace string) ScheduleStore` and `ScheduleStore.List(ctx) ([]schedule.TaskDefinition, error)` (from prompt 1). If `pkg/store/store.go` does NOT exist, STOP and report `status: failed` with message "prompt 1 (pkg/store) not yet deployed".

Verified k8s client APIs needed for wiring:
- `versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"` — `versioned.NewForConfig(c *rest.Config) (*versioned.Clientset, error)`.
- `externalversions "github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"`:
  - `externalversions.NewSharedInformerFactoryWithOptions(client versioned.Interface, defaultResync time.Duration, options ...externalversions.SharedInformerOption) externalversions.SharedInformerFactory`
  - `externalversions.WithNamespace(namespace string) externalversions.SharedInformerOption`
  - `factory.Task().V1().Schedules().Lister()` -> `ScheduleLister`
  - `factory.Task().V1().Schedules().Informer()` -> `cache.SharedIndexInformer`
  - `factory.StartWithContext(ctx)` ; `factory.WaitForCacheSyncWithContext(ctx) cache.SyncResult`
- `rest.InClusterConfig() (*rest.Config, error)` from `k8s.io/client-go/rest`.
- `cache.SyncResult` from `k8s.io/client-go/tools/cache` — it is a `map[reflect.Type]bool`; a sync succeeded for all informers when every value is `true`. Verify the exact type by reading `WaitForCacheSyncWithContext` in `/workspace/k8s/client/informers/externalversions/factory.go` and assert success accordingly (e.g. range the result; if any value is false, the cache did not sync within the deadline).
</context>

<requirements>

### A. Change `TasksForDate` to take the slice (decouple from the static var)

1. In `/workspace/pkg/schedule/tasks_for_date.go`, change the public function signature to:
   ```go
   func TasksForDate(defs []TaskDefinition, date Date) []TaskDefinition {
       return filterInventoryByDate(defs, date)
   }
   ```
   Keep `filterInventoryByDate` exactly as-is (it already takes `defs`). Update the GoDoc comment to say the caller supplies the definitions (no longer reads a package-level inventory).

2. In `/workspace/pkg/schedule/inventory_export_test.go`: delete `FilterInventoryByDateForTest` (now redundant — the public `TasksForDate` takes a slice). Delete `AllDefinitionsForTest` (it references the static var which is being removed). If removing both leaves the file empty of declarations, delete the whole file.

### B. Delete the static inventory

3. Delete `/workspace/pkg/schedule/inventory.go` entirely (the 45-entry `var inventory` and `func Inventory()`).

4. After deletion, `pkg/schedule` will have dangling references in tests. Find them: `grep -rn "schedule.Inventory()\|inventory\b\|AllDefinitionsForTest\|FilterInventoryByDateForTest" pkg/schedule/`. Rewrite each affected test to build its own small `[]schedule.TaskDefinition` fixture instead of reading the static inventory. Specifically inspect and fix:
   - `/workspace/pkg/schedule/tasks_for_date_test.go` — already passes custom fixtures to the worker; update it to call the new `TasksForDate(defs, date)` signature.
   - `/workspace/pkg/schedule/inventory_validation_test.go`, `/workspace/pkg/schedule/inventory_accessor_test.go`, `/workspace/pkg/schedule/canonical_slugs_test.go` — these validate the static inventory's CONTENTS (slug uniqueness, placeholder set, weekday allow-lists, canonical slug count). The static inventory is gone, so these data-content invariants no longer apply at the Go level (they move to the CRD layer, out of scope). DELETE these three test files. Confirm via `grep` that nothing else references the symbols they defined.

### C. Tick and trigger read from the store

5. In `/workspace/pkg/tick/tick.go`: the tick must read the inventory from the store each tick, then date-filter it. Add a store dependency to the constructor. Change `NewTick` to:
   ```go
   func NewTick(
       ctx context.Context,
       store store.ScheduleStore,
       pub publisher.Publisher,
       clock libtime.CurrentDateTimeGetter,
       metrics Metrics,
   ) (Tick, error)
   ```
   Remove the `inventory []schedule.TaskDefinition` parameter and the `inventory` struct field. Add a `store store.ScheduleStore` field. Import `"github.com/bborbe/recurring-task-creator/pkg/store"`.
   In the `tick()` method, replace `defs := schedule.TasksForDate(date)` with:
   ```go
   all, err := t.store.List(ctx)
   if err != nil {
       glog.Errorf("tick: list store failed for %04d-%02d-%02d: %v", date.Year, date.Month, date.Day, err)
       return
   }
   defs := schedule.TasksForDate(all, date)
   ```
   The rest of `tick()` (gauge, per-task publish loop, metrics) is unchanged. A store-list failure logs and skips this tick (next hourly tick retries) — it does NOT abort the loop or crash.

6. In `/workspace/pkg/handler/trigger.go`: add a store dependency. Change the constructor to:
   ```go
   func NewTriggerHandler(store store.ScheduleStore, publisher publisher.Publisher) http.Handler
   ```
   Inside the handler, after parsing the date, replace `tasks := schedule.TasksForDate(date)` with:
   ```go
   all, err := store.List(req.Context())
   if err != nil {
       glog.Errorf("trigger: list store failed for %s: %v", param, err)
       writeTriggerError(resp, http.StatusInternalServerError, "failed to read schedule inventory")
       return
   }
   tasks := schedule.TasksForDate(all, date)
   ```
   Keep the slug-sort and the per-task publish loop unchanged. Import `"github.com/bborbe/recurring-task-creator/pkg/store"`. Add the `errors`/`internal-server-error` path test in step 12.

### D. Factory rewiring

7. In `/workspace/pkg/factory/factory.go`:
   - `CreateTick` gains the store param and drops the `schedule.Inventory()` call:
     ```go
     func CreateTick(
         ctx context.Context,
         store store.ScheduleStore,
         pub publisher.Publisher,
         clock libtime.CurrentDateTimeGetter,
         metrics tick.Metrics,
     ) tick.Tick {
         t, err := tick.NewTick(ctx, store, pub, clock, metrics)
         if err != nil {
             panic(errors.Wrap(ctx, err, "create tick failed"))
         }
         return t
     }
     ```
   - `CreateTriggerHandler` gains the store param:
     ```go
     func CreateTriggerHandler(store store.ScheduleStore, publisher publisher.Publisher) http.Handler {
         return handler.NewTriggerHandler(store, publisher)
     }
     ```
   - `CreateTickLoop` gains the store param (it is the only caller in `cmd/run-once`):
     ```go
     func CreateTickLoop(
         ctx context.Context,
         store store.ScheduleStore,
         syncProducer libkafka.SyncProducer,
         branch cqrsbase.Branch,
         dryRun bool,
     ) tick.Tick {
         pub := CreatePublisher(CreateCommandSender(syncProducer, branch, dryRun), dryRun)
         clock := libtime.NewCurrentDateTime()
         metrics := tick.NewPrometheusMetrics()
         return CreateTick(ctx, store, pub, clock, metrics)
     }
     ```
   - Add a factory helper that builds the informer factory + store from a `versioned.Interface` and namespace. The factory pattern forbids business logic, but plumbing object construction is allowed. Add:
     ```go
     // CreateScheduleStore builds the informer-backed ScheduleStore for the
     // given namespace. The caller is responsible for starting the returned
     // factory (StartWithContext) and waiting for cache sync before reading.
     func CreateScheduleStore(client versioned.Interface, namespace string) (externalversions.SharedInformerFactory, store.ScheduleStore) {
         informerFactory := externalversions.NewSharedInformerFactoryWithOptions(client, 0, externalversions.WithNamespace(namespace))
         lister := informerFactory.Task().V1().Schedules().Lister()
         // touch the informer so the factory registers it before Start
         _ = informerFactory.Task().V1().Schedules().Informer()
         return informerFactory, store.NewScheduleStore(lister, namespace)
     }
     ```
     Imports to add: `versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"`, `externalversions "github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"`, `"github.com/bborbe/recurring-task-creator/pkg/store"`. Remove the now-unused `"github.com/bborbe/recurring-task-creator/pkg/schedule"` import if nothing else in the file uses it.

### E. main.go wiring

8. In `/workspace/main.go`:
   - Add a `Namespace` field to the `application` struct:
     ```go
     Namespace string `required:"true" arg:"namespace" env:"NAMESPACE" usage:"Pod namespace for Schedule CR watch"`
     ```
     Place it next to the other string config fields.
   - In `Run`, BEFORE building the tick, install the CRD and build+sync the store. Insert after the build-info metric line, before (or near) the sender/publisher block:
     ```go
     connector := pkg.NewK8sConnector(rest.InClusterConfig, func(c *rest.Config) (apiextensionsclient.Interface, error) {
         return apiextensionsclient.NewForConfig(c)
     })
     if err := connector.SetupCustomResourceDefinition(ctx); err != nil {
         return errors.Wrap(ctx, err, "setup CRD failed")
     }

     restConfig, err := rest.InClusterConfig()
     if err != nil {
         return errors.Wrap(ctx, err, "in-cluster config failed")
     }
     versionedClient, err := versioned.NewForConfig(restConfig)
     if err != nil {
         return errors.Wrap(ctx, err, "build versioned client failed")
     }
     informerFactory, scheduleStore := factory.CreateScheduleStore(versionedClient, a.Namespace)

     // Lifecycle: start the informer goroutines with the LONG-LIVED `ctx`
     // so they keep running for the life of the process. ONLY bound the
     // initial cache sync with a 30s deadline — `StartWithContext` must
     // NOT receive a deadline-bounded context, or the informer goroutines
     // will exit at the 30s mark and silently stop delivering updates.
     informerFactory.StartWithContext(ctx)
     syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
     defer syncCancel()
     syncResult := informerFactory.WaitForCacheSyncWithContext(syncCtx)
     for _, ok := range syncResult {
         if !ok {
             return errors.Errorf(ctx, "informer cache did not sync within 30s")
         }
     }
     ```
     Use `errors.Errorf(ctx, "...")` (the project's `github.com/bborbe/errors` package — same form used in `pkg/k8s_connector.go` for static-string error messages). Do NOT use `errors.New(ctx, ...)`; the package does not expose `New`.
   - Pass the store into the tick and trigger wiring:
     ```go
     tickLoop := factory.CreateTick(ctx, scheduleStore, pub, clock, metrics)
     ...
     a.TriggerHandler = factory.CreateTriggerHandler(scheduleStore, pub)
     ```
   - Add imports: `pkg "github.com/bborbe/recurring-task-creator/pkg"`, `versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"`, `"k8s.io/client-go/rest"`, `apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"`. `time` and `errors` are already imported.

### F. cmd/run-once wiring

9. In `/workspace/cmd/run-once/main.go`:
   - Add the same `Namespace` field to its `application` struct:
     ```go
     Namespace string `required:"true" arg:"namespace" env:"NAMESPACE" usage:"Pod namespace for Schedule CR watch"`
     ```
   - In `Run`, install the CRD, build the versioned client, build the store, start the factory and wait for sync (same pattern as main.go step 8) BEFORE calling `CreateTickLoop`. Then:
     ```go
     return factory.CreateTickLoop(ctx, scheduleStore, syncProducer, a.Stage, a.DryRun).RunOnce(ctx)
     ```
   - Add the same imports as main.go (`pkg`, `versioned`, `rest`, `apiextensionsclient`, `externalversions` is not needed directly since `CreateScheduleStore` returns the factory). Add `"time"`.
   - This binary is a real smoke-test against the cluster: it must reach the identical CRD-install + cache-sync path as production. Do not stub it out.

### G. StatefulSet Downward API

10. In `/workspace/k8s/recurring-task-creator-sts.yaml`, add to `spec.template.spec.containers[0].env` (the env list that currently has `LISTEN`, `KAFKA_BROKERS`, `STAGE`, `DRY_RUN`, `SENTRY_DSN`, `SENTRY_PROXY`, `TZ`):
    ```yaml
    - name: NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    ```
    This injects the pod's own namespace via the Downward API.

### H. Rewrite consumer tests

11. `/workspace/pkg/tick/tick_test.go` and `/workspace/pkg/tick/metrics_test.go`: these call `tick.NewTick(ctx, schedule.Inventory(), ...)` / build fixtures from the static inventory. Rewrite them to:
    - Construct a `*mocks.FakeScheduleStore` (from prompt 1, at `/workspace/mocks/store-store.go`) and set `fakeStore.ListReturns(<fixture defs>, nil)`.
    - Pass the fake store to `tick.NewTick(ctx, fakeStore, pub, clock, metrics)`.
    - Add a test: `fakeStore.ListReturns(nil, errors.New(ctx, "boom"))` (or a plain test error) and assert the tick logs+skips (no publish call, no panic, `Run`/`RunOnce` returns nil for RunOnce). Confirm `fakeStore.ListCallCount()` and `publisherMock.PublishCallCount() == 0` on the error path.

12. `/workspace/pkg/handler/trigger_test.go`: rewrite to pass a `*mocks.FakeScheduleStore` to `NewTriggerHandler(fakeStore, pub)`. Update existing happy-path specs to set `fakeStore.ListReturns(<fixture defs>, nil)`. Add a spec for the store-error path: `fakeStore.ListReturns(nil, <test error>)` -> handler responds HTTP 500 with body `{"error":"failed to read schedule inventory"}` and makes zero publish calls.

13. `/workspace/pkg/factory/factory_test.go`: update any test that calls `CreateTick`, `CreateTriggerHandler`, or `CreateTickLoop` to the new signatures (pass a `*mocks.FakeScheduleStore`). Add a test for `CreateScheduleStore` that passes a `fake.NewSimpleClientset(...)` (`fake "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned/fake"`) and asserts it returns a non-nil factory and a non-nil store, and that after `StartWithContext` + sync the store lists the seeded CR.

14. Locate every `schedule.Inventory()` call site in test files with `grep -rn 'schedule\.Inventory()' /workspace/pkg/` (also catches import-aliased forms via the trailing `.Inventory()` — confirm each match's import). For each match, replace with a hand-built `[]schedule.TaskDefinition` fixture covering only the recurrence kinds and weekday-targeted entries the test actually asserts on. Do NOT reintroduce a static-inventory dependency by, e.g., embedding the full 45-entry fixture inline. If the test was iterating the inventory only to assert "publisher accepts every entry" with no per-entry assertion, the test was a fitness-fn over the slice — delete it; the integration test added in step 13 (`CreateScheduleStore` + fake clientset + sync + list) covers the same property at the store layer.

### I. CHANGELOG

15. Add to `/workspace/CHANGELOG.md` under `## Unreleased` (append; do not replace existing entries):
    ```
    - feat: Replace the static 45-entry `pkg/schedule` inventory with a Kubernetes informer-backed `pkg/store.ScheduleStore` over the `Schedule` CRD; the tick and `GET /trigger?date=` now read today's tasks from the live cache. On boot the binary installs the CRD via `K8sConnector.SetupCustomResourceDefinition`, builds a single-namespace informer (pod namespace from the `NAMESPACE` Downward-API env), and blocks on cache sync (30s deadline) before the first tick. `pkg/schedule/inventory.go` and `schedule.Inventory()` are deleted; `schedule.TasksForDate` now takes the definition slice as a parameter. Editing the recurring-task inventory is now a `kubectl apply`.
    ```
    Follow `changelog-guide.md` style.
</requirements>

<constraints>
- Single-namespace scope: the informer watches `Schedule` resources ONLY in the pod's own namespace (`a.Namespace`, from `NAMESPACE` env). Do NOT add a cluster-wide watch or a namespace flag with a default — `required:"true"`, no default.
- The CRD MUST be installed (`SetupCustomResourceDefinition`) AND the informer cache MUST be synced before the first tick fires. Order in `Run`: setup CRD -> build client -> build store -> start factory -> wait sync (30s) -> run tick loop. A sync failure within 30s returns a wrapped error from `Run` (the binary CrashLoopBackOffs; this is correct — no inventory means no safe tick).
- On-disk CRD bootstrap (the 45 YAML files) is OUT OF SCOPE. Do not create any `Schedule` YAML manifests.
- Do NOT add a refresh-interval knob or any informer-resync tunable — `defaultResync` is `0` (no periodic resync; the informer's watch keeps the cache live). This is invariant.
- Both startup paths (`main.go` AND `cmd/run-once/main.go`) reach the identical CRD-install + cache-sync wiring. Updating only one is a compile/behavior bug — update both.
- Error wrapping: `errors.Wrap(ctx, err, "...")` / `errors.Errorf(ctx, "...")` from `github.com/bborbe/errors`. Never `fmt.Errorf`, never `context.Background()` in `pkg/`/`main.go`/`cmd/` production code.
- Factory stays zero-business-logic: `CreateScheduleStore` is pure plumbing (construct factory, get lister, return store) — no loops over data, no conditionals on CR contents.
- No Jira / ADF imports anywhere new. `pkg/schedule` stays free of Kafka/HTTP/k8s imports (you are only changing `TasksForDate`'s signature there).
- Slugs are frozen.
- Use Counterfeiter mocks (`mocks.FakeScheduleStore`, existing publisher/metrics fakes) — no hand-written mocks. In-test `fake.NewSimpleClientset` is the supported way to drive the real informer.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass (after you rewrite the ones that reference the deleted static inventory).
</constraints>

<verification>
Run from `/workspace`:
```
# static slice is gone:
test ! -f pkg/schedule/inventory.go && echo "inventory.go deleted OK"
! grep -rn "schedule.Inventory()" pkg/ main.go cmd/
! grep -rn "func Inventory()" pkg/schedule/

# new signature in place:
grep -n "func TasksForDate(defs \[\]TaskDefinition, date Date)" pkg/schedule/tasks_for_date.go
grep -n "store store.ScheduleStore" pkg/tick/tick.go
grep -n "func NewTriggerHandler(store store.ScheduleStore" pkg/handler/trigger.go
grep -n "func CreateScheduleStore(" pkg/factory/factory.go

# both binaries wire CRD + namespace:
grep -n "SetupCustomResourceDefinition" main.go cmd/run-once/main.go
grep -n "NAMESPACE" main.go cmd/run-once/main.go
grep -n "WaitForCacheSyncWithContext" main.go cmd/run-once/main.go

# downward API in STS:
grep -n "fieldPath: metadata.namespace" k8s/recurring-task-creator-sts.yaml

# changelog:
grep -n "informer-backed" CHANGELOG.md

# coverage for changed packages:
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/tick/... ./pkg/handler/... ./pkg/factory/... ./pkg/schedule/... && go tool cover -func=/tmp/cover.out | tail -1

make test
make precommit
```
`make precommit` MUST exit 0. If it fails, fix and re-run only the failing target, then re-run `make precommit` once.
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
