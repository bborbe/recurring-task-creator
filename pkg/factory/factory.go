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

	versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"
	externalversions "github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"
	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/store"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. When dryRun is true, the publisher logs
// the would-be CreateCommand and skips the sender call. Pure plumbing: no
// business logic.
func CreatePublisher(sender task.CreateCommandSender, dryRun bool) publisher.Publisher {
	return publisher.NewPublisher(sender, publisher.NewFrontmatterFormatter(), dryRun)
}

// CreateTick builds the hourly cron loop. scheduleStore provides the
// current recurring-task definitions per tick; pub sends one CreateCommand
// per inventory entry; clock is the wall-clock source; metrics records
// per-publish outcomes and the tick-start timestamp.
//
// NewTick can fail at construction time if time.LoadLocation("Europe/Berlin")
// fails (tzdata missing from the container image). That is a container-build
// bug, not a runtime fault — CreateTick panics with a wrapped error if it
// happens. The binary will CrashLoopBackOff with the tzdata error visible
// in the pod logs.
func CreateTick(
	ctx context.Context,
	scheduleStore store.ScheduleStore,
	pub publisher.Publisher,
	clock libtime.CurrentDateTimeGetter,
	metrics tick.Metrics,
) tick.Tick {
	t, err := tick.NewTick(ctx, scheduleStore, pub, clock, metrics)
	if err != nil {
		panic(errors.Wrap(ctx, err, "create tick failed"))
	}
	return t
}

// CreateHealthzHandler returns the liveness-probe HTTP handler. Pure plumbing.
func CreateHealthzHandler() http.Handler {
	return handler.NewHealthzHandler()
}

// CreateTriggerHandler returns the operator-replay HTTP handler. The handler
// reads the current task definitions from scheduleStore, date-filters them
// via schedule.TasksForDate(all, date) (slug-sorted), and calls
// publisher.Publish for each entry against the parsed date. Pure plumbing:
// no business logic, no closure capture, no state.
func CreateTriggerHandler(scheduleStore store.ScheduleStore, pub publisher.Publisher) http.Handler {
	return handler.NewTriggerHandler(scheduleStore, pub)
}

// CreateTickLoop builds a one-shot tick loop for cmd/run-once. It wires the
// store, publisher, clock, and metrics then delegates to CreateTick.
func CreateTickLoop(
	ctx context.Context,
	scheduleStore store.ScheduleStore,
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
	return CreateTick(ctx, scheduleStore, pub, clock, metrics)
}

// CreateCommandSender builds the task.CreateCommandSender. When dryRun is
// true, returns a no-op sender. Pure plumbing.
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

// CreateScheduleStore builds the informer-backed ScheduleStore for the
// given namespace. The caller is responsible for starting the returned
// factory (StartWithContext) and waiting for cache sync before reading.
func CreateScheduleStore(
	client versioned.Interface,
	namespace string,
) (externalversions.SharedInformerFactory, store.ScheduleStore) {
	informerFactory := externalversions.NewSharedInformerFactoryWithOptions(
		client,
		0,
		externalversions.WithNamespace(namespace),
	)
	lister := informerFactory.Task().V1().Schedules().Lister()
	// touch the informer so the factory registers it before Start
	_ = informerFactory.Task().V1().Schedules().Informer()
	return informerFactory, store.NewScheduleStore(lister, namespace)
}
