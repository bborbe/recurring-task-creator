// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("PeriodTokenBuilder.Build", func() {
	build := func(def schedule.TaskDefinition, date schedule.Date) publisher.PeriodToken {
		tok, err := publisher.NewPeriodTokenBuilder().Build(context.Background(), def, date)
		Expect(err).NotTo(HaveOccurred())
		return tok
	}

	Describe("RecurrenceMonthly PeriodOffset", func() {
		DescribeTable(
			"anchors to first of month before shifting to avoid day-overflow bugs",
			func(date schedule.Date, offset int, expectedToken string) {
				def := schedule.TaskDefinition{
					Slug:         "monthly-review",
					Recurrence:   schedule.RecurrenceMonthly,
					PeriodOffset: offset,
				}
				Expect(string(build(def, date))).To(Equal(expectedToken))
			},
			// Control/regression entries — these pass on old code too (target month has enough days).
			Entry("fire on 1st, offset=-1 → prior month",
				schedule.NewDate(2026, time.July, 1),
				-1,
				"2026-06",
			),
			Entry("fire on 30th, offset=-1 → prior month (June has 30 days)",
				schedule.NewDate(2026, time.July, 30),
				-1,
				"2026-06",
			),
			Entry("fire on 31st, offset=-1 → prior month (December has 31 days)",
				schedule.NewDate(2026, time.January, 31),
				-1,
				"2025-12",
			),
			// Discriminating entries — old code produces wrong token on these (overflow into next month).
			Entry("fire on 31st, offset=-1 → June (not July) — June has only 30 days",
				schedule.NewDate(2026, time.July, 31),
				-1,
				"2026-06",
			),
			Entry("fire on 31st, offset=-1 → February (not March) — Feb has only 28/29 days",
				schedule.NewDate(2026, time.March, 31),
				-1,
				"2026-02",
			),
			Entry("fire on 31st, offset=-1 → April (not May) — April has only 30 days",
				schedule.NewDate(2026, time.May, 31),
				-1,
				"2026-04",
			),
			Entry(
				"fire on 31st, offset=-1 → July (not August) — July has 31 days but August target does too",
				schedule.NewDate(2026, time.August, 31),
				-1,
				"2026-07",
			),
			// Zero offset must not change behavior.
			Entry("offset=0 on 31st → fire month unchanged (day-1 anchor is a no-op at offset 0)",
				schedule.NewDate(2026, time.July, 31),
				0,
				"2026-07",
			),
		)
	})

	Describe("RecurrenceQuarterly PeriodOffset", func() {
		DescribeTable("anchors to first of month before shifting",
			func(date schedule.Date, offset int, expectedToken string) {
				def := schedule.TaskDefinition{
					Slug:         "quarterly-review",
					Recurrence:   schedule.RecurrenceQuarterly,
					PeriodOffset: offset,
				}
				Expect(string(build(def, date))).To(Equal(expectedToken))
			},
			// Discriminating case: Dec 31 shifted by -3 months would be "Sep 31" → Oct 1 on old code → Q4 not Q3.
			Entry("fire Dec 31, offset=-1 → Q3 (not Q4) — Sep has only 30 days",
				schedule.NewDate(2026, time.December, 31),
				-1,
				"2026Q3",
			),
			// Control/regression entries.
			Entry("fire Jul 31, offset=-1 → Q2 (Jul is Q3; prior quarter is Q2)",
				schedule.NewDate(2026, time.July, 31),
				-1,
				"2026Q2",
			),
			Entry("fire Oct 31, offset=-1 → Q3 (Oct is Q4; prior quarter is Q3)",
				schedule.NewDate(2026, time.October, 31),
				-1,
				"2026Q3",
			),
		)
	})

	Describe("RecurrenceYearly PeriodOffset", func() {
		DescribeTable("anchors to first of month before shifting",
			func(date schedule.Date, offset int, expectedToken string) {
				def := schedule.TaskDefinition{
					Slug:         "yearly-review",
					Recurrence:   schedule.RecurrenceYearly,
					PeriodOffset: offset,
				}
				Expect(string(build(def, date))).To(Equal(expectedToken))
			},
			// Defensive hygiene: leap day shifted by -1 year hits the year-boundary,
			// but fmtYear only reads .Year() so December overflow cannot reach the
			// following year anyway. We keep the day-1 anchor for consistency.
			Entry("fire Feb 29 (leap), offset=-1 → prior year",
				schedule.NewDate(2028, time.February, 29),
				-1,
				"2027",
			),
			Entry("fire Jan 1, offset=-1 → prior year",
				schedule.NewDate(2026, time.January, 1),
				-1,
				"2025",
			),
		)
	})

	Describe("non-offset kinds unaffected", func() {
		It("RecurrenceDaily produces YYYY-MM-DD token on an arbitrary date", func() {
			def := schedule.TaskDefinition{
				Slug:       "daily-task",
				Recurrence: schedule.RecurrenceDaily,
			}
			Expect(
				string(build(def, schedule.NewDate(2026, time.July, 31))),
			).To(Equal("2026-07-31"))
		})

		It("RecurrenceWeekly produces YYYYWww token on an arbitrary date", func() {
			def := schedule.TaskDefinition{
				Slug:       "weekly-task",
				Recurrence: schedule.RecurrenceWeekly,
			}
			// 2026-07-31 is a Friday, ISO week 2026W31.
			Expect(string(build(def, schedule.NewDate(2026, time.July, 31)))).To(Equal("2026W31"))
		})
	})
})
