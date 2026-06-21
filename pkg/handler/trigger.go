// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/bborbe/errors"
	libhttp "github.com/bborbe/http"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/store"
)

// triggerErrorEntry is one per-task failure in the /trigger response.
// Always emitted, even when empty (no omitempty on the errors slice).
type triggerErrorEntry struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

// triggerResponse is the JSON shape of GET /trigger on a 2xx.
// `errors` is always present, never omitted.
type triggerResponse struct {
	Date      string              `json:"date"`
	Published int                 `json:"published"`
	Errors    []triggerErrorEntry `json:"errors"`
}

// NewTriggerHandler returns an HTTP handler that replays the recurring-task
// publishes for one civil date. The date is supplied as the optional `date`
// query parameter (any libtime-parseable format, e.g. YYYY-MM-DD or RFC3339);
// when missing, empty, or unparseable, the handler falls back to the clock's
// current civil date. scheduleStore provides the current recurring-task
// definitions; a store error returns HTTP 500. For each entry in the
// date-filtered inventory (schedule.TasksForDate, slug-sorted), the handler
// calls publisher.Publish(ctx, def, date). Per-task errors are accumulated in
// the response's `errors` array — the iteration does NOT short-circuit. The
// response is always HTTP 200 on a successful store read, regardless of
// whether any individual publish failed.
//
// Security: this handler intentionally has no authentication. The service is
// deployed cluster-internal-only (no k8s Ingress); idempotency via
// deterministic UUID5 makes accidental replay safe.
func NewTriggerHandler(
	clock libtime.CurrentDateTimeGetter,
	scheduleStore store.ScheduleStore,
	pub publisher.Publisher,
) http.Handler {
	return libhttp.NewErrorHandler(libhttp.NewJSONHandler(
		libhttp.JSONHandlerFunc(
			func(ctx context.Context, req *http.Request) (any, error) {
				dt := libtime.ParseDateTimeDefault(
					ctx,
					req.URL.Query().Get("date"),
					clock.Now(),
				)
				date := schedule.NewDate(dt.Year(), dt.Month(), dt.Day())

				all, err := scheduleStore.List(ctx)
				if err != nil {
					return nil, errors.Wrap(ctx, err, "failed to read schedule inventory")
				}

				tasks := schedule.TasksForDate(all, date)
				sort.Slice(tasks, func(i, j int) bool { return tasks[i].Slug < tasks[j].Slug })

				dateStr := fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day)
				glog.V(2).Infof("trigger: processing %d task(s) for %s", len(tasks), dateStr)

				out := triggerResponse{
					Date:      dateStr,
					Published: 0,
					Errors:    []triggerErrorEntry{},
				}
				for _, def := range tasks {
					if pubErr := pub.Publish(ctx, def, date); pubErr != nil {
						glog.Errorf(
							"trigger: publish failed for slug %q on %s: %v",
							def.Slug,
							dateStr,
							pubErr,
						)
						out.Errors = append(out.Errors, triggerErrorEntry{
							Slug:  def.Slug,
							Error: pubErr.Error(),
						})
						continue
					}
					out.Published++
				}
				return out, nil
			},
		),
	))
}
