---
status: completed
spec: [004-k8s-manifests]
summary: 'Rewrote k8s/recurring-task-creator-sts.yaml to the spec''s required shape (replicas: 1, agent-node affinity, env block in spec order with TZ=Europe/Berlin, non-root pod with read-only root FS, emptyDir-only volumes), updated example.env with the 5 missing templating env vars, appended a feat: bullet to CHANGELOG.md, and confirmed all 28 spec AC greps pass and make precommit exits 0'
container: recurring-task-creator-mvp-exec-009-k8s-manifest-customization
dark-factory-version: v0.177.1
created: "2026-06-14T12:16:41Z"
queued: "2026-06-14T12:26:23Z"
started: "2026-06-14T12:45:51Z"
completed: "2026-06-14T12:52:12Z"
branch: dark-factory/k8s-manifests
---

<summary>
- The pod now actually runs: a single replica StatefulSet replaces the skeleton's `replicas: 0`, so a green deploy produces a real pod instead of zero pods.
- The pod's environment matches what the Spec 3 binary reads: `LISTEN=:9090`, `KAFKA_BROKERS`, `STAGE`, `SENTRY_DSN` (from a Secret), `SENTRY_PROXY`, `TZ=Europe/Berlin` — in that order, with no leftover skeleton's `DATADIR` and no invented `BATCH_SIZE`.
- The skeleton's boltkv `datadir` PVC and its `volumeClaimTemplates` block are gone; the pod has one writable `emptyDir` mounted at `/tmp` so a read-only root filesystem is feasible.
- The pod runs as UID 65534 (nobody) on a read-only root filesystem with all Linux capabilities dropped — defense in depth against container escape.
- Resource limits match the maintainer watcher precedent (20m/20Mi requests, 200m/100Mi limits) so the pod fits the agent-class node's bin-packing budget.
- The pod schedules only on `node_type=agent` nodes, matching every other bborbe publisher.
- `k8s/Makefile`, `k8s/recurring-task-creator-svc.yaml`, `k8s/recurring-task-creator-ing.yaml`, `k8s/recurring-task-creator-secret.yaml`, and `k8s/recurring-task-creator-user.yaml` are unchanged from the skeleton — they were already correct.
- The `k8s/` directory contains exactly six entries: the five YAML files plus the `Makefile`. No `ServiceAccount`, `Role`, `ServiceMonitor`, `HPA`, `PDB`, or `NetworkPolicy` was added.
- `example.env` exports the deploy-loop env vars the manifest templating reads (`STAGE`, `SENTRY_DSN_KEY`, `SENTRY_PROXY_URL`, `LOGLEVEL`, `RANDOM`) without removing any existing key.
- `make precommit` exits 0; rendered manifests pass `kubeconform` for both `dev` and `prod` namespaces when re-run via the documented PR-description command.
</summary>

<objective>
Customize the inherited go-skeleton `k8s/` manifests so the Spec 1-4 binary can be deployed to the bborbe quant cluster as the `recurring-task-creator` service in `dev` and `prod` namespaces. The change is YAML-only: rewrite `k8s/recurring-task-creator-sts.yaml` to match the spec's env block, security context, resource limits, emptyDir-only volume layout, and `node_type=agent` affinity; confirm the four other manifests and the `k8s/Makefile` are already correct; add the missing env vars to `example.env`; verify with `make precommit` plus the file-content greps and a one-shot `kubeconform` loop documented in the PR description.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions.

Read these files in full before editing (paths under `/workspace/`):

- `k8s/recurring-task-creator-sts.yaml` (101 lines) — the file you are rewriting. Key points it currently has that you will remove: `replicas: 0` (line 13), `DATADIR` env var (lines 52-53), `volumeMounts: datadir at /data` (lines 84-86), `volumeClaimTemplates` block (lines 91-100), `nodeAffinity.values: ["{{ \"NAMESPACE\" | env }}"]` (lines 33-37). Key points it currently has that you will preserve verbatim: `keel.sh/*` annotations (lines 7-10), `serviceName: recurring-task-creator` (line 17), `imagePullSecrets: [docker-example]` (lines 87-88), `updateStrategy.type: RollingUpdate` (lines 89-90), `args: ["-v={{\"LOGLEVEL\" | env}}"]` (line 41), `prometheus.io/*` pod annotations (lines 20-25), `image` template (line 56), `imagePullPolicy: Always` (line 57), `livenessProbe` block (lines 58-66), `readinessProbe` block (lines 70-76), `ports: containerPort 9090 name http` (lines 67-69), `random: '{{ "RANDOM" | env }}'` annotations (lines 11 and 25).
- `k8s/recurring-task-creator-svc.yaml` (12 lines) — already correct per spec Desired Behavior #11. No changes.
- `k8s/recurring-task-creator-ing.yaml` (26 lines) — already correct per spec Desired Behavior #12. No changes.
- `k8s/recurring-task-creator-secret.yaml` (9 lines) — already correct per spec Desired Behavior #13. No changes.
- `k8s/recurring-task-creator-user.yaml` (11 lines) — already correct per spec Desired Behavior #14. No changes.
- `k8s/Makefile` (3 lines) — already correct per spec Desired Behavior #18. No changes.
- `example.env` (6 lines) — needs five new keys added; see requirement #9.
- `Makefile.k8s` (4 lines) — defines the `apply` target using `teamvault-config-parser`. The `kubeconform` loop in `<verification>` mirrors the templating pattern from this file (`source ${ROOTDIR}/example.env && teamvault-config-parser -teamvault-config="${TEAMVAULT}" -logtostderr -v=0`).
- `Makefile.variables` (5 lines) — defines `BRANCH`, `HOSTNAME`, `ROOTDIR`, `TEAMVAULT` defaults.
- `Makefile.precommit` (88 lines) — `make precommit` runs `ensure format generate test check addlicense`. The `test`, `lint`, `vet`, `errcheck`, `vulncheck`, `gosec` targets are Go-only; `trivy` scans the filesystem (so it sees the YAML); `addlicense` is Go-only.
- `CHANGELOG.md` — append a `feat:` bullet under `## Unreleased` (currently has 2 entries from Spec 1 and Spec 2; do not replace).
- `Dockerfile` — confirms the image embeds tzdata (the spec's `TZ=Europe/Berlin` env var matters for log timestamps because the binary uses `time.LoadLocation` at startup; this is asserted by the spec, not configured by this prompt).

Coding-plugin doc (in-container path; the YOLO container mounts the coding plugin under `/home/node/`):

- `/home/node/.claude/plugins/marketplaces/coding/docs/k8s-manifest-guide.md` — rules for: workload-kind semantics, filename convention (`<name>-<suffix>.yaml` with `deploy`/`sts`/`svc`/`ing`/`secret`/etc.), per-env templating with `{{ "KEY" | env }}`, annotation hygiene (only add what a controller reads), standard `k8s/Makefile` shape (include `Makefile.variables` + `Makefile.k8s`), `imagePullPolicy: Always` pairs with mutable tags.

The skeleton's current env block order is wrong for the spec: `LISTEN`, `SENTRY_DSN`, `SENTRY_PROXY`, `DATADIR`, `KAFKA_BROKERS`. The new env block (per spec DB #3) is `LISTEN`, `KAFKA_BROKERS`, `STAGE`, `SENTRY_DSN`, `SENTRY_PROXY`, `TZ` — note `KAFKA_BROKERS` moves up from position 5 to position 2, and `DATADIR` is removed entirely.

The skeleton's current node affinity uses `values: ["{{ \"NAMESPACE\" | env }}"]` (resolves to `dev` or `prod`). The spec wants the literal `values: [agent]` — agent-class nodes host every other bborbe publisher, regardless of the service's deploy namespace. This is a hard change; the executor must NOT preserve the template form.

The skeleton's current resource `limits: { cpu: 500m, memory: 50Mi }`. The spec wants `limits: { cpu: 200m, memory: 100Mi }` — both numbers change. `requests: { cpu: 20m, memory: 20Mi }` is preserved.

The skeleton's current PVC is `storageClassName: local-path, accessModes: [ReadWriteOnce], storage: 1Gi`. The entire `volumeClaimTemplates` block is removed; the only volume left is `tmp` of type `emptyDir: {}`.

The `k8s/Makefile` is frozen at 3 lines. Do NOT add a `make validate` target — the spec's AC says "a documented command in the PR description that the reviewer can re-run," not a Makefile target.

This prompt touches YAML, `example.env`, and `CHANGELOG.md` only. No Go source changes. No `go.mod`/`go.sum` changes. No new files under `k8s/`. The skeleton's `keel.sh/*` annotations, `imagePullSecrets`, `serviceName`, `updateStrategy`, `prometheus.io/*` pod annotations, and `random: '{{ "RANDOM" | env }}'` annotations are all preserved as-is from the skeleton.
</context>

<requirements>

## 1. Rewrite `k8s/recurring-task-creator-sts.yaml`

The skeleton file is 101 lines. Replace it with the manifest below. Every value in this manifest is demanded by the spec; do not invent knobs.

The full new file content (replace the whole file with this):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: recurring-task-creator
  namespace: '{{ "NAMESPACE" | env }}'
  annotations:
    keel.sh/policy: force
    keel.sh/trigger: poll
    keel.sh/match-tag: "true"
    keel.sh/pollSchedule: "@every 1m"
    random: '{{ "RANDOM" | env }}'
spec:
  replicas: 1
  selector:
    matchLabels:
      app: recurring-task-creator
  serviceName: recurring-task-creator
  template:
    metadata:
      annotations:
        prometheus.io/path: /metrics
        prometheus.io/port: "9090"
        prometheus.io/scheme: http
        prometheus.io/scrape: "true"
        random: '{{ "RANDOM" | env }}'
      labels:
        app: recurring-task-creator
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node_type
                    operator: In
                    values:
                      - agent
      securityContext:
        fsGroup: 65534
      containers:
        - name: service
          args:
            - -v={{"LOGLEVEL" | env}}
          env:
            - name: LISTEN
              value: ':9090'
            - name: KAFKA_BROKERS
              value: '{{ "KAFKA_BROKERS" | env }}'
            - name: STAGE
              value: '{{ "STAGE" | env }}'
            - name: SENTRY_DSN
              valueFrom:
                secretKeyRef:
                  key: sentry-dsn
                  name: recurring-task-creator
            - name: SENTRY_PROXY
              value: '{{"SENTRY_PROXY_URL" | env}}'
            - name: TZ
              value: Europe/Berlin
          image: '{{"DOCKER_REGISTRY" | env}}/recurring-task-creator:{{"BRANCH" | env}}'
          imagePullPolicy: Always
          livenessProbe:
            failureThreshold: 5
            httpGet:
              path: /healthz
              port: 9090
              scheme: HTTP
            initialDelaySeconds: 10
            successThreshold: 1
            timeoutSeconds: 5
          ports:
            - containerPort: 9090
              name: http
          readinessProbe:
            httpGet:
              path: /readiness
              port: 9090
              scheme: HTTP
            initialDelaySeconds: 5
            timeoutSeconds: 5
          resources:
            limits:
              cpu: 200m
              memory: 100Mi
            requests:
              cpu: 20m
              memory: 20Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            runAsUser: 65534
          volumeMounts:
            - mountPath: /tmp
              name: tmp
      imagePullSecrets:
        - name: docker-example
      volumes:
        - name: tmp
          emptyDir: {}
  updateStrategy:
    type: RollingUpdate
```

After writing the new file, run these greps to confirm forbidden tokens are gone:

```bash
grep -nE 'BATCH_SIZE|DATADIR|boltkv' /workspace/k8s/recurring-task-creator-sts.yaml
grep -nE 'volumeClaimTemplates' /workspace/k8s/recurring-task-creator-sts.yaml
grep -nE 'datadir' /workspace/k8s/recurring-task-creator-sts.yaml
```

All three must return no matches. (The first grep is one of the spec's AC checks; running it against the file specifically — before the directory-wide AC grep — gives a tighter signal that the rewrite removed the right tokens.)

## 2. Confirm `k8s/recurring-task-creator-svc.yaml` is unchanged

This file is already correct per spec Desired Behavior #11. Do not edit it. Verify by reading the file and confirming: `kind: Service`, `name: recurring-task-creator`, `namespace: '{{ "NAMESPACE" | env }}'`, port `9090` named `http`, selector `app: recurring-task-creator`. The skeleton's `clusterIP` field is absent (default), which is correct.

## 3. Confirm `k8s/recurring-task-creator-ing.yaml` is unchanged

This file is already correct per spec Desired Behavior #12. Do not edit it. Verify by reading the file and confirming: `kind: Ingress`, `name: recurring-task-creator`, `ingressClassName: 'traefik'`, host `recurring-task-creator.{{ "NAMESPACE" | env }}.example.com`, backend service `recurring-task-creator` port `http`, `tls.secretName: tls-example`, traefik annotations `websecure` + `tls: "true"`.

## 4. Confirm `k8s/recurring-task-creator-secret.yaml` is unchanged

This file is already correct per spec Desired Behavior #13. Do not edit it. Verify by reading the file and confirming: `kind: Secret`, `type: Opaque`, `name: recurring-task-creator`, exactly one `data` key `sentry-dsn` sourced from `'{{ "SENTRY_DSN_KEY" | env | teamvaultUrl | base64 }}'`.

## 5. Confirm `k8s/recurring-task-creator-user.yaml` is unchanged

This file is already correct per spec Desired Behavior #14. Do not edit it. Verify by reading the file and confirming: `kind: KafkaUser` (apiVersion `kafka.strimzi.io/v1beta2`), `name: '{{ "NAMESPACE" | env }}-recurring-task-creator'`, `namespace: strimzi`, label `strimzi.io/cluster: my-cluster`, `spec.authentication.type: tls`.

## 6. Confirm `k8s/Makefile` is unchanged

This file is already correct per spec Desired Behavior #18. Do not edit it. Verify by reading the file and confirming three `include` lines: `../Makefile.variables`, `../Makefile.env`, `../Makefile.k8s`.

## 7. Confirm `k8s/` directory contains exactly six entries

Run:

```bash
ls /workspace/k8s/ | sort
```

Expected output (six lines, exact):

```
Makefile
recurring-task-creator-ing.yaml
recurring-task-creator-secret.yaml
recurring-task-creator-sts.yaml
recurring-task-creator-svc.yaml
recurring-task-creator-user.yaml
```

If the listing is anything other than these six lines, STOP and report the discrepancy in the completion report under `## Improvements` (category: PROMPT). Do not delete or rename files to make the listing match — a mismatch means a hidden file (e.g. `.bak`, `*-deploy.yaml` leftover) or a missing manifest. Investigate, then either remove the leftover (if it's clearly a skeleton artifact with no deploy value, e.g. an empty `*.bak`) or report it.

## 8. Verify forbidden tokens are absent across the whole `k8s/` tree

Run:

```bash
grep -rnE 'BATCH_SIZE|DATADIR|boltkv' /workspace/k8s/
```

Expected: no output, exit code 1. This is a spec AC check.

Also run:

```bash
grep -lE '^kind: (ServiceAccount|Role|RoleBinding|ClusterRole|ClusterRoleBinding|ServiceMonitor|HorizontalPodAutoscaler|PodDisruptionBudget|NetworkPolicy)$' /workspace/k8s/*.yaml
```

Expected: no output, exit code 1. This is a spec AC check that no extra resource kinds were added.

## 9. Update `example.env` to add the five missing env vars

Current `example.env` (6 lines) exports `KAFKA_BROKERS`, `SKELETON_PORT`, `DOCKER_REGISTRY`, `IMAGE`, `CLUSTER_CONTEXT`. The skeleton's `k8s/recurring-task-creator-sts.yaml` reads FIVE more env vars from the templating layer that `example.env` does not export: `STAGE`, `SENTRY_DSN_KEY`, `SENTRY_PROXY_URL`, `LOGLEVEL`, and `RANDOM`. Without these exports, the deploy loop (`make apply` in the deployment worktree) would render the templating placeholders as empty strings, and the resulting manifest would either fail schema validation (`STAGE`, `SENTRY_DSN_KEY`, `SENTRY_PROXY_URL`, `LOGLEVEL` are all referenced in the templating syntax and would render as empty if unset) or break keel.sh's poll-loop cache invalidation (`RANDOM`).

The skeleton's `Makefile.k8s` `apply` target uses `source ${ROOTDIR}/example.env`, so every key in the manifest's `{{ "KEY" | env }}` template must be exported by `example.env`. The new file (replace the existing 6 lines with this 11-line file):

```bash
export STAGE=dev
export SENTRY_DSN_KEY=changeme-teamvault-sentry-dsn-key
export SENTRY_PROXY_URL=http://sentry-proxy.quant.svc.cluster.local:8080
export LOGLEVEL=0
export RANDOM=$$(date +%s)
export KAFKA_BROKERS=localhost:9092
export SKELETON_PORT=8080
export DOCKER_REGISTRY=docker.io
export IMAGE=bborbe/recurring-task-creator
export CLUSTER_CONTEXT=test
```

Notes for the executor:
- `STAGE=dev` matches the spec's "dev deploy defaults" convention. The deploy worktree for `prod` will override this with `export STAGE=prod` in its own `example.env` (the worktree-level file shadows the repo-level one).
- `SENTRY_DSN_KEY=changeme-teamvault-sentry-dsn-key` is a placeholder. The real DSN is resolved at apply time by `teamvault-config-parser` from the teamvault entry named by `SENTRY_DSN_KEY` (see `k8s/recurring-task-creator-secret.yaml`'s `'{{ "SENTRY_DSN_KEY" | env | teamvaultUrl | base64 }}'`).
- `SENTRY_PROXY_URL=http://sentry-proxy.quant.svc.cluster.local:8080` is a placeholder. Override per namespace as needed.
- `LOGLEVEL=0` matches the skeleton's convention (0 = `glog` INFO; higher numbers add V-level verbosity).
- `RANDOM=$$(date +%s)` uses `$$` so the shell in `make` does not expand the command at parse time — the date is evaluated every time `source example.env` runs. This is what keel.sh needs to invalidate its poll-loop cache.
- The existing five keys (`KAFKA_BROKERS`, `SKELETON_PORT`, `DOCKER_REGISTRY`, `IMAGE`, `CLUSTER_CONTEXT`) are preserved in their original form. Do NOT remove them.
- The file starts with `export STAGE=...` because the spec's env block reads `STAGE` first, and reading the file top-down matches the dependency order of the manifest.

## 10. Run `make precommit` and confirm exit code 0

From `/workspace/`:

```bash
make precommit
```

This runs `ensure format generate test check addlicense` per `Makefile.precommit`. The `test`, `lint`, `vet`, `errcheck`, `gosec`, `vulncheck` targets are Go-only and will pass because this prompt makes no Go changes. The `trivy` target scans the filesystem including YAML and should pass (the skeleton's `.trivyignore` is at the repo root and pre-existing). The `addlicense` target is Go-only and should pass.

If `make precommit` exits non-zero, fix the issue and re-run. If the failure is YAML-related (a syntax error in your rewrite, a license header, a trivy secret detection), fix the manifest. If the failure is Go-related and unrelated to your YAML changes, document it in `## Improvements` (category: PROMPT) and do NOT broaden scope to fix it.

## 11. Run the AC greps from the spec against the new manifest

The 28 AC greps from `/workspace/specs/in-progress/004-k8s-manifests.md` (the `## Acceptance Criteria` section) are the load-bearing correctness check. Run each one and report the exit code in the completion report. Group them by file:

```bash
# sts.yaml greps (most of the AC list)
grep -nE '^  replicas: 1$' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].env[].name' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].env[] | select(.name=="TZ") | .value' /workspace/k8s/recurring-task-creator-sts.yaml
grep -nE 'name: STAGE' -A1 /workspace/k8s/recurring-task-creator-sts.yaml
grep -nE 'name: KAFKA_BROKERS' -A1 /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].env[] | select(.name=="SENTRY_DSN") | .valueFrom.secretKeyRef' /workspace/k8s/recurring-task-creator-sts.yaml
grep -rnE 'BATCH_SIZE|DATADIR|boltkv' /workspace/k8s/
yq '.spec.volumeClaimTemplates' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.volumes[]' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].volumeMounts[]' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].livenessProbe' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].readinessProbe' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].ports[]' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].securityContext' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.securityContext.fsGroup' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.containers[0].resources' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.metadata.annotations' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.metadata.annotations' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0]' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.template.spec.imagePullSecrets[0].name' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.serviceName' /workspace/k8s/recurring-task-creator-sts.yaml
yq '.spec.updateStrategy.type' /workspace/k8s/recurring-task-creator-sts.yaml

# svc.yaml greps
yq '.spec.ports[0]' /workspace/k8s/recurring-task-creator-svc.yaml
yq '.spec.selector' /workspace/k8s/recurring-task-creator-svc.yaml

# ing.yaml greps
grep -nE 'recurring-task-creator\.\{\{ "NAMESPACE" \| env \}\}\.quant\.benjamin-borbe\.de' /workspace/k8s/recurring-task-creator-ing.yaml
grep -nE 'tls-example|ingressClassName: .traefik.' /workspace/k8s/recurring-task-creator-ing.yaml

# secret.yaml grep
yq '.data | keys' /workspace/k8s/recurring-task-creator-secret.yaml

# user.yaml greps
yq '.kind' /workspace/k8s/recurring-task-creator-user.yaml
yq '.metadata.namespace' /workspace/k8s/recurring-task-creator-user.yaml
yq '.spec.authentication.type' /workspace/k8s/recurring-task-creator-user.yaml

# directory greps
ls /workspace/k8s/ | sort
grep -lE '^kind: (ServiceAccount|Role|RoleBinding|ClusterRole|ClusterRoleBinding|ServiceMonitor|HorizontalPodAutoscaler|PodDisruptionBudget|NetworkPolicy)$' /workspace/k8s/*.yaml
```

The exact match expectations are in the spec's `## Acceptance Criteria` section — read it before running. The executor's completion report must include the exit code of every grep above. If `yq` is not available in the YOLO container, fall back to `grep -nE` against the literal string forms (e.g. `grep -nE 'allowPrivilegeEscalation: false' /workspace/k8s/recurring-task-creator-sts.yaml` instead of the `yq` shape).

## 12. Document the `kubeconform` validation command in the PR description

The one-shot validation command (re-runnable by the PR reviewer) goes in the PR description body, not in the repo. The command is:

```bash
cd /workspace/k8s && for ns in dev prod; do
  for f in recurring-task-creator-sts.yaml recurring-task-creator-svc.yaml recurring-task-creator-ing.yaml recurring-task-creator-secret.yaml recurring-task-creator-user.yaml; do
    NAMESPACE=$ns BRANCH=$ns DOCKER_REGISTRY=docker.benjamin-borbe.de STAGE=$ns \
      KAFKA_BROKERS=kafka.strimzi:9093 SENTRY_DSN_KEY=fake LOGLEVEL=0 \
      SENTRY_PROXY_URL=http://fake RANDOM=1 \
      teamvault-config-parser -teamvault-config="${TEAMVAULT:-/dev/null}" -logtostderr -v=0 \
        < "$f" | kubeconform -strict -ignore-missing-schemas -
  done
done
```

Notes for the executor:
- The `teamvault-config-parser -teamvault-config="${TEAMVAULT:-/dev/null}" -logtostderr -v=0` invocation is the exact templating pattern from `/workspace/Makefile.k8s`'s `apply` target. `${TEAMVAULT:-/dev/null}` makes the command runnable without a real `~/.teamvault.json` (the `sentry-dsn` field is the only one that needs teamvault; the others are plain env substitutions).
- The dummy values (`fake`, `http://fake`, `kafka.strimzi:9093`) are not used for connect-time validation — they exist only to make the templating step produce valid output. `kubeconform` only checks the rendered YAML's schema and field validity, not the data values.
- If `kubeconform` is not installed in the YOLO container (likely — it is not in `/usr/local/bin`), do not install it as part of this prompt. Document the command in the PR description and verify it is well-formed (parses with `bash -n`) and that the templating step works locally:
  ```bash
  NAMESPACE=dev BRANCH=dev DOCKER_REGISTRY=docker.benjamin-borbe.de STAGE=dev \
    KAFKA_BROKERS=kafka.strimzi:9093 SENTRY_DSN_KEY=fake LOGLEVEL=0 \
    SENTRY_PROXY_URL=http://fake RANDOM=1 \
    teamvault-config-parser -teamvault-config="/dev/null" -logtostderr -v=0 \
      < /workspace/k8s/recurring-task-creator-sts.yaml
  ```
  Expected: a rendered YAML printed to stdout, no stderr, exit code 0. If `teamvault-config-parser` is not installed in the YOLO container, run the bare `kubectl apply --dry-run=client -f -` check on the raw `k8s/*.yaml` files instead (without templating) and document the limitation in the PR description.
- Do NOT add a `make validate` target to `k8s/Makefile`. The spec explicitly accepts "a documented command in the PR description that the reviewer can re-run" — no Makefile target is needed.

## 13. Update `CHANGELOG.md`

Append ONE bullet to the `## Unreleased` section in `/workspace/CHANGELOG.md`. The bullet must be in the changelog format defined in `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`:

```
- feat: Customize k8s/ manifests for recurring-task-creator deploy: single-replica StatefulSet, env block (LISTEN/KAFKA_BROKERS/STAGE/SENTRY_DSN/SENTRY_PROXY/TZ=Europe/Berlin), non-root pod with read-only root FS, agent-node affinity, emptyDir-only volumes, example.env exports templating env vars
```

Do NOT remove the existing two bullets (`pkg/publisher` and `pkg/schedule`). Do NOT add multiple bullets for the same change. Do NOT use the prompt filename as the entry text. The bullet must be specific — name the manifest files, the env var set, and the security posture; do not write "feat: update k8s manifests".
</requirements>

<constraints>
- The `{{ "KEY" | env }}` and `{{ "KEY" | env | teamvaultUrl | base64 }}` template syntax MUST be preserved verbatim — `Makefile.k8s`'s `apply` target renders it. Do not switch to Helm, Kustomize, or plain YAML.
- The names `recurring-task-creator` (Service / StatefulSet / Secret / pod `app` label / Ingress / KafkaUser prefix), the host pattern `recurring-task-creator.<NAMESPACE>.example.com`, the `KafkaUser` in `strimzi` namespace, the `tls-example` TLS secret, the `traefik` ingress class, the `docker-example` image pull secret, and the `my-cluster` strimzi label MUST all match the spec and the skeleton — DNS, RBAC, Prometheus discovery, strimzi, and traefik all depend on these exact strings.
- DO NOT add any of: `ServiceAccount`, `Role`, `RoleBinding`, `ClusterRole`, `ClusterRoleBinding`, `ServiceMonitor`, `HorizontalPodAutoscaler`, `PodDisruptionBudget`, `NetworkPolicy`, `*-error-critical-alert.yaml` — the spec Non-goals list all of these as out of scope.
- DO NOT change the HTTP port (fixed at `9090`), the timezone (fixed at `Europe/Berlin`), the env var names (`LISTEN`, `KAFKA_BROKERS`, `STAGE`, `SENTRY_DSN`, `SENTRY_PROXY`, `TZ`), or the keeplived/keel.sh annotations.
- DO NOT add a new `Makefile.env` variable pair beyond what is already required for Kafka mTLS and Sentry — every new env var is a new secret to rotate.
- DO NOT add a `make validate` target to `k8s/Makefile`. The `k8s/Makefile` is frozen at 3 lines.
- DO NOT commit. The YOLO container is forbidden from committing. Dark-factory handles git operations.
- DO NOT touch any Go source file (no `main.go`, no `pkg/`, no `go.mod`, no `go.sum`, no `mocks/`).
- DO NOT modify `k8s/Makefile`, `Makefile.k8s`, `Makefile.variables`, or `Makefile.precommit`.
- DO NOT modify `k8s/recurring-task-creator-svc.yaml`, `k8s/recurring-task-creator-ing.yaml`, `k8s/recurring-task-creator-secret.yaml`, or `k8s/recurring-task-creator-user.yaml` — they are already correct.
- DO NOT remove the skeleton's `keel.sh/*` annotations — auto-redeploy on tag push is the cluster convention.
- DO NOT remove the skeleton's `imagePullSecrets: [{name: docker-example}]` — the quant registry requires it.
- The manifest is correct iff the spec's AC greps pass. `make precommit` is a defensive gate, not the load-bearing correctness signal.

## Failure-mode coverage (from the spec's `## Failure Modes` table)

These failure-mode rows are addressed by the prompt's requirements above; do not add new logic, but verify the file shape that handles each row:

- **KAFKA_BROKERS unset at apply time** — the new env block declares `KAFKA_BROKERS` from `'{{ "KAFKA_BROKERS" | env }}'` (requirement #1). The `example.env` update (requirement #9) sets a default, so a deploy loop that forgets to override still produces a valid template (it just publishes to `localhost:9092`, which is wrong but the templating does not fail).
- **Strimzi has not yet provisioned the KafkaUser cert** — unchanged from skeleton; the `KafkaUser` resource is applied first by strimzi's per-namespace reconciler, then the pod consumes the mounted cert.
- **tls-example secret rotation** — unchanged from skeleton; the Ingress already references `tls-example`.
- **Two pods running simultaneously during rolling update** — `replicas: 1` plus `RollingUpdate` ensures one-at-a-time rollout; idempotency is at the Spec 2 / Spec 3 layer.
- **Pod drain / node loss** — `RollingUpdate` is preserved; the new pod lands on another `node_type=agent` node.
- **Image pull failure** — unchanged from skeleton; `imagePullPolicy: Always` + `keel.sh/poll` is preserved.
- **Sentry DSN rotation staleness** — unchanged from skeleton; the Secret is re-rendered on every `make apply`.
- **kubeconform rejects a rendered manifest** — addressed by requirement #12's PR-description validation block.
- **Pod OOM-killed** — addressed by requirement #1's new `limits: { cpu: 200m, memory: 100Mi }` (matches maintainer watcher precedent).
- **Read-only root FS blocks a library write** — addressed by requirement #1's `emptyDir` at `/tmp` (writable scratch space for any library that needs it).
</constraints>

<verification>
From `/workspace/`:

1. `make precommit` — must exit 0.
2. `ls /workspace/k8s/ | sort` — must return exactly six lines: `Makefile`, `recurring-task-creator-ing.yaml`, `recurring-task-creator-secret.yaml`, `recurring-task-creator-sts.yaml`, `recurring-task-creator-svc.yaml`, `recurring-task-creator-user.yaml`.
3. `grep -rnE 'BATCH_SIZE|DATADIR|boltkv' /workspace/k8s/` — must return no matches.
4. `grep -lE '^kind: (ServiceAccount|Role|RoleBinding|ClusterRole|ClusterRoleBinding|ServiceMonitor|HorizontalPodAutoscaler|PodDisruptionBudget|NetworkPolicy)$' /workspace/k8s/*.yaml` — must return no matches.
5. The 28 `yq` and `grep` AC checks from requirement #11 — every one must match the spec's expected value. List the exit codes in the completion report.
6. The templating step in requirement #12 — `teamvault-config-parser ... < /workspace/k8s/recurring-task-creator-sts.yaml` must produce a rendered YAML on stdout with no stderr and exit code 0. If `teamvault-config-parser` is not installed, fall back to `kubectl apply --dry-run=client -f /workspace/k8s/recurring-task-creator-sts.yaml` and document the fallback in the PR description.
7. `cat /workspace/CHANGELOG.md` — must show the new `feat:` bullet under `## Unreleased` (above the `## v0.0.2` divider).
8. `cat /workspace/example.env` — must show 11 `export` lines, including the new `STAGE`, `SENTRY_DSN_KEY`, `SENTRY_PROXY_URL`, `LOGLEVEL`, `RANDOM` keys, with all five original keys preserved.

Report the exit code of every command above in the completion report's `## Verification` block.
</verification>

## Open Questions

These were resolved during prompt generation by picking the most reasonable default; flag in the PR description if a reviewer disagrees.

- **`RANDOM=$$(date +%s)` in `example.env`** — uses `$$` so make does not expand at parse time. Some skeletons use a fixed value and rely on keeplived/keel.sh to poll. The spec does not specify; the `$$(date +%s)` form is the convention used by the parent skeleton's other services.
- **`SENTRY_PROXY_URL` placeholder value** — `http://sentry-proxy.quant.svc.cluster.local:8080` is a placeholder. The spec does not name the cluster-internal Sentry proxy URL. The deploy worktree's per-namespace `example.env` is expected to override this.
- **`STAGE=dev` in `example.env`** — defaults to `dev` because the repo is shipped from a `dev` deploy context. The prod worktree overrides with `export STAGE=prod`.
- **Image pull secret name `docker-example`** — the spec AC says "or whichever name the parent module's `Makefile.env` uses for the quant registry." The skeleton uses `docker-example`; this prompt preserves it without further verification against `~/Documents/workspaces/maintainer/watcher/github-pr/k8s/` (that path is on the maintainer's host machine, not in the YOLO container).
