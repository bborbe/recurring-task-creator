// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"context"
	"net/http"
	"time"

	"github.com/bborbe/agent/command/task"
	cqrsbase "github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	"github.com/bborbe/cron"
	"github.com/bborbe/errors"
	libkafka "github.com/bborbe/kafka"
	liblog "github.com/bborbe/log"
	"github.com/bborbe/run"
	libsentry "github.com/bborbe/sentry"
	libtime "github.com/bborbe/time"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"

	"github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"
	"github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"
	"github.com/bborbe/recurring-task-creator/pkg"
	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/store"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. When dryRun is true, the publisher logs
// the would-be CreateCommand and skips the sender call. Pure plumbing: no
// business logic.
func CreatePublisher(
	syncProducer libkafka.SyncProducer,
	branch cqrsbase.Branch,
	dryRun bool,
) publisher.Publisher {
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
	renderer := publisher.NewRenderer()
	return publisher.NewPublisher(
		sender,
		renderer,
		publisher.NewFrontmatterFormatter(renderer),
		publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
		dryRun,
	)
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
// publisher.Publish for each entry against the resolved date (the `date`
// query parameter, or the clock's current civil date if missing/unparseable).
// Pure plumbing: no business logic, no closure capture, no state.
func CreateTriggerHandler(
	clock libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	pub publisher.Publisher,
) http.Handler {
	return handler.NewTriggerHandler(clock, scheduleStore, pub)
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
		syncProducer,
		branch,
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

// CreateCleanup wires the cleanup orchestrator and the cron loop around it.
// Pure plumbing: schedule store, vault client, clock, metrics, and the cron
// expression. Returns a run.Runnable so the main binary can compose it
// with run.CancelOnFirstFinish.
func CreateCleanup(
	sentryClient libsentry.Client,
	currentTimeGetter libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	vaultClient cleanup.VaultClient,
	metrics cleanup.Metrics,
	cronExpr cron.Expression,
) run.Runnable {
	supersedance := &cleanup.Supersedance{
		Store:        scheduleStore,
		TokenBuilder: publisher.NewPeriodTokenBuilder(),
		Reader:       vaultClient,
		Writer:       vaultClient,
		Metrics:      metrics,
		Clock:        currentTimeGetter,
	}
	return cron.NewExpressionCron(
		cronExpr,
		run.Func(func(ctx context.Context) error {
			now := currentTimeGetter.Now().Time()
			berlinLoc, err := time.LoadLocation("Europe/Berlin")
			if err != nil {
				return errors.Wrap(ctx, err, "load Europe/Berlin location")
			}
			berlinNow := now.In(berlinLoc)
			year, month, day := berlinNow.Date()
			date := schedule.NewDate(year, month, day)
			if err := supersedance.Run(ctx, date); err != nil {
				sentryClient.CaptureException(err, nil, nil)
			}
			return nil
		}),
	)
}

func CreateInformatFactory(
	ctx context.Context,
	namespace string,
) (externalversions.SharedInformerFactory, error) {
	connector := pkg.NewK8sConnector(
		rest.InClusterConfig,
		func(c *rest.Config) (apiextensionsclient.Interface, error) {
			return apiextensionsclient.NewForConfig(c)
		},
	)
	if err := connector.SetupCustomResourceDefinition(ctx); err != nil {
		return nil, errors.Wrap(ctx, err, "setup CRD failed")
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(ctx, err, "in-cluster config failed")
	}
	versionedClient, err := versioned.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "build versioned client failed")
	}
	informerFactory := externalversions.NewSharedInformerFactoryWithOptions(
		versionedClient,
		0,
		externalversions.WithNamespace(namespace),
	)
	// touch the informer so the factory registers it before Start
	_ = informerFactory.Task().V1().Schedules().Informer()

	// Lifecycle: start the informer goroutines with the LONG-LIVED `ctx`
	// so they keep running for the life of the process. ONLY bound the
	// initial cache sync with a 30s deadline — `StartWithContext` must
	// NOT receive a deadline-bounded context, or the informer goroutines
	// will exit at the 30s mark and silently stop delivering updates.
	informerFactory.StartWithContext(ctx)
	return informerFactory, nil
}
