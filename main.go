// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/factory"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
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
	Stage           string            `required:"true"  arg:"stage"             env:"STAGE"             usage:"Deployment stage (dev|prod) — used as Kafka topic branch prefix"`
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
			cqrsbase.Branch(a.Stage),
			liblog.DefaultSamplerFactory,
		), "personal")
	}
	pub := factory.CreatePublisher(sender, a.DryRun)

	clock := libtime.NewCurrentDateTime()
	tickLoop := factory.CreateTick(ctx, scheduleStore, pub, clock, tick.NewPrometheusMetrics())
	cleanupTick, err := a.createCleanupTick(ctx, sentryClient, scheduleStore, clock)
	if err != nil {
		return errors.Wrap(ctx, err, "create cleanup tick failed")
	}

	return run.CancelOnFirstFinish(
		ctx,
		a.createHTTPServer(clock, scheduleStore, pub),
		tickLoop.Run,
		cleanupTick,
	)
}

func (a *application) createCleanupTick(
	ctx context.Context,
	sentryClient libsentry.Client,
	scheduleStore store.ScheduleStore,
	clock libtime.CurrentDateTimeGetter,
) (run.Func, error) {
	if a.GitRestURL == "" {
		// Cleanup cron disabled: no git-rest endpoint configured.
		// Skip wiring entirely so the existing publisher keeps running.
		glog.V(2).Infof("cleanup cron disabled: GIT_REST_URL not set")
		return func(context.Context) error { return nil }, nil
	}
	vaultClient := cleanup.NewGitRestClient(nil, a.GitRestURL, a.GatewaySecret)
	supersedance := &cleanup.Supersedance{
		Store:        scheduleStore,
		TokenBuilder: publisher.NewPeriodTokenBuilder(),
		Reader:       vaultClient,
		Writer:       vaultClient,
		Metrics:      cleanup.NewPrometheusMetrics(),
		Clock:        clock,
	}
	interval, err := cronToInterval(a.CleanupCron)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "parse CLEANUP_CRON")
	}
	glog.V(2).Infof("cleanup cron enabled: every %s (CLEANUP_CRON=%q)", interval, a.CleanupCron)
	return run.Func(func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				date, err := berlinDate(clock)
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
	}), nil
}

// cronToInterval converts a standard 5-field cron expression to the matching
// Go time.Duration between fires. Supports the subset the cleanup cron
// needs: `M H DoM Mo DoW` with `*` or explicit single values. Returns the
// duration until the NEXT fire from now, so the ticker fires at the right
// minute boundary rather than waiting a full period from process start.
// Returns an error on unsupported forms.
func cronToInterval(expr string) (time.Duration, error) {
	parts := splitFields(expr)
	if len(parts) != 5 {
		return 0, errors.Errorf(
			context.Background(),
			"cron: expected 5 fields, got %d in %q",
			len(parts),
			expr,
		)
	}
	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return 0, errors.Wrap(context.Background(), err, "cron: minute field")
	}
	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return 0, errors.Wrap(context.Background(), err, "cron: hour field")
	}
	if parts[2] != "*" {
		return 0, errors.Errorf(
			context.Background(),
			"cron: day-of-month not supported, got %q",
			parts[2],
		)
	}
	if parts[3] != "*" {
		return 0, errors.Errorf(context.Background(), "cron: month not supported, got %q", parts[3])
	}
	if parts[4] != "*" {
		return 0, errors.Errorf(
			context.Background(),
			"cron: day-of-week not supported, got %q",
			parts[4],
		)
	}
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now), nil
}

// parseField accepts "*" or a single integer in [min, max].
func parseField(s string, min, max int) (int, error) {
	if s == "*" {
		return -1, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.Wrap(context.Background(), err, "not an integer")
	}
	if n < min || n > max {
		return 0, errors.Errorf(context.Background(), "out of range [%d, %d]: %d", min, max, n)
	}
	return n, nil
}

var fieldSep = regexp.MustCompile(`\s+`)

func splitFields(expr string) []string {
	return fieldSep.Split(strings.TrimSpace(expr), -1)
}

func berlinDate(clock libtime.CurrentDateTimeGetter) (schedule.Date, error) {
	now := clock.Now().Time()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return schedule.Date{}, errors.Wrap(context.Background(), err, "load Europe/Berlin")
	}
	berlin := now.In(loc)
	return schedule.NewDate(berlin.Year(), berlin.Month(), berlin.Day()), nil
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
