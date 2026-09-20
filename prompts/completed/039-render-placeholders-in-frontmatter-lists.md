---
status: completed
summary: FrontmatterFormatter now renders placeholder tokens in the string entries of list-valued frontmatter, with GoDoc, specs, README, and changelog updated
execution_id: recurring-task-creator-frontmatter-list-exec-039-render-placeholders-in-frontmatter-lists
dark-factory-version: v0.196.0
created: "2026-09-20T00:00:00Z"
queued: "2026-09-20T09:33:21Z"
started: "2026-09-20T10:03:35Z"
completed: "2026-09-20T10:09:32Z"
---

# render placeholders in frontmatter list elements

<summary>
- Operator-supplied frontmatter list values now get their `{{...}}` placeholder tokens substituted, matching what already happens for scalar string values.
- A Schedule CR can therefore declare a list whose entries name the week they fire in, instead of being stuck with a hardcoded literal that goes stale.
- Lists whose entries contain no placeholder are returned byte-identical to today, so every existing Schedule CR keeps behaving exactly as it does now.
- Non-string list entries (numbers, nested maps) pass through untouched — only string entries are rendered.
- The formatter's documented contract now covers list rendering alongside scalar rendering.
- No CRD schema change, no new placeholder token, and no change to title or body rendering.
</summary>

<objective>
Teach the publisher's frontmatter formatter to substitute placeholder tokens inside operator-supplied list values, so a Schedule CR can express a dependency on another recurring task by its materialized name (which carries the firing week) rather than a literal that silently rots at the next period boundary.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

Read these files fully before changing anything:

- `/workspace/pkg/publisher/frontmatter.go` — `FrontmatterFormatter` (the interface GoDoc at the top and the `Format` method's own GoDoc) and `frontmatterFormatter.Format`. The merge loop is the only behavioral code change in this file. Note the shape: `if s, ok := v.(string); ok { out[k] = f.renderer.Render(...); continue }` then `out[k] = v`.
- `/workspace/pkg/publisher/renderer.go` — the `Renderer` interface and its `Render(template, slug string, date schedule.Date) string` method. This is the single seam for placeholder substitution; reuse it, do not add a second substitution path.
- `/workspace/pkg/publisher/placeholders.go` — the closed placeholder set. `{{current_week}}` renders `YYYYWNN` (e.g. `2026W38`) and `{{current_date}}` renders `YYYY-MM-DD`.
- `/workspace/pkg/publisher/frontmatter_test.go` — the existing spec suite. Note the `It("passes non-string values through unchanged (int, slice, map)")` spec (~line 198): its fixture list entries contain NO placeholder tokens, so its assertions must keep passing unchanged.
- `/workspace/pkg/publisher/publisher_test.go` — two existing fixtures carry a token-free `[]interface{}` frontmatter list (`"goals": []interface{}{"[[Example Goal]]"}` at ~lines 929 and 1053); both must keep passing unchanged.
- `/workspace/README.md` (~line 81) and `/workspace/CHANGELOG.md` (SemVer preamble + `## v0.12.1` heading at the top) — the two non-Go files requirements 4 and 5 change.
- `lib.TaskFrontmatter` (imported as `lib "github.com/bborbe/agent"` in `pkg/publisher/frontmatter.go`) is `map[string]interface{}`, so the operator map is untyped and a YAML list arrives as `[]interface{}`. The dep is NOT vendored (`vendor/` is gitignored and `make ensure` deletes it), so there is no in-container source path — use `go doc github.com/bborbe/agent.TaskFrontmatter` if you need to confirm the type.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, DescribeTable.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc accuracy.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format.

Prior art worth knowing: PR #16 ("render placeholders in operator frontmatter string values") introduced this renderer seam and deliberately scoped it to scalar strings. This prompt extends that same seam to list entries — it does not replace or re-architect it.
</context>

<requirements>

### 1. Render string entries inside list values

In `/workspace/pkg/publisher/frontmatter.go`, extend the merge loop inside `frontmatterFormatter.Format` so that list values have their string entries rendered.

The loop currently has exactly two arms (render scalar string / pass everything else through). Add a list arm between them. It must handle the CR/YAML list shape, `[]interface{}` — the shape the CR path produces and the shape the existing specs assert against.

Scope note: handle `[]interface{}` only. A `[]string` value has no production caller in this repo (the CR path is YAML → `[]interface{}`), so adding an arm for it would be unrequested variation — if a real `[]string` caller ever appears, that is a separate change.

Required behaviour:

- Each string entry is rendered through the injected renderer: `f.renderer.Render(entry, slug, date)`.
- Each non-string entry in the list is preserved as-is (an int stays an int, a nested map stays a nested map) — do not stringify or drop it.
- The result stays a `[]interface{}` — callers and existing assertions compare these types.
- A list whose entries contain no placeholder token must come out equal to its input. Rendering a token-free string is already a no-op, so this follows — but assert it, because it is the backwards-compatibility guarantee.

Factor the entry-rendering into a small unexported helper rather than inlining both loops into `Format`; `Format` is already carrying the merge, the defer-date stamp, and the provenance stamp.

Do NOT recurse into nested maps or into lists-of-lists. This change is scoped to a flat list of scalars — that is the shape every operator frontmatter list in use today has. Deep recursion is unrequested and untested; leave it out.

### 2. Keep the GoDoc honest

The `FrontmatterFormatter` type GoDoc (~line 21, "renders placeholder tokens in string values") and the `Format` method GoDoc (~line 30, "non-string values (ints, slices, maps) pass through unchanged") both become wrong once lists render.

Update them to say what is now true: string values AND the string entries of list values are rendered; other non-string values (ints, maps, non-string list entries) pass through unchanged. Keep the existing doc-comment style and keep the surrounding contract text (defaults, defer_date, auto_abort_prior, created_by ordering) intact. Sweep the sibling GoDoc that repeats the old rule and update it in the same pass: `pkg/publisher/placeholders.go` (~lines 22 and 59, "string-valued frontmatter entries"), `pkg/publisher/renderer.go` (~line 18), and `pkg/publisher/frontmatter.go` (~line 63, `NewFrontmatterFormatter` — "renders string-valued frontmatter"). This is a comment-only edit; do NOT add or remove a placeholder token. While you are on `placeholders.go` line 23, the `renderTemplate` name it cites no longer exists — the substitution function now lives behind `Renderer.Render` in `pkg/publisher/renderer.go` (see `pkg/publisher/render.go` lines 12-18). Correct that stale reference in the same pass.

### 3. Spec the new behaviour

In `/workspace/pkg/publisher/frontmatter_test.go`, add specs covering at minimum:

- A `[]interface{}` containing a token-bearing string renders it (`{{current_week}}` → the ISO week token for the suite's `date`) while a token-free sibling entry is untouched.
- A mixed `[]interface{}` (string with token, string without, and a non-string such as an int) renders only the token-bearing entry and preserves both others with their original types.
- A token-free list comes out equal to its input (the backwards-compatibility lock) — state explicitly that this is the `Format` output, not a YAML round-trip.
- A rendered list survives a YAML round-trip: marshal the result with `yaml.Marshal(map[string]interface{}(fm))`, unmarshal into `map[string]interface{}`, and assert the list still holds the rendered entry (mirrors the existing `auto_abort_prior` / `defer_date` round-trip specs in this file).

Put the new specs in a new `Describe("placeholder rendering in list entries", ...)` block (nested alongside the existing `Describe("placeholder rendering in string values")` at ~line 160, which stays as-is), and use the existing suite's `date` fixture and `DescribeTable` where it fits the file's established style. The existing "passes non-string values through unchanged" spec must remain and must keep passing without edits — if it fails, the change is wrong, not the spec.

### 4. Changelog

Add a `## Unreleased` section immediately after the SemVer preamble block in `/workspace/CHANGELOG.md` (there is no `## Unreleased` section today — the newest heading is `## v0.12.1`). One `feat:` bullet describing the user-visible change. The preamble must stay exactly where it is, above every version heading.

### 5. Update the README placeholder contract

`/workspace/README.md` line ~81 currently reads: "Substituted in `title`, `body`, and any **string-valued** `frontmatter` field. Non-string frontmatter values (ints, slices, maps) pass through unchanged." That sentence becomes false once list entries render. Rewrite it to say the substitution applies to `title`, `body`, any string-valued `frontmatter` field, and the string entries of list-valued `frontmatter` fields; non-string values and non-string list entries pass through unchanged. Leave the placeholder table and the v0.3.0 removal note below it untouched.

</requirements>

<constraints>
- Do NOT change the CRD schema (`pkg/k8s_connector_schema.go`) or any k8s type. `spec.template.frontmatter` is untyped (`TaskFrontmatter` = `map[string]interface{}`), so it already accepts a list value with no schema work. No `make generatek8s` is needed.
- Do NOT add a placeholder token to `pkg/publisher/placeholders.go`. The closed set is unchanged.
- Do NOT change title or body rendering, and do NOT change `Publisher` in `pkg/publisher/publisher.go`.
- Do NOT recurse into nested maps or lists-of-lists (see requirement 1).
- Do NOT alter the force-set ordering: `defer_date`, then `auto_abort_prior`, then `created_by` last. An operator-supplied `created_by` must still be overridden.
- Do NOT change the `Renderer` interface signature or add a second substitution path — reuse `Renderer.Render`.
- License headers (BSD-2-Clause) on every modified `.go` file. GoDoc updates start with the item's name.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` for any business-logic error path (this prompt adds none); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg` package; the list-rendering path must be exercised.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the list arm exists and the GoDoc was updated:

```bash
cd /workspace && grep -n '\[\]interface{}' pkg/publisher/frontmatter.go
```

Must return ≥1 line.

Confirm the backwards-compatibility spec survived unedited and the new specs are present:

```bash
cd /workspace && grep -n 'passes non-string values through unchanged' pkg/publisher/frontmatter_test.go
cd /workspace && grep -n 'list entries' pkg/publisher/frontmatter_test.go
```

The first must return ≥1 line. The second must return ≥1 line describing the new list specs — do not use `current_week` as the anchor, it already appears in the pre-existing scalar specs and would pass before any change is made.

Confirm the README contract was updated:

```bash
cd /workspace && grep -n 'list-valued' README.md
```

Must return ≥1 line.

Run the publisher specs verbosely and confirm the new list specs pass alongside every pre-existing spec:

```bash
cd /workspace && go test -v ./pkg/publisher/
```

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.

Self-check before finishing: re-run the `<verification>` commands and confirm each result, then walk requirement 1 through 4 and confirm each is satisfied by the change on disk.
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

- (fill in per the reflection rules; write `- None` if nothing)
</completion>
