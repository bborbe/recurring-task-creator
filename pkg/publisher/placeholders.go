// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"time"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// placeholder is one entry in the declarative placeholder table.
// name is the literal token as it appears in a template; compute is
// the pure function rendering that token for a given date.
type placeholder struct {
	name    string
	compute func(date schedule.Date) string
}

// placeholders is the closed, ordered table of every placeholder
// accepted in title, body, and string-valued frontmatter entries.
// Adding a placeholder = adding one row; renderTemplate and
// SupportedPlaceholders both derive from this slice — single source
// of truth.
//
// `{{next_sat_date}}` / `{{next_sun_date}}` return *today* when today's
// weekday equals the target (delta 0); they return the following
// occurrence (delta 1-6) otherwise. Rationale: a Sunday Schedule firing
// on Sun should stamp `planned_date: <today>`, not `<+7 days>`.
//
// The pre-v0.2.0 kebab-case alias names (`{{date}}`, `{{iso-week}}`,
// `{{next-iso-week}}`, `{{month}}`, `{{last-month}}`, `{{quarter}}`,
// `{{last-quarter}}`, `{{year}}`, `{{last-year}}`) were removed in
// v0.3.0 after every operator-owned Schedule CR migrated to the
// canonical names.
var placeholders = []placeholder{
	// Date
	{"{{current_date}}", currentDate},
	{"{{next_sat_date}}", nextSatDate},
	{"{{next_sun_date}}", nextSunDate},
	// Week
	{"{{current_week}}", currentWeek},
	{"{{next_week}}", nextWeek},
	// Month
	{"{{current_month}}", currentMonth},
	{"{{next_month}}", nextMonth},
	{"{{last_month}}", lastMonth},
	// Quarter
	{"{{current_quarter}}", currentQuarter},
	{"{{last_quarter}}", lastQuarter},
	// Year
	{"{{current_year}}", currentYear},
	{"{{next_year}}", nextYear},
	{"{{last_year}}", lastYear},
}

// SupportedPlaceholders is the ordered list of every placeholder name
// accepted in title, body, and string-valued frontmatter. Derived from
// the placeholders table. Exposed for callers (inventory validators,
// docs generators) that need the closed-enum list.
var SupportedPlaceholders = func() []string {
	names := make([]string, len(placeholders))
	for i, p := range placeholders {
		names[i] = p.name
	}
	return names
}()

// --- compute functions ---

func currentDate(d schedule.Date) string {
	return fmtDate(d.Year, int(d.Month), d.Day)
}

func currentWeek(d schedule.Date) string {
	year, week := d.Time().ISOWeek()
	return fmtIsoWeek(year, week)
}

func nextWeek(d schedule.Date) string {
	year, week := d.Time().AddDate(0, 0, 7).ISOWeek()
	return fmtIsoWeek(year, week)
}

func currentMonth(d schedule.Date) string {
	t := d.Time()
	return fmtMonthYear(t.Year(), int(t.Month()))
}

func lastMonth(d schedule.Date) string {
	t := firstOfPreviousMonth(d.Time())
	return fmtMonthYear(t.Year(), int(t.Month()))
}

func nextMonth(d schedule.Date) string {
	t := firstOfNextMonth(d.Time())
	return fmtMonthYear(t.Year(), int(t.Month()))
}

func currentQuarter(d schedule.Date) string {
	t := d.Time()
	return fmtQuarter(t.Year(), quarterOf(t.Month()))
}

func lastQuarter(d schedule.Date) string {
	year, q := previousQuarter(d.Time().Year(), int(d.Time().Month()))
	return fmtQuarter(year, q)
}

func currentYear(d schedule.Date) string { return fmtYear(d.Time().Year()) }

func nextYear(d schedule.Date) string { return fmtYear(d.Time().Year() + 1) }

func lastYear(d schedule.Date) string { return fmtYear(d.Time().Year() - 1) }

func nextSatDate(d schedule.Date) string { return fmtDateT(nextWeekday(d.Time(), time.Saturday)) }

func nextSunDate(d schedule.Date) string { return fmtDateT(nextWeekday(d.Time(), time.Sunday)) }

// nextWeekday returns the next time.Time whose weekday equals target.
// Returns base itself when base's weekday IS target (delta 0); returns
// base+1..base+6 otherwise. Inclusive-today semantic: a Sunday
// Schedule firing on Sun gets <today> for `{{next_sun_date}}`, not
// <today+7>.
func nextWeekday(base time.Time, target time.Weekday) time.Time {
	delta := (int(target) - int(base.Weekday()) + 7) % 7
	return base.AddDate(0, 0, delta)
}

// firstOfNextMonth returns the first day of the calendar month after base.
func firstOfNextMonth(base time.Time) time.Time {
	y, m, _ := base.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
}

// fmtDateT renders a time.Time as YYYY-MM-DD.
func fmtDateT(t time.Time) string {
	y, m, d := t.Date()
	return fmtDate(y, int(m), d)
}
