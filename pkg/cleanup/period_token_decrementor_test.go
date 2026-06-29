// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("PriorPeriodToken", func() {
	builder := publisher.NewPeriodTokenBuilder()
	ctx := context.Background()

	DescribeTable("returns correct prior period token",
		func(def schedule.TaskDefinition, currentDate schedule.Date, expectedToken string) {
			token, err := cleanup.PriorPeriodToken(ctx, builder, def, currentDate)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(string(token)).Should(Equal(expectedToken))
		},
		// Daily: currentDate=2026-06-29 → prior=2026-06-28
		Entry("RecurrenceDaily 2026-06-29 → 2026-06-28",
			schedule.TaskDefinition{Slug: "daily-task", Recurrence: schedule.RecurrenceDaily},
			schedule.NewDate(2026, time.June, 29),
			"2026-06-28",
		),
		Entry("RecurrenceDaily 2026-07-01 → 2026-06-30",
			schedule.TaskDefinition{Slug: "daily-task", Recurrence: schedule.RecurrenceDaily},
			schedule.NewDate(2026, time.July, 1),
			"2026-06-30",
		),
		Entry("RecurrenceDaily 2026-03-01 → 2026-02-28",
			schedule.TaskDefinition{Slug: "daily-task", Recurrence: schedule.RecurrenceDaily},
			schedule.NewDate(2026, time.March, 1),
			"2026-02-28",
		),

		// Weekly: currentDate=2026-06-29 (Monday ISO week 27) → prior=2026-06-22
		Entry("RecurrenceWeekly 2026-06-29 → 2026W26",
			schedule.TaskDefinition{Slug: "weekly-task", Recurrence: schedule.RecurrenceWeekly},
			schedule.NewDate(2026, time.June, 29),
			"2026W26",
		),
		Entry("RecurrenceWeekly 2026-06-22 → 2026W25",
			schedule.TaskDefinition{Slug: "weekly-task", Recurrence: schedule.RecurrenceWeekly},
			schedule.NewDate(2026, time.June, 22),
			"2026W25",
		),
		Entry("RecurrenceWeekly 2026-01-05 → 2026W01",
			schedule.TaskDefinition{Slug: "weekly-task", Recurrence: schedule.RecurrenceWeekly},
			schedule.NewDate(2026, time.January, 5),
			"2026W01",
		),

		// Weekday: {Mon, Wed, Fri}. 2026-06-29 is Monday → prior Fri 2026-06-26.
		// 2026-06-26 is a Friday.
		Entry("RecurrenceWeekday Mon→prior Fri 2026-06-29",
			schedule.TaskDefinition{
				Slug:       "weekday-task",
				Recurrence: schedule.RecurrenceWeekday,
				Weekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			},
			schedule.NewDate(2026, time.June, 29),
			"2026W26-fri",
		),
		Entry("RecurrenceWeekday Wed→prior Mon 2026-06-24",
			schedule.TaskDefinition{
				Slug:       "weekday-task",
				Recurrence: schedule.RecurrenceWeekday,
				Weekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			},
			schedule.NewDate(2026, time.June, 24),
			"2026W26-mon",
		),
		Entry("RecurrenceWeekday Fri→prior Wed 2026-06-26",
			schedule.TaskDefinition{
				Slug:       "weekday-task",
				Recurrence: schedule.RecurrenceWeekday,
				Weekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			},
			schedule.NewDate(2026, time.June, 26),
			"2026W26-wed",
		),

		// Monthly: currentDate=2026-06-29 → priorMonth=May 2026-05
		Entry("RecurrenceMonthly 2026-06 → 2026-05",
			schedule.TaskDefinition{Slug: "monthly-task", Recurrence: schedule.RecurrenceMonthly},
			schedule.NewDate(2026, time.June, 29),
			"2026-05",
		),
		Entry("RecurrenceMonthly 2026-03 → 2026-02",
			schedule.TaskDefinition{Slug: "monthly-task", Recurrence: schedule.RecurrenceMonthly},
			schedule.NewDate(2026, time.March, 15),
			"2026-02",
		),
		Entry("RecurrenceMonthly 2026-01 → 2025-12",
			schedule.TaskDefinition{Slug: "monthly-task", Recurrence: schedule.RecurrenceMonthly},
			schedule.NewDate(2026, time.January, 10),
			"2025-12",
		),

		// Quarterly: 2026-Q2 (Apr-Jun) → Q1 (Jan-Mar)
		Entry(
			"RecurrenceQuarterly 2026-06 → 2026Q1",
			schedule.TaskDefinition{
				Slug:       "quarterly-task",
				Recurrence: schedule.RecurrenceQuarterly,
			},
			schedule.NewDate(2026, time.June, 29),
			"2026Q1",
		),
		Entry(
			"RecurrenceQuarterly 2026-04 → 2026Q1",
			schedule.TaskDefinition{
				Slug:       "quarterly-task",
				Recurrence: schedule.RecurrenceQuarterly,
			},
			schedule.NewDate(2026, time.April, 1),
			"2026Q1",
		),
		Entry(
			"RecurrenceQuarterly 2026-01 → 2025Q4",
			schedule.TaskDefinition{
				Slug:       "quarterly-task",
				Recurrence: schedule.RecurrenceQuarterly,
			},
			schedule.NewDate(2026, time.January, 15),
			"2025Q4",
		),

		// Yearly: 2026 → 2025
		Entry("RecurrenceYearly 2026 → 2025",
			schedule.TaskDefinition{Slug: "yearly-task", Recurrence: schedule.RecurrenceYearly},
			schedule.NewDate(2026, time.June, 29),
			"2025",
		),
		Entry("RecurrenceYearly 2025 → 2024",
			schedule.TaskDefinition{Slug: "yearly-task", Recurrence: schedule.RecurrenceYearly},
			schedule.NewDate(2025, time.December, 31),
			"2024",
		),
		Entry("RecurrenceYearly 2027 → 2026",
			schedule.TaskDefinition{Slug: "yearly-task", Recurrence: schedule.RecurrenceYearly},
			schedule.NewDate(2027, time.January, 1),
			"2026",
		),
	)
})
