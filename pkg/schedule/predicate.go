// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import "time"

// predicate decides whether an inventory entry fires on a given civil date.
type predicate func(d Date) bool

// OnWeekdays returns a predicate that fires when d.weekday() is in the given set.
func OnWeekdays(days ...time.Weekday) predicate {
	set := make(map[time.Weekday]bool, len(days))
	for _, d := range days {
		set[d] = true
	}
	return func(d Date) bool {
		return set[d.weekday()]
	}
}

// OnDaysOfMonth returns a predicate that fires when d.Day is in the given set.
func OnDaysOfMonth(days ...int) predicate {
	set := make(map[int]bool, len(days))
	for _, d := range days {
		set[d] = true
	}
	return func(d Date) bool {
		return set[d.Day]
	}
}

// OnMonthAndDay returns a predicate that fires when d.Month == month && d.Day == day.
func OnMonthAndDay(month time.Month, day int) predicate {
	return func(d Date) bool {
		return d.Month == month && d.Day == day
	}
}

// EveryDay returns a predicate that always fires.
func EveryDay() predicate {
	return func(d Date) bool {
		return true
	}
}

// OnFirstDayOfQuarter returns a predicate that fires on the first day of Jan/Apr/Jul/Oct.
func OnFirstDayOfQuarter() predicate {
	return func(d Date) bool {
		return d.Day == 1 && (d.Month == time.January ||
			d.Month == time.April ||
			d.Month == time.July ||
			d.Month == time.October)
	}
}

// OnFirstDayOfYear returns a predicate that fires on January 1st.
func OnFirstDayOfYear() predicate {
	return func(d Date) bool {
		return d.Month == time.January && d.Day == 1
	}
}

// OnFirstDayOfMonth returns a predicate that fires when d.Day == 1.
func OnFirstDayOfMonth() predicate {
	return func(d Date) bool {
		return d.Day == 1
	}
}
