// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"time"

	lib "github.com/bborbe/agent"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/publisher-frontmatter-formatter.go --fake-name PublisherFrontmatterFormatter . FrontmatterFormatter

// FrontmatterFormatter builds the YAML frontmatter stamped onto every
// published task. The formatter is the single seam between operator-
// supplied frontmatter (from `Schedule.spec.template.frontmatter`) and
// the wire-level `task.CreateCommand.Frontmatter` payload: it seeds
// published-author defaults, renders placeholder tokens in string
// values via the publisher's closed placeholder set, merges operator
// keys (which may override the defaults), and force-sets the provenance
// key `created_by: recurring-task-creator` last so a Schedule CR cannot
// impersonate a different author.
type FrontmatterFormatter interface {
	// Format returns the frontmatter for one published task. String
	// values in `operator` are rendered through the same placeholder
	// substitution as title/body (`{{date}}`, `{{iso-week}}`, etc.);
	// non-string values (ints, slices, maps) pass through unchanged.
	// `slug` and `date` parameterize the placeholder render; `date` is
	// the Berlin civil date the task fires for (the publisher converts
	// wall-clock once at the tick boundary).
	//
	// `recurrence` drives the computed `defer_date` stamp: the earliest
	// date the task should surface. Point-shaped kinds (Daily, Weekday,
	// OnDate) defer to the fire date itself; span-shaped kinds defer to the
	// start of their period — Monday of the firing ISO week for Weekly, 1st
	// of the firing month/quarter/year for the period-anchored kinds.
	// PeriodOffset is NOT applied (the offset=-1 review schedules fire
	// throughout the period they review, so the firing period's start is
	// the correct surface date). `defer_date` is stamped AFTER operator
	// keys are merged (an operator-supplied `defer_date` cannot override
	// the computed value) and BEFORE `auto_abort_prior`/`created_by` are
	// force-set. The value is an ISO "YYYY-MM-DD" string.
	//
	// `autoAbortPrior` is stamped onto the result as the
	// `auto_abort_prior` key, AFTER operator keys are merged (so an
	// operator-supplied `auto_abort_prior` cannot override the spec-level
	// value) and BEFORE `created_by` is force-set (so `created_by` stays
	// the last provenance key). The value is a Go bool serialized as a
	// YAML boolean true/false.
	Format(
		operator lib.TaskFrontmatter,
		slug string,
		date schedule.Date,
		autoAbortPrior bool,
		recurrence schedule.RecurrenceKind,
	) lib.TaskFrontmatter
}

// NewFrontmatterFormatter returns the default FrontmatterFormatter that
// renders string-valued frontmatter via the injected Renderer (same
// placeholder semantics as title/body). Stateless: safe to construct
// once and share across goroutines.
func NewFrontmatterFormatter(renderer Renderer) FrontmatterFormatter {
	return &frontmatterFormatter{renderer: renderer}
}

type frontmatterFormatter struct {
	renderer Renderer
}

func (f *frontmatterFormatter) Format(
	operator lib.TaskFrontmatter,
	slug string,
	date schedule.Date,
	autoAbortPrior bool,
	recurrence schedule.RecurrenceKind,
) lib.TaskFrontmatter {
	out := lib.TaskFrontmatter{
		"status":    "in_progress",
		"page_type": "task",
	}
	for k, v := range operator {
		if s, ok := v.(string); ok {
			out[k] = f.renderer.Render(s, slug, date)
			continue
		}
		out[k] = v
	}
	out["defer_date"] = deferDateFor(recurrence, date)
	out["auto_abort_prior"] = autoAbortPrior
	out["created_by"] = "recurring-task-creator"
	return out
}

// deferDateFor returns the period-start date for a (recurrence, date) pair,
// as an ISO "YYYY-MM-DD" string. Point-shaped kinds (Daily, Weekday,
// OnDate) defer to the fire date itself; span-shaped kinds defer to the
// start of their period — Monday of the firing ISO week for Weekly, 1st of
// the firing month/quarter/year for the period-anchored kinds. PeriodOffset
// is intentionally NOT applied: the offset=-1 review schedules fire
// throughout the period they review, so the firing period's start is the
// correct surface date. Pure function of its inputs — no clock access, so
// the publisher's byte-identical payload invariant holds.
func deferDateFor(recurrence schedule.RecurrenceKind, date schedule.Date) string {
	t := date.Time()
	switch recurrence {
	case schedule.RecurrenceDaily, schedule.RecurrenceWeekday, schedule.RecurrenceOnDate:
		return fmtDate(date.Year, int(date.Month), date.Day)
	case schedule.RecurrenceWeekly:
		return fmtDateT(mondayOfISOWeek(t))
	case schedule.RecurrenceMonthly:
		return fmtDate(t.Year(), int(t.Month()), 1)
	case schedule.RecurrenceQuarterly:
		q := quarterOf(t.Month())
		firstMonth := time.Month((q-1)*3 + 1)
		return fmtDate(t.Year(), int(firstMonth), 1)
	case schedule.RecurrenceYearly:
		return fmtDate(t.Year(), 1, 1)
	default:
		return fmtDate(date.Year, int(date.Month), date.Day)
	}
}

// mondayOfISOWeek returns the Monday of the ISO week containing t.
func mondayOfISOWeek(t time.Time) time.Time {
	// Go's Weekday: Sunday=0 .. Saturday=6. ISO weeks start Monday.
	// Days since Monday = (weekday - Monday + 7) % 7.
	daysSinceMonday := (int(t.Weekday()) - int(time.Monday) + 7) % 7
	return t.AddDate(0, 0, -daysSinceMonday)
}
