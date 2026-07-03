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
	"time"

	cqrsbase "github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libkafka "github.com/bborbe/kafka"
	libsentry "github.com/bborbe/sentry"
	"github.com/bborbe/service"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"

	versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"
	pkg "github.com/bborbe/recurring-task-creator/pkg"
	"github.com/bborbe/recurring-task-creator/pkg/factory"
)

const serviceName = "recurring-task-creator-run-once"

func main() {
	app := &application{}
	os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
	SentryDSN    string               `required:"false" arg:"sentry-dsn"    env:"SENTRY_DSN"    usage:"SentryDSN"                                                         display:"length"`
	SentryProxy  string               `required:"false" arg:"sentry-proxy"  env:"SENTRY_PROXY"  usage:"Sentry Proxy"`
	KafkaBrokers string               `required:"false" arg:"kafka-brokers" env:"KAFKA_BROKERS" usage:"Comma separated list of Kafka brokers (ignored when DRY_RUN=true)"`
	Stage        cqrsbase.Branch      `required:"false" arg:"stage"         env:"STAGE"         usage:"Deployment stage (dev|prod); Kafka topic prefix is set separately via TOPIC_PREFIX"                    default:"dev"`
	TopicPrefix  cqrsbase.TopicPrefix `required:"false" arg:"topic-prefix"  env:"TOPIC_PREFIX"  usage:"Explicit Kafka topic prefix; empty means unprefixed topics"`
	Namespace    string               `required:"true"  arg:"namespace"     env:"NAMESPACE"     usage:"Pod namespace for Schedule CR watch"`
	DryRun       bool                 `required:"false" arg:"dry-run"       env:"DRY_RUN"       usage:"if true, log every would-be CreateCommand and skip the Kafka send"                  default:"false"`
}

func (a *application) Run(ctx context.Context, _ libsentry.Client) error {
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

	syncProducer, err := libkafka.NewSyncProducerWithName(
		ctx,
		libkafka.ParseBrokersFromString(a.KafkaBrokers),
		serviceName,
	)
	if err != nil {
		return errors.Wrap(ctx, err, "create sync producer failed")
	}
	defer syncProducer.Close()

	return factory.CreateTickLoop(
		ctx,
		scheduleStore,
		syncProducer,
		a.TopicPrefix,
		a.DryRun,
	).RunOnce(ctx)
}
