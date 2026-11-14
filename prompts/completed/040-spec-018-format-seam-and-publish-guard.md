---
status: completed
spec: [018-role-mode-interactive-guard]
summary: Changed FrontmatterFormatter.Format to return (map, error), enforced the mode/role rule at the formatter seam and propagated refusal from Publish before sending, regenerated the mock, added formatter and publish-path specs, and recorded the guard in CHANGELOG and architecture docs (also landing the prerequisite ModeRoleRule).
execution_id: 68b0ced1-4024-5e3e-94db-39df70648379-exec-040-spec-018-format-seam-and-publish-guard
dark-factory-version: dev
created: "2026-11-14T12:50:00Z"
queued: "2026-11-14T13:04:27Z"
started: "2026-11-14T13:04:28Z"
completed: "2026-11-14T13:14:57Z"
branch: dark-factory/role-mode-interactive-guard
---

# Wire the mode/role guard into the formatter seam and publish path

<summary>
- Publishing now refuses a task whose frontmatter declares interactive mode without a human or manager role.
- The refusal happens before anything is sent, so no wrong-mode task materializes and no K3s-style incident can be scheduled.
- The error names the schedule and the values it found, so an operator can fix the template without reading the whole definition.
- Every template that is compliant today publishes exactly the same command as before — same identifier, title, body, and frontmatter.
- The formatter itself enforces the rule, not merely the publisher that calls it.
- The generated test double is regenerated to match the formatter's new two-value contract.
- A publish-path test replays the incident scenario and proves the command sender is never called.
- The changelog and architecture notes record the guard.
</summary>

<objective>
Change the `FrontmatterFormatter.Format` seam to return `(lib.TaskFrontmatter, error)`, validate the operator-supplied map against the mode/role rule before any defaults, merge, or rendering, and propagate a refusal from `Publish` before the `task.CreateCommand` is sent. On compliance, return exactly the map today's implementation returns.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:

- `/workspace/pkg/publisher/frontmatter.go` — the `FrontmatterFormatter` interface (its GoDoc and the `Format` method declaration) and `frontmatterFormatter.Format`. This is the frozen seam that changes from one result to two.
- `/workspace/pkg/publisher/publisher.go` — `Publish` is the single production caller of `Format` (the `Frontmatter:` field of the `task.CreateCommand` literal). It already has `ctx`, `def.Slug`, and `date` in scope.
- `/workspace/pkg/publisher/mode_role_rule.go` — the rule landed by the previous prompt (`ModeRoleRule{Slug, Frontmatter}.Validate(ctx)`). Use it; do not reimplement the rule.
- `/workspace/pkg/publisher/frontmatter_test.go` — the formatter suite. Every `f.Format(` call site (roughly 25) must adopt the two-value return. Existing expected values must not change.
- `/workspace/pkg/publisher/publisher_test.go` — the publisher suite, including the capture-sender pattern (`sender *taskmocks.TaskCreateCommandSender`, `capture()` reading `SendCommandArgsForCall(0)`, `SendCommandCallCount()`), and the `Describe("sender interaction", ...)` block that proves a refused publish does not call the sender.
- `/workspace/pkg/publisher/publisher_suite_test.go` — the Ginkgo suite entry point and its `//go:generate` counterfeiter directive.
- `/workspace/mocks/publisher-frontmatter-formatter.go` — generated from the `//counterfeiter:generate` directive above the interface. It is regenerated, never hand-edited.
- `/workspace/pkg/factory/factory.go` — `CreatePublisher` wires `NewPublisher(... publisher.NewFrontmatterFormatter(renderer) ...)`. The `NewPublisher` and `NewFrontmatterFormatter` signatures do not change, so this file needs no edit; confirm it still compiles.
- `/workspace/pkg/tick/tick.go` — the tick loop already logs a per-slug `Publish` error and `continue`s to the next schedule. Do not change it; the guard's refusal surfaces as that existing error line.
- `/workspace/docs/architecture.md` — the `pkg/publisher` row in the Packages table describes the formatter's behavior; keep it accurate.
- `/workspace/CHANGELOG.md` — there is no `## Unreleased` heading today; the newest heading is `## v0.14.3`.

Library facts verified against the module cache, not memory:

- `lib.TaskFrontmatter` is `map[string]interface{}` (`lib "github.com/bborbe/agent"`).
- `schedule.Date` has fields `Year int`, `Month time.Month`, `Day int`, `Hour int`.
- `task.CreateCommand` has fields `TaskIdentifier`, `Title`, `Frontmatter`, `Body`, `TargetVault`.
- `github.com/bborbe/errors` v1.6.6: `func Wrapf(ctx context.Context, err error, format string, args ...interface{}) error` and `func Errorf(ctx context.Context, format string, args ...interface{}) error`.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` wrapping, including the no-`context.Background()` rule.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc accuracy after a signature change.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format.
</context>

<requirements>

<!-- OPEN QUESTION for audit: the spec freezes Format's parameter list (no context.Context), but validation.HasValidation requires a ctx. This prompt uses context.TODO() — not context.Background(), which docs/dod.md bans — with an explanatory comment. The alternative is adding ctx to Format, which the spec explicitly forbids. If the reviewer wants ctx threaded, the frozen seam must be reopened first. -->

### 1. Change the seam to return an error

In `/workspace/pkg/publisher/frontmatter.go`, change both the interface method and the implementation to return a second `error` result. The parameter list and order are frozen:

```go
Format(
	operator lib.TaskFrontmatter,
	slug string,
	date schedule.Date,
	autoAbortPrior bool,
	recurrence schedule.RecurrenceKind,
) (lib.TaskFrontmatter, error)
```

At the very top of `frontmatterFormatter.Format`, before building defaults, merging operator keys, or rendering anything, evaluate the rule against the operator-supplied map:

```go
if err := (ModeRoleRule{Slug: slug, Frontmatter: operator}).Validate(context.TODO()); err != nil {
	return nil, err
}
```

Add `"context"` to the import block. Add a short comment explaining that `Format`'s frozen signature carries no context and the rule is a pure in-memory check, so `context.TODO()` is used rather than `context.Background()` (which `docs/dod.md` bans in business logic).

On refusal return `nil, err`. On compliance, the rest of the function is unchanged: seed `status`/`page_type`/`phase`, merge operator keys (rendering scalar strings and list string entries), stamp `defer_date`, then `auto_abort_prior`, then force `created_by` last. Return `out, nil`.

Update the interface GoDoc and the `Format` method GoDoc to say that a contradictory `mode`/`role` pair is refused with a non-nil error and a nil map. Keep the existing contract text about defaults, rendering, `defer_date`, `auto_abort_prior`, and `created_by` ordering intact.

### 2. Propagate the refusal in `Publish`

In `/workspace/pkg/publisher/publisher.go`, hoist the formatter call above the `task.CreateCommand` literal and return before the command is built or sent:

```go
frontmatter, err := p.formatter.Format(
	def.Frontmatter,
	def.Slug,
	date,
	def.AutoAbortPrior,
	def.Recurrence,
)
if err != nil {
	return errors.Wrapf(
		ctx,
		err,
		"publish failed: format frontmatter for slug %q on %04d-%02d-%02d",
		def.Slug, date.Year, date.Month, date.Day,
	)
}
```

Then use `Frontmatter: frontmatter` in the `task.CreateCommand` literal. The sender must never be called for a refused template. `err` is already in scope from the identifier creation, so `frontmatter, err :=` is valid. Do not change the dry-run path, the title/body rendering, or the identifier derivation.

### 3. Regenerate the counterfeiter fake

Do not hand-edit `/workspace/mocks/publisher-frontmatter-formatter.go`. Run:

```bash
cd /workspace && make generate
```

`make generate` removes the stale mocks directory, recreates `mocks/mocks.go`, and runs `go generate -mod=mod ./...`. The regenerated `PublisherFrontmatterFormatter.Format` must return `(lib.TaskFrontmatter, error)` and expose the matching `FormatReturns(result1 lib.TaskFrontmatter, result2 error)` helpers. Confirm the mock package compiles.

### 4. Adapt existing formatter tests without changing expectations

In `/workspace/pkg/publisher/frontmatter_test.go`, update every `f.Format(...)` call site to the two-value form:

```go
fm, err := f.Format(...)
Expect(err).NotTo(HaveOccurred())
```

Apply the same pattern inside `DescribeTable` callbacks and the determinism spec (two calls, two error checks). Do NOT edit any existing expected value: no `HaveKeyWithValue`, `HaveLen`, `HaveKey`, `Equal`, `BeNil`, or YAML round-trip argument changes. The only edits at existing call sites are the added error result and its `NotTo(HaveOccurred())` assertion.

### 5. Add direct formatter refusal specs

In `/workspace/pkg/publisher/frontmatter_test.go`, add a `Describe("mode/role guard", ...)` block covering the formatter itself:

- A violating operator map (`{"mode": "interactive"}` with no role, slug `shutdown-k3s`) returns `fm == nil` and a non-nil error whose message contains `shutdown-k3s`. Assert `Expect(fm).To(BeNil())` and `Expect(err).To(HaveOccurred())`.
- A compliant pair (`{"mode": "interactive", "role": "human"}`) returns a non-nil map, a nil error, and still contains both keys unchanged.

### 6. Add the publish-path refusal specs

In `/workspace/pkg/publisher/publisher_test.go`, add a `Describe("mode/role pair", ...)` block so the spec's reproduction focus (`-ginkgo.focus='mode/role pair'`) selects it. Use the suite's real `pub` (real renderer, real formatter, real identifier creator) and the `sender` capture fake, which is fresh per `BeforeEach`.

- Publish the reproduction definition with slug `shutdown-k3s`, `TitleTemplate: "Shutdown K3s"`, `Recurrence: schedule.RecurrenceWeekly`, and `Frontmatter: lib.TaskFrontmatter{"mode": "interactive"}` on `schedule.NewDate(2026, time.June, 20)`. Assert a non-nil error whose message contains `shutdown-k3s`, `interactive`, and `absent`, and `Expect(sender.SendCommandCallCount()).To(Equal(0))`.
- Publish the same definition with `role: agent` added. Assert a non-nil error containing `shutdown-k3s` and `agent`, and `SendCommandCallCount()` equal to 0.
- Publish a compliant definition (`mode: interactive`, `role: human`). Assert `Publish` succeeds, `SendCommandCallCount()` equals 1, and the captured frontmatter still has `mode: interactive` and `role: human`.

### 7. Record the guard

- `/workspace/CHANGELOG.md`: add a `## Unreleased` heading immediately after the SemVer preamble block and before `## v0.14.3`. Add one `fix:` bullet describing the user-visible change; the bullet must mention `mode: interactive` and `role ... human` (or `manager`). Example: `- fix: refuse to publish a task whose frontmatter pairs \`mode: interactive\` with a role other than \`human\` or \`manager\`; the supervisor forces that pair to headless, so the declared mode was silently ignored`.
- `/workspace/docs/architecture.md`: in the `pkg/publisher` row of the Packages table, add one sentence stating that the publisher refuses to publish a task whose frontmatter pairs `mode: interactive` with a role other than `human` or `manager`. Do not restructure the table.

</requirements>

<constraints>
- **Frozen seam.** `Format` keeps the same parameter list and order; the only signature change is one added `error` result. The interface declaration and the implementation must match. No other method signature in the package changes.
- **Frozen call site.** The single production caller is `Publish` in `pkg/publisher/publisher.go` (the `Frontmatter:` field of the `task.CreateCommand` literal). No other production code calls `Format`.
- **Frozen mock surface.** `mocks/publisher-frontmatter-formatter.go` is generated from the `//counterfeiter:generate` directive above the interface; it is regenerated with `make generate`, never hand-edited.
- **Do NOT change `NewPublisher` or `NewFrontmatterFormatter` signatures.** `pkg/factory/factory.go`, `main.go`, and `cmd/run-once/main.go` need no changes; confirm they still compile.
- **Do NOT change `pkg/tick/tick.go`.** It already logs a per-slug publish error and continues to the next schedule, which is the required behavior when one schedule is refused and another is not.
- **Compliant output is byte-identical.** Same identifier, title, body, and frontmatter map (defaults, phase seed, computed `defer_date`, `auto_abort_prior`, `created_by` last, scalar and list placeholder rendering, determinism).
- **Guard scope is exactly the `mode`/`role` pair.** Do NOT validate any other frontmatter key.
- **Do NOT reject the pair at CRD admission.** No CEL rule, no schema change, no `make generatek8s`.
- **Do NOT change the supervisor or its role-mode matrix**, and do NOT change the default that an absent `role` resolves to `agent`.
- **Do NOT edit any `bborbe/nuke` schedule template.**
- **Do NOT add a warn-only mode, a `--strict` flag, an allowlist of exempt schedules, or any config knob that disables the guard.**
- **Do NOT make the guard placeholder-aware.** Compare the supplied `role` and `mode` strings literally; never render them first.
- Tests stay in the existing Ginkgo v2 / Gomega suite (`publisher_suite_test.go`, 60-second timeout). No new suite entry point, no stdlib table tests, no `*testing.T` outside the suite entry point, no mock for the pure rule.
- Project Definition of Done applies (`/workspace/docs/dod.md`): `github.com/bborbe/errors` for wrapping; GoDoc on every exported type and method; no `time.Now()`; no `context.Background()` in business logic (test files are exempt). `Format` uses `context.TODO()` for the pure in-memory check as described in requirement 1.
- License headers (BSD-2-Clause) on every modified/new `.go` file.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && go test -count=1 ./pkg/publisher/
```

Must exit 0.

Confirm the seam now returns an error (the printed declaration must end in `(lib.TaskFrontmatter, error)`):

```bash
cd /workspace && go doc ./pkg/publisher FrontmatterFormatter.Format
```

Confirm the rule is still composed from the validation library (must return ≥1 line):

```bash
cd /workspace && grep -n 'validation.All' pkg/publisher/*.go
```

Confirm the reproduction fixture exists in the publisher suite (non-zero count):

```bash
cd /workspace && grep -c 'shutdown-k3s' pkg/publisher/publisher_test.go
```

Regenerate the mock and confirm its `Format` signature is two-valued:

```bash
cd /workspace && make generate
cd /workspace && grep -n 'func (fake \*PublisherFrontmatterFormatter) Format' mocks/publisher-frontmatter-formatter.go
```

The printed line must end in `(lib.TaskFrontmatter, error)`.

Confirm no pre-existing expected value was edited (only added error handling and new test blocks are allowed; a changed assertion shows up as a `-` line):

```bash
cd /workspace && ! git diff HEAD -U0 -- pkg/publisher/frontmatter_test.go pkg/publisher/publisher_test.go | grep -qE '^-.*(HaveKeyWithValue|HaveLen|Equal)'
```

Confirm the changelog entry is under `## Unreleased`:

```bash
cd /workspace && awk '/^## Unreleased/{f=1;next} /^## /{f=0} f' CHANGELOG.md | grep -qE 'mode.*interactive|role.*human'
```

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If it exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.

Self-check before finishing: re-run the verification commands and confirm each result, then walk requirements 1-7 and confirm each is satisfied by the files on disk.
</verification>
