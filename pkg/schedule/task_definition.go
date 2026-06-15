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

	// Weekday is the day of the week the entry is intended for. It is
	// consulted ONLY when Recurrence == RecurrenceWeekly: the publisher
	// appends the lowercase 3-letter weekday abbreviation to the weekly
	// period token (e.g. "2026W25-sat"). For the other four RecurrenceKinds
	// (daily / monthly / quarterly / yearly) Weekday is ignored and MUST
	// remain the zero value (time.Sunday).
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
