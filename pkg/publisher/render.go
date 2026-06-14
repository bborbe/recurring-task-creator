// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"fmt"
	"strings"
	"time"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// renderTemplate replaces every placeholder in template with its rendered
// value for date. Substitutes the exact slice returned by
// schedule.SupportedPlaceholders — one strings.ReplaceAll per token, in
// slice order. No regex, no template engine. Unknown placeholders cannot
// appear at this layer (Spec 1's inventory validation rejects them at
// test time).
func renderTemplate(template, slug string, date schedule.Date) string {
	values := buildPlaceholderValues(slug, date)
	out := template
	for _, ph := range schedule.SupportedPlaceholders {
		out = strings.ReplaceAll(out, ph, values[ph])
	}
	return out
}

// buildPlaceholderValues returns a map from each supported placeholder to
// its rendered string for date. The map covers every entry in
// schedule.SupportedPlaceholders. The slug parameter is reserved for
// future placeholders that depend on the slug itself; the current set
// (date/iso-week/next-iso-week/month/last-month/quarter/last-quarter/
// year/last-year) does not.
func buildPlaceholderValues(slug string, date schedule.Date) map[string]string {
	_ = slug
	base := dateToTime(date)
	isoYear, isoWeek := base.ISOWeek()
	next := base.AddDate(0, 0, 7)
	nextIsoYear, nextIsoWeek := next.ISOWeek()
	lastMonth := firstOfPreviousMonth(base)
	lastQuarterYear, lastQuarter := previousQuarter(base.Year(), int(base.Month()))
	return map[string]string{
		"{{date}}":          fmtDate(date.Year, int(date.Month), date.Day),
		"{{iso-week}}":      fmtIsoWeek(isoYear, isoWeek),
		"{{next-iso-week}}": fmtIsoWeek(nextIsoYear, nextIsoWeek),
		"{{month}}":         fmtMonthYear(base.Year(), int(base.Month())),
		"{{last-month}}":    fmtMonthYear(lastMonth.Year(), int(lastMonth.Month())),
		"{{quarter}}":       fmtQuarter(base.Year(), quarterOf(base.Month())),
		"{{last-quarter}}":  fmtQuarter(lastQuarterYear, lastQuarter),
		"{{year}}":          fmtYear(base.Year()),
		"{{last-year}}":     fmtYear(base.Year() - 1),
	}
}

// dateToTime exposes schedule.Date's midnight-UTC carrier through a
// publisher-local helper so the publisher can run ISOWeek() and
// AddDate(0, 0, 7) without re-implementing the conversion. The
// midnight-UTC choice is timezone-agnostic for a fixed civil (Y,M,D) —
// see pkg/schedule/date.go.
func dateToTime(d schedule.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// fmtDate renders YYYY-MM-DD.
func fmtDate(year, month, day int) string {
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// fmtIsoWeek renders YYYYWNN (uppercase W, two-digit week with leading zero).
// Matches the source provider's dateToWeek format ("%04dW%02d").
func fmtIsoWeek(year, week int) string {
	return fmt.Sprintf("%04dW%02d", year, week)
}

// fmtMonthYear renders YYYY-MM.
func fmtMonthYear(year, month int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}

// fmtQuarter renders YYYYQN (uppercase Q, single-digit quarter 1-4).
// Matches the source provider's dateToQuarter format ("%dQ%d") and the
// existing vault convention (e.g. "2025Q4", "2026Q1").
func fmtQuarter(year, quarter int) string {
	return fmt.Sprintf("%dQ%d", year, quarter)
}

// fmtYear renders YYYY.
func fmtYear(year int) string {
	return fmt.Sprintf("%04d", year)
}

// quarterOf returns 1..4 for the given month.
func quarterOf(m time.Month) int {
	return (int(m)-1)/3 + 1
}

// firstOfPreviousMonth returns the first day of the calendar month before base.
func firstOfPreviousMonth(base time.Time) time.Time {
	y, m, _ := base.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
}

// previousQuarter returns (year, quarter) for the quarter before (year, month).
// Q1 of any year rolls back to Q4 of the previous year.
func previousQuarter(year, month int) (int, int) {
	q := (month-1)/3 + 1
	if q == 1 {
		return year - 1, 4
	}
	return year, q - 1
}
