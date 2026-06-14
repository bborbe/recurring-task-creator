// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory_test

import (
	"context"
	"time"

	taskmocks "github.com/bborbe/agent/lib/command/task/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
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
