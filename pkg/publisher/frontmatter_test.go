// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"time"

	lib "github.com/bborbe/agent/lib"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
			fm := f.Format(lib.TaskFrontmatter{}, "test-slug", date)
			Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
			Expect(fm).To(HaveKeyWithValue("page_type", "task"))
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
			Expect(fm).To(HaveLen(3))
		})

		It("force-sets created_by even when operator tries to override it", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"created_by": "impersonator"},
				"test-slug", date,
			)
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
		})

		It("lets operator override status + page_type defaults", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"status": "draft", "page_type": "log"},
				"test-slug", date,
			)
			Expect(fm).To(HaveKeyWithValue("status", "draft"))
			Expect(fm).To(HaveKeyWithValue("page_type", "log"))
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
		})
	})

	Describe("placeholder rendering in string values", func() {
		DescribeTable(
			"every supported placeholder renders",
			func(key, placeholder, expected string) {
				fm := f.Format(
					lib.TaskFrontmatter{key: placeholder},
					"test-slug", date,
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
				"test-slug", date,
			)
			Expect(fm).To(HaveKeyWithValue("note", "due by 2026-06-20 (week 2026W25)"))
		})

		It("leaves strings without placeholders unchanged", func() {
			fm := f.Format(
				lib.TaskFrontmatter{"assignee": "alice", "category": "ops"},
				"test-slug", date,
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
				"test-slug", date,
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
			fm1 := f.Format(input, "test-slug", date)
			fm2 := f.Format(input, "test-slug", date)
			Expect(fm1).To(Equal(fm2))
		})
	})
})
