# Recurring Task Creator

[![Go Reference](https://pkg.go.dev/badge/github.com/bborbe/recurring-task-creator.svg)](https://pkg.go.dev/github.com/bborbe/recurring-task-creator)
[![CI](https://github.com/bborbe/recurring-task-creator/actions/workflows/ci.yml/badge.svg)](https://github.com/bborbe/recurring-task-creator/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bborbe/recurring-task-creator)](https://goreportcard.com/report/github.com/bborbe/recurring-task-creator)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/bborbe/recurring-task-creator)

Publishes `task.CreateCommand` events to Kafka on a fixed schedule so a downstream task-controller materializes recurring tasks (daily / weekly / weekday / monthly / quarterly / yearly) as Obsidian vault `.md` files.

Task definitions live as Kubernetes `Schedule` Custom Resources — adding / editing / removing a recurring task is `kubectl apply`, not a code release.

## How it works

```
kubectl apply -f schedule.yaml          ← operator
        │
        ▼
   K8s API server  (Schedule CRD, group task.benjamin-borbe.de/v1)
        │ watch
        ▼
   recurring-task-creator pod  ← installs the CRD at boot,
                                 runs an informer in its own pod
                                 namespace, hourly tick reads from
                                 the cache
        │ task.CreateCommand
        ▼
   Kafka  ← topic per vault (e.g. task-command-personal)
        │
        ▼
   agent-task-controller pod  ← consumes the topic,
                                writes the markdown file
        │
        ▼
   Obsidian vault git remote  ← committed by the controller
```

Identifiers are deterministic: `UUID5("recurring-<slug>-<period-token>")`, where the period token is `YYYY-MM-DD` (daily), `YYYYWww` (weekly), `YYYYWww-<3-letter-weekday>` (weekday), `YYYY-MM` (monthly), `YYYYQN` (quarterly), `YYYY` (yearly). The downstream controller dedups on identifier → safe to re-publish every tick, safe to manual `/trigger?date=YYYY-MM-DD` replay, safe to crash-restart.

## Define a schedule

See `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml` for the canonical shape:

```yaml
apiVersion: task.benjamin-borbe.de/v1
kind: Schedule
metadata:
  name: weekly-review              # frozen — UUID5 input
  namespace: default               # the recurring-task-creator pod watches its own namespace
spec:
  vault: default                   # routes to agent-task-controller-<vault>
  title: Weekly Review {{current_week}}   # placeholder-rendered; period token also suffixed
  schedule:
    recurrence: Weekday            # Daily | Weekly | Weekday | Monthly | Quarterly | Yearly
    weekday: Saturday              # required iff recurrence == Weekday (Monday..Sunday)
  template:
    body: |
      Reflect on the past week ({{current_week}}).
      Plan the next ({{next_week}}).
    frontmatter:                   # YAML frontmatter stamped onto the generated task file
      priority: 2
      status: in_progress
      page_type: task
      assignee: bborbe
      planned_date: "{{current_date}}"   # placeholder-rendered in string values
      due_date: "{{next_sun_date}}"
```

The CRD's OpenAPI schema enforces the recurrence enum, the weekday enum, and a CEL rule that requires `weekday` iff `recurrence == "Weekday"`.

## Template placeholders

Substituted in `title`, `body`, and any **string-valued** `frontmatter` field. Non-string frontmatter values (ints, slices, maps) pass through unchanged. Closed set — unknown tokens like `{{foo}}` render verbatim. All values are computed against the Berlin civil date the task fires for.

| Placeholder | Renders | Example (Sat 2026-06-20) |
|---|---|---|
| `{{current_date}}` | `YYYY-MM-DD` | `2026-06-20` |
| `{{next_sat_date}}` | `YYYY-MM-DD` (today if today IS Sat) | `2026-06-20` |
| `{{next_sun_date}}` | `YYYY-MM-DD` (today if today IS Sun) | `2026-06-21` |
| `{{current_week}}` | `YYYYWNN` (ISO) | `2026W25` |
| `{{next_week}}` | `YYYYWNN +7d` | `2026W26` |
| `{{current_month}}` | `YYYY-MM` | `2026-06` |
| `{{next_month}}` | `YYYY-MM +1mo` | `2026-07` |
| `{{last_month}}` | `YYYY-MM −1mo` | `2026-05` |
| `{{current_quarter}}` | `YYYYQN` | `2026Q2` |
| `{{last_quarter}}` | `YYYYQN −1q` | `2026Q1` |
| `{{current_year}}` | `YYYY` | `2026` |
| `{{next_year}}` | `YYYY +1y` | `2027` |
| `{{last_year}}` | `YYYY −1y` | `2025` |

`{{next_sat_date}}` / `{{next_sun_date}}` use **inclusive-today** semantics so a Sunday Schedule firing on Sun stamps `planned_date=<today>`, not `<today+7>`.

> **Removed in v0.3.0:** the pre-v0.2.0 kebab-case alias names (`{{date}}`, `{{iso-week}}`, `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`, `{{last-quarter}}`, `{{year}}`, `{{last-year}}`) no longer render. Use the canonical snake_case names above.

## Build + deploy

```bash
make test                          # ginkgo specs + coverage
BRANCH=dev  make buca              # build, push image, apply STS + RBAC to dev
BRANCH=prod make buca              # same for prod
```

`make buca` is a small chain (`make build && make upload && make apply`). The applied tree is the **binary deployment** only — `StatefulSet`, `Service`, `KafkaUser`, `Role` + `RoleBinding` for namespace-scoped `Schedule` watch, and `ClusterRole` + `ClusterRoleBinding` for the CRD self-install.

`Schedule` CRs themselves live in a separate (typically private) repository — they reference the deployed image but are operator-owned data, not part of the binary. The repo deployer applies them with `kubectl apply -k <overlay>` (kustomize) or any GitOps mechanism of choice.

## RBAC

The pod's ServiceAccount needs:

- `apiextensions.k8s.io/customresourcedefinitions` `get / create / update / patch` at cluster scope — for the boot-time CRD install
- `task.benjamin-borbe.de/schedules` `get / list / watch` in the pod's own namespace — for the informer

Manifests: `k8s/recurring-task-creator-{sa,clusterrole,clusterrolebinding,role,rolebinding}.yaml`.

## Local smoke-test

```bash
DRY_RUN=true ./recurring-task-creator-run-once -logtostderr -v=2
```

Skips Kafka init entirely (uses a noop sender); logs every `(slug, date, identifier)` triple the publisher *would* send. Use to verify a Schedule's behavior before deploying.

## Repo layout

| Path | Purpose |
|---|---|
| `main.go` + `cmd/run-once/main.go` | Long-lived service entry point + one-tick smoke binary |
| `pkg/k8s_connector.go` | Self-installing CRD (`SetupCustomResourceDefinition`) — get-or-create-or-update on every binary boot, 30s timeout |
| `pkg/k8s_connector_schema.go` | The Go-built `JSONSchemaProps` (vault regex, recurrence enum, weekday enum, CEL rule) |
| `pkg/store` | Informer-backed `ScheduleStore` — the only source of truth at tick time |
| `pkg/publisher` | Builds `task.CreateCommand`, computes the period-anchored UUID5, sends via injected sender |
| `pkg/tick` | Hourly cron loop + Prometheus metrics |
| `pkg/handler` | HTTP — `/healthz`, `/trigger?date=YYYY-MM-DD`, `/setloglevel/{n}`, `/metrics` |
| `pkg/factory` | Composition root — `Create*` constructors that wire everything |
| `pkg/schedule` | Internal types — `Date`, `RecurrenceKind`, `TaskDefinition`, `TasksForDate` |
| `k8s/apis/task.benjamin-borbe.de/v1/` | CRD Go types (hand-written) + `zz_generated.deepcopy.go` |
| `k8s/client/` | Generated typed clientset + informers + listers + applyconfiguration |
| `k8s/recurring-task-creator-*.yaml` | Binary deployment manifests (STS + RBAC) |
| `hack/update-codegen.sh` | Wraps `kube_codegen.sh` for the client tree |
| `mocks/` | Counterfeiter mocks |
| `specs/`, `prompts/` | dark-factory development history (specs → prompts → daemon-executed code) |

## License

BSD-2-Clause. See `LICENSE`.
