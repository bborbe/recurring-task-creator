// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"

	"github.com/bborbe/errors"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// PeriodToken is the period-anchored token string appended to a recurring
// task's title and fed into the UUID5 identifier — "YYYY-MM-DD" for daily,
// "YYYYWNN" for weekly, "YYYYWNN-<3-letter-weekday>" for weekday, "YYYY-MM"
// for monthly, "YYYYQN" for quarterly, "YYYY" for yearly. Wrapped in a
// named string type so calls that take both a slug and a token can't accept
// them in the wrong order without a compile error.
type PeriodToken string

//counterfeiter:generate -o ../../mocks/publisher-period-token-builder.go --fake-name PublisherPeriodTokenBuilder . PeriodTokenBuilder

// PeriodTokenBuilder builds the period-anchored token for a given
// (definition, date) pair. The token formula honors def.Recurrence,
// def.Weekday (only meaningful for RecurrenceWeekday), and
// def.PeriodOffset (only meaningful for the period-anchored kinds —
// Monthly, Quarterly, Yearly).
type PeriodTokenBuilder interface {
	// Build returns the period-anchored token for (def, date). An unknown
	// RecurrenceKind is a build-time data error (closed enum, no valid
	// runtime reason for a new value), so the implementation returns an
	// error rather than a sentinel string.
	Build(ctx context.Context, def schedule.TaskDefinition, date schedule.Date) (PeriodToken, error)
}

// NewPeriodTokenBuilder returns the default PeriodTokenBuilder. Stateless.
func NewPeriodTokenBuilder() PeriodTokenBuilder {
	return &periodTokenBuilder{}
}

type periodTokenBuilder struct{}

func (b *periodTokenBuilder) Build(
	ctx context.Context,
	def schedule.TaskDefinition,
	date schedule.Date,
) (PeriodToken, error) {
	base := date.Time()
	switch def.Recurrence {
	case schedule.RecurrenceDaily:
		return PeriodToken(fmtDate(date.Year, int(date.Month), date.Day)), nil
	case schedule.RecurrenceWeekly:
		isoYear, isoWeek := base.ISOWeek()
		return PeriodToken(fmtIsoWeek(isoYear, isoWeek)), nil
	case schedule.RecurrenceWeekday:
		isoYear, isoWeek := base.ISOWeek()
		return PeriodToken(fmtIsoWeek(isoYear, isoWeek) + "-" + weekdayAbbrev(def.Weekday)), nil
	case schedule.RecurrenceMonthly:
		shifted := base.AddDate(0, def.PeriodOffset, 0)
		return PeriodToken(fmtMonthYear(shifted.Year(), int(shifted.Month()))), nil
	case schedule.RecurrenceQuarterly:
		shifted := base.AddDate(0, def.PeriodOffset*3, 0)
		return PeriodToken(fmtQuarter(shifted.Year(), quarterOf(shifted.Month()))), nil
	case schedule.RecurrenceYearly:
		shifted := base.AddDate(def.PeriodOffset, 0, 0)
		return PeriodToken(fmtYear(shifted.Year())), nil
	default:
		return "", errors.Errorf(
			ctx,
			"PeriodTokenBuilder.Build: unknown recurrence kind %q",
			def.Recurrence,
		)
	}
}
