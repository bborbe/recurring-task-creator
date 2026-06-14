// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

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
			schedule.RecurrenceMonthly:   true,
			schedule.RecurrenceQuarterly: true,
			schedule.RecurrenceYearly:    true,
		}
		for _, def := range schedule.AllDefinitionsForTest() {
			Expect(allowed).To(HaveKey(def.Recurrence),
				"entry %q has unknown Recurrence %q", def.Slug, def.Recurrence)
		}
	})
})
