---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-14T12:05:08Z"
generating: "2026-06-14T12:13:39Z"
prompted: "2026-06-14T12:25:32Z"
verifying: "2026-06-14T12:52:12Z"
branch: dark-factory/k8s-manifests
---

## Summary

- Customize the inherited go-skeleton `k8s/` manifests so the binary built by Specs 1-4 can be deployed to the bborbe quant cluster as the `recurring-task-creator` service in `dev` and `prod` namespaces.
- Flip the StatefulSet from `replicas: 0` to `replicas: 1` (single pod — recurring publishes must NOT double-fire across replicas; deterministic UUID5 + controller dedup is the safety net, not a load-balancer assumption).
- Replace skeleton env wiring (`DATADIR`, `LISTEN=:9090` only) with the env shape the Spec 3 binary reads: `LISTEN=:9090`, `KAFKA_BROKERS`, `STAGE`, `SENTRY_DSN` (secret), `SENTRY_PROXY`, `TZ=Europe/Berlin`. Remove `boltkv` PVC — the service is stateless across pod restarts.
- Drop skeleton's PVC `datadir` volume (boltkv leftover, no persistent state needed); add `emptyDir` for `/tmp` so a read-only root filesystem is feasible.
- Add a pod security context matching the maintainer watcher precedent (`runAsNonRoot`, `runAsUser: 65534`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`).
- Keep the existing `Service`, `Ingress` (host `recurring-task-creator.<NAMESPACE>.example.com` — covers Spec 4's `/trigger`), `Secret`, and `KafkaUser` (mTLS via strimzi, already-correct skeleton shape). Keep Prometheus scrape via pod annotations (`prometheus.io/scrape: "true"`) — no `ServiceMonitor` CRD; that is not the bborbe convention.

## Problem

The recurring-task-creator binary will be feature-complete after Specs 3 and 4 land, but the `k8s/` directory is still the unmodified go-skeleton it was forked from. The skeleton declares `replicas: 0` (so a naive `kubectl apply` deploys nothing), mounts a `datadir` PVC the binary will never write to (boltkv leftover), sets only the skeleton's two env vars (`LISTEN`, `SENTRY_DSN`) so the binary cannot find Kafka brokers, omits `STAGE` (which `pkg/tick`'s startup-time clock-region check expects and which is also a Sentry-tag input), and omits `TZ=Europe/Berlin` (which Spec 3 makes load-bearing — `time.LoadLocation("Europe/Berlin")` fails fast at constructor time if tzdata is missing, but the env tag matters for log timestamps too).

Without this spec, even a green CI build cannot be deployed: `make buca` will succeed at building the image, but the resulting pod will either (a) not start because the replica count is zero, or (b) start but immediately fail because it cannot resolve `KAFKA_BROKERS`. The migration from `jira-task-creator` cannot complete until the manifests are right.

The skeleton's other defaults — Service, Ingress, Secret, KafkaUser — are already correct for this service's deployment shape (one HTTP port, one TLS host, one Kafka mTLS identity). The work here is targeted manifest customization, not a rewrite.

## Goal

After this work, running `BRANCH=dev make buca` from the deployment worktree builds the image, pushes it to the registry, and triggers a rolling update of a single-replica StatefulSet in the `dev` namespace whose pod (a) reads `KAFKA_BROKERS`, `STAGE=dev`, `SENTRY_DSN`, `SENTRY_PROXY`, and `TZ=Europe/Berlin` from its own env, (b) exposes HTTP on `:9090` for `/healthz`, `/readiness`, `/metrics`, `/setloglevel/{level}`, and `/trigger` (Spec 4), (c) is reachable inside the cluster at `recurring-task-creator.dev:9090` and from outside at `https://recurring-task-creator.dev.example.com/`, (d) is scraped by the cluster's Prometheus via the `prometheus.io/scrape: "true"` pod annotation, (e) is authenticated to Kafka via mTLS using a strimzi `KafkaUser` named `dev-recurring-task-creator` (and `prod-recurring-task-creator` for the prod deploy), and (f) runs as non-root with a read-only root filesystem. The same flow works for `BRANCH=prod` against the `prod` namespace, with no per-namespace YAML branching beyond the `NAMESPACE` template variable already supported by `Makefile.k8s`.

## Non-goals

- Do NOT run `make buca` as part of this spec's verification — deploy is a post-merge runbook step. Verification stops at `kubectl --dry-run` and `kubeval`.
- Do NOT add a `ServiceAccount` or RBAC manifests — this service makes no Kubernetes API calls. The namespace's `default` ServiceAccount is sufficient. If a future consumer demands variation (e.g. a sidecar that watches ConfigMaps), that's a separate spec.
- Do NOT add a `ServiceMonitor` CRD — the bborbe convention across maintainer, agent, jira-task-creator is pod-annotation-based Prometheus discovery (`prometheus.io/scrape: "true"`). The skeleton already uses it.
- Do NOT switch Kafka auth from mTLS (strimzi `KafkaUser`) to SASL username/password — the skeleton's mTLS shape is the cluster's convention; SASL would require a new `Secret` shape, a new `Makefile.env` variable pair, and divergence from every other publisher.
- Do NOT add a PVC — the service has no state that survives pod restart. Spec 3 made `pkg/tick` stateless; the publisher's only output is Kafka.
- Do NOT add leader election, lease coordination, or any "single-fire across replicas" mechanism — invariant; `replicas: 1` plus deterministic UUID5 idempotency is the contract. If a future consumer demands HA, that's a separate spec.
- Do NOT make `TZ` configurable — `Europe/Berlin` is the contract (Spec 3 Non-goal). If a future consumer demands variation, that's a separate spec.
- Do NOT make the HTTP/metrics port configurable per environment — fixed at `9090` to match every other bborbe service. If a future consumer demands variation, that's a separate spec.
- Do NOT split healthz/metrics/HTTP onto multiple ports — invariant; one `LISTEN=:9090` per the convention used by every reference service. If a future consumer demands variation, that's a separate spec.
- Do NOT add a `HorizontalPodAutoscaler` or `PodDisruptionBudget` — single replica by design.
- Do NOT add a `NetworkPolicy` — outside this service's scope; cluster-wide policy is owned elsewhere.
- Do NOT add a Sentry alert manifest (`*-error-critical-alert.yaml` like `jira-task-creator` has) — alerting is post-MVP; if a future consumer demands variation, that's a separate spec.
- Do NOT remove the existing `keel.sh` annotations — auto-redeploy on tag push is the convention.
- Do NOT remove the `imagePullSecrets` `docker-example` reference — the registry requires it.
- Do NOT add any new `Makefile.env` variables beyond what is already required for Kafka mTLS and Sentry — every new env var is a new secret to rotate.

## Desired Behavior

1. The StatefulSet declares `replicas: 1` and is named `recurring-task-creator` in the namespace resolved from the `NAMESPACE` env template variable (`dev` for `BRANCH=dev`, `prod` for `BRANCH=prod`).
2. The pod template has exactly one container named `service`, image `{{"DOCKER_REGISTRY" | env}}/recurring-task-creator:{{"BRANCH" | env}}`, `imagePullPolicy: Always`.
3. The container's env block contains, in this order: `LISTEN=:9090`, `KAFKA_BROKERS={{ "KAFKA_BROKERS" | env }}`, `STAGE={{ "STAGE" | env }}`, `SENTRY_DSN` from `secretKeyRef` (secret `recurring-task-creator`, key `sentry-dsn`), `SENTRY_PROXY={{"SENTRY_PROXY_URL" | env}}`, `TZ=Europe/Berlin`.
4. The container's `args` is exactly `["-v={{"LOGLEVEL" | env}}"]` — matches the bborbe convention; no other args.
5. The container's liveness probe is `httpGet { path: /healthz, port: 9090, scheme: HTTP }` with `initialDelaySeconds: 10`, `timeoutSeconds: 5`, `failureThreshold: 5`, `successThreshold: 1`. The readiness probe is `httpGet { path: /readiness, port: 9090, scheme: HTTP }` with `initialDelaySeconds: 5`, `timeoutSeconds: 5`. (Spec 4 implements `/healthz`; this spec accepts the cross-dependency — both ship in the same branch before deploy.)
6. The container exposes one port: `containerPort: 9090, name: http`.
7. Pod template annotations include `prometheus.io/scrape: "true"`, `prometheus.io/port: "9090"`, `prometheus.io/path: /metrics`, `prometheus.io/scheme: http`. No `ServiceMonitor` CRD is added.
8. Container `securityContext` sets `runAsNonRoot: true`, `runAsUser: 65534`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`. Pod `securityContext` sets `fsGroup: 65534`.
9. Container `resources` are `requests: { cpu: 20m, memory: 20Mi }`, `limits: { cpu: 200m, memory: 100Mi }` — matches the maintainer watcher precedent (closest reference service).
10. Container `volumeMounts` are exactly two: `tmp` at `/tmp` (writable scratch for any library that needs it under read-only root), and no `datadir` mount. The pod's `volumes` block declares `tmp` as an `emptyDir: {}`. No `volumeClaimTemplates` block.
11. The `Service` named `recurring-task-creator` in the same namespace exposes port `9090` named `http`, selecting pods by `app: recurring-task-creator`. ClusterIP type (default).
12. The `Ingress` named `recurring-task-creator` routes the host `recurring-task-creator.{{ "NAMESPACE" | env }}.example.com` to the service's `http` port via traefik, with TLS terminated by `tls-example`.
13. The `Secret` named `recurring-task-creator` contains exactly one key — `sentry-dsn` — sourced from teamvault. No SASL credentials, no other secret keys.
14. The `KafkaUser` (strimzi CRD) named `{{ "NAMESPACE" | env }}-recurring-task-creator` in namespace `strimzi` has `spec.authentication.type: tls`. The pod consumes the resulting client cert via the strimzi-managed mount path (existing cluster convention, not configured per manifest).
15. The StatefulSet declares `serviceName: recurring-task-creator` (matches the Service name, required for headless DNS) and `updateStrategy: { type: RollingUpdate }`.
16. The pod's node affinity requires `node_type In [agent]` — same affinity bborbe uses for maintainer watchers and jira-task-creator (the cluster has agent-class nodes for low-traffic publishers).
17. The StatefulSet declares `keel.sh/policy: force`, `keel.sh/trigger: poll`, `keel.sh/match-tag: "true"`, `keel.sh/pollSchedule: "@every 1m"` — same auto-redeploy contract as every other bborbe service.
18. The `Makefile` under `k8s/` inherits `../Makefile.variables`, `../Makefile.env`, and `../Makefile.k8s` — unchanged from skeleton; no new targets added.
19. The list of YAML files in `k8s/` is exactly: `recurring-task-creator-sts.yaml`, `recurring-task-creator-svc.yaml`, `recurring-task-creator-ing.yaml`, `recurring-task-creator-secret.yaml`, `recurring-task-creator-user.yaml`, plus the unchanged `Makefile`. No new files added; no skeleton files left behind.
20. Rendered manifests for `NAMESPACE=dev`, `BRANCH=dev`, `DOCKER_REGISTRY=docker.example.com`, `STAGE=dev` pass `kubeval` (or `kubeconform`) against the cluster's API versions. Same for `NAMESPACE=prod`.

## Constraints

- The existing template syntax (`{{ "VAR" | env }}`, `{{ ... | teamvaultUrl | base64 }}`) is the project's template engine and MUST be preserved — `Makefile.k8s` from the parent module renders it. Do not switch to Helm, Kustomize, or plain YAML.
- The Service name, StatefulSet name, Secret name, and pod `app` label MUST all be `recurring-task-creator` — DNS, RBAC, and Prometheus discovery all depend on this exact string.
- The KafkaUser MUST live in namespace `strimzi`, not the service namespace — strimzi watches a single namespace for `KafkaUser` resources cluster-wide; this is non-negotiable cluster convention.
- The host pattern `recurring-task-creator.<NAMESPACE>.example.com` MUST match the cluster's traefik TLS SAN list (covered by `tls-example`); deviation breaks TLS.
- The Spec 3 binary will NOT start if `TZ=Europe/Berlin` is missing AND tzdata is absent from the image — the container image MUST include tzdata. (The bborbe Go base image already does; this is asserted, not configured.)
- The probe paths `/healthz` and `/readiness` MUST match what `main.go` registers; Spec 3 already registers `/healthz` and `/readiness` per the existing admin-handler convention. Spec 4 adds `/trigger` but the probes do not depend on it.
- Resource limits MUST stay within `cpu: 200m`, `memory: 100Mi` — matches the maintainer watcher precedent and fits the agent-class node's bin-packing budget.
- The Ingress class MUST be `traefik` and the TLS secret MUST be `tls-example` — cluster convention.
- Reference: `~/Documents/workspaces/coding/docs/k8s-manifest-guide.md` (manifest conventions, env templating, secret handling) — apply throughout.
- Reference: `~/Documents/workspaces/maintainer/watcher/github-pr/k8s/` — closest-shape exemplar (single-replica STS, Kafka publisher, agent-node affinity, read-only root FS, mTLS via KafkaUser). When in doubt, mirror this layout.
- Reference: `docs/dod.md` — the project's Definition of Done. This spec ships YAML, not Go, so the "Code Quality" and "Testing" gates do not apply directly; the "Build & Tooling" gate (no stray `make precommit` regressions) does.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---|---|---|---|---|---|
| Operator runs `BRANCH=dev make apply` (or equivalent) but `KAFKA_BROKERS` env is unset | Template renders an empty string for `KAFKA_BROKERS`; the pod starts but `pkg/tick`'s Kafka producer construction fails; pod enters CrashLoopBackOff | Set `KAFKA_BROKERS` in the deploy worktree's env and re-apply | `kubectl logs` shows the Kafka producer error; pod's `RESTARTS` count climbs; Sentry receives the startup error | Reversible — re-apply with corrected env | N/A (single replica) |
| Strimzi has not yet provisioned the `KafkaUser` cert when the pod first rolls | Pod fails its readiness probe because the Kafka producer cannot authenticate; remains `NotReady` | Wait for strimzi to provision (seconds-to-minutes) — pod becomes Ready automatically; no manual action | `kubectl describe pod` shows readiness probe failures and the strimzi-managed mount path is empty | Reversible — strimzi reconciles | N/A |
| `tls-example` secret expires or is rotated mid-deploy | Existing TLS termination continues for the cached cert; new connections may fail after the cert's actual expiry | Cluster operator's TLS rotation playbook handles this — out of scope for this service | `curl -v https://recurring-task-creator.dev.example.com/healthz` shows TLS error | Reversible at the cluster level | N/A |
| Two pods running simultaneously (e.g. mid-rolling-update overlap) | Both pods perform their initial tick; both publish the same `(slug, date)` set; controller dedups on the deterministic UUID5; one create, one no-op per task. (Same as Spec 3's failure-mode row.) | None needed — idempotent by design | Counter `recurring_tasks_published_total{result="success"}` briefly doubles per task; controller logs show duplicate-skip per second pod | Idempotent | Safe per Spec 2 contract |
| Operator drains the node running the pod | `RollingUpdate` strategy moves the pod to another `node_type=agent` node; on startup the new pod performs its initial tick (Spec 3) and republishes the day's set; controller dedups | None needed | New pod's `glog.V(2)` "tick loop: initial tick complete" line within ~10s of becoming Ready | Idempotent | Safe |
| Pod image pull fails (registry down, tag missing) | Pod enters `ErrImagePull` / `ImagePullBackOff`; previous pod (if any) keeps running | Push the image with the correct tag and let keel.sh re-poll, or `kubectl rollout restart` | `kubectl get pod` shows `ImagePullBackOff`; keel.sh logs the poll failure | Reversible | N/A |
| Operator forgets to update `recurring-task-creator-secret.yaml` after rotating the Sentry DSN in teamvault | New pod starts with the stale DSN baked into the rendered Secret on apply; Sentry events go to the old project until the next apply | Re-run `make apply` to re-render and re-apply the Secret | No automatic detection (Sentry receives events at the old project) | Reversible — re-apply | N/A |
| `kubeval` / `kubeconform` rejects a rendered manifest (e.g. an env block typo introduced during edits) | CI fails at the validation step before any apply; deploy aborts | Fix the manifest, re-run validation locally | Validation tool's stderr names the failing path | Reversible at PR-review time | N/A |
| Pod OOM-killed (memory limit too tight) | Pod restarts; on restart performs initial tick; controller dedups | Raise the memory limit in a follow-up PR; the limit chosen (100Mi) matches the maintainer watcher precedent, so OOM here implies a leak in `pkg/tick` (bug) | `kubectl describe pod` shows `OOMKilled`; counter `kube_pod_container_status_restarts_total` increments | Reversible | Safe — restart re-runs initial tick |
| Read-only root FS blocks a library write at runtime (defensive — Spec 3's deps do not write outside `/tmp`) | Pod logs the write error; pod may crash depending on the library's error handling | Add a writable volume mount in a follow-up PR; or fix the library to honor a `TMPDIR` env | `kubectl logs` shows the write error | Reversible | Safe — restart |

External unavailability (Kafka brokers, image registry, teamvault, strimzi) is covered. Schema drift is irrelevant here (no schemas). Partial-progress crash is covered (pod restart re-runs initial tick). Rate limiting: deploy frequency is operator-driven, not a system load. Resource exhaustion is covered (OOM row + limits choice rationale). Clock skew: irrelevant at the manifest layer — Spec 3 handles it at the application layer.

## Security / Abuse Cases

- The Ingress exposes `/trigger` (Spec 4) and `/setloglevel/{level}` to the public internet (behind the `example.com` TLS gateway). Both endpoints must be protected at the application layer — Spec 4 owns `/trigger` auth; `/setloglevel/{level}` is the bborbe convention's existing behavior and accepts the same authentication model as the rest of the cluster's admin endpoints. This spec does NOT add per-route Ingress auth; the application layer is the trust boundary.
- The KafkaUser identity grants Kafka topic produce permissions; topic ACLs are managed cluster-wide (`KafkaTopic` and `KafkaACL` CRDs outside this service's k8s/ directory). This spec does NOT create or modify ACLs.
- The Sentry DSN is the only secret value in the `Secret` manifest. It is rendered at apply time from teamvault, never committed to git in cleartext.
- No env var contains user input — all env vars are operator-set deploy-time configuration.
- The pod runs as UID 65534 (nobody) on a read-only root FS with all capabilities dropped — defense in depth against container escape.
- The `tmp` emptyDir is writable but not shared across pods; no cross-pod data leak vector.

## Acceptance Criteria

- [ ] `k8s/recurring-task-creator-sts.yaml` declares `spec.replicas: 1` (not 0) — evidence: `grep -nE '^  replicas: 1$' k8s/recurring-task-creator-sts.yaml` returns one match.
- [ ] The StatefulSet env block contains exactly the six env vars listed in Desired Behavior #3, in that order — evidence: `yq '.spec.template.spec.containers[0].env[].name' k8s/recurring-task-creator-sts.yaml` outputs the literal sequence `LISTEN`, `KAFKA_BROKERS`, `STAGE`, `SENTRY_DSN`, `SENTRY_PROXY`, `TZ` and nothing else.
- [ ] `TZ` is set to the literal string `Europe/Berlin` — evidence: `yq '.spec.template.spec.containers[0].env[] | select(.name=="TZ") | .value' k8s/recurring-task-creator-sts.yaml` returns `Europe/Berlin`.
- [ ] `STAGE` is sourced from the template variable `STAGE` — evidence: `grep -nE 'name: STAGE' -A1 k8s/recurring-task-creator-sts.yaml` shows the next line contains `'{{ "STAGE" | env }}'`.
- [ ] `KAFKA_BROKERS` is sourced from the template variable `KAFKA_BROKERS` — evidence: `grep -nE 'name: KAFKA_BROKERS' -A1 k8s/recurring-task-creator-sts.yaml` shows the next line contains `'{{ "KAFKA_BROKERS" | env }}'`.
- [ ] `SENTRY_DSN` is sourced from `secretKeyRef` with `name: recurring-task-creator` and `key: sentry-dsn` — evidence: `yq '.spec.template.spec.containers[0].env[] | select(.name=="SENTRY_DSN") | .valueFrom.secretKeyRef' k8s/recurring-task-creator-sts.yaml` returns the object `{name: recurring-task-creator, key: sentry-dsn}`.
- [ ] The StatefulSet declares no `BATCH_SIZE` or `DATADIR` env var, no `boltkv` reference anywhere — evidence: `grep -nE 'BATCH_SIZE|DATADIR|boltkv' k8s/` returns no matches.
- [ ] The StatefulSet declares no `volumeClaimTemplates` block — evidence: `yq '.spec.volumeClaimTemplates' k8s/recurring-task-creator-sts.yaml` returns `null` (or the key is absent).
- [ ] The pod template declares one volume named `tmp` of type `emptyDir`, mounted at `/tmp` — evidence: `yq '.spec.template.spec.volumes[]' k8s/recurring-task-creator-sts.yaml` shows exactly one entry `{name: tmp, emptyDir: {}}` AND `yq '.spec.template.spec.containers[0].volumeMounts[]' k8s/recurring-task-creator-sts.yaml` shows exactly one entry `{name: tmp, mountPath: /tmp}`.
- [ ] Liveness probe hits `/healthz` on port `9090` with `initialDelaySeconds: 10`, `timeoutSeconds: 5`, `failureThreshold: 5` — evidence: `yq '.spec.template.spec.containers[0].livenessProbe' k8s/recurring-task-creator-sts.yaml` matches exactly the values in Desired Behavior #5.
- [ ] Readiness probe hits `/readiness` on port `9090` with `initialDelaySeconds: 5`, `timeoutSeconds: 5` — evidence: `yq '.spec.template.spec.containers[0].readinessProbe' k8s/recurring-task-creator-sts.yaml` matches Desired Behavior #5.
- [ ] Container exposes exactly one port `9090` named `http` — evidence: `yq '.spec.template.spec.containers[0].ports[]' k8s/recurring-task-creator-sts.yaml` returns exactly one entry `{containerPort: 9090, name: http}`.
- [ ] Container `securityContext` has `runAsNonRoot: true`, `runAsUser: 65534`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]` — evidence: `yq '.spec.template.spec.containers[0].securityContext' k8s/recurring-task-creator-sts.yaml` returns these five fields with the specified values.
- [ ] Pod `securityContext` has `fsGroup: 65534` — evidence: `yq '.spec.template.spec.securityContext.fsGroup' k8s/recurring-task-creator-sts.yaml` returns `65534`.
- [ ] Container resources are `requests: {cpu: 20m, memory: 20Mi}`, `limits: {cpu: 200m, memory: 100Mi}` — evidence: `yq '.spec.template.spec.containers[0].resources' k8s/recurring-task-creator-sts.yaml` matches exactly.
- [ ] Pod template annotations include `prometheus.io/scrape: "true"`, `prometheus.io/port: "9090"`, `prometheus.io/path: /metrics`, `prometheus.io/scheme: http` — evidence: `yq '.spec.template.metadata.annotations' k8s/recurring-task-creator-sts.yaml` contains these four keys with these values.
- [ ] StatefulSet annotations include all four `keel.sh/*` keys (`policy: force`, `trigger: poll`, `match-tag: "true"`, `pollSchedule: "@every 1m"`) — evidence: `yq '.metadata.annotations' k8s/recurring-task-creator-sts.yaml` contains these four `keel.sh/*` keys with these values.
- [ ] Node affinity requires `node_type In [agent]` — evidence: `yq '.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0]' k8s/recurring-task-creator-sts.yaml` returns `{key: node_type, operator: In, values: [agent]}`.
- [ ] `imagePullSecrets` lists `docker-example` (or whichever name the parent module's `Makefile.env` uses for the quant registry — verify against `~/Documents/workspaces/maintainer/watcher/github-pr/k8s/maintainer-watcher-github-pr-sts.yaml`) — evidence: `yq '.spec.template.spec.imagePullSecrets[0].name' k8s/recurring-task-creator-sts.yaml` returns the convention name.
- [ ] StatefulSet `serviceName` is `recurring-task-creator` — evidence: `yq '.spec.serviceName' k8s/recurring-task-creator-sts.yaml` returns `recurring-task-creator`.
- [ ] StatefulSet `updateStrategy.type` is `RollingUpdate` — evidence: `yq '.spec.updateStrategy.type' k8s/recurring-task-creator-sts.yaml` returns `RollingUpdate`.
- [ ] `k8s/recurring-task-creator-svc.yaml` declares port `9090` named `http`, selector `app: recurring-task-creator` — evidence: `yq '.spec.ports[0]' k8s/recurring-task-creator-svc.yaml` returns `{name: http, port: 9090}` AND `yq '.spec.selector' k8s/recurring-task-creator-svc.yaml` returns `{app: recurring-task-creator}`.
- [ ] `k8s/recurring-task-creator-ing.yaml` routes host `recurring-task-creator.{{ "NAMESPACE" | env }}.example.com` to service `recurring-task-creator` port `http`, with TLS via `tls-example` and `ingressClassName: traefik` — evidence: `grep -nE 'recurring-task-creator\.\{\{ "NAMESPACE" \| env \}\}\.quant\.benjamin-borbe\.de' k8s/recurring-task-creator-ing.yaml` returns at least two matches (host rule + TLS host); `grep -nE 'tls-example|ingressClassName: .traefik.' k8s/recurring-task-creator-ing.yaml` returns at least two matches.
- [ ] `k8s/recurring-task-creator-secret.yaml` declares exactly one data key `sentry-dsn` — evidence: `yq '.data | keys' k8s/recurring-task-creator-secret.yaml` returns `[sentry-dsn]` (exactly one element).
- [ ] `k8s/recurring-task-creator-user.yaml` declares a `KafkaUser` in namespace `strimzi` with `spec.authentication.type: tls` — evidence: `yq '.kind' k8s/recurring-task-creator-user.yaml` returns `KafkaUser` AND `yq '.metadata.namespace' k8s/recurring-task-creator-user.yaml` returns `strimzi` AND `yq '.spec.authentication.type' k8s/recurring-task-creator-user.yaml` returns `tls`.
- [ ] No `ServiceAccount`, `Role`, `RoleBinding`, `ClusterRole`, `ClusterRoleBinding`, `ServiceMonitor`, `HorizontalPodAutoscaler`, `PodDisruptionBudget`, or `NetworkPolicy` YAML files exist in `k8s/` — evidence: `grep -lE '^kind: (ServiceAccount|Role|RoleBinding|ClusterRole|ClusterRoleBinding|ServiceMonitor|HorizontalPodAutoscaler|PodDisruptionBudget|NetworkPolicy)$' k8s/*.yaml` returns no matches.
- [ ] The `k8s/` directory contains exactly six entries: `Makefile`, `recurring-task-creator-ing.yaml`, `recurring-task-creator-secret.yaml`, `recurring-task-creator-sts.yaml`, `recurring-task-creator-svc.yaml`, `recurring-task-creator-user.yaml` — evidence: `ls k8s/ | sort` returns exactly this six-line list.
- [ ] The rendered manifests pass `kubeconform` (or `kubeval`) for `NAMESPACE=dev` and `NAMESPACE=prod` — evidence: a one-shot validation script (or a `make validate` target if added to `Makefile.k8s` by the prompt-writer) exits 0 for both namespaces. Acceptable evidence shape: a documented command in the PR description that the reviewer can re-run, with exit code 0.
- [ ] `make precommit` exits 0 in the recurring-task-creator module — evidence: exit code 0. (No Go changes in this spec, but the project's lint and license-header checks still run over the changed YAML — defensive.)

Scenario coverage: NO new scenario. Manifest correctness is verified by file-content inspection plus `kubeconform` parse-and-schema validation; the real-cluster apply is a post-merge runbook step (operator runs `BRANCH=dev make buca` from `~/Documents/workspaces/recurring-task-creator-mvp-dev` or equivalent deployment worktree, then verifies pod Ready + Prometheus scrape + first tick logged within 60s). Adding a scenario here would require a kind/minikube cluster plus a fake strimzi plus a fake Kafka — that is the deploy-runbook's verification, not this spec's.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-mvp
make precommit
```

Expected: exit code 0; YAML files are well-formed; no forbidden tokens (`BATCH_SIZE`, `DATADIR`, `boltkv`) appear in `k8s/`; license-header check (if applied to YAML in this repo) passes.

Manifest-level validation (run by the prompt-writer; documented in the PR description so a reviewer can repeat):

```
cd ~/Documents/workspaces/recurring-task-creator-mvp/k8s
for ns in dev prod; do
  for f in *.yaml; do
    NAMESPACE=$ns BRANCH=$ns DOCKER_REGISTRY=docker.example.com STAGE=$ns \
      KAFKA_BROKERS=kafka.strimzi:9093 SENTRY_DSN_KEY=fake LOGLEVEL=0 \
      SENTRY_PROXY_URL=http://fake RANDOM=1 \
      <render-tool> "$f" | kubeconform -strict -ignore-missing-schemas -
  done
done
```

Expected: exit code 0 for every `(namespace, file)` pair. The exact `<render-tool>` invocation is whatever `Makefile.k8s` uses internally to expand `{{ ... | env }}` — the prompt-writer extracts it from the Makefile and pastes the working command in the PR description.

Smoke check post-merge (NOT part of this spec's verification; documented in the runbook follow-up):

```
cd ~/Documents/workspaces/recurring-task-creator-mvp-dev  # or equivalent deployment worktree
git pull && git merge master
cd k8s && BRANCH=dev make buca
kubectlquant -n dev get pod -l app=recurring-task-creator
kubectlquant -n dev logs -l app=recurring-task-creator --tail=50 | grep 'tick loop'
```

## Do-Nothing Option

Without this spec, the binary compiles and tests green but cannot be deployed: the skeleton's `replicas: 0` means `make buca` produces a release artifact that schedules zero pods, and even if the operator hand-edits the replica count, the missing `KAFKA_BROKERS` and `STAGE` env vars guarantee a CrashLoopBackOff on first roll. The existing `jira-task-creator` keeps firing in production, so the user is not blocked from receiving tasks — but the migration goal of "retire jira-task-creator" is stuck one step short of the finish line for as long as the manifests stay un-customized. Folding this work into Spec 4 (HTTP `/trigger`) would force the prompt-writer to ship YAML edits in the same diff as Go code, which makes review harder and recombines two distinct concerns (HTTP handler design vs deploy shape). Splitting it later, after the operator has hand-edited manifests in a one-off PR, just creates drift between what is deployed and what is in the repo. Recommendation: do this spec immediately after Spec 4 lands, so the deploy runbook is unblocked and the migration completes in one round.
