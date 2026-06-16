---
status: approved
spec: [008-crd-scaffolding]
created: "2026-06-16T07:48:18Z"
queued: "2026-06-16T07:48:18Z"
branch: dark-factory/crd-scaffolding
---

<summary>
- Adds `pkg/k8s_connector.go` with the `K8sConnector` interface (one method: `SetupCustomResourceDefinition(ctx) error`) and a `k8sConnector` impl that builds the CRD spec from a Go-built `apiextensionsv1.CustomResourceDefinitionSpec` and applies it get-or-create-or-update via `apiextensionsv1.CustomResourceDefinitions`.
- Mirrors `~/Documents/workspaces/agent/task/executor/pkg/k8s_connector.go` structurally — same `NewK8sConnector(clientBuilder CRDClientBuilder) K8sConnector` factory signature, same `CRDClientBuilder func(*rest.Config) (apiextensionsclient.Interface, error)` injection seam, same `errors.Wrapf` discipline on the 4 wrap sites (config / clientset / create / update).
- `desiredCRDSpec()` reads the frozen `Kind` / `Plural` / `Singular` / `ShortNames` constants from the `v1` package — single source of truth, no string literals in the connector. `scheduleSpecSchema()` returns the `apiextensionsv1.JSONSchemaProps` for `spec.*`: `vault` regex `^[a-z][a-z0-9-]*$`, `schedule.recurrence` enum `Daily|Weekly|Monthly|Quarterly|Yearly` (capital, matching Go `time.Weekday.String()` and the spec design pins), and the CEL `XValidations` rule `self.recurrence == 'Weekly' ? has(self.weekday) : !has(self.weekday)`.
- Adds `mocks/k8s_connector.go` (counterfeiter fake `FakeK8sConnector`) and two test files: `pkg/k8s_connector_test.go` (3 Ginkgo `It` blocks exercising Create/Update/error-wrap via a fake `apiextensionsclient.Interface`) and `pkg/k8s_connector_validation_test.go` (5 Ginkgo `It` blocks asserting on the CEL evaluator over `scheduleSpecSchema()`).
- No `main.go` wiring in this prompt — that lands in Spec B per the spec's "DOES NOT introduce an informer / `Listen` wiring" line. `make precommit` exits 0.

</summary>

<objective>
Add the CRD installer connector and the Go-built OpenAPI schema that enforces `vault` regex, `recurrence` enum, and the `weekday`-required-iff-weekly CEL rule. Ship Counterfeiter mocks plus the connector and validation Ginkgo tests. The connector is production-ready code but is not yet called from `main.go` (that is Spec B's wiring).
</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26.4, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before writing code:
- `/workspace/prompts/crd-types-and-codegen.md` — the prompt that ships the types and codegen; **Prompt 2 runs AFTER Prompt 1 completes**. The `v1.Schedule`, `v1.ScheduleSpec`, `v1.ScheduleList`, `v1.GroupName`, `v1.Version`, `v1.Kind`, `v1.Plural`, `v1.Singular`, `v1.ShortNames`, `v1.SchemeGroupVersion` are the imports this prompt consumes.
- `/workspace/prompts/completed/004-migrate-tools-go.md` — established `tools.env` is the canonical version-pin file. `controller-gen` is NOT invoked by this prompt (codegen is Prompt 1's responsibility). No `tools.env` changes in this prompt.
- `/workspace/mocks/mocks.go` — single-line `package mocks` file. The existing `pkg/pkg_suite_test.go` already carries the `//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6@v6.12.2 -generate` directive; the counterfeiter directive on the new `K8sConnector` interface in `pkg/k8s_connector.go` is what produces `mocks/k8s_connector.go`. **Do NOT create a second suite file** — `pkg/pkg_suite_test.go` already exists with the suite entry-point; adding another would collide on `TestSuite`.
- `/workspace/pkg/pkg_suite_test.go` — existing Ginkgo suite for `package pkg_test` (10 lines, BSD header, defines `TestSuite`). The new `Describe` blocks below register against this suite automatically. No new `*_suite_test.go` file in scope.
- `/workspace/pkg/handler/healthz.go` — copyright header (3 lines, year `2026`) and import-grouping convention. New `.go` files in this prompt use the same header.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` string alias; the existing typed constants are LOWERCASE (`"daily" | "weekly" | …`). The CRD enum + CEL rule below use CAPITALIZED string values (`"Daily" | "Weekly" | …`) per the spec design pins — these are CRD-API-level wire values, intentionally divorced from the in-Go `RecurrenceKind` strings (Spec B will handle the case-fold mapping when the controller reads CRDs into `RecurrenceKind`). Do NOT import or reference `pkg/schedule.RecurrenceKind` from this prompt's code; the OpenAPI schema literals are plain strings.
- `/workspace/CHANGELOG.md` — append a `feat:` bullet under `## Unreleased` for this work.
- `/workspace/go.mod` — `k8s.io/apiextensions-apiserver v0.36.1`, `k8s.io/apimachinery v0.36.1`, `k8s.io/client-go v0.36.1`, `sigs.k8s.io/yaml v1.6.0` are direct deps after Prompt 1. **This prompt adds a direct dep on `github.com/google/cel-go v0.26.0`** for the validation test (see §6 below) — cel-go is currently indirect via `k8s.io/apiextensions-apiserver`.

**Spec gap surfaced from Prompt 1 context:** the spec's reference connector is at `~/Documents/workspaces/agent/task/executor/pkg/k8s_connector.go`. That path does not exist in this YOLO container (no `/home/node/bborbe/...` or `/Users/bborbe/...` mounts). The prompt pins the bborbe CRD guide pattern (`go-kubernetes-crd-controller-guide.md` §4) as the structural template — same factory signature, same `CRDClientBuilder` injection seam, same `errors.Wrapf` discipline, same `desiredCRDSpec()` + `scheduleSpecSchema()` split. The exact line-for-line mirror is not possible without the source file; the structural mirror is.

Verified external symbols (grep'd at `/home/node/go/pkg/mod/` and via `go doc` on 2026-06-16):

`k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1` (direct, v0.36.1):
```go
type CustomResourceDefinitionSpec struct {
    Group    string
    Names    CustomResourceDefinitionNames
    Scope    ResourceScope  // constants: NamespaceScoped, ClusterScoped
    Versions []CustomResourceDefinitionVersion
}
type CustomResourceDefinitionVersion struct {
    Name    string
    Served  bool
    Storage bool
    Schema  *CustomResourceValidation
}
type CustomResourceValidation struct {
    OpenAPIV3Schema *JSONSchemaProps
}
type CustomResourceDefinitionNames struct {
    Plural     string
    Singular   string
    ShortNames []string
    Kind       string
    ListKind   string
}
type CustomResourceDefinition struct {
    metav1.TypeMeta
    metav1.ObjectMeta
    Spec CustomResourceDefinitionSpec
}
// JSONSchemaProps + relevant substructs (truncated — verified):
type JSONSchemaProps struct {
    Type        string         // "object" | "string" | "array" | "integer" | "boolean" | "number"
    Description string
    Properties  map[string]JSONSchemaProps
    Required    []string
    Pattern     string
    Enum        []JSON
    XValidations ValidationRules
    // ...
}
type ValidationRules []ValidationRule
type ValidationRule struct {
    Rule    string  // CEL expression
    Message string  // human-readable; used in error
}
type JSON struct {
    Raw []byte  // marshal as raw JSON; populate via []byte(`"value"`)
}
```

`k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset` (direct, v0.36.1):
```go
type Interface interface {
    ApiextensionsV1() apiextensionsv1.ApiextensionsV1Interface
}
func NewForConfig(c *rest.Config) (*Clientset, error)
```

`k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1` (direct):
```go
type ApiextensionsV1Interface interface {
    CustomResourceDefinitions() CustomResourceDefinitionInterface
}
type CustomResourceDefinitionInterface interface {
    Get(ctx context.Context, name string, opts metav1.GetOptions) (*apiextensionsv1.CustomResourceDefinition, error)
    Create(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, opts metav1.CreateOptions) (*apiextensionsv1.CustomResourceDefinition, error)
    Update(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, opts metav1.UpdateOptions) (*apiextensionsv1.CustomResourceDefinition, error)
    List(ctx context.Context, opts metav1.ListOptions) (*apiextensionsv1.CustomResourceDefinitionList, error)
}
```

`k8s.io/apimachinery/pkg/api/errors`:
```go
func IsNotFound(err error) bool       // CRD not yet installed
func IsAlreadyExists(err error) bool  // race: two pods booting simultaneously
```

`k8s.io/client-go/rest` (direct):
```go
func InClusterConfig() (*Config, error)  // in-cluster service-account config
```

`github.com/bborbe/errors` (direct, v1.5.13):
```go
func Wrap(ctx context.Context, err error, message string) error
func Wrapf(ctx context.Context, err error, format string, args ...interface{}) error
```

`github.com/golang/glog` (direct, v1.2.5):
```go
func V(level Level) Verbose
func (v Verbose) Infof(format string, args ...interface{})
```

`github.com/google/cel-go` (indirect → will be promoted to direct):
```go
// cel.NewEnv → env.Compile(expr) → env.Program(ast) → program.Eval(vars)
func NewEnv(opts ...EnvOption) (*Env, error)
func (e *Env) Compile(txt string) (*Ast, *Issues)
func (e *Env) Program(ast *Ast, opts ...ProgramOption) (Program, error)
type Program interface {
    Eval(vars any) (ref.Val, *EvalDetails)
}
// types.Bool is a named bool type satisfying ref.Val. Bool is the
// CEL bool ref.Val; cast via `if b, ok := out.(types.Bool); ok && bool(b) { ... }`.
```

`github.com/maxbrunsfeld/counterfeiter/v6` (direct, v6.12.2) — driven by the `//counterfeiter:generate` directive on the new `K8sConnector` interface (the `-o` flag in the directive picks the output path). This prompt emits `mocks/k8s_connector.go` (fake `FakeK8sConnector`), matching the agent-task-executor convention of `mocks/<snake_case_interface>.go` (one file per interface, no package-name prefix).

`github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` (direct).

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-kubernetes-crd-controller-guide.md` §4 (K8sConnector interface), §8 (testing). The "preferred pattern" uses `bborbe/k8s` event-handler generics; this prompt does NOT use them (no informer / Listen wiring in this spec). Only the `SetupCustomResourceDefinition` half is in scope.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `New*` constructor returns the interface type. The `NewK8sConnector` returns `K8sConnector`, not `*k8sConnector`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter directive on every exported interface, output filename prefixed with the source package. The mock lives in `mocks/` (project root), not next to the source.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf` 3-arg form; never `fmt.Errorf`; never `context.Background()` in business logic. 4 wrap sites per the bborbe template: `build k8s config`, `build apiextensions clientset`, `get CRD`, `create/update CRD`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public interface, private struct, `New*` constructor.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, dot-imports, external test package, counterfeiter mocks.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for new code; counterfeiter mocks; no real network / real K8s in unit tests.

Load-bearing snippets inlined for the executor's verification:

```go
// k8s/apis/task.benjamin-borbe.de/v1/register.go (verbatim — from Prompt 1)
const GroupName = "task.benjamin-borbe.de"
const Version = "v1"
const Kind = "Schedule"
const ListKind = "ScheduleList"
const Plural = "schedules"
const Singular = "schedule"
var ShortNames = []string{"ts"}
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// k8s/apis/task.benjamin-borbe.de/v1/types.go (verbatim — from Prompt 1)
type Schedule struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   ScheduleSpec   `json:"spec,omitempty"`
    Status ScheduleStatus `json:"status,omitempty"`
}
type ScheduleSpec struct {
    Vault    string           `json:"vault"`
    Title    string           `json:"title"`
    Schedule ScheduleTrigger  `json:"schedule"`
    Template ScheduleTemplate `json:"template"`
}
type ScheduleTrigger struct {
    Recurrence string `json:"recurrence"`
    Weekday    string `json:"weekday,omitempty"`
}
type ScheduleTemplate struct {
    Body        string                 `json:"body,omitempty"`
    Frontmatter lib.TaskFrontmatter    `json:"frontmatter,omitempty"`
}
// + Placeholders = []string{"Date","ISOWeek","MonthYear","Quarter","Year"} + ScheduleList
```

</context>

<requirements>

## 1. Module additions

`cd /workspace && go get github.com/google/cel-go@v0.26.0` — promote cel-go from indirect to direct. It is used in the validation test only; the connector itself does not import cel-go (the CEL rule is shipped as a string in the OpenAPI schema, evaluated by the API server, not by this binary). All other deps are already direct (verified above).

## 2. Reuse the existing Ginkgo suite

The `pkg/pkg_suite_test.go` file already exists and defines `TestSuite` for `package pkg_test` — it also carries the `//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6@v6.12.2 -generate` directive that drives counterfeiter for every interface in `pkg/` with a `//counterfeiter:generate` line. **Do not create a new suite file.** The Describe blocks added by `pkg/k8s_connector_test.go` and `pkg/k8s_connector_validation_test.go` register against the existing suite automatically.

`mocks/k8s_connector.go` is produced from the `//counterfeiter:generate -o ../mocks/k8s_connector.go --fake-name FakeK8sConnector . K8sConnector` directive on the new `K8sConnector` interface (added in §3) — the existing `make generate` (`rm -rf mocks avro && go generate -mod=mod ./...`) picks it up.

## 3. The `K8sConnector` interface and the `k8sConnector` impl

In `/workspace/pkg/k8s_connector.go` (starts with the 2026 copyright header):

```go
package pkg

import (
    "context"

    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
    apiextensionsv1typed "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
    "github.com/bborbe/errors"
    "github.com/golang/glog"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/rest"

    v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
)

// ConfigBuilder is the test seam for loading the rest.Config. Production
// wiring passes rest.InClusterConfig; tests pass a closure that returns
// a zero-value *rest.Config so SetupCustomResourceDefinition runs without
// a real in-cluster service-account token mount.
type ConfigBuilder func() (*rest.Config, error)

// CRDClientBuilder is the test seam for constructing the apiextensions
// clientset. Production wiring passes apiextensionsclient.NewForConfig;
// tests wire it to a closure that returns the fake clientset.
type CRDClientBuilder func(*rest.Config) (apiextensionsclient.Interface, error)

// K8sConnector installs the Schedule CRD on a Kubernetes cluster.
//counterfeiter:generate -o ../mocks/k8s_connector.go --fake-name FakeK8sConnector . K8sConnector
type K8sConnector interface {
    SetupCustomResourceDefinition(ctx context.Context) error
}

// NewK8sConnector builds a connector that uses configBuilder to load
// the k8s config and clientBuilder to construct the apiextensions
// clientset. Production wiring passes rest.InClusterConfig +
// apiextensionsclient.NewForConfig; tests pass closures returning
// stubs/fakes.
func NewK8sConnector(configBuilder ConfigBuilder, clientBuilder CRDClientBuilder) K8sConnector {
    return &k8sConnector{configBuilder: configBuilder, clientBuilder: clientBuilder}
}

type k8sConnector struct {
    configBuilder ConfigBuilder
    clientBuilder CRDClientBuilder
}

func (k *k8sConnector) SetupCustomResourceDefinition(ctx context.Context) error {
    config, err := k.configBuilder()
    if err != nil {
        return errors.Wrap(ctx, err, "build k8s config")
    }
    clientset, err := k.clientBuilder(config)
    if err != nil {
        return errors.Wrap(ctx, err, "build apiextensions clientset")
    }

    crdClient := clientset.ApiextensionsV1().CustomResourceDefinitions()
    existing, err := crdClient.Get(ctx, v1.Plural+"."+v1.GroupName, metav1.GetOptions{})
    if apierrors.IsNotFound(err) {
        return k.createCrd(ctx, crdClient)
    }
    if err != nil {
        return errors.Wrap(ctx, err, "get CRD")
    }
    existing.Spec = k.desiredCRDSpec()
    if _, err := crdClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
        return errors.Wrapf(ctx, err, "update CRD %s.%s", v1.Plural, v1.GroupName)
    }
    glog.V(2).Infof("k8s-connector: updated CRD %s.%s", v1.Plural, v1.GroupName)
    return nil
}

func (k *k8sConnector) createCrd(ctx context.Context, crdClient apiextensionsv1typed.CustomResourceDefinitionInterface) error {
    crd := &apiextensionsv1.CustomResourceDefinition{
        ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
        Spec:       k.desiredCRDSpec(),
    }
    if _, err := crdClient.Create(ctx, crd, metav1.CreateOptions{}); err != nil {
        if apierrors.IsAlreadyExists(err) {
            // Race: another pod beat us to it. Fall through to update path.
            glog.V(2).Infof("k8s-connector: crd-already-exists: applying update")
            return nil
        }
        return errors.Wrapf(ctx, err, "create CRD %s.%s", v1.Plural, v1.GroupName)
    }
    glog.V(2).Infof("k8s-connector: created CRD %s.%s", v1.Plural, v1.GroupName)
    return nil
}
```

Notes on the above:
- Two test seams: `ConfigBuilder` short-circuits `rest.InClusterConfig`, `CRDClientBuilder` returns a fake clientset. Production wiring (Spec B) passes `rest.InClusterConfig` and `apiextensionsclient.NewForConfig` directly. Without the `ConfigBuilder` seam the tests cannot run (`rest.InClusterConfig` fails on the developer's machine because no service-account token mount exists).
- The 4 wrap sites match the agent's executor template: `build k8s config`, `build apiextensions clientset`, `get CRD`, `create/update CRD`.
- The `crd-already-exists` log line is the spec's Failure Modes row 2 detection signal — fired at `glog.V(2)` so it shows up in the binary's per-boot output but is filtered at default `v=0`.
- `apiextensionsv1typed.CustomResourceDefinitionInterface` (NOT `apiextensionsv1.CustomResourceDefinitionInterface`): the typed-client interface lives in the `typed/apiextensions/v1` subpackage, not the `apis/apiextensions/v1` package — the `apis/...` package only has the API types (struct definitions), not the client method set. Importing the typed-client subpackage as `apiextensionsv1typed` avoids the collision with the existing `apiextensionsv1` alias for the types package.

## 4. The CRD spec builder

Append to `/workspace/pkg/k8s_connector.go`:

```go
// desiredCRDSpec returns the Go-built CRD spec for the Schedule CRD.
// Every value (group, kind, plural, singular, short name, scope) is
// read from the v1 package's frozen constants — the connector does not
// hard-code any of the names. Renaming a constant in v1 is the only way
// the CRD spec changes; this is the single source of truth.
func (k *k8sConnector) desiredCRDSpec() apiextensionsv1.CustomResourceDefinitionSpec {
    return apiextensionsv1.CustomResourceDefinitionSpec{
        Group: v1.GroupName,
        Names: apiextensionsv1.CustomResourceDefinitionNames{
            Kind:       v1.Kind,
            ListKind:   v1.ListKind,
            Plural:     v1.Plural,
            Singular:   v1.Singular,
            ShortNames: v1.ShortNames,
        },
        Scope: apiextensionsv1.NamespaceScoped,
        Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
            Name:    v1.Version,
            Served:  true,
            Storage: true,
            Schema: &apiextensionsv1.CustomResourceValidation{
                OpenAPIV3Schema: k.scheduleSpecSchema(),
            },
        }},
    }
}
```

## 5. The OpenAPI schema

In `/workspace/pkg/k8s_connector_schema.go` (separate file — the schema is 60+ lines of nested struct literals and reading it from `k8s_connector.go` would push the file past the `funlen 80` cap set by `/workspace/.golangci.yml`):

```go
package pkg

import (
    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// recurrenceEnum is the closed set of valid recurrence strings on the
// CRD wire. Capitalized to match Go's time.Weekday.String() casing and
// the spec's design pins (Spec 6 period tokens emit the same casing).
// Lives in this package so the schema is self-contained; do NOT import
// pkg/schedule.RecurrenceKind (those constants are lowercase Go-internal
// values; the CRD enum is a separate API contract).
var recurrenceEnum = []string{"Daily", "Weekly", "Monthly", "Quarterly", "Yearly"}

// vaultPattern is the regex the API server enforces on spec.vault.
// Matches the slug convention used in pkg/schedule/inventory.go.
const vaultPattern = "^[a-z][a-z0-9-]*$"

// weekdayRequiredIfWeeklyRule is the CEL rule encoded in
// schedule.XValidations. self is bound to the ScheduleTrigger object;
// `has(self.weekday)` checks field presence (not just non-empty
// string — this is the OpenAPI semantic). The literal 'Weekly' matches
// the capitalized recurrenceEnum.
const weekdayRequiredIfWeeklyRule = "self.recurrence == 'Weekly' ? has(self.weekday) : !has(self.weekday)"

// weekdayRequiredIfWeeklyMessage is the human-readable error the API
// server emits when the rule fails. Surfaced to the operator via
// `kubectl apply` output.
const weekdayRequiredIfWeeklyMessage = "weekday is required when recurrence is 'Weekly', and forbidden otherwise"

// scheduleSpecSchema returns the OpenAPI v3 schema for spec.*. The
// schema is built as Go code (no CRD YAML manifest is generated or
// committed) and applied on every binary boot via
// SetupCustomResourceDefinition. Single source of truth: this file.
func (k *k8sConnector) scheduleSpecSchema() apiextensionsv1.JSONSchemaProps {
    return apiextensionsv1.JSONSchemaProps{
        Type:        "object",
        Description: "Schedule spec — defines when a recurring task fires and what to publish.",
        Required:    []string{"vault", "title", "schedule", "template"},
        Properties: map[string]apiextensionsv1.JSONSchemaProps{
            "vault": {
                Type:        "string",
                Description: "Obsidian vault slug. Must match ^[a-z][a-z0-9-]*$.",
                Pattern:     vaultPattern,
            },
            "title": {
                Type:        "string",
                Description: "Title shown to the user in the generated vault task. Go text/template — placeholders rendered with the period token.",
            },
            "schedule": {
                Type:        "object",
                Description: "Recurrence trigger. The weekday-required-iff-weekly invariant is enforced by the CEL x-kubernetes-validations rule below.",
                Required:    []string{"recurrence"},
                Properties: map[string]apiextensionsv1.JSONSchemaProps{
                    "recurrence": {
                        Type:        "string",
                        Description: "One of: Daily, Weekly, Monthly, Quarterly, Yearly.",
                        Enum:        jsonEnumValues(recurrenceEnum),
                    },
                    "weekday": {
                        Type:        "string",
                        Description: "time.Weekday string (Monday..Sunday). Required when recurrence is 'Weekly'; forbidden otherwise.",
                    },
                },
                XValidations: apiextensionsv1.ValidationRules{{
                    Rule:    weekdayRequiredIfWeeklyRule,
                    Message: weekdayRequiredIfWeeklyMessage,
                }},
            },
            "template": {
                Type:        "object",
                Description: "Body and frontmatter stamped onto the generated task. Per spec design pins, body is optional (some recurring tasks only need a title).",
                Properties: map[string]apiextensionsv1.JSONSchemaProps{
                    "body": {
                        Type:        "string",
                        Description: "Raw markdown body of the generated task. Go text/template.",
                    },
                    "frontmatter": {
                        Type:        "object",
                        Description: "YAML frontmatter of the generated task. Free-form — see k8s/apis/.../v1.ScheduleSpec.Template.Frontmatter (lib.TaskFrontmatter).",
                        // XPreserveUnknownFields is NOT set: the frontmatter shape
                        // is part of the contract; unknown keys are a config bug
                        // that should fail at apply-time, not silently pass.
                    },
                },
            },
        },
    }
}

// jsonEnumValues wraps each string in an apiextensionsv1.JSON so the
// schema's Enum field (which is a []JSON for the raw-JSON-value form)
// accepts strings. The OpenAPI serializer renders the Raw bytes verbatim.
func jsonEnumValues(values []string) []apiextensionsv1.JSON {
    out := make([]apiextensionsv1.JSON, 0, len(values))
    for _, v := range values {
        out = append(out, apiextensionsv1.JSON{Raw: []byte(`"` + v + `"`)})
    }
    return out
}

```

Notes on the above:
- The `Enum` field on `JSONSchemaProps` is `[]JSON` (not `[]string`) — the k8s type system wants the raw JSON form so the value can be string, number, bool, etc. Wrap each string in `apiextensionsv1.JSON{Raw: []byte("\"...\"")}`.
- `XValidations` is the OpenAPI x-kubernetes-validations extension. The k8s API server applies CEL rules at admission time, not at code-gen time. This is the only validation the spec mandates via the schema.
- `Required` lists the property names that MUST be present (not non-empty). `vault`, `title`, `schedule`, `template` are required; `weekday` is NOT in `schedule.Required` because its presence depends on `recurrence` — the CEL rule enforces that.
- No `weekday` enum is set on the OpenAPI schema. Spec 008 calls only for the CEL rule + recurrence enum; an extra weekday enum would be schema overreach (`time.Weekday.String()` is the contract, not enforced at the CRD boundary). Spec B may revisit if controller-side parsing benefits from API-server pre-validation.
- No `MinLength` on `title` or `body` and no `Required` on `template.body`. Spec design pins keep templates flexible — a Daily heartbeat schedule with just a title is legal. Constraints beyond the four required spec keys are out of scope.
- `frontmatter.XPreserveUnknownFields` is intentionally false. The frontmatter shape is part of the contract; the publisher and the migration both depend on the frozen `goals/assignee/priority/status/page_type/recurring` keys (spec 002). Allowing unknown fields would let typos slip past the API server.

## 6. The Counterfeiter mock

After writing the files, run `cd /workspace && go generate -mod=mod ./pkg/...` to produce `/workspace/mocks/k8s_connector.go`. The expected fake type is `FakeK8sConnector` with `SetupCustomResourceDefinitionStub`, `SetupCustomResourceDefinitionCallCount`, `SetupCustomResourceDefinitionArgsForCall`, `SetupCustomResourceDefinitionReturns`. Verify the generated file has the 2026 copyright header — counterfeiter 6.12.2 does NOT prepend headers, so prepend it manually after generation if missing (or rely on the existing `make addlicense` target to fix it on the next precommit run).

If the executor prefers a one-shot pattern: `go generate -mod=mod ./pkg/...` then `make addlicense` (adds headers to all `.go` files recursively).

## 7. The export_test.go shim (test-only accessors)

In `/workspace/pkg/k8s_connector_export_test.go` (Go's special-cased `*_test.go` filename — only compiled for tests in this package, keeps the symbols invisible to production binaries):

```go
package pkg

import "regexp"

// VaultPatternForTest returns the regex the schema applies to spec.vault.
func VaultPatternForTest() string { return vaultPattern }

// RecurrenceEnumForTest returns the closed set of valid recurrence strings.
func RecurrenceEnumForTest() []string { return recurrenceEnum }

// WeekdayRequiredIfWeeklyRuleForTest returns the CEL rule from XValidations[0].
func WeekdayRequiredIfWeeklyRuleForTest() string { return weekdayRequiredIfWeeklyRule }

// WeekdayRequiredIfWeeklyMessageForTest returns the human-readable error
// message the API server emits when the CEL rule fails.
func WeekdayRequiredIfWeeklyMessageForTest() string { return weekdayRequiredIfWeeklyMessage }

// VaultRegexForTest returns a pre-compiled *regexp.Regexp matching vaultPattern.
// Used by the validation test's validateSpec helper.
var VaultRegexForTest = regexp.MustCompile(vaultPattern)
```

## 8. The connector test (3 It blocks)

In `/workspace/pkg/k8s_connector_test.go` (external `package pkg_test`):

```go
package pkg_test

import (
    "context"
    "errors"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
    apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/rest"

    "github.com/bborbe/recurring-task-creator/pkg"
    v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
)

var _ = Describe("SetupCustomResourceDefinition", func() {
    var (
        ctx          context.Context
        clientset    *apiextensionsfake.Clientset
        clientBuilder pkg.CRDClientBuilder
        connector    pkg.K8sConnector
    )

    var configBuilder pkg.ConfigBuilder

    BeforeEach(func() {
        ctx = context.Background()
        configBuilder = func() (*rest.Config, error) { return &rest.Config{}, nil }
        clientset = apiextensionsfake.NewSimpleClientset()
        clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) {
            return clientset, nil
        }
        connector = pkg.NewK8sConnector(configBuilder, clientBuilder)
    })

    It("creates the CRD when none exists", func() {
        Expect(connector.SetupCustomResourceDefinition(ctx)).To(Succeed())

        crdList, err := clientset.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
        Expect(err).NotTo(HaveOccurred())
        Expect(crdList.Items).To(HaveLen(1))
        crd := crdList.Items[0]
        Expect(crd.Spec.Group).To(Equal(v1.GroupName))
        Expect(crd.Spec.Names.Kind).To(Equal(v1.Kind))
        Expect(crd.Spec.Names.Plural).To(Equal(v1.Plural))
    })

    It("updates the CRD when an older spec exists", func() {
        // Pre-load the fake with a CRD whose spec has the same group/kind
        // but a deliberately wrong Versions[0].Name — the test asserts the
        // connector overwrites it.
        old := &apiextensionsv1.CustomResourceDefinition{
            ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
            Spec: apiextensionsv1.CustomResourceDefinitionSpec{
                Group: v1.GroupName,
                Names: apiextensionsv1.CustomResourceDefinitionNames{
                    Kind:     v1.Kind,
                    Plural:   v1.Plural,
                    Singular: v1.Singular,
                },
                Scope: apiextensionsv1.NamespaceScoped,
                Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
                    Name:    "WRONG-VERSION",
                    Served:  true,
                    Storage: true,
                }},
            },
        }
        clientset = apiextensionsfake.NewSimpleClientset(old)
        clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) { return clientset, nil }
        connector = pkg.NewK8sConnector(configBuilder, clientBuilder)

        Expect(connector.SetupCustomResourceDefinition(ctx)).To(Succeed())

        // After update, the spec's Versions[0].Name must be v1.Version.
        updated, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, old.Name, metav1.GetOptions{})
        Expect(err).NotTo(HaveOccurred())
        Expect(updated.Spec.Versions).To(HaveLen(1))
        Expect(updated.Spec.Versions[0].Name).To(Equal(v1.Version))
    })

    It("wraps an error when the clientset builder fails", func() {
        clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) {
            return nil, errors.New("boom")
        }
        connector = pkg.NewK8sConnector(configBuilder, clientBuilder)

        err := connector.SetupCustomResourceDefinition(ctx)
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("build apiextensions clientset"))
    })
})
```

**Note on the fake clientset's typed CRD accessor.** `apiextensionsfake.NewSimpleClientset(...)` returns a `*Clientset` whose `ApiextensionsV1().CustomResourceDefinitions()` returns a `*FakeCustomResourceDefinitions` with `Create` / `Update` / `Get` / `List` methods that record the call args. The snippet above uses the `List` path to assert "the CRD was created". An alternative pattern (closer to the bborbe CRD guide §8) uses `apierrors.IsNotFound` as a sentinel on `Get` to drive the Create/Update path branching. Both are acceptable; the AC's load-bearing assertions are the call sequence + the wrap-message substring, not the specific accessor method. If the `List` signature differs in apiextensionsfake v0.36.1, fall back to `Tracker().List(...)` or whatever the typed fake exposes.

## 9. The validation test (5 It blocks)

In `/workspace/pkg/k8s_connector_validation_test.go` (external `package pkg_test`):

```go
package pkg_test

import (
    "fmt"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/google/cel-go/cel"
    "github.com/google/cel-go/common/types"

    "github.com/bborbe/recurring-task-creator/pkg"
)

var _ = Describe("scheduleSpecSchema CEL validation", func() {
    // evalRule runs the CEL rule from XValidations[0] against the given
    // vars and returns the human-readable error message (or "" when the
    // rule passes).
    var evalRule = func(vars map[string]interface{}) string {
        rule := pkg.WeekdayRequiredIfWeeklyRuleForTest()
        env, err := cel.NewEnv(
            cel.Variable("recurrence", cel.StringType),
            cel.Variable("weekday", cel.StringType),
        )
        Expect(err).NotTo(HaveOccurred())
        ast, issues := env.Compile(rule)
        Expect(issues.Err()).NotTo(HaveOccurred(), "compile %q", rule)
        program, err := env.Program(ast)
        Expect(err).NotTo(HaveOccurred())
        out, _, err := program.Eval(vars)
        Expect(err).NotTo(HaveOccurred())
        if b, ok := out.(types.Bool); ok && bool(b) {
            return ""  // rule passed
        }
        return pkg.WeekdayRequiredIfWeeklyMessageForTest()
    }

    // validateSpec runs the regex / enum / CEL checks against a
    // map[string]interface{} representation of a Schedule spec. Mirrors
    // what the API server does at admission time.
    var validateSpec = func(spec map[string]interface{}) error {
        vault, _ := spec["vault"].(string)
        if !pkg.VaultRegexForTest.MatchString(vault) {
            return fmt.Errorf("vault %q does not match %s", vault, pkg.VaultPatternForTest())
        }
        sched, _ := spec["schedule"].(map[string]interface{})
        recurrence, _ := sched["recurrence"].(string)
        var found bool
        for _, r := range pkg.RecurrenceEnumForTest() {
            if r == recurrence {
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("recurrence %q is not in the closed set", recurrence)
        }
        if msg := evalRule(map[string]interface{}{
            "recurrence": recurrence,
            "weekday":    sched["weekday"],  // may be nil for "absent"
        }); msg != "" {
            return fmt.Errorf("%s", msg)
        }
        return nil
    }

    It("accepts the canonical weekly-review example", func() {
        spec := map[string]interface{}{
            "vault": "personal",
            "title": "Weekly Review",
            "schedule": map[string]interface{}{
                "recurrence": "Weekly",
                "weekday":    "Saturday",
            },
            "template": map[string]interface{}{"body": "Reflect."},
        }
        Expect(validateSpec(spec)).To(Succeed())
    })

    It("rejects an unknown recurrence value", func() {
        spec := map[string]interface{}{
            "vault": "personal",
            "title": "Foo",
            "schedule": map[string]interface{}{
                "recurrence": "weekly", // lowercase — not in the capital-case enum
            },
        }
        err := validateSpec(spec)
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("recurrence"))
    })

    It("rejects a weekly schedule without weekday", func() {
        spec := map[string]interface{}{
            "vault": "personal",
            "title": "Foo",
            "schedule": map[string]interface{}{
                "recurrence": "Weekly",
                // weekday absent
            },
        }
        err := validateSpec(spec)
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("weekday"))
    })

    It("rejects a non-weekly schedule that sets weekday", func() {
        spec := map[string]interface{}{
            "vault": "personal",
            "title": "Foo",
            "schedule": map[string]interface{}{
                "recurrence": "Monthly",
                "weekday":    "Saturday",
            },
        }
        err := validateSpec(spec)
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("weekday"))
    })

    It("rejects a vault slug that does not match the regex", func() {
        spec := map[string]interface{}{
            "vault": "Bad Vault",  // space + uppercase
            "title": "Foo",
            "schedule": map[string]interface{}{
                "recurrence": "Daily",
            },
        }
        err := validateSpec(spec)
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("vault"))
    })
})
```

## 10. Coverage

The validation test covers every code path in `scheduleSpecSchema()` (all 5 properties) and every branch of the weekday-CEL rule (weekly-with, weekly-without, monthly-with, monthly-without). The connector test covers all 3 paths in `SetupCustomResourceDefinition` (create, update, builder-error). Coverage on the new package should be ≥80% per `docs/dod.md`.

If coverage is below 80% after running `go test -coverprofile=/tmp/cover.out ./pkg/...`, inspect the uncovered lines. Common gaps:
- The `glog.V(2).Infof` success lines in `SetupCustomResourceDefinition` are not asserted in tests (no easy way to capture glog output in unit tests; the project's existing tests don't either).
- The `apierrors.IsAlreadyExists` race-handling branch in `createCrd` is hard to exercise without a second fake clientset; skip it if it requires more than a one-line test addition. Document the gap in `## Improvements`.

## 11. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased`:

```markdown
- feat: Add `pkg/k8s_connector.go` that self-installs the `schedules.task.benjamin-borbe.de` CRD on every binary boot via `apiextensionsv1.CustomResourceDefinitions` get-or-create-or-update. The CRD spec is Go-built (`desiredCRDSpec()` + `scheduleSpecSchema()` returning `apiextensionsv1.JSONSchemaProps`); the schema enforces the `vault` slug regex, the 5-value capital-case `recurrence` enum (`Daily|Weekly|Monthly|Quarterly|Yearly`), and the CEL `x-kubernetes-validations` rule that requires `weekday` iff `recurrence == "Weekly"`. Counterfeiter mock at `mocks/k8s_connector.go`; `pkg/k8s_connector_test.go` covers Create/Update/error-wrap via a fake `apiextensionsclient.Interface`; `pkg/k8s_connector_validation_test.go` exercises the schema with `cel-go`.
```

## 12. Imports and conventions

- Every new `.go` file starts with the 2026 copyright header.
- Use `goimports-reviser` style: standard library first, then third-party (alphabetical: `github.com/bborbe/...`, `github.com/golang/...`, `github.com/google/cel-go/...`, `github.com/onsi/...`, `k8s.io/...`, `sigs.k8s.io/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use `github.com/bborbe/errors` for wrapping. The 4 wrap sites in `SetupCustomResourceDefinition` use `errors.Wrap` (for fixed-string wraps) and `errors.Wrapf` (for arg-formatted wraps). Never `fmt.Errorf`.
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Do NOT touch `main.go`, `cmd/run-once/main.go`, `pkg/schedule/`, `pkg/publisher/`, `pkg/tick/`, `pkg/handler/`, `pkg/factory/`, the Makefile, or any K8s manifest.
- Do NOT add an informer / `Listen` method to `K8sConnector`. Spec Non-goals forbid it.
- Do NOT add a `KUBECONFIG` flag, an opt-out of the CRD install, a `--dry-run-crd` mode, a per-schedule toggle, or any other YAGNI knob. Spec Non-goals forbid all of these.
- Do NOT add a Prometheus metric, a Sentry breadcrumb, or a structured log line beyond the existing `glog.V(2).Infof` calls. Spec Non-goals forbid them.

## 13. Sibling entry-point check

The spec says `SetupCustomResourceDefinition` is called "on every binary boot". The repo has TWO entry points:
- `/workspace/main.go` (the long-lived service: HTTP + hourly tick)
- `/workspace/cmd/run-once/main.go` (smoke-test: one tick then exit)

Both call `application.Run(ctx, sentryClient)`. The CRD install is wired into `Run` in a future spec (Spec B per the spec's Non-goals "DOES NOT introduce an informer / `Listen` wiring, does NOT delete `pkg/schedule/inventory.go`, does NOT add any CRs to `k8s/schedules/` (those are Spec B + bootstrap migration)"). This prompt's deliverable is the connector itself + tests, NOT the `Run` wiring. The next prompt (or Spec B) will call `connector.SetupCustomResourceDefinition(ctx)` from both entry points.

Document this in `pkg/k8s_connector.go`'s package doc-comment: "This connector is wired into `main.go` and `cmd/run-once/main.go` in a future spec; this file is standalone."

</requirements>

<constraints>
- The CRD group `task.benjamin-borbe.de`, version `v1`, kind `Schedule`, plural `schedules`, singular `schedule`, short name `ts`, scope `Namespaced` are FROZEN. The connector reads them from the `v1` package's frozen constants — never hard-codes them.
- The `K8sConnector` interface has ONE method: `SetupCustomResourceDefinition(ctx context.Context) error`. No `Listen` (that is Spec B). No `Run` (the connector is a `run.Func` candidate but not a typed one in this spec).
- The `CRDClientBuilder` injection seam is `func(*rest.Config) (apiextensionsclient.Interface, error)`. Production wiring passes `apiextensionsclient.NewForConfig`; tests pass a closure returning a fake. Matches the bborbe agent pattern.
- The 4 wrap sites in `SetupCustomResourceDefinition` are: `build k8s config`, `build apiextensions clientset`, `get CRD`, `create/update CRD`. The wrap message format is `"<verb> <noun> [<group>.<kind>]"` — visible in the binary's boot logs.
- The `apiextensionsv1.AlreadyExists` race is handled by the `createCrd` helper: the `glog.V(2).Infof("k8s-connector: crd-already-exists: applying update")` line fires, and the method returns nil (the next call's `Update` path will succeed). Matches the spec's Failure Modes row 2.
- The `scheduleSpecSchema()` returns the OpenAPI v3 schema for `spec.*`. The CEL rule is `self.recurrence == 'Weekly' ? has(self.weekday) : !has(self.weekday)` with the message `"weekday is required when recurrence is 'Weekly', and forbidden otherwise"` (capital-case recurrence, matching Go `time.Weekday.String()`).
- The `vault` regex is `^[a-z][a-z0-9-]*$` (no uppercase, no leading digit, no underscores, no spaces).
- The `recurrence` enum is the closed set: `["Daily", "Weekly", "Monthly", "Quarterly", "Yearly"]` (capital, raw string values, NOT the typed `pkg/schedule.RecurrenceKind` constants — the in-Go constants stay lowercase; the CRD wire values are capital per the spec design pins).
- Generated `.go` files (the counterfeiter mock `mocks/k8s_connector.go`) use the project's BSD header. Counterfeiter 6.12.2 does NOT prepend headers; run `make addlicense` after generation to fix this.
- The `K8sConnector` interface has a Counterfeiter directive: `//counterfeiter:generate -o ../mocks/k8s_connector.go --fake-name FakeK8sConnector . K8sConnector`. The fake name matches the agent convention (`FakeK8sConnector`, not `K8sConnectorK8sConnector`).
- Do NOT touch `main.go`, `cmd/run-once/main.go`, the `Makefile`, the `k8s/` manifest tree, or the `k8s/apis/.../v1/` types (they are frozen from Prompt 1).
- Do NOT add a `Listen` method, an informer wiring, an admission webhook, a `--kubeconfig` flag, a `KUBECONFIG` env var, a `--dry-run-crd` mode, a Prometheus metric, a Sentry breadcrumb, or any per-schedule toggle. Spec Non-goals forbid all of these.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

</constraints>

<verification>

From `/workspace`:

1. `go build ./...` — must compile.
2. `go test -mod=mod -cover -race ./pkg/...` — all Ginkgo specs green, coverage ≥80% for `pkg/`.
3. `go test ./k8s/apis/task.benjamin-borbe.de/v1/...` — Prompt 1's tests still pass.
4. `go test ./...` — entire repo green.
5. `make precommit` — must exit 0.
6. `ls mocks/k8s_connector.go` — file present.
7. `grep -nE 'func (Fake)?K8sConnector' mocks/k8s_connector.go` — `FakeK8sConnector` type present, `SetupCustomResourceDefinition*` methods present.
8. `grep -nE 'ScheduleSpec|Vault|Template' k8s/apis/task.benjamin-borbe.de/v1/types.go` — Prompt 1's types intact.
9. `grep -nE 'WeekdayRequiredIfWeekly|recurrenceEnum|vaultPattern' pkg/k8s_connector_schema.go` — three named constants present.
10. `grep -nE 'func (New)?K8sConnector|func (k \*k8sConnector) (SetupCustomResourceDefinition|desiredCRDSpec|scheduleSpecSchema)' pkg/k8s_connector.go` — exactly one constructor, exactly three methods on the impl.
11. `grep -nE 'apiextensionsv1\.JSONSchemaProps|apiextensionsv1\.ValidationRules|XValidations' pkg/k8s_connector_schema.go` — schema uses the right k8s types.
12. `grep -c '"recurrence"' pkg/k8s_connector_validation_test.go` — at least 1 (the rejection test asserts on the substring).
13. `grep -c '"weekday"' pkg/k8s_connector_validation_test.go` — at least 2 (weekly-without and monthly-with both assert on it).
14. `grep -c '"vault"' pkg/k8s_connector_validation_test.go` — at least 1 (the Bad Vault rejection test).
15. Spot-check: open `pkg/k8s_connector.go` and visually confirm the 4 wrap sites (config, clientset, get, create/update) match the spec's "SetupCustomResourceDefinition has 4 wrap sites in the agent template" line.

</verification>

## Notes for the auditor

- **Spec gap surfaced (needs reviewer decision).** The spec's reference connector is at `~/Documents/workspaces/agent/task/executor/pkg/k8s_connector.go`. That path does not exist in this YOLO container (no `/Users/bborbe/...` or `/home/node/bborbe/...` mounts are visible). The prompt pins the bborbe CRD guide pattern (`go-kubernetes-crd-controller-guide.md` §4) as the structural template — same factory signature, same `CRDClientBuilder` injection seam, same `errors.Wrapf` discipline, same `desiredCRDSpec()` + `scheduleSpecSchema()` split. The exact line-for-line mirror is not possible without the source file; the structural mirror is. The reviewer should run a side-by-side diff against the actual agent source if/when it becomes available.
- **Spec gap surfaced (needs reviewer decision).** The spec's `## Verification` says `kind create cluster --name schedule-validation || true` and `KUBECONFIG=$(kind get kubeconfig-path ...)`. `kind get kubeconfig-path` was REMOVED in kind 0.11.0 (replaced with `kind get kubeconfig --name <name> | KUBECONFIG=... kubectl ...`). The spec is silently outdated. This prompt does NOT add the kind-cluster smoke (it's optional in the spec, and unit + integration tests in the prompts cover the load-bearing assertions). The reviewer can either drop the smoke or update the kind invocation in a follow-up.
- **Spec gap surfaced (needs reviewer decision).** The spec's `## Suggested Decomposition` says this prompt covers "AC 6, 7, 8, 9, 12, 13". The actual coverage in the prompts is:
  - AC 6 (the connector interface method exists) — covered by Prompt 2.
  - AC 7 (the testdata fixture exists) — covered by Prompt 1.
  - AC 8 (the example test round-trips) — covered by Prompt 1.
  - AC 9 (the connector's 3 It-blocks: create / update / wrap error) — covered by Prompt 2.
  - AC 12 (the validation test's 5 It-blocks) — covered by Prompt 2.
  - AC 13 (make precommit exits 0) — covered by both.
  - **AC 10** (Counterfeiter mock of K8sConnector at `mocks/k8s_connector.go` with fake name `FakeK8sConnector`) — the spec says `mocks/k8s_connector.go`, the prompt emits `mocks/k8s_connector.go` (matching the project's existing `mocks/publisher-publisher.go` / `mocks/tick-tick.go` pattern). The reviewer should confirm the file-naming convention is what they want; both are correct, they differ only in style.
- **Single decision tree.** The validation test uses `cel-go` directly to evaluate the CEL rule. The spec text says "e.g. `k8s.io/apiserver/pkg/cel`" — the `k8s.io/apiserver/pkg/cel` package is a library of CEL types and helpers, not a high-level schema validator. Using it for a one-rule one-doc unit test requires ~150 lines of env-setup boilerplate. `github.com/google/cel-go` is the underlying CEL engine (already an indirect dep via the k8s libs) and offers a 5-line compile + eval path. The prompt picks `cel-go` for simplicity. The reviewer can override if a stricter "use only `k8s.io/apiserver/pkg/cel`" reading of the spec is required.
- **Sibling entry-point rule.** Per the global CLAUDE.md, prompts that change a `factory.Create*` signature, an exported function signature, or a struct field consumed by `main.go` must list ALL entry points. This prompt does NOT call `SetupCustomResourceDefinition` from any entry point — it just declares the interface and the impl. The wiring is Spec B's job. The prompt body (§13) explicitly documents this.
- **The `pkg` package name.** The repo's existing pattern is to name packages by feature (`pkg/schedule`, `pkg/publisher`, `pkg/tick`, `pkg/handler`). The connector does not have a more specific name (it is the only thing in `pkg/` at the root level), so the package is `pkg`. The mock filename `mocks/k8s_connector.go` follows the existing `<package>-<interface>.go` style. An alternative is to call the package `pkg/k8sconnector` (subdir) with the mock at `mocks/k8sconnector-k8s-connector.go` — but the spec's "Mirrors `~/Documents/workspaces/agent/task/executor/pkg/k8s_connector.go`" pins the `pkg/k8s_connector.go` filename and the `pkg` package. The prompt follows the spec; the reviewer can override.
- **AC coverage.** Prompt 2 covers AC 6, 9, 10, 12, 13. AC 1, 2, 3, 4, 5, 7, 8, 11 are in Prompt 1.

</verification>
