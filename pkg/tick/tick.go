// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tick

import (
	"context"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// ScheduleLookup is the pure function that the tick invokes every hour to
// compute "what should fire today?". The type is satisfied by
// schedule.TasksForDate; exposing it here makes the constructor signature
// self-documenting and lets tests substitute a fake.
type ScheduleLookup func(date schedule.Date) []schedule.TaskDefinition

// Tick runs the hourly cron loop. Run blocks until ctx is cancelled.
//
//counterfeiter:generate -o ../mocks/tick-tick.go --fake-name TickTick . Tick
type Tick interface {
	// Run performs an initial tick synchronously, then enters a 1-hour loop
	// that fires on time.NewTicker. Returns nil on clean context cancellation.
	Run(ctx context.Context) error
}

// NewTick builds the hourly cron loop. scheduleFn is invoked every tick
// to compute the day's task set; publisher is called once per entry;
// clock is the wall-clock source; metrics records per-publish outcomes
// and the tick-start timestamp. Returns a wrapped error if
// time.LoadLocation("Europe/Berlin") fails at struct init.
func NewTick(
	ctx context.Context,
	scheduleFn ScheduleLookup,
	pub publisher.Publisher,
	clock libtime.CurrentDateTimeGetter,
	metrics Metrics,
) (Tick, error) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, errors.Wrap(ctx, err, "load location Europe/Berlin failed")
	}
	return &tick{
		scheduleFn: scheduleFn,
		publisher:  pub,
		clock:      clock,
		metrics:    metrics,
		berlin:     berlin,
	}, nil
}

type tick struct {
	scheduleFn ScheduleLookup
	publisher  publisher.Publisher
	clock      libtime.CurrentDateTimeGetter
	metrics    Metrics
	berlin     *time.Location
}

// Run performs an initial tick synchronously, then enters a 1-hour loop
// that fires on a time.NewTicker. Returns nil on clean context cancellation.
func (t *tick) Run(ctx context.Context) error {
	t.tick(ctx)

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			glog.V(2).Infof("tick loop: context cancelled, exiting cleanly")
			return nil
		case <-ticker.C:
			t.tick(ctx)
		}
	}
}

// tick performs one full pass: read clock, convert to Berlin civil date,
// call scheduleFn, iterate, and call publisher.Publish for each entry.
// Per-task errors are logged and counted but never abort the pass.
func (t *tick) tick(ctx context.Context) {
	now := t.clock.Now().Time().In(t.berlin)
	year, month, day := now.Date()
	date := schedule.NewDate(year, month, day)

	t.metrics.SetLastTickTimestamp(float64(t.clock.Now().Time().Unix()))

	tasks := t.scheduleFn(date)
	if len(tasks) == 0 {
		glog.V(2).Infof("no tasks for date %04d-%02d-%02d", date.Year, date.Month, date.Day)
		return
	}

	for _, def := range tasks {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := t.publisher.Publish(ctx, def, date); err != nil {
			glog.Errorf(
				"tick: publish failed for slug %q on %04d-%02d-%02d: %v",
				def.Slug, date.Year, date.Month, date.Day, err,
			)
			t.metrics.IncPublished("error", string(def.Recurrence))
			continue
		}
		t.metrics.IncPublished("success", string(def.Recurrence))
	}
}
