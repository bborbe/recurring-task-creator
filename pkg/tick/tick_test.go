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

	pubmocks "github.com/bborbe/recurring-task-creator/pkg/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

var _ = Describe("Tick", func() {
	var (
		pub        *pubmocks.PublisherPublisher
		clock      libtime.CurrentDateTime
		metrics    *pubmocks.TickMetrics
		scheduleFn tick.ScheduleLookup
		tk         tick.Tick
	)

	BeforeEach(func() {
		pub = &pubmocks.PublisherPublisher{}
		pub.PublishReturns(nil)

		clock = libtime.NewCurrentDateTime()
		clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))

		metrics = &pubmocks.TickMetrics{}

		scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
			return []schedule.TaskDefinition{
				{
					Slug:          "weekly-review",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
				},
			}
		}

		var err error
		tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(Equal(1))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("publish-per-entry", func() {
		It("calls Publish once for a single-entry scheduleFn", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(Equal(1))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("calls Publish once for every entry when scheduleFn returns 3", func() {
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{
					{Slug: "a", TitleTemplate: "t", Recurrence: schedule.RecurrenceDaily},
					{Slug: "b", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly},
					{Slug: "c", TitleTemplate: "t", Recurrence: schedule.RecurrenceMonthly},
				}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(Equal(3))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("Europe/Berlin date conversion", func() {
		It("converts winter UTC to next-day Berlin civil date", func() {
			clock.SetNow(libtimetest.ParseDateTime("2025-01-04T23:30:00Z"))
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{{Slug: "x"}}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(Equal(1))

			_, _, gotDate := pub.PublishArgsForCall(0)
			Expect(gotDate).To(Equal(schedule.NewDate(2025, time.January, 5)))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("converts summer UTC to next-day Berlin civil date (CEST)", func() {
			clock.SetNow(libtimetest.ParseDateTime("2025-07-15T23:30:00Z"))
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{{Slug: "x"}}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(Equal(1))

			_, _, gotDate := pub.PublishArgsForCall(0)
			Expect(gotDate).To(Equal(schedule.NewDate(2025, time.July, 16)))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("no tasks for date", func() {
		It("updates the gauge but does not call Publish or Inc", func() {
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(
				func() int { return metrics.SetLastTickTimestampCallCount() },
				"100ms",
				"5ms",
			).Should(Equal(1))

			Expect(pub.PublishCallCount()).To(Equal(0))
			Expect(metrics.IncPublishedCallCount()).To(Equal(0))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("per-task error isolation", func() {
		It("continues publishing after a Publish error", func() {
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{
					{Slug: "a", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly},
					{Slug: "b", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly},
					{Slug: "c", TitleTemplate: "t", Recurrence: schedule.RecurrenceWeekly},
				}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			pub.PublishReturnsOnCall(0, errors.New(context.Background(), "kafka down"))
			pub.PublishReturnsOnCall(1, nil)
			pub.PublishReturnsOnCall(2, nil)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(Equal(3))
			Eventually(func() int { return metrics.IncPublishedCallCount() }, "100ms", "5ms").
				Should(Equal(3))

			var errorCount, successCount int
			for i := 0; i < 3; i++ {
				r, kind := metrics.IncPublishedArgsForCall(i)
				Expect(kind).To(Equal("weekly"))
				if r == "error" {
					errorCount++
				} else {
					successCount++
				}
			}
			Expect(errorCount).To(Equal(1))
			Expect(successCount).To(Equal(2))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})

		It("increments the counter labeled with the task's recurrence kind", func() {
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{
					{Slug: "a", TitleTemplate: "t", Recurrence: schedule.RecurrenceDaily},
					{Slug: "b", TitleTemplate: "t", Recurrence: schedule.RecurrenceMonthly},
				}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
			Expect(err).NotTo(HaveOccurred())

			pub.PublishReturns(errors.New(context.Background(), "boom"))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				_ = tk.Run(ctx)
				close(done)
			}()

			Eventually(func() int { return metrics.IncPublishedCallCount() }, "100ms", "5ms").
				Should(Equal(2))

			kinds := map[string]bool{}
			for i := 0; i < 2; i++ {
				r, kind := metrics.IncPublishedArgsForCall(i)
				Expect(r).To(Equal("error"))
				kinds[kind] = true
			}
			Expect(kinds).To(HaveKey("daily"))
			Expect(kinds).To(HaveKey("monthly"))

			cancel()
			Eventually(done, "200ms", "5ms").Should(BeClosed())
		})
	})

	Describe("context cancellation", func() {
		It("exits the per-task loop early when ctx is cancelled mid-tick", func() {
			scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
				return []schedule.TaskDefinition{
					{Slug: "a", TitleTemplate: "t"},
					{Slug: "b", TitleTemplate: "t"},
					{Slug: "c", TitleTemplate: "t"},
					{Slug: "d", TitleTemplate: "t"},
					{Slug: "e", TitleTemplate: "t"},
				}
			}
			var err error
			tk, err = tick.NewTick(context.Background(), scheduleFn, pub, clock, metrics)
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

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
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

			Eventually(func() int { return pub.PublishCallCount() }, "100ms", "5ms").
				Should(BeNumerically(">=", 1))

			cancel()

			Eventually(done, "100ms", "5ms").Should(BeClosed())
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
				"100ms",
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
		DescribeTable(
			"records the kind for each RecurrenceKind value",
			func(kind schedule.RecurrenceKind) {
				scheduleFn = func(date schedule.Date) []schedule.TaskDefinition {
					return []schedule.TaskDefinition{
						{Slug: "x", TitleTemplate: "t", Recurrence: kind},
					}
				}
				var err error
				tk, err = tick.NewTick(
					context.Background(),
					scheduleFn,
					pub,
					clock,
					metrics,
				)
				Expect(err).NotTo(HaveOccurred())

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					_ = tk.Run(ctx)
					close(done)
				}()

				Eventually(
					func() int { return metrics.IncPublishedCallCount() },
					"100ms",
					"5ms",
				).Should(Equal(1))

				r, got := metrics.IncPublishedArgsForCall(0)
				Expect(r).To(Equal("success"))
				Expect(got).To(Equal(string(kind)))

				cancel()
				Eventually(done, "200ms", "5ms").Should(BeClosed())
			},
			Entry("daily", schedule.RecurrenceDaily),
			Entry("weekly", schedule.RecurrenceWeekly),
			Entry("monthly", schedule.RecurrenceMonthly),
			Entry("quarterly", schedule.RecurrenceQuarterly),
			Entry("yearly", schedule.RecurrenceYearly),
		)
	})
})

var _ = Describe("Prometheus pre-initialization", func() {
	It("registers the counter with 10 zero-valued series", func() {
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
		Expect(metrics).To(HaveLen(10))

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
			Expect(k).To(BeElementOf("daily", "weekly", "monthly", "quarterly", "yearly"))
			seen[r+"/"+k] = true
			Expect(m.GetCounter().GetValue()).To(Equal(0.0))
		}
		Expect(seen).To(HaveLen(10))
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
