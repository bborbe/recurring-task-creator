// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("placeholders table", func() {
	// 2026-06-20 is a Saturday; ISO 2026W25; June Q2 2026.
	saturday := schedule.NewDate(2026, time.June, 20)
	// 2026-06-21 is a Sunday; same ISO week.
	sunday := schedule.NewDate(2026, time.June, 21)
	// 2026-12-31 is a Thursday; ISO 2026W53; Q4; rolls year/month/quarter
	// boundaries for the *_next_* checks.
	yearEnd := schedule.NewDate(2026, time.December, 31)

	DescribeTable(
		"canonical names render correctly for a representative Saturday",
		func(placeholder, expected string) {
			out := publisher.NewRenderer().Render(placeholder, "test-slug", saturday)
			Expect(out).To(Equal(expected))
		},
		// Date
		Entry("current_date on Sat", "{{current_date}}", "2026-06-20"),
		Entry("next_sat_date on Sat returns today", "{{next_sat_date}}", "2026-06-20"),
		Entry("next_sun_date on Sat returns tomorrow", "{{next_sun_date}}", "2026-06-21"),
		// Week
		Entry("current_week", "{{current_week}}", "2026W25"),
		Entry("next_week", "{{next_week}}", "2026W26"),
		// Month
		Entry("current_month", "{{current_month}}", "2026-06"),
		Entry("next_month", "{{next_month}}", "2026-07"),
		Entry("last_month", "{{last_month}}", "2026-05"),
		// Quarter
		Entry("current_quarter", "{{current_quarter}}", "2026Q2"),
		Entry("last_quarter", "{{last_quarter}}", "2026Q1"),
		// Year
		Entry("current_year", "{{current_year}}", "2026"),
		Entry("next_year", "{{next_year}}", "2027"),
		Entry("last_year", "{{last_year}}", "2025"),
	)

	DescribeTable(
		"next_sat_date / next_sun_date are inclusive-today",
		func(date schedule.Date, placeholder, expected string) {
			Expect(publisher.NewRenderer().Render(placeholder, "s", date)).To(Equal(expected))
		},
		Entry("next_sat_date on Sat returns today", saturday, "{{next_sat_date}}", "2026-06-20"),
		Entry("next_sun_date on Sun returns today", sunday, "{{next_sun_date}}", "2026-06-21"),
		Entry("next_sat_date on Sun returns Sat+6", sunday, "{{next_sat_date}}", "2026-06-27"),
		Entry("next_sun_date on Sat returns Sun+1", saturday, "{{next_sun_date}}", "2026-06-21"),
	)

	DescribeTable(
		"period rollover at year end",
		func(placeholder, expected string) {
			Expect(publisher.NewRenderer().Render(placeholder, "s", yearEnd)).To(Equal(expected))
		},
		Entry("next_month rolls to next year", "{{next_month}}", "2027-01"),
		Entry("next_year increments", "{{next_year}}", "2027"),
		Entry("current_quarter Q4", "{{current_quarter}}", "2026Q4"),
	)

	DescribeTable(
		"backward-compat aliases render same value as canonical names",
		func(alias, canonical string) {
			aliasOut := publisher.NewRenderer().Render(alias, "s", saturday)
			canonicalOut := publisher.NewRenderer().Render(canonical, "s", saturday)
			Expect(aliasOut).To(Equal(canonicalOut))
			Expect(aliasOut).NotTo(BeEmpty())
		},
		Entry("date == current_date", "{{date}}", "{{current_date}}"),
		Entry("iso-week == current_week", "{{iso-week}}", "{{current_week}}"),
		Entry("next-iso-week == next_week", "{{next-iso-week}}", "{{next_week}}"),
		Entry("month == current_month", "{{month}}", "{{current_month}}"),
		Entry("last-month == last_month", "{{last-month}}", "{{last_month}}"),
		Entry("quarter == current_quarter", "{{quarter}}", "{{current_quarter}}"),
		Entry("last-quarter == last_quarter", "{{last-quarter}}", "{{last_quarter}}"),
		Entry("year == current_year", "{{year}}", "{{current_year}}"),
		Entry("last-year == last_year", "{{last-year}}", "{{last_year}}"),
	)

	Describe("SupportedPlaceholders", func() {
		It("derives from the table — 13 canonical + 9 aliases = 22 entries", func() {
			Expect(publisher.SupportedPlaceholders).To(HaveLen(22))
		})

		It("contains every canonical name", func() {
			canonicals := []string{
				"{{current_date}}",
				"{{next_sat_date}}",
				"{{next_sun_date}}",
				"{{current_week}}",
				"{{next_week}}",
				"{{current_month}}",
				"{{next_month}}",
				"{{last_month}}",
				"{{current_quarter}}",
				"{{last_quarter}}",
				"{{current_year}}",
				"{{next_year}}",
				"{{last_year}}",
			}
			for _, name := range canonicals {
				Expect(publisher.SupportedPlaceholders).To(ContainElement(name))
			}
		})

		It("contains every backward-compat alias", func() {
			aliases := []string{
				"{{date}}",
				"{{iso-week}}",
				"{{next-iso-week}}",
				"{{month}}",
				"{{last-month}}",
				"{{quarter}}",
				"{{last-quarter}}",
				"{{year}}",
				"{{last-year}}",
			}
			for _, name := range aliases {
				Expect(publisher.SupportedPlaceholders).To(ContainElement(name))
			}
		})
	})

	Describe("renderTemplate substitution semantics", func() {
		It("substitutes multiple placeholders in one string", func() {
			out := publisher.NewRenderer().Render(
				"week {{current_week}} ends on Sun {{next_sun_date}}",
				"s", saturday,
			)
			Expect(out).To(Equal("week 2026W25 ends on Sun 2026-06-21"))
		})

		It("leaves strings without placeholders unchanged", func() {
			out := publisher.NewRenderer().Render("no placeholders here", "s", saturday)
			Expect(out).To(Equal("no placeholders here"))
		})

		It("ignores tokens that look like placeholders but are not in the table", func() {
			out := publisher.NewRenderer().Render("{{unknown_token}}", "s", saturday)
			Expect(out).To(Equal("{{unknown_token}}"))
		})
	})
})
