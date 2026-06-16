// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import "time"

// Date is a civil date (year, month, day) with no time, location, or zone
// ambiguity in its public surface. It is the only input shape accepted by
// publisher.Publish and the TasksForDate filter.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate constructs a Date from year/month/day.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// IsZero reports whether the Date is the zero value (Year == 0 && Month == 0 && Day == 0).
func (d Date) IsZero() bool {
	return d.Year == 0 && d.Month == 0 && d.Day == 0
}

// toTime returns the date as midnight UTC. Used internally for Weekday and
// ISOWeek derivation. Europe/Berlin civil dates are the contract; midnight UTC
// is just the carrier for stdlib weekday/iso-week computation, which is
// timezone-agnostic for a fixed civil (Y,M,D).
func (d Date) toTime() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// Time converts the civil Date to its midnight-UTC carrier time.Time.
// Pure conversion — no system clock access, no DST math. Provided so
// consumers can run stdlib time arithmetic (ISOWeek, AddDate, Format)
// against the carrier without re-implementing the conversion or
// forcing a duplicate helper in their own package.
func (d Date) Time() time.Time {
	return d.toTime()
}
