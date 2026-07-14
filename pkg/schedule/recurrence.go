// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

// RecurrenceKind classifies how often an entry repeats. Closed set.
type RecurrenceKind string

const (
	RecurrenceDaily     RecurrenceKind = "daily"
	RecurrenceWeekly    RecurrenceKind = "weekly"
	RecurrenceWeekday   RecurrenceKind = "weekday"
	RecurrenceMonthly   RecurrenceKind = "monthly"
	RecurrenceQuarterly RecurrenceKind = "quarterly"
	RecurrenceYearly    RecurrenceKind = "yearly"
	// RecurrenceOnDate fires on one fixed calendar date (Month + Day) every
	// year — e.g. 03-15 for a birthday. Point-shaped match-fire, mirroring
	// how RecurrenceWeekday matches a day-of-week. Its publisher period token
	// is the fire date's 4-digit year ("YYYY"), so replays within a year are
	// idempotent (UUID5 dedup collapses them to one task file).
	RecurrenceOnDate RecurrenceKind = "ondate"
)

// AllRecurrenceKinds is the canonical, closed set of RecurrenceKind values
// in stable declaration order. Consumers that need to iterate over every
// kind (e.g. pre-initializing Prometheus counter label combinations) range
// over this slice — never hand-roll a duplicate slice.
var AllRecurrenceKinds = []RecurrenceKind{
	RecurrenceDaily,
	RecurrenceWeekly,
	RecurrenceWeekday,
	RecurrenceMonthly,
	RecurrenceQuarterly,
	RecurrenceYearly,
	RecurrenceOnDate,
}
