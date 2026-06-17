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
  title: Weekly Review             # Go text/template; period token suffixed by publisher
  schedule:
    recurrence: Weekday            # Daily | Weekly | Weekday | Monthly | Quarterly | Yearly
    weekday: Saturday              # required iff recurrence == Weekday (Monday..Sunday)
  template:
    body: |
      Reflect on the past week.
      Plan the next.
    frontmatter:                   # YAML frontmatter stamped onto the generated task file
      priority: 2
      status: in_progress
      page_type: task
      assignee: bborbe
```

The CRD's OpenAPI schema enforces the recurrence enum, the weekday enum, and a CEL rule that requires `weekday` iff `recurrence == "Weekday"`.

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
