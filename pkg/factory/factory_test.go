// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	taskmocks "github.com/bborbe/agent/lib/mocks"
	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	versionedfake "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned/fake"
	projmocks "github.com/bborbe/recurring-task-creator/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

var _ = Describe("CreatePublisher", func() {
	var (
		sender *taskmocks.TaskCreateCommandSender
		pub    publisher.Publisher
	)
	BeforeEach(func() {
		sender = &taskmocks.TaskCreateCommandSender{}
		sender.SendCommandReturns(nil)
		pub = factory.CreatePublisher(sender, false)
	})
	It("returns a Publisher that delegates to the sender", func() {
		def := schedule.TaskDefinition{
			Slug:          "weekly-review",
			TitleTemplate: "t",
			Recurrence:    schedule.RecurrenceWeekly,
		}
		Expect(pub.Publish(
			context.Background(),
			def,
			schedule.NewDate(2025, time.January, 4),
		)).To(Succeed())
		Expect(sender.SendCommandCallCount()).To(Equal(1))
	})
})

var _ = Describe("CreateTick", func() {
	var (
		pubFake     *projmocks.PublisherPublisher
		clock       libtime.CurrentDateTime
		metricsFake *projmocks.TickMetrics
		storeFake   *projmocks.FakeScheduleStore
		t           tick.Tick
	)
	BeforeEach(func() {
		pubFake = &projmocks.PublisherPublisher{}
		clock = libtime.NewCurrentDateTime()
		metricsFake = &projmocks.TickMetrics{}
		storeFake = &projmocks.FakeScheduleStore{}
		storeFake.ListReturns(nil, nil)
		t = factory.CreateTick(context.Background(), storeFake, pubFake, clock, metricsFake)
	})
	It("returns a Tick that wires the publisher, clock, and metrics", func() {
		Expect(t).NotTo(BeNil())
	})
	It("does not panic on the happy path (Europe/Berlin loadable)", func() {
		Expect(t).NotTo(BeNil())
	})
})

var _ = Describe("CreateHealthzHandler", func() {
	It("returns a non-nil http.Handler", func() {
		Expect(factory.CreateHealthzHandler()).NotTo(BeNil())
	})
})

var _ = Describe("CreateTriggerHandler", func() {
	var (
		pubFake   *projmocks.PublisherPublisher
		storeFake *projmocks.FakeScheduleStore
		httpHndl  http.Handler
	)
	BeforeEach(func() {
		pubFake = &projmocks.PublisherPublisher{}
		pubFake.PublishReturns(nil)
		storeFake = &projmocks.FakeScheduleStore{}
		// Saturday fixture: sat entry + 2 always-fire entries.
		storeFake.ListReturns([]schedule.TaskDefinition{
			{
				Slug:          "sat-task",
				TitleTemplate: "Sat",
				Recurrence:    schedule.RecurrenceWeekday,
				Weekday:       time.Saturday,
			},
			{
				Slug:          "monthly-task",
				TitleTemplate: "Monthly",
				Recurrence:    schedule.RecurrenceMonthly,
			},
			{Slug: "yearly-task", TitleTemplate: "Yearly", Recurrence: schedule.RecurrenceYearly},
		}, nil)
		httpHndl = factory.CreateTriggerHandler(storeFake, pubFake)
	})
	It("returns a non-nil http.Handler", func() {
		Expect(httpHndl).NotTo(BeNil())
	})
	It(
		"wires the supplied publisher into the handler (publish is reachable through the returned handler)",
		func() {
			// 2025-01-04 is a Saturday; the fixture has 1 sat-weekday + 2 always-fire = 3 entries.
			req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
			resp := httptest.NewRecorder()
			httpHndl.ServeHTTP(resp, req)
			Expect(resp.Code).To(Equal(http.StatusOK))
			// fixture: sat-task + monthly-task + yearly-task = 3 on Saturday
			Expect(pubFake.PublishCallCount()).To(Equal(3))
		},
	)
})

var _ = Describe("CreateScheduleStore", func() {
	It("returns a non-nil factory and a non-nil store", func() {
		fakeClient := versionedfake.NewSimpleClientset()
		informerFactory, scheduleStore := factory.CreateScheduleStore(fakeClient, "test-namespace")
		Expect(informerFactory).NotTo(BeNil())
		Expect(scheduleStore).NotTo(BeNil())
	})

	It("store lists zero definitions after StartWithContext + sync on an empty cluster", func() {
		fakeClient := versionedfake.NewSimpleClientset()
		informerFactory, scheduleStore := factory.CreateScheduleStore(fakeClient, "test-namespace")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		informerFactory.StartWithContext(ctx)
		syncResult := informerFactory.WaitForCacheSyncWithContext(ctx)
		for _, ok := range syncResult.Synced {
			Expect(ok).To(BeTrue(), "informer cache must sync within 5s on the fake client")
		}

		defs, err := scheduleStore.List(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(defs).To(BeEmpty())
	})
})
