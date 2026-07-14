// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import "github.com/golang/glog"

// TasksForDate returns the subset of defs that fires on the given civil
// date. The caller supplies the definition slice; this function no longer
// reads a package-level inventory. The filter rule is:
//
//   - RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly,
//     RecurrenceQuarterly, RecurrenceYearly: always-fire (the entry
//     fires on every day; this is the spec 006 always-fire semantic,
//     preserved by spec 009).
//   - RecurrenceWeekday: fires only when date.Time().Weekday() is a
//     member of the entry's Weekdays set. An empty Weekdays set never
//     fires (the CRD CEL rule rejects empty lists at apply time).
//   - RecurrenceOnDate: fires only when both the entry's Month and the
//     entry's Day equal the civil date's month and day. A zero Month or
//     Day never fires (the CRD CEL rule rejects zero values at apply time).
//   - Unknown kinds: skipped with a Warning log message — never fired.
//
// The result is NOT sorted; the caller may sort on Slug if a stable
// order is required (the HTTP trigger handler does so for the response
// body).
//
// Pure function: no I/O, no clock, no env. The Europe/Berlin civil-date
// conversion (and the ISO-week boundary math that goes with it) is the
// caller's responsibility — this function takes a civil Date, not a
// time.Time with a location.
//
// An empty defs slice yields an empty slice. A defs slice that contains
// only RecurrenceWeekday entries whose Weekdays set does not include the
// given date's weekday also yields an empty slice.
func TasksForDate(defs []TaskDefinition, date Date) []TaskDefinition {
	return filterInventoryByDate(defs, date)
}

// filterInventoryByDate is the package-internal implementation. It
// exists as a separate function so the synthetic-fixture tests in
// tasks_for_date_test.go can pass small custom inventories without
// touching the package-level inventory. Production callers go through
// TasksForDate (which reads the canonical inventory).
func filterInventoryByDate(defs []TaskDefinition, date Date) []TaskDefinition {
	dateWeekday := date.Time().Weekday()
	dateMonth := date.Time().Month()
	dateDay := date.Time().Day()
	out := make([]TaskDefinition, 0, len(defs))
	for _, def := range defs {
		switch def.Recurrence {
		case RecurrenceWeekday:
			for _, wd := range def.Weekdays {
				if wd == dateWeekday {
					out = append(out, def)
					break
				}
			}
		case RecurrenceOnDate:
			if def.Month == dateMonth && def.Day == dateDay {
				out = append(out, def)
			}
		case RecurrenceDaily,
			RecurrenceWeekly,
			RecurrenceMonthly,
			RecurrenceQuarterly,
			RecurrenceYearly:
			// Always-fire — the entry fires on every civil date.
			out = append(out, def)
		default:
			glog.Warningf(
				"filterInventoryByDate: unknown recurrence kind %q for slug %q — skipping",
				def.Recurrence, def.Slug,
			)
		}
	}
	return out
}
