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

	DescribeTable("weekday normalization — all 14 day strings map to canonical time.Weekday",
		func(input string, expected time.Weekday) {
			cr := &v1.Schedule{
				ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
				Spec: v1.ScheduleSpec{
					Title: "Test",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Weekday",
						Weekday:    input,
					},
				},
			}
			def, err := store.AdaptScheduleForTest(ctx, cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(def.Weekdays).To(Equal([]time.Weekday{expected}))
		},
		Entry("Monday long", "Monday", time.Monday),
		Entry("Tuesday long", "Tuesday", time.Tuesday),
		Entry("Wednesday long", "Wednesday", time.Wednesday),
		Entry("Thursday long", "Thursday", time.Thursday),
		Entry("Friday long", "Friday", time.Friday),
		Entry("Saturday long", "Saturday", time.Saturday),
		Entry("Sunday long", "Sunday", time.Sunday),
		Entry("Mon short", "Mon", time.Monday),
		Entry("Tue short", "Tue", time.Tuesday),
		Entry("Wed short", "Wed", time.Wednesday),
		Entry("Thu short", "Thu", time.Thursday),
		Entry("Fri short", "Fri", time.Friday),
		Entry("Sat short", "Sat", time.Saturday),
		Entry("Sun short", "Sun", time.Sunday),
	)

	It("maps weekday Saturday (single-string path)", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title: "Test",
				Schedule: v1.ScheduleTrigger{
					Recurrence: "Weekday",
					Weekday:    "Saturday",
				},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Weekdays).To(Equal([]time.Weekday{time.Saturday}))
	})

	It("mixed-form list [Mon,Wednesday,Fri] produces {Monday,Wednesday,Friday}", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title: "Test",
				Schedule: v1.ScheduleTrigger{
					Recurrence: "Weekday",
					Weekdays:   []string{"Mon", "Wednesday", "Fri"},
				},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Weekdays).To(Equal([]time.Weekday{time.Monday, time.Wednesday, time.Friday}))
	})

	It(
		"long-form and mixed-form lists produce the same Weekdays (long==mixed equivalence)",
		func() {
			crLong := &v1.Schedule{
				ObjectMeta: metav1.ObjectMeta{Name: "test-long"},
				Spec: v1.ScheduleSpec{
					Title: "Test",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Weekday",
						Weekdays:   []string{"Monday", "Wednesday", "Friday"},
					},
				},
			}
			crMixed := &v1.Schedule{
				ObjectMeta: metav1.ObjectMeta{Name: "test-mixed"},
				Spec: v1.ScheduleSpec{
					Title: "Test",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Weekday",
						Weekdays:   []string{"Mon", "Wednesday", "Fri"},
					},
				},
			}
			defLong, err := store.AdaptScheduleForTest(ctx, crLong)
			Expect(err).NotTo(HaveOccurred())
			defMixed, err := store.AdaptScheduleForTest(ctx, crMixed)
			Expect(err).NotTo(HaveOccurred())
			Expect(defLong.Weekdays).To(Equal(defMixed.Weekdays))
		},
	)

	It(
		"single-string weekday and one-element list weekdays produce identical def.Weekdays",
		func() {
			crSingle := &v1.Schedule{
				ObjectMeta: metav1.ObjectMeta{Name: "test-single"},
				Spec: v1.ScheduleSpec{
					Title: "Test",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Weekday",
						Weekday:    "Monday",
					},
				},
			}
			crList := &v1.Schedule{
				ObjectMeta: metav1.ObjectMeta{Name: "test-list"},
				Spec: v1.ScheduleSpec{
					Title: "Test",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Weekday",
						Weekdays:   []string{"Monday"},
					},
				},
			}
			defSingle, err := store.AdaptScheduleForTest(ctx, crSingle)
			Expect(err).NotTo(HaveOccurred())
			defList, err := store.AdaptScheduleForTest(ctx, crList)
			Expect(err).NotTo(HaveOccurred())
			Expect(defSingle.Weekdays).To(Equal(defList.Weekdays))
			Expect(defSingle.Weekdays).To(Equal([]time.Weekday{time.Monday}))
		},
	)

	It("maps empty weekday to nil Weekdays for a Daily CR", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title:    "Test",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Weekdays).To(BeEmpty())
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

	It("returns error for unknown weekday (single-string path)", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title: "Test",
				Schedule: v1.ScheduleTrigger{
					Recurrence: "Weekday",
					Weekday:    "Funday",
				},
			},
		}
		_, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown weekday"))
	})

	It("returns error for unknown weekday (list path)", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-slug"},
			Spec: v1.ScheduleSpec{
				Title: "Test",
				Schedule: v1.ScheduleTrigger{
					Recurrence: "Weekday",
					Weekdays:   []string{"Funday"},
				},
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

	It("resolves a nil autoAbortPrior pointer to false", func() {
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "no-flag"},
			Spec: v1.ScheduleSpec{
				Title:    "No Flag",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily"},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.AutoAbortPrior).To(BeFalse())
	})

	It("resolves an explicit false autoAbortPrior to false", func() {
		f := false
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "explicit-false"},
			Spec: v1.ScheduleSpec{
				Title:    "Explicit False",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily", AutoAbortPrior: &f},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.AutoAbortPrior).To(BeFalse())
	})

	It("resolves an explicit true autoAbortPrior to true", func() {
		t := true
		cr := &v1.Schedule{
			ObjectMeta: metav1.ObjectMeta{Name: "explicit-true"},
			Spec: v1.ScheduleSpec{
				Title:    "Explicit True",
				Schedule: v1.ScheduleTrigger{Recurrence: "Daily", AutoAbortPrior: &t},
				Template: v1.ScheduleTemplate{Body: "B"},
			},
		}
		def, err := store.AdaptScheduleForTest(ctx, cr)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.AutoAbortPrior).To(BeTrue())
	})
})
