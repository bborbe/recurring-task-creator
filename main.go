// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"time"

	cqrsbase "github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libhttp "github.com/bborbe/http"
	libkafka "github.com/bborbe/kafka"
	liblog "github.com/bborbe/log"
	libmetrics "github.com/bborbe/metrics"
	"github.com/bborbe/run"
	libsentry "github.com/bborbe/sentry"
	"github.com/bborbe/service"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"
	"github.com/bborbe/recurring-task-creator/pkg"
	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/store"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

const serviceName = "recurring-task-creator"

func main() {
	app := &application{}
	os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
	SentryDSN       string            `required:"true"  arg:"sentry-dsn"        env:"SENTRY_DSN"        usage:"SentryDSN"                                                         display:"length"`
	SentryProxy     string            `required:"false" arg:"sentry-proxy"      env:"SENTRY_PROXY"      usage:"Sentry Proxy"`
	Listen          string            `required:"true"  arg:"listen"            env:"LISTEN"            usage:"address to listen to"`
	KafkaBrokers    string            `required:"true"  arg:"kafka-brokers"     env:"KAFKA_BROKERS"     usage:"Comma separated list of Kafka brokers"`
	Stage           cqrsbase.Branch   `required:"true"  arg:"stage"             env:"STAGE"             usage:"Deployment stage (dev|prod) — used as Kafka topic branch prefix"`
	Namespace       string            `required:"true"  arg:"namespace"         env:"NAMESPACE"         usage:"Pod namespace for Schedule CR watch"`
	BuildGitVersion string            `required:"false" arg:"build-git-version" env:"BUILD_GIT_VERSION" usage:"Build Git version"                                                                  default:"dev"`
	BuildGitCommit  string            `required:"false" arg:"build-git-commit"  env:"BUILD_GIT_COMMIT"  usage:"Build Git commit hash"                                                              default:"none"`
	BuildDate       *libtime.DateTime `required:"false" arg:"build-date"        env:"BUILD_DATE"        usage:"Build timestamp (RFC3339)"`
	DryRun          bool              `required:"false" arg:"dry-run"           env:"DRY_RUN"           usage:"if true, log every would-be CreateCommand and skip the Kafka send"                  default:"false"`

	// Cleanup cron: runs inside the same pod as the publisher. Auto-aborts
	// prior in_progress recurring-task instances whose next period has
	// already materialized, per Schedule.spec.skipAutoCleanup.
	CleanupCron   string `required:"false" arg:"cleanup-cron"   env:"CLEANUP_CRON"   usage:"Cron expression for the cleanup tick (default '17 * * * *')"       default:"17 * * * *"`
	GitRestURL    string `required:"false" arg:"git-rest-url"   env:"GIT_REST_URL"   usage:"Base URL of the git-rest HTTP service (e.g. http://git-rest:8080)"`
	GatewaySecret string `required:"false" arg:"gateway-secret" env:"GATEWAY_SECRET" usage:"X-Gateway-Secret forwarded to git-rest"                                                 display:"length"`
}

func (a *application) Run(ctx context.Context, sentryClient libsentry.Client) error {
	libmetrics.NewBuildInfoMetrics().SetBuildInfo(a.BuildGitVersion, a.BuildGitCommit, a.BuildDate)

	currentDateTime := libtime.NewCurrentDateTime()

	syncProducer, err := libkafka.NewSyncProducerWithName(
		ctx,
		libkafka.ParseBrokersFromString(a.KafkaBrokers),
		serviceName,
	)
	if err != nil {
		return errors.Wrap(ctx, err, "create sync producer failed")
	}
	defer syncProducer.Close()

	informerFactory, err := factory.CreateInformatFactory(ctx, a.Namespace)
	if err != nil {
		return errors.Wrapf(ctx, err, "create informat factory failed")
	}

	scheduleStore := store.NewScheduleStore(
		informerFactory.Task().V1().Schedules().Lister(),
		a.Namespace,
	)

	return run.CancelOnFirstFinish(
		ctx,
		a.waitForCacheSyncWithContext(informerFactory),
		a.createTick(currentDateTime, scheduleStore, syncProducer),
		a.createCleanupTick(sentryClient, scheduleStore, currentDateTime),
		a.createHTTPServer(currentDateTime, scheduleStore, syncProducer),
	)
}

func (a *application) createCleanupTick(
	sentryClient libsentry.Client,
	scheduleStore store.ScheduleStore,
	currentDateTimeGetter libtime.CurrentDateTimeGetter,
) run.Func {
	return func(ctx context.Context) error {
		if a.GitRestURL == "" {
			// Cleanup cron disabled: no git-rest endpoint configured.
			// Skip wiring entirely so the existing publisher keeps running
			// without spamming Sentry with transport-failure sentry events.
			glog.V(2).Infof("cleanup cron disabled: GIT_REST_URL not set")
			return nil
		}
		vaultClient := cleanup.NewGitRestClient(nil, a.GitRestURL, a.GatewaySecret)
		supersedance := &cleanup.Supersedance{
			Store:        scheduleStore,
			TokenBuilder: publisher.NewPeriodTokenBuilder(),
			Reader:       vaultClient,
			Writer:       vaultClient,
			Metrics:      cleanup.NewPrometheusMetrics(),
			Clock:        currentDateTimeGetter,
		}
		interval, err := pkg.CronToInterval(ctx, a.CleanupCron)
		if err != nil {
			return errors.Wrap(ctx, err, "parse CLEANUP_CRON")
		}
		glog.V(2).Infof("cleanup cron enabled: every %s (CLEANUP_CRON=%q)", interval, a.CleanupCron)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				date, err := pkg.BerlinDate(ctx, currentDateTimeGetter)
				if err != nil {
					sentryClient.CaptureException(err, nil, nil)
					continue
				}
				glog.V(2).Infof("cleanup: tick started for %s", date)
				if err := supersedance.Run(ctx, date); err != nil {
					glog.Errorf("cleanup: tick failed: %v", err)
					sentryClient.CaptureException(err, nil, nil)
				}
			}
		}
	}
}

func (a *application) waitForCacheSyncWithContext(
	informerFactory externalversions.SharedInformerFactory,
) run.Func {
	return func(ctx context.Context) error {
		syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
		defer syncCancel()

		syncResult := informerFactory.WaitForCacheSyncWithContext(syncCtx)
		if syncResult.Err != nil {
			return errors.Wrap(ctx, syncResult.Err, "informer cache sync failed")
		}
		for _, ok := range syncResult.Synced {
			if !ok {
				return errors.Errorf(ctx, "informer cache did not sync within 30s")
			}
		}
		return nil
	}
}

func (a *application) createTick(
	currentDateTime libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	syncProducer libkafka.SyncProducer,
) run.Func {
	return func(ctx context.Context) error {
		return factory.CreateTick(
			ctx,
			scheduleStore,
			factory.CreatePublisher(syncProducer, a.Stage, a.DryRun),
			currentDateTime,
			tick.NewPrometheusMetrics(),
		).Run(ctx)
	}
}

func (a *application) createHTTPServer(
	clock libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	syncProducer libkafka.SyncProducer,
) run.Func {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		pub := factory.CreatePublisher(syncProducer, a.Stage, a.DryRun)

		router := mux.NewRouter()
		router.Path("/healthz").Handler(factory.CreateHealthzHandler())
		router.Path("/readiness").Handler(libhttp.NewPrintHandler("OK"))
		router.Path("/metrics").Handler(promhttp.Handler())
		router.Path("/setloglevel/{level}").
			Handler(liblog.NewSetLoglevelHandler(ctx, liblog.NewLogLevelSetter(2, 5*time.Minute)))
		router.Path("/trigger").Methods("GET").Handler(
			factory.CreateTriggerHandler(clock, scheduleStore, pub))

		glog.V(2).Infof("starting http server listen on %s", a.Listen)
		return libhttp.NewServer(a.Listen, router).Run(ctx)
	}
}
