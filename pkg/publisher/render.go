// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"fmt"
	"time"
)

// The placeholder substitution function previously declared here moved
// to pkg/publisher/renderer.go behind the Renderer interface — Publisher
// and FrontmatterFormatter now both depend on an injected Renderer
// rather than calling a same-package private helper. The format helpers
// (fmtDate / fmtIsoWeek / fmtMonthYear / fmtQuarter / fmtYear and the
// period-arithmetic helpers below) remain here as a leaf-level utility
// set consumed by the placeholders table.

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
