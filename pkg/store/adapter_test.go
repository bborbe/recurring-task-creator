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

	It("copies frontmatter verbatim from CRD into TaskDefinition.Frontmatter", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "slug-with-fm"},
			Spec: v1.ScheduleSpec{
				Title:    "T",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
				Template: v1.ScheduleTemplate{
					Body: "B",
					Frontmatter: map[string]interface{}{
						"assignee": "alice",
						"priority": 4,
						"goals":    []interface{}{"[[Example Goal]]"},
						"category": "ops",
					},
				},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Frontmatter).To(HaveKeyWithValue("assignee", "alice"))
		Expect(def.Frontmatter).To(HaveKeyWithValue("priority", 4))
		Expect(def.Frontmatter).To(HaveKeyWithValue("goals", []interface{}{"[[Example Goal]]"}))
		Expect(def.Frontmatter).To(HaveKeyWithValue("category", "ops"))
		Expect(def.Frontmatter).To(HaveLen(4))
	})

	It("leaves Frontmatter nil when the CR's template.frontmatter is absent", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "no-fm"},
			Spec: v1.ScheduleSpec{
				Title:    "T",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Frontmatter).To(BeNil())
	})

	It("propagates PeriodOffset from CR to TaskDefinition", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "review"},
			Spec: v1.ScheduleSpec{
				Title:    "Review Month",
				Schedule: v1.ScheduleTrigger{Recurrence: "Monthly", PeriodOffset: -1},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.PeriodOffset).To(Equal(-1))
	})

	It("defaults PeriodOffset to 0 when omitted on the CR", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "plan"},
			Spec: v1.ScheduleSpec{
				Title:    "Plan Month",
				Schedule: v1.ScheduleTrigger{Recurrence: "Monthly"},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.PeriodOffset).To(Equal(0))
	})
})
