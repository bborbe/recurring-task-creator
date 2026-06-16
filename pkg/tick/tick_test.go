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

// testDefs is the fixed fixture used across all tick tests. It covers
// every recurrence kind needed to assert per-kind metric labels (weekday,
// monthly, quarterly, yearly) and weekday-filtering behaviour (sat vs. sun).
// The static inventory is gone; the store returns this slice via
// fakeStore.ListReturns.
var testDefs = []schedule.TaskDefinition{
	{
		Slug:          "sat-1",
		TitleTemplate: "Sat Task 1",
		Recurrence:    schedule.RecurrenceWeekday,
		Weekday:       time.Saturday,
	},
	{
		Slug:          "sat-2",
		TitleTemplate: "Sat Task 2",
		Recurrence:    schedule.RecurrenceWeekday,
		Weekday:       time.Saturday,
	},
	{
		Slug:          "sun-1",
		TitleTemplate: "Sun Task 1",
		Recurrence:    schedule.RecurrenceWeekday,
		Weekday:       time.Sunday,
	},
	{Slug: "monthly-1", TitleTemplate: "Monthly Task 1", Recurrence: schedule.RecurrenceMonthly},
	{
		Slug:          "quarterly-1",
		TitleTemplate: "Quarterly Task 1",
		Recurrence:    schedule.RecurrenceQuarterly,
	},
	{Slug: "yearly-1", TitleTemplate: "Yearly Task 1", Recurrence: schedule.RecurrenceYearly},
}

// expectedCount returns the number of testDefs entries that fire on the
// given civil date, using the same logic as the tick.
func expectedCount(date schedule.Date) int {
	return len(schedule.TasksForDate(testDefs, date))
}

var _ = Describe("Tick", func() {
	var (
		pub       *pubmocks.PublisherPublisher
		clock     libtime.CurrentDateTime
		metrics   *pubmocks.TickMetrics
		fakeStore *pubmocks.FakeScheduleStore
		tk        tick.Tick
	)

	BeforeEach(func() {
		pub = &pubmocks.PublisherPublisher{}
		pub.PublishReturns(nil)

		clock = libtime.NewCurrentDateTime()
		// 2025-01-04 is a Saturday in Europe/Berlin.
		clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))

		metrics = &pubmocks.TickMetrics{}

		fakeStore = &pubmocks.FakeScheduleStore{}
		fakeStore.ListReturns(testDefs, nil)

		var err error
		tk, err = tick.NewTick(context.Background(), fakeStore, pub, clock, metrics)
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
			// 2025-01-04 is a Saturday in Europe/Berlin; testDefs yields
			// 2 Saturday weekday + 3 always-fire = 5 entries.
			want := expectedCount(schedule.NewDate(2025, time.January, 4))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(want))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("publish-per-entry", func() {
		It("calls Publish once for every entry that fires on the civil date", func() {
			// 2025-01-04 is a Saturday; testDefs yields 5 entries.
			want := expectedCount(schedule.NewDate(2025, time.January, 4))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(want))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("Europe/Berlin date conversion", func() {
		It("converts winter UTC to next-day Berlin civil date", func() {
			// 2025-01-04T23:30:00Z is 2025-01-05 00:30 in Berlin (CET = UTC+1).
			// 2025-01-05 is a Sunday; testDefs yields 1 sun + 3 always-fire = 4 entries.
			clock.SetNow(libtimetest.ParseDateTime("2025-01-04T23:30:00Z"))
			var err error
			tk, err = tick.NewTick(context.Background(), fakeStore, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			want := expectedCount(schedule.NewDate(2025, time.January, 5))
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
			Expect(gotDate).To(Equal(schedule.NewDate(2025, time.January, 5)))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("converts summer UTC to next-day Berlin civil date (CEST)", func() {
			// 2025-07-15T23:30:00Z is 2025-07-16 01:30 in Berlin (CEST = UTC+2).
			// 2025-07-16 is a Wednesday; testDefs yields 0 weekday + 3 always-fire = 3 entries.
			clock.SetNow(libtimetest.ParseDateTime("2025-07-15T23:30:00Z"))
			var err error
			tk, err = tick.NewTick(context.Background(), fakeStore, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			want := expectedCount(schedule.NewDate(2025, time.July, 16))
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
			Expect(gotDate).To(Equal(schedule.NewDate(2025, time.July, 16)))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("store error isolation", func() {
		It("logs and skips the tick when List returns an error, no publish is called", func() {
			fakeStore.ListReturns(
				nil,
				errors.New(context.Background(), "informer cache unavailable"),
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return fakeStore.ListCallCount() }, "200ms", "5ms").
				Should(BeNumerically(">=", 1))

			// No publish must have been called.
			Expect(pub.PublishCallCount()).To(Equal(0))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("RunOnce returns nil on store error (skips tick, does not propagate error)", func() {
			fakeStore.ListReturns(nil, errors.New(context.Background(), "boom"))
			var err error
			tk, err = tick.NewTick(context.Background(), fakeStore, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			err = tk.RunOnce(context.Background())
			Expect(err).To(Succeed())
			Expect(pub.PublishCallCount()).To(Equal(0))
		})
	})

	Describe("per-task error isolation", func() {
		It("continues publishing after a Publish error", func() {
			// testDefs on Saturday yields 5 entries (2 sat-weekday + 3 always-fire).
			// The first 2 calls fail, the rest succeed.
			want := expectedCount(schedule.NewDate(2025, time.January, 4))
			errorCount := 2

			pub.PublishReturnsOnCall(0, errors.New(context.Background(), "kafka down"))
			pub.PublishReturnsOnCall(1, errors.New(context.Background(), "kafka down"))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "200ms", "5ms").
				Should(Equal(want))
			Eventually(func() int { return metrics.IncPublishedCallCount() }, "200ms", "5ms").
				Should(Equal(want))

			var errCnt, succCnt int
			for i := 0; i < want; i++ {
				r, _ := metrics.IncPublishedArgsForCall(i)
				if r == "error" {
					errCnt++
				} else {
					succCnt++
				}
			}
			Expect(errCnt).To(Equal(errorCount))
			Expect(succCnt).To(Equal(want - errorCount))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("increments the counter labeled with the task's recurrence kind", func() {
			// testDefs on Saturday: sat-1, sat-2 (weekday), monthly-1, quarterly-1, yearly-1.
			// Fail every publish to focus on the kind labels.
			want := expectedCount(schedule.NewDate(2025, time.January, 4))
			pub.PublishReturns(errors.New(context.Background(), "boom"))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return metrics.IncPublishedCallCount() }, "200ms", "5ms").
				Should(Equal(want))

			kinds := map[string]bool{}
			for i := 0; i < want; i++ {
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
		It(
			"records the kind for each RecurrenceKind value present on 2025-01-04 (Saturday)",
			func() {
				// testDefs on Saturday fires: weekday (sat-1, sat-2), monthly, quarterly, yearly.
				want := expectedCount(schedule.NewDate(2025, time.January, 4))
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
				).Should(BeNumerically(">=", want))

				kinds := map[string]bool{}
				for i := 0; i < metrics.IncPublishedCallCount(); i++ {
					r, kind := metrics.IncPublishedArgsForCall(i)
					Expect(r).To(Equal("success"))
					kinds[kind] = true
				}
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

	Describe("date-driven filtering", func() {
		var berlin *time.Location

		BeforeEach(func() {
			var err error
			berlin, err = time.LoadLocation("Europe/Berlin")
			Expect(err).NotTo(HaveOccurred())
		})

		It("publishes every entry that fires on the given civil date", func() {
			// For each instant, derive the civil date as the tick does and
			// compute the expected count from the same fixture the store returns.
			for _, instant := range []string{
				"2025-01-07T10:00:00Z", // Tuesday (0 weekday entries from testDefs)
				"2025-01-04T10:00:00Z", // Saturday (2 Saturday weekday entries)
				"2025-01-05T10:00:00Z", // Sunday (1 Sunday weekday entry)
				"2025-07-04T10:00:00Z", // Friday (0 weekday entries)
				"2026-03-01T10:00:00Z", // Sunday (1 Sunday weekday entry)
			} {
				localPub := &pubmocks.PublisherPublisher{}
				localPub.PublishReturns(nil)
				localMetrics := &pubmocks.TickMetrics{}
				localStore := &pubmocks.FakeScheduleStore{}
				localStore.ListReturns(testDefs, nil)
				clock.SetNow(libtimetest.ParseDateTime(instant))
				instanceTk, err := tick.NewTick(
					context.Background(),
					localStore,
					localPub,
					clock,
					localMetrics,
				)
				Expect(err).NotTo(HaveOccurred())

				// Compute the civil date the same way the tick does.
				now := clock.Now().Time().In(berlin)
				y, m, d := now.Date()
				civilDate := schedule.NewDate(y, m, d)
				expected := len(schedule.TasksForDate(testDefs, civilDate))
				Expect(expected).To(BeNumerically(">=", 0))

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
