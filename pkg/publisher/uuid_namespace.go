// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher

import (
	"context"

	"github.com/bborbe/agent/lib"
	"github.com/bborbe/errors"
	"github.com/google/uuid"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// uuidNamespace is the UUID5 namespace used to derive TaskIdentifier values
// for recurring tasks. It is FROZEN — never read from env or flag, never
// regenerated, never changed. Changing it is a breaking change to the entire
// downstream Kafka stream (every identifier collides; the controller will
// create a duplicate vault file for every recurring task on the next tick).
//
// If a future spec needs a new namespace, define a new constant alongside
// this one with a distinct name and do not edit this one.
var uuidNamespace uuid.UUID = uuid.MustParse("f4e1c5b7-3a82-4d59-9e7c-1c8b9d2e4f6a")

// buildPeriodToken returns the period-anchored token for the given
// (recurrence, date) pair. The token is the same string the corresponding
// title-rendering formatter produces — "YYYY-MM-DD" for daily, "YYYYWNN"
// for weekly, "YYYY-MM" for monthly, "YYYYQN" for quarterly, "YYYY" for
// yearly. Anchoring by def.Recurrence (not def.Fires) is intentional: the
// publisher's identifier layer is period-stable, the schedule's firing
// predicate is a hint about which day inside the period the user wants
// to see the task.
//
// Berlin local time governs the period boundary; the date passed in is
// already Berlin-local (the tick converts wall-clock to Berlin civil date
// before calling Publish).
//
// An unknown RecurrenceKind is a build-time data error (closed enum, no
// valid runtime reason for a new value), so the function returns an error
// rather than a sentinel string. The caller wraps with the slug.
func buildPeriodToken(
	ctx context.Context,
	recurrence schedule.RecurrenceKind,
	date schedule.Date,
) (string, error) {
	base := date.Time()
	switch recurrence {
	case schedule.RecurrenceDaily:
		return fmtDate(date.Year, int(date.Month), date.Day), nil
	case schedule.RecurrenceWeekly:
		isoYear, isoWeek := base.ISOWeek()
		return fmtIsoWeek(isoYear, isoWeek), nil
	case schedule.RecurrenceMonthly:
		return fmtMonthYear(base.Year(), int(base.Month())), nil
	case schedule.RecurrenceQuarterly:
		return fmtQuarter(base.Year(), quarterOf(base.Month())), nil
	case schedule.RecurrenceYearly:
		return fmtYear(base.Year()), nil
	default:
		return "", errors.Errorf(
			ctx,
			"buildPeriodToken: unknown recurrence kind %q",
			recurrence,
		)
	}
}

// buildTaskIdentifier returns the deterministic TaskIdentifier for the
// (slug, recurrence, date) triple. The identifier is
// UUID5(uuidNamespace, "recurring-<slug>-<period-token>"), where
// <period-token> is the period-anchored token derived from recurrence
// and date (see buildPeriodToken). Same input on a second call produces
// the same identifier across processes, redeploys, and replays — this is
// the contract the controller's de-dup relies on.
//
// For weekly / monthly / quarterly / yearly entries the identifier is
// stable across all days inside one period, so the hourly tick can
// publish the full inventory every hour without producing duplicate
// vault files. For daily entries (and any future entry that should
// remain date-anchored) the identifier is the civil date itself.
func buildTaskIdentifier(
	ctx context.Context,
	slug string,
	recurrence schedule.RecurrenceKind,
	date schedule.Date,
) (lib.TaskIdentifier, error) {
	token, err := buildPeriodToken(ctx, recurrence, date)
	if err != nil {
		return "", errors.Wrapf(ctx, err, "buildTaskIdentifier: slug %q", slug)
	}
	name := "recurring-" + slug + "-" + token
	return lib.TaskIdentifier(uuid.NewSHA1(uuidNamespace, []byte(name)).String()), nil
}
