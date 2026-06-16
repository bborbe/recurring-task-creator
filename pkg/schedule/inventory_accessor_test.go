// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("Inventory accessor", func() {
	It("returns a defensive copy of the canonical inventory", func() {
		inv := schedule.Inventory()
		Expect(inv).To(HaveLen(45))
		// Mutating the returned slice MUST NOT affect subsequent calls.
		inv[0].Slug = "corrupted"
		Expect(schedule.Inventory()[0].Slug).NotTo(Equal("corrupted"))
	})
})

var _ = Describe("Date.IsZero", func() {
	It("returns true for the zero value", func() {
		Expect(schedule.Date{}.IsZero()).To(BeTrue())
	})

	It("returns false for a non-zero date", func() {
		Expect(schedule.NewDate(2025, 1, 1).IsZero()).To(BeFalse())
	})
})
