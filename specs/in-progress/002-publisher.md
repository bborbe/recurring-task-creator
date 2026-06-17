---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-14T11:32:54Z"
generating: "2026-06-14T11:36:22Z"
prompted: "2026-06-14T11:48:06Z"
verifying: "2026-06-14T12:05:12Z"
branch: dark-factory/publisher
---

## Summary

- Turn one `(TaskDefinition, Date)` pair into one validated `task.CreateCommand` ready to send to Kafka.
- Render the placeholder set frozen in Spec 1 (`{{date}}`, `{{iso-week}}`, `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`, `{{last-quarter}}`, `{{year}}`, `{{last-year}}`) into title and body, using the exact `YYYYWWW` (uppercase `W`) and `YYYYQQ` (uppercase `Q`) shapes the source providers emit.
- Identify each command deterministically by `UUID5(namespace, "recurring-<slug>-<YYYY-MM-DD>")` so the same `(slug, date)` always produces the same `TaskIdentifier` across reboots, redeploys, and manual replays — the controller's de-dup hinges on this.
- Stamp every command's frontmatter with the migration's fixed shape (`assignee: bborbe`, `status: in_progress`, `page_type: task`, `goals`, `priority: 2`, `recurring: <kind>`) so vault files land in the right place without per-task wiring.
- Out of scope: hourly cron tick (Spec 3), HTTP `/trigger` (Spec 4), K8s manifests (Spec 5). Deleting `pkg/handler` / `pkg/mathutil` is also out of scope. The publisher only displaces `pkg/factory`'s skeleton handler wiring if and where it must.

## Problem

Spec 1 produced a pure `TasksForDate(date) []TaskDefinition` lookup but no way to ship anything. Without a publisher, the inventory cannot reach Kafka and the controller cannot materialize vault files — the migration off `jira-task-creator` stays stuck at "we know what should fire but cannot fire it." The publisher is the smallest unit that converts an inventory row plus a date into a wire-ready command and hands it to a `task.CreateCommandSender`. It also locks in the determinism contract (UUID5 keyed by slug+date) on which the entire idempotency story rests: if two ticks one hour apart produce different identifiers for the same task, the controller will create duplicate vault files and the migration's "hourly idempotent cron" guarantee collapses. Encoding identifier generation, placeholder rendering, and frontmatter shape in one auditable package now — before the cron loop and HTTP trigger compose on top — is the only way to keep that contract testable in isolation.

## Goal

After this work, a single in-process `Publish(ctx, def, date)` call constructs the canonical `task.CreateCommand` for that `(definition, date)` pair, sends it via an injected `task.CreateCommandSender`, and returns. The identifier is the deterministic UUID5 derived from `recurring-<slug>-<YYYY-MM-DD>`; calling `Publish` for the same pair on a second tick produces a byte-identical command (same identifier, same title, same body, same frontmatter). The factory layer wires a real Kafka-backed `CreateCommandSender` from a `libkafka.SyncProducer` so the binary (in a later spec) can compose schedule + publisher without bespoke plumbing. No cron, no HTTP, no inventory traversal lives in the publisher — those compose on top of it.

## Non-goals

- Do NOT introduce an hourly cron loop, `time.Now` reads, or any scheduling — Spec 3.
- Do NOT add an HTTP `/trigger?date=` handler — Spec 4.
- Do NOT add K8s manifests, secrets, or deploy plumbing — Spec 5.
- Do NOT walk `TasksForDate` inside the publisher — `Publish` takes one `TaskDefinition` and one `Date`. The caller (cron in Spec 3) does the iteration. This keeps the publisher unit-testable without a fake inventory.
- Do NOT implement de-duplication inside the publisher — idempotency is the controller's job, achieved by the deterministic identifier. The publisher always sends; the controller no-ops on duplicate identifiers.
- Do NOT add per-task opt-out flags, runtime toggles, or any mechanism to disable specific tasks from config — invariant; if a future consumer demands variation, that is a separate spec.
- Do NOT make the UUID5 namespace configurable — it is a package-level constant. Changing it is a breaking change to the entire Kafka stream and must come with its own spec.
- Do NOT make the frontmatter shape (assignee, status, page_type, goals, priority) configurable — invariant for this migration; per-task overrides are out of scope.
- Do NOT delete the skeleton example packages `pkg/handler` or `pkg/mathutil` — they are unrelated to the publisher and untouched here. `pkg/factory` is edited only to add publisher wiring; existing skeleton handler factories may stay or be removed alongside, agent decides at impl time based on whether they still compile.
- Do NOT batch, buffer, or retry inside the publisher — one call, one send. Retry / backoff is the cron loop's concern (Spec 3); Kafka send failures bubble up wrapped.
- Do NOT introduce a `Publisher` interface with multiple implementations — one concrete type, one constructor. If a future consumer demands a fake, generate a counterfeiter mock from the concrete interface defined alongside.

## Desired Behavior

1. A single Go package owns the "definition + date → CreateCommand + send" transformation.
2. The publisher exposes one method that takes a context, one `TaskDefinition` (from `pkg/schedule`), and one `Date` (from `pkg/schedule`), and returns an error.
3. The constructed `TaskIdentifier` is `UUID5(namespace, "recurring-<slug>-<YYYY-MM-DD>")` where `namespace` is a package-level UUID constant. Same `(slug, date)` always yields the same identifier across processes.
4. The constructed `Title` is `def.TitleTemplate` with every supported placeholder replaced by its rendered value for `date`. Unknown placeholders are impossible at this layer because Spec 1 guarantees only `SupportedPlaceholders` appear in inventory entries.
5. The constructed `Body` is `def.BodyTemplate` with the same placeholder substitution applied. Body may be empty; empty bodies are valid.
6. Placeholder rendering uses the formats frozen in Spec 1: `{{date}}` → `YYYY-MM-DD`; `{{iso-week}}` → `YYYYWWW` (uppercase `W`, e.g. `2025W01`); `{{next-iso-week}}` → the ISO week of `date + 7 days` in the same `YYYYWWW` shape; `{{month}}` → `YYYY-MM`; `{{last-month}}` → the previous calendar month in `YYYY-MM` (December rolls year back); `{{quarter}}` → `YYYYQQ` (uppercase `Q`, e.g. `2025Q01`); `{{last-quarter}}` → the previous quarter in `YYYYQQ` (Q1 rolls year back); `{{year}}` → `YYYY`; `{{last-year}}` → `YYYY - 1`.
7. The constructed `Frontmatter` is exactly: `assignee: bborbe`, `status: in_progress`, `page_type: task`, `goals: ["[[Example Goal]]"]`, `priority: 2`, `recurring: <kind>` where `<kind>` is the string value of `def.Recurrence` (`daily | weekly | monthly | quarterly | yearly`). No other keys are set by the publisher.
8. The publisher delegates wire formatting and Kafka I/O entirely to the injected `task.CreateCommandSender` — it never imports `kafka-go` or `sarama` directly.
9. Send failures from the underlying sender are wrapped with `github.com/bborbe/errors` carrying enough context to identify the slug and date (so cron-loop logs in Spec 3 are actionable).
10. A factory function in `pkg/factory` constructs the publisher from a `task.CreateCommandSender` (which itself is built from a `libkafka.SyncProducer` via the existing `factory.CreateKafkaCreateSender` shape). Wiring follows the canonical pattern in `~/Documents/workspaces/maintainer/watcher/github-build/pkg/factory/factory.go`.

## Constraints

- Module pre-check: the publisher introduces direct dependencies on `github.com/bborbe/agent/lib` (for `TaskFrontmatter`, `TaskIdentifier`) and `github.com/bborbe/agent/lib/command/task` (for `CreateCommand`, `CreateCommandSender`). Confirmed importable from this private repo as of 2026-06-13 (public module, `go get` clean, `agent/lib` v0.65.0+). `github.com/bborbe/cqrs/base` arrives transitively via `agent/lib/command/task`. `github.com/google/uuid` is required for UUID5; pin via `go get` to the version already used in the bborbe ecosystem.
- The UUID5 namespace is a package-level constant of type `uuid.UUID`, defined once, never read from env or flag. Its literal value MUST be pinned in code with a comment marking it as immutable (changing it is a breaking change to every downstream identifier).
- The package MUST NOT import `net/http`, `time.Now`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/bborbe/jira-task-creator/...`, or anything that reads a clock or opens a connection. All time-derived values come from the `Date` argument.
- The package MUST NOT walk the inventory; it MUST NOT call `schedule.TasksForDate`. The caller is responsible for iteration.
- Placeholder substitution MUST be implemented as one `strings.ReplaceAll` per token in `schedule.SupportedPlaceholders` order — no regex, no template engine. The closed placeholder set keeps this cheap and auditable.
- `CreateCommand.Validate` (from `agent/lib/command/task`) MUST pass for every command this publisher constructs from a valid inventory entry. The publisher does not duplicate validation; it relies on `CreateCommandSender.SendCommand` calling `Validate` internally.
- The publisher follows the project's standard architecture: Interface → Constructor → Struct → Method pattern (see `~/Documents/workspaces/coding/docs/go-architecture-patterns.md`).
- Tests follow Ginkgo v2 / Gomega; mocks generated with counterfeiter against the `task.CreateCommandSender` interface (see `~/Documents/workspaces/coding/docs/go-mocking-guide.md`, `~/Documents/workspaces/coding/docs/go-testing-guide.md`).
- Error wrapping uses `github.com/bborbe/errors.Wrap`/`Wrapf` per `~/Documents/workspaces/coding/docs/go-error-wrapping-guide.md`.
- Factory wiring follows `~/Documents/workspaces/coding/docs/go-factory-pattern.md`.
- `make precommit` MUST pass in the changed module.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---------|-------------------|----------|-----------|---------------|-------------|
| `def.Slug` is empty | Publisher returns wrapped error before constructing identifier or calling sender | Caller (cron) logs and skips; inventory invariant in Spec 1 already prevents this | Returned error contains the literal string `empty slug` | Reversible (no side effect occurred) | N/A — no shared state |
| `date.IsZero()` is true | Publisher returns wrapped error before calling sender | Caller logs and skips | Returned error contains the literal string `zero date` | Reversible | N/A |
| Kafka send fails (broker down, network error) | Publisher returns wrapped error from `SendCommand` with slug and ISO date in the message | Cron loop (Spec 3) retries on the next tick; deterministic identifier means the retry is safe | Returned error chain contains slug substring and `YYYY-MM-DD` date substring | Reversible at the publisher layer; the message may or may not have reached Kafka, but the identifier guarantees the controller no-ops on the duplicate when it does | Two concurrent ticks for the same `(slug, date)` produce identical commands; controller de-dups on identifier |
| `CreateCommand.Validate` fails (title too long, forbidden char, body > 500 KiB) | `SendCommand` returns a validation error; publisher wraps it and returns | Fix the inventory entry; this is a bug in Spec 1's inventory, not a runtime condition | Returned error chain contains `validate CreateCommand` | Reversible (no Kafka I/O occurred — `SendCommand` validates first) | N/A |
| Same `(slug, date)` published twice (e.g. cron retries after broker hiccup) | Identifier is byte-identical on both sends; controller no-ops on the duplicate | No recovery needed; this is the designed idempotency path | Two Kafka messages with the same `taskIdentifier` field; controller log shows one create, one skip | Idempotent by design | Safe under concurrent calls — `Publish` holds no state between calls |
| Partial-progress crash between `Validate` and `SendCommand` | Either nothing was sent (no side effect) or the send completed before crash (controller de-dups on next tick) | Cron's next tick re-publishes — deterministic identifier makes this safe | Operator sees either no message or one message; never a corrupted half-send | Reversible — at-least-once delivery with idempotent identifier collapses to exactly-once at the vault layer | Cron tick + manual replay racing on the same `(slug, date)` produce identical commands; harmless |
| Sender argument is `nil` | Constructor panics at wire-time — never at request time (fail-fast factory convention, matches `factory.MustNew*` pattern across bborbe ecosystem) | Fix wiring in `pkg/factory` | Panic at startup with sender-name string in the panic message | N/A — startup failure, no production state | N/A |

## Security / Abuse Cases

Not applicable in the strict sense — this package has no HTTP surface, no file I/O, no env reads, no user input crossing a trust boundary in this layer. All inputs are typed values produced by `pkg/schedule` (a pure, in-process package). Templates are static, defined in Go source, not loaded at runtime. The only "user-controllable" value at any future layer is the date in `/trigger?date=` (Spec 4), and date parsing is that spec's concern; by the time it reaches the publisher, it is a typed `Date` value.

## Acceptance Criteria

- [ ] `make precommit` exits 0 in the recurring-task-creator module — evidence: exit code 0.
- [ ] Publisher package exists with a single concrete type and constructor following Interface → Constructor → Struct → Method — evidence: `grep -E '^(type|func New)' pkg/publisher/*.go` lists exactly one interface, one constructor, one struct, one `Publish` method.
- [ ] For `def.Slug = "weekly-review"`, `date = 2025-01-04`, `Publish` constructs a command whose `TaskIdentifier` is `UUID5(namespace, "recurring-weekly-review-2025-01-04")` — evidence: Ginkgo test compares the captured `CreateCommand.TaskIdentifier` byte-for-byte against the expected UUID5 literal hard-coded in the test (computed offline with the pinned namespace).
- [ ] Calling `Publish` twice with the same `(def, date)` produces two `CreateCommand` values with identical `TaskIdentifier`, `Title`, `Body`, and `Frontmatter` (deep equality) — evidence: Ginkgo `Expect(cmd1).To(Equal(cmd2))` passes via captured sender invocations.
- [ ] `{{date}}` in a title renders as `YYYY-MM-DD` for `2025-01-04` → `2025-01-04` — evidence: Ginkgo `Expect(cmd.Title).To(Equal("... 2025-01-04 ..."))` passes.
- [ ] `{{iso-week}}` renders as `YYYYWWW` for `2025-01-04` → `2025W01` (uppercase `W`, two-digit week with leading zero) — evidence: Ginkgo assertion on `cmd.Title` for an inventory entry whose template contains `{{iso-week}}`.
- [ ] `{{next-iso-week}}` for `2025-01-04` → `2025W02` (week of `date + 7 days`) — evidence: Ginkgo assertion on `cmd.Title`.
- [ ] `{{month}}` for `2025-01-04` → `2025-01`; `{{last-month}}` for `2025-01-04` → `2024-12` (year roll-back) — evidence: two Ginkgo assertions on `cmd.Title`.
- [ ] `{{quarter}}` for `2025-04-01` → `2025Q02`; `{{last-quarter}}` for `2025-01-01` → `2024Q04` (year roll-back) — evidence: two Ginkgo assertions on `cmd.Title`.
- [ ] `{{year}}` for `2025-04-01` → `2025`; `{{last-year}}` for `2025-01-01` → `2024` — evidence: two Ginkgo assertions on `cmd.Title`.
- [ ] Placeholder substitution applies to `Body` identically to `Title` — evidence: a test where the inventory entry has `{{date}}` in `BodyTemplate` shows `cmd.Body` contains the rendered date.
- [ ] `cmd.Frontmatter["assignee"] == "bborbe"`, `cmd.Frontmatter["status"] == "in_progress"`, `cmd.Frontmatter["page_type"] == "task"`, `cmd.Frontmatter["priority"] == 2`, `cmd.Frontmatter["goals"]` equals `[]string{"[[Example Goal]]"}` (or its `[]interface{}` equivalent if the type demands it) — evidence: Ginkgo assertions on each key.
- [ ] `cmd.Frontmatter["recurring"]` equals the string value of `def.Recurrence` for each of the five `RecurrenceKind` constants — evidence: a Ginkgo table test iterating over `daily | weekly | monthly | quarterly | yearly` confirms each lands as the matching string.
- [ ] `Publish` calls `CreateCommandSender.SendCommand` exactly once per call when inputs are valid — evidence: counterfeiter `SendCommandCallCount() == 1`.
- [ ] `Publish` does NOT call `SendCommand` when `def.Slug` is empty or `date.IsZero()` — evidence: counterfeiter `SendCommandCallCount() == 0`, and the returned error chain contains `empty slug` / `zero date` respectively.
- [ ] When `SendCommand` returns an error, `Publish` returns a wrapped error whose chain contains the slug substring and the ISO date `YYYY-MM-DD` — evidence: Ginkgo `Expect(err.Error()).To(ContainSubstring(slug))` and `ContainSubstring("2025-01-04")`.
- [ ] The package does NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, `github.com/bborbe/jira-task-creator/...`, or call `time.Now` — evidence: `grep -E '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/publisher/*.go` returns no matches.
- [ ] The package does NOT import `github.com/bborbe/recurring-task-creator/pkg/schedule`'s `TasksForDate` symbol (it may import the package for types only) — evidence: `grep -n 'TasksForDate' pkg/publisher/*.go` returns no matches.
- [ ] The UUID5 namespace constant is defined exactly once at package scope with a doc-comment marking it immutable — evidence: `grep -nE 'var.*uuid\.UUID|var.*Namespace' pkg/publisher/*.go` returns exactly one match; comment includes the word "immutable" or "frozen" or "never change".
- [ ] A counterfeiter mock for `task.CreateCommandSender` is generated under `mocks/` per the project's mocking convention — evidence: file exists at `mocks/task-create-command-sender.go` (or equivalent project-standard path) AND is referenced in publisher tests.
- [ ] `pkg/factory` exposes a constructor that builds the publisher from a `task.CreateCommandSender` (and ultimately from a `libkafka.SyncProducer` via the existing `CreateKafkaCreateSender` shape) — evidence: `grep -nE 'func Create.*Publisher' pkg/factory/*.go` returns at least one match; the function compiles and is exercised by a factory test that injects a counterfeiter `CreateCommandSender` fake.
- [ ] No scenario test added — covered by unit tests above; see scenario rule below.

Scenario coverage: NO new scenario. The publisher is pure transformation + one mockable sender call. Unit tests with a counterfeiter `CreateCommandSender` reach every behavior. No real Kafka broker, no real cluster, no real Docker required. The integration test that exercises real Kafka is deferred to Spec 3 (cron tick) or Spec 5 (deploy), whichever first composes the publisher with live infrastructure.

## Verification

```
cd ~/Documents/workspaces/recurring-task-creator-mvp
make precommit
```

Expected: exit code 0, all tests green, lint clean, license headers present, no forbidden imports.

## Suggested Decomposition

Single-layer spec (one new package, one factory edit, one mock generation). DB × AC ≈ 10 × 22 = 220 by raw count, but every AC except the factory wiring lives on the same `Publish` method. The placeholder-rendering ACs are mechanically generated from a single substitution loop and a small format-helper set. Splitting would create artificial seams between "build the command" and "wire the factory" that re-converge in the same package on the next prompt.

Recommend a single prompt. If the executor insists on splitting:

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Publisher package: types, constructor, `Publish` method, UUID5 namespace constant, placeholder rendering helpers, frontmatter builder, unit tests with counterfeiter mock | 1, 2, 3, 4, 5, 6, 7, 8, 9 | identifier, determinism, all placeholder render cases, frontmatter shape, error wrapping, no-forbidden-imports, mock-generated | — |
| 2 | Factory wiring: `pkg/factory` constructor that builds publisher from `task.CreateCommandSender` and `libkafka.SyncProducer`; factory test | 10 | factory-constructor-exists, factory-test-passes | prompt 1 |

Rationale: prompt 1 owns the contract; prompt 2 plugs it into the binary's wire graph. No cycles. Default is still single-prompt.

## Do-Nothing Option

Without the publisher, Spec 1's inventory is a museum piece — observable in tests, invisible to Kafka. The cron tick (Spec 3) has nothing to call. The migration off `jira-task-creator` stalls at the boundary between "we know what should fire" and "anything fires." Folding the publisher into the cron-tick spec would re-tangle two contracts (determinism + scheduling) and re-create exactly the verification problem that drove the spec split in the first place: a single failed cron test would no longer tell us whether the identifier is wrong or the loop is wrong. Existing `jira-task-creator` continues to fire correctly, so there is no production pressure, but every week of delay is another week of recurring tasks not landing in the vault. Recommendation: do this spec next, exactly as scoped.
