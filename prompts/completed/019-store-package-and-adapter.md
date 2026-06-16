---
status: completed
spec: [010-informer-backed-inventory]
summary: Created pkg/store package with ScheduleStore interface, informer-backed implementation, adapter, counterfeiter mock, and full tests at 100% statement coverage; make precommit exits 0
container: recurring-task-creator-informer-inventory-exec-019-store-package-and-adapter
dark-factory-version: v0.179.0-dirty
created: "2026-06-16T00:00:00Z"
queued: "2026-06-16T19:26:22Z"
started: "2026-06-16T19:26:23Z"
completed: "2026-06-16T19:36:13Z"
branch: dark-factory/informer-backed-inventory
---

<summary>
- Adds a new runtime store that holds the recurring-task inventory in memory, fed by a Kubernetes informer cache over the `Schedule` CRD instead of a hard-coded Go slice.
- The store exposes one read method that returns the current set of task definitions; callers (tick, trigger handler) will later read from it instead of the static slice.
- Translates each `Schedule` custom resource into the internal task-definition shape, lower-casing the recurrence and mapping the weekday name to a Go weekday.
- A custom resource with an unrecognized recurrence value is logged and dropped from the result rather than crashing the read — one bad CR never poisons the whole inventory.
- Ships a Counterfeiter mock for the store interface so the wiring prompt can test consumers in isolation.
- Includes unit tests for the adapter (every recurrence value, weekday parsing, invalid-recurrence drop) and an integration test that drives a real informer over a fake clientset.
- This prompt does NOT wire anything into `main.go`, the factory, or `cmd/run-once`; the old static slice stays in place and untouched. Wiring is the next prompt.
</summary>

<objective>
Create a new `pkg/store` package containing a `ScheduleStore` interface and an informer-backed implementation that converts `Schedule` custom resources into `[]schedule.TaskDefinition`, plus a Counterfeiter mock and full tests. No production wiring in this prompt — the static inventory remains in place.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions.

Read these coding-plugin docs before implementing:
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` (interface + private struct + `New*` constructor, error wrapping, counterfeiter)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` (`errors.Wrap(ctx, err, "...")`, never `fmt.Errorf`, never `context.Background()`)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` (Ginkgo v2 / Gomega suite, external `_test` package, coverage >=80%)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` (counterfeiter annotations)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-test-types-guide.md` (unit vs integration)

Read these source files fully before implementing:
- `/workspace/pkg/schedule/task_definition.go` — the `TaskDefinition` struct you map into. Fields: `Slug string`, `TitleTemplate string`, `BodyTemplate string`, `Recurrence RecurrenceKind`, `Weekday time.Weekday`.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` is a `string` type. Valid lowercase values: `"daily"`, `"weekly"`, `"weekday"`, `"monthly"`, `"quarterly"`, `"yearly"`. `AllRecurrenceKinds` is the canonical slice.
- `/workspace/k8s/apis/task.benjamin-borbe.de/v1/types.go` — the `Schedule` CR. Relevant fields: `Schedule.Name` (from `metav1.ObjectMeta`, this is the slug), `Schedule.Spec.Title`, `Schedule.Spec.Template.Body`, `Schedule.Spec.Schedule.Recurrence` (capital-case string like `"Weekday"`), `Schedule.Spec.Schedule.Weekday` (capital-case string like `"Monday"`, may be empty).
- `/workspace/pkg/k8s_connector.go` — read for the error-wrapping idiom (`errors.Wrap(ctx, err, "...")`) and `//counterfeiter:generate` annotation style.
- `/workspace/mocks/k8s_connector.go` — existing generated mock, for naming/layout reference.

Verified informer/lister API (already generated under `k8s/client/...`):
- Versioned clientset: `versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"` — `versioned.NewForConfig(c *rest.Config) (*versioned.Clientset, error)`; `versioned.Interface` is the interface type the informer factory accepts.
- Informer factory: `externalversions "github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"`
  - `externalversions.NewSharedInformerFactoryWithOptions(client versioned.Interface, defaultResync time.Duration, options ...externalversions.SharedInformerOption) externalversions.SharedInformerFactory`
  - `externalversions.WithNamespace(namespace string) externalversions.SharedInformerOption`
  - `factory.Task().V1().Schedules().Lister()` returns a `ScheduleLister`
  - `factory.Task().V1().Schedules().Informer()` returns `cache.SharedIndexInformer`
  - `factory.StartWithContext(ctx context.Context)`
  - `factory.WaitForCacheSyncWithContext(ctx context.Context) cache.SyncResult`
- Lister: `listersv1 "github.com/bborbe/recurring-task-creator/k8s/client/listers/task.benjamin-borbe.de/v1"`
  - `ScheduleLister.Schedules(namespace string) ScheduleNamespaceLister`
  - `ScheduleNamespaceLister.List(selector labels.Selector) ([]*v1.Schedule, error)` where `labels` is `k8s.io/apimachinery/pkg/labels` (use `labels.Everything()`)
- Fake clientset for the integration test: `fake "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned/fake"` — `fake.NewSimpleClientset(objects ...runtime.Object) *fake.Clientset` (implements `versioned.Interface`).
- CR type alias in imports: `v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"`.
</context>

<requirements>
1. Create `/workspace/pkg/store/store.go` (package `store`) with the standard license header:
   ```
   // Copyright (c) 2026 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.
   ```

2. Define the interface and counterfeiter annotation in `store.go`:
   ```go
   //counterfeiter:generate -o ../../mocks/store-store.go --fake-name FakeScheduleStore . ScheduleStore

   // ScheduleStore returns the current recurring-task inventory, read from
   // the informer cache over the Schedule CRD.
   type ScheduleStore interface {
       // List returns every Schedule CR in the watched namespace, adapted
       // to []schedule.TaskDefinition. CRs whose Recurrence value is not a
       // known RecurrenceKind are logged and dropped (never abort the read).
       // A lister error is wrapped and returned.
       List(ctx context.Context) ([]schedule.TaskDefinition, error)
   }
   ```
   Confirm the package needs a counterfeiter bootstrap. If `pkg/store` has no `generate.go` declaring the counterfeiter tool import, add `/workspace/pkg/store/generate.go`:
   ```go
   // Copyright (c) 2026 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.

   package store

   //go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
   ```
   Check whether an existing package relies on a repo-level counterfeiter directive instead. If the repo uses an inline `//go:generate` per package, follow that; if it uses a single root directive, do NOT add `generate.go`. Match whatever `pkg/tick` does — the project convention is the `//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate` directive in `/workspace/pkg/tick/tick_suite_test.go` (NOT in `tick.go`). Read `/workspace/pkg/tick/tick_suite_test.go` for the exact form and mirror it in your store suite.

3. Implement the private struct + constructor in `store.go`:
   ```go
   func NewScheduleStore(lister listersv1.ScheduleLister, namespace string) ScheduleStore {
       return &scheduleStore{lister: lister, namespace: namespace}
   }

   type scheduleStore struct {
       lister    listersv1.ScheduleLister
       namespace string
   }
   ```
   The constructor takes the `ScheduleLister` (verified import `listersv1 "github.com/bborbe/recurring-task-creator/k8s/client/listers/task.benjamin-borbe.de/v1"`) and the namespace string. Wiring of the informer factory that produces this lister happens in prompt 2 — this constructor accepts an already-built lister so it is trivially unit-testable.

4. Implement `List`:
   ```go
   func (s *scheduleStore) List(ctx context.Context) ([]schedule.TaskDefinition, error) {
       crs, err := s.lister.Schedules(s.namespace).List(labels.Everything())
       if err != nil {
           return nil, errors.Wrap(ctx, err, "list schedules from informer cache")
       }
       out := make([]schedule.TaskDefinition, 0, len(crs))
       for _, cr := range crs {
           def, err := adaptSchedule(cr)
           if err != nil {
               glog.Warningf("store: dropping schedule %q: %v", cr.Name, err)
               continue
           }
           out = append(out, def)
       }
       return out, nil
   }
   ```
   Imports: `"github.com/bborbe/errors"`, `"github.com/golang/glog"`, `labels "k8s.io/apimachinery/pkg/labels"`, `"github.com/bborbe/recurring-task-creator/pkg/schedule"`.

5. Implement the adapter `adaptSchedule(cr *v1.Schedule) (schedule.TaskDefinition, error)` in `store.go` (or a sibling file `/workspace/pkg/store/adapter.go` in package `store` — your choice, keep each file under 300 lines):
   - `Slug` = `cr.Name`
   - `TitleTemplate` = `cr.Spec.Title`
   - `BodyTemplate` = `cr.Spec.Template.Body`
   - `Recurrence`: `kind := schedule.RecurrenceKind(strings.ToLower(cr.Spec.Schedule.Recurrence))`. Validate `kind` is one of `schedule.AllRecurrenceKinds`. If not, return a wrapped error: `errors.Errorf(ctx, "unknown recurrence %q", cr.Spec.Schedule.Recurrence)` — but `adaptSchedule` has no `ctx`. Use the no-context error constructor consistent with the repo: check `go-error-wrapping-guide.md`; if a context-free error is needed, use `errors.New(ctx, ...)` only where a ctx exists. Simplest correct approach: give `adaptSchedule` a `ctx context.Context` first parameter so it can call `errors.Errorf(ctx, "unknown recurrence %q", cr.Spec.Schedule.Recurrence)`, and pass `ctx` from `List`. Adopt that signature: `adaptSchedule(ctx context.Context, cr *v1.Schedule) (schedule.TaskDefinition, error)`.
   - `Weekday`: if `cr.Spec.Schedule.Weekday == ""`, leave the zero value (`time.Sunday`). Otherwise look it up in a package-level map:
     ```go
     var weekdayByName = map[string]time.Weekday{
         "Sunday": time.Sunday, "Monday": time.Monday, "Tuesday": time.Tuesday,
         "Wednesday": time.Wednesday, "Thursday": time.Thursday,
         "Friday": time.Friday, "Saturday": time.Saturday,
     }
     ```
     If the name is non-empty but not in the map, return a wrapped error `errors.Errorf(ctx, "unknown weekday %q", cr.Spec.Schedule.Weekday)` (this CR is dropped, same as bad recurrence).
   - Validate recurrence membership by ranging `schedule.AllRecurrenceKinds` (do NOT hand-roll a duplicate set).

6. Generate the mock. Run `go generate ./pkg/store/...` (or the repo's mock-generation make target — check the Makefile for a `generate` target). Confirm `/workspace/mocks/store-store.go` is created with `FakeScheduleStore`.

7. Create the Ginkgo suite file `/workspace/pkg/store/store_suite_test.go`, package `store_test`, with the standard `TestStore(t *testing.T)` bootstrap matching `/workspace/pkg/handler/handler_suite_test.go` (read it for the exact RunSpecs boilerplate).

8. Create unit tests `/workspace/pkg/store/adapter_test.go` (package `store_test`). Because `adaptSchedule` is unexported, expose it to the external test via an export-test file `/workspace/pkg/store/store_export_test.go` (package `store`):
   ```go
   // Copyright header...
   package store

   import "context"

   // AdaptScheduleForTest exposes the unexported adapter to external tests.
   func AdaptScheduleForTest(ctx context.Context, cr *v1.Schedule) (schedule.TaskDefinition, error) {
       return adaptSchedule(ctx, cr)
   }
   ```
   (adjust imports). Unit-test these cases, each constructing a `*v1.Schedule` literal:
   - Each of the six recurrence values in capital-case (`"Daily"`, `"Weekly"`, `"Weekday"`, `"Monthly"`, `"Quarterly"`, `"Yearly"`) maps to the correct lowercase `RecurrenceKind`.
   - `Weekday: "Saturday"` maps to `time.Saturday`; empty weekday maps to `time.Sunday` (zero value).
   - `Recurrence: "Bogus"` returns an error (entry would be dropped).
   - `Weekday: "Funday"` returns an error.
   - `Slug`/`TitleTemplate`/`BodyTemplate` map from `Name`/`Spec.Title`/`Spec.Template.Body`.

9. Create the integration test `/workspace/pkg/store/store_integration_test.go` (package `store_test`) that exercises the full informer path:
   - Build `client := fake.NewSimpleClientset(<two *v1.Schedule objects, one valid Weekday CR, one invalid-recurrence CR>)`. The objects must have `ObjectMeta.Namespace` set to a fixed test namespace (e.g. `"test-ns"`) and `ObjectMeta.Name` set to the slug.
   - Build `factory := externalversions.NewSharedInformerFactoryWithOptions(client, 0, externalversions.WithNamespace("test-ns"))`.
   - `informer := factory.Task().V1().Schedules().Informer()` then `lister := factory.Task().V1().Schedules().Lister()`.
   - `ctx, cancel := context.WithCancel(context.Background())`; `defer cancel()`; `factory.StartWithContext(ctx)`.
   - Wait for sync: `Eventually(func() bool { return informer.HasSynced() }).Should(BeTrue())` (Gomega Eventually, default timeout). Do NOT block on a bare channel forever.
   - `store := store.NewScheduleStore(lister, "test-ns")`.
   - `defs, err := store.List(ctx)` — expect `err == nil`, expect exactly ONE definition returned (the valid CR; the invalid-recurrence CR is dropped), and assert its `Slug`/`Recurrence`/`Weekday`.

10. Do NOT touch `/workspace/pkg/schedule/inventory.go`, `/workspace/main.go`, `/workspace/cmd/run-once/main.go`, `/workspace/pkg/factory/factory.go`, `/workspace/pkg/tick/tick.go`, or `/workspace/pkg/handler/trigger.go` in this prompt. The static slice and all existing wiring stay exactly as-is.

11. Do NOT add a CHANGELOG entry in this prompt — the CHANGELOG entry for spec 010 lands in prompt 2 (so the feature is logged as one user-facing change when the static slice is actually deleted and the store goes live).

12. Coverage: `pkg/store` must reach >=80% statement coverage. The error-drop paths (bad recurrence, bad weekday) and the lister-error path must each have a test. For the lister-error path, use the generated mock or a small stub lister that returns an error from `List`, OR assert the wrap by constructing a `scheduleStore` against a lister whose namespace returns an error — simplest is a tiny in-test fake `ScheduleLister`/`ScheduleNamespaceLister` returning a sentinel error; confirm `store.List` returns a non-nil wrapped error.
</requirements>

<constraints>
- No Jira / ADF / Kafka / HTTP imports in `pkg/schedule/` — that package stays pure (you are not editing it here anyway).
- Slugs are frozen — the adapter must use `cr.Name` verbatim as the slug; no transformation.
- Error wrapping: always `errors.Wrap(ctx, err, "...")` / `errors.Errorf(ctx, "...")` from `github.com/bborbe/errors`. Never `fmt.Errorf`. Never `context.Background()` inside `pkg/` production code (test files may use it).
- Interfaces return interfaces, structs are private, constructors are `New*` — follow `go-patterns.md`.
- Use Counterfeiter mocks only — no hand-written mocks in `mocks/`. In-test stub listers for the lister-error path are acceptable since the generated client interfaces are large.
- External test packages (`package store_test`) for behavior tests; `store_export_test.go` (package `store`) only to expose unexported helpers.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:
```
ls pkg/store/store.go pkg/store/store_suite_test.go mocks/store-store.go
grep -n "FakeScheduleStore" mocks/store-store.go
grep -n "func NewScheduleStore" pkg/store/store.go
grep -n "weekdayByName" pkg/store/*.go
# inventory.go and wiring must be UNTOUCHED:
grep -n "func Inventory()" pkg/schedule/inventory.go
grep -n "schedule.Inventory()" pkg/factory/factory.go
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/store/... && go tool cover -func=/tmp/cover.out | tail -1
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
