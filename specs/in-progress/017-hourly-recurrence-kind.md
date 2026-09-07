---
status: prompted
approved: "2026-09-07T20:57:01Z"
generating: "2026-09-07T21:00:33Z"
prompted: "2026-09-07T21:20:15Z"
branch: dark-factory/hourly-recurrence-kind
---

## Summary

- Add `hourly` to the closed recurrence set, so a Schedule definition fires one distinct task file per civil hour instead of once per day.
- The tick loop already runs hourly; it only computes the Europe/Berlin civil date (no hour), and the period token is day-granular (`YYYY-MM-DD`), so an hourly schedule would collapse to one task file per day without a new hour-granular token.
- Thread the Berlin civil hour through the pipeline: the schedule date carries an hour, the hourly tick populates it, and the publisher's period token for `hourly` becomes hour-granular (`YYYYMMDDHH`), giving each civil hour a distinct UUID5 identifier and thus a distinct task file.
- The Schedule CRD accepts `recurrence: "Hourly"`; `periodOffset` stays forbidden for Hourly (the existing CEL rule already rejects it — only comments change).
- No new CRD fields, no sub-hourly cadence, no changes to any existing kind; verify dev-first, then prod, on the deployed StatefulSets.

## Problem

`recurring-task-creator`'s smallest cadence is daily (daily/weekly/weekday/monthly/quarterly/yearly/ondate). The Check Failed Builds Watcher in the vuln-fix pipeline needs an hourly build-failure check: it must materialize a fresh task file every hour a build is failing. Today a full day of failures collapses into a single daily task, so the per-hour failure signal the watcher needs is lost. The infrastructure is already in place — the tick loop already fires every hour — but it only computes the civil date and the publisher's dedup token is day-granular, so an `hourly` kind would silently produce one file per day, not one per hour, unless the hour is carried end-to-end.

## Goal

After this work:

- `hourly` is a member of the closed `RecurrenceKind` set and of `AllRecurrenceKinds`, so the store adapter accepts it at parse time and metrics label pre-initialization picks it up.
- An `hourly` entry fires on every tick (always-fire semantics, like daily), and the publisher materializes a distinct task file per civil hour of the Europe/Berlin clock.
- The civil hour flows through the pipeline: the schedule date carries it, the hourly tick populates it, and the publisher's period token for `hourly` is hour-granular so each fire gets a distinct UUID5 identifier.
- The Schedule CRD accepts `recurrence: "Hourly"`; `periodOffset` remains rejected for Hourly.
- Every existing recurrence kind keeps its exact firing behavior and period-token format.

## Non-goals

- Do NOT add sub-hourly recurrence (no minute/second granularity).
- Do NOT change the tick-loop cadence — it is already hourly and stays hourly.
- Do NOT add `periodOffset` support for `hourly` — the CEL rule keeps `periodOffset` valid only for Monthly/Quarterly/Yearly.
- Do NOT change the firing behavior or period-token format of any existing kind.
- Do NOT add new CRD fields — no hour-of-day selector, no timezone field. An `hourly` entry fires every civil hour in Europe/Berlin, like every other kind.
- Do NOT change the `/trigger?date=` handler. A manual trigger of an `hourly` entry uses hour 0 (documented limitation, see Failure Modes).
- Do NOT touch the Check Failed Builds Watcher / analysis agents themselves — those are separate tasks.
- Do NOT add an hour-of-day selection list or a per-feature opt-out flag — invariant; if a future consumer demands per-hour selection, that is a separate spec.

## Acceptance Criteria

- [ ] `grep -nE 'RecurrenceHourly\s+RecurrenceKind = "hourly"' pkg/schedule/recurrence.go` returns ≥1 — evidence: matched const declaration line.
- [ ] `RecurrenceHourly` appears in the `AllRecurrenceKinds` slice — evidence: `grep -nA12 'AllRecurrenceKinds = ' pkg/schedule/recurrence.go | grep -c 'RecurrenceHourly'` returns ≥1.
- [ ] A Ginkgo spec in `pkg/schedule` proves an `hourly` entry is included by `TasksForDate` on an arbitrary civil date (always-fire), and the existing unknown-kind skip-with-warning still holds — evidence: passing spec names printed by `go test -v ./pkg/schedule/`.
- [ ] `schedule.Date` carries an `Hour int` field (0-23) whose GoDoc states the zero value for date-only construction sites and that it is consulted only by the hourly period token — evidence: `grep -nE 'Hour\s+int' pkg/schedule/date.go` returns ≥1 and the doc comment names the hourly-only rule.
- [ ] A Ginkgo spec in `pkg/tick` proves the hour is threaded: with the injected clock fixed at an instant whose Europe/Berlin civil hour is H (e.g. `2025-01-04T13:30:00Z` → Berlin `2025-01-04 14:30`, Hour=14), the `schedule.Date` captured by the fake publisher carries `Hour=H` — evidence: passing spec name printed by `go test -v ./pkg/tick/`.
- [ ] Ginkgo specs in `pkg/publisher` prove the period token for `(RecurrenceHourly, Date{2026, September, 7, Hour=13})` equals `"2026090713"`, and that hour 12 vs hour 13 on the same day yield different tokens (thus different UUID5 identifiers and different task files) — evidence: passing spec names printed by `go test -v ./pkg/publisher/`.
- [ ] The Go-built CRD schema declares `"Hourly"` in the recurrence enum and the CEL rule still rejects non-zero `periodOffset` on Hourly — evidence: (a) `grep -nE '"Hourly"' pkg/k8s_connector_schema.go` returns ≥1; (b) a Ginkgo spec in the k8s_connector validation suite accepts `recurrence: "Hourly"` and rejects `recurrence: "Hourly"` with `periodOffset: 1` — passing spec names printed by `go test -v ./pkg/`.
- [ ] **Negative:** no existing kind's period-token code path changed — evidence: `git diff pkg/publisher/period_token.go | grep -c '^-[^-]'` returns 0 (no removed lines), and every pre-existing publisher token spec still passes (`go test -v ./pkg/publisher/` prints the existing daily/weekly/weekday/monthly/quarterly/yearly/ondate token specs green).
- [ ] `make precommit` exits 0 from the repo root — evidence: exit code 0.
- [ ] **Post-Deploy (Rung-2):** an Hourly Schedule CR fires a distinct task file per civil hour in dev — evidence: apply a throwaway `recurrence: Hourly` CR in `erpnext` on dev; the CRD accepts it (no admission error); `kubectldev -n erpnext logs <recurring-task-creator-pod> --since=2h | grep 'sent CreateCommand'` shows ≥2 distinct hour-granular tokens for the slug across two consecutive hourly ticks; and ≥2 distinct materialized task files (distinct hour tokens in the title) exist in the CR's target vault. The two fires span up to ~60 minutes (the ticker is hourly; a StatefulSet restart triggers an immediate initial tick).
  - `deploy_check:` `kubectldev -n erpnext get statefulset/recurring-task-creator -o jsonpath='{.spec.template.spec.containers[0].image}' | awk -F: '{print $NF}'`
  - `deploy_target:` `$(git rev-parse --short HEAD)`
- [ ] **Post-Deploy (Rung-3):** the same throwaway Hourly CR check passes on prod — evidence: apply the same `recurrence: Hourly` CR in `erpnext` on prod; `kubectlprod -n erpnext logs <recurring-task-creator-pod> --since=2h | grep 'sent CreateCommand'` shows ≥2 distinct hour-granular tokens for the slug; ≥2 distinct materialized task files in the CR's target vault.
  - `deploy_check:` `kubectlprod -n erpnext get statefulset/recurring-task-creator -o jsonpath='{.spec.template.spec.containers[0].image}' | awk -F: '{print $NF}'`
  - `deploy_target:` `v$(git describe --tags --abbrev=0)` — the semver released for this change (e.g. `v0.12.0`), matching the prod image tag suffix

**Scenario coverage: NO new scenario.** The behavior is reachable by unit + integration tests (injected-clock tick test, period-token and validation specs) and the operator-executable rungs observe the real deployed system; no E2E scenario is warranted.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
go test -v ./pkg/schedule/ ./pkg/tick/ ./pkg/publisher/ ./pkg/
grep -nE 'RecurrenceHourly' pkg/schedule/recurrence.go
grep -nE 'Hour\s+int' pkg/schedule/date.go
grep -nE '"Hourly"' pkg/k8s_connector_schema.go
git diff pkg/publisher/period_token.go | grep -c '^-[^-]'
```

Expected: `make precommit` exits 0; `go test -v` prints the new hourly always-fire, tick hour-threading, hour-granular token, token-distinctness, and CEL accept/reject specs green alongside all pre-existing specs; the three greps return ≥1 line each; the removed-line count is `0`.

### Operator-executable (runs on the host, spec-verification ladder — Rung 2/3 after deploy)

- Rung-2 (dev): apply a throwaway Hourly CR (`spec.schedule.recurrence: "Hourly"`, unique slug) in `erpnext` on dev; confirm admission accepts it; confirm `kubectldev -n erpnext logs` shows a `sent CreateCommand` line per hourly tick for the slug; confirm ≥2 distinct materialized task files (distinct hour tokens) after two consecutive hourly ticks; delete the throwaway CR and its task files.
- Rung-3 (prod): repeat the same CR + log + vault-file check on prod after the release; clean up.

## Desired Behavior

1. `pkg/schedule/recurrence.go` declares `RecurrenceHourly RecurrenceKind = "hourly"` and appends it to `AllRecurrenceKinds` (declaration order preserved; the slice stays the single closed validity source). Consumers that derive from `AllRecurrenceKinds` — the store adapter's parse-time kind check and the metrics label pre-initialization — accept and handle the new kind with no further code.
2. `filterInventoryByDate` (`pkg/schedule/tasks_for_date.go`) adds `RecurrenceHourly` to the explicit always-fire case group, so an `hourly` entry fires on every tick. The `default:` branch stays skip-with-warning — an unrecognized kind still never fires.
3. `schedule.Date` (`pkg/schedule/date.go`) gains an `Hour int` field (0-23). GoDoc states: zero for every date-only construction site (the `/trigger` handler, existing tests, callers that know only a civil date); consulted only by the hourly period token. The field is additive — every existing kind ignores it, and its presence never changes an existing (date-only) behavior.
4. The hourly tick (`pkg/tick/tick.go`) computes the Europe/Berlin civil hour from the same Berlin-adjusted clock read it already takes and populates `Date.Hour` with it, alongside the existing year/month/day. The tick cadence and the one-pass-per-tick publishing loop are unchanged.
5. The publisher's period-token builder (`pkg/publisher/period_token.go`) adds a `RecurrenceHourly` case returning the compact hour-granular token `YYYYMMDDHH` (e.g. `2026090713`) built from the civil date plus the civil hour. Each civil hour therefore produces a distinct period token, a distinct UUID5 identifier (`recurring-<slug>-<YYYYMMDDHH>`), and a distinct task file. `PeriodOffset` is not applied to Hourly.
6. The Go-built CRD JSONSchema (`pkg/k8s_connector_schema.go`) adds `"Hourly"` to the `recurrence` enum and to the field description's kind list; the `periodOffset` CEL rule string is unchanged (Hourly is not in its allowed list, so non-zero `periodOffset` is rejected), while the rule's comment is clarified to name Hourly as periodOffset-forbidden. The GoDoc enumerating recurrence kinds on the k8s types (`k8s/apis/.../types.go`) mentions Hourly.

## Constraints

- `RecurrenceKind` remains a closed enum; `AllRecurrenceKinds` stays declaration-ordered and is the single validity source — no inline kind switch that bypasses it (`go-enum-type-pattern.md`).
- Existing kinds' firing behavior and period-token formats (`YYYY-MM-DD`, `YYYYWww`, `YYYYWww-<wd>`, `YYYY-MM`, `YYYYQq`, `YYYY`) MUST NOT change — only additive. The UUID5 namespace and existing slugs are frozen.
- The tick loop cadence (one fire per elapsed hour, one pass per fire) is unchanged; the service stays a single-replica StatefulSet per stage (dev, prod) in ns `erpnext` — no new replica, no new deployment surface.
- `schedule.Date` stays the only input shape to `TasksForDate` and the publisher; the new `Hour` field is additive, and existing construction sites keep producing `Hour=0`.
- The `/trigger?date=` handler is unchanged; a manual trigger of an `hourly` entry materializes the hour-00 file of that day (documented limitation, not an error).
- Project DoD applies (`docs/dod.md`): Ginkgo v2 / Gomega, `bborbe/errors` 3-arg `Wrap`, no `context.Background()` / `time.Now()` in business logic, GoDoc on exports, `make precommit` clean. Coding guides apply: `go-enum-type-pattern.md`, `go-testing-guide.md`, `go-time-injection.md`, `go-glog-guide.md`.
- Documentation that enumerates recurrence kinds or token formats (`docs/architecture.md`) stays accurate for the new kind and token.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Hour dropped from the hourly token (regression to day-granular `YYYYMMDD`) | All hourly fires in a day share one identifier → collapse to one task file per day — exactly the bug this feature exists to fix. Detection: the CR's target vault shows one file per day for the slug instead of ~24; Reversibility: reversible. | Fix the token and redeploy; the token-distinctness spec (AC 6) is the regression lock — it fails on any such revert. |
| Hourly match accidentally dropped from the always-fire case group | The `hourly` entry stops firing; no error is logged (silent). Detection: pod logs lack `sent CreateCommand` for the slug; vault files stop appearing. | Fix or roll back the image; the always-fire spec (AC 3) is the regression lock. |
| `default:` always-fire regression (unknown kinds fire again) | An unrecognized kind publishes every tick — a flood of task files. Detection: `grep WARN` in pod logs is absent where it was expected, and unexpected task files appear; Concurrency: single replica bounds the storm to one pass per hour. | Roll back the image; the unknown-kind skip spec (AC 3) is the regression lock. |
| DST fall-back (25-hour day): two ticks land in civil hour 02 | Both produce token `YYYYMMDD02`; UUID5 dedup collapses them to one task file — no duplicate. | None needed; documented behavior. |
| DST spring-forward (23-hour day): civil hour 02 never exists | No hour-02 file that day (23 files instead of 24). | None needed; documented behavior. |
| Ticker misalignment (elapsed-hour ticker fires at e.g. :37, not on the civil hour boundary) | The token uses the civil hour at fire time; a fire straddling a boundary yields that boundary's hour token — at most one extra or one missing file per boundary, dedup absorbs repeats. | None needed; documented behavior. |
| Version skew during rollout (new CRD + old binary, or binary-first) | Old binary's adapter rejects `recurrence: "hourly"` via `AllRecurrenceKinds` (entry not stored, error logged); old CRD rejects `recurrence: "Hourly"` at admission (enum). Either way the hourly entry is non-functional and existing kinds are unaffected — no crash, no partial fire. Detection: admission error or adapter error log line. | Complete the rollout (deploy both new artifacts together); no data repair needed. |
| Kafka broker / schedule store unavailable during a tick | Store-list error is logged (`tick.go`), the tick is skipped, no task file materializes for that hour; the next hourly tick retries, and the hour-granular UUID5 dedup absorbs the late fire. Detection: store-list error log line. | None — automatic retry on the next hourly tick; verify via the existing error log line. |
| Crash mid-publish-pass | A partial set of the hour's task files materializes; the next tick's re-publish re-sends the whole due set and the hour-granular UUID5 identifier dedups to the missing files only — no duplicates. Detection: partial vault files for the slug in that hour. | None — dedup absorbs; re-publish on the next tick. |
| Manual `/trigger` of an hourly entry | Handler builds a date with Hour=0 → token `YYYYMMDD00` → one hour-00 task file that day (dedup collapses it with a real midnight tick if one fired). Single file, no flood. | None needed; documented limitation (Non-goal). |
| Operator authors many hourly CRs (volume misconfiguration) | ~24 task files per CR per day — intended per-CR behavior, but multiplied across CRs it floods the vault. Detection: task-file count in the target vault. | Remove or re-cadence the CRs (admission-time data fix, no code change). |

## Security / Abuse Cases

- **Attacker control:** the only external surface is the existing `/trigger?date=` handler (cluster-internal, unauthenticated, unchanged) and CRD admission. `recurrence` is enum-validated, so `Hourly` is the only new accepted value; no new user-controlled input parsing is added.
- **Publish-volume widening:** an `hourly` CR multiplies per-CR file volume by ~24 vs daily. Any actor who can already create Schedule CRs could flood the vault with up to 24 files/day per CR — this is inherited exposure (daily CRs already produce 1/day each) scaled by a constant, not a new trust boundary. Dedup still bounds output to one file per `(slug, hour)`.
- **Replay safety:** the period-anchored UUID5 identifier makes replays idempotent at hour granularity — a pod restart or trigger replay within the same civil hour produces the same identifier and no duplicate file.
- **Nothing can hang or retry forever:** the tick loop is unchanged (single pass, bounded work per tick); no new loops, timers, or network calls are added.

## Suggested Decomposition

Prompts should be generated in this order; each row is one prompt.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Schedule core: `RecurrenceHourly` const + `AllRecurrenceKinds` entry, `Hour` field on `schedule.Date` (GoDoc: zero for date-only construction, hourly-only), always-fire case in `filterInventoryByDate` (default stays skip-with-warning), with Ginkgo specs for hourly always-fire + unknown-kind skip + Date.Hour doc. | 1, 2, 3 | 1, 2, 3, 4 | — |
| 2 | Tick hour threading: populate `Date.Hour` from the Berlin civil hour in the hourly tick, with the injected-clock Ginkgo spec (e.g. `2025-01-04T13:30:00Z` → Hour=14). | 4 | 5 | prompt 1 (uses `Date.Hour`) |
| 3 | Publisher token: `RecurrenceHourly → YYYYMMDDHH` in the period-token builder, with the format + distinctness Ginkgo specs (hour 12 vs 13 differ). | 5 | 6, 8 | prompt 1 (uses the kind) |
| 4 | CRD + docs: `"Hourly"` in the recurrence enum + description, `periodOffset` CEL comment clarification (rule unchanged), kinds GoDoc on the k8s types, with the validation Ginkgo specs (accept Hourly, reject non-zero periodOffset). | 6 | 7, 8 | prompt 1 (uses the kind) |

Rationale: prompt 1 establishes the kind and the `Date.Hour` field that every other layer consumes; prompts 2, 3, and 4 are independent of each other after prompt 1. AC 9 (`make precommit`) is the global gate every prompt keeps green; ACs 10-11 are operator rungs that run only after merge + deploy, outside the prompt sequence.

## Do-Nothing Option

If we don't do this, the smallest cadence stays daily: the Check Failed Builds Watcher cannot materialize a task per hour, and a day of build failures collapses into a single daily task — the vuln-fix pipeline keeps losing the per-hour failure signal it needs. The only alternative is a separate hourly cron outside `recurring-task-creator`, which splits the scheduling mechanism and loses the UUID5 dedup + vault-task pipeline integration. The change itself is small (one enum value, one Date field, one token case, one CRD enum entry) on infrastructure that already ticks hourly.
