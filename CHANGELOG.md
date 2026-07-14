# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## Unreleased

- feat: add `RecurrenceOnDate` recurrence kind to `pkg/schedule` — fires on a fixed calendar month+day every year (e.g. birthdays). The kind is appended to `AllRecurrenceKinds`, `Month time.Month` and `Day int` fields are added to `TaskDefinition`, and `filterInventoryByDate` gains a match-fire case. Unknown recurrence kinds in the switch are now skipped with a `glog.Warning` instead of silently always-firing, hardening the always-fire kinds' invariant.
- feat: add `RecurrenceOnDate` period token (`YYYY`) to the publisher — token is the fire date's 4-digit year via `fmtYear`, no `PeriodOffset` applied, matching `Yearly` token shape for once-per-year dedup

## v0.9.1

- Update Go to 1.26.5 and bump bborbe/agent, cqrs, errors, http, kafka, log, metrics, run, sentry, service, time and transitive dependencies
- Ignore GO-2026-5932 (golang.org/x/crypto/openpgp unmaintained, no fix available) in vulncheck and trivy

## v0.9.0

- feat: add standalone Helm chart in `helm/` (StatefulSet + RBAC + Service + optional Strimzi KafkaUser / Sentry Secret). Restores the in-repo deploy tooling that v0.8.1 removed, now co-located with the binary so the deploy contract (env such as `TOPIC_PREFIX`) lives with the code that defines it. Per-cluster config stays in the quant config repo. Publish with `make helm-publish` to `oci://registry-1.docker.io/bborbe/recurring-task-creator`.

## v0.8.2

- Update bborbe/agent, cqrs, errors, http, kafka, log, metrics, run, sentry, service, time dependencies
- Update google/cel-go and go-openapi/swag transitive dependencies
- Update klauspost/compress, prometheus/procfs, k8s.io/utils indirect dependencies

## v0.8.1

- refactor: converge build to the bborbe/kafka-topic-reader publish-only model — make buca now builds and pushes docker.io/bborbe/recurring-task-creator:$(VERSION); deploy machinery removed.

## v0.8.0

- feat: Bump `github.com/bborbe/agent` v0.70.0 → v0.72.0, `github.com/bborbe/cqrs` v0.5.3 → v0.6.0
- feat: Add explicit `TopicPrefix base.TopicPrefix` config field (`arg:"topic-prefix"`, `env:"TOPIC_PREFIX"`, optional) to both `main.go` and `cmd/run-once/main.go`; Kafka command topics are now built from `TopicPrefix` only (empty means unprefixed, no leading dash) — the existing `Stage`/`STAGE` field is retained unchanged for its other (non-topic) uses
- test: Add golden test proving published command topic literals — `develop-agent-task-v1-request` (dev), `master-agent-task-v1-request` (prod), and `agent-task-v1-request` (empty prefix) — via `factory.CreateCommandSender` wired to the real `github.com/bborbe/kafka/mocks.KafkaSyncProducer` fake
- chore: k8s manifest (`k8s/recurring-task-creator-sts.yaml`) now also sets `TOPIC_PREFIX`; `dev.env`/`prod.env` pin it to `develop`/`master` respectively to keep existing deployments' topic names byte-identical to the previous implicit `STAGE`-derived mapping

## v0.7.0

- feat: stamp `auto_abort_prior: <bool>` onto every published task's frontmatter, mirroring the resolved `TaskDefinition.AutoAbortPrior`. The key is set after operator-supplied frontmatter is merged (so an operator-supplied `auto_abort_prior` cannot override the spec-level value) and before `created_by` is force-set (preserving the provenance invariant). The value is a Go `bool` serialized as a YAML boolean `true`/`false`, never a string. Existing Schedules publish byte-identical task identifiers, titles, and bodies — the only payload change is the added frontmatter key.
- feat: add optional `spec.schedule.autoAbortPrior` boolean on the `Schedule` CRD — an opt-in flag (default `false` when omitted) marking a Schedule whose prior-period instance may be auto-aborted by the downstream task-controller when the next instance materializes. Carried as a `*bool` on `ScheduleTrigger` so unset is distinguishable from explicit `false`, with nil-safe deepcopy and apply-configuration plumbing. The CRD schema declares it as `Type: "boolean"` (no enum, not required), so the API server rejects non-boolean values at `kubectl apply` time. Existing Schedules without the field are unaffected; this is the API-contract layer only — no publishing behavior change yet.
- feat: carry `autoAbortPrior` from the CRD into the domain layer — add a plain `AutoAbortPrior bool` field to `schedule.TaskDefinition`, and have the store adapter (`adaptSchedule`) resolve the CR's `spec.schedule.autoAbortPrior` `*bool` onto it nil-safely (nil → `false`, explicit `false` → `false`, explicit `true` → `true`). Bridges the API contract (prompt 1) to the pure-data `TaskDefinition` the publisher consumes (prompt 3); the schedule layer stays free of Kafka/HTTP/agent imports. No publishing behavior change yet.
- feat: add opt-in `spec.autoAbortPrior` boolean to the `Schedule` CRD (default `false`, optional) and stamp `auto_abort_prior: <bool>` onto every materialized task's frontmatter, mirroring the field. The stamp is set after operator keys and before the force-set `created_by` provenance key. New Schedules are safe by default — no prior instance is auto-aborted unless an operator explicitly sets `autoAbortPrior: true`. The downstream controller-side gate flip ships separately.

## v0.6.1

- chore(deps): migrate to github.com/bborbe/agent v0.70.0 (was github.com/bborbe/agent/lib v0.68.0)

## v0.6.0

- fix: cap `spec.schedule.weekdays` at `maxItems: 7` and rewrite the cross-form no-duplicates CEL rule to a bounded set-size form, so the Kubernetes API server's CEL cost estimator stops rejecting the `Schedule` CRD ("estimated rule cost exceeds budget"); resolves the dev-pod CrashLoopBackOff. Duplicate-day rejection (`[Mon, Monday]`, `[Tue, Tue]`) is unchanged.
- fix: replace structurally-invalid `oneOf{string,array}` on `spec.schedule.weekday` with two type-pure sibling fields — `weekday: string` (7 long-form days, backward-compatible) and `weekdays: []string` (new, 14-value enum, `minItems: 1`) — resolving the dev-pod CrashLoopBackOff caused by the Kubernetes API server rejecting the non-structural CRD schema.
- fix: Replace structurally-invalid oneOf weekday CRD schema with type-pure `weekday` string + new `weekdays` list field; CEL enforces exactly-one-of on Weekday recurrence. Fixes dev-pod CrashLoopBackOff on CRD install.
- feat: widen `spec.schedule.weekday` CRD field to accept a single day string OR a non-empty list of day strings — one `Schedule` CR can now target Mon–Fri instead of five near-identical CRs. Adds 14-value weekday enum (long form `Monday..Sunday` + short form `Mon..Sun`). CEL rules added to reject empty lists (`weekday: []`) and duplicate days including cross-form pairs like `[Mon, Monday]`. Existing single-string CRs are unaffected. Go-side normalization of short forms to `time.Weekday` lands in Prompt 2.
- feat: `spec.schedule.weekday` on the `Schedule` CR now accepts a non-empty list of weekdays in addition to a single string, and accepts short forms (`Mon`..`Sun`) alongside long forms (`Monday`..`Sunday`), freely mixed. A list `[Mon, Wed, Fri]` collapses what used to be three near-identical CRs into one and publishes one task file per matching weekday per ISO week, each with its own per-day period token (`-mon`/`-wed`/`-fri`) and UUID5. Single-string CRs are byte-identical to before (same UUID5, no vault file regeneration). Empty lists, duplicate days (including cross-form like `[Mon, Monday]`), unknown day strings, and any `weekday` on a non-`Weekday` recurrence are rejected at `kubectl apply` time by CRD CEL validation.

## v0.5.0

- refactor: extract `PeriodTokenBuilder` and `TaskIdentifierCreator` interfaces in `pkg/publisher`; introduce strong `type PeriodToken string`. Collapses the 6-parameter `buildTaskIdentifier(ctx, slug, recurrence, date, weekday, periodOffset)` into a 3-parameter `Create(ctx, def, date)` returning `(identifier, periodToken)` in one call — eliminates the previous duplicate period-token computation in `publisher.Publish`. Counterfeiter mocks generated for both interfaces. `factory.CreatePublisher` wires the new dependencies; the public `Publisher` contract is unchanged from a caller perspective. Internal-only — no Schedule CR shape change, no UUID5 input change, identifiers stable.
- feat: add optional `spec.schedule.periodOffset` (int, default 0) on `Schedule` CR — shifts the period-anchored token by N periods. Lets review-style schedules fire on the first day of the next period and name the just-completed period (e.g. `monthly-review` with `periodOffset: -1` firing on 2026-07-01 produces task `Review Month - 2026-06`). The shift also feeds into the UUID5 input, so re-publishing the same `(slug, fire-date, offset)` triple stays idempotent. Only valid for `Monthly` / `Quarterly` / `Yearly`; date-anchored kinds (`Daily` / `Weekly` / `Weekday`) reject non-zero values via a CEL rule. Body placeholders (`{{current_month}}` etc.) still render against the unshifted fire date — only the period-token suffix and the identifier shift.

## v0.4.0

- feat: `/trigger` `date` query parameter is now optional — falls back to the clock's current civil date when missing, empty, or unparseable (via `libtime.ParseDateTimeDefault`).
- refactor: `NewTriggerHandler` switched to the `libhttp.NewErrorHandler` + `NewJSONHandler` + `JSONHandlerFunc` pattern; `CurrentDateTimeGetter` injected for the NOW fallback.
- chore: bump indirect deps (`vault-cli` v0.68.0 → v0.83.0, `sarama`, `fatih/color`, `cel.dev/expr`).

## v0.3.0

- feat!: remove the pre-v0.2.0 kebab-case placeholder aliases (`{{date}}`, `{{iso-week}}`, `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`, `{{last-quarter}}`, `{{year}}`, `{{last-year}}`). Use the canonical snake_case names introduced in v0.2.0 (`{{current_date}}`, `{{current_week}}`, `{{next_week}}`, `{{current_month}}`, `{{last_month}}`, `{{current_quarter}}`, `{{last_quarter}}`, `{{current_year}}`, `{{last_year}}`). BREAKING CHANGE — Schedule CRs that still use kebab-case names will land literal `{{...}}` strings in their generated task files. Operator must migrate every Schedule CR YAML to the canonical names before deploying this release.

## v0.2.0

- feat: add canonical placeholder names — `{{current_date}}`, `{{current_week}}`, `{{next_week}}`, `{{current_month}}`, `{{next_month}}`, `{{last_month}}`, `{{current_quarter}}`, `{{last_quarter}}`, `{{current_year}}`, `{{next_year}}`, `{{last_year}}`
- feat: add weekday-targeted date placeholders — `{{next_sat_date}}`, `{{next_sun_date}}` (inclusive-today: when today IS the target weekday, return today's date rather than +7)
- refactor: declarative `[]placeholder{name, compute func}` table in `pkg/publisher/placeholders.go`; `SupportedPlaceholders` derives from the table — adding a placeholder is now one row
- refactor: extract `Renderer` interface + `NewRenderer` constructor; `NewPublisher` and `NewFrontmatterFormatter` take the renderer as a constructor dep. Mockable via Counterfeiter (`mocks/publisher-renderer.go`), tested in isolation in `pkg/publisher/placeholders_test.go`
- deprecate: kebab-case alias names — `{{date}}`, `{{iso-week}}`, `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`, `{{last-quarter}}`, `{{year}}`, `{{last-year}}` — still rendered (alias to new canonical names) but slated for removal in the next minor. Migrate Schedule CR YAMLs at leisure.

## v0.1.0

- feat: render placeholders in operator-supplied string frontmatter values (same closed set as title/body); non-string values pass through unchanged. Enables `planned_date: "{{date}}"` and similar dynamic frontmatter in Schedule CRs.
- refactor: extract `FrontmatterFormatter` interface + `NewFrontmatterFormatter` constructor; `NewPublisher` now takes the formatter as a constructor dependency. Mockable via Counterfeiter (`mocks/publisher-frontmatter-formatter.go`), tested in isolation in `pkg/publisher/frontmatter_test.go`.

## v0.0.2

- bump alpine 3.23 → 3.24 in Dockerfile
- update bborbe/kafka v1.23.2 → v1.25.0 and bborbe/cqrs v0.5.2 → v0.5.3
- update k8s.io/{api,apimachinery,client-go,apiextensions-apiserver} v0.36.1 → v0.36.2
- update cel-go v0.26.0 → v0.28.1, ginkgo v2.29.0 → v2.31.0, gomega v1.41.0 → v1.42.0
- drop stoewer/go-strcase indirect dep; exclude cloud.google.com/go v0.26.0

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
