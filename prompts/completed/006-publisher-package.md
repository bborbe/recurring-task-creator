---
status: completed
spec: [002-publisher]
summary: Added pkg/publisher package with deterministic UUID5 task identifier, frozen frontmatter shape, and 9-placeholder template rendering; wired CreatePublisher in pkg/factory; all 29 Ginkgo specs pass with 100% statement coverage; make precommit exits 0
container: recurring-task-creator-mvp-exec-006-publisher-package
dark-factory-version: v0.177.1
created: "2026-06-14T11:40:25Z"
queued: "2026-06-14T11:52:36Z"
started: "2026-06-14T11:52:37Z"
completed: "2026-06-14T12:05:12Z"
branch: dark-factory/publisher
---

<summary>
- Adds a new `pkg/publisher` Go package that converts one `(schedule.TaskDefinition, schedule.Date)` pair into one validated `task.CreateCommand` and sends it through an injected `task.CreateCommandSender`.
- The package identifier is the deterministic `UUID5(namespace, "recurring-<slug>-<YYYY-MM-DD>")` — same input on a second tick always produces a byte-identical `TaskIdentifier`, which is the contract the controller's de-dup relies on.
- Renders the nine placeholders frozen in Spec 1 (`{{date}}`, `{{iso-week}}`, `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`, `{{last-quarter}}`, `{{year}}`, `{{last-year}}`) using the exact `YYYYWWW` (uppercase `W`) and `YYYYQQ` (uppercase `Q`) shapes the source providers emitted; substitution is one `strings.ReplaceAll` per token in `SupportedPlaceholders` order — no regex, no template engine.
- Stamps every command's frontmatter with the fixed migration shape (`assignee: bborbe`, `status: in_progress`, `page_type: task`, `goals: ["[[Example Goal]]"]`, `priority: 2`, `recurring: <kind>`); no other keys, no override knob, no per-task opt-out.
- The `pkg/factory` factory gains `CreatePublisher(task.CreateCommandSender) publisher.Publisher` — zero-logic composition over `NewPublisher(sender)`; the wire-side `task.NewCreateCommandSender` call lives in `main.go` (Spec 3 wires it from `libkafka.NewSyncProducerWithName`).
- A pre-generated counterfeiter mock for `task.CreateCommandSender` is reused from `github.com/bborbe/agent/lib/command/task/mocks` (no new mock file needed in this repo).
- `make precommit` exits 0 after the change; no new HTTP, no cron, no K8s surface, no per-task toggles.
</summary>

<objective>
Add a `pkg/publisher` package to `github.com/bborbe/recurring-task-creator` that owns the "definition + date → validated `task.CreateCommand` + send" transformation, with a deterministic UUID5 identifier and a frozen frontmatter shape. The factory layer in `pkg/factory` gains a `CreatePublisher(sender task.CreateCommandSender) publisher.Publisher` factory. The publisher must remain pure (no I/O, no clock, no env reads) and depend on no network, HTTP, Kafka, or jira-task-creator surface.
</objective>

<context>

Read `/workspace/CLAUDE.md` for project conventions (Go 1.26, BSD license header year `2026`, `make precommit`, Ginkgo v2 / Gomega, Counterfeiter v6).

Read these source files fully before writing code:
- `/workspace/pkg/schedule/date.go` — `Date{Year int, Month time.Month, Day int}`, `NewDate(...)`, `IsZero()`, private `toTime()`, `weekday()`. The publisher will need to convert a `Date` to a `time.Time` for `ISOWeek()` and `AddDate(0, -1, 0)`-style arithmetic — use the same midnight-UTC carrier pattern (`time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)`).
- `/workspace/pkg/schedule/task_definition.go` — `TaskDefinition{Slug, TitleTemplate, BodyTemplate, Recurrence, Fires}` and the exported `SupportedPlaceholders` slice. The publisher iterates this slice in order; it does NOT call `TasksForDate`.
- `/workspace/pkg/schedule/recurrence.go` — `RecurrenceKind` is a string alias. The string value (`"daily" | "weekly" | "monthly" | "quarterly" | "yearly"`) goes into `Frontmatter["recurring"]`.
- `/workspace/pkg/schedule/inventory.go` — read for format-string shape evidence (uppercase `W` and `Q` in the source-provider format strings). Do NOT import it; the publisher only imports `pkg/schedule` for types.
- `/workspace/pkg/schedule/no_forbidden_imports_test.go` — Ginkgo walk-and-grep pattern. The publisher's forbidden-imports test reuses the same shape.
- `/workspace/pkg/factory/factory.go` — current `CreateTestLoglevelHandler` / `CreateSentryAlertHandler`; append a new `CreatePublisher` factory.
- `/workspace/pkg/handler/sentry-alert.go` and `/workspace/pkg/handler/test-loglevel.go` — copyright header (3 lines, year `2026`) and import-grouping convention.
- `/workspace/mocks/mocks.go` — package marker file at the project root `mocks/` dir; no new local mocks needed (the `CreateCommandSender` mock lives in the upstream module — see below).
- `/workspace/CHANGELOG.md` — append a `feat:` bullet under `## Unreleased` for this package.
- `/workspace/go.mod` — direct deps currently include `github.com/bborbe/errors`, `github.com/onsi/ginkgo/v2`, `github.com/onsi/gomega`. This spec adds direct deps on `github.com/bborbe/agent/lib` (for `lib.TaskIdentifier` and `lib.TaskFrontmatter`), `github.com/bborbe/agent/lib/command/task` (for `task.CreateCommand`, `task.CreateCommandSender`, `task.NewCreateCommandSender`), and `github.com/google/uuid` (for `uuid.NewSHA1`). All three resolve via `go get` as of 2026-06-13 (verified during prompt generation).

Verified external symbols (grep'd at `/home/node/go/pkg/mod/` on 2026-06-14):

`github.com/bborbe/agent/lib/command/task` package surface (read `/home/node/go/pkg/mod/github.com/bborbe/agent/lib@v0.65.0/command/task/`):
```go
// create-command.go
const CreateCommandOperation base.CommandOperation = "create-task"

type CreateCommand struct {
    TaskIdentifier lib.TaskIdentifier  `json:"taskIdentifier"`
    Title          string              `json:"title"`
    Frontmatter    lib.TaskFrontmatter `json:"frontmatter"`
    Body           string              `json:"body,omitempty"`
}
func (cmd CreateCommand) Validate(ctx context.Context) error

// create-command-sender.go
type CreateCommandSender interface {
    SendCommand(ctx context.Context, cmd CreateCommand) error
}
func NewCreateCommandSender(commandObjectSender cdb.CommandObjectSender) CreateCommandSender
```

`github.com/bborbe/agent/lib` types (read `/home/node/go/pkg/mod/github.com/bborbe/agent/lib@v0.65.0/`):
```go
// agent_task-identifier.go
type TaskIdentifier string
func (t TaskIdentifier) String() string
func (t TaskIdentifier) Validate(ctx context.Context) error

// agent_task-frontmatter.go
type TaskFrontmatter map[string]interface{}
// (no constructor — build via map literal or `make(lib.TaskFrontmatter, 6)`)
```

`github.com/google/uuid` (read `/home/node/go/pkg/mod/github.com/google/uuid@v1.6.0/hash.go`):
```go
// hash.go line 57
func NewSHA1(space UUID, data []byte) UUID
// uuid.go — `type UUID [16]byte`; no NameSpaceDNS/etc. is required because the publisher
// uses its own package-level namespace constant (see §3).
```

`github.com/bborbe/errors` (read `/home/node/go/pkg/mod/github.com/bborbe/errors@v1.5.13/`):
```go
func Wrap(ctx context.Context, err error, message string) error
func Wrapf(ctx context.Context, err error, format string, args ...interface{}) error
func New(ctx context.Context, message string) error
func Errorf(ctx context.Context, format string, args ...interface{}) error
```

Pre-existing counterfeiter mock for `task.CreateCommandSender` (read `/home/node/go/pkg/mod/github.com/bborbe/agent/lib@v0.65.0/command/task/mocks/task-create-command-sender.go`):
```go
package mocks // imported as taskmocks "github.com/bborbe/agent/lib/command/task/mocks"

type TaskCreateCommandSender struct {
    SendCommandStub func(context.Context, task.CreateCommand) error
    // ...standard counterfeiter fields and methods:
    // SendCommandCallCount() int
    // SendCommandArgsForCall(i int) (context.Context, task.CreateCommand)
    // SendCommandReturns(err error)
    // SendCommandReturnsOnCall(i int, err error)
}
```
**This mock is reused as-is; do NOT generate a new one inside this repo.**

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method. The publisher defines a `Publisher` interface with a counterfeiter directive, a `NewPublisher(sender task.CreateCommandSender) Publisher` constructor returning the interface, and a private `publisher` struct. Also read §"private-struct-matches-interface" and §"constructor-returns-interface".
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `Create*` prefix in `pkg/factory`, zero business logic, returns interface type. `CreatePublisher` is a one-line pass-through; `task.NewCreateCommandSender(cdb.NewCommandObjectSender(...))` construction belongs in `main.go Run` (Spec 3), not in the factory.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter directive on every exported interface, `--fake-name` and output filename prefixed with the source package, `//go:generate go run -mod=mod github.com/maxbrunsfeld/counterfeiter/v6 -generate` in `<package>_suite_test.go`. The publisher mocks the upstream `task.CreateCommandSender` via the pre-generated `taskmocks.TaskCreateCommandSender`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — wrap with `github.com/bborbe/errors` (`errors.Wrapf(ctx, err, "... slug=%s date=%s", ...)`); never `fmt.Errorf`; never `context.Background()` in business logic.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, dot-imports, `BeforeEach`, `Expect`, `ConsistOf`, `Equal`. External test package (`package publisher_test`).
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage ≥80% for new code; counterfeiter mocks (reused upstream here); no real network / Kafka in unit tests.

</context>

<requirements>

## 1. Module additions

`cd /workspace && go get github.com/bborbe/agent/lib github.com/google/uuid` (run on the host before writing the package; if already present, the `go.mod` is unchanged). All three modules resolve to versions confirmed in `<context>` above.

## 2. Create the package directory and suite

Create `/workspace/pkg/publisher/` with these files. All new `.go` files start with the 2026 copyright header (3 lines, year `2026`):

```
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
```

a. `/workspace/pkg/publisher/publisher_suite_test.go` — Ginkgo suite entry for `package publisher_test`. Mirror `/workspace/pkg/schedule/schedule_suite_test.go` exactly:

```go
package publisher_test

import (
    "testing"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/onsi/gomega/format"
)

//go:generate go run -mod=mod github.com/maxbrunsfeld/counterfeiter/v6 -generate
func TestSuite(t *testing.T) {
    time.Local = time.UTC
    format.TruncatedDiff = false
    RegisterFailHandler(Fail)
    suiteConfig, reporterConfig := GinkgoConfiguration()
    suiteConfig.Timeout = 60 * time.Second
    RunSpecs(t, "Publisher Suite", suiteConfig, reporterConfig)
}
```

The `//go:generate` line is included so a future `go generate` run on this package can refresh the locally-generated mock for the publisher's own `Publisher` interface (counterfeiter will see the directive in `publisher.go` and regenerate into `../mocks/publisher-publisher.go`). It is a no-op when no such directive exists yet.

## 3. The UUID5 namespace constant

In `/workspace/pkg/publisher/uuid_namespace.go` (this is the only file declaring the constant; do not split it):

```go
package publisher

import "github.com/google/uuid"

// uuidNamespace is the UUID5 namespace used to derive TaskIdentifier values
// for recurring tasks. It is FROZEN — never read from env or flag, never
// regenerated, never changed. Changing it is a breaking change to the entire
// downstream Kafka stream (every identifier collides; the controller will
// create a duplicate vault file for every recurring task on the next tick).
//
// If a future spec needs a new namespace, define a new constant alongside
// this one with a distinct name and do not edit this one.
var uuidNamespace = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")
```

The literal `f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a` is the namespace UUID this prompt pins. It MUST be byte-identical across the whole codebase. The single line in this file is the only definition; do not duplicate it elsewhere. `uuid.MustParse` is a stdlib helper from `github.com/google/uuid` that panics on parse failure — invalid here is a build-time impossibility, so `MustParse` is appropriate.

## 4. The Publisher interface and implementation

In `/workspace/pkg/publisher/publisher.go`:

```go
package publisher

import (
    "context"
    "strings"

    "github.com/bborbe/errors"
    lib "github.com/bborbe/agent/lib"
    "github.com/bborbe/agent/lib/command/task"
    "github.com/google/uuid"

    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// goalsLink is the wiki link placed in Frontmatter["goals"] for every
// recurring task created by this service. Frozen; do not move or rename
// without a new spec.
const goalsLink = "[[Example Goal]]"

// Publisher turns one (TaskDefinition, Date) pair into a validated
// task.CreateCommand and sends it via the injected task.CreateCommandSender.
//counterfeiter:generate -o ../mocks/publisher-publisher.go --fake-name PublisherPublisher . Publisher
type Publisher interface {
    // Publish builds a CreateCommand for (def, date) and sends it. The
    // returned error is wrapped with the slug and ISO date in its message.
    // Same (def, date) on a second call produces a byte-identical command.
    Publish(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) error
}

// NewPublisher returns a Publisher that sends through sender. The sender
// is invoked exactly once per Publish call (when inputs are valid). It
// validates the constructed command internally — see
// task.CreateCommandSender.SendCommand in github.com/bborbe/agent/lib/command/task.
func NewPublisher(sender task.CreateCommandSender) Publisher {
    return &publisher{sender: sender}
}

type publisher struct {
    sender task.CreateCommandSender
}

func (p *publisher) Publish(
    ctx context.Context,
    def schedule.TaskDefinition,
    date schedule.Date,
) error {
    if def.Slug == "" {
        return errors.Errorf(ctx, "publish failed: empty slug")
    }
    if date.IsZero() {
        return errors.Errorf(ctx, "publish failed: zero date for slug %q", def.Slug)
    }
    cmd := task.CreateCommand{
        TaskIdentifier: buildTaskIdentifier(def.Slug, date),
        Title:          renderTemplate(def.TitleTemplate, def.Slug, date),
        Frontmatter:    buildFrontmatter(def.Recurrence),
        Body:           renderTemplate(def.BodyTemplate, def.Slug, date),
    }
    if err := p.sender.SendCommand(ctx, cmd); err != nil {
        return errors.Wrapf(
            ctx,
            err,
            "publish failed: send CreateCommand for slug %q on %04d-%02d-%02d",
            def.Slug, date.Year, date.Month, date.Day,
        )
    }
    return nil
}
```

Notes on the above:
- `def.Fires` is intentionally NOT read — the publisher takes one definition, not a date predicate check. The schedule's `TasksForDate` is the caller's job (Spec 3).
- The `EmptySlug` / `ZeroDate` errors are returned BEFORE the sender is touched; counterfeiter `SendCommandCallCount()` must be 0 in those paths.
- `Frontmatter` is constructed via `buildFrontmatter` (§5). `Body` may be empty; the empty string is valid for `task.CreateCommand.Body` and the upstream `validateCreateBody` accepts it (only enforces a 500 KiB upper bound).

## 5. The `buildTaskIdentifier` helper (UUID5 derivation)

In `/workspace/pkg/publisher/uuid_namespace.go` (append, do not create a second file), add:

```go
// buildTaskIdentifier returns the deterministic TaskIdentifier for the
// (slug, date) pair. The identifier is UUID5(uuidNamespace, "recurring-<slug>-<YYYY-MM-DD>").
// Same input on a second call produces the same identifier across processes,
// redeploys, and replays — this is the contract the controller's de-dup
// relies on.
func buildTaskIdentifier(slug string, date schedule.Date) lib.TaskIdentifier {
    name := "recurring-" + slug + "-" + isoDate(date)
    return lib.TaskIdentifier(uuid.NewSHA1(uuidNamespace, []byte(name)).String())
}

func isoDate(date schedule.Date) string {
    return fmtDate(date.Year, int(date.Month), date.Day)
}
```

Place the helpers in the same file as the namespace constant. Use `fmt.Sprintf` with the format string `"%04d-%02d-%02d"` to produce `YYYY-MM-DD`. Import `fmt` in that file.

## 6. The `renderTemplate` helper (placeholder substitution)

In `/workspace/pkg/publisher/render.go`:

```go
package publisher

import (
    "fmt"
    "strings"
    "time"

    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// renderTemplate replaces every placeholder in template with its rendered
// value for date. Substitutes the exact slice returned by
// schedule.SupportedPlaceholders — one strings.ReplaceAll per token, in
// slice order. No regex, no template engine. Unknown placeholders cannot
// appear at this layer (Spec 1's inventory validation rejects them at
// test time).
func renderTemplate(template, slug string, date schedule.Date) string {
    values := buildPlaceholderValues(slug, date)
    out := template
    for _, ph := range schedule.SupportedPlaceholders {
        out = strings.ReplaceAll(out, ph, values[ph])
    }
    return out
}

// buildPlaceholderValues returns a map from each supported placeholder to
// its rendered string for date. The map covers every entry in
// schedule.SupportedPlaceholders.
func buildPlaceholderValues(slug string, date schedule.Date) map[string]string {
    base := dateToTime(date)
    isoYear, isoWeek := base.ISOWeek()
    next := base.AddDate(0, 0, 7)
    nextIsoYear, nextIsoWeek := next.ISOWeek()
    lastMonth := firstOfPreviousMonth(base)
    lastQuarterYear, lastQuarter := previousQuarter(base.Year(), int(base.Month()))
    return map[string]string{
        "{{date}}":          fmtDate(date.Year, int(date.Month), date.Day),
        "{{iso-week}}":      fmtIsoWeek(isoYear, isoWeek),
        "{{next-iso-week}}": fmtIsoWeek(nextIsoYear, nextIsoWeek),
        "{{month}}":         fmtMonthYear(base.Year(), int(base.Month())),
        "{{last-month}}":    fmtMonthYear(lastMonth.Year(), int(lastMonth.Month())),
        "{{quarter}}":       fmtQuarter(base.Year(), quarterOf(base.Month())),
        "{{last-quarter}}":  fmtQuarter(lastQuarterYear, lastQuarter),
        "{{year}}":          fmtYear(base.Year()),
        "{{last-year}}":     fmtYear(base.Year() - 1),
    }
}
```

(`dateToTime` is defined as a shim at the bottom of this file — see end of section.)

The exact format strings the helpers use (the spec is byte-strict on these — uppercase `W` and `Q` are mandatory and match the source providers' output shape):

```go
// fmtDate renders YYYY-MM-DD.
func fmtDate(year, month, day int) string {
    return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// fmtIsoWeek renders YYYYWNN (uppercase W, two-digit week with leading zero).
// Matches the source provider's dateToWeek format ("%04dW%02d").
func fmtIsoWeek(year, week int) string {
    return fmt.Sprintf("%04dW%02d", year, week)
}

// fmtMonthYear renders YYYY-MM.
func fmtMonthYear(year, month int) string {
    return fmt.Sprintf("%04d-%02d", year, month)
}

// fmtQuarter renders YYYYQNN (uppercase Q, two-digit quarter with leading zero).
// Matches the source provider's dateToQuarter format ("%dQ%d", upgraded to
// zero-padded width — see spec rationale: the spec's expected output is
// "2025Q02" and "2025Q01" with a two-digit quarter; the source's "%dQ%d"
// renders "2025Q2" and "2025Q1" — the spec wins because the spec's
// acceptance criteria are the contract, not the source).
func fmtQuarter(year, quarter int) string {
    return fmt.Sprintf("%04dQ%02d", year, quarter)
}

// fmtYear renders YYYY.
func fmtYear(year int) string {
    return fmt.Sprintf("%04d", year)
}

// quarterOf returns 1..4 for the given month.
func quarterOf(m time.Month) int {
    return (int(m)-1)/3 + 1
}

// firstOfPreviousMonth returns the first day of the calendar month before base.
func firstOfPreviousMonth(base time.Time) time.Time {
    y, m, _ := base.Date()
    return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
}

// previousQuarter returns (year, quarter) for the quarter before (year, month).
// Q1 of any year rolls back to Q4 of the previous year.
func previousQuarter(year, month int) (int, int) {
    q := (month-1)/3 + 1
    if q == 1 {
        return year - 1, 4
    }
    return year, q - 1
}
```

Important: `Date` is the public type from `pkg/schedule`; do NOT alias it. The `d.toTime()` method is unexported, so `renderTemplate` lives in a different package and cannot call it directly. Add this shim at the bottom of `render.go` (replacing the `date.toTime()` call in `buildPlaceholderValues` shown above):

```go
// dateToTime exposes schedule.Date's midnight-UTC carrier through a
// publisher-local helper so the publisher can run ISOWeek() and
// AddDate(0, 0, 7) without re-implementing the conversion. The
// midnight-UTC choice is timezone-agnostic for a fixed civil (Y,M,D) —
// see pkg/schedule/date.go.
func dateToTime(d schedule.Date) time.Time {
    return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}
```

**Critical: ISO-week year.** `time.Time.ISOWeek()` returns `(isoYear, isoWeek)`. The ISO year DIFFERS from the calendar year at year boundaries (e.g. `2024-12-30` belongs to ISO `2025W01`). The snippet above uses both return values via `isoYear, isoWeek := base.ISOWeek()`. Do NOT substitute `base.Year()` for the year — that ships a silent bug at year boundaries.

## 7. The `buildFrontmatter` helper

In `/workspace/pkg/publisher/frontmatter.go`:

```go
package publisher

import (
    lib "github.com/bborbe/agent/lib"

    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// buildFrontmatter returns the exact frontmatter shape for every recurring
// task. The shape is FROZEN: changing any of these keys or values is a
// breaking change to the migration's vault-file layout.
func buildFrontmatter(recurrence schedule.RecurrenceKind) lib.TaskFrontmatter {
    return lib.TaskFrontmatter{
        "assignee":  "bborbe",
        "status":    "in_progress",
        "page_type": "task",
        "goals":     []interface{}{goalsLink},
        "priority":  2,
        "recurring": string(recurrence),
    }
}
```

`lib.TaskFrontmatter` is `map[string]interface{}` (verified); `[]interface{}{...}` is the correct literal form for a JSON-array-typed value. `priority` is `2` (int), not `float64` — when the frontmatter is JSON-serialized by `cdb.CommandObjectSender`, the value is `2` (an integer). `recurring` is the string value of `RecurrenceKind` (`"daily" | "weekly" | "monthly" | "quarterly" | "yearly"`).

## 8. The forbidden-imports guard

In `/workspace/pkg/publisher/no_forbidden_imports_test.go`:

```go
package publisher_test

import (
    "os"
    "strings"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("package surface", func() {
    It("imports no forbidden packages", func() {
        forbidden := []string{
            `"net/http"`,
            `"github.com/segmentio/kafka-go"`,
            `"github.com/IBM/sarama"`,
            `"github.com/bborbe/jira-task-creator"`,
        }
        // also block any time.Now read
        forbiddenText := append(forbidden, "time.Now()")
        entries, err := os.ReadDir(".")
        Expect(err).NotTo(HaveOccurred())
        for _, e := range entries {
            name := e.Name()
            if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
                continue
            }
            data, err := os.ReadFile(name)
            Expect(err).NotTo(HaveOccurred(), name)
            text := string(data)
            for _, f := range forbiddenText {
                Expect(strings.Contains(text, f)).To(BeFalse(),
                    "%s contains forbidden token %s", name, f)
            }
            Expect(strings.Contains(text, `"github.com/bborbe/jira-task-creator/`)).
                To(BeFalse(), "%s imports a github.com/bborbe/jira-task-creator/... subpackage", name)
        }
    })
})
```

`google/uuid` is NOT in the forbidden list here — the publisher's identifier contract requires it.

## 9. The publisher unit tests

In `/workspace/pkg/publisher/publisher_test.go` (Ginkgo v2 / Gomega; external `package publisher_test`). Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega`.

Imports required:
```go
import (
    "context"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/google/uuid"

    "github.com/bborbe/errors"
    taskmocks "github.com/bborbe/agent/lib/command/task/mocks"

    "github.com/bborbe/recurring-task-creator/pkg/publisher"
    "github.com/bborbe/recurring-task-creator/pkg/schedule"
)
```

Test surface (one `Describe("Publisher", ...)` block, one `BeforeEach` that constructs `sender = &taskmocks.TaskCreateCommandSender{}` and `pub = publisher.NewPublisher(sender)`):

a. **Identifier is the UUID5 of the canonical key.** For `def.Slug = "weekly-review"`, `date = schedule.NewDate(2025, time.January, 4)`, capture the `task.CreateCommand` passed to `sender.SendCommand` (via `sender.SendCommandArgsForCall(0)`) and assert:
```go
expected := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte("recurring-weekly-review-2025-01-04")).String()
Expect(string(captured.TaskIdentifier)).To(Equal(expected))
```
The test must read `uuidNamespace` from the package's own surface. Add a small test-only accessor in `export_test.go` (the special-cased Go filename — only compiled for tests in this package; keeps the symbol invisible to production binaries) with `package publisher`:
```go
package publisher

import "github.com/google/uuid"

// UuidNamespaceForTest exposes the frozen UUID5 namespace to external
// tests so they can compute the expected identifier offline.
func UuidNamespaceForTest() uuid.UUID { return uuidNamespace }
```

b. **Two calls with the same (def, date) produce deep-equal commands.** Call `Publish` twice, capture both `CreateCommand` values from the two `SendCommand` calls, `Expect(cmd1).To(Equal(cmd2))`.

c. **Placeholder rendering — one `It` per case.** All assertions operate on `cmd.Title` for entries whose `TitleTemplate` contains the relevant placeholder:
- `{{date}}` for `2025-01-04` → `2025-01-04` in the rendered title.
- `{{iso-week}}` for `2025-01-04` → `2025W01` (uppercase `W`, two-digit week).
- `{{next-iso-week}}` for `2025-01-04` → `2025W02` (week of date+7d).
- `{{month}}` for `2025-01-04` → `2025-01`; `{{last-month}}` for `2025-01-04` → `2024-12` (year roll-back).
- `{{quarter}}` for `2025-04-01` → `2025Q02`; `{{last-quarter}}` for `2025-01-01` → `2024Q04` (year roll-back).
- `{{year}}` for `2025-04-01` → `2025`; `{{last-year}}` for `2025-01-01` → `2024`.

For each test, use a one-off `schedule.TaskDefinition{Slug: "test-slug", TitleTemplate: "prefix {{date}} suffix", Recurrence: schedule.RecurrenceWeekly}`-style literal. Use a `BeforeEach` that sets `sender.SendCommandReturns(nil)` so the send is a no-op; capture the command via `sender.SendCommandArgsForCall(0)`.

d. **Body placeholder substitution.** Build a def with `BodyTemplate: "body contains {{date}}"`, call `Publish` for `2025-01-04`, assert `cmd.Body == "body contains 2025-01-04"`.

e. **Frontmatter shape — full set.** Build a def with `Recurrence: schedule.RecurrenceWeekly`, call `Publish` for any valid date, assert on `cmd.Frontmatter`:
```go
Expect(cmd.Frontmatter).To(HaveKeyWithValue("assignee", "bborbe"))
Expect(cmd.Frontmatter).To(HaveKeyWithValue("status", "in_progress"))
Expect(cmd.Frontmatter).To(HaveKeyWithValue("page_type", "task"))
Expect(cmd.Frontmatter).To(HaveKeyWithValue("priority", 2))
Expect(cmd.Frontmatter).To(HaveKeyWithValue("recurring", "weekly"))
Expect(cmd.Frontmatter).To(HaveKeyWithValue("goals", []interface{}{"[[Example Goal]]"}))
Expect(cmd.Frontmatter).To(HaveLen(6))
```

f. **Recurrence table.** A `DescribeTable` over the five `RecurrenceKind` values that asserts `cmd.Frontmatter["recurring"]` matches the string value. The body's `Recurrence: entry` is the table entry.

g. **Sender is called exactly once per valid Publish call.** After one `Publish`, `Expect(sender.SendCommandCallCount()).To(Equal(1))`.

h. **Sender is NOT called on empty slug.** `Publish(ctx, schedule.TaskDefinition{Slug: ""}, validDate)` returns a non-nil error; the error chain contains `"empty slug"`; `sender.SendCommandCallCount()` is 0.

i. **Sender is NOT called on zero date.** `Publish(ctx, validDef, schedule.Date{})` returns a non-nil error; the error chain contains `"zero date"`; `sender.SendCommandCallCount()` is 0.

j. **Sender error is wrapped with slug and ISO date.** Set `sender.SendCommandReturns(errors.Errorf(ctx, "broker down"))`, call `Publish(ctx, def{Slug: "weekly-review"}, NewDate(2025, time.January, 4))`, assert:
```go
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("weekly-review"))
Expect(err.Error()).To(ContainSubstring("2025-01-04"))
```

k. **No side effect on error path.** For each error path in (h), (i), (j), the call count after the failed `Publish` is exactly 1 (for j) or 0 (for h/i).

l. **Boundary contract — produced command passes `task.CreateCommand.Validate`.** The publisher's whole purpose is producing a `CreateCommand` that the downstream sender chain (and ultimately the task-controller's JSON unmarshal + validation) will accept. The mock sender is a no-op and bypasses `Validate`. Add a Ginkgo `It`/`DescribeTable` that, for each of the five `RecurrenceKind` values, captures the constructed command via `sender.SendCommandArgsForCall(0)` and calls `Expect(captured.Validate(ctx)).To(Succeed())` directly. Catches level-1 boundary regressions (oversized body, control-character title, reserved Windows name in title) that unit-mocking would otherwise hide. Imports add `task "github.com/bborbe/agent/lib/command/task"` for the typed receiver.

m. **ISO-week year boundary** — covers the year-boundary bug class. For `date = schedule.NewDate(2024, time.December, 30)` (Monday belonging to ISO `2025W01`), build a def with `TitleTemplate: "{{iso-week}}"`, capture the command, assert `cmd.Title == "2025W01"` (ISO year `2025`, NOT calendar year `2024`). Without this case the year-bug ships silently — see `pkg/publisher/render.go` `buildPlaceholderValues` `ISOWeek()` return discipline.

## 10. Factory wiring

In `/workspace/pkg/factory/factory.go`, append (preserve existing two factories, do not delete them — `pkg/handler` and `pkg/mathutil` are out of scope):

```go
import (
    // existing imports...
    "github.com/bborbe/agent/lib/command/task"

    "github.com/bborbe/recurring-task-creator/pkg/publisher"
)

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. Pure plumbing: no business logic.
func CreatePublisher(sender task.CreateCommandSender) publisher.Publisher {
    return publisher.NewPublisher(sender)
}
```

Do NOT call `task.NewCreateCommandSender` or `cdb.NewCommandObjectSender` from the factory. The `libkafka.SyncProducer` → `cdb.CommandObjectSender` → `task.CreateCommandSender` chain lives in `main.go Run` (Spec 3). Factories never return `error`, never contain conditionals, never schedule cleanup.

## 11. The factory test (one minimal compile-and-wire check)

In `/workspace/pkg/factory/factory_test.go` (`package factory_test`):

```go
var _ = Describe("CreatePublisher", func() {
    var (
        sender *taskmocks.TaskCreateCommandSender
        pub    publisher.Publisher
    )
    BeforeEach(func() {
        sender = &taskmocks.TaskCreateCommandSender{}
        sender.SendCommandReturns(nil)
        pub = factory.CreatePublisher(sender)
    })
    It("returns a Publisher that delegates to the sender", func() {
        def := schedule.TaskDefinition{Slug: "weekly-review", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly}
        Expect(pub.Publish(context.Background(), def, schedule.NewDate(2025, time.January, 4))).To(Succeed())
        Expect(sender.SendCommandCallCount()).To(Equal(1))
    })
})
```

Imports required: `context`, `time`, `factory "github.com/bborbe/recurring-task-creator/pkg/factory"`, `publisher "github.com/bborbe/recurring-task-creator/pkg/publisher"`, `schedule "github.com/bborbe/recurring-task-creator/pkg/schedule"`, `taskmocks "github.com/bborbe/agent/lib/command/task/mocks"`, dot-imports of ginkgo/v2 and gomega. Update `pkg/factory/factory_suite_test.go` to include the `//go:generate go run -mod=mod github.com/maxbrunsfeld/counterfeiter/v6 -generate` directive (it is already present in the existing suite file).

## 12. Changelog entry

Append to `/workspace/CHANGELOG.md` under `## Unreleased` (the section already exists from Spec 1):

```markdown
- feat: Add `pkg/publisher` package that builds a deterministic `task.CreateCommand` from `(schedule.TaskDefinition, schedule.Date)` and sends it via an injected `task.CreateCommandSender`; identifier is UUID5 of `"recurring-<slug>-<YYYY-MM-DD>"`; frontmatter is frozen at `assignee/status/page_type/priority/goals/recurring`
```

The bullet must use the `feat:` prefix (minor bump per `changelog-guide.md`).

## 13. Imports and conventions

- Every new `.go` file starts with the 2026 copyright header.
- Use `goimports`-style grouping: standard library first, then third-party (in alphabetical order: `github.com/bborbe/...`, `github.com/google/...`), then internal (`github.com/bborbe/recurring-task-creator/...`).
- Use `github.com/bborbe/errors` for any error wrapping. Do NOT use `fmt.Errorf` (the linter will reject it).
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Do NOT touch `main.go`, `Makefile`, `Makefile.precommit`, `Makefile.variables`, `k8s/`, or any file under `pkg/handler/` or `pkg/mathutil/`.
- Do NOT add a new Prometheus metric, opt-out flag, or runtime config knob. The spec's Non-goals forbid them.

## 14. Verification

After all files are written, run these from the repo root `/workspace`:

1. `cd /workspace && go build ./...` — must compile.
2. `cd /workspace && go test ./pkg/publisher/... ./pkg/factory/...` — all Ginkgo specs green.
3. `cd /workspace && make precommit` — exits 0.
4. `cd /workspace && grep -nE '^var.*uuid\.UUID' pkg/publisher/*.go` — exactly one match (`uuidNamespace` in `uuid_namespace.go`); the line above is the doc comment marking it frozen.
5. `cd /workspace && grep -nE '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/publisher/*.go` — must return no matches.
6. `cd /workspace && grep -n 'TasksForDate' pkg/publisher/*.go` — must return no matches (publisher never walks the inventory).
7. `cd /workspace && grep -nE 'func Create.*Publisher' pkg/factory/*.go` — must return one match (`CreatePublisher`).

If any check fails, fix the underlying code; do NOT silence the test.

</requirements>

<constraints>
- The package MUST NOT import `net/http`, `github.com/segmentio/kafka-go`, `github.com/IBM/sarama`, or any `github.com/bborbe/jira-task-creator/...` package.
- The package MUST NOT call `time.Now()`, MUST NOT read env, MUST NOT open a network connection, MUST NOT walk the inventory (`TasksForDate` is in `pkg/schedule` and the publisher must not call it).
- The UUID5 namespace (`uuidNamespace` in `pkg/publisher/uuid_namespace.go`) is a single package-level `uuid.UUID` constant, frozen. It MUST NOT be configurable, env-read, or duplicated.
- Placeholder substitution is one `strings.ReplaceAll` per entry of `schedule.SupportedPlaceholders` in slice order. No regex. No template engine. Unknown placeholders cannot reach this layer (Spec 1 enforces that).
- The Publisher interface and the `NewPublisher` constructor follow the project pattern: `Publisher` (interface) → `NewPublisher(sender) Publisher` → `publisher` (private struct). The `NewPublisher` returns the interface type, not the concrete struct.
- The `CreatePublisher` factory in `pkg/factory/factory.go` is one line: `return publisher.NewPublisher(sender)`. Zero business logic, no `error` return, no cleanup closure.
- Errors are wrapped with `github.com/bborbe/errors` (`Wrapf` for sender errors, `Errorf` for validation errors). The slug and ISO date appear in the wrapped message so Spec 3 cron logs are actionable.
- Tests use Ginkgo v2 / Gomega; mocks are the pre-generated `taskmocks.TaskCreateCommandSender` from `github.com/bborbe/agent/lib/command/task/mocks`. No hand-written mocks.
- The publisher follows the `definition-of-done.md` rule: ≥80% statement coverage on new code. Cover every error path, the determinism contract, every placeholder rendering, and the frontmatter shape.
- Do NOT add a Prometheus metric, an opt-out flag, a runtime config knob, or any per-task disable mechanism. Spec Non-goals forbid all of these.
- Do NOT add `//go:generate counterfeiter ...` (uses globally installed binary). Use `//counterfeiter:generate` on the interface and `//go:generate go run -mod=mod github.com/maxbrunsfeld/counterfeiter/v6 -generate` in the suite file.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>

From `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/publisher/... ./pkg/factory/...` — all Ginkgo specs green.
3. `grep -E '^var.*uuid\.UUID' pkg/publisher/uuid_namespace.go` — exactly one match.
4. `grep -nE '"(net/http|github\.com/segmentio/kafka-go|github\.com/IBM/sarama|github\.com/bborbe/jira-task-creator)"|time\.Now\(\)' pkg/publisher/*.go` — no matches.
5. `grep -n 'TasksForDate' pkg/publisher/*.go` — no matches.
6. `grep -nE 'func Create.*Publisher' pkg/factory/factory.go` — exactly one match.
7. `grep -nE '^(type|func New)' pkg/publisher/*.go` — exactly one `Publisher` interface, one `NewPublisher` constructor, one `publisher` struct, one `Publish` method (matching the spec AC).
8. Spot-check: open `pkg/publisher/uuid_namespace.go` and confirm the namespace UUID is `f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a` and the comment marks it frozen.

Expected final `make precommit` output: exit code 0, all tests green, lint clean, license headers present, no forbidden imports.

## Open Questions (for the human reviewer)

- **Reference factory pattern not verified.** The spec says wiring follows `~/Documents/workspaces/maintainer/watcher/github-build/pkg/factory/factory.go`. That path does not exist on this YOLO container; the prompt instead mirrors the local `pkg/factory/factory.go` (`CreateTestLoglevelHandler` / `CreateSentryAlertHandler` → `CreatePublisher`). The watcher reference would only be a one-line pass-through factory (factory-pattern §4.1), which the local mirror captures.
- **Upstream mock location.** The pre-generated `taskmocks.TaskCreateCommandSender` is imported from `github.com/bborbe/agent/lib/command/task/mocks`. If a future bump of `bborbe/agent/lib` ever moves that file, the test imports break — `go get` will surface the failure.
- **Source-vs-spec `Q` and `W` width.** The source providers' `dateToQuarter` uses `%dQ%d` (no zero-pad). The spec's AC pins `2025Q02` (two-digit, zero-padded). The publisher follows the spec; this is documented inline in the `fmtQuarter` doc-comment.
- **No scenario file.** The spec explicitly says NO scenario is needed. Unit + integration surface is fully reachable from a counterfeiter `CreateCommandSender`.

</verification>
