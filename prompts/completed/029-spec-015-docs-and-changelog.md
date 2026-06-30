---
status: completed
spec: [015-auto-abort-prior-field]
summary: 'Docs closeout for spec 015 — added CHANGELOG ## Unreleased entry and architecture.md note for spec.autoAbortPrior/auto_abort_prior; license-header/GoDoc sweep passes; make precommit exits 0.'
execution_id: recurring-task-controller-auto-abort-exec-029-spec-015-docs-and-changelog
dark-factory-version: dev
created: "2026-06-30T18:00:00Z"
queued: "2026-06-30T17:48:54Z"
started: "2026-06-30T19:40:58Z"
completed: "2026-06-30T19:45:34Z"
branch: dark-factory/auto-abort-prior-field
---

<summary>
- The changelog records the new opt-in `spec.autoAbortPrior` Schedule field and the `auto_abort_prior` frontmatter stamp.
- The architecture doc's CR shape and publisher-frontmatter description mention the new field so the next maintainer sees it.
- A final sweep confirms every touched Go file has its license header and that the new exported field carries GoDoc.
- No behavior change here — this is the documentation closeout for the feature.
- Depends on the code prompts (1–3) having landed.
</summary>

<objective>
Close out spec 015 with documentation: a `CHANGELOG.md` entry describing the `spec.autoAbortPrior` field and the `auto_abort_prior` frontmatter stamp, an architecture-doc note for the new field, and a sweep confirming license headers and GoDoc on the touched files.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 4 of 4 for spec 015 — the docs/changelog closeout. It DEPENDS ON prompts 1–3 having landed. Guard: if `grep -qn 'auto_abort_prior' /workspace/pkg/publisher/frontmatter.go` returns no match (exit non-zero), the publisher stamp (prompt 3) has not landed — STOP and report `status: failed` with summary "publisher stamp (prompt 3) not yet deployed".

Read these files fully before changing anything:
- `/workspace/CHANGELOG.md` — note the top title block and that there is currently NO `## Unreleased` section (the newest released section is `## v0.6.1`). You create `## Unreleased` immediately below the title preamble, above `## v0.6.1`.
- `/workspace/docs/architecture.md` — note the `pkg/publisher` row (around line 34) describing frontmatter building, and any CR-shape YAML snippet showing `spec.schedule` fields. Find the publisher frontmatter description and the `created_by` / defaults mention.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format: `- <prefix>: <what> [context]`; one bullet per logical change; name the field/key.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-licensing-guide.md` — BSD-2-Clause header on every `.go` file.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc on exported symbols.
</context>

<requirements>

### 1. Add the CHANGELOG entry

In `/workspace/CHANGELOG.md`, create a `## Unreleased` section immediately below the title preamble (the lines ending at "...backwards-compatible bug fixes.") and ABOVE `## v0.6.1`. If `## Unreleased` already exists, append to it instead of creating a duplicate. Add this bullet:

```
- feat: add opt-in `spec.autoAbortPrior` boolean to the `Schedule` CRD (default `false`, optional) and stamp `auto_abort_prior: <bool>` onto every materialized task's frontmatter, mirroring the field. The stamp is set after operator keys and before the force-set `created_by` provenance key. New Schedules are safe by default — no prior instance is auto-aborted unless an operator explicitly sets `autoAbortPrior: true`. The downstream controller-side gate flip ships separately.
```

Use the `feat:` prefix (this is a new backward-compatible field → minor bump). Do NOT copy verification bash comments or the prompt filename into the changelog.

### 2. Update the architecture doc

In `/workspace/docs/architecture.md`:

- In the `pkg/publisher` row's frontmatter description (around line 34), extend the frontmatter sentence to note the new stamp, e.g. add: "Force-sets `auto_abort_prior: <bool>` (mirrored from the Schedule's `spec.autoAbortPrior`) after operator keys and before `created_by`."
- If the doc contains a `Schedule` CR YAML snippet showing `spec.schedule` fields (`recurrence`, `weekday`, `periodOffset`), add an `autoAbortPrior` line with a brief comment, e.g.:
  ```yaml
  autoAbortPrior: false  # optional; opt-in (default false) — prior instance may be auto-aborted by the controller
  ```
- If the doc has prose describing the frontmatter provenance/`created_by` ordering, add a sentence that `auto_abort_prior` is stamped after operator keys and before `created_by`.
- Keep the rest of the doc untouched. If `docs/architecture.md` has no `spec.schedule` YAML snippet and no frontmatter-ordering prose, only the `pkg/publisher` row sentence needs updating — do not invent new sections.

### 3. License-header + GoDoc sweep

Confirm every `.go` file touched across prompts 1–3 carries the BSD-2-Clause header and that the new exported field has GoDoc. This is a verification sweep, not new code — if `make precommit`'s `addlicense` target or the linter flags a missing header or missing GoDoc, fix it. Files in scope:
- `k8s/apis/task.benjamin-borbe.de/v1/types.go`
- `k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go`
- `k8s/client/applyconfiguration/task.benjamin-borbe.de/v1/scheduletrigger.go`
- `pkg/k8s_connector_schema.go`
- `pkg/schedule/task_definition.go`
- `pkg/store/adapter.go`
- `pkg/publisher/frontmatter.go`
- `pkg/publisher/publisher.go`

Run `grep -L "Use of this source code is governed by a BSD-style" <each file>` to find any missing header; add the standard 3-line BSD-2-Clause header (matching the existing files in this repo) to any file that lacks it. (The apply-config file uses an Apache header from k8s codegen — leave that file's existing header as-is; it is generated boilerplate and not subject to the repo's BSD header rule.)

</requirements>

<constraints>
- This prompt makes NO functional code change — it is docs + changelog + a header/GoDoc sweep only.
- The CHANGELOG entry uses the `feat:` prefix and names the `spec.autoAbortPrior` field and the `auto_abort_prior` frontmatter key.
- Do NOT introduce a config knob, env var, or tunable threshold anywhere. (Spec Non-goals.)
- Do NOT modify the apply-config file's existing Apache generated header.
- License headers (BSD-2-Clause) on every repo-owned `.go` file; GoDoc on the new exported field.
- Project DoD applies (`/workspace/docs/dod.md`).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && grep -n 'autoAbortPrior\|auto_abort_prior' CHANGELOG.md
cd /workspace && grep -n 'auto_abort_prior\|autoAbortPrior' docs/architecture.md
```

Both must return ≥1 line.

Confirm headers present on the repo-owned touched files:

```bash
cd /workspace && grep -L "Use of this source code is governed by a BSD-style" \
  k8s/apis/task.benjamin-borbe.de/v1/types.go \
  k8s/apis/task.benjamin-borbe.de/v1/zz_generated.deepcopy.go \
  pkg/k8s_connector_schema.go \
  pkg/schedule/task_definition.go \
  pkg/store/adapter.go \
  pkg/publisher/frontmatter.go \
  pkg/publisher/publisher.go
```

Must print nothing (every listed file has the header).

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize.
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
