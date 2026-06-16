// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// periodTitlePlaceholders is the exact set of placeholders that spec 008
// stripped from TitleTemplate values. The publisher's automatic
// `<bare> - <period-token>` suffix (added in Prompt 1) replaces them.
// TitleTemplate entries MUST NOT contain any of these placeholders
// (failure mode row 1 of spec 008); BodyTemplate entries MAY still contain
// them per the spec's Desired Behavior 5 and the schedule placeholder-
// support contract. The list is closed: adding a new period-style
// placeholder is a new spec.
var periodTitlePlaceholders = []string{
	"{{iso-week}}",
	"{{next-iso-week}}",
	"{{month}}",
	"{{last-month}}",
	"{{quarter}}",
	"{{last-quarter}}",
	"{{year}}",
	"{{last-year}}",
}

var _ = Describe("inventory", func() {
	It("has unique slugs", func() {
		seen := map[string]int{}
		for _, def := range schedule.AllDefinitionsForTest() {
			seen[def.Slug]++
		}
		for slug, n := range seen {
			Expect(n).To(Equal(1), "slug %q appears %d times", slug, n)
		}
	})

	It("uses only supported placeholders in TitleTemplate and BodyTemplate", func() {
		tokenRE := regexp.MustCompile(`\{\{[^}]+\}\}`)
		supported := map[string]bool{}
		for _, p := range schedule.SupportedPlaceholders {
			supported[p] = true
		}
		for _, def := range schedule.AllDefinitionsForTest() {
			for _, tok := range tokenRE.FindAllString(def.TitleTemplate, -1) {
				Expect(supported).To(HaveKey(tok),
					"entry %q uses unsupported placeholder %q in TitleTemplate", def.Slug, tok)
			}
			for _, tok := range tokenRE.FindAllString(def.BodyTemplate, -1) {
				Expect(supported).To(HaveKey(tok),
					"entry %q uses unsupported placeholder %q in BodyTemplate", def.Slug, tok)
			}
		}
	})

	It("uses recurrence kinds from the closed set", func() {
		allowed := map[schedule.RecurrenceKind]bool{
			schedule.RecurrenceDaily:     true,
			schedule.RecurrenceWeekly:    true,
			schedule.RecurrenceWeekday:   true,
			schedule.RecurrenceMonthly:   true,
			schedule.RecurrenceQuarterly: true,
			schedule.RecurrenceYearly:    true,
		}
		for _, def := range schedule.AllDefinitionsForTest() {
			Expect(allowed).To(HaveKey(def.Recurrence),
				"entry %q has unknown Recurrence %q", def.Slug, def.Recurrence)
		}
	})

	It("inventory contains exactly 12 Saturday RecurrenceWeekday entries", func() {
		n := 0
		for _, def := range schedule.AllDefinitionsForTest() {
			if def.Recurrence == schedule.RecurrenceWeekday && def.Weekday == time.Saturday {
				n++
			}
		}
		Expect(n).To(Equal(12),
			"expected 12 RecurrenceWeekday entries with Weekday=time.Saturday, got %d", n)
	})

	It("inventory contains exactly 9 Sunday RecurrenceWeekday entries", func() {
		n := 0
		for _, def := range schedule.AllDefinitionsForTest() {
			if def.Recurrence == schedule.RecurrenceWeekday && def.Weekday == time.Sunday {
				n++
			}
		}
		Expect(n).To(Equal(9),
			"expected 9 RecurrenceWeekday entries with Weekday=time.Sunday, got %d", n)
	})

	It("inventory contains zero RecurrenceWeekly entries", func() {
		n := 0
		for _, def := range schedule.AllDefinitionsForTest() {
			if def.Recurrence == schedule.RecurrenceWeekly {
				n++
			}
		}
		Expect(n).To(Equal(0),
			"expected 0 RecurrenceWeekly entries after spec 009, got %d", n)
	})

	It("has no period placeholders in any TitleTemplate", func() {
		// After spec 008, the eight period-style placeholders (periodTitlePlaceholders)
		// are replaced by the publisher's automatic title-suffix. A TitleTemplate that
		// still contains one of them would render as "Foo 2026W01 - 2026W01-sat" — a
		// double-token shape that no inventory entry intends. The publisher's
		// strings.TrimSpace hides the visible bug at render time, but the data invariant
		// is broken. This spec catches it at build time.
		for _, def := range schedule.AllDefinitionsForTest() {
			trimmed := strings.TrimSpace(def.TitleTemplate)
			for _, ph := range periodTitlePlaceholders {
				Expect(strings.Contains(trimmed, ph)).To(BeFalse(),
					"entry %q TitleTemplate %q still contains period placeholder %q; "+
						"spec 008 strips these from TitleTemplate (the publisher's suffix replaces them)",
					def.Slug, def.TitleTemplate, ph)
			}
		}
	})

	It("has a non-empty TitleTemplate for every entry", func() {
		// After spec 008's placeholder stripping, a sloppy edit could empty an
		// entry's TitleTemplate. The publisher's strings.TrimSpace + " - " + suffix
		// logic would render such an entry as just " - 2026-06" — useless to the
		// user. Catch it at build time.
		for _, def := range schedule.AllDefinitionsForTest() {
			Expect(strings.TrimSpace(def.TitleTemplate)).NotTo(BeEmpty(),
				"entry %q has empty TitleTemplate; spec 008 requires a non-empty bare title", def.Slug)
		}
	})
})
