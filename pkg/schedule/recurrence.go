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
}
