// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
	"encoding/json"
	"net/http"
)

// NewHealthzHandler returns an HTTP handler that responds with HTTP 200 and
// a fixed JSON body {"status":"ok"}. It is the liveness-probe target for the
// Service in the Spec 5 manifests. No state, no I/O, no dependencies —
// safe to call at any cadence from any source.
func NewHealthzHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(resp).Encode(map[string]string{"status": "ok"})
	})
}
