// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command recurring-task-creator-run-once runs a single tick (compute today's
// task set and publish each via Kafka) then exits. Intended for local
// smoke-testing without entering the hourly loop or starting the HTTP server.
package main

import (
	"context"
	"os"

	"github.com/bborbe/agent/lib/command/task"
	cqrsbase "github.com/bborbe/cqrs/base"
	cdb "github.com/bborbe/cqrs/cdb"
	"github.com/bborbe/errors"
	libkafka "github.com/bborbe/kafka"
	liblog "github.com/bborbe/log"
	libsentry "github.com/bborbe/sentry"
	"github.com/bborbe/service"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/tick"
)

const serviceName = "recurring-task-creator-run-once"

func main() {
	app := &application{}
	os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
	SentryDSN    string `required:"false" arg:"sentry-dsn"    env:"SENTRY_DSN"    usage:"SentryDSN"                                                         display:"length"`
	SentryProxy  string `required:"false" arg:"sentry-proxy"  env:"SENTRY_PROXY"  usage:"Sentry Proxy"`
	KafkaBrokers string `required:"false" arg:"kafka-brokers" env:"KAFKA_BROKERS" usage:"Comma separated list of Kafka brokers (ignored when DRY_RUN=true)"`
	DryRun       bool   `required:"false" arg:"dry-run"       env:"DRY_RUN"       usage:"if true, log every would-be CreateCommand and skip the Kafka send"                  default:"false"`
}

func (a *application) Run(ctx context.Context, _ libsentry.Client) error {
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
			cqrsbase.Branch("master"),
			liblog.DefaultSamplerFactory,
		))
	}
	pub := factory.CreatePublisher(sender, a.DryRun)
	clock := libtime.NewCurrentDateTime()
	metrics := tick.NewPrometheusMetrics()
	tickLoop := factory.CreateTick(ctx, pub, clock, metrics)
	return tickLoop.RunOnce(ctx)
}
