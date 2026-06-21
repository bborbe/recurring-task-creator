// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// triggerTestDefs is the fixture the FakeScheduleStore returns for trigger tests.
// Covers all recurrence kinds and both weekday targets.
var triggerTestDefs = []schedule.TaskDefinition{
	{
		Slug:          "sat-task",
		TitleTemplate: "Sat Task",
		Recurrence:    schedule.RecurrenceWeekday,
		Weekday:       time.Saturday,
	},
	{
		Slug:          "sun-task",
		TitleTemplate: "Sun Task",
		Recurrence:    schedule.RecurrenceWeekday,
		Weekday:       time.Sunday,
	},
	{Slug: "monthly-task", TitleTemplate: "Monthly Task", Recurrence: schedule.RecurrenceMonthly},
	{
		Slug:          "quarterly-task",
		TitleTemplate: "Quarterly Task",
		Recurrence:    schedule.RecurrenceQuarterly,
	},
	{Slug: "yearly-task", TitleTemplate: "Yearly Task", Recurrence: schedule.RecurrenceYearly},
}

// countForDate returns the number of triggerTestDefs entries that fire on the given date.
func countForDate(date schedule.Date) int {
	return len(schedule.TasksForDate(triggerTestDefs, date))
}

var _ = Describe("TriggerHandler", func() {
	var (
		fakePublisher *mocks.PublisherPublisher
		fakeStore     *mocks.FakeScheduleStore
		clock         libtime.CurrentDateTime
		httpHandler   http.Handler
	)

	BeforeEach(func() {
		fakePublisher = &mocks.PublisherPublisher{}
		fakeStore = &mocks.FakeScheduleStore{}
		fakeStore.ListReturns(triggerTestDefs, nil)
		clock = libtime.NewCurrentDateTime()
		// 2025-01-04 Saturday 10:00 UTC
		clock.SetNow(libtimetest.ParseDateTime("2025-01-04T10:00:00Z"))
		httpHandler = handler.NewTriggerHandler(clock, fakeStore, fakePublisher)
	})

	// ---------- date fallback (no/empty/unparseable date query) ----------

	It("falls back to clock's civil date when no date query is set", func() {
		req := httptest.NewRequest("GET", "/trigger", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		// 2025-01-04 is Saturday; fixture fires 4 entries on Saturday.
		Expect(fakePublisher.PublishCallCount()).To(Equal(countForDate(
			schedule.NewDate(2025, time.January, 4))))

		var body struct {
			Date string `json:"date"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		Expect(body.Date).To(Equal("2025-01-04"))
	})

	It("falls back to clock when date query is empty", func() {
		req := httptest.NewRequest("GET", "/trigger?date=", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(fakePublisher.PublishCallCount()).To(Equal(countForDate(
			schedule.NewDate(2025, time.January, 4))))
	})

	It("falls back to clock when date query is unparseable", func() {
		req := httptest.NewRequest("GET", "/trigger?date=not-a-date", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		var body struct {
			Date string `json:"date"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		Expect(body.Date).To(Equal("2025-01-04"))
	})

	// ---------- 500 path (store error) ----------

	It("returns 500 with 'failed to read schedule inventory' when store.List fails", func() {
		fakeStore.ListReturns(nil, errors.New(context.Background(), "informer cache unavailable"))

		req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusInternalServerError))
		Expect(resp.Body.String()).To(ContainSubstring("failed to read schedule inventory"))
		Expect(fakePublisher.PublishCallCount()).To(Equal(0))
	})

	// ---------- happy path: fixture store, fake publisher ----------

	It("publishes every entry that fires on the given civil date", func() {
		date := schedule.NewDate(2025, time.January, 4) // Saturday
		want := countForDate(date)
		Expect(want).To(BeNumerically(">", 0))

		req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
		resp := httptest.NewRecorder()
		httpHandler.ServeHTTP(resp, req)

		Expect(resp.Code).To(Equal(http.StatusOK))
		Expect(fakePublisher.PublishCallCount()).To(Equal(want))
	})

	It("responds 200 with date, published=N, errors=[] when all publishes succeed", func() {
		fakePublisher.PublishReturns(nil)
		date := schedule.NewDate(2025, time.January, 4) // Saturday
		want := countForDate(date)

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
		Expect(body.Published).To(Equal(want))
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
		"returns 200 with errors[] populated and published=want-1 when one publish fails",
		func() {
			date := schedule.NewDate(2025, time.January, 4) // Saturday
			tasks := schedule.TasksForDate(triggerTestDefs, date)
			Expect(tasks).NotTo(BeEmpty())
			target := tasks[0].Slug

			fakePublisher.PublishCalls(
				func(ctx context.Context, def schedule.TaskDefinition, d schedule.Date) error {
					if def.Slug == target {
						return errors.New(ctx, "simulated publish failure")
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
			fakePublisher.PublishReturns(errors.New(context.Background(), "all down"))
			date := schedule.NewDate(2025, time.January, 4) // Saturday
			want := countForDate(date)

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
			Expect(body.Errors).To(HaveLen(want))
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

	Describe("date-filter behavior", func() {
		It(
			"publishes the Saturday weekday entry plus always-fire entries on a Saturday",
			func() {
				date := schedule.NewDate(2025, time.January, 4) // Saturday
				want := countForDate(date)
				// sat-task (weekday Saturday) + monthly + quarterly + yearly = 4
				Expect(want).To(Equal(4))

				req := httptest.NewRequest("GET", "/trigger?date=2025-01-04", nil)
				resp := httptest.NewRecorder()
				httpHandler.ServeHTTP(resp, req)

				Expect(resp.Code).To(Equal(http.StatusOK))
				Expect(fakePublisher.PublishCallCount()).To(Equal(want))
			},
		)

		It(
			"publishes the Sunday weekday entry plus always-fire entries on a Sunday",
			func() {
				date := schedule.NewDate(2025, time.January, 5) // Sunday
				want := countForDate(date)
				// sun-task (weekday Sunday) + monthly + quarterly + yearly = 4
				Expect(want).To(Equal(4))

				req := httptest.NewRequest("GET", "/trigger?date=2025-01-05", nil)
				resp := httptest.NewRecorder()
				httpHandler.ServeHTTP(resp, req)

				Expect(resp.Code).To(Equal(http.StatusOK))
				Expect(fakePublisher.PublishCallCount()).To(Equal(want))
			},
		)

		It("publishes 0 weekday-kind tasks on a Tuesday (regression fix)", func() {
			date := schedule.NewDate(2025, time.January, 7) // Tuesday
			tasks := schedule.TasksForDate(triggerTestDefs, date)
			weekdayKinds := 0
			for _, def := range tasks {
				if def.Recurrence == schedule.RecurrenceWeekday {
					weekdayKinds++
				}
			}
			Expect(weekdayKinds).To(Equal(0),
				"expected zero RecurrenceWeekday entries on a Tuesday, got %d", weekdayKinds)

			req := httptest.NewRequest("GET", "/trigger?date=2025-01-07", nil)
			resp := httptest.NewRecorder()
			httpHandler.ServeHTTP(resp, req)

			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(fakePublisher.PublishCallCount()).To(Equal(len(tasks)))
		})
	})
})
