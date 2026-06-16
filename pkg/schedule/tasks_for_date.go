// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

// TasksForDate returns the subset of the canonical inventory that fires
// on the given civil date. The filter rule is:
//
//   - RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly,
//     RecurrenceQuarterly, RecurrenceYearly: always-fire (the entry
//     fires on every day; this is the spec 006 always-fire semantic,
//     preserved by spec 009).
//   - RecurrenceWeekday: fires only on the day whose weekday equals
//     the entry's Weekday field. Computed via date.Time().Weekday().
//
// The returned slice is a defensive copy — mutating it does NOT affect
// the package-level inventory state. The result is NOT sorted; the
// caller may sort on Slug if a stable order is required (the HTTP
// trigger handler does so for the response body).
//
// Pure function: no I/O, no clock, no env. The Europe/Berlin civil-date
// conversion (and the ISO-week boundary math that goes with it) is the
// caller's responsibility — this function takes a civil Date, not a
// time.Time with a location. The tick (pkg/tick) and the trigger HTTP
// handler (pkg/handler/trigger) both convert their wall-clock input
// to a Europe/Berlin civil Date before calling this function.
//
// An empty inventory yields an empty slice. An inventory that contains
// only RecurrenceWeekday entries whose Weekday does not match the
// given date's weekday also yields an empty slice — this is the
// regression fix from spec 009: weekday-pinned tasks no longer fire
// on a non-target weekday.
func TasksForDate(date Date) []TaskDefinition {
	return filterInventoryByDate(inventory, date)
}

// filterInventoryByDate is the package-internal implementation. It
// exists as a separate function so the synthetic-fixture tests in
// tasks_for_date_test.go can pass small custom inventories without
// touching the package-level inventory. Production callers go through
// TasksForDate (which reads the canonical inventory).
func filterInventoryByDate(defs []TaskDefinition, date Date) []TaskDefinition {
	dateWeekday := date.Time().Weekday()
	out := make([]TaskDefinition, 0, len(defs))
	for _, def := range defs {
		switch def.Recurrence {
		case RecurrenceWeekday:
			if def.Weekday == dateWeekday {
				out = append(out, def)
			}
		default:
			// Daily, Weekly, Monthly, Quarterly, Yearly — always-fire.
			out = append(out, def)
		}
	}
	return out
}
