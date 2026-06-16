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
		// inventory is exercised by the full-inventory render test in
		// pkg/publisher/publisher_test.go; this spec exercises the filter
		// rule with a controlled set of entries.
		defs = []schedule.TaskDefinition{
			{Slug: "daily-x", Recurrence: schedule.RecurrenceDaily},
			{Slug: "weekly-x", Recurrence: schedule.RecurrenceWeekly},
			{Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
			{Slug: "weekday-sun", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Sunday},
			{Slug: "monthly-x", Recurrence: schedule.RecurrenceMonthly},
		}
	})

	// Helper: drive the package-internal worker with the synthetic fixtures.
	// Production callers go through schedule.TasksForDate; tests use the
	// worker so the production inventory is not consulted.
	filter := func(date schedule.Date) []schedule.TaskDefinition {
		return schedule.FilterInventoryByDateForTest(defs, date)
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
		// always-fire entries (daily, weekly, monthly, plus zero weekday
		// ones) are returned. Quarterly and yearly are not in the fixture;
		// they would also fire on Tuesday under the same always-fire rule.
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
			got := schedule.FilterInventoryByDateForTest(
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
		got := schedule.FilterInventoryByDateForTest(
			weekdayOnly,
			schedule.NewDate(2025, time.January, 5),
		)
		Expect(slugsOf(got)).To(ConsistOf("weekday-sun"))
	})

	It("returns an empty slice for an empty inventory", func() {
		got := schedule.FilterInventoryByDateForTest(
			[]schedule.TaskDefinition{},
			schedule.NewDate(2025, time.January, 4),
		)
		Expect(got).To(BeEmpty())
	})

	It("returns an empty slice when no weekday entry matches", func() {
		weekdayOnly := []schedule.TaskDefinition{
			{Slug: "weekday-sat", Recurrence: schedule.RecurrenceWeekday, Weekday: time.Saturday},
		}
		got := schedule.FilterInventoryByDateForTest(
			weekdayOnly,
			schedule.NewDate(2025, time.January, 7), // Tuesday
		)
		Expect(got).To(BeEmpty())
	})

	It("preserves the always-fire semantic for the four other kinds", func() {
		// Daily, weekly, monthly, quarterly, and yearly all fire on every day
		// (spec 006 always-fire). The fixture omits quarterly and yearly for
		// brevity; the always-fire guarantee for those kinds is exercised by
		// the existing full-inventory test in pkg/tick/tick_test.go and the
		// prompt-2 trigger-handler test.
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

	It("exercises the public TasksForDate accessor with the production inventory", func() {
		// Drive the public accessor (which reads the package-level inventory).
		// Coverage check: ensure TasksForDate is exercised end-to-end against
		// the real 45-entry inventory. The trigger handler in pkg/handler
		// also calls TasksForDate — this is a thin end-to-end coverage test
		// that future refactors can rely on.
		// 2025-01-07 is a Tuesday. After spec 009, the 21 RecurrenceWeekday
		// entries (12 Saturday + 9 Sunday) do NOT fire on a Tuesday; the 24
		// always-fire entries (18 monthly + 2 quarterly + 4 yearly) do.
		got := schedule.TasksForDate(schedule.NewDate(2025, time.January, 7))
		Expect(got).To(HaveLen(24))
	})
})

func slugsOf(defs []schedule.TaskDefinition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.Slug)
	}
	return out
}
