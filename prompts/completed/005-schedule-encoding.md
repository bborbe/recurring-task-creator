---
status: completed
spec: [001-schedule-encoding]
summary: Created pkg/schedule with 45-entry inventory, Date/RecurrenceKind/predicate types, and pure TasksForDate function; all 35 Ginkgo specs green, make precommit exits 0
container: recurring-task-creator-exec-005-schedule-encoding
dark-factory-version: v0.177.1
created: "2026-06-07T13:30:00Z"
queued: "2026-06-14T10:24:39Z"
started: "2026-06-14T10:39:45Z"
completed: "2026-06-14T10:51:56Z"
branch: dark-factory/005-schedule-encoding
---

<summary>
- Adds a single self-contained Go package that encodes the entire recurring-task inventory (45 entries) as typed data.
- Exposes one pure function that takes a calendar date and returns the deterministic set of task definitions firing that day.
- Each entry carries a stable kebab-case slug, a title template, a markdown body template, a recurrence kind, and a predicate built from a closed set of primitives.
- Slugs are frozen and globally unique; the package validates this and the placeholder set at test time.
- No clock, no network, no Kafka, no UUID, no Jira imports — the layer is pure and dependency-free except for the standard library and `github.com/bborbe/errors`.
- Ginkgo v2 / Gomega tests cover six representative dates, sort-order determinism, deep-equality determinism, slug uniqueness, placeholder validation, the frozen full slug list, and a guard against forbidden imports.
- `make precommit` passes after the change.
- This is the data-layer foundation for a later publisher spec; nothing in this spec publishes anything anywhere.
</summary>

<objective>
Create a new Go package `pkg/schedule` inside `github.com/bborbe/recurring-task-creator` that owns the full recurring-task inventory as typed data, and that exposes one pure function `TasksForDate(date Date) []TaskDefinition` returning — for any civil date — the deterministic, slug-sorted set of definitions that fire on that date. The set returned must match, slug-for-slug, what the corresponding `jira-task-creator` provider would emit on the same date.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions (Go version, lint, copyright header, Ginkgo style).

Read these files before writing code:
- `/workspace/pkg/pkg_suite_test.go` — Ginkgo suite entry point for the existing `pkg` package; mirror its style (`time.Local = time.UTC`, `format.TruncatedDiff = false`, dot-imports of ginkgo/v2 and gomega, `TestSuite(t *testing.T)`). The new `pkg/schedule` package needs its own suite file in the same style.
- `/workspace/pkg/handler/sentry-alert.go` and `/workspace/pkg/handler/sentry-alert_test.go` — representative example of the copyright header + import grouping convention used in this repo. Copyright header is exactly these three lines, with the year `2026`:
  ```
  // Copyright (c) 2026 Benjamin Borbe All rights reserved.
  // Use of this source code is governed by a BSD-style
  // license that can be found in the LICENSE file.
  ```
- `/workspace/go.mod` — confirms `github.com/bborbe/errors`, `github.com/onsi/ginkgo/v2`, `github.com/onsi/gomega` are direct deps. Do not add new module deps.
- `/workspace/Makefile` and `/workspace/Makefile.precommit` — entrypoints for `make precommit`.

Coding-guideline references (inside the YOLO container; read these before writing Go):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method pattern. For this spec the public surface is intentionally minimal: a struct type `TaskDefinition`, a function `TasksForDate`, and a small `Date` value type. No interfaces are required because there is no I/O seam to mock; the package is pure.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega style. Use `Describe` / `Context` / `It`, `BeforeEach`, `Expect(...).To(ConsistOf(...))`, `Expect(...).To(Equal(...))`. No `testing.T` assertions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — when wrapping errors use `github.com/bborbe/errors` (`errors.Wrapf(ctx, err, "...")` / `errors.Errorf(ctx, "...")`); never `fmt.Errorf`. This spec creates no production errors at runtime, but the linter expects this convention if any error path is added.

Source-of-truth context (DO NOT depend on reading these at execution time — the inventory below is exhaustive):

The current `jira-task-creator` recurring inventory lives in four files outside this repo:
`pkg/provider/trading/trading_daily-story-provider.go`, `..._monthly-story-provider.go`, `..._quartly-story-provider.go`, `..._yearly-story-provider.go`. They contain 45 non-disabled entries gated by `switch weekday`, `switch dayOfMonth`, `if month == M && dayOfMonth == D`, or "always fires" (monthly = day-of-month == 1, quarterly = first day of Jan/Apr/Jul/Oct, yearly = first day of Jan). Disabled (commented-out) entries are excluded from this spec: `CreateCheckSentryTask`, `CreateReviewTradesTask`, `CreateDarwinexInvestor`. The exact 45-entry table is inlined in `<requirements>` below.

Title placeholders in the source providers come from these formatters (verified):
- `dateToWeek(d) = fmt.Sprintf("%04dW%02d", year, week)` from `d.Time().ISOWeek()` — produces `2025W01` (uppercase `W`). This is the spec's `{{iso-week}}`.
- `dateToNextWeek(d) = dateToWeek(d + 7 days)` — same uppercase-`W` shape. Spec's `{{next-iso-week}}`.
- `dateToMonth(d) = fmt.Sprintf("%04d-%02d", year, month)` — produces `2025-04`. Spec's `{{month}}`.
- `dateToLastMonth(d)` — produces `%04d-%02d` of the previous month (wraps to December of previous year for January). Spec's `{{last-month}}`.
- `dateToQuarter(d) = fmt.Sprintf("%dQ%d", year, quarter)` where `quarter = (int(month)-1)/3 + 1` — produces `2025Q2` (uppercase `Q`). Spec's `{{quarter}}`.
- `dateToLastQuarter(d)` — produces `%04dQ%d`; Q1 wraps to previous year's Q4. Spec's `{{last-quarter}}`.
- `dateToYear(d) = fmt.Sprintf("%04d", year)`. Spec's `{{year}}`.
- `dateToLastYear(d) = dateToYear(d - 1 year)`. Spec's `{{last-year}}`.
- `d.Format(time.DateOnly)` = `2006-01-02`. Spec's `{{date}}`.

The one entry that needed runtime I/O in the original source (`CreateUpdateK3s`, which fetched the latest K3s version via `LatestVersionGetter`) ships in this spec with a static title (no version-suffix template) and a static markdown body that links to the K3s release channels. The slug is `update-k3s`. The Goal demands slug-for-slug fidelity, not title-for-title verbatim fidelity; static-title is consistent with the constraint "no I/O, no clock, no network."
</context>

<requirements>

## 1. Create package directory and suite

Create `/workspace/pkg/schedule/` with these files (all using the 2026 copyright header above):

a. `/workspace/pkg/schedule/schedule_suite_test.go` — Ginkgo suite entry for `package schedule_test`. Copy the pattern from `pkg/pkg_suite_test.go` exactly: imports `testing` and `time`, dot-imports `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega`, imports `github.com/onsi/gomega/format`. The function:
```go
func TestSuite(t *testing.T) {
    time.Local = time.UTC
    format.TruncatedDiff = false
    RegisterFailHandler(Fail)
    suiteConfig, reporterConfig := GinkgoConfiguration()
    suiteConfig.Timeout = 60 * time.Second
    RunSpecs(t, "Schedule Suite", suiteConfig, reporterConfig)
}
```
Do NOT add `//go:generate ... counterfeiter` — this package has no mocks.

## 2. Define `Date` value type

In `/workspace/pkg/schedule/date.go` (`package schedule`):

```go
// Date is a civil date (year, month, day) with no time, location, or zone
// ambiguity in its public surface. It is the only input shape accepted by
// TasksForDate.
type Date struct {
    Year  int
    Month time.Month
    Day   int
}

// NewDate constructs a Date from year/month/day.
func NewDate(year int, month time.Month, day int) Date {
    return Date{Year: year, Month: month, Day: day}
}

// IsZero reports whether the Date is the zero value (Year == 0 && Month == 0 && Day == 0).
func (d Date) IsZero() bool {
    return d.Year == 0 && d.Month == 0 && d.Day == 0
}
```

Add these internal helpers in the same file (lower-case, package-private):

```go
// toTime returns the date as midnight UTC. Used internally for Weekday and
// ISOWeek derivation. Europe/Berlin civil dates are the contract; midnight UTC
// is just the carrier for stdlib weekday/iso-week computation, which is
// timezone-agnostic for a fixed civil (Y,M,D).
func (d Date) toTime() time.Time {
    return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func (d Date) weekday() time.Weekday    { return d.toTime().Weekday() }
func (d Date) isoWeek() (year, week int) { return d.toTime().ISOWeek() }
```

Import `"time"` plainly — no alias needed. The struct field `Month time.Month` does not collide with the package name.

## 3. Define the recurrence-kind enum

In `/workspace/pkg/schedule/recurrence.go`:

```go
// RecurrenceKind classifies how often an entry repeats. Closed set.
type RecurrenceKind string

const (
    RecurrenceDaily     RecurrenceKind = "daily"
    RecurrenceWeekly    RecurrenceKind = "weekly"
    RecurrenceMonthly   RecurrenceKind = "monthly"
    RecurrenceQuarterly RecurrenceKind = "quarterly"
    RecurrenceYearly    RecurrenceKind = "yearly"
)
```

## 4. Define the predicate primitive set (closed)

In `/workspace/pkg/schedule/predicate.go`:

A predicate is a function `func(Date) bool`. Constructors are the ONLY way callers build them — there is no exported `type Predicate func(Date) bool`, just constructor functions returning that type alias, so that no caller can hand-craft a primitive outside the closed set.

```go
// predicate decides whether an inventory entry fires on a given civil date.
type predicate func(d Date) bool
```

(Lower-case `predicate` keeps the type internal; constructors return `predicate`.)

Provide EXACTLY these constructors, all exported, and NO OTHERS:

a. `OnWeekdays(days ...time.Weekday) predicate` — fires when `d.weekday()` is in the set.

b. `OnDaysOfMonth(days ...int) predicate` — fires when `d.Day` is in the set.

c. `OnMonthAndDay(month time.Month, day int) predicate` — fires when `d.Month == month && d.Day == day`.

d. `EveryDay() predicate` — always returns true.

e. `OnFirstDayOfQuarter() predicate` — fires when `d.Day == 1 && (d.Month == time.January || d.Month == time.April || d.Month == time.July || d.Month == time.October)`.

f. `OnFirstDayOfYear() predicate` — fires when `d.Month == time.January && d.Day == 1`.

g. `OnFirstDayOfMonth() predicate` — fires when `d.Day == 1`.

DO NOT add an `OnWeekdaysEveryOtherIsoWeek` constructor (or any other ISO-week-parity primitive). The closed primitive set in this spec excludes ISO-week-parity; the current inventory does not need it. If a future entry needs it, it is a follow-up spec.

DO NOT export `predicate` itself. DO NOT add `And` / `Or` / `Not` combinators — the inventory does not need them; every entry uses exactly one primitive. Combinators are a follow-up spec if ever needed.

## 5. Define `TaskDefinition` and the supported placeholder set

In `/workspace/pkg/schedule/task_definition.go`:

```go
// TaskDefinition is one entry in the recurring-task inventory.
type TaskDefinition struct {
    // Slug is a stable, kebab-case identifier unique across the inventory.
    // Once committed, a slug rename is a breaking change to the future Kafka
    // stream and requires a separate spec.
    Slug string

    // TitleTemplate is the title shown to the user. Supports only the
    // placeholders listed in SupportedPlaceholders below.
    TitleTemplate string

    // BodyTemplate is raw markdown. Supports the same placeholder set.
    BodyTemplate string

    // Recurrence classifies the cadence (daily/weekly/monthly/quarterly/yearly).
    Recurrence RecurrenceKind

    // Fires reports whether this definition fires on the given civil date.
    Fires predicate
}

// SupportedPlaceholders lists the EXACT set of placeholders accepted in
// TitleTemplate and BodyTemplate. Any other `{{...}}` token in an inventory
// entry is a build-time test failure.
var SupportedPlaceholders = []string{
    "{{date}}",          // YYYY-MM-DD
    "{{iso-week}}",      // YYYYWWW (uppercase W, matches source dateToWeek)
    "{{next-iso-week}}", // YYYYWWW for date+7d
    "{{month}}",         // YYYY-MM
    "{{last-month}}",    // YYYY-MM of previous month (wraps Jan -> prev-year Dec)
    "{{quarter}}",       // YYYYQQ (uppercase Q, matches source dateToQuarter)
    "{{last-quarter}}",  // YYYYQQ of previous quarter (wraps Q1 -> prev-year Q4)
    "{{year}}",          // YYYY
    "{{last-year}}",     // YYYY of previous year
}
```

NOTE on placeholder shape strings in comments: `YYYYWWW` and `YYYYQQ` describe the literal output shape with the uppercase letter from the source formatters. The actual rendered values are e.g. `2025W01`, `2025Q2` — uppercase `W` and `Q` are mandatory and must round-trip via the format strings `"%04dW%02d"` and `"%dQ%d"` respectively.

## 6. Implement `TasksForDate`

In `/workspace/pkg/schedule/tasks_for_date.go`:

```go
// TasksForDate returns every inventory entry whose predicate fires on d,
// sorted by Slug ascending. Pure: same input always yields the same slice
// (deep equality). No I/O, no clock, no global state.
//
// The zero Date value returns an empty slice; it never panics.
func TasksForDate(d Date) []TaskDefinition {
    if d.IsZero() {
        return []TaskDefinition{}
    }
    var out []TaskDefinition
    for _, def := range inventory {
        if def.Fires(d) {
            out = append(out, def)
        }
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
    if out == nil {
        return []TaskDefinition{}
    }
    return out
}
```

Notes:
- The `nil` slice is normalized to an empty non-nil slice so `Equal` and `ConsistOf` assertions are stable.
- Sort by `Slug` ascending — this satisfies determinism even if `inventory` is later reordered.

## 7. Encode the full 45-entry inventory

In `/workspace/pkg/schedule/inventory.go`:

```go
// inventory is the canonical, frozen recurring-task inventory. Order in this
// slice has no semantic meaning — TasksForDate sorts by Slug before returning.
var inventory = []TaskDefinition{ /* 45 entries below */ }
```

Use `var inventory = []TaskDefinition{...}` and NOT `const` — Go consts cannot be initialized from function calls, and every `Fires` field is built by a constructor call.

Encode EXACTLY these 45 entries. Each row below specifies: slug, recurrence, predicate constructor call, title template, body template (raw markdown). Use the placeholders from §5 verbatim; literal `{{` and `}}` in the strings.

For body templates, paragraphs are separated by a blank line (`\n\n`). Links are `[text](url)`. Where the source provider built multiple ADF paragraphs, flatten to one markdown paragraph per source paragraph, separated by blank lines. Lists keep their source markers (`* `, `1. `, `- `). The exact body wording does not need to match the source byte-for-byte — the test surface asserts slugs, sort order, placeholder validity, uniqueness, and the canonical slug list. Bodies are static markdown defined in code; they cross no trust boundary.

### Weekly — Saturday (12 entries, predicate `OnWeekdays(time.Saturday)`, recurrence `RecurrenceWeekly`)

| Slug | TitleTemplate | BodyTemplate (markdown, paragraph-separated) |
|------|---------------|----------------------------------------------|
| `shutdown-k3s` | `Shutdown K3s` | `Shutdown K3s cleanly so BoltDB files are not corrupt during backups.\n\n~/Documents/workspaces/scripts/remote-k3s-shutdown-nuke.sh\n\n[K3s Cluster Weekly Reboot Procedure](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FK3s%20Cluster%20Weekly%20Reboot%20Procedure)\n\n[jira-task-creator](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2Fjira-task-creator)` |
| `turn-on-hell` | `Turn on hell` | `power on hell` |
| `weekly-review` | `Weekly Review {{iso-week}}` | `Complete weekly review.\n\nIn Obsidian run (in order):\n\n1. /complete-week - Bot performance, fills weekly note\n2. /weekly-trading-review {{iso-week}} - Portfolio balances` |
| `check-ftmo-demo-accounts` | `Check FTMO Demo Accounts` | `Check if ftmo-demo account is expiring soon (e.g., this weekend).\n\nIf expiring soon, follow renewal guide:\n\n~/Documents/Obsidian/Personal/40 Trading/FTMO Demo Account Renewal Guide.md\n\nDashboards:\n\n* [FTMO](https://trader.ftmo.com/accounts-overview)\n* [Dev](https://dev.quant.benjamin-borbe.de/account/detail/ftmo-demo)` |
| `lexoffice-invoices` | `LexOffice Accounting` | `[LexOffice](https://app.lexoffice.de/fis/#)` |
| `moneymoney-review` | `Review MoneyMoney` | `Review MoneyMoney` |
| `opnsense-update` | `OPNsense Update` | `[OPNsense Firmware Updates](https://opnsense.hm.benjamin-borbe.de/ui/core/firmware#updates)` |
| `home-assistant-update-backup` | `Home Assistant Update + Backup` | `Weekly Home Assistant maintenance.\n\nSteps:\n\n1. Login\n2. Create backup\n3. Download backup\n4. Update all\n\n[Home Assistant](http://homeassistant.local:8123/config/dashboard)` |
| `renew-gmail-oauth-tokens` | `Renew Gmail OAuth Tokens` | `Renew Gmail OAuth tokens (expire every 7 days) for all environments:\n\nDev: [OAuth Init](https://dev.quant.benjamin-borbe.de/admin/core-mail-controller/oauth2/init)\n\nProd: [OAuth Init](https://prod.quant.benjamin-borbe.de/admin/core-mail-controller/oauth2/init)` |
| `plan-next-week` | `Plan Week {{next-iso-week}}` | `Create plan for week {{next-iso-week}}\n\nIn Obsidian run:\n\n/plan-week` |
| `run-update-all-saturday` | `Run update-all.sh (before restart)` | `Run system updates before weekend restart (sun.hm and fire.hm)\n\n/Users/bborbe/Documents/workspaces/scripts/update-all.sh` |
| `topic-backup-saturday` | `Backup Kafka Topics` | `Backup Kafka topics before weekend restart\n\nPrerequisite: hell must be powered on (see "Turn on hell" subtask).\n\ncd /Users/bborbe/Documents/workspaces/trading/strimzi/topic-backuper/cmd/backup\n\nmake backup` |

### Weekly — Sunday (9 entries, predicate `OnWeekdays(time.Sunday)`, recurrence `RecurrenceWeekly`)

| Slug | TitleTemplate | BodyTemplate |
|------|---------------|--------------|
| `complete-rsync-backups` | `Complete Rsync Backups` | `* check backup status\n** [Backup Status](https://backup.hell.hm.benjamin-borbe.de/status)` |
| `complete-longhorn-backups` | `Complete Longhorn Backups` | `[Longhorn Volumes](https://longhorn.quant.benjamin-borbe.de/#/volume)` |
| `turn-off-hell` | `Turn off hell` | `power off hell` |
| `turn-off-sun` | `Turn off sun` | `power off sun` |
| `turn-off-fire` | `Turn off fire` | `power off fire` |
| `docker-registry-gc` | `Docker Registry GC` | `Run garbage collection on docker registry to free storage space.\n\nkubectlquant -n docker-registry get pods\n\nkubectlquant -n docker-registry exec -it <POD_NAME> -- registry garbage-collect /etc/docker/registry/config.yml` |
| `rebuild-trading-dev-prod` | `Rebuild Trading Dev+Prod` | `Rebuild and redeploy all trading services for dev and prod.\n\nRunbook: Trading - Rebuild Dev and Prod` |
| `check-bot-is-healthy` | `Bot is Healthy` | `check bot is ready for trading on monday\n\n* kubectlquant get po --all-namespaces|grep -v Running|grep -v Complete\n* [Prometheus Alerts](https://prometheus.quant.benjamin-borbe.de/alerts)\n* [Karma Active Alerts](https://karma.quant.benjamin-borbe.de/?q=%40state%3Dactive)` |
| `run-update-all` | `Run update-all.sh` | `Run system updates across all servers\n\n/Users/bborbe/Documents/workspaces/scripts/update-all.sh` |

### Day-of-Month = 5 (1 entry, predicate `OnDaysOfMonth(5)`, recurrence `RecurrenceMonthly`)

| Slug | TitleTemplate | BodyTemplate |
|------|---------------|--------------|
| `update-finances` | `Update Finances spreadsheet` | `Fill spreadsheet at 5. of each month\n\n[Finances Spreadsheet](https://docs.google.com/spreadsheets/d/1Bmj_zmpomXJHW5sRTIcEE0xolIlGrYtO0FkY3nrxkPc/edit?usp=sharing)` |

### May 1st only (2 entries, predicate `OnMonthAndDay(time.May, 1)`, recurrence `RecurrenceYearly`)

| Slug | TitleTemplate | BodyTemplate |
|------|---------------|--------------|
| `capitalcom-apikey-prod` | `Change Capitalcom ApiKey - Prod` | `[Capital.com API Settings](https://capital.com/trading/platform/?popup=api-key-generation&tab=APISettings)` |
| `capitalcom-apikey-dev` | `Change Capitalcom ApiKey - Dev` | `[Capital.com API Settings](https://capital.com/trading/platform/?popup=api-key-generation&tab=APISettings)` |

### Monthly — day 1 of every month (17 entries, predicate `OnFirstDayOfMonth()`, recurrence `RecurrenceMonthly`)

| Slug | TitleTemplate | BodyTemplate |
|------|---------------|--------------|
| `backup-atlassian-confluence` | `Atlassian Confluence Backup` | `[Atlassian Confluence Backup](https://borbe.atlassian.net/wiki/plugins/servlet/ondemandbackupmanager/admin)\n\nSave to: smb://hell.hm.benjamin-borbe.de/bborbe/Backups/Atlassian-Cloud/` |
| `backup-atlassian-jira` | `Atlassian Jira Backup` | `[Atlassian Jira Backup](https://borbe.atlassian.net/secure/admin/CloudExport.jspa)\n\nSave to: smb://hell.hm.benjamin-borbe.de/bborbe/Backups/Atlassian-Cloud/` |
| `backup-google-drive` | `Backup Google Drive` | `[Google Drive Backup Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FGoogle%20Drive%20Backup%20Guide)\n\nRequirements:\n\n1. Hell server must be powered on\n2. Google Drive Desktop synced\n\nScript: ~/Documents/workspaces/scripts/backup-google-drive.sh` |
| `backup-pictures` | `Backup Images` | `Backup iPhone images to file server and archive to external drive.\n\n[How to Back Up iPhone](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FHow%20to%20Back%20Up%20iPhone)\n\nSteps:\n\n1. Import images via USB backup\n2. Rename and move to file server\n3. Archive to external drive using rsync\n4. Delete images from iPhone` |
| `monthly-review` | `Review Month {{last-month}}` | `Create review for month {{last-month}}\n\nIn Obsidian run:\n\n/monthly-trading-review {{last-month}}` |
| `plan-month` | `Plan Month {{month}}` | `Create plan for month {{month}}\n\nIn Obsidian run:\n\n/plan-month` |
| `trading-profits` | `Add Trading Profits to Sheet` | `[Trading Profits Sheet](https://docs.google.com/spreadsheets/d/1F6ObbGvRciK4ZdvB3BuRCf7LJFWdL-46teXvXlR0waI/edit?usp=sharing)` |
| `update-frontend` | `Update Frontend` | `Follow Frontend Update Guide in Obsidian vault.\n\nGuide location: 50 Knowledge Base/Frontend Update Guide.md\n\n[Verification](https://prod.quant.benjamin-borbe.de/chart?epic=BTCUSD&broker=capitalcom&source=standard&bidAsk=bid&resolution=1m&from=NOW-7d&until=NOW)` |
| `update-go-mod` | `Update Trading Bot Dependencies` | `Update all dependencies of all trading services.\n\nDone when: All projects updated, tests pass, changes merged to master.\n\n[Updater Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FUpdater%20Guide)` |
| `update-inventar` | `Update inventar` | `Review purchases and add new items to Obsidian inventory.\n\n[Monthly Inventory Update Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FMonthly%20Inventory%20Update%20Guide)\n\nSteps:\n\n1. Review trading business invoices in Google Drive\n2. Check for personal purchases\n3. Add items following the inventory guide\n4. Verify items appear in dashboard` |
| `update-journal` | `Update Trading Journal` | `Use /update-trading-journal in Claude Code` |
| `world-apply` | `World apply` | `[World Apply Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FWorld%20Apply%20Guide)` |
| `update-screego` | `Update Screego` | `[Screego Docker Hub](https://hub.docker.com/r/screego/server/tags)\n\ncat ~/Documents/workspaces/world/configuration/world.go|grep screego` |
| `update-poste` | `Update Poste` | `[Poste Docker Hub](https://hub.docker.com/r/analogic/poste.io/tags)\n\ncat ~/Documents/workspaces/world/configuration/world.go|grep poste\n\n# update poste version in world.go\n\nmake install\n\nworld apply -a poste -v=2` |
| `update-minio` | `Update Minio` | `helm repo add minio-operator https://operator.min.io\n\nhelm repo update\n\nhelm list -n minio-operator\n\nhelm search repo minio-operator/operator\n\nhelm list -n minio\n\nhelm search repo minio-operator/tenant\n\nOpen "backup" Intellij Project\n\nUpdate Version in k8s/minio/operator/Makefile\n\nmake upgrade\n\nUpdate Version in k8s/minio/tenant/Makefile\n\nmake upgrade` |
| `update-library` | `Update Github Libraries` | `Automated workflow (recommended):\n\n[Updater Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FUpdater%20Guide)\n\nManual workflow:\n\n[Go Library Update Workflow](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FGo%20Library%20Update%20Workflow)\n\nTask tracking:\n\n[Update All Go Libraries Task](obsidian://open?vault=Personal&file=24%20Tasks%2FUpdate%20All%20Go%20Libraries%20Following%20Workflow)` |
| `update-k3s` | `Update K3s` | `[K3s Release Channels](https://update.k3s.io/v1-release/channels)\n\n[K3s Upgrade Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FK3s%20Upgrade)\n\n* Update Hell\n* Update Nuke` |

Note on `update-k3s`: the source `CreateUpdateK3s` interpolates the latest K3s version via an HTTP lookup. This spec forbids I/O, so the static title `Update K3s` is shipped. Slug fidelity is preserved.

### Quarterly — first day of Jan/Apr/Jul/Oct (2 entries, predicate `OnFirstDayOfQuarter()`, recurrence `RecurrenceQuarterly`)

| Slug | TitleTemplate | BodyTemplate |
|------|---------------|--------------|
| `quarter-review` | `Review Quarter {{last-quarter}}` | `Create review for quarter {{last-quarter}}\n\nIn Obsidian run:\n\n/quarterly-trading-review {{last-quarter}}` |
| `quarter-plan` | `Plan Quarter {{quarter}}` | `Create plan for quarter {{quarter}}\n\nIn Obsidian run:\n\n/plan-quarter` |

### Yearly — first day of January (2 entries, predicate `OnFirstDayOfYear()`, recurrence `RecurrenceYearly`)

| Slug | TitleTemplate | BodyTemplate |
|------|---------------|--------------|
| `yearly-review` | `Review Year {{last-year}}` | `Create review for year {{last-year}}\n\nIn Obsidian run:\n\n/yearly-trading-review {{last-year}}` |
| `plan-year` | `Plan Year {{year}}` | `Create plan for year {{year}}\n\nIn Obsidian run:\n\n/plan-year` |

Total: 12 + 9 + 1 + 2 + 17 + 2 + 2 = **45 entries**.

## 8. Tests — write all of these in `/workspace/pkg/schedule/`

All test files are `package schedule_test` (external) except the slug-list canonical pin which can live in either; keep it external for consistency.

### 8a. `/workspace/pkg/schedule/predicate_test.go`

Cover each predicate constructor with table-driven `Describe("OnWeekdays", ...)` / `Describe("OnDaysOfMonth", ...)` / `Describe("OnMonthAndDay", ...)` / `Describe("EveryDay", ...)` / `Describe("OnFirstDayOfQuarter", ...)` / `Describe("OnFirstDayOfYear", ...)` / `Describe("OnFirstDayOfMonth", ...)`. For each: at least one date that fires and one that does not. Use `NewDate(2025, time.January, 4)` (Saturday, W01), `NewDate(2025, time.January, 5)` (Sunday), `NewDate(2025, time.April, 1)` (Tuesday, quarter boundary), `NewDate(2025, time.July, 1)` (quarter boundary), `NewDate(2025, time.October, 1)` (quarter boundary), `NewDate(2025, time.January, 1)` (year boundary, Wednesday), `NewDate(2025, time.March, 5)` (Wednesday, DOM=5), `NewDate(2025, time.May, 1)`.

### 8b. `/workspace/pkg/schedule/inventory_validation_test.go`

```go
var _ = Describe("inventory", func() {
    It("has unique slugs", func() {
        seen := map[string]int{}
        for _, def := range schedule.AllDefinitionsForTest() {
            seen[def.Slug]++
        }
        for slug, n := range seen {
            Expect(n).To(Equal(1), "slug %q appears %d times", slug, n)
        }
    })

    It("uses only supported placeholders in TitleTemplate and BodyTemplate", func() {
        // Compile a regex matching any `{{...}}` token in the template.
        tokenRE := regexp.MustCompile(`\{\{[^}]+\}\}`)
        supported := map[string]bool{}
        for _, p := range schedule.SupportedPlaceholders {
            supported[p] = true
        }
        for _, def := range schedule.AllDefinitionsForTest() {
            for _, tok := range tokenRE.FindAllString(def.TitleTemplate, -1) {
                Expect(supported).To(HaveKey(tok),
                    "entry %q uses unsupported placeholder %q in TitleTemplate", def.Slug, tok)
            }
            for _, tok := range tokenRE.FindAllString(def.BodyTemplate, -1) {
                Expect(supported).To(HaveKey(tok),
                    "entry %q uses unsupported placeholder %q in BodyTemplate", def.Slug, tok)
            }
        }
    })

    It("uses recurrence kinds from the closed set", func() {
        allowed := map[schedule.RecurrenceKind]bool{
            schedule.RecurrenceDaily: true, schedule.RecurrenceWeekly: true,
            schedule.RecurrenceMonthly: true, schedule.RecurrenceQuarterly: true,
            schedule.RecurrenceYearly: true,
        }
        for _, def := range schedule.AllDefinitionsForTest() {
            Expect(allowed).To(HaveKey(def.Recurrence),
                "entry %q has unknown Recurrence %q", def.Slug, def.Recurrence)
        }
    })
})
```

Add the test-only accessor in `/workspace/pkg/schedule/inventory_export_test.go` (internal package, `package schedule`):

```go
package schedule

// AllDefinitionsForTest exposes the inventory slice to external tests. The
// `_test.go` suffix keeps it out of production binaries.
func AllDefinitionsForTest() []TaskDefinition {
    out := make([]TaskDefinition, len(inventory))
    copy(out, inventory)
    return out
}
```

### 8c. `/workspace/pkg/schedule/tasks_for_date_test.go`

Cover EVERY acceptance-criterion date from the spec. Use a helper `slugs(defs []schedule.TaskDefinition) []string` to project to a slug list.

```go
var _ = Describe("TasksForDate", func() {
    It("returns the Saturday arm for 2025-01-04", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 4))
        Expect(slugs(defs)).To(ConsistOf(
            "shutdown-k3s", "turn-on-hell", "weekly-review",
            "check-ftmo-demo-accounts", "lexoffice-invoices",
            "moneymoney-review", "opnsense-update",
            "home-assistant-update-backup", "renew-gmail-oauth-tokens",
            "plan-next-week", "run-update-all-saturday", "topic-backup-saturday",
        ))
    })

    It("returns the Sunday arm for 2025-01-05", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 5))
        Expect(slugs(defs)).To(ConsistOf(
            "complete-rsync-backups", "complete-longhorn-backups",
            "turn-off-hell", "turn-off-sun", "turn-off-fire",
            "docker-registry-gc", "rebuild-trading-dev-prod",
            "check-bot-is-healthy", "run-update-all",
        ))
    })

    It("returns only update-finances for 2025-03-05 (Wednesday, DOM=5)", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.March, 5))
        Expect(slugs(defs)).To(ConsistOf("update-finances"))
    })

    It("returns monthly union + capitalcom pair for 2025-05-01 (19 slugs)", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.May, 1))
        Expect(slugs(defs)).To(ConsistOf(
            // monthly (17):
            "backup-atlassian-confluence", "backup-atlassian-jira",
            "backup-google-drive", "backup-pictures",
            "monthly-review", "plan-month", "trading-profits",
            "update-frontend", "update-go-mod", "update-inventar",
            "update-journal", "world-apply", "update-screego",
            "update-poste", "update-minio", "update-library", "update-k3s",
            // capitalcom (2):
            "capitalcom-apikey-prod", "capitalcom-apikey-dev",
        ))
        Expect(defs).To(HaveLen(19))
    })

    It("returns monthly union + quarterly for 2025-04-01", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.April, 1))
        Expect(slugs(defs)).To(ConsistOf(
            "backup-atlassian-confluence", "backup-atlassian-jira",
            "backup-google-drive", "backup-pictures",
            "monthly-review", "plan-month", "trading-profits",
            "update-frontend", "update-go-mod", "update-inventar",
            "update-journal", "world-apply", "update-screego",
            "update-poste", "update-minio", "update-library", "update-k3s",
            "quarter-review", "quarter-plan",
        ))
    })

    It("returns monthly + quarterly + yearly for 2025-01-01", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 1))
        Expect(slugs(defs)).To(ConsistOf(
            "backup-atlassian-confluence", "backup-atlassian-jira",
            "backup-google-drive", "backup-pictures",
            "monthly-review", "plan-month", "trading-profits",
            "update-frontend", "update-go-mod", "update-inventar",
            "update-journal", "world-apply", "update-screego",
            "update-poste", "update-minio", "update-library", "update-k3s",
            "quarter-review", "quarter-plan",
            "yearly-review", "plan-year",
        ))
    })

    It("returns slugs in ascending sorted order on every call", func() {
        defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 1))
        got := slugs(defs)
        sorted := append([]string(nil), got...)
        sort.Strings(sorted)
        Expect(got).To(Equal(sorted))
    })

    It("is referentially transparent (deep equality on repeated calls)", func() {
        d := schedule.NewDate(2025, time.April, 1)
        a := schedule.TasksForDate(d)
        b := schedule.TasksForDate(d)
        Expect(a).To(Equal(b))
    })

    It("returns an empty slice for the zero Date (no panic)", func() {
        defs := schedule.TasksForDate(schedule.Date{})
        Expect(defs).To(BeEmpty())
        Expect(defs).NotTo(BeNil())
    })
})
```

### 8d. `/workspace/pkg/schedule/canonical_slugs_test.go`

Pin the full alphabetically-sorted slug list. Computed list (45 slugs, sorted):

```go
var canonicalSlugs = []string{
    "backup-atlassian-confluence",
    "backup-atlassian-jira",
    "backup-google-drive",
    "backup-pictures",
    "capitalcom-apikey-dev",
    "capitalcom-apikey-prod",
    "check-bot-is-healthy",
    "check-ftmo-demo-accounts",
    "complete-longhorn-backups",
    "complete-rsync-backups",
    "docker-registry-gc",
    "home-assistant-update-backup",
    "lexoffice-invoices",
    "moneymoney-review",
    "monthly-review",
    "opnsense-update",
    "plan-month",
    "plan-next-week",
    "plan-year",
    "quarter-plan",
    "quarter-review",
    "rebuild-trading-dev-prod",
    "renew-gmail-oauth-tokens",
    "run-update-all",
    "run-update-all-saturday",
    "shutdown-k3s",
    "topic-backup-saturday",
    "trading-profits",
    "turn-off-fire",
    "turn-off-hell",
    "turn-off-sun",
    "turn-on-hell",
    "update-finances",
    "update-frontend",
    "update-go-mod",
    "update-inventar",
    "update-journal",
    "update-k3s",
    "update-library",
    "update-minio",
    "update-poste",
    "update-screego",
    "weekly-review",
    "world-apply",
    "yearly-review",
}
```

Verify the length is 45 in your editor before saving. Then:

```go
var _ = Describe("inventory canonical slug list", func() {
    It("matches the frozen sorted list of all slugs", func() {
        all := schedule.AllDefinitionsForTest()
        got := make([]string, len(all))
        for i, def := range all {
            got[i] = def.Slug
        }
        sort.Strings(got)
        Expect(got).To(Equal(canonicalSlugs))
        Expect(got).To(HaveLen(45))
    })
})
```

### 8e. `/workspace/pkg/schedule/no_forbidden_imports_test.go`

Walks `pkg/schedule/*.go` (excluding `*_test.go`) and asserts none of the forbidden import paths appear. Implement using `go/parser` and `go/ast`:

```go
var _ = Describe("package surface", func() {
    It("imports no forbidden packages", func() {
        forbidden := []string{
            `"github.com/segmentio/kafka-go"`,
            `"github.com/google/uuid"`,
            `"net/http"`,
            `"github.com/bborbe/jira-task-creator"`, // also blocks any subpackage by prefix check below
        }
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
            for _, f := range forbidden {
                Expect(strings.Contains(text, f)).To(BeFalse(),
                    "%s contains forbidden import %s", name, f)
            }
            Expect(strings.Contains(text, `"github.com/bborbe/jira-task-creator/`)).To(BeFalse(),
                "%s imports a github.com/bborbe/jira-task-creator/... subpackage", name)
        }
    })
})
```

The test runs with `pkg/schedule` as its working directory (Ginkgo default for `go test ./...`), so `os.ReadDir(".")` enumerates exactly the package files.

## 9. Imports and conventions

- Every new `.go` file starts with the 2026 copyright header (3 lines exactly as shown in `<context>`).
- Use `goimports`-style grouping: standard library first, then third-party, then internal — each group separated by a blank line. See `pkg/handler/sentry-alert_test.go` for the pattern.
- Use `github.com/bborbe/errors` for any error wrapping (`errors.Wrapf(ctx, err, "...")`, `errors.Errorf(ctx, "...")`). Do NOT use `fmt.Errorf`. (The spec is mostly error-free at runtime; this is the convention for any future addition.)
- Dot-import `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` in `*_test.go` files only.
- Do NOT add any new dep to `go.mod`. Everything needed is already a direct dep.
- Do NOT touch `pkg/handler/`, `pkg/factory/`, `pkg/mathutil/`, `main.go`, `Makefile`, or any K8s manifest.

## 10. Verify and wire-up

After all files are written:
1. Run `cd /workspace && go build ./pkg/schedule/...` — must compile.
2. Run `cd /workspace && go test ./pkg/schedule/...` — all Ginkgo specs green.
3. Run `cd /workspace && make precommit` — exits 0.

If `make precommit` flags an unused-variable, missing-license-header, or import-grouping issue, fix it locally; do NOT broaden the scope.

</requirements>

<constraints>
- Package `pkg/schedule` is self-contained: NO imports of `github.com/segmentio/kafka-go`, `github.com/google/uuid`, `net/http`, `time.Now`, `github.com/bborbe/jira-task-creator/...`, or any package that touches a clock or network. (Standard-library `time` is allowed for `time.Weekday`, `time.Month`, ISO-week computation. `os` and `strings` are allowed in the forbidden-imports test only.)
- `TasksForDate` is referentially transparent: same input date always returns the same slice (deep equality, including slug order).
- Date input is a civil date (year, month, day). The public `Date` type has no `time.Time` field, no `time.Location` field — there is no zone ambiguity in the public signature.
- Slugs once written in this spec are frozen — a slug rename is a breaking change to the future Kafka stream and requires its own spec.
- Tests follow Ginkgo v2 / Gomega style. No `*testing.T` assertions inside `It` blocks.
- Do NOT add per-task opt-out flags, runtime feature toggles, configurability of placeholders, configurability of timezone, or any mechanism to disable individual inventory entries — non-goals from the spec.
- Do NOT render templates inside `TasksForDate` — the function returns `TaskDefinition` with placeholders intact. Rendering is a follow-up spec.
- Do NOT add Markdown-to-ADF / HTML / any-other-format conversion — body templates ship as raw markdown.
- Do NOT carry over disabled tasks (`CreateCheckSentryTask`, `CreateReviewTradesTask`, `CreateDarwinexInvestor`).
- Do NOT touch or delete the existing skeleton packages `pkg/factory`, `pkg/handler`, `pkg/mathutil`, or anything in `main.go` — deletion belongs to the publisher spec, not this one.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- Use `var` (not `const`) for the inventory slice — Go consts cannot be initialized from function calls (`OnWeekdays(...)` etc.).
- Do NOT introduce `fmt.Errorf` anywhere.
</constraints>

<verification>
Run these from the repo root `/workspace`:

1. `make precommit` — must exit 0.
2. `go test ./pkg/schedule/...` — all specs green.
3. `grep -E '"(github\.com/segmentio/kafka-go|github\.com/google/uuid|net/http|github\.com/bborbe/jira-task-creator)"' pkg/schedule/*.go` — must return no matches (excluding `*_test.go` files; the forbidden-imports Ginkgo test verifies this for production files).
4. `ls pkg/schedule/` — must list at minimum: `date.go`, `recurrence.go`, `predicate.go`, `task_definition.go`, `tasks_for_date.go`, `inventory.go`, `inventory_export_test.go`, `schedule_suite_test.go`, `predicate_test.go`, `inventory_validation_test.go`, `tasks_for_date_test.go`, `canonical_slugs_test.go`, `no_forbidden_imports_test.go`.
5. Spot-check: open `pkg/schedule/inventory.go` and visually confirm 45 entries (the canonical-slugs test in §8d is the load-bearing assertion of 45).
</verification>
