---
status: completed
summary: 'Wired "personal" as the second arg of task.NewCreateCommandSender in main.go and cmd/run-once/main.go, and added an ## Unreleased CHANGELOG entry. NoopSender path untouched. make precommit passes.'
container: recurring-task-creator-default-vault-exec-011-default-target-vault-personal
dark-factory-version: v0.177.1
created: "2026-06-15T18:35:00Z"
queued: "2026-06-15T16:58:41Z"
started: "2026-06-15T17:01:27Z"
completed: "2026-06-15T17:06:45Z"
---

<summary>
- Recurring tasks should land in the Personal Obsidian vault, not OpenClaw. The agent-task-controller routes `task.CreateCommand` events by `TargetVault`; an empty field falls back to `openclaw` (legacy behavior). To target Personal, every published command needs `TargetVault: "personal"`.
- The `task.NewCreateCommandSender` constructor (in `github.com/bborbe/agent/lib/command/task` v0.68.0+) takes a second argument `defaultVault string` that stamps the default onto every command whose `TargetVault` is empty.
- This prompt wires the constant `"personal"` as the second argument at both Kafka-sender callsites: the main server (`main.go`) and the smoke-test CLI (`cmd/run-once/main.go`).
- No new tests required — the routing predicate + sender substitution are exhaustively tested in `bborbe/agent/lib/command/task`. The change here is one constant string per callsite.
- Every existing test in this repo continues to pass; `make precommit` must remain green.
- Adds an `## Unreleased` CHANGELOG entry naming the default-vault wiring.
</summary>

<objective>
After this prompt lands, every `task.CreateCommand` produced by `recurring-task-creator` carries `TargetVault: "personal"`, so the personal `agent-task-controller` instance (deployed with `VAULT_NAME=personal`) is the one that materializes the recurring tasks in the vault. End state: `grep -c 'NewCreateCommandSender' main.go cmd/run-once/main.go` returns one match each, and each match passes the literal string `"personal"` as the second positional argument.
</objective>

<context>
- `/workspace/main.go` — server entry point. Constructs the Kafka-backed sender at ~line 86 with one argument; needs the second argument added.
- `/workspace/cmd/run-once/main.go` — single-tick CLI. Same shape as `main.go` at ~line 68; same edit.
- `/workspace/CHANGELOG.md` — needs an `## Unreleased` entry naming the change.
- `/workspace/CLAUDE.md` — project conventions
- Upstream library: `github.com/bborbe/agent/lib/command/task` at v0.68.0. The new signature is `func NewCreateCommandSender(sender cdb.CommandObjectSender, defaultVault string) CreateCommandSender`. Empty `defaultVault` preserves pre-spec behavior (no substitution).
</context>

<requirements>

1. **Update `/workspace/main.go`** — server entrypoint sender wiring

   Find the existing block (around line 86):
   ```go
   sender = task.NewCreateCommandSender(cdb.NewCommandObjectSender(
       syncProducer,
       cqrsbase.Branch(a.Stage),
       liblog.DefaultSamplerFactory,
   ))
   ```

   Add the second argument `"personal"`:
   ```go
   sender = task.NewCreateCommandSender(cdb.NewCommandObjectSender(
       syncProducer,
       cqrsbase.Branch(a.Stage),
       liblog.DefaultSamplerFactory,
   ), "personal")
   ```

   The closing paren of `cdb.NewCommandObjectSender(...)` is followed by `, "personal"` then the closing paren of `task.NewCreateCommandSender(...)`. Preserve the surrounding indentation; one new arg, no other edits.

2. **Update `/workspace/cmd/run-once/main.go`** — smoke-test CLI sender wiring

   Find the existing block (around line 68):
   ```go
   sender = task.NewCreateCommandSender(cdb.NewCommandObjectSender(
       syncProducer,
       cqrsbase.Branch(a.Stage),
       liblog.DefaultSamplerFactory,
   ))
   ```

   Apply the same change:
   ```go
   sender = task.NewCreateCommandSender(cdb.NewCommandObjectSender(
       syncProducer,
       cqrsbase.Branch(a.Stage),
       liblog.DefaultSamplerFactory,
   ), "personal")
   ```

3. **No change to the dry-run / NoopSender path**

   The DRY_RUN path uses `publisher.NewNoopSender()` (no real sender involved). Do NOT touch that branch — `NewNoopSender` does not take a defaultVault. Re-confirm in `main.go` and `cmd/run-once/main.go` that the dry-run sender construction is unchanged after your edits.

4. **Update `/workspace/CHANGELOG.md`** — add `## Unreleased` entry above the latest version header

   Insert:
   ```markdown
   ## Unreleased

   - feat(main, cmd/run-once): stamp `TargetVault: "personal"` on every published `CreateCommand` so recurring tasks land in the Personal vault. The new second argument of `task.NewCreateCommandSender` (added in `github.com/bborbe/agent/lib` v0.68.0) is wired with the constant `"personal"`. Empty input `TargetVault` is substituted; explicit non-empty values are preserved.
   ```

5. **Verification — `make precommit` must pass**

   ```bash
   cd /workspace && make precommit
   ```

6. **Verification — both callsites now pass `"personal"`**

   ```bash
   cd /workspace && grep -E 'NewCreateCommandSender\b' main.go cmd/run-once/main.go
   ```

   Both lines should end with `), "personal")` (or have `"personal"` as the second positional argument depending on formatting).

</requirements>

<constraints>
- Pure two-line wiring change + a CHANGELOG bullet
- Do NOT bump the upstream `github.com/bborbe/agent/lib` version in `go.mod` if it is not already at v0.68.0 or higher. If `go.mod` shows a version below v0.68.0, the prompt has been queued ahead of its dependency — **abort with a failure status** naming the required minimum (`github.com/bborbe/agent/lib v0.68.0`), do not attempt to bump
- Do NOT add a config knob, env var, or constructor parameter for the default vault. The string `"personal"` is the deployment-time decision; making it configurable is YAGNI
- Do NOT touch the dry-run / `NewNoopSender` path
- Do NOT change `pkg/schedule/`, `pkg/publisher/`, `pkg/tick/`, `pkg/factory/`, or any test file. The change is at the composition root only
- Preserve `bborbe/errors` 3-arg `Wrap(ctx, err, msg)` and `Wrapf` — no `fmt.Errorf`
- No `context.Background()` in business logic
- No `go mod vendor` — the project does not vendor
</constraints>

<verification>
```bash
# 1. Tests pass
cd /workspace && make precommit

# 2. Both callsites pass "personal" as second arg
cd /workspace && grep -E '"personal"' main.go cmd/run-once/main.go
# Expect at least one match in each file

# 3. NoopSender path untouched
cd /workspace && grep -n 'NewNoopSender' main.go cmd/run-once/main.go
# Expect the noop call still has zero args

# 4. CHANGELOG has the Unreleased entry
cd /workspace && grep -A 3 '## Unreleased' CHANGELOG.md | grep -E 'TargetVault.*personal|personal.*TargetVault'
```
</verification>
