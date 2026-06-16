// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import "time"

// TaskDefinition is one entry in the recurring-task inventory.
type TaskDefinition struct {
	// Slug is a stable, kebab-case identifier unique across the inventory.
	// Once committed, a slug rename is a breaking change to the future Kafka
	// stream and requires a separate spec.
	Slug string

	// TitleTemplate is the title shown to the user. Supports only the
	// placeholders listed in SupportedPlaceholders below.
	TitleTemplate string

	// BodyTemplate is raw markdown. Supports the same placeholder set.
	BodyTemplate string

	// Recurrence classifies the cadence (daily/weekly/monthly/quarterly/yearly).
	Recurrence RecurrenceKind

	// Weekday is the day of the week the entry is intended for. Its
	// semantics depend on the entry's Recurrence:
	//
	//   - RecurrenceWeekday: REQUIRED (non-zero). The entry fires only on
	//     the day whose weekday equals this value; the publisher appends
	//     the lowercase 3-letter weekday abbreviation to the period token
	//     (e.g. "2026W25-sat"). The disambiguation from RecurrenceWeekly
	//     was introduced in spec 009.
	//
	//   - RecurrenceWeekly: FORBIDDEN (must be the zero value, time.Sunday).
	//     The entry fires on every day inside its ISO week (always-fire
	//     semantic introduced in spec 006); this field is not consulted.
	//     The inventory contains zero RecurrenceWeekly entries after
	//     spec 009 — the kind is reserved for future use.
	//
	//   - RecurrenceDaily / RecurrenceMonthly / RecurrenceQuarterly /
	//     RecurrenceYearly: ignored. May be the zero value or any other
	//     value without effect on firing or rendering.
	Weekday time.Weekday
}

// SupportedPlaceholders lists the EXACT set of placeholders accepted in
// TitleTemplate and BodyTemplate. Any other `{{...}}` token in an inventory
// entry is a build-time test failure.
var SupportedPlaceholders = []string{
	"{{date}}",
	"{{iso-week}}",
	"{{next-iso-week}}",
	"{{month}}",
	"{{last-month}}",
	"{{quarter}}",
	"{{last-quarter}}",
	"{{year}}",
	"{{last-year}}",
}
