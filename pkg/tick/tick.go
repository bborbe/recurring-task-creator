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
	"github.com/bborbe/recurring-task-creator/pkg/store"
)

//counterfeiter:generate -o ../../mocks/tick-tick.go --fake-name TickTick . Tick

// Tick runs the hourly cron loop. Run blocks until ctx is cancelled.
type Tick interface {
	// Run performs an initial tick synchronously, then enters a 1-hour loop
	// that fires on time.NewTicker. Returns nil on clean context cancellation.
	Run(ctx context.Context) error
	// RunOnce performs a single tick (publish every entry in the inventory)
	// and returns. Intended for local smoke-testing via cmd/run-once.
	RunOnce(ctx context.Context) error
}

// NewTick builds the hourly cron loop. store supplies the recurring-task
// definitions per tick via its List method; pub sends one CreateCommand per
// entry per tick; clock is the wall-clock source; metrics records per-publish
// outcomes and the tick-start timestamp. Returns a wrapped error if
// time.LoadLocation("Europe/Berlin") fails at struct init.
func NewTick(
	ctx context.Context,
	scheduleStore store.ScheduleStore,
	pub publisher.Publisher,
	clock libtime.CurrentDateTimeGetter,
	metrics Metrics,
) (Tick, error) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, errors.Wrap(ctx, err, "load location Europe/Berlin failed")
	}
	return &tick{
		store:     scheduleStore,
		publisher: pub,
		clock:     clock,
		metrics:   metrics,
		berlin:    berlin,
	}, nil
}

type tick struct {
	store     store.ScheduleStore
	publisher publisher.Publisher
	clock     libtime.CurrentDateTimeGetter
	metrics   Metrics
	berlin    *time.Location
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

// RunOnce performs a single tick and returns. Useful for cmd/run-once
// smoke-testing without entering the long-lived ticker loop.
func (t *tick) RunOnce(ctx context.Context) error {
	t.tick(ctx)
	return nil
}

// tick performs one full pass: read clock, convert to Berlin civil date,
// update the gauge, list the store, date-filter via schedule.TasksForDate,
// and call publisher.Publish for each entry. A store-list failure is logged
// and skips this tick (the next hourly tick will retry). Per-task publish
// errors are logged and counted but never abort the pass.
func (t *tick) tick(ctx context.Context) {
	now := t.clock.Now().Time().In(t.berlin)
	t.metrics.SetLastTickTimestamp(float64(now.Unix()))
	year, month, day := now.Date()
	date := schedule.NewDate(year, month, day)

	all, err := t.store.List(ctx)
	if err != nil {
		glog.Errorf(
			"tick: list store failed for %04d-%02d-%02d: %v",
			date.Year,
			date.Month,
			date.Day,
			err,
		)
		return
	}

	defs := schedule.TasksForDate(all, date)

	if len(defs) == 0 {
		glog.V(2).Infof("no tasks for date %04d-%02d-%02d", date.Year, date.Month, date.Day)
		return
	}

	for _, def := range defs {
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
