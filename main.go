// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"time"

	"github.com/bborbe/agent/command/task"
	cqrsbase "github.com/bborbe/cqrs/base"
	cdb "github.com/bborbe/cqrs/cdb"
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
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"

	versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"
	pkg "github.com/bborbe/recurring-task-creator/pkg"
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
	SentryDSN       string               `required:"true"  arg:"sentry-dsn"        env:"SENTRY_DSN"        usage:"SentryDSN"                                                         display:"length"`
	SentryProxy     string               `required:"false" arg:"sentry-proxy"      env:"SENTRY_PROXY"      usage:"Sentry Proxy"`
	Listen          string               `required:"true"  arg:"listen"            env:"LISTEN"            usage:"address to listen to"`
	KafkaBrokers    string               `required:"true"  arg:"kafka-brokers"     env:"KAFKA_BROKERS"     usage:"Comma separated list of Kafka brokers"`
	Stage           string               `required:"true"  arg:"stage"             env:"STAGE"             usage:"Deployment stage (dev|prod) — used as Kafka topic branch prefix"`
	TopicPrefix     cqrsbase.TopicPrefix `required:"false" arg:"topic-prefix"      env:"TOPIC_PREFIX"      usage:"Explicit Kafka topic prefix; empty means unprefixed topics"`
	Namespace       string               `required:"true"  arg:"namespace"         env:"NAMESPACE"         usage:"Pod namespace for Schedule CR watch"`
	BuildGitVersion string               `required:"false" arg:"build-git-version" env:"BUILD_GIT_VERSION" usage:"Build Git version"                                                                  default:"dev"`
	BuildGitCommit  string               `required:"false" arg:"build-git-commit"  env:"BUILD_GIT_COMMIT"  usage:"Build Git commit hash"                                                              default:"none"`
	BuildDate       *libtime.DateTime    `required:"false" arg:"build-date"        env:"BUILD_DATE"        usage:"Build timestamp (RFC3339)"`
	DryRun          bool                 `required:"false" arg:"dry-run"           env:"DRY_RUN"           usage:"if true, log every would-be CreateCommand and skip the Kafka send"                  default:"false"`
}

func (a *application) Run(ctx context.Context, _ libsentry.Client) error {
	libmetrics.NewBuildInfoMetrics().SetBuildInfo(a.BuildGitVersion, a.BuildGitCommit, a.BuildDate)

	connector := pkg.NewK8sConnector(
		rest.InClusterConfig,
		func(c *rest.Config) (apiextensionsclient.Interface, error) {
			return apiextensionsclient.NewForConfig(c)
		},
	)
	if err := connector.SetupCustomResourceDefinition(ctx); err != nil {
		return errors.Wrap(ctx, err, "setup CRD failed")
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(ctx, err, "in-cluster config failed")
	}
	versionedClient, err := versioned.NewForConfig(restConfig)
	if err != nil {
		return errors.Wrap(ctx, err, "build versioned client failed")
	}
	informerFactory, scheduleStore := factory.CreateScheduleStore(versionedClient, a.Namespace)

	// Lifecycle: start the informer goroutines with the LONG-LIVED `ctx`
	// so they keep running for the life of the process. ONLY bound the
	// initial cache sync with a 30s deadline — `StartWithContext` must
	// NOT receive a deadline-bounded context, or the informer goroutines
	// will exit at the 30s mark and silently stop delivering updates.
	informerFactory.StartWithContext(ctx)
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

	var sender task.CreateCommandSender
	if a.DryRun {
		sender = publisher.NewNoopSender()
	} else {
		saramaClient, err := libkafka.CreateSaramaClient(
			ctx,
			libkafka.ParseBrokersFromString(a.KafkaBrokers),
		)
		if err != nil {
			return errors.Wrap(ctx, err, "create sarama client failed")
		}
		defer saramaClient.Close()

		syncProducer, err := libkafka.NewSyncProducerWithName(
			ctx,
			libkafka.ParseBrokersFromString(a.KafkaBrokers),
			serviceName,
		)
		if err != nil {
			return errors.Wrap(ctx, err, "create sync producer failed")
		}
		defer syncProducer.Close()

		sender = task.NewCreateCommandSender(cdb.NewCommandObjectSender(
			syncProducer,
			a.TopicPrefix,
			liblog.DefaultSamplerFactory,
		), "personal")
	}
	pub := factory.CreatePublisher(sender, a.DryRun)

	clock := libtime.NewCurrentDateTime()
	metrics := tick.NewPrometheusMetrics()
	tickLoop := factory.CreateTick(ctx, scheduleStore, pub, clock, metrics)

	return run.CancelOnFirstFinish(
		ctx,
		a.createHTTPServer(clock, scheduleStore, pub),
		tickLoop.Run,
	)
}

func (a *application) createHTTPServer(
	clock libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	pub publisher.Publisher,
) run.Func {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

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
