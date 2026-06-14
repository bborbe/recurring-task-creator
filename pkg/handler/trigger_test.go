// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("TriggerHandler", func() {
	var (
		fakePublisher *mocks.PublisherPublisher
		httpHandler   http.Handler
	)

	BeforeEach(func() {
		fakePublisher = &mocks.PublisherPublisher{}
		httpHandler = handler.NewTriggerHandler(fakePublisher)
	})

	// ---------- 400 paths (missing/invalid date) ----------

	It("returns 400 with 'missing date parameter' when no date query is set", func() {
		req := httptest.NewRequest("GET", "/trigger", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		Expect(resp.Header().Get("Content-Type")).To(Equal("application/json"))
		Expect(resp.Body.String()).To(ContainSubstring("missing date parameter"))
		Expect(fakePublisher.PublishCallCount()).To(Equal(0))
	})

	It("returns 400 with 'missing date parameter' when date query is empty", func() {
		req := httptest.NewRequest("GET", "/trigger?date=", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		Expect(resp.Body.String()).To(ContainSubstring("missing date parameter"))
		Expect(fakePublisher.PublishCallCount()).To(Equal(0))
	})

	It("returns 400 with 'invalid date format' for non-date input", func() {
		req := httptest.NewRequest("GET", "/trigger?date=not-a-date", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		Expect(resp.Body.String()).To(ContainSubstring("invalid date format, expected YYYY-MM-DD"))
		Expect(fakePublisher.PublishCallCount()).To(Equal(0))
	})

	It("returns 400 with 'invalid date format' for day-of-month=32 (parse-fail)", func() {
		req := httptest.NewRequest("GET", "/trigger?date=2025-01-32", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusBadRequest))
		Expect(resp.Body.String()).To(ContainSubstring("invalid date format, expected YYYY-MM-DD"))
		Expect(fakePublisher.PublishCallCount()).To(Equal(0))
	})

	// ---------- happy path: real schedule, fake publisher ----------

	It("calls publisher.Publish once for every entry returned by schedule.TasksForDate", func() {
		date := schedule.NewDate(2025, time.January, 4)
		tasks := schedule.TasksForDate(date)

		req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
	})

	It("responds 200 with date, published=N, errors=[] when all publishes succeed", func() {
		date := schedule.NewDate(2025, time.January, 4)
		tasks := schedule.TasksForDate(date)
		fakePublisher.PublishReturns(nil)

		req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(resp.Header().Get("Content-Type")).To(Equal("application/json"))

		var body struct {
			Date      string `json:"date"`
			Published int    `json:"published"`
			Errors    []struct {
				Slug  string `json:"slug"`
				Error string `json:"error"`
			} `json:"errors"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		Expect(body.Date).To(Equal("2025-01-04"))
		Expect(body.Published).To(Equal(len(tasks)))
		Expect(body.Errors).To(BeEmpty())
	})

	It("serializes errors as [] (not null) when no errors occurred", func() {
		fakePublisher.PublishReturns(nil)

		req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Body.String()).To(ContainSubstring(`"errors":[]`))
	})

	It(
		"returns 200 with errors[] populated and published=len(tasks)-1 when one publish fails",
		func() {
			date := schedule.NewDate(2025, time.January, 4)
			tasks := schedule.TasksForDate(date)
			target := tasks[0].Slug

			// Use PublishStub (not PublishReturns) so the fake returns nil for
			// every call EXCEPT the one matching the target slug.
			fakePublisher.PublishCalls(
				func(ctx context.Context, def schedule.TaskDefinition, d schedule.Date) error {
					if def.Slug == target {
						return errors.New("simulated publish failure")
					}
					return nil
				},
			)

			req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
			resp := httptest.NewRecorder()
			httpHandler.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))

			var body struct {
				Date      string `json:"date"`
				Published int    `json:"published"`
				Errors    []struct {
					Slug  string `json:"slug"`
					Error string `json:"error"`
				} `json:"errors"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.Published).To(Equal(len(tasks) - 1))
			Expect(body.Errors).To(HaveLen(1))
			Expect(body.Errors[0].Slug).To(Equal(target))
			Expect(body.Errors[0].Error).To(ContainSubstring("simulated publish failure"))
		},
	)

	It(
		"returns 200 (not 5xx) with published=0 and full errors array when every publish fails",
		func() {
			date := schedule.NewDate(2025, time.January, 4)
			tasks := schedule.TasksForDate(date)
			fakePublisher.PublishReturns(errors.New("all down"))

			req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
			resp := httptest.NewRecorder()
			httpHandler.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))

			var body struct {
				Published int `json:"published"`
				Errors    []struct {
					Slug  string `json:"slug"`
					Error string `json:"error"`
				} `json:"errors"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.Published).To(Equal(0))
			Expect(body.Errors).To(HaveLen(len(tasks)))
		},
	)

	It("propagates the request context to publisher.Publish", func() {
		type ctxKey struct{}
		ctx := context.WithValue(context.Background(), ctxKey{}, "marker")
		req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil).WithContext(ctx)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(fakePublisher.PublishCallCount()).To(BeNumerically(">", 0))
		for i := 0; i < fakePublisher.PublishCallCount(); i++ {
			callCtx, _, _ := fakePublisher.PublishArgsForCall(i)
			Expect(callCtx.Value(ctxKey{})).To(Equal("marker"))
		}
	})
})
