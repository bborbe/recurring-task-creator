// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"context"
	"net/http"

	"github.com/bborbe/agent/lib/command/task"
	cqrsbase "github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	"github.com/bborbe/errors"
	libkafka "github.com/bborbe/kafka"
	liblog "github.com/bborbe/log"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. When dryRun is true, the publisher logs
// the would-be CreateCommand and skips the sender call. Pure plumbing: no
// business logic.
func CreatePublisher(sender task.CreateCommandSender, dryRun bool) publisher.Publisher {
	return publisher.NewPublisher(sender, dryRun)
}

// CreateTick builds the hourly cron loop. The full inventory is published
// every tick; per-day filtering is gone (Spec 6). pub sends one
// CreateCommand per inventory entry; clock is the wall-clock source;
// metrics records per-publish outcomes and the tick-start timestamp.
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
	t, err := tick.NewTick(ctx, schedule.Inventory(), pub, clock, metrics)
	if err != nil {
		panic(errors.Wrap(ctx, err, "create tick failed"))
	}
	return t
}

// CreateHealthzHandler returns the liveness-probe HTTP handler. Pure plumbing.
func CreateHealthzHandler() http.Handler {
	return handler.NewHealthzHandler()
}

// CreateTriggerHandler returns the operator-replay HTTP handler. The
// handler iterates schedule.Inventory() (full inventory, slug-sorted)
// and calls publisher.Publish for each entry against the parsed date.
// Pure plumbing: no business logic, no closure capture, no state.
func CreateTriggerHandler(publisher publisher.Publisher) http.Handler {
	return handler.NewTriggerHandler(publisher)
}

func CreateTickLoop(
	ctx context.Context,
	syncProducer libkafka.SyncProducer,
	branch cqrsbase.Branch,
	dryRun bool,
) tick.Tick {

	pub := CreatePublisher(
		CreateCommandSender(syncProducer, branch, dryRun),
		dryRun,
	)
	clock := libtime.NewCurrentDateTime()
	metrics := tick.NewPrometheusMetrics()
	tickLoop := CreateTick(ctx, pub, clock, metrics)
	return tickLoop
}

func CreateCommandSender(
	syncProducer libkafka.SyncProducer,
	branch cqrsbase.Branch,
	dryRun bool,
) task.CreateCommandSender {
	var sender task.CreateCommandSender
	if dryRun {
		sender = publisher.NewNoopSender()
	} else {
		sender = task.NewCreateCommandSender(cdb.NewCommandObjectSender(
			syncProducer,
			branch,
			liblog.DefaultSamplerFactory,
		), "personal")
	}
	return sender
}
