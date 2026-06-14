// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

// RecurrenceKind classifies how often an entry repeats. Closed set.
type RecurrenceKind string

const (
	RecurrenceDaily     RecurrenceKind = "daily"
	RecurrenceWeekly    RecurrenceKind = "weekly"
	RecurrenceMonthly   RecurrenceKind = "monthly"
	RecurrenceQuarterly RecurrenceKind = "quarterly"
	RecurrenceYearly    RecurrenceKind = "yearly"
)
