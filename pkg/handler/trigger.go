// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/golang/glog"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
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
// publishes for one civil date. The date is supplied as the `date` query
// parameter in YYYY-MM-DD format. For each entry in the full inventory
// (schedule.Inventory(), slug-sorted), the handler calls
// publisher.Publish(req.Context(), def, date). Per-task errors are
// accumulated in the response's `errors` array — the iteration does
// NOT short-circuit on error. The response is always HTTP 200 on a
// successfully parsed date, regardless of whether any individual
// publish failed.
//
// The handler iterates the same set of entries the hourly tick iterates
// (the full inventory); per-day filtering is gone (Spec 7). Malformed or
// missing `date` parameter returns HTTP 400 with a JSON body of the form
// {"error":"<message>"}. The handler holds no per-request state and is
// safe to call concurrently for the same date (the controller dedups
// by deterministic UUID5).
//
// Security: this handler intentionally has no authentication. The
// service is deployed cluster-internal-only (no k8s Ingress); all
// external access is brokered by ~/Documents/workspaces/trading/frontend/
// gateway, which owns auth. The /trigger surface is reachable only
// inside the cluster. Idempotency via deterministic UUID5 also makes
// accidental replay safe.
func NewTriggerHandler(publisher publisher.Publisher) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		param := req.URL.Query().Get("date")
		if param == "" {
			writeTriggerError(resp, http.StatusBadRequest, "missing date parameter")
			return
		}
		// time.Parse here is input parsing, not a clock read; no time.Now().
		t, err := time.Parse("2006-01-02", param)
		if err != nil {
			writeTriggerError(
				resp,
				http.StatusBadRequest,
				"invalid date format, expected YYYY-MM-DD",
			)
			return
		}
		date := schedule.NewDate(t.Year(), t.Month(), t.Day())
		defs := schedule.Inventory()
		sort.Slice(defs, func(i, j int) bool { return defs[i].Slug < defs[j].Slug })
		tasks := defs

		glog.V(2).
			Infof("trigger: processing %d task(s) for %04d-%02d-%02d", len(tasks), date.Year, date.Month, date.Day)

		out := triggerResponse{
			Date:      fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day),
			Published: 0,
			Errors:    []triggerErrorEntry{},
		}
		for _, def := range tasks {
			if pubErr := publisher.Publish(req.Context(), def, date); pubErr != nil {
				glog.Errorf(
					"trigger: publish failed for slug %q on %s: %v",
					def.Slug,
					param,
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

		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(resp).Encode(out)
	})
}

// writeTriggerError writes a JSON error body with the given status and message.
// Used for the missing/invalid `date` parameter paths.
func writeTriggerError(resp http.ResponseWriter, status int, message string) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	_ = json.NewEncoder(resp).Encode(map[string]string{"error": message})
}
