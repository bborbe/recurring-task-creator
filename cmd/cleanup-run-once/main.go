// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command recurring-task-creator-cleanup-run-once runs a single cleanup tick
// (compute today's prior-period supersedances and apply them via git-rest) then
// exits. Intended for local smoke-testing without entering the hourly loop.
package main

import (
	"context"
	"os"
	"time"

	"github.com/bborbe/errors"
	libsentry "github.com/bborbe/sentry"
	"github.com/bborbe/service"
	libtime "github.com/bborbe/time"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"

	versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"
	pkg "github.com/bborbe/recurring-task-creator/pkg"
	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

func main() {
	app := &application{}
	os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
	SentryDSN     string `required:"false" arg:"sentry-dsn"      env:"SENTRY_DSN"        usage:"SentryDSN"                                                        display:"length"`
	SentryProxy   string `required:"false" arg:"sentry-proxy"    env:"SENTRY_PROXY"      usage:"Sentry Proxy"`
	Namespace     string `required:"true"  arg:"namespace"       env:"NAMESPACE"         usage:"Pod namespace for Schedule CR watch"`
	GitRestURL    string `required:"true"  arg:"git-rest-url"    env:"GIT_REST_URL"      usage:"Base URL of the git-rest HTTP service (e.g. http://git-rest:8080)"`
	GatewaySecret string `required:"false" arg:"gateway-secret"  env:"GATEWAY_SECRET"    usage:"X-Gateway-Secret forwarded to git-rest"                             display:"length"`
}

func (a *application) Run(ctx context.Context, sentryClient libsentry.Client) error {
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
	// initial cache sync with a 30s deadline.
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

	vaultClient := cleanup.NewGitRestClient(nil, a.GitRestURL, a.GatewaySecret)
	metrics := cleanup.NewPrometheusMetrics()
	clock := libtime.NewCurrentDateTime()

	// Resolve current Berlin civil date for one-shot tick.
	now := clock.Now().Time()
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return errors.Wrap(ctx, err, "load Europe/Berlin location")
	}
	berlinNow := now.In(berlin)
	year, month, day := berlinNow.Date()
	date := schedule.NewDate(year, month, day)

	supersedance := &cleanup.Supersedance{
		Store:        scheduleStore,
		TokenBuilder: publisher.NewPeriodTokenBuilder(),
		Reader:       vaultClient,
		Writer:       vaultClient,
		Metrics:      metrics,
		Clock:        clock,
	}

	return supersedance.Run(ctx, date)
}
