// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"context"
	"net/http"

	"github.com/bborbe/agent/lib/command/task"
	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. Pure plumbing: no business logic.
func CreatePublisher(sender task.CreateCommandSender) publisher.Publisher {
	return publisher.NewPublisher(sender)
}

// CreateTick builds the hourly cron loop. schedule.TasksForDate is
// injected as the lookup so the caller never imports the inventory
// directly. pub sends one CreateCommand per task; clock is the wall-clock
// source; metrics records per-publish outcomes and the tick-start
// timestamp.
//
// NewTick can fail at construction time if time.LoadLocation("Europe/Berlin")
// fails (tzdata missing from the container image). That is a container-build
// bug, not a runtime fault — CreateTick panics with a wrapped error if it
// happens, per the factory pattern's "no error return" rule. The binary
// will CrashLoopBackOff with the tzdata error visible in the pod logs.
func CreateTick(
	ctx context.Context,
	pub publisher.Publisher,
	clock libtime.CurrentDateTimeGetter,
	metrics tick.Metrics,
) tick.Tick {
	t, err := tick.NewTick(ctx, schedule.TasksForDate, pub, clock, metrics)
	if err != nil {
		panic(errors.Wrap(ctx, err, "create tick failed"))
	}
	return t
}

// CreateHealthzHandler returns the liveness-probe HTTP handler. Pure plumbing.
func CreateHealthzHandler() http.Handler {
	return handler.NewHealthzHandler()
}

// CreateTriggerHandler returns the operator-replay HTTP handler. lookup is
// injected as the per-date task source so the handler never imports the
// inventory directly; production wiring passes schedule.TasksForDate.
func CreateTriggerHandler(
	publisher publisher.Publisher,
	lookup schedule.ScheduleLookup,
) http.Handler {
	return handler.NewTriggerHandler(publisher, lookup)
}
