// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tick

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

//counterfeiter:generate -o ../../mocks/tick-metrics.go --fake-name TickMetrics . Metrics

// Metrics records observability events for the hourly tick loop.
type Metrics interface {
	// IncPublished is called once per Publish attempt with the outcome
	// ("success" | "error") and the recurrence kind of the task.
	IncPublished(result string, recurrence string)

	// SetLastTickTimestamp is called at the start of every tick with the
	// wall-clock time of that tick start as Unix seconds (float).
	SetLastTickTimestamp(seconds float64)
}

// NewPrometheusMetrics returns a Metrics backed by the package-level
// Prometheus counter and gauge. The counter is pre-initialized in init()
// so the first call into the metrics surface is just an Inc/Add.
func NewPrometheusMetrics() Metrics {
	return &prometheusMetrics{
		counter: recurringTasksPublishedTotal,
		gauge:   recurringTasksLastTickTimestamp,
	}
}

// recurringTasksPublishedTotal counts Publish outcomes by result and recurrence.
// Pre-initialized to zero for all 14 combinations in init() so Prometheus
// scrapers see the series before the first event.
var recurringTasksPublishedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "recurring_tasks_published_total",
		Help: "Total number of recurring-task Publish attempts grouped by outcome and recurrence kind.",
	},
	[]string{"result", "recurrence"},
)

// recurringTasksLastTickTimestamp is the Unix-seconds timestamp at which
// the most recent tick started. Updated on every tick (initial + hourly).
var recurringTasksLastTickTimestamp = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "recurring_tasks_last_tick_timestamp_seconds",
		Help: "Unix timestamp (seconds) at which the most recent tick started.",
	},
)

func init() {
	prometheus.MustRegister(recurringTasksPublishedTotal, recurringTasksLastTickTimestamp)
	for _, kind := range schedule.AllRecurrenceKinds {
		for _, result := range []string{"success", "error"} {
			recurringTasksPublishedTotal.With(prometheus.Labels{
				"result":     result,
				"recurrence": string(kind),
			}).Add(0)
		}
	}
}

// prometheusMetrics is the Prometheus-backed implementation of Metrics.
// The counter and gauge are package-level singletons registered once in init()
// and pre-initialized for all fourteen result/recurrence label combinations.
type prometheusMetrics struct {
	counter *prometheus.CounterVec
	gauge   prometheus.Gauge
}

func (m *prometheusMetrics) IncPublished(result string, recurrence string) {
	m.counter.With(prometheus.Labels{
		"result":     result,
		"recurrence": recurrence,
	}).Inc()
}

func (m *prometheusMetrics) SetLastTickTimestamp(seconds float64) {
	m.gauge.Set(seconds)
}
