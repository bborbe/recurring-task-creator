// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"time"

	lib "github.com/bborbe/agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("FrontmatterFormatter", func() {
	var (
		f    publisher.FrontmatterFormatter
		date schedule.Date
	)
	BeforeEach(func() {
		f = publisher.NewFrontmatterFormatter(publisher.NewRenderer())
		date = schedule.NewDate(2026, time.June, 20)
	})

	Describe("defaults + provenance", func() {
		It("seeds status=in_progress and page_type=task when operator supplies nothing", func() {
			fm := f.Format(
				lib.TaskFrontmatter{},
				"test-slug",
				date,
				false,
				schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
			Expect(fm).To(HaveKeyWithValue("page_type", "task"))
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
			Expect(fm).To(HaveKeyWithValue("auto_abort_prior", false))
			Expect(fm).To(HaveKeyWithValue("defer_date", "2026-06-20"))
			Expect(fm).To(HaveLen(5))
		})

		It("force-sets created_by even when operator tries to override it", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"created_by": "impersonator"},
				"test-slug", date, false, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
		})

		It("lets operator override status + page_type defaults", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"status": "draft", "page_type": "log"},
				"test-slug", date, false, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("status", "draft"))
			Expect(fm).To(HaveKeyWithValue("page_type", "log"))
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
		})
	})

	Describe("auto_abort_prior stamp", func() {
		It("stamps auto_abort_prior=false when the flag is false", func() {
			fm := f.Format(lib.TaskFrontmatter{}, "slug", date, false, schedule.RecurrenceDaily)
			Expect(fm).To(HaveKeyWithValue("auto_abort_prior", false))
		})

		It("stamps auto_abort_prior=true when the flag is true", func() {
			fm := f.Format(lib.TaskFrontmatter{}, "slug", date, true, schedule.RecurrenceDaily)
			Expect(fm).To(HaveKeyWithValue("auto_abort_prior", true))
		})

		It("ignores an operator-supplied auto_abort_prior; spec-level value wins", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"auto_abort_prior": true},
				"slug", date, false, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("auto_abort_prior", false))
		})

		It("force-sets created_by last, after auto_abort_prior", func() {
			fm := f.Format(
				lib.TaskFrontmatter{
					"auto_abort_prior": true,
					"created_by":       "impersonator",
				},
				"slug", date, true, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("auto_abort_prior", true))
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
		})

		It("auto_abort_prior round-trips through YAML as a boolean, not a string", func() {
			fm := f.Format(lib.TaskFrontmatter{}, "slug", date, true, schedule.RecurrenceDaily)
			raw, err := yaml.Marshal(map[string]interface{}(fm))
			Expect(err).NotTo(HaveOccurred())
			var back map[string]interface{}
			Expect(yaml.Unmarshal(raw, &back)).To(Succeed())
			v, ok := back["auto_abort_prior"]
			Expect(ok).To(BeTrue())
			_, isString := v.(string)
			Expect(isString).To(BeFalse(), "auto_abort_prior must not round-trip as a string")
			Expect(v).To(Equal(true))
		})
	})

	Describe("defer_date stamp", func() {
		DescribeTable(
			"stamps the period-start date per recurrence kind",
			func(kind schedule.RecurrenceKind, want string) {
				fm := f.Format(lib.TaskFrontmatter{}, "slug", date, false, kind)
				Expect(fm).To(HaveKeyWithValue("defer_date", want))
			},
			Entry("daily → fire date", schedule.RecurrenceDaily, "2026-06-20"),
			Entry("weekday → fire date", schedule.RecurrenceWeekday, "2026-06-20"),
			Entry("ondate → fire date", schedule.RecurrenceOnDate, "2026-06-20"),
			Entry("hourly → fire date", schedule.RecurrenceHourly, "2026-06-20"),
			Entry(
				"weekly → Monday of the firing ISO week",
				schedule.RecurrenceWeekly,
				"2026-06-15",
			),
			Entry("monthly → 1st of firing month", schedule.RecurrenceMonthly, "2026-06-01"),
			Entry("quarterly → 1st of firing quarter", schedule.RecurrenceQuarterly, "2026-04-01"),
			Entry("yearly → 1st of firing year", schedule.RecurrenceYearly, "2026-01-01"),
		)

		It("ignores an operator-supplied defer_date; computed value wins", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"defer_date": "1999-01-01"},
				"slug", date, false, schedule.RecurrenceMonthly,
			)
			Expect(fm).To(HaveKeyWithValue("defer_date", "2026-06-01"))
		})

		It("defer_date round-trips through YAML as a date string, not a bool", func() {
			fm := f.Format(lib.TaskFrontmatter{}, "slug", date, false, schedule.RecurrenceMonthly)
			raw, err := yaml.Marshal(map[string]interface{}(fm))
			Expect(err).NotTo(HaveOccurred())
			var back map[string]interface{}
			Expect(yaml.Unmarshal(raw, &back)).To(Succeed())
			v, ok := back["defer_date"]
			Expect(ok).To(BeTrue())
			_, isString := v.(string)
			Expect(isString).To(BeTrue(), "defer_date must round-trip as a string")
			Expect(v).To(Equal("2026-06-01"))
		})

		It("weekly fires on a Sunday and defers to the Monday of the same ISO week", func() {
			// 2026-06-21 (Sun) is in ISO week 2026W25, whose Monday is 2026-06-15.
			sun := schedule.NewDate(2026, time.June, 21)
			fm := f.Format(lib.TaskFrontmatter{}, "slug", sun, false, schedule.RecurrenceWeekly)
			Expect(fm).To(HaveKeyWithValue("defer_date", "2026-06-15"))
		})
	})

	Describe("placeholder rendering in string values", func() {
		DescribeTable(
			"every supported placeholder renders",
			func(key, placeholder, expected string) {
				fm := f.Format(
					lib.TaskFrontmatter{key: placeholder},
					"test-slug", date, false, schedule.RecurrenceDaily,
				)
				Expect(fm).To(HaveKeyWithValue(key, expected))
			},
			Entry("date", "planned_date", "{{current_date}}", "2026-06-20"),
			Entry("iso-week", "period_week", "{{current_week}}", "2026W25"),
			Entry("next-iso-week", "next_week", "{{next_week}}", "2026W26"),
			Entry("month", "period_month", "{{current_month}}", "2026-06"),
			Entry("last-month", "previous_month", "{{last_month}}", "2026-05"),
			Entry("quarter", "period_quarter", "{{current_quarter}}", "2026Q2"),
			Entry("last-quarter", "previous_quarter", "{{last_quarter}}", "2026Q1"),
			Entry("year", "period_year", "{{current_year}}", "2026"),
			Entry("last-year", "previous_year", "{{last_year}}", "2025"),
		)

		It("substitutes inside longer strings, not just bare placeholders", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"note": "due by {{current_date}} (week {{current_week}})"},
				"test-slug", date, false, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("note", "due by 2026-06-20 (week 2026W25)"))
		})

		It("leaves strings without placeholders unchanged", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"assignee": "alice", "category": "ops"},
				"test-slug", date, false, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("assignee", "alice"))
			Expect(fm).To(HaveKeyWithValue("category", "ops"))
		})

		It("passes non-string values through unchanged (int, slice, map)", func() {
			fm := f.Format(
				lib.TaskFrontmatter{
					"priority": 4,
					"goals":    []interface{}{"[[Goal A]]", "[[Goal B]]"},
					"meta":     map[string]interface{}{"nested": "value"},
				},
				"test-slug", date, false, schedule.RecurrenceDaily,
			)
			Expect(fm).To(HaveKeyWithValue("priority", 4))
			Expect(fm).To(HaveKeyWithValue("goals", []interface{}{"[[Goal A]]", "[[Goal B]]"}))
			Expect(fm).To(HaveKeyWithValue("meta", map[string]interface{}{"nested": "value"}))
		})
	})

	Describe("determinism", func() {
		It("same input on a second call produces an equal map", func() {
			input := lib.TaskFrontmatter{
				"planned_date": "{{current_date}}",
				"priority":     4,
				"assignee":     "alice",
			}
			fm1 := f.Format(input, "test-slug", date, false, schedule.RecurrenceDaily)
			fm2 := f.Format(input, "test-slug", date, false, schedule.RecurrenceDaily)
			Expect(fm1).To(Equal(fm2))
		})
	})
})
