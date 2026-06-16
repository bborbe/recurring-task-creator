// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/store"
)

var _ = Describe("adaptSchedule", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("recurrence mapping",
		func(input string, expected schedule.RecurrenceKind) {
			cr := &v1.Schedule{
				ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
				Spec: v1.ScheduleSpec{
					Title:    "Test Title",
					Schedule: v1.ScheduleTrigger{Recurrence: input},
				},
			}
			def, err := store.AdaptScheduleForTest(ctx, cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(def.Recurrence).To(Equal(expected))
		},
		Entry("daily", "Daily", schedule.RecurrenceDaily),
		Entry("weekly", "Weekly", schedule.RecurrenceWeekly),
		Entry("weekday", "Weekday", schedule.RecurrenceWeekday),
		Entry("monthly", "Monthly", schedule.RecurrenceMonthly),
		Entry("quarterly", "Quarterly", schedule.RecurrenceQuarterly),
		Entry("yearly", "Yearly", schedule.RecurrenceYearly),
	)

	It("maps weekday Saturday", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title:    "Test",
				Schedule: v1.ScheduleTrigger{Recurrence: "Weekday", Weekday: "Saturday"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Weekday).To(Equal(time.Saturday))
	})

	It("maps empty weekday to zero value (time.Sunday)", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title:    "Test",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Weekday).To(Equal(time.Sunday))
	})

	It("returns error for unknown recurrence", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title:    "Test",
				Schedule: v1.ScheduleTrigger{Recurrence: "Bogus"},
			},
		}
		_, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown recurrence"))
	})

	It("returns error for unknown weekday", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title:    "Test",
				Schedule: v1.ScheduleTrigger{Recurrence: "Weekday", Weekday: "Funday"},
			},
		}
		_, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown weekday"))
	})

	It("maps slug, title template, and body template", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "my-slug"},
			Spec: v1.ScheduleSpec{
				Title:    "My Title",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
				Template: v1.ScheduleTemplate{Body: "My Body"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Slug).To(Equal("my-slug"))
		Expect(def.TitleTemplate).To(Equal("My Title"))
		Expect(def.BodyTemplate).To(Equal("My Body"))
	})
})
