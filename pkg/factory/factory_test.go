// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory_test

import (
	"context"
	"time"

	taskmocks "github.com/bborbe/agent/lib/command/task/mocks"
	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/factory"
	projmocks "github.com/bborbe/recurring-task-creator/pkg/mocks"
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
		pub = factory.CreatePublisher(sender)
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
		t           tick.Tick
	)
	BeforeEach(func() {
		pubFake = &projmocks.PublisherPublisher{}
		clock = libtime.NewCurrentDateTime()
		metricsFake = &projmocks.TickMetrics{}
		t = factory.CreateTick(context.Background(), pubFake, clock, metricsFake)
	})
	It("returns a Tick that wires the publisher, clock, and metrics", func() {
		Expect(t).NotTo(BeNil())
	})
	It("does not panic on the happy path (Europe/Berlin loadable)", func() {
		// Implicit: if CreateTick panicked, the BeforeEach would have
		// failed this test. The presence of a non-nil Tick IS the proof.
		Expect(t).NotTo(BeNil())
	})
})
