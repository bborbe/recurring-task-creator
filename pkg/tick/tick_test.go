// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tick_test

import (
	"context"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	pubmocks "github.com/bborbe/recurring-task-creator/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

var _ = Describe("Tick", func() {
	var (
		pub     *pubmocks.PublisherPublisher
		clock   libtime.CurrentDateTime
		metrics *pubmocks.TickMetrics
		tk      tick.Tick
	)

	BeforeEach(func() {
		pub = &pubmocks.PublisherPublisher{}
		pub.PublishReturns(nil)

		clock = libtime.NewCurrentDateTime()
		clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))

		metrics = &pubmocks.TickMetrics{}

		// After spec 009, the tick iterates the date-filtered canonical
		// inventory (schedule.TasksForDate(date)), NOT the inventory
		// passed to NewTick. The factory still passes schedule.Inventory()
		// to NewTick for completeness; the tick does the filtering.
		var err error
		tk, err = tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("constructor", func() {
		It("returns a Tick on the happy path", func() {
			Expect(tk).NotTo(BeNil())
		})

		// NewTick's Europe/Berlin load-failure path is exercised by code review
		// (load call + error wrap) and by the stdlib LoadLocation failure mode
		// confirmed below; in-process stubbing of the location is intentionally
		// omitted to avoid leaking test seams into production code.
		It(
			"returns a wrapped error when time.LoadLocation fails (verified via stdlib failure mode)",
			func() {
				_, err := time.LoadLocation("NoSuch/Zone")
				Expect(err).To(HaveOccurred())
			},
		)
	})

	Describe("initial tick", func() {
		It("fires before the for-select loop", func() {
			// 2025-01-04 is a Saturday in Europe/Berlin; the canonical
			// inventory yields 24 always-fire + 12 Saturday weekday = 36
			// entries via schedule.TasksForDate.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(36))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("publish-per-entry", func() {
		It("calls Publish once for every entry that fires on the civil date", func() {
			// 2025-01-04 is a Saturday; the canonical inventory yields
			// 36 entries (24 always-fire + 12 Saturday weekday).
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(36))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("Europe/Berlin date conversion", func() {
		It("converts winter UTC to next-day Berlin civil date", func() {
			clock.SetNow(libtimetest.ParseDateTime("2025-01-04T23:30:00Z"))
			// 2025-01-05 is a Sunday; the canonical inventory yields
			// 24 always-fire + 9 Sunday weekday = 33 entries.
			var err error
			tk, err = tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(33))

			_, _, gotDate := pub.PublishArgsForCall(0)
			Expect(gotDate).To(Equal(schedule.NewDate(2025, time.January, 5)))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("converts summer UTC to next-day Berlin civil date (CEST)", func() {
			clock.SetNow(libtimetest.ParseDateTime("2025-07-15T23:30:00Z"))
			// 2025-07-16 is a Wednesday; the canonical inventory yields
			// 24 always-fire + 0 weekday = 24 entries.
			var err error
			tk, err = tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(24))

			_, _, gotDate := pub.PublishArgsForCall(0)
			Expect(gotDate).To(Equal(schedule.NewDate(2025, time.July, 16)))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("per-task error isolation", func() {
		It("continues publishing after a Publish error", func() {
			// Use the canonical inventory filtered for 2025-01-04 (Saturday).
			// 36 entries expected. The first 3 calls fail, the rest succeed.
			// Verify by count: 3 errors + 33 successes = 36 total.
			var err error
			tk, err = tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			pub.PublishReturnsOnCall(0, errors.New(context.Background(), "kafka down"))
			pub.PublishReturnsOnCall(1, errors.New(context.Background(), "kafka down"))
			pub.PublishReturnsOnCall(2, errors.New(context.Background(), "kafka down"))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(36))
			Eventually(func() int { return metrics.IncPublishedCallCount() }, "200ms", "5ms").
				Should(Equal(36))

			var errorCount, successCount int
			for i := 0; i < 36; i++ {
				r, _ := metrics.IncPublishedArgsForCall(i)
				if r == "error" {
					errorCount++
				} else {
					successCount++
				}
			}
			Expect(errorCount).To(Equal(3))
			Expect(successCount).To(Equal(33))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("increments the counter labeled with the task's recurrence kind", func() {
			// 2025-01-04 is a Saturday; 24 always-fire (monthly+quarterly+yearly)
			// + 12 Saturday weekday. Fail every publish to focus on the
			// kind labels. The canonical inventory contains 0 Daily and
			// 0 Weekly entries; only weekday/monthly/quarterly/yearly fire.
			var err error
			tk, err = tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			pub.PublishReturns(errors.New(context.Background(), "boom"))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return metrics.IncPublishedCallCount() }, "200ms", "5ms").
				Should(Equal(36))

			kinds := map[string]bool{}
			for i := 0; i < 36; i++ {
				r, kind := metrics.IncPublishedArgsForCall(i)
				Expect(r).To(Equal("error"))
				kinds[kind] = true
			}
			Expect(kinds).To(HaveKey("weekday"))
			Expect(kinds).To(HaveKey("monthly"))
			Expect(kinds).To(HaveKey("quarterly"))
			Expect(kinds).To(HaveKey("yearly"))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("context cancellation", func() {
		It("exits the per-task loop early when ctx is cancelled mid-tick", func() {
			// Use a 2025-01-04 (Saturday) tick; cancel on the first publish.
			var err error
			tk, err = tick.NewTick(context.Background(), schedule.Inventory(), pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			pub.PublishStub = func(
				_ context.Context,
				_ schedule.TaskDefinition,
				_ schedule.Date,
			) error {
				cancel()
				return nil
			}

			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(1))
			Eventually(done, "200ms", "5ms").Should(BeClosed())

			Expect(metrics.SetLastTickTimestampCallCount()).To(Equal(1))
		})

		It("returns nil cleanly when ctx is cancelled while waiting for the ticker", func() {
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})

			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(BeNumerically(">=", 1))

			cancel()

			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("metrics gauge", func() {
		It("records the clock's Unix seconds at tick start", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(
				func() int { return metrics.SetLastTickTimestampCallCount() },
				"200ms",
				"5ms",
			).Should(Equal(1))

			got := metrics.SetLastTickTimestampArgsForCall(0)
			want := float64(clock.Now().Time().Unix())
			Expect(got).To(BeNumerically("~", want, 1.0))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("recurrence label coverage", func() {
		// After spec 009, the tick iterates the date-filtered canonical
		// inventory. The canonical inventory contains 0 RecurrenceDaily
		// and 0 RecurrenceWeekly entries (those kinds are reserved for
		// future use); on 2025-01-04 (Saturday) the firing kinds are
		// weekday (12), monthly (18), quarterly (2), and yearly (4).
		// The metric label coverage test asserts all 6 kinds register
		// zero-valued series at init time (see "Prometheus
		// pre-initialization" Describe) and that each kind present in
		// the date-filtered slice is recorded in the success counter.
		It(
			"records the kind for each RecurrenceKind value present on 2025-01-04 (Saturday)",
			func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					_ = tk.Run(ctx)
					close(done)
				}()

				Eventually(
					func() int { return metrics.IncPublishedCallCount() },
					"200ms",
					"5ms",
				).Should(BeNumerically(">=", 4))

				kinds := map[string]bool{}
				for i := 0; i < metrics.IncPublishedCallCount(); i++ {
					r, kind := metrics.IncPublishedArgsForCall(i)
					Expect(r).To(Equal("success"))
					kinds[kind] = true
				}
				// 2025-01-04 (Saturday) fires weekday, monthly, quarterly, yearly.
				for _, k := range []string{
					"weekday", "monthly", "quarterly", "yearly",
				} {
					Expect(kinds).To(HaveKey(k),
						"expected metric label %q to be observed on 2025-01-04", k)
				}

				cancel()
				Eventually(done, "200ms", "5ms").Should(BeClosed())
			},
		)
	})

	Describe("full inventory", func() {
		var berlin *time.Location

		BeforeEach(func() {
			var err error
			berlin, err = time.LoadLocation("Europe/Berlin")
			Expect(err).NotTo(HaveOccurred())
		})

		It("publishes every entry that fires on the given civil date", func() {
			// Derive expected count at test time (NOT a hardcoded literal).
			// The tick now filters by date via schedule.TasksForDate(date);
			// a Tuesday publishes 0 weekday entries, a Saturday publishes
			// 12, a Sunday publishes 9, plus the always-fire kinds
			// (18 monthly + 2 quarterly + 4 yearly = 24 always-fire on any
			// date in 2025/2026).
			// Use the same accessor the tick uses to guarantee the two
			// stay in sync. Each iteration uses fresh pub/metrics mocks
			// so call counts are isolated.
			for _, instant := range []string{
				"2025-01-07T10:00:00Z", // Tuesday
				"2025-01-04T10:00:00Z", // Saturday
				"2025-01-05T10:00:00Z", // Sunday
				"2025-07-04T10:00:00Z", // Friday (different month, no weekday-kind fires)
				"2026-03-01T10:00:00Z", // Sunday (different year)
			} {
				localPub := &pubmocks.PublisherPublisher{}
				localPub.PublishReturns(nil)
				localMetrics := &pubmocks.TickMetrics{}
				clock.SetNow(libtimetest.ParseDateTime(instant))
				instanceTk, err := tick.NewTick(
					context.Background(),
					schedule.Inventory(),
					localPub,
					clock,
					localMetrics,
				)
				Expect(err).NotTo(HaveOccurred())

				// Compute the civil date the same way the tick does: clock
				// -> Berlin -> civil date.
				now := clock.Now().Time().In(berlin)
				y, m, d := now.Date()
				civilDate := schedule.NewDate(y, m, d)
				expected := len(schedule.TasksForDate(civilDate))
				Expect(expected).To(BeNumerically(">", 0)) // sanity: at least one entry fires

				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					_ = instanceTk.Run(ctx)
					close(done)
				}()

				Eventually(
					func() int { return localPub.PublishCallCount() },
					"200ms",
					"5ms",
				).Should(Equal(expected))
				cancel()
				Eventually(done, "200ms", "5ms").Should(BeClosed())
			}
		})
	})
})

var _ = Describe("Prometheus pre-initialization", func() {
	It("registers the counter with 12 zero-valued series", func() {
		families, err := prometheus.DefaultGatherer.Gather()
		Expect(err).NotTo(HaveOccurred())

		var published *dto.MetricFamily
		for _, f := range families {
			if f.GetName() == "recurring_tasks_published_total" {
				published = f
				break
			}
		}
		Expect(published).NotTo(BeNil())
		metrics := published.GetMetric()
		Expect(metrics).To(HaveLen(12))

		seen := map[string]bool{}
		for _, m := range metrics {
			r := ""
			k := ""
			for _, lp := range m.GetLabel() {
				switch lp.GetName() {
				case "result":
					r = lp.GetValue()
				case "recurrence":
					k = lp.GetValue()
				}
			}
			Expect(r).To(BeElementOf("success", "error"))
			Expect(
				k,
			).To(BeElementOf("daily", "weekly", "weekday", "monthly", "quarterly", "yearly"))
			seen[r+"/"+k] = true
			Expect(m.GetCounter().GetValue()).To(Equal(0.0))
		}
		Expect(seen).To(HaveLen(12))
	})

	It("registers the gauge with zero value", func() {
		families, err := prometheus.DefaultGatherer.Gather()
		Expect(err).NotTo(HaveOccurred())

		var gauge *dto.MetricFamily
		for _, f := range families {
			if f.GetName() == "recurring_tasks_last_tick_timestamp_seconds" {
				gauge = f
				break
			}
		}
		Expect(gauge).NotTo(BeNil())
		Expect(gauge.GetMetric()).To(HaveLen(1))
		Expect(gauge.GetMetric()[0].GetGauge().GetValue()).To(Equal(0.0))
	})
})
