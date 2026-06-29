---
status: completed
spec: [015-recurring-task-cleanup-cron]
summary: Added recurring-task-creator-cleanup sibling binary with git-rest HTTP client, hourly cron loop, deployed StatefulSet, Makefile/Dockerfile multi-binary build, and docs
execution_id: recurring-task-creator-cleanup-exec-028-spec-015-cleanup-binary-gitrest-sts
dark-factory-version: v0.188.1
created: "2026-06-29T19:45:00Z"
queued: "2026-06-29T19:34:17Z"
started: "2026-06-29T20:11:52Z"
completed: "2026-06-29T20:32:58Z"
branch: dark-factory/recurring-task-cleanup-cron
---

<summary>
- Ships the cleanup logic as a deployable sibling binary that runs on an hourly cron tick, offset from the publisher so the two don't collide.
- Implements the vault read/write interfaces against the existing git-rest HTTP service, forwarding the gateway auth header and using a finite per-call timeout.
- Wires the binary the same way the existing run-once smoke binary is wired: CRD self-install, informer cache, sentry, then the orchestrator on a loop.
- Adds a Kubernetes StatefulSet manifest for the new binary, mirroring the publisher's, plus its cron and git-rest env vars.
- Adds a one-tick smoke-test entrypoint so the binary's wiring is build-and-run verifiable without entering the hourly loop.
- Updates the Dockerfile/Makefile to build the new binary image, and documents the binary and its env vars in the README and architecture docs.
- The cleanup cron reads and writes the vault only; the publisher, the tick, the trigger handler, and the Kafka materialize path are untouched.
</summary>

<objective>
Make the `pkg/cleanup` package deployable: implement a git-rest HTTP client satisfying `VaultReader`/`VaultWriter`, add a factory wiring function, a `cmd/cleanup` hourly-cron binary plus a `cmd/cleanup-run-once` one-tick smoke binary mirroring `cmd/run-once`, a StatefulSet manifest, Dockerfile/Makefile image build, and README/docs updates.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This prompt DEPENDS ON prompt 2 (`2-spec-015-cleanup-package.md`) having landed. Verify before starting:

```bash
grep -q 'type Supersedance struct' /workspace/pkg/cleanup/supersedance.go
grep -q 'type VaultReader interface' /workspace/pkg/cleanup/vault.go
```

If either exits non-zero, prompt 2 has not landed — STOP and report `status: failed` with summary "prompt 2 (pkg/cleanup) not yet deployed".

Read these files fully before wiring:
- `/workspace/cmd/run-once/main.go` — the EXACT structure to mirror for `cmd/cleanup-run-once/main.go` and (loop variant) `cmd/cleanup/main.go`: `service.Main`, the `application` struct with libargument tags, CRD self-install via `pkg.NewK8sConnector(...).SetupCustomResourceDefinition(ctx)`, `versioned.NewForConfig`, `factory.CreateScheduleStore`, the informer `StartWithContext(ctx)` + 30s-bounded `WaitForCacheSyncWithContext`, then the run call.
- `/workspace/cmd/run-once/Makefile` and `/workspace/cmd/Makefile` — the per-binary Makefile pattern (the no-op `apply` target for non-deployed CLIs; cleanup-run-once is a CLI, cmd/cleanup IS deployed).
- `/workspace/main.go` — `run.CancelOnFirstFinish(ctx, ...)` usage and how the long-lived loop is structured. The cleanup loop binary uses the same `run` primitive.
- `/workspace/pkg/factory/factory.go` — the `Create*` factory pattern (zero business logic), `CreateScheduleStore`, `CreateTickLoop`. Add `CreateCleanup(...)` here mirroring `CreateTickLoop`.
- `/workspace/pkg/cleanup/supersedance.go`, `/workspace/pkg/cleanup/vault.go`, `/workspace/pkg/cleanup/metrics.go` — the package built in prompt 2: `Supersedance` struct fields, `VaultReader`/`VaultWriter` interfaces, `ErrVaultConflict` sentinel, `NewPrometheusMetrics()`, `PriorPeriodToken`.
- `/workspace/pkg/handler/trigger.go` and `/workspace/main.go` (imports) — confirm how `libhttp "github.com/bborbe/http"` is used in this repo (`libhttp.NewServer`, `libhttp.NewErrorHandler`, etc.). The git-rest client uses a standard `*http.Client` with a timeout; grep `grep -rn 'http.Client\|libhttp\.' /workspace/pkg /workspace/main.go` to find the in-repo convention before choosing.
- `/workspace/k8s/recurring-task-creator-sts.yaml` — the STS to mirror for `k8s/recurring-task-creator-cleanup-sts.yaml`. Note the env block (NAMESPACE via Downward API, SENTRY_DSN from secret, TZ=Europe/Berlin), the keel annotations, securityContext, resources, probes, the `{{ "X" | env }}` templating, and `image: '{{"DOCKER_REGISTRY" | env}}/recurring-task-creator:{{"BRANCH" | env}}'`.
- `/workspace/k8s/Makefile`, `/workspace/Makefile.docker`, `/workspace/Dockerfile` — the image-build wiring. Read to determine whether this repo builds one image with multiple binaries or one image per binary, then follow that pattern for the cleanup binary.
- `/workspace/README.md` — the commands/env-vars section to extend.
- `/workspace/docs/architecture.md` and `/workspace/docs/system-map.md` — the packages table and system diagram to extend with the cleanup cron row.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `Create*` prefix, zero logic, constructor returns interfaces.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-k8s-binary-conventions.md` — binary structure, env-var conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` 3-arg wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — `run.CancelOnFirstFinish` over raw goroutines.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega; compile-test pattern for `main` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — CHANGELOG entry format.

OPEN QUESTION — cron library (resolve at implementation time, fail-closed):
The spec references `cron.NewExpressionCron` from `github.com/bborbe/trading/lib/cron` and `libcron.MustParse` / `libcron.Expression` from `github.com/bborbe/cron`. Resolve as follows, in order:
1. Run `grep -n 'github.com/bborbe/cron' /workspace/go.mod`. If `github.com/bborbe/cron` is already a (direct or indirect) dependency, grep its source for the required symbols: `grep -rn 'func MustParse\|func NewExpressionCron\|type Expression' "$(go env GOMODCACHE)"/github.com/bborbe/cron@*/`. Use it ONLY if grep confirms every symbol.
2. If `github.com/bborbe/cron` is NOT present, run `go get github.com/bborbe/cron@latest` and re-run the symbol grep. If every symbol resolves, proceed.
3. If symbols still do not resolve, STOP — set `"status": "failed"` with summary `cron library symbols unresolved; spec author must pin a version`. Do NOT implement an in-repo fallback cron loop — duplicating `pkg/tick/tick.go` violates the spec's reuse-of-tagesschau-factory goal and creates drift.

OPEN QUESTION — git-rest client (resolve at implementation time, fail-closed):
The spec references `github.com/bborbe/agent/pkg/gitrestclient` as a reuse target. The agent module IS a dependency (`github.com/bborbe/agent v0.70.0`). Resolve as follows:
1. Run `find "$(go env GOMODCACHE)"/github.com/bborbe/agent@v0.70.0 -type d -name gitrestclient`. If the directory exists, grep it for the symbols we need: `grep -rn 'func New\|GetFile\|ListFiles\|UpdateFile' "$(go env GOMODCACHE)"/github.com/bborbe/agent@v0.70.0/...gitrestclient.../*.go`. If it exists and exposes a usable client, adapt it behind the `pkg/cleanup` interfaces (`VaultReader` + `VaultWriter`).
2. If `pkg/gitrestclient` does NOT exist or does not expose the methods needed, STOP — set `"status": "failed"` with summary `git-rest client pkg/gitrestclient unavailable; spec author must add or fix the dependency`. Do NOT invent a parallel `pkg/gitrestvault` — the spec explicitly requires reuse of the existing client.
</context>

<requirements>

### 1. git-rest vault client (satisfies `cleanup.VaultReader` / `cleanup.VaultWriter`)

Per the git-rest OPEN QUESTION above. The resulting type(s) must satisfy both `cleanup.VaultReader` and `cleanup.VaultWriter`. Requirements regardless of path:
- Finite HTTP timeout (10s per call) — no infinite hang; retry is the next hourly tick, not inline.
- Forward `X-Gateway-Secret: <GATEWAY_SECRET>` header when `GATEWAY_SECRET` env is non-empty; omit it otherwise.
- `UpdateFile` is merge-aware: re-read the current bytes inside the call, apply the mutator, write back; a git-rest 409 response → error wrapping `cleanup.ErrVaultConflict`.
- `bborbe/errors` 3-arg wrapping on every error path; no `fmt.Errorf`, no `context.Background()`.
- Constructor follows the repo's interface→constructor→struct pattern: `New...(httpClient, baseURL, gatewaySecret) cleanup.VaultReader` (or a combined `VaultClient` satisfying both).
- Unit tests with `httptest.Server` covering: successful GET, successful list, 404 not-found, 409 conflict (assert `errors.Is(err, cleanup.ErrVaultConflict)`), 5xx error, and gateway-secret header forwarding. Coverage ≥80% for the new client package.

### 2. `pkg/factory/factory.go` — `CreateCleanup`

Add a `Create*` factory function with ZERO business logic (per the factory pattern), mirroring `CreateTickLoop`:

```go
// CreateCleanup wires the cleanup orchestrator and the cron loop around it.
// Pure plumbing: schedule store, vault client, clock, metrics, sentry, and
// the cron expression. Returns a run.Func that the main binary can compose
// alongside the HTTP server via run.CancelOnFirstFinish.
func CreateCleanup(
	sentryClient libsentry.Client,
	currentTimeGetter libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	vaultClient cleanup.VaultClient,
	metrics cleanup.Metrics,
	cronExpr libcron.Expression,
) run.Func {
	return func(ctx context.Context) error {
		supersedance := &cleanup.Supersedance{
			Store:        scheduleStore,
			TokenBuilder: publisher.NewPeriodTokenBuilder(),
			VaultClient:  vaultClient,
			Metrics:      metrics,
			Clock:        currentTimeGetter,
		}
		return cron.NewExpressionCron(
			sentryClient,
			supersedance.Run,
			cronExpr,
		).Run(ctx)
	}
}
```

(Match the actual `Supersedance` field names and constructor shape from prompt 2 — read the file. If `Supersedance` exposes a `New...` constructor instead of a struct literal, use that.)

### 3. `cmd/cleanup-run-once/main.go` — one-tick smoke binary

Mirror `cmd/run-once/main.go` exactly, but: no Kafka producer (cleanup does not touch Kafka); add `GIT_REST_URL` (required) and `GATEWAY_SECRET` (optional) to the `application` struct; build the git-rest client; build `factory.CreateCleanup(...)`; resolve the current Berlin civil date (same `time.LoadLocation("Europe/Berlin")` path the publisher tick uses — read `pkg/tick/tick.go`); call `supersedance.Run(ctx, today)` ONCE and return. Exits non-zero on error. Add `cmd/cleanup-run-once/main_test.go` (compile test, mirror `cmd/run-once/main_test.go`) and `cmd/cleanup-run-once/Makefile` (mirror `cmd/run-once/Makefile`, including the no-op `apply` target — this is a local CLI, not deployed).

The `application` struct env/arg tags (libargument), mirroring `cmd/run-once`:
- `SentryDSN` (`SENTRY_DSN`, required:false, display:length), `SentryProxy` (`SENTRY_PROXY`), `Namespace` (`NAMESPACE`, required:true), `GitRestURL` (`GIT_REST_URL`, required:true), `GatewaySecret` (`GATEWAY_SECRET`, required:false, display:length).

### 4. `cmd/cleanup/main.go` — hourly-cron deployed binary

Mirror `cmd/cleanup-run-once/main.go`'s wiring (CRD install, informer, git-rest client, factory) but instead of one tick, run the hourly cron loop per the cron-library OPEN QUESTION resolution. Add the `CLEANUP_CRON` env (`CLEANUP_CRON`, required:false, default `"17 * * * *"`). On startup, log `cleanup: started` at V(2) (the Post-Deploy AC greps for this exact line). On each tick, resolve the current Berlin civil date and call `supersedance.Run(ctx, today)`; on tick error, capture via sentry and continue to the next tick (do not exit the loop on a per-tick error — only a context-cancelled / fatal wiring error exits non-zero). Use `run.CancelOnFirstFinish` if combining the loop with anything else; otherwise the loop runs until ctx cancellation (SIGTERM). Add `cmd/cleanup/main_test.go` (compile test) and `cmd/cleanup/Makefile`.

The `cmd/cleanup/Makefile` is a DEPLOYED binary — it does NOT use the no-op `apply` target; it participates in the image build and apply sweep. Mirror how the root/`main.go` binary is built and applied (read `/workspace/Makefile.docker` and `/workspace/k8s/Makefile`).

### 5. `k8s/recurring-task-creator-cleanup-sts.yaml` — StatefulSet manifest

Mirror `/workspace/k8s/recurring-task-creator-sts.yaml`. Changes:
- `metadata.name`, `selector.matchLabels.app`, `serviceName`, `template.labels.app` → `recurring-task-creator-cleanup`.
- Env block: keep `SENTRY_DSN` (from the `recurring-task-creator` secret), `SENTRY_PROXY`, `TZ: Europe/Berlin`, `NAMESPACE` (Downward API). REMOVE `KAFKA_BROKERS`, `STAGE`, `DRY_RUN` (cleanup touches no Kafka). ADD:
  - `CLEANUP_CRON` with value `'{{ "CLEANUP_CRON" | env }}'` and a sensible default documented in README (`17 * * * *`).
  - `GIT_REST_URL` pointing at the personal vault git-rest service (value `'{{ "GIT_REST_URL" | env }}'`).
  - `GATEWAY_SECRET` from the secret if present, else templated env (mirror `SENTRY_PROXY`'s optional pattern).
- `image:` → `recurring-task-creator-cleanup:{{"BRANCH" | env}}` if the repo builds one image per binary, OR the same image with a different entrypoint/args if it builds one multi-binary image — follow whatever pattern step 4's Makefile/Dockerfile resolution established. Keep them consistent.
- Keep `replicas: 1` (the spec's two-replica failure mode notes replicas must be 1).
- Keep probes pointing at `/healthz` / `/readiness` ONLY IF the cleanup binary serves HTTP. If `cmd/cleanup` does NOT start an HTTP server (it may not — it is a cron loop), REMOVE the liveness/readiness HTTP probes and the `ports` block. The spec does NOT mandate an HTTP server; if Prometheus scraping is needed for the `recurring_task_cleanup_superseded_total` counter, defer to the operator's monitoring stack (push gateway) rather than embedding an HTTP server in the cron binary. Do NOT duplicate `main.go`'s `createHTTPServer` scaffolding in `cmd/cleanup` — keep the cron binary a single-purpose binary.

`CLEANUP_CRON` must appear at least twice in the manifest (env name + value reference) per the AC: `grep -c 'CLEANUP_CRON' k8s/recurring-task-creator-cleanup-sts.yaml` returns ≥2.

### 6. Dockerfile / Makefile image build

Per the one-image-vs-per-binary decision from step 4. If per-binary: add a cleanup build stage/Dockerfile (mirror the existing `Dockerfile`, building `./cmd/cleanup`) and a Makefile target. If one multi-binary image: ensure `./cmd/cleanup` is built and the STS sets the right entrypoint/args. Keep `ENV ZONEINFO` / tzdata copy so `time.LoadLocation("Europe/Berlin")` works in the scratch image (the existing Dockerfile already copies `zoneinfo.zip`).

### 7. README + docs

- `/workspace/README.md`: document the new `recurring-task-creator-cleanup` binary, its purpose (hourly auto-abort of prior in-progress instances), and its env vars. `grep -cE 'CLEANUP_CRON|GIT_REST_URL' README.md` must return ≥2.
- `/workspace/docs/architecture.md`: add a `pkg/cleanup` row to the packages table and document the new binary.
- `/workspace/docs/system-map.md`: add the cleanup cron to the system diagram (reads/writes vault via git-rest; no Kafka).

### 8. CHANGELOG

Append under `## Unreleased`:

```
- feat: Add `recurring-task-creator-cleanup` sibling binary — hourly cron (default `17 * * * *`) that auto-aborts prior in-progress recurring-task instances via git-rest; new `GIT_REST_URL` / `GATEWAY_SECRET` / `CLEANUP_CRON` env and StatefulSet manifest.
```

</requirements>

<constraints>
- **Do NOT change** the publisher, the tick, the `/trigger` handler, the period-token / UUID5 derivation, or the Kafka materialize path. The cleanup binary is a NEW sibling; it reads and writes the vault only.
- **Git-rest is the only vault writer.** No direct git CLI, no `os.WriteFile`, no in-process git library.
- **No Kafka** in the cleanup binary — no producer, no `KAFKA_BROKERS`/`STAGE`/`DRY_RUN` env.
- **Single-namespace informer** mirrors the publisher: watch Schedule CRs only in the pod's own namespace via `NAMESPACE` Downward-API env.
- **replicas: 1** in the STS (the two-replica case is a deployment-mistake failure mode, not a supported config).
- **Finite HTTP timeout (10s)**; retry is the next hourly tick — no inline retries, no exponential backoff.
- **Europe/Berlin civil date** is the only clock surface — same `time.LoadLocation("Europe/Berlin")` path as the publisher.
- **Factory has zero business logic** — `CreateCleanup` is pure plumbing (no loops, no conditionals).
- **Do NOT add the `github.com/bborbe/trading` module** to pull a cron wrapper — see the cron OPEN QUESTION; if `github.com/bborbe/cron` symbols do not resolve at implementation time, STOP with `status: failed` per the cron OPEN QUESTION — do NOT implement an in-repo fallback cron loop.
- **Grep-verify every external symbol** before importing it (per global rules): `cron.NewExpressionCron`, `libcron.MustParse`, any `gitrestclient` symbol. If a symbol cannot be grep-confirmed in `$(go env GOMODCACHE)`, STOP with `status: failed` per the relevant OPEN QUESTION — do NOT invent in-repo fallbacks (no parallel `pkg/gitrestvault`, no in-repo ticker loop).
- Project DoD applies (`/workspace/docs/dod.md`): `bborbe/errors` 3-arg wrapping; Ginkgo v2 / Gomega; no `time.Now()`/`context.Background()` in business logic; counterfeiter mocks; coverage ≥80% for new code (the git-rest client package); `make precommit` clean.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && go build ./cmd/cleanup/... ./cmd/cleanup-run-once/...
cd /workspace && make test
```

Targeted:

```bash
cd /workspace && go test -v ./cmd/cleanup/... ./cmd/cleanup-run-once/...
cd /workspace && grep -c 'CLEANUP_CRON' k8s/recurring-task-creator-cleanup-sts.yaml   # >= 2
cd /workspace && grep -cE 'CLEANUP_CRON|GIT_REST_URL' README.md                       # >= 2
cd /workspace && grep -n 'cleanup: started' cmd/cleanup/main.go                       # >= 1
```

Confirm:
- `cmd/cleanup` and `cmd/cleanup-run-once` build and their compile tests pass.
- The git-rest client tests pass (GET, list, 404, 409→`ErrVaultConflict`, 5xx, gateway-secret header).
- The STS manifest exists with `CLEANUP_CRON` ≥2, `replicas: 1`, no Kafka env.
- README documents `CLEANUP_CRON` and `GIT_REST_URL`.

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.
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

- (fill in per the reflection rules; write `- None` if nothing — and DOCUMENT the cron-library and git-rest-client resolution paths you chose, category PROMPT)
</completion>
