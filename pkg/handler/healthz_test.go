// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/handler"
)

var _ = Describe("HealthzHandler", func() {
	var httpHandler http.Handler
	BeforeEach(func() {
		httpHandler = handler.NewHealthzHandler()
	})

	It("returns 200 with application/json content type and the literal status body", func() {
		req := httptest.NewRequest("GET", "/healthz", nil)
		resp := httptest.NewRecorder()

		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(resp.Header().Get("Content-Type")).To(Equal("application/json"))
		Expect(resp.Body.String()).To(ContainSubstring(`"status":"ok"`))
	})

	It("does not depend on request body, query params, or method", func() {
		req := httptest.NewRequest("POST", "/healthz", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)
		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(resp.Body.String()).To(ContainSubstring(`"status":"ok"`))
	})
})
