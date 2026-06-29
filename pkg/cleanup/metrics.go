// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/cleanup-metrics.go --fake-name CleanupMetrics . Metrics

// Metrics records cleanup-cron supersede outcomes.
type Metrics interface {
	// IncSuperseded is called once per file the cron attempts to supersede.
	// result is one of "success", "conflict", "error"; recurrence is the
	// kind string.
	IncSuperseded(result string, recurrence string)
}

// recurringTaskCleanupSupersededTotal counts cleanup supersede outcomes by result and recurrence.
// Pre-initialized to zero for all combinations in init() so Prometheus scrapers see the series
// before the first event.
var recurringTaskCleanupSupersededTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "recurring_task_cleanup_superseded_total",
		Help: "Total number of cleanup supersede attempts grouped by outcome and recurrence kind.",
	},
	[]string{"recurrence", "result"},
)

func init() {
	prometheus.MustRegister(recurringTaskCleanupSupersededTotal)
	for _, kind := range schedule.AllRecurrenceKinds {
		for _, result := range []string{"success", "conflict", "error"} {
			recurringTaskCleanupSupersededTotal.With(prometheus.Labels{
				"recurrence": string(kind),
				"result":     result,
			}).Add(0)
		}
	}
}

// prometheusMetrics is the Prometheus-backed implementation of Metrics.
type prometheusMetrics struct{}

// NewPrometheusMetrics returns a Metrics backed by Prometheus counters.
func NewPrometheusMetrics() Metrics {
	return &prometheusMetrics{}
}

func (m *prometheusMetrics) IncSuperseded(result string, recurrence string) {
	recurringTaskCleanupSupersededTotal.With(prometheus.Labels{
		"result":     result,
		"recurrence": recurrence,
	}).Inc()
}
