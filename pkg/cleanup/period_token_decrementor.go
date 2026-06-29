// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup

import (
	"context"
	"time"

	"github.com/bborbe/errors"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// PriorPeriodToken returns the period-anchored token of the period
// immediately before currentDate for def's recurrence kind, computed by
// shifting the date one period back and re-invoking the existing
// publisher.PeriodTokenBuilder. Deterministic: the same (def, currentDate)
// always yields the same token, matching the title/UUID5 the publisher
// produced for that prior instance.
func PriorPeriodToken(
	ctx context.Context,
	builder publisher.PeriodTokenBuilder,
	def schedule.TaskDefinition,
	currentDate schedule.Date,
) (publisher.PeriodToken, error) {
	priorDate, err := shiftPriorDate(ctx, def.Recurrence, def.Weekdays, currentDate)
	if err != nil {
		return "", errors.Wrap(ctx, err, "compute prior date")
	}
	return builder.Build(ctx, def, priorDate)
}

// shiftPriorDate returns the civil date of the period immediately before currentDate
// for the given recurrence kind.
func shiftPriorDate(
	ctx context.Context,
	kind schedule.RecurrenceKind,
	weekdays []time.Weekday,
	currentDate schedule.Date,
) (schedule.Date, error) {
	t := currentDate.Time()
	switch kind {
	case schedule.RecurrenceDaily:
		prior := t.AddDate(0, 0, -1)
		return schedule.NewDate(prior.Year(), prior.Month(), prior.Day()), nil
	case schedule.RecurrenceWeekday:
		// Walk back day-by-day (max 7 iterations) until we find a weekday in the set.
		for i := 0; i < 7; i++ {
			candidate := t.AddDate(0, 0, -i-1)
			if weekdayInSet(candidate.Weekday(), weekdays) {
				return schedule.NewDate(candidate.Year(), candidate.Month(), candidate.Day()), nil
			}
		}
		return schedule.Date{}, errors.Errorf(
			ctx,
			"shiftPriorDate: weekday set %v produced no valid firing day in the 7 days before %v",
			weekdays,
			currentDate,
		)
	case schedule.RecurrenceWeekly:
		prior := t.AddDate(0, 0, -7)
		return schedule.NewDate(prior.Year(), prior.Month(), prior.Day()), nil
	case schedule.RecurrenceMonthly:
		prior := t.AddDate(0, -1, 0)
		return schedule.NewDate(prior.Year(), prior.Month(), prior.Day()), nil
	case schedule.RecurrenceQuarterly:
		prior := t.AddDate(0, -3, 0)
		return schedule.NewDate(prior.Year(), prior.Month(), prior.Day()), nil
	case schedule.RecurrenceYearly:
		prior := t.AddDate(-1, 0, 0)
		return schedule.NewDate(prior.Year(), prior.Month(), prior.Day()), nil
	default:
		return schedule.Date{}, errors.Errorf(
			ctx,
			"shiftPriorDate: unknown recurrence kind %q",
			kind,
		)
	}
}

// weekdayInSet reports whether w appears in set.
func weekdayInSet(w time.Weekday, set []time.Weekday) bool {
	for _, s := range set {
		if s == w {
			return true
		}
	}
	return false
}
