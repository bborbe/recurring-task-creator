---
status: completed
spec: [017-hourly-recurrence-kind]
summary: Threaded the Europe/Berlin civil hour through the hourly tick (pkg/tick stamps schedule.Date.Hour), updated the two Berlin conversion specs with explicit hours, added the hour-threading spec, and appended the CHANGELOG entry; make test and make precommit both pass
execution_id: recurring-task-creator-hourly-exec-036-spec-017-tick-hour
dark-factory-version: dev
created: "2026-09-07T21:06:00Z"
queued: "2026-09-07T21:31:23Z"
started: "2026-09-07T21:37:10Z"
completed: "2026-09-07T21:41:57Z"
branch: dark-factory/hourly-recurrence-kind
---

<summary>
- The hourly tick now reads the Europe/Berlin civil hour from the same Berlin-adjusted clock read it already takes and stamps it onto the civil date it passes to the publisher.
- With the injected clock fixed at an instant whose Berlin civil hour is 14, the date captured by the fake publisher carries hour 14.
- The two existing Berlin date-conversion specs are updated so their date assertions include the expected hour, keeping the whole tree green.
- No change to the tick cadence, the one-pass-per-tick loop, metrics, log lines, or any existing kind's behavior.
</summary>

<objective>
Thread the Europe/Berlin civil hour through the hourly tick so the date handed to `publisher.Publish` carries `Hour`, enabling the publisher's hour-granular period token (prompt 3) to produce a distinct task file per civil hour.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions first.

This is prompt 2 of 4 for spec 017. It DEPENDS ON prompt 1 having landed: `schedule.Date.Hour` must exist. Guard: if `grep -qn 'Hour int' /workspace/pkg/schedule/date.go` returns no match (exit non-zero), prompt 1 has NOT landed — STOP and report `status: failed` with summary "schedule core (prompt 1) not yet deployed — Date.Hour undefined".

Read these files fully before changing anything:
- `/workspace/pkg/tick/tick.go` — the `tick` struct (with the `berlin *time.Location` field loaded in `NewTick` via `time.LoadLocation("Europe/Berlin")`) and the `tick(ctx)` method. The method already reads `now := t.clock.Now().Time().In(t.berlin)`, then `year, month, day := now.Date()` and `date := schedule.NewDate(year, month, day)`. You add one line populating `date.Hour`.
- `/workspace/pkg/tick/tick_test.go` — `package tick_test`. Note the `libtime.CurrentDateTime` + `clock.SetNow(libtimetest.ParseDateTime("..."))` injection pattern, the `pubmocks.PublisherPublisher` fake (`.PublishReturns(nil)`, `pub.PublishArgsForCall(0)` returns `(ctx, def, date)`), and the two existing `Europe/Berlin date conversion` specs that assert `gotDate).To(Equal(schedule.NewDate(...)))` — these assertions must be updated to carry the hour. `libtime` is `github.com/bborbe/time`, `libtimetest` is `github.com/bborbe/time/test`.
- `/workspace/pkg/schedule/date.go` — the `Date` struct with the new `Hour int` field (prompt 1) and `NewDate(year, month, day)`.

Coding guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — injected clock, `SetNow` in tests.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, counterfeiter fakes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format.
</context>

<requirements>

### 1. Populate `Date.Hour` in the tick

In `/workspace/pkg/tick/tick.go`, in the `tick(ctx)` method, populate the civil hour on the date right after the date is constructed. The `now` value is already the Europe/Berlin civil time (`t.clock.Now().Time().In(t.berlin)`), so `now.Hour()` is the Europe/Berlin civil hour (0-23):

```go
	now := t.clock.Now().Time().In(t.berlin)
	t.metrics.SetLastTickTimestamp(float64(now.Unix()))
	year, month, day := now.Date()
	date := schedule.NewDate(year, month, day)
	date.Hour = now.Hour()
```

`date.Hour` is the exported field added by prompt 1. Do NOT change anything else in `tick()` — the tick cadence, the one-pass-per-tick publish loop, the store-error handling, the per-task error isolation, the metrics calls, and the log message formats all stay exactly as they are. `now.Hour()` does not read the system clock — it derives from the already-injected `t.clock`.

### 2. Update the two existing Berlin date-conversion assertions

In `/workspace/pkg/tick/tick_test.go`, the two specs under `Describe("Europe/Berlin date conversion")` assert the captured date with `Expect(gotDate).To(Equal(schedule.NewDate(...)))`. Now that the tick stamps `Hour`, these assertions must carry the expected hour or they FAIL:

- The winter spec (`clock.SetNow(libtimetest.ParseDateTime("2025-01-04T23:30:00Z"))`): Berlin is `2025-01-05 00:30` (CET = UTC+1), so `Hour=0`. `schedule.NewDate(2025, time.January, 5)` already equals `Date{..., Hour: 0}`, so this assertion happens to keep passing — but make the hour explicit for clarity:
  `Expect(gotDate).To(Equal(schedule.Date{Year: 2025, Month: time.January, Day: 5, Hour: 0}))`.
- The summer spec (`clock.SetNow(libtimetest.ParseDateTime("2025-07-15T23:30:00Z"))`): Berlin is `2025-07-16 01:30` (CEST = UTC+2), so `Hour=1`. `schedule.NewDate(2025, time.July, 16)` produces `Hour=0`, so the current assertion FAILS once the tick stamps `Hour=1` — update it to:
  `Expect(gotDate).To(Equal(schedule.Date{Year: 2025, Month: time.July, Day: 16, Hour: 1}))`.

### 3. Add the AC 5 spec — tick threads the Berlin civil hour

In `/workspace/pkg/tick/tick_test.go`, add a spec under `Describe("Europe/Berlin date conversion")` proving the hour is threaded. Mirror the existing spec structure (set the clock, construct the tick, run in a goroutine, wait for the publish call, read `PublishArgsForCall(0)`):

```go
	It("threads the Europe/Berlin civil hour onto the date handed to the publisher", func() {
		// 2025-01-04T13:30:00Z is 2025-01-04 14:30 in Berlin (CET = UTC+1), Hour=14.
		clock.SetNow(libtimetest.ParseDateTime("2025-01-04T13:30:00Z"))
		var err error
		tk, err = tick.NewTick(context.Background(), fakeStore, pub, clock, metrics)
		Expect(err).NotTo(HaveOccurred())

		want := expectedCount(schedule.Date{Year: 2025, Month: time.January, Day: 4, Hour: 14})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			_ = tk.Run(ctx)
			close(done)
		}()

		Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
			Should(Equal(want))

		_, _, gotDate := pub.PublishArgsForCall(0)
		Expect(gotDate).To(Equal(schedule.Date{Year: 2025, Month: time.January, Day: 4, Hour: 14}))

		cancel()
		Eventually(done, "200ms", "5ms").Should(BeClosed())
	})
```

Note: `expectedCount` takes a `schedule.Date` and calls `schedule.TasksForDate(testDefs, date)` — the `Hour` field is ignored by the filter (always-fire, prompt 1), so passing the hour-bearing date is fine; the count is the same as for the same civil day. Keep every other existing spec unchanged and passing.

### 4. Changelog

In `/workspace/CHANGELOG.md`, append to the existing `## Unreleased` section (created by prompt 1 — if it is somehow absent, create it) one entry:

```
- feat: thread the Europe/Berlin civil hour through the hourly tick — `pkg/tick` stamps `schedule.Date.Hour` from the same Berlin-adjusted clock read it already takes, so the publisher can build hour-granular period tokens
```

</requirements>

<constraints>
- The tick-loop cadence (one fire per elapsed hour, one pass per fire) is UNCHANGED — you add one field-population line, nothing else. No new timers, no new loops, no new network calls.
- `now.Hour()` derives from the injected `t.clock` — the tick must NOT call `time.Now()` anywhere (project rule: no `time.Now()` in business logic).
- Do NOT change the log message formats in `tick()` (`%04d-%02d-%02d` date-only lines stay as-is), the store-error handling, the per-task error isolation, or the metrics calls.
- Do NOT touch `pkg/schedule`, `pkg/publisher`, or `pkg/handler` — this prompt is tick-only. In particular, do NOT change the `/trigger?date=` handler (a manual trigger of an `hourly` entry deliberately uses Hour=0 — documented limitation, spec Non-goal).
- Do NOT add any config knob, env var, tunable threshold, or opt-out flag (spec Non-goals).
- License headers (BSD-2-Clause) on every modified `.go` file.
- Project DoD applies (`/workspace/docs/dod.md`): Ginkgo v2 / Gomega; `bborbe/errors` 3-arg wrapping on any business-logic error path (this prompt adds none); no `fmt.Errorf`; no `context.Background()` in business logic (test code is exempt).
- Coverage ≥80% for the changed `pkg/tick` package; the hour-threading path must be exercised by the new spec.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- `make precommit` exits 0 from the repo root.
</constraints>

<verification>
Run from `/workspace`:

```bash
cd /workspace && make test
```

Confirm the hour population is present:

```bash
cd /workspace && grep -n 'date.Hour = now.Hour()' pkg/tick/tick.go
```

Run the tick specs verbosely and confirm the new hour-threading spec (Hour=14) and the two updated Berlin conversion specs pass:

```bash
cd /workspace && go test -v ./pkg/tick/
```

Finally:

```bash
cd /workspace && make precommit
```

Must exit 0. If `make precommit` exits non-zero, report `status: failed` with the exit code — do not rationalize a failure as success.
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
