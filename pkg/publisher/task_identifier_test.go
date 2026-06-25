// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"
	"time"

	lib "github.com/bborbe/agent"
	"github.com/bborbe/errors"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	projmocks "github.com/bborbe/recurring-task-creator/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("TaskIdentifierCreator", func() {
	var (
		fakeBuilder *projmocks.PublisherPeriodTokenBuilder
		creator     publisher.TaskIdentifierCreator
		ctx         context.Context
	)

	BeforeEach(func() {
		fakeBuilder = &projmocks.PublisherPeriodTokenBuilder{}
		creator = publisher.NewTaskIdentifierCreator(fakeBuilder)
		ctx = context.Background()
	})

	It("returns UUID5(namespace, recurring-<slug>-<token>) and the token verbatim", func() {
		fakeBuilder.BuildReturns(publisher.PeriodToken("2026-06"), nil)
		def := schedule.TaskDefinition{
			Slug:       "monthly-review",
			Recurrence: schedule.RecurrenceMonthly,
		}
		id, token, err := creator.Create(ctx, def, schedule.NewDate(2026, time.July, 1))
		Expect(err).NotTo(HaveOccurred())
		Expect(token).To(Equal(publisher.PeriodToken("2026-06")))
		expected := lib.TaskIdentifier(
			uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte("recurring-monthly-review-2026-06")).
				String(),
		)
		Expect(id).To(Equal(expected))
	})

	It("passes (def, date) through to the PeriodTokenBuilder verbatim", func() {
		fakeBuilder.BuildReturns(publisher.PeriodToken("ignored"), nil)
		def := schedule.TaskDefinition{Slug: "x", Recurrence: schedule.RecurrenceDaily}
		date := schedule.NewDate(2026, time.June, 15)
		_, _, err := creator.Create(ctx, def, date)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeBuilder.BuildCallCount()).To(Equal(1))
		_, gotDef, gotDate := fakeBuilder.BuildArgsForCall(0)
		Expect(gotDef).To(Equal(def))
		Expect(gotDate).To(Equal(date))
	})

	It("wraps the builder's error with the slug for diagnosis", func() {
		fakeBuilder.BuildReturns("", errors.Errorf(ctx, "build boom"))
		def := schedule.TaskDefinition{Slug: "broken-slug", Recurrence: schedule.RecurrenceDaily}
		_, _, err := creator.Create(ctx, def, schedule.NewDate(2026, time.June, 15))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("broken-slug"))
		Expect(err.Error()).To(ContainSubstring("build boom"))
	})
})
