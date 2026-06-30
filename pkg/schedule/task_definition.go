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

	// Weekdays is the canonical set of weekdays a RecurrenceWeekday entry
	// fires on. Non-empty for RecurrenceWeekday (one or more days); nil/empty
	// and ignored for every other kind. Produced by the store adapter, which
	// normalizes the CR's string-or-list weekday value (long or short form)
	// to canonical time.Weekday values. The matcher (TasksForDate) fires the
	// entry on any day whose weekday is in this set; the publisher's period
	// token encodes the FIRING day's weekday (guaranteed in this set on a
	// firing day), so a list {Monday,Wednesday,Friday} yields three distinct
	// task files per ISO week, one per matching day.
	Weekdays []time.Weekday

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

	// AutoAbortPrior is the opt-in flag resolved from the CR's
	// spec.autoAbortPrior pointer by the store adapter (nil → false). A plain
	// bool, consistent with PeriodOffset's plain-int style — the schedule
	// layer stays pure data. The publisher mirrors this value onto every
	// materialized task's frontmatter as the `auto_abort_prior` key; the
	// downstream task-controller reads that key as its auto-abort eligibility
	// gate. Default false means a Schedule never opts into auto-abort unless
	// the operator explicitly sets spec.autoAbortPrior: true.
	AutoAbortPrior bool
}

// The closed list of accepted placeholder tokens lives in
// pkg/publisher/placeholders.go as the publisher's `placeholders`
// table (and is exposed as `publisher.SupportedPlaceholders` for any
// caller needing the names). It was relocated out of this pure-data
// package because rendering is a publisher concern; the schedule
// layer carries only operator-authored template strings.
