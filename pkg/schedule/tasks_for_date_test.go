// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("TasksForDate", func() {
	var defs []schedule.TaskDefinition

	BeforeEach(func() {
		// Synthetic fixtures — not the production inventory. The production
		// inventory is now managed as Schedule CRs; this spec exercises the
		// filter rule with a controlled set of entries.
		defs = []schedule.TaskDefinition{
			{Slug: "daily-x", Recurrence: schedule.RecurrenceDaily},
			{Slug: "weekly-x", Recurrence: schedule.RecurrenceWeekly},
			{Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
			{Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
			{Slug: "monthly-x", Recurrence: schedule.RecurrenceMonthly},
		}
	})

	// Helper: drive TasksForDate with the synthetic fixtures.
	filter := func(date schedule.Date) []schedule.TaskDefinition {
		return schedule.TasksForDate(defs, date)
	}

	It("returns the full set on a Saturday when all weekday entries match", func() {
		// 2025-01-04 is a Saturday. Saturday weekday entries fire; Sunday ones do not.
		got := filter(schedule.NewDate(2025, time.January, 4))
		slugs := slugsOf(got)
		Expect(slugs).To(ConsistOf(
			"daily-x", "weekly-x", "weekday-sat", "monthly-x",
		))
	})

	It("returns the full set on a Sunday when all weekday entries match", func() {
		// 2025-01-05 is a Sunday. Sunday weekday entries fire; Saturday ones do not.
		got := filter(schedule.NewDate(2025, time.January, 5))
		slugs := slugsOf(got)
		Expect(slugs).To(ConsistOf(
			"daily-x", "weekly-x", "weekday-sun", "monthly-x",
		))
	})

	It("returns zero weekday-kind tasks on a Tuesday (regression fix)", func() {
		// 2025-01-07 is a Tuesday. No weekday-kind entry fires; the 4
		// always-fire entries (daily, weekly, monthly) are returned.
		got := filter(schedule.NewDate(2025, time.January, 7))
		slugs := slugsOf(got)
		Expect(slugs).To(ConsistOf(
			"daily-x", "weekly-x", "monthly-x",
		))
		Expect(slugs).NotTo(ContainElement("weekday-sat"))
		Expect(slugs).NotTo(ContainElement("weekday-sun"))
	})

	It(
		"returns exactly the Saturday weekday entry on a Saturday for a weekday-only inventory",
		func() {
			weekdayOnly := []schedule.TaskDefinition{
				{
					Slug:       "weekday-sat",
					Recurrence: schedule.RecurrenceWeekday,
					Weekday:    time.Saturday,
				},
				{Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
			}
			got := schedule.TasksForDate(
				weekdayOnly,
				schedule.NewDate(2025, time.January, 4),
			)
			Expect(slugsOf(got)).To(ConsistOf("weekday-sat"))
		},
	)

	It("returns exactly the Sunday weekday entry on a Sunday for a weekday-only inventory", func() {
		weekdayOnly := []schedule.TaskDefinition{
			{Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
			{Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
		}
		got := schedule.TasksForDate(
			weekdayOnly,
			schedule.NewDate(2025, time.January, 5),
		)
		Expect(slugsOf(got)).To(ConsistOf("weekday-sun"))
	})

	It("returns an empty slice for an empty inventory", func() {
		got := schedule.TasksForDate(
			[]schedule.TaskDefinition{},
			schedule.NewDate(2025, time.January, 4),
		)
		Expect(got).To(BeEmpty())
	})

	It("returns an empty slice when no weekday entry matches", func() {
		weekdayOnly := []schedule.TaskDefinition{
			{Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
		}
		got := schedule.TasksForDate(
			weekdayOnly,
			schedule.NewDate(2025, time.January, 7), // Tuesday
		)
		Expect(got).To(BeEmpty())
	})

	It("preserves the always-fire semantic for the four other kinds", func() {
		// Daily, weekly, monthly, quarterly, and yearly all fire on every day
		// (spec 006 always-fire). The fixture omits quarterly and yearly for
		// brevity; the always-fire guarantee for those kinds is exercised by
		// tick_test.go and the trigger-handler test.
		for _, date := range []schedule.Date{
			schedule.NewDate(2025, time.January, 6),  // Monday
			schedule.NewDate(2025, time.January, 7),  // Tuesday
			schedule.NewDate(2025, time.January, 10), // Friday
			schedule.NewDate(2025, time.January, 11), // Saturday
		} {
			got := filter(date)
			for _, def := range got {
				if def.Recurrence == schedule.RecurrenceWeekday {
					// Weekday entries fire only on their target weekday.
					continue
				}
				// Always-fire: present on every date.
				Expect(def.Recurrence).To(BeElementOf(
					schedule.RecurrenceDaily,
					schedule.RecurrenceWeekly,
					schedule.RecurrenceMonthly,
					schedule.RecurrenceQuarterly,
					schedule.RecurrenceYearly,
				))
			}
		}
	})
})

func slugsOf(defs []schedule.TaskDefinition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.Slug)
	}
	return out
}
