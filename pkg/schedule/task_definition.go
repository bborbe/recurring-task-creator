// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import "time"

// Frontmatter is operator-defined YAML frontmatter stamped onto the
// generated vault file. Structurally identical to `bborbe/agent/lib`'s
// `TaskFrontmatter` (`map[string]interface{}`) but declared locally so
// the pure-data layer here doesn't pull in the agent module just to
// name a map type. The publisher converts to/from `lib.TaskFrontmatter`
// at its package boundary.
type Frontmatter = map[string]interface{}

// TaskDefinition is one entry in the recurring-task inventory.
type TaskDefinition struct {
	// Slug is a stable, kebab-case identifier unique across the inventory.
	// Once committed, a slug rename is a breaking change to the future Kafka
	// stream and requires a separate spec.
	Slug string

	// TitleTemplate is the title shown to the user. Supports only the
	// placeholders listed in publisher.SupportedPlaceholders (declared
	// in pkg/publisher/placeholders.go).
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

	// Frontmatter is operator-defined YAML frontmatter stamped onto the
	// generated vault file. Sourced from the `spec.template.frontmatter`
	// field on the Schedule CR (free-form map[string]interface{}). The
	// publisher seeds two defaults (`status: in_progress`,
	// `page_type: task`) and lets operator keys override them on
	// collision. `created_by: recurring-task-creator` is force-set as
	// provenance and cannot be overridden by configuration.
	Frontmatter Frontmatter

	// PeriodOffset shifts the period-anchored token by N periods. Default 0
	// (current period). Only meaningful for RecurrenceMonthly /
	// RecurrenceQuarterly / RecurrenceYearly — the CRD's CEL rule rejects
	// non-zero values for the date-anchored kinds (Daily / Weekly / Weekday).
	// Negative offsets name a prior period; the publisher's buildPeriodToken
	// applies the shift to the fire date before formatting the token.
	PeriodOffset int
}

// The closed list of accepted placeholder tokens lives in
// pkg/publisher/placeholders.go as the publisher's `placeholders`
// table (and is exposed as `publisher.SupportedPlaceholders` for any
// caller needing the names). It was relocated out of this pure-data
// package because rendering is a publisher concern; the schedule
// layer carries only operator-authored template strings.
