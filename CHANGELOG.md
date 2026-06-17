# Changelog

## v0.0.1

Initial public release.

### Binary

- Kubernetes operator that publishes `task.CreateCommand` Kafka events on a fixed hourly schedule.
- Self-installs the `schedules.task.benjamin-borbe.de` Custom Resource Definition on boot.
- Watches `Schedule` CRs in its own pod namespace via a client-go informer; reads today's tasks from the live cache.
- Six recurrence kinds: `Daily`, `Weekly`, `Weekday`, `Monthly`, `Quarterly`, `Yearly`.
- Per-task deterministic identifier `UUID5("recurring-<slug>-<period-token>")` — safe to re-publish every tick, safe to manual replay, safe to crash-restart.
- Operator-configurable YAML frontmatter via `spec.template.frontmatter` (free-form `map[string]interface{}` on the CRD); publisher seeds `status: in_progress` + `page_type: task` defaults that operators may override, and force-sets `created_by: recurring-task-creator` as provenance.

### HTTP

- `GET /trigger?date=YYYY-MM-DD` — manual replay of the day's publishes; returns per-task JSON summary with error accumulation.
- `GET /healthz`, `GET /readiness`, `GET /metrics`, `GET /setloglevel/{level}`.

### Operability

- Single-replica StatefulSet, non-root pod with read-only root filesystem.
- Hourly tick, `Europe/Berlin` civil date, injectable clock for tests.
- `DRY_RUN` env flag — logs would-be `CreateCommand`s and skips the Kafka send.
- Prometheus counters: per-task publish success/failure; gauge: last-tick timestamp.

### Operator workflow

- Adding / editing / removing a recurring task is `kubectl apply`, not a code release. See `k8s/apis/task.benjamin-borbe.de/v1/testdata/example.yaml`.
