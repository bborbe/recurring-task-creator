// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/bborbe/agent/lib/command/task"
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

	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
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
	BuildGitVersion string            `required:"false" arg:"build-git-version" env:"BUILD_GIT_VERSION" usage:"Build Git version"                                                                  default:"dev"`
	BuildGitCommit  string            `required:"false" arg:"build-git-commit"  env:"BUILD_GIT_COMMIT"  usage:"Build Git commit hash"                                                              default:"none"`
	BuildDate       *libtime.DateTime `required:"false" arg:"build-date"        env:"BUILD_DATE"        usage:"Build timestamp (RFC3339)"`
	DryRun          bool              `required:"false" arg:"dry-run"           env:"DRY_RUN"           usage:"if true, log every would-be CreateCommand and skip the Kafka send"                  default:"false"`

	// HealthzHandler + TriggerHandler are wired in Run() and consumed by
	// runHTTPServer(). Exposing them on application matches the maintainer
	// watcher pattern (e.g. github-build/main.go).
	HealthzHandler http.Handler
	TriggerHandler http.Handler
}

func (a *application) Run(ctx context.Context, _ libsentry.Client) error {
	libmetrics.NewBuildInfoMetrics().SetBuildInfo(a.BuildGitVersion, a.BuildGitCommit, a.BuildDate)

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

	sender := task.NewCreateCommandSender(cdb.NewCommandObjectSender(
		syncProducer,
		cqrsbase.Branch("master"),
		liblog.DefaultSamplerFactory,
	))
	pub := factory.CreatePublisher(sender, a.DryRun)

	clock := libtime.NewCurrentDateTime()
	metrics := tick.NewPrometheusMetrics()
	tickLoop := factory.CreateTick(ctx, pub, clock, metrics)

	a.HealthzHandler = factory.CreateHealthzHandler()
	a.TriggerHandler = factory.CreateTriggerHandler(pub, schedule.TasksForDate)

	return run.CancelOnFirstFinish(
		ctx,
		a.runHTTPServer(),
		tickLoop.Run,
	)
}

func (a *application) runHTTPServer() run.Func {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		router := mux.NewRouter()
		router.Path("/healthz").Handler(a.HealthzHandler)
		router.Path("/readiness").Handler(libhttp.NewPrintHandler("OK"))
		router.Path("/metrics").Handler(promhttp.Handler())
		router.Path("/setloglevel/{level}").
			Handler(liblog.NewSetLoglevelHandler(ctx, liblog.NewLogLevelSetter(2, 5*time.Minute)))
		router.Path("/trigger").Methods("GET").Handler(a.TriggerHandler)

		glog.V(2).Infof("starting http server listen on %s", a.Listen)
		return libhttp.NewServer(a.Listen, router).Run(ctx)
	}
}
