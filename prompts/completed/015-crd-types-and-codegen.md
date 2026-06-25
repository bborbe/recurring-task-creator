---
status: completed
spec: [011-title-period-tokens-and-drop-recurring-frontmatter]
summary: Added k8s/apis/task.benjamin-borbe.de/v1 Go types for the Schedule CRD (Schedule, ScheduleSpec, ScheduleStatus, ScheduleList, ScheduleTrigger, ScheduleTemplate, Placeholders), wired codegen via hack/update-codegen.sh sourcing kube_codegen.sh for clientset/informers/listers/applyconfiguration, added testdata/example.yaml fixture and a sigs.k8s.io/yaml round-trip test that uses UnmarshalStrict to reject unknown fields; make precommit passes; make generatek8s is idempotent.
container: recurring-task-creator-crd-scaffolding-exec-015-crd-types-and-codegen
dark-factory-version: v0.177.1
created: "2026-06-16T07:48:18Z"
queued: "2026-06-16T07:48:18Z"
started: "2026-06-16T07:48:31Z"
completed: "2026-06-16T08:17:50Z"
branch: dark-factory/crd-scaffolding
---

<summary>
- Adds the `k8s/apis/task.benjamin-borbe.de/v1/` Go package with `Schedule`, `ScheduleSpec`, `ScheduleStatus`, `ScheduleList`, `ScheduleTrigger`, `ScheduleTemplate` types — the single source of truth for the recurring-task CRD. CRD group `task.benjamin-borbe.de`, version `v1`, kind `Schedule`, plural `schedules`, scope `Namespaced`. Names are frozen for the life of v1.
- Wires the codegen pipeline: `hack/update-codegen.sh` runs `kube::codegen::gen_helpers` (DeepCopy) and `kube::codegen::gen_client` (clientset + informers + listers) from the vendored `k8s.io/code-generator` module, pinned via a new `tools.env` variable (not `tools.go` — the project migrated off that pattern in spec 004). A new `make generatek8s` target invokes the script; `make generate` is unchanged.
- Adds `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` (the canonical `weekly-review` Schedule) and a `Ginkgo` round-trip test using `sigs.k8s.io/yaml.UnmarshalStrict` so an unknown field like `weeday: Saturday` fails the build.
- The package compiles, the `var Placeholders = []string{"Date","ISOWeek","MonthYear","Quarter","Year"}` GoDoc block is added, and `make generate && git diff --exit-code k8s/` is clean on a second run.
- `make precommit` exits 0; no new direct Go deps needed (`k8s.io/apiextensions-apiserver`, `k8s.io/apimachinery`, `k8s.io/client-go`, `sigs.k8s.io/yaml` are already in `go.sum` as indirects — promoting them to direct is fine).

</summary>

<objective>
Add the `Schedule` CustomResourceDefinition types and the codegen wiring that produces a typed clientset + informers + listers + DeepCopy. No `SetupCustomResourceDefinition` work and no informer wiring in this prompt — that lands in Prompt 2. This prompt ends with `make generatek8s` producing an idempotent, committed `k8s/client/...` tree and a passing round-trip test for `testdata/example.yaml`.
</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26.4, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6, `tools.env` for tool versions).

Read these source files fully before writing code:
- `/workspace/go.mod` — `k8s.io/apiextensions-apiserver v0.36.1`, `k8s.io/apimachinery v0.36.1`, `k8s.io/client-go v0.36.1`, `sigs.k8s.io/yaml v1.6.0` are already in `go.sum` as indirect. Promoting to direct (with a `go get`) is fine.
- `/workspace/tools.env` — the canonical pinned-version file. Add `K8S_CODEGEN_VERSION` and `CONTROLLER_GEN_VERSION` here.
- `/workspace/Makefile` — `make generate` currently does `rm -rf mocks avro && go generate -mod=mod ./...` (the counterfeiter run). Add a NEW target `generatek8s` that runs `bash hack/update-codegen.sh` — do NOT replace the existing `make generate` flow (counterfeiter on a fresh tree deletes `k8s/client/...` if it is accidentally generated under `mocks/`; the two targets must be independent).
- `/workspace/.golangci.yml` — read to know the `funlen 80`, `nestif 4`, `golines 100` caps so the type files stay compliant.
- `/workspace/Makefile.precommit` — `precommit: ensure format generate test check addlicense`. The new `generatek8s` target is intentionally NOT in `precommit` (codegen is expensive and the generated tree is stable day-to-day — see guide `go-kubernetes-crd-controller-guide.md` §3 "Workflow when types change").
- `/workspace/pkg/schedule/date.go` — `Date{Year int, Month time.Month, Day int}` is the schedule civil-date type; the CRD types do NOT reuse it (`metav1.Time` is the CRD convention for timestamp fields).
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` string alias; the CRD uses raw `string` for `Spec.Schedule.Recurrence` to keep the types YAML-clean and the enum enforcement in the OpenAPI schema (not the Go type).

Verified external symbols (read via `go doc` on 2026-06-16 in the YOLO container):

`k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1` (indirect, will be promoted to direct):
```go
type CustomResourceDefinitionSpec struct {
    Group    string
    Names    CustomResourceDefinitionNames
    Scope    ResourceScope  // apiextensionsv1.NamespaceScoped
    Versions []CustomResourceDefinitionVersion
    // ...
}
type CustomResourceDefinitionNames struct {
    Plural     string
    Singular   string
    ShortNames []string
    Kind       string
    ListKind   string
    // ...
}
type CustomResourceDefinitionVersion struct {
    Name    string
    Served  bool
    Storage bool
    Schema  *CustomResourceValidation  // contains OpenAPIV3Schema JSONSchemaProps
}
```

`k8s.io/apimachinery/pkg/apis/meta/v1` (indirect, will be promoted):
```go
type TypeMeta struct { Kind, APIVersion string }
type ObjectMeta struct { Name, Namespace string; // ... +k8s:required markers, etc. }
type ListMeta struct { // ... }
type Time struct { time.Time }  // embedded carrier, json-marshals as RFC3339
```

`k8s.io/apimachinery/pkg/runtime`:
```go
type Object interface {
    GetObjectKind() schema.ObjectKind
    DeepCopyObject() Object
}
```

`k8s.io/code-generator` (NOT yet in `go.mod` — must be added as direct dep):
- The package provides `kube_codegen.sh` (verified at `/home/node/go/pkg/mod/k8s.io/code-generator@v0.36.1/kube_codegen.sh`).
- Two top-level helpers the prompt uses:
  - `kube::codegen::gen_helpers --boilerplate <file> <input-dir>` — runs `deepcopy-gen` + `defaulter-gen` + `conversion-gen`. The output is `zz_generated.deepcopy.go` next to each input `.go` file.
  - `kube::codegen::gen_client --with-watch --with-applyconfig --output-dir <dir> --output-pkg <import-path> --boilerplate <file> <input-dir>` — runs `client-gen` + `lister-gen` + `informer-gen` + `applyconfiguration-gen`. Output is `k8s/client/{clientset/versioned,informers/externalversions,listers,applyconfiguration}/...`.
- The script `generate-internal-groups.sh` that spec text mentions is REMOVED upstream (its first line: `echo "ERROR: $(basename "$0") has been removed. ERROR: Please use k8s.io/code-generator/kube_codegen.sh instead."`). Do NOT use it.
- Boilerplate license header for generated files: `hack/boilerplate.go.txt` (3-line Apache 2.0 header — see `/home/node/go/pkg/mod/k8s.io/apiextensions-apiserver@v0.36.1/hack/boilerplate.go.txt` for the exact text). This is a different shape from the project's BSD header — generated files under `k8s/` use the Apache header; generated files under `mocks/` use the project's BSD header via the existing `addlicense` Makefile target.

`sigs.k8s.io/yaml` (already in `go.sum`):
```go
func UnmarshalStrict(yamlBytes []byte, obj interface{}, opts ...JSONOpt) error
```
Verified signature. Strict mode fails on unknown fields — that is the `weeday: Saturday` guard.

`sigs.k8s.io/controller-tools/cmd/controller-gen` (NOT yet in `go.mod` — must be added as direct dep for `DeepCopy` generation as an alternative to `kube::codegen::gen_helpers`):
- The bborbe CRD guide §3 uses BOTH `controller-gen` (for `zz_generated.deepcopy.go`) and `kube::codegen::gen_client` (for clientset/informer/lister). For maximum reliability, this prompt uses `kube::codegen::gen_helpers` for DeepCopy so the entire codegen pipeline is one script — `controller-gen` is only required if `gen_helpers` proves insufficient. Pin a version compatible with k8s 0.36.x — `v0.16.5` is the matching release.

`github.com/bborbe/agent/lib` (already direct, v0.68.0) — `lib.TaskFrontmatter` type:
```go
type TaskFrontmatter map[string]interface{}
```

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` §2 (repository layout), §3 (types + `tools.go`/codegen) — NOTE the guide's `tools.go` example is from before the 004 migration; the project's actual pattern is `tools.env` + Makefile `@version` (see `/workspace/prompts/completed/004-migrate-tools-go.md`). Mirror the migrated pattern.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct → Method. The CRD types do NOT need a `New*` constructor; the marker-driven `client-gen` + `deepcopy-gen` produce the runtime interface implementations. Just declare the structs with the right kubebuilder markers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, dot-imports, external test package (`package v1_test`).
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for new code; counterfeiter mocks (reused upstream here).

Load-bearing snippets inlined for the executor's verification:

```go
// pkg/schedule/recurrence.go (verbatim — frozen)
type RecurrenceKind string
const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)

// pkg/schedule/task_definition.go (verbatim — frozen)
type TaskDefinition struct {
    Slug, TitleTemplate, BodyTemplate string
    Recurrence RecurrenceKind
    Weekday    time.Weekday
}

// github.com/bborbe/agent/lib (verbatim — frozen). Actual package path:
// import "github.com/bborbe/agent/lib" — the TaskFrontmatter type is
// declared in lib/agent_task-frontmatter.go.
type TaskFrontmatter map[string]interface{}
```

</context>

<requirements>

## 1. Tool version pinning — `tools.env`

Add to `/workspace/tools.env` (alphabetical order with the existing entries — match the canonical style of `/workspace/prompts/completed/004-migrate-tools-go.md`):

```
K8S_CODEGEN_VERSION         ?= v0.36.1
CONTROLLER_GEN_VERSION      ?= v0.16.5
```

Pin rationale: `K8S_CODEGEN_VERSION` must match the k8s libs already in `go.mod` (`v0.36.1`). `CONTROLLER_GEN_VERSION` is the latest release of `sigs.k8s.io/controller-tools` whose client-gen output is compatible with `k8s.io/client-go v0.36.1`.

If a future spec needs `controller-gen` for additional generation (e.g. `object` generator, `rbac` generator), `CONTROLLER_GEN_VERSION` already exists — do NOT re-introduce `tools.go`.

## 2. Boilerplate header for generated k8s files

Create `/workspace/hack/boilerplate.go.txt` with the canonical Apache 2.0 header that all `kube_codegen.sh` generators prepend. **Do NOT use the project's BSD header here** — the generated clientset + informers + listers + DeepCopy all expect the Apache header (this is what `addlicense` skips — the `hack/` and `k8s/client/` trees are excluded from `addlicense` via the existing Makefile `find` filter, see `Makefile.precommit`).

Write this file verbatim — the bytes are load-bearing for codegen idempotency:

```
/*
Copyright YEAR The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

The `kube_codegen.sh` script substitutes `YEAR` with the current year at generation time.

## 3. The codegen shell script

Create `/workspace/hack/update-codegen.sh` (chmod +x via `git update-index --chmod=+x` is not needed — `bash hack/update-codegen.sh` is how the Makefile invokes it):

```bash
#!/usr/bin/env bash
#
# Regenerates DeepCopy + clientset + informers + listers + applyconfiguration
# for the Schedule CRD. Idempotent: a second run produces no diff.
#
# Sources kube_codegen.sh from the vendored k8s.io/code-generator module
# (resolved via `go list -m -f '{{.Dir}}'`, which works inside and outside
# the module directory). Do NOT add `go mod vendor` to the pre-codegen
# sequence — kube_codegen.sh is a shell script and `go mod vendor` only
# vendors .go files.
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)
THIS_PKG="github.com/bborbe/recurring-task-creator"

CODEGEN_PKG=$(cd "${SCRIPT_ROOT}" && go list -m -f '{{.Dir}}' k8s.io/code-generator 2>/dev/null || echo "")
if [[ -z "${CODEGEN_PKG}" ]]; then
    echo "k8s.io/code-generator not found in go.mod. Run: go get k8s.io/code-generator@v0.36.1" >&2
    exit 1
fi

source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/k8s/apis"

kube::codegen::gen_client \
    --with-watch \
    --with-applyconfig \
    --output-dir "${SCRIPT_ROOT}/k8s/client" \
    --output-pkg "${THIS_PKG}/k8s/client" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/k8s/apis"
```

## 4. The Makefile target

Append a new target to `/workspace/Makefile.precommit` (the file that actually contains the existing `generate:` target — confirmed via `grep -n '^generate:' Makefile.precommit`). Insert immediately after the existing `generate:` target, before `TESTFLAGS_RACE`:

```makefile
.PHONY: generatek8s
generatek8s:
	bash hack/update-codegen.sh
```

`make generatek8s` is intentionally separate from `make generate` (which is `rm -rf mocks avro && go generate ...`). The two trees must not collide. The existing `make precommit` (which depends on `generate`) is unchanged.

## 5. Add the new direct deps

`cd /workspace && go get k8s.io/apimachinery@v0.36.1 k8s.io/code-generator@v0.36.1 sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5`

(The first two move from indirect to direct; the third is brand-new.) After `go get`, `go mod tidy` cleans up. If `controller-tools` adds a deep dep tree that is unwanted at runtime, the alternative is to keep `controller-tools` as a `tools.go` dep — but the project migrated off `tools.go` in spec 004, so the Makefile `@version` pattern is preferred (the controller-gen binary only runs in the codegen script, never in production).

**Spec gap surfaced for the reviewer:** the spec's Constraints section says "pin [controller-gen] via `tools.go` like the other build deps" — this is OUTDATED (the project uses `tools.env` since spec 004). The prompt uses the project-current pattern; the spec should be amended to drop the `tools.go` mention in a follow-up.

## 6. The types package

Create `/workspace/k8s/apis/task.benjamin-borbe.de/v1/` with four files. Every new hand-written `.go` file (`doc.go`, `register.go`, `types.go`, `example_test.go`) starts with the **project's BSD header** (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` + `// Use of this source code is governed by a BSD-style` + `// license that can be found in the LICENSE file.`), matching every other hand-written file in the repo (see e.g. `pkg/schedule/recurrence.go`). The Apache 2.0 header in `hack/boilerplate.go.txt` is consumed by `kube_codegen.sh` for the GENERATED files only (`zz_generated.deepcopy.go`, the entire `k8s/client/` tree). Hand-written = BSD; generated = Apache.

### 6a. `doc.go`

```go
// +kubebuilder:object:generate=true
// +groupName=task.benjamin-borbe.de

// Package v1 is the v1 API for the Schedule custom resource definition.
// CRD group: task.benjamin-borbe.de. Version: v1. Kind: Schedule.
// Plural: schedules. Short name: ts. Scope: Namespaced. Names are frozen
// for the life of v1.
package v1
```

The two markers are the contract `controller-gen` / `deepcopy-gen` consume. `+kubebuilder:object:generate=true` opts every type in this package into DeepCopy generation. `+groupName=task.benjamin-borbe.de` populates the SchemeBuilder's GroupName.

### 6b. `register.go`

Standard scheme-builder boilerplate, modeled on the bborbe CRD guide §3 (`k8s/apis/<group>.example.com/v1/register.go`):

```go
// +kubebuilder:object:generate=true
// +groupName=task.benjamin-borbe.de

package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    runtime "k8s.io/apimachinery/pkg/runtime"
    schema "k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the API group for the Schedule CRD. Frozen.
const GroupName = "task.benjamin-borbe.de"

// Version is the API version (v1). Frozen.
const Version = "v1"

// SchemeGroupVersion is the group/version pair used by the typed clientset.
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// Resource takes an unqualified resource name and returns a GroupResource.
func Resource(resource string) schema.GroupResource {
    return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
    // SchemeBuilder collects the functions that add types to the scheme.
    SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

    // AddToScheme applies the registered functions to a runtime.Scheme.
    AddToScheme = SchemeBuilder.AddToScheme
)

// Kind is the resource Kind. Frozen.
const Kind = "Schedule"

// ListKind is the resource ListKind. Frozen.
const ListKind = "ScheduleList"

// Plural is the resource plural name. Frozen.
const Plural = "schedules"

// Singular is the resource singular name. Frozen.
const Singular = "schedule"

// ShortNames are short names for the resource. Frozen.
var ShortNames = []string{"ts"}

// addKnownTypes registers the Schedule + ScheduleList types with the scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(SchemeGroupVersion,
        &Schedule{},
        &ScheduleList{},
    )
    metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
    return nil
}
```

The `Kind`, `ListKind`, `Plural`, `Singular`, `ShortNames` constants/vars are the SINGLE source of truth — `desiredCRDSpec()` in Prompt 2 reads them directly, never hard-codes "Schedule" or "schedules".

### 6c. `types.go`

```go
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    lib "github.com/bborbe/agent/lib"
)

// Schedule is the Schema for the Schedule CRD. Names are frozen for the
// life of v1 (spec 008).
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Schedule struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ScheduleSpec   `json:"spec,omitempty"`
    Status ScheduleStatus `json:"status,omitempty"`
}

// ScheduleSpec is the spec of a Schedule.
type ScheduleSpec struct {
    // Vault is the Obsidian vault slug the generated task lives in.
    // Constrained to ^[a-z][a-z0-9-]*$ by the OpenAPI schema (enforced at
    // the API-server boundary via SetupCustomResourceDefinition in
    // pkg/k8s_connector.go).
    Vault string `json:"vault"`

    // Title is the title shown to the user.
    Title string `json:"title"`

    // Schedule describes when the task fires. The weekday-required-iff-weekly
    // invariant is encoded as a CEL x-kubernetes-validations rule in
    // scheduleSpecSchema (Prompt 2).
    Schedule ScheduleTrigger `json:"schedule"`

    // Template is the body + frontmatter stamped onto the generated task.
    Template ScheduleTemplate `json:"template"`
}

// ScheduleTrigger is the recurrence subnode.
type ScheduleTrigger struct {
    // Recurrence is one of: "Daily", "Weekly", "Monthly", "Quarterly", "Yearly"
    // (capitalized, matching Go's time.Weekday.String() style and Spec 6's
    // period-token output). Constrained by the OpenAPI enum in scheduleSpecSchema.
    Recurrence string `json:"recurrence"`

    // Weekday is required when Recurrence == "Weekly"; forbidden otherwise.
    // Values are time.Weekday.String() form: "Monday", "Tuesday", "Wednesday",
    // "Thursday", "Friday", "Saturday", "Sunday". Encoded as the CEL rule in
    // scheduleSpecSchema. The Go type is `string` (not `*string`) so JSON
    // omits the field cleanly when unset; the schema's presence check is
    // `has(self.weekday)`.
    // +optional
    Weekday string `json:"weekday,omitempty"`
}

// ScheduleTemplate is the body + frontmatter the generated task carries.
type ScheduleTemplate struct {
    // Body is raw markdown. Free-form; not validated.
    Body string `json:"body,omitempty"`

    // Frontmatter is the YAML frontmatter of the generated vault file.
    // Reuses lib.TaskFrontmatter from github.com/bborbe/agent/lib.
    Frontmatter lib.TaskFrontmatter `json:"frontmatter,omitempty"`
}

// ScheduleStatus describes the observed state of a Schedule.
type ScheduleStatus struct {
    // LastTickedAt is the wall-clock time of the most recent successful tick
    // for this Schedule. +optional.
    // +optional
    LastTickedAt metav1.Time `json:"lastTickedAt,omitempty"`

    // LastPublishedTaskIdentifier is the deterministic UUID5 of the most
    // recently published task for this Schedule. +optional.
    // +optional
    LastPublishedTaskIdentifier string `json:"lastPublishedTaskIdentifier,omitempty"`
}

// ScheduleList is a list of Schedules.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ScheduleList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`

    Items []Schedule `json:"items"`
}

// Placeholders is the documented placeholder set the rendered task
// template can reference. Rendered by the publisher (spec 002) using
// the same strings. Closed set — adding a new placeholder is a new spec.
var Placeholders = []string{
    "Date",      // Renders YYYY-MM-DD.
    "ISOWeek",   // Renders YYYYWNN (uppercase W, two-digit week).
    "MonthYear", // Renders YYYY-MM.
    "Quarter",   // Renders YYYYQNN (uppercase Q, two-digit quarter).
    "Year",      // Renders YYYY.
}
```

Notes on the above:
- `+kubebuilder:object:root=true` on `Schedule` (in addition to the package-level marker) is required by `controller-gen` to know the root type for client-gen.
- `+genclient` + `+genclient:noStatus` opt the type into clientset generation; the `:noStatus` modifier suppresses `UpdateStatus`/`ApplyStatus` methods (this CR has a status field but the spec does not require status writes — that's Spec B's job).
- `+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object` on `Schedule` AND `ScheduleList` is what `deepcopy-gen` reads to generate `DeepCopyObject() runtime.Object`. **Required on BOTH** — one of the most common bugs in hand-written CRD setups is forgetting it on the list type, which then fails to satisfy `runtime.Object` at compile time.
- `ScheduleTrigger.Weekday` is `string` not `*string` so the JSON omitempty path works for the "weekly with no weekday" error case (the OpenAPI schema is what actually enforces the rule). Pointer types in kubebuilder schemas have a different `required` semantics that complicates `XPreserveUnknownFields` and `XValidations` (out of scope here).
- `lib.TaskFrontmatter` is a `map[string]interface{}`; the CRD `Schema` (in Prompt 2) describes it as a free-form object, not a structured one — frontmatter is intentionally user-controlled.
- `ScheduleStatus` is declared but the spec says NO status subresource in v1; the `+kubebuilder:subresource:status` marker on `Schedule` is therefore intentionally OMITTED in this prompt (Prompt 2's OpenAPI schema does not include a `status` block). The Go field is still declared so the type is complete for future Spec B work — the `json:"status,omitempty"` keeps the field absent from the wire when zero.

### 6d. `zz_generated.deepcopy.go`

This file is GENERATED. Do NOT write it by hand. After running `make generatek8s`, the file appears at `k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go`. The executor's job is to verify the file was generated (idempotency check below).

## 7. The example fixture + round-trip test

### 7a. `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml`

```yaml
apiVersion: task.benjamin-borbe.de/v1
kind: Schedule
metadata:
  name: weekly-review
  namespace: personal
spec:
  vault: personal
  title: Weekly Review
  schedule:
    recurrence: Weekly
    weekday: Saturday
  template:
    body: |
      Reflect on the past week.
      Plan the next.
    frontmatter:
      goals:
        - "[[Example Goal]]"
      priority: 2
      status: in_progress
      page_type: task
      assignee: bborbe
      recurring: Weekly
```

This is the canonical example from the design pin ("Weekly Review" task in the recurring-task inventory). Single source of truth — Prompt 2's `pkg/k8s_connector_test.go` re-uses the same content via the `//go:embed` pattern OR by reading the file from disk; whichever is simpler.

### 7b. `k8s/apis/task.benjamin-borbe.de/v1/example_test.go`

External `package v1_test`:

```go
package v1_test

import (
    "os"
    "path/filepath"
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "sigs.k8s.io/yaml"

    v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
)

// file path resolution: example_test.go is at k8s/apis/.../v1/example_test.go,
// testdata is at k8s/apis/.../v1/testdata/example.yaml. `testdata/` is the
// conventional Go testdata directory; `os.ReadFile("testdata/example.yaml")`
// resolves relative to the package's source directory.
const examplePath = "testdata/example.yaml"

var _ = Describe("example.yaml", func() {
    var (
        raw []byte
        sch v1.Schedule
    )

    BeforeEach(func() {
        path, err := filepath.Abs(examplePath)
        Expect(err).NotTo(HaveOccurred())
        raw, err = os.ReadFile(path)
        Expect(err).NotTo(HaveOccurred(), "read %s", path)
        Expect(raw).NotTo(BeEmpty())
    })

    It("parses as a canonical Schedule", func() {
        Expect(yaml.UnmarshalStrict(raw, &sch)).To(Succeed())
    })

    It("has the frozen apiVersion and kind", func() {
        Expect(yaml.UnmarshalStrict(raw, &sch)).To(Succeed())
        Expect(sch.APIVersion).To(Equal("task.benjamin-borbe.de/v1"))
        Expect(sch.Kind).To(Equal("Schedule"))
        Expect(sch.Name).To(Equal("weekly-review"))
        Expect(sch.Namespace).To(Equal("personal"))
    })

    It("round-trips every Spec field", func() {
        Expect(yaml.UnmarshalStrict(raw, &sch)).To(Succeed())
        Expect(sch.Spec.Vault).To(Equal("personal"))
        Expect(sch.Spec.Title).To(Equal("Weekly Review"))
        Expect(sch.Spec.Schedule.Recurrence).To(Equal("Weekly"))
        Expect(sch.Spec.Schedule.Weekday).To(Equal("Saturday"))
        Expect(sch.Spec.Template.Body).To(ContainSubstring("Reflect on the past week."))
        // sigs.k8s.io/yaml round-trips YAML through JSON, so YAML integers
        // become float64 in a map[string]interface{}; assert numerically.
        Expect(sch.Spec.Template.Frontmatter["priority"]).To(BeNumerically("==", 2))
        Expect(sch.Spec.Template.Frontmatter).To(HaveKeyWithValue("recurring", "Weekly"))
    })

    It("rejects an unknown field (strict-unmarshal guard)", func() {
        bad := []byte(`apiVersion: task.benjamin-borbe.de/v1
kind: Schedule
metadata:
  name: weekly-review
  namespace: personal
spec:
  vault: personal
  title: Weekly Review
  schedule:
    recurrence: Weekly
    weeday: Saturday
`)
        var s v1.Schedule
        err := yaml.UnmarshalStrict(bad, &s)
        Expect(err).To(HaveOccurred(), "strict-unmarshal must reject misspelled weekday field")
        Expect(err.Error()).To(ContainSubstring("weeday"))
    })
})

func TestSuite(t *testing.T) {
    // Ginkgo bootstrap — the package's suite runs without external test
    // packages so the test entrypoint is colocated with the specs.
    // Mirror the style of pkg/schedule/schedule_suite_test.go.
    RegisterFailHandler(Fail)
    suiteConfig, reporterConfig := GinkgoConfiguration()
    RunSpecs(t, "v1 Suite", suiteConfig, reporterConfig)
}
```

## 8. Idempotency check

After running `make generatek8s` once, the SECOND run must produce zero diff in `k8s/`:

```bash
cd /workspace && make generatek8s
cd /workspace && git diff --exit-code k8s/
```

This must exit 0. If it does not, inspect the diff — common causes:
- `controller-gen` annotation drift (re-run `go mod tidy` to refresh the version)
- A hand-edit to `zz_generated.deepcopy.go` (revert + re-run `make generatek8s`)
- A `tools.env` bump that changed the `kube_codegen.sh` output (this is a real version-drift signal — flag in the summary)

## 9. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased`:

```markdown
- feat: Add `k8s/apis/task.benjamin-borbe.de/v1` Go types for the `Schedule` CRD (group `task.benjamin-borbe.de`, kind `Schedule`, plural `schedules`, scope `Namespaced`, version `v1`); `make generatek8s` runs `kube_codegen.sh` to produce DeepCopy + clientset + informers + listers + applyconfiguration under `k8s/client/...`; add `testdata/example.yaml` fixture and a round-trip test that uses `sigs.k8s.io/yaml.UnmarshalStrict` to reject unknown fields
```

Two bullets if you want to be more granular (one for the types, one for the codegen wiring) — single bullet is fine.

## 10. Imports and conventions

- Generated `.go` files under `k8s/` use the Apache 2.0 header from `hack/boilerplate.go.txt`. The new `types.go` / `doc.go` / `register.go` files DO NOT have a hand-written header — they are inputs to the codegen pipeline, not generated artifacts, so they share the project's BSD header (`// Copyright (c) 2026 Benjamin Borbe...`) consistent with the rest of the repo.
- `goimports-reviser` style: standard library first, then third-party (alphabetical: `k8s.io/...`, `sigs.k8s.io/...`, `github.com/bborbe/...`), then internal.
- Use `github.com/bborbe/errors` for any error wrapping (Prompt 2 uses it for the connector — this prompt introduces no error paths).
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Do NOT touch `main.go`, `pkg/schedule/`, `pkg/publisher/`, `pkg/tick/`, `pkg/handler/`, `pkg/factory/`, or any existing `Makefile` target other than ADDING the `generatek8s` line and the `include` / path / variable order.
- Do NOT introduce `Schedule` CRs under `k8s/schedules/`. Only `k8s/apis/.../v1/testdata/example.yaml` is in this prompt.
- Do NOT add a Prometheus metric, an opt-out flag, a runtime config knob, an informer wiring, an admission webhook, or any per-schedule toggle. Spec Non-goals forbid all of these.

</requirements>

<constraints>
- The CRD group `task.benjamin-borbe.de`, version `v1`, kind `Schedule`, plural `schedules`, singular `schedule`, short name `ts`, scope `Namespaced` are FROZEN. Renames require a v2 + conversion webhook (out of scope per spec Non-goals).
- The Go types use `metav1.TypeMeta` + `metav1.ObjectMeta` + `runtime.Object` markers (`+genclient`, `+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object` on BOTH `Schedule` AND `ScheduleList`).
- `controller-gen` and `k8s.io/code-generator` are pinned via `tools.env` (NOT `tools.go` — the project migrated off `tools.go` in spec 004). The `K8S_CODEGEN_VERSION` and `CONTROLLER_GEN_VERSION` entries are the single source of truth.
- The codegen shell script `hack/update-codegen.sh` sources `kube_codegen.sh` from the vendored `k8s.io/code-generator` module via `go list -m -f '{{.Dir}}'`. `go mod vendor` is NOT a pre-codegen step (vendoring skips shell scripts).
- `make generatek8s` is intentionally separate from `make generate` (the existing counterfeiter target). It is NOT in `make precommit` — codegen is manual and the generated tree is stable day-to-day.
- The example fixture `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` round-trips through `sigs.k8s.io/yaml.UnmarshalStrict` to fail on unknown fields. A misspelled `weeday: Saturday` MUST cause the example test to fail.
- Generated `.go` files use the Apache 2.0 header from `hack/boilerplate.go.txt`. Hand-written source files (`types.go`, `doc.go`, `register.go`, `example_test.go`) use the project's BSD header (year 2026).
- Do NOT add a `Schedule` CRD to `k8s/schedules/`. Only the test fixture under `k8s/apis/.../testdata/` is in scope.
- Do NOT introduce an informer / `Listen` wiring, do NOT delete `pkg/schedule/inventory.go`, do NOT change `task.CreateCommand` or any field on it, do NOT add `v1alpha1` or any non-v1 version, do NOT add an admission webhook. Spec Non-goals forbid all of these.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

</constraints>

<verification>

From `/workspace`:

1. `go build ./...` — must compile.
2. `go test ./k8s/apis/task.benjamin-borbe.de/v1/...` — round-trip + strict-unmarshal specs green.
3. `make generatek8s && git diff --exit-code k8s/` — exit 0 (idempotent codegen).
4. `grep -nE 'type Schedule(Spec|Status|List)? struct' k8s/apis/task.benjamin-borbe.de/v1/types.go | wc -l` — exactly 4 matches.
5. `grep -nE '\+groupName=task\.benjamin-borbe\.de' k8s/apis/task.benjamin-borbe.de/v1/doc.go` — exactly 1 match.
6. `grep -RE 'package versioned|package externalversions|package v1' k8s/client/ | wc -l` — ≥ 3 matches (the client tree exists).
7. `ls k8s/apis/task.benjamin-borbe.de/v1/{doc.go,register.go,types.go,zz_generated.deepcopy.go}` — all four files present.
8. `ls k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` — file present.
9. `grep -c 'kind: Schedule' k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` — exactly 1.
10. `make precommit` — must exit 0.
11. `grep -nE 'ScheduleTrigger|ScheduleTemplate|Placeholders' k8s/apis/task.benjamin-borbe.de/v1/types.go` — must show the new types declared in spec section 6c.
12. `grep -nE '\+genclient|\+genclient:noStatus|\+k8s:deepcopy-gen:interfaces=' k8s/apis/task.benjamin-borbe.de/v1/types.go` — at least 3 matches (the client + deepcopy markers).
13. `cat /workspace/tools.env | grep -E 'K8S_CODEGEN_VERSION|CONTROLLER_GEN_VERSION'` — both new variables present.

</verification>

## Notes for the auditor

- **Spec gap surfaced (needs reviewer decision).** The spec's Constraints section says "pin [controller-gen] via `tools.go` like the other build deps" — this is OUTDATED. The project migrated off `tools.go` to `tools.env` in spec 004 (`/workspace/prompts/completed/004-migrate-tools-go.md`). This prompt uses the project-current `tools.env` + Makefile `@version` pattern. The spec should be amended in a follow-up; the prompt's chosen path is the only one that compiles against the current repo.
- **Spec gap surfaced (needs reviewer decision).** The spec's Desired Behavior #4 and the Suggested Decomposition mention `generate-internal-groups.sh`. This script is REMOVED upstream — its first line is literally `echo "ERROR: $(basename "$0") has been removed. ERROR: Please use k8s.io/code-generator/kube_codegen.sh instead."` The prompt uses the modern `kube_codegen.sh` + `kube::codegen::gen_helpers` / `kube::codegen::gen_client` path. Same follow-up applies — the spec should be amended.
- **Single decision tree.** `controller-gen` is added as a direct dep in `go.mod` even though only `kube::codegen::gen_helpers` runs in the script. The alternative is to keep `controller-gen` as a non-tracked tool (purely `go run`-on-demand from `CONTROLLER_GEN_VERSION`); the prompt picks the direct-dep path because `controller-gen` IS a transitive input to the same codegen pipeline (some projects use it for `object` / `rbac` generation, and adding a fresh `go run` to the script later would require a new `go get` and a `go mod tidy` round-trip). The cost is one extra direct dep entry; the benefit is "one `go get` and everything is wired".
- **Why `tools.env` over `tools.go` for this spec.** Spec 004 (`migrate-tools-go`) is committed and reviewed. Re-introducing `tools.go` even partially would re-trigger the `go.mod` pollution that spec 004 specifically removed. The prompt's pattern matches the rest of the repo.
- **AC coverage.** Prompt 1 covers AC 1, 2, 3, 4, 5, 10, 11, 14 per the spec's Suggested Decomposition (types, codegen, example fixture, round-trip test, `make precommit`). AC 6 (Go-built schema), AC 7 (`SetupCustomResourceDefinition` smoke), AC 8 (vault Pattern regex grep), AC 9 (CEL rule grep), AC 12 (Counterfeiter mock + Create/Update It-blocks), AC 13 (CEL validator over `scheduleSpecSchema()`) are covered in Prompt 2.
- **No `main.go` wiring.** This prompt does not call `SetupCustomResourceDefinition` from `main.go` or `cmd/run-once/main.go`. That wiring is the role of Spec B (per the spec's "DOES NOT introduce an informer" line and the spec's stated "applies it on every binary boot" — which is the wiring target, not this prompt's responsibility).

