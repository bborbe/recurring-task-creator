// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command recurring-task-creator-cleanup runs the cleanup cron loop that
// auto-aborts prior-period in-progress recurring-task instances via git-rest.
// Intended to run as a long-lived sidecar/process; does NOT serve HTTP.
package main

import (
	"context"
	"os"
	"time"

	"github.com/bborbe/cron"
	"github.com/bborbe/errors"
	libsentry "github.com/bborbe/sentry"
	"github.com/bborbe/service"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"

	versioned "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned"
	pkg "github.com/bborbe/recurring-task-creator/pkg"
	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/factory"
)

func main() {
	app := &application{}
	os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
	SentryDSN     string `required:"false" arg:"sentry-dsn"     env:"SENTRY_DSN"     usage:"SentryDSN"                                                         display:"length"`
	SentryProxy   string `required:"false" arg:"sentry-proxy"   env:"SENTRY_PROXY"   usage:"Sentry Proxy"`
	Namespace     string `required:"true"  arg:"namespace"      env:"NAMESPACE"      usage:"Pod namespace for Schedule CR watch"`
	GitRestURL    string `required:"true"  arg:"git-rest-url"   env:"GIT_REST_URL"   usage:"Base URL of the git-rest HTTP service (e.g. http://git-rest:8080)"`
	GatewaySecret string `required:"false" arg:"gateway-secret" env:"GATEWAY_SECRET" usage:"X-Gateway-Secret forwarded to git-rest"                            display:"length"`
	CleanupCron   string `required:"false" arg:"cleanup-cron"   env:"CLEANUP_CRON"   usage:"Cron expression for the cleanup tick (default '17 * * * *')"                        default:"17 * * * *"`
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
	cronExpr := cron.Expression(a.CleanupCron)

	glog.V(2).Infof("cleanup: started")

	return factory.CreateCleanup(
		sentryClient,
		clock,
		scheduleStore,
		vaultClient,
		metrics,
		cronExpr,
	).Run(ctx)
}
