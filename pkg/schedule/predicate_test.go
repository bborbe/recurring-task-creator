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

var _ = Describe("OnWeekdays", func() {
	It("fires on Saturday", func() {
		p := schedule.OnWeekdays(time.Saturday)
		Expect(p(schedule.NewDate(2025, time.January, 4))).To(BeTrue())
	})
	It("does not fire on Sunday", func() {
		p := schedule.OnWeekdays(time.Saturday)
		Expect(p(schedule.NewDate(2025, time.January, 5))).To(BeFalse())
	})
	It("fires on any day in the set", func() {
		p := schedule.OnWeekdays(time.Saturday, time.Sunday)
		Expect(p(schedule.NewDate(2025, time.January, 5))).To(BeTrue())
		Expect(p(schedule.NewDate(2025, time.January, 6))).To(BeFalse())
	})
})

var _ = Describe("OnDaysOfMonth", func() {
	It("fires on the given day of month", func() {
		p := schedule.OnDaysOfMonth(5)
		Expect(p(schedule.NewDate(2025, time.March, 5))).To(BeTrue())
	})
	It("does not fire on a different day", func() {
		p := schedule.OnDaysOfMonth(5)
		Expect(p(schedule.NewDate(2025, time.March, 6))).To(BeFalse())
	})
	It("fires on any day in the set", func() {
		p := schedule.OnDaysOfMonth(1, 15)
		Expect(p(schedule.NewDate(2025, time.March, 15))).To(BeTrue())
		Expect(p(schedule.NewDate(2025, time.March, 14))).To(BeFalse())
	})
})

var _ = Describe("OnMonthAndDay", func() {
	It("fires on the exact month and day", func() {
		p := schedule.OnMonthAndDay(time.May, 1)
		Expect(p(schedule.NewDate(2025, time.May, 1))).To(BeTrue())
	})
	It("does not fire on the same day in a different month", func() {
		p := schedule.OnMonthAndDay(time.May, 1)
		Expect(p(schedule.NewDate(2025, time.April, 1))).To(BeFalse())
	})
	It("does not fire on a different day of the same month", func() {
		p := schedule.OnMonthAndDay(time.May, 1)
		Expect(p(schedule.NewDate(2025, time.May, 2))).To(BeFalse())
	})
})

var _ = Describe("EveryDay", func() {
	It("fires on every date", func() {
		p := schedule.EveryDay()
		Expect(p(schedule.NewDate(2025, time.January, 1))).To(BeTrue())
		Expect(p(schedule.NewDate(2025, time.July, 15))).To(BeTrue())
	})
})

var _ = Describe("OnFirstDayOfQuarter", func() {
	It("fires on first day of January", func() {
		p := schedule.OnFirstDayOfQuarter()
		Expect(p(schedule.NewDate(2025, time.January, 1))).To(BeTrue())
	})
	It("fires on first day of April", func() {
		p := schedule.OnFirstDayOfQuarter()
		Expect(p(schedule.NewDate(2025, time.April, 1))).To(BeTrue())
	})
	It("fires on first day of July", func() {
		p := schedule.OnFirstDayOfQuarter()
		Expect(p(schedule.NewDate(2025, time.July, 1))).To(BeTrue())
	})
	It("fires on first day of October", func() {
		p := schedule.OnFirstDayOfQuarter()
		Expect(p(schedule.NewDate(2025, time.October, 1))).To(BeTrue())
	})
	It("does not fire on first day of February", func() {
		p := schedule.OnFirstDayOfQuarter()
		Expect(p(schedule.NewDate(2025, time.February, 1))).To(BeFalse())
	})
	It("does not fire on day 2 of a quarter month", func() {
		p := schedule.OnFirstDayOfQuarter()
		Expect(p(schedule.NewDate(2025, time.April, 2))).To(BeFalse())
	})
})

var _ = Describe("OnFirstDayOfYear", func() {
	It("fires on January 1", func() {
		p := schedule.OnFirstDayOfYear()
		Expect(p(schedule.NewDate(2025, time.January, 1))).To(BeTrue())
	})
	It("does not fire on February 1", func() {
		p := schedule.OnFirstDayOfYear()
		Expect(p(schedule.NewDate(2025, time.February, 1))).To(BeFalse())
	})
	It("does not fire on January 2", func() {
		p := schedule.OnFirstDayOfYear()
		Expect(p(schedule.NewDate(2025, time.January, 2))).To(BeFalse())
	})
})

var _ = Describe("OnFirstDayOfMonth", func() {
	It("fires on day 1 of any month", func() {
		p := schedule.OnFirstDayOfMonth()
		Expect(p(schedule.NewDate(2025, time.March, 1))).To(BeTrue())
		Expect(p(schedule.NewDate(2025, time.January, 1))).To(BeTrue())
	})
	It("does not fire on day 2", func() {
		p := schedule.OnFirstDayOfMonth()
		Expect(p(schedule.NewDate(2025, time.March, 2))).To(BeFalse())
	})
})
