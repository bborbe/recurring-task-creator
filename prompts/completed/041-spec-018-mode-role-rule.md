---
status: completed
spec: [018-role-mode-interactive-guard]
summary: 'Landed and fully tested ModeRoleRule, the validation value type that refuses mode: interactive unless role is literally human or manager, with matrix/robustness specs and bborbe/validation now a direct dependency.'
execution_id: 68b0ced1-4024-5e3e-94db-39df70648379-exec-041-spec-018-mode-role-rule
dark-factory-version: dev
created: "2026-11-14T12:50:00Z"
queued: "2026-11-14T13:07:44Z"
started: "2026-11-14T13:14:58Z"
completed: "2026-11-14T13:16:28Z"
branch: dark-factory/role-mode-interactive-guard
---

# Land the mode/role pairing rule

<summary>
- The generator gains a reusable rule that recognizes the self-contradictory frontmatter pair `mode: interactive` with any role other than `human` or `manager`.
- A refused pair produces an error naming the schedule slug, the observed `mode` value, and the `role` value it found — or the literal word `absent` when the key is missing.
- Pairs the supervisor already treats as interactive (`role: human`, `role: manager`) stay valid.
- Schedules that declare no interactive mode, or that carry unusual value types, are unaffected and never panic.
- A role that looks like a placeholder is compared as a literal string, so it cannot slip past the guard.
- The rule is built from the shared validation library rather than ad-hoc conditionals.
- This prompt only lands and tests the rule; it does not change what the publisher sends yet.
- A full table test pins every accepted and refused combination, so removing the rule breaks the build.
</summary>

<objective>
Add a self-contained, fully tested validation value type in `pkg/publisher` that encodes the supervisor's role-mode contract: `mode: interactive` is legal only when `role` is `human` or `manager`. The type reports a precise error naming the schedule slug and the offending values. No wiring into the publish path happens in this prompt.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:

- `/workspace/pkg/publisher/frontmatter.go` — the `FrontmatterFormatter` interface and `frontmatterFormatter.Format`. This prompt does NOT change them; it adds a sibling rule type that the next prompt will wire in.
- `/workspace/pkg/publisher/publisher.go` — `Publish`, the only production caller of the formatter. Also unchanged here.
- `/workspace/pkg/publisher/frontmatter_test.go` and `/workspace/pkg/publisher/publisher_suite_test.go` — the existing Ginkgo v2 / Gomega suite style (`package publisher_test`, 60-second suite timeout). The new test file follows this style.
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition.Frontmatter` is the alias `type Frontmatter = map[string]interface{}`.
- `/workspace/go.mod` — `github.com/bborbe/validation v1.5.9` is currently an indirect requirement (in the second `require` block with a `// indirect` marker).

Library facts verified against the vendored module cache, not memory:

- `lib.TaskFrontmatter` is `type TaskFrontmatter map[string]interface{}` from `github.com/bborbe/agent` (imported as `lib "github.com/bborbe/agent"` in `pkg/publisher/frontmatter.go`).
- `github.com/bborbe/validation` v1.5.9:
  - `type HasValidation interface { Validate(ctx context.Context) error }`
  - `type HasValidationFunc func(ctx context.Context) error` (implements `HasValidation`)
  - `type All []HasValidation` with `func (l All) Validate(ctx context.Context) error`; wraps a failure as `"validate failed: ..."`
  - `func Name(fieldname string, validation HasValidation) HasValidation`; wraps a failure as `"validate <fieldname> failed: ..."`
- `github.com/bborbe/errors` v1.6.6: `func Errorf(ctx context.Context, format string, args ...interface{}) error`.

The supervisor's rule being encoded (from the spec): `role: human` and `role: manager` are forced to interactive; an absent `role` defaults to `agent`, and `agent` is forced to headless. Therefore `mode: interactive` plus any role outside {`human`, `manager`} is self-contradictory.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-validation-framework-guide.md` — the `validation.All` / `validation.Name` / `validation.HasValidationFunc` composition pattern.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` wrapping rules.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable` / `Entry` (no stdlib table tests).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc on exported identifiers.
</context>

<requirements>

### 1. Add the rule value type

Create `/workspace/pkg/publisher/mode_role_rule.go` with the BSD-2-Clause license header. Define an exported value type that implements `validation.HasValidation`:

```go
// ModeRoleRule validates the supervisor-consumed mode/role pair in an
// operator-supplied frontmatter map.
type ModeRoleRule struct {
	Slug        string
	Frontmatter lib.TaskFrontmatter
}

// Validate implements validation.HasValidation.
func (r ModeRoleRule) Validate(ctx context.Context) error
```

`Validate` must be composed from the validation library, not a hand-rolled chain:

```go
return validation.All{
	validation.Name("mode/role pair", validation.HasValidationFunc(r.validateModeRole)),
}.Validate(ctx)
```

Put the branching in an unexported method `func (r ModeRoleRule) validateModeRole(ctx context.Context) error`. The rule is pure and in-memory: no I/O, no clock, no retry, no goroutines.

Required behavior, evaluated against the operator-supplied map exactly as supplied (no defaults, no merge, no placeholder rendering):

- If the map is nil, or `mode` is absent, or `mode` is not the string `"interactive"`: return `nil`.
- If `mode` is the string `"interactive"` and the `role` key is absent: return an error.
- If `mode` is the string `"interactive"` and `role` is present but is not a string (bool, int, `[]interface{}`, etc.): return `nil` (no constraint).
- If `mode` is the string `"interactive"` and `role` is a string: return `nil` when the role is exactly `"human"` or `"manager"`; return an error for every other string, including placeholder-looking strings such as `"{{iso-week}}"` (compare the literal supplied string, never its rendered form).
- Never assume a key exists, never type-assert without the comma-ok form, and never dereference a nil map. Every non-matching shape passes.

Use `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors` for both refusal branches. The message must contain, in order of what an operator needs:

- the schedule slug verbatim (for example via `%q`),
- the observed `mode` value (`interactive`),
- the `role` value it found, or the literal word `absent` when the key is missing.

Recommended wording (agent may adjust tokens around these, but these substrings are asserted by tests):

```go
return errors.Errorf(ctx,
	"schedule %q declares mode %q with role absent; interactive requires role human or manager",
	r.Slug, mode)
```

```go
return errors.Errorf(ctx,
	"schedule %q declares mode %q with role %q; interactive requires role human or manager",
	r.Slug, mode, role)
```

Do NOT dump the whole frontmatter map into the error — it can carry unrelated operator content (assignees, notes, goal links) into logs.

Add GoDoc to the exported type and method. Do not add a constructor unless the implementation needs one; a struct literal from the same package and from tests is sufficient.

### 2. Make the dependency direct

The new import of `github.com/bborbe/validation` makes it a direct requirement. Run `go mod tidy` (or `make ensure`) so the module moves out of the `// indirect` block in `/workspace/go.mod`. Do not hand-edit the version; keep `v1.5.9`.

### 3. Add the matrix and robustness tests

Create `/workspace/pkg/publisher/mode_role_rule_test.go` in `package publisher_test` with the BSD-2-Clause license header. Follow the existing suite style: `Describe("ModeRoleRule", ...)`, `DescribeTable` / `Entry`, `. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`, `lib "github.com/bborbe/agent"`, and `"github.com/bborbe/recurring-task-creator/pkg/publisher"`. Call `ModeRoleRule{...}.Validate(context.Background())` (test code may use `context.Background()`).

The table must assert an error for all of:

- `mode: interactive`, `role` absent
- `mode: interactive`, `role: agent`
- `mode: interactive`, `role: "{{iso-week}}"` (literal comparison, not rendered)

The table must assert a nil error for all of:

- `mode: interactive`, `role: human`
- `mode: interactive`, `role: manager`
- no `mode`, `role: agent`
- `mode: headless`, `role: agent`
- nil frontmatter map
- `mode` a bool, an int, and a `[]interface{}`
- `mode: interactive` with `role` an int, a bool, and a `[]interface{}`

Add separate `It` blocks for the error-message contract:

- fixture slug `shutdown-k3s` with `mode: interactive` and no role: the error contains `shutdown-k3s`, `interactive`, and `absent` (use `MatchError(ContainSubstring(...))`).
- fixture slug `shutdown-k3s` with `mode: interactive` and `role: agent`: the error contains `shutdown-k3s`, `interactive`, and `agent`.
- fixture slug `weekly-review` with `mode: interactive` and no role: the error contains `weekly-review` and does NOT contain `shutdown-k3s` (proves the slug is taken from the input, not hardcoded).

The refusal entries are the lock that fails if the rule is removed or widened: each asserts `HaveOccurred()`, and each acceptance entry asserts `NotTo(HaveOccurred())`.

### 4. Do not wire it in yet

Do NOT change `FrontmatterFormatter.Format`, `frontmatterFormatter.Format`, `Publish`, the counterfeiter fake, or any existing test. This prompt lands the rule in isolation; the next prompt changes the seam.

</requirements>

<constraints>
- **Guard scope is exactly the `mode`/`role` pair.** Do NOT validate any other frontmatter key.
- **Do NOT reject the pair at CRD admission.** No CEL rule, no schema change, no `make generatek8s`.
- **Do NOT change the supervisor or its role-mode matrix**, and do NOT change the default that an absent `role` resolves to `agent`.
- **Do NOT edit any `bborbe/nuke` schedule template.**
- **Do NOT add a warn-only mode, a `--strict` flag, an allowlist of exempt schedules, or any config knob that disables the guard.**
- **Do NOT make the guard placeholder-aware.** Compare the supplied `role` and `mode` strings literally; never render them first.
- **The guard is publish-time only.** It is not a schema or admission concern.
- Tests stay in the existing Ginkgo v2 / Gomega suite (`publisher_suite_test.go`, 60-second timeout). No new suite entry point, no stdlib table tests, no `*testing.T` outside the suite entry point, no mock for the pure rule.
- Project Definition of Done applies (`/workspace/docs/dod.md`): `github.com/bborbe/errors` for error creation/wrapping; GoDoc on every exported type and method; no `time.Now()`; no `context.Background()` in business logic (test files are exempt).
- Use `github.com/bborbe/validation` (`validation.All` / `validation.Name` / `validation.HasValidationFunc`); no hand-rolled `if` chain in `Validate`.
- License headers (BSD-2-Clause) on every new `.go` file.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && go test -count=1 ./pkg/publisher/
```

Must exit 0.

Confirm the composition is present in the new file (each must return ≥1 line):

```bash
cd /workspace && grep -n 'validation.All' pkg/publisher/mode_role_rule.go
cd /workspace && grep -n 'validation.Name' pkg/publisher/mode_role_rule.go
cd /workspace && grep -n 'validation.HasValidationFunc' pkg/publisher/mode_role_rule.go
```

Confirm the exported rule and its method are documented:

```bash
cd /workspace && go doc ./pkg/publisher ModeRoleRule
```

Must print the `ModeRoleRule` type and a `Validate(ctx context.Context) error` method.

Confirm the dependency is no longer indirect:

```bash
cd /workspace && ! grep -q 'bborbe/validation.*// indirect' go.mod
```

Confirm the fixture and matrix entries exist (must return ≥1 line each):

```bash
cd /workspace && grep -n 'shutdown-k3s' pkg/publisher/mode_role_rule_test.go
cd /workspace && grep -n 'absent' pkg/publisher/mode_role_rule_test.go
```

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If it exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.

Self-check before finishing: re-run the verification commands and confirm each result, then walk requirements 1-4 and confirm each is satisfied by the files on disk.
</verification>
