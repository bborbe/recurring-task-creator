// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// CronToInterval converts a standard 5-field cron expression to the matching
// Go time.Duration between fires. Supports the subset the cleanup cron
// needs: `M H DoM Mo DoW` with `*` or explicit single values. Returns the
// duration until the NEXT fire from now, so the ticker fires at the right
// minute boundary rather than waiting a full period from process start.
// Returns an error on unsupported forms.
func CronToInterval(ctx context.Context, expr string) (time.Duration, error) {
	parts := SplitFields(expr)
	if len(parts) != 5 {
		return 0, errors.Errorf(
			ctx,
			"cron: expected 5 fields, got %d in %q",
			len(parts),
			expr,
		)
	}
	minute, err := ParseField(ctx, parts[0], 0, 59)
	if err != nil {
		return 0, errors.Wrap(ctx, err, "cron: minute field")
	}
	hour, err := ParseField(ctx, parts[1], 0, 23)
	if err != nil {
		return 0, errors.Wrap(ctx, err, "cron: hour field")
	}
	if parts[2] != "*" {
		return 0, errors.Errorf(
			ctx,
			"cron: day-of-month not supported, got %q",
			parts[2],
		)
	}
	if parts[3] != "*" {
		return 0, errors.Errorf(ctx, "cron: month not supported, got %q", parts[3])
	}
	if parts[4] != "*" {
		return 0, errors.Errorf(
			ctx,
			"cron: day-of-week not supported, got %q",
			parts[4],
		)
	}
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now), nil
}

// ParseField accepts "*" or a single integer in [min, max].
func ParseField(ctx context.Context, s string, min, max int) (int, error) {
	if s == "*" {
		return -1, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.Wrap(ctx, err, "not an integer")
	}
	if n < min || n > max {
		return 0, errors.Errorf(ctx, "out of range [%d, %d]: %d", min, max, n)
	}
	return n, nil
}

var fieldSep = regexp.MustCompile(`\s+`)

func SplitFields(expr string) []string {
	return fieldSep.Split(strings.TrimSpace(expr), -1)
}

func BerlinDate(ctx context.Context, clock libtime.CurrentDateTimeGetter) (schedule.Date, error) {
	now := clock.Now().Time()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return schedule.Date{}, errors.Wrap(ctx, err, "load Europe/Berlin")
	}
	berlin := now.In(loc)
	return schedule.NewDate(berlin.Year(), berlin.Month(), berlin.Day()), nil
}
