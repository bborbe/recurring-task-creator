---
status: approved
approved: "2026-11-14T12:41:31Z"
branch: dark-factory/role-mode-interactive-guard
---

## Summary

- A recurring-schedule template can declare `mode: interactive` while leaving `role` unset. An unset `role` resolves to `agent`, which the supervisor's role-mode matrix forces to `headless` — so the declaration is inert and silently contradicted by the runtime.
- Measured consequence, 2026-11-14: the `shutdown-k3s` schedule materialized as a headless worker and stopped K3s on all ten nuke nodes at 10:31 local, hours before its Saturday-evening window.
- The generator gains a publish-time guard: `mode: interactive` is accepted only when `role` is `human` or `manager`. Every other pairing — role absent, or `role: agent` — is refused before the task command is sent.
- The refusal is an error, not a warning: a cluster log line is invisible to the operator, and every template in the cluster today is already compliant, so refusing breaks nothing legitimate.
- Rendering for compliant templates is unchanged, byte for byte.

## Problem

A recurring-schedule template is a Kubernetes `Schedule` custom resource whose `spec.template.frontmatter` map is stamped through to the materialized vault task. Two of those keys — `mode` and `role` — are consumed downstream by the supervisor to decide whether the materialized task runs as an interactive session or as a headless worker. The generator copies the map verbatim and validates nothing about the pair, so a template can declare `mode: interactive` while omitting `role` entirely; the absent role defaults to `agent`, and the matrix forces `agent` to `headless`. The operator's stated intent and the runtime's actual behavior diverge with no error, no log line, and no test. The result is a scheduled task that runs unattended in a mode the template declared it was not — which is exactly how a K3s shutdown meant for Saturday evening fired on a Friday morning across ten nodes.

## Reproduction

The pair reaches the wire through the publish path, which is deterministic and runnable against the current HEAD with no cluster access:

```go
def := schedule.TaskDefinition{
    Slug:          "shutdown-k3s",
    TitleTemplate: "Shutdown K3s",
    Recurrence:    schedule.RecurrenceWeekly,
    Frontmatter:   lib.TaskFrontmatter{"mode": "interactive"},
}
```

Publish it through a publisher built with the real renderer, the real frontmatter formatter, the real identifier creator, and a capture sender (the pattern already used in the `frontmatter` block of `pkg/publisher/publisher_test.go`). The runnable form is a Ginkgo focus in the existing suite:

```bash
go test -count=1 ./pkg/publisher/ -ginkgo.focus='mode/role pair'
```

Before the fix that focus reports the sender was called once; after the fix it reports a nil sender count and a non-nil error. Observed on the current HEAD:

- `Publish` returns a nil error.
- The captured command's frontmatter has `mode: interactive`.
- The captured command's frontmatter has no `role` key.
- The capture sender was called exactly once — the command was sent.

Downstream consequence, measured 2026-11-14: the `shutdown-k3s` task opened as a headless worker and stopped K3s on all ten nuke nodes at 10:31 local time, outside its Saturday-evening window.

## Expected vs Actual

**Expected:** a template whose frontmatter pairs `mode: interactive` with a role that the supervisor's matrix forces to `headless` — role absent, or `role: agent` — does not produce a task command. The publish fails with an error naming the offending schedule, so the operator sees the contradiction instead of the runtime silently resolving it.

**Actual:** the publish succeeds, the command is sent, the task materializes with `mode: interactive` and no `role`, and the supervisor runs it headless.

## Why this is a bug

`mode` is a supervisor-consumed key whose meaning is defined relative to `role`. The pair `mode: interactive` + `role: agent` (or role absent, which resolves to `agent`) is self-contradictory: the matrix resolves it to `headless`, so the declared mode has no effect. Nothing in the generator, the CRD schema, or the publish path rejects the contradiction, and the only observable is the wrong thing happening at fire time. A configuration flag that is silently ignored is a defect, and this one has a physical blast radius.

## Goal

After this work:

- The publish path refuses any template whose frontmatter pairs `mode: interactive` with a role other than `human` or `manager`.
- The refusal is a returned error that names the offending schedule slug and the pair it found, so an operator can fix the template without reading it.
- Every pairing that is compliant today — including `role: human` + `mode: interactive`, and every template that sets no `mode` at all — publishes exactly the same command as before.
- A test that exercises the full pairing matrix fails if the guard is removed, so the pair cannot be reintroduced silently.
- The materialized frontmatter for a compliant template is unchanged in every key: defaults, phase seed, computed `defer_date`, `auto_abort_prior`, `created_by`, placeholder rendering in scalar and list values, and determinism across calls.

## Non-goals

- Do NOT reject the pair at CRD admission (no new CEL rule, no schema change). The guard is publish-time only: a non-compliant template still applies to the cluster and fails when it fires.
- Do NOT change the supervisor or its role-mode matrix, and do NOT change the default that an absent `role` resolves to `agent`.
- Do NOT edit any `bborbe/nuke` schedule template — that remediation is already merged and applied.
- Do NOT add a warn-only mode, a `--strict` flag, an allowlist of exempt schedules, or any config knob that disables the guard. The guard is the point of the change; an escape hatch is a regression. If a future consumer needs a variation, that is a separate spec.
- Do NOT validate any frontmatter key other than the `mode`/`role` pair.
- Do NOT make the guard placeholder-aware: `role` and `mode` are compared as literal supplied strings, never against their rendered form.

## Acceptance Criteria

- [ ] **The pairing matrix is refused and accepted as specified.** A Ginkgo table asserts a non-nil error for (`mode: interactive`, role absent) and (`mode: interactive`, `role: agent`), and a nil error for (`mode: interactive`, `role: human`), (`mode: interactive`, `role: manager`), (no `mode`, `role: agent`), and (`mode: headless`, `role: agent`). The table also asserts a non-nil error for (`mode: interactive`, `role: "{{iso-week}}"`) — a placeholder is compared as the literal supplied string, never as its rendered form, which is what proves the guard is not placeholder-aware. Evidence: `go test -count=1 ./pkg/publisher/` exits 0; and with the guard temporarily stubbed to always return nil (the `HasValidationFunc` body replaced with `return nil`, reverted with `git checkout -- pkg/publisher/<file>.go`), the same suite exits non-zero and names the refusal entries as failures.
- [ ] **The refusal error names the slug it was given, and the pair.** For a `shutdown-k3s` fixture the message contains `shutdown-k3s`, `interactive`, and — for the absent-role case — the literal `absent`; for the `role: agent` case it contains `agent`. A second fixture with a different slug (for example `weekly-review`) produces a message naming that slug and not `shutdown-k3s`, proving the slug is taken from the input rather than hardcoded. Evidence: Gomega `MatchError(ContainSubstring(...))` assertions on both fixtures passing, including the negative assertion that the second message does not contain `shutdown-k3s`.
- [ ] **Unexpected value shapes neither panic nor impose a constraint.** A nil frontmatter map, maps whose `mode` is a bool, int, or list, and maps whose `role` is an int, a bool, or a `[]interface{}` all validate without error and without panic. Evidence: a Ginkgo table covering those inputs passes; `go test -count=1 ./pkg/publisher/` exits 0.
- [ ] **`Format` itself refuses, not merely its caller.** Calling `Format` directly with a violating operator map returns a nil frontmatter map and a non-nil error. Evidence: `Expect(fm).To(BeNil())` and `Expect(err).To(HaveOccurred())` pass in the formatter suite.
- [ ] **The publish path refuses before sending, and that is the reproduction replay.** Publishing the `shutdown-k3s` definition from Reproduction through the real publish path returns a non-nil error whose message contains the slug, and the capture sender's call count is 0. This is the replay of the reproduction: the same definition that today produces a sent command must produce none. Evidence: `SendCommandCallCount()` equals 0 and `HaveOccurred()` passes in the publish-path test.
- [ ] **Compliant rendering is unchanged.** Every existing assertion in the frontmatter and publisher suites passes without any expected value being edited. Evidence: `git diff origin/master -- pkg/publisher/frontmatter_test.go pkg/publisher/publisher_test.go` shows no changed `HaveKeyWithValue` / `HaveLen` / `Equal` argument — only the added error handling at call sites; the determinism spec still passes.
- [ ] **The seam change and the validation-library wiring are visible in the source.** `Format` returns a frontmatter map and an error; the guard is composed from `validation.All`. Evidence: `grep -n 'validation.All' pkg/publisher/*.go` returns ≥1 line; `go doc ./pkg/publisher FrontmatterFormatter.Format` prints a signature ending in a two-value result whose second value is `error`; `go generate -mod=mod ./...` then `git diff --exit-code mocks/` is empty (the counterfeiter fake matches the new signature).
- [ ] **`make precommit` exits 0 and `CHANGELOG.md` records the guard.** Evidence: exit code 0; `grep -nE 'mode.*interactive|role.*human' CHANGELOG.md` returns ≥1 line under an `## Unreleased` heading (create the heading if absent — the file currently ends at a released section).

Scenario coverage: **no new scenario.** The behavior is reachable and fully exercised at the publish seam with the real renderer, the real formatter and the real identifier creator — only the command sender is a capture fake, which is the existing pattern in this package. No cluster, container, or multi-service interaction is needed to observe the refusal.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — format, generate, test, lint, vet, vulnerability scan, license headers; exits 0.
- `go test -count=1 ./pkg/publisher/` — the publisher suite passes.
- `grep -c 'shutdown-k3s' pkg/publisher/*_test.go` — prints a count ≥1, proving the fixture the refusal criteria are written against exists in the suite.
- `go doc ./pkg/publisher FrontmatterFormatter.Format` — the printed `Format` declaration ends in `(lib.TaskFrontmatter, error)`; on today's HEAD it ends in `) lib.TaskFrontmatter`, so a single-value result means the seam change did not land.
- `grep -n 'validation.All' pkg/publisher/*.go` — returns ≥1 line.
- `git diff --exit-code mocks/` after `go generate ./...` — empty.

### Operator-executable

None. The guard is proven at the publish seam, which is the only path a `Schedule` reaches a vault task through (this service performs no direct vault writes). Adding a cluster probe would require injecting a deliberately broken `Schedule` into dev, which this spec does not ask for.

## Desired Behavior

1. The rule is evaluated against the operator-supplied map and refuses exactly the pair `mode: interactive` with a role outside {`human`, `manager`}. Compliant pairs, and every other frontmatter key, are unaffected. The rule is expressed through `github.com/bborbe/validation` (see Constraints) rather than a hand-rolled `if` chain; the concrete type name and its internal composition are agent decides at impl time.
2. The validation reads the operator-supplied frontmatter exactly as the template declared it — before defaults are seeded, before merging, before placeholder rendering — so the check reports the template's own words rather than a rendered or defaulted derivative.
3. A compliant pair produces a nil error. A non-compliant pair produces an error whose message names the schedule slug verbatim, contains the observed `mode` value, and states the `role` value it found; when `role` is missing it says the key is `absent` rather than printing an empty string. Wording beyond those required tokens is agent decides at impl time.
4. `Format` returns the rendered frontmatter and an error. On a violation it returns a nil map and the error; on compliance it returns exactly the map today's implementation returns — same keys, same values, same types, same list shapes, same computed `defer_date`, same forced `created_by` last.
5. `Publish` propagates the formatter error, wraps it with the schedule slug and the fire date, and returns before the command is sent — the sender is never called for a refused template. Whether the formatter call is hoisted above the `task.CreateCommand` literal or left inline is agent decides at impl time.
6. Value shapes that are not the literal string pair impose no constraint and never panic: a nil frontmatter map, a non-string `mode`, and a non-string `role` all validate successfully and pass through the existing rendering rules unchanged.

## Constraints

- **Frozen seam — the formatter interface and its implementation.** In `pkg/publisher/frontmatter.go` the method becomes `Format(operator lib.TaskFrontmatter, slug string, date schedule.Date, autoAbortPrior bool, recurrence schedule.RecurrenceKind) (lib.TaskFrontmatter, error)` — same parameter list, same order, one added error result. The interface declaration and the implementation must match; no other method signature in the package changes.
- **Frozen call site.** The single production caller is `Publish` in `pkg/publisher/publisher.go` (the `Frontmatter:` field of the `task.CreateCommand` literal). No other production code calls `Format`.
- **Frozen mock surface.** `mocks/publisher-frontmatter-formatter.go` is generated from the `//counterfeiter:generate` directive above the interface; it is regenerated, never hand-edited. Any test that stubs the formatter must adopt the two-value return.
- The guard must be expressed through `github.com/bborbe/validation` (`validation.All{...}`, `validation.Name(...)`, `validation.HasValidationFunc(...)`) — no hand-rolled `if` chain in `Format`. The module already vendors `validation` v1.5.9 as an indirect dependency; importing it directly makes it a direct requirement.
- Tests stay in the existing Ginkgo v2 / Gomega suite in the publisher package (`publisher_suite_test.go`, 60-second suite timeout). No new suite file, no `testing.T` outside the suite entry point, no stdlib table tests, no mocks for the pure rule — the publish-path test uses the real formatter with a capture sender.
- Project Definition of Done applies (`docs/dod.md`): `bborbe/errors` for wrapping, GoDoc on every exported type and method, no `time.Now()` in business logic, no `context.Background()` in business logic.
- `docs/architecture.md` describes the publisher's frontmatter behavior; the description of what the formatter does must stay accurate after the change (one added sentence on the guard is sufficient).
- The change must not alter the emitted command for any compliant template — same identifier, same title, same body, same frontmatter map.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility |
|---------|-------------------|-----------|----------|---------------|
| Template declares `mode: interactive` with no `role` | Publish refused; error names the slug and reports role `absent`; no command sent, so no task materializes for that fire | The publisher's error log line for the tick; no vault task for the period | Add `role: human` (or `manager`) to the template, or delete the `mode:` line; the next tick publishes normally | Reversible — but the current period's task is missed and must be created by hand |
| Template declares `mode: interactive` with `role: agent` | Same refusal, error reports `agent` | Same as above | Same as above | Reversible, same missed-period caveat |
| A non-compliant template is already in the cluster (applied before this guard) | Accepted at admission; refused at its first fire after the deploy | First fire produces no task and an error naming the slug | Fix the template; re-create the missed task manually | Partial — the miss is a real, observable gap |
| Template frontmatter contains an unexpected value shape (bool `mode`, numeric `role`, nil map) | No constraint applied, no panic, rendering proceeds under the existing rules | None needed — behavior is the pre-change behavior | None | N/A |
| Two schedules fire in the same tick and one is refused | The refused one returns an error; the other publishes normally — the refusal must not abort the tick's other work | Per-schedule error line; the compliant schedule's task still appears | Fix the refused template only | Reversible |
| Guard regresses (someone removes it or widens the accepted roles) | The pairing-matrix test fails at `make precommit` | Non-zero `go test ./pkg/publisher/` with the refusal entries named | Restore the rule | Reversible — defensive lock |
| Guard rejects a pair that is actually compliant (over-reach) | Every publish of that template fails at fire time | Error line naming a template the operator believes compliant | Widen the accepted role set in a follow-up change; the spec's rule is `human` or `manager` and must not be widened silently | Reversible |
| Schema drift: the supervisor renames `mode` or `role`, or changes the absent-role default | The guard silently stops matching (no refusal fires) or refuses everything | The pairing-matrix test pins the two key names and the two accepted role values, so a rename in this repo is a visible edit | Separate spec covering the supervisor contract | Reversible |

## Security / Abuse Cases

- **What an attacker controls:** the `Schedule` CR frontmatter — cluster-admin-authored input, not HTTP or user input. There is no untrusted network path into this change.
- **Trust boundary:** the CRD admits the frontmatter map as free-form (`XPreserveUnknownFields`); the publisher is the first component to interpret any of it. The guard is the boundary that stops a declared-but-inert intent from reaching a worker.
- **Robustness is the abuse surface:** a panic in the formatter would take down the publish loop and skip every other schedule's tick in that pass. The guard must therefore never assume a key exists, never type-assert without the comma-ok form, and never dereference a nil map — covered by an acceptance criterion.
- **Data in errors:** the error names only the slug, the `mode` value, and the `role` value. It must not dump the whole frontmatter map — that map can carry unrelated operator content (assignees, goal links, notes) into logs.
- **No hang, no retry loop:** the guard is a pure in-memory check with no I/O, no clock, and no retry. It cannot block or loop.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | The rule: a `Validate(ctx) error` value type in the publisher package encoding `mode: interactive` ⇒ `role` ∈ {`human`, `manager`}, composed from `validation.All` / `validation.Name` / `validation.HasValidationFunc`, with the required error tokens (slug, mode value, role value, `absent`). Its Ginkgo table covers the full matrix plus the non-string and nil-map shapes. Import `github.com/bborbe/validation` directly so `go.mod` drops the indirect marker. No wiring yet. | 1, 2 | 1, 2, 3 | — |
| 2 | The seam: change `Format` to return `(lib.TaskFrontmatter, error)`, validate the operator map before seeding/merging/rendering, return nil + error on violation, and return today's map unchanged on compliance. Propagate the error in `Publish` (wrapped with slug and date) before sending the command. Regenerate the counterfeiter fake, adapt existing call sites to the two-value return without touching any expected value, add the publish-path test, and land the CHANGELOG entry and the `docs/architecture.md` sentence. | 3, 4, 5, 6 | 4, 5, 6, 7, 8 | prompt 1 |

Rationale: prompt 1 lands a self-contained, fully tested rule with no call-site churn, so its matrix can be reviewed in isolation. Prompt 2 then changes the interface — the only step that forces every call site and the generated fake to move at once — and proves the refusal at the publish seam. Doing it in the other order would leave `Format` returning an error with nothing to report.

## Assumptions

- Every `Schedule` template in the cluster is already compliant as of 2026-11-14: three carry `role: human` and the remaining thirty-six had the inert `mode:` line removed. Refusing the non-compliant pair therefore breaks no legitimate template on the day it ships.
- `mode` and `role` are the only two frontmatter keys involved in the interactive-versus-headless decision, and an absent `role` resolves to `agent` (which the matrix forces to `headless`). The rule's authoritative home is `bborbe/claude-supervisor` `docs/role-mode-matrix.md` § Rule 2 — `human` and `manager` are forced `interactive`, `agent` is forced `headless` "whatever its body carries and whatever its `mode:` field says on disk" (operator ruling 2026-11-12) — and `docs/fleet-surface.md` § Spawn a worker item 6 records that a stale `mode: interactive` on a `role: agent` row is rewritten to `headless`. This spec encodes that rule; it does not re-derive the matrix.
- The publish path is the only route by which a `Schedule` becomes a vault task — this service performs no direct vault writes and sends every task through the injected command sender, so a refusal at that seam is a complete refusal.
- The frontmatter map reaches the publisher as `map[string]interface{}` with CR-decoded value shapes (strings, bools, ints, `[]interface{}`), which is what the existing rendering rules already handle.

## Do-Nothing Option

Do nothing and the generator keeps copying `mode: interactive` through to the task with no role, and the supervisor keeps resolving that to `headless`. The templates that caused the 2026-11-14 incident are fixed today, but nothing prevents the pair from being reintroduced on the next new schedule — the same class of silent, physically-consequential misconfiguration, with the same absence of any signal until the wrong thing runs. The fix is small, every current template is already compliant, and the cost of the next occurrence is a cluster-wide outage on a schedule nobody intended.
