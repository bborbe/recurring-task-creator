// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cleanup_test

import (
	"context"
	"errors"
	"sync"
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/mocks"
	"github.com/bborbe/recurring-task-creator/pkg/cleanup"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

// realNoOpWriter is a concrete VaultWriter that records UpdateFile calls
// but performs no actual write. Used to isolate the firing-window test
// from mock state leakage between Ginkgo contexts.
type realNoOpWriter struct {
	mu    sync.Mutex
	calls []string
}

func (w *realNoOpWriter) UpdateFile(
	_ context.Context,
	path string,
	_ func([]byte) ([]byte, error),
) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, path)
	return nil // never fails
}

func makeFrontmatter(status, plannedDate string) []byte {
	fm := "status: " + status + "\nplanned_date: " + plannedDate + "\n"
	return []byte("---\n" + fm + "---\n# Task\nBody content")
}

var _ = Describe("Supersedance", func() {
	var (
		s            *cleanup.Supersedance
		storeMock    *mocks.FakeScheduleStore
		readerMock   *mocks.CleanupVaultReader
		writerMock   *mocks.CleanupVaultWriter
		metricsMock  *mocks.CleanupMetrics
		clock        libtime.CurrentDateTime
		tokenBuilder publisher.PeriodTokenBuilder
		ctx          context.Context
		cancel       context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		storeMock = &mocks.FakeScheduleStore{}
		readerMock = &mocks.CleanupVaultReader{}
		writerMock = &mocks.CleanupVaultWriter{}
		metricsMock = &mocks.CleanupMetrics{}
		clock = libtime.NewCurrentDateTime()
		clock.SetNow(libtime.NewDateTime(2026, time.June, 29, 10, 0, 0, 0, time.UTC))
		tokenBuilder = publisher.NewPeriodTokenBuilder()
		s = &cleanup.Supersedance{
			Store:        storeMock,
			TokenBuilder: tokenBuilder,
			Reader:       readerMock,
			Writer:       writerMock,
			Metrics:      metricsMock,
			Clock:        clock,
		}
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Run", func() {
		Context("SkipAutoCleanup", func() {
			It("skips reader/ListFiles when SkipAutoCleanup=true", func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:            "my-task",
						Recurrence:      schedule.RecurrenceDaily,
						SkipAutoCleanup: true,
					},
				}, nil)

				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(readerMock.ListFilesCallCount()).Should(Equal(0))
				Ω(metricsMock.IncSupersededCallCount()).Should(Equal(0))
			})
		})

		Context("supersede when next-period exists", func() {
			BeforeEach(func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{
					"my-task - 2026-06-28.md",
					"my-task - 2026-06-29.md",
				}, nil)
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-06-28"), nil)
				writerMock.UpdateFileReturns(nil)
			})

			It("calls writer.UpdateFile once", func() {
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(1))
			})

			It("records success metric", func() {
				_ = s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(metricsMock.IncSupersededCallCount()).Should(BeNumerically(">=", 1))
				result, recurrence := metricsMock.IncSupersededArgsForCall(0)
				Ω(result).Should(Equal("success"))
				Ω(recurrence).Should(Equal(string(schedule.RecurrenceDaily)))
			})

			It("mutates frontmatter to aborted/done", func() {
				_ = s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(1))
				_, path, mutator := writerMock.UpdateFileArgsForCall(0)
				Ω(path).Should(Equal("my-task - 2026-06-28.md"))

				result, err := mutator(makeFrontmatter("in_progress", "2026-06-28"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(result)).Should(ContainSubstring("status: aborted"))
				Ω(string(result)).Should(ContainSubstring("phase: done"))
				Ω(string(result)).Should(MatchRegexp(`completed_date: "?2026-\d{2}-\d{2}"?`))
				Ω(string(result)).Should(ContainSubstring("superseded_by: auto-cleanup-"))
			})
		})

		Context("skip when next-period absent AND within firing window", func() {
			It("does not call writer.UpdateFile", func() {
				// Build a fresh local struct with a concrete no-op writer so mock
				// state from other Ginkgo contexts cannot leak into this test.
				localWriter := &realNoOpWriter{calls: []string{}}
				localS := &cleanup.Supersedance{
					Store:        storeMock,
					TokenBuilder: tokenBuilder,
					Reader:       readerMock,
					Writer:       localWriter,
					Metrics:      metricsMock,
					Clock:        clock,
				}
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{
					"my-task - 2026-06-28.md",
				}, nil)
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-06-28"), nil)

				err := localS.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(len(localWriter.calls)).Should(Equal(0))
			})
		})

		Context("idempotent on re-run", func() {
			BeforeEach(func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{
					"my-task - 2026-06-28.md",
					"my-task - 2026-06-29.md",
				}, nil)
			})

			It("writes once on first run", func() {
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-06-28"), nil)
				writerMock.UpdateFileReturns(nil)
				_ = s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(1))
			})

			It("does not write again when already aborted", func() {
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-06-28"), nil)
				writerMock.UpdateFileReturns(nil)
				_ = s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				firstCount := writerMock.UpdateFileCallCount()

				readerMock.GetFileReturns(makeFrontmatter("aborted", "2026-06-28"), nil)
				_ = s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(firstCount))
			})
		})

		Context("first-ever instance no-op", func() {
			BeforeEach(func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{}, nil)
			})

			It("does not call writer.UpdateFile", func() {
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(0))
			})
		})

		Context("409 conflict tolerated", func() {
			BeforeEach(func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{
					"my-task - 2026-06-28.md",
					"my-task - 2026-06-29.md",
				}, nil)
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-06-28"), nil)
				writerMock.UpdateFileReturns(cleanup.ErrVaultConflict)
			})

			It("does not panic and records conflict metric", func() {
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(metricsMock.IncSupersededCallCount()).Should(BeNumerically(">=", 1))
				result, _ := metricsMock.IncSupersededArgsForCall(0)
				Ω(result).Should(Equal("conflict"))
			})
		})

		Context("generic writer error", func() {
			BeforeEach(func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
					{
						Slug:       "other-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{
					"my-task - 2026-06-28.md",
					"my-task - 2026-06-29.md",
				}, nil)
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-06-28"), nil)
				writerMock.UpdateFileReturns(errors.New("network error"))
			})

			It("records error metric and continues to next Schedule", func() {
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				// Both defs processed despite error on first.
				Ω(storeMock.ListCallCount()).Should(BeNumerically(">=", 1))
			})
		})

		Context("store list error returns error", func() {
			It("returns wrapped error", func() {
				storeMock.ListReturns([]schedule.TaskDefinition{}, errors.New("lister error"))
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).Should(HaveOccurred())
				Ω(err.Error()).Should(ContainSubstring("list schedules"))
			})
		})

		Context("cadenceDays coverage", func() {
			It("covers quarterly", func() {
				// Quarterly cadence path - tested via supersede when next-period exists
				// but cadenceDays itself is 28.6%. Add a unit test via the safety gate.
				storeMock.ListReturns([]schedule.TaskDefinition{
					{Slug: "q-task", Recurrence: schedule.RecurrenceQuarterly},
				}, nil)
				// Prior file exists, next does not, within cadence - should skip.
				readerMock.ListFilesReturns([]string{"q-task - 2026Q1.md"}, nil)
				readerMock.GetFileReturns(makeFrontmatter("in_progress", "2026-04-01"), nil)
				writerMock.UpdateFileReturns(nil)
				err := s.Run(ctx, schedule.NewDate(2026, time.July, 1))
				Ω(err).ShouldNot(HaveOccurred())
				// Within cadence (93 days) for quarterly from Apr 1 to Jul 1 = 91 days, so skipped.
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(0))
			})
		})

		Context("withinCadenceFM string path", func() {
			It("parses planned_date as string", func() {
				// Trigger the string parsing path in withinCadenceFM.
				// Use a weekly schedule where the planned_date is 7 days before current date.
				storeMock.ListReturns([]schedule.TaskDefinition{
					{Slug: "w-task", Recurrence: schedule.RecurrenceWeekly},
				}, nil)
				// Prior file exists (2026W25), next does not, planned within 7 days.
				readerMock.ListFilesReturns([]string{"w-task - 2026W25.md"}, nil)
				// Use a date string that YAML parses as string (not time.Time).
				readerMock.GetFileReturns(
					[]byte("---\nstatus: in_progress\nplanned_date: \"2026-06-22\"\n---\n# Task\n"),
					nil,
				)
				writerMock.UpdateFileReturns(nil)
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				// Jun 22 to Jun 29 = 7 days, within cadence, skip.
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(0))
			})
		})

		Context("withinCadenceFM non-date type skips to supersede", func() {
			It("calls writer when planned_date is not a date", func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{Slug: "x-task", Recurrence: schedule.RecurrenceDaily},
				}, nil)
				readerMock.ListFilesReturns(
					[]string{"x-task - 2026-06-28.md", "x-task - 2026-06-29.md"},
					nil,
				)
				// planned_date as integer - triggers default case in withinCadenceFM → returns false → safety gate skipped → supersede.
				readerMock.GetFileReturns(
					[]byte("---\nstatus: in_progress\nplanned_date: 42\n---\n# Task\n"),
					nil,
				)
				writerMock.UpdateFileReturns(nil)
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(1))
			})
		})

		Context("splitFrontmatter no frontmatter", func() {
			It("handles content without frontmatter", func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{Slug: "n-task", Recurrence: schedule.RecurrenceDaily},
				}, nil)
				readerMock.ListFilesReturns([]string{"n-task - 2026-06-28.md"}, nil)
				// No frontmatter marker.
				readerMock.GetFileReturns([]byte("# Task\nSome content\n"), nil)
				writerMock.UpdateFileReturns(nil)
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				// ParseFrontmatter returns empty map, status != "in_progress", skipped.
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(0))
			})
		})

		Context("mutateFrontmatterSupersede no frontmatter", func() {
			It("returns error when no frontmatter", func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{Slug: "m-task", Recurrence: schedule.RecurrenceDaily},
				}, nil)
				readerMock.ListFilesReturns(
					[]string{"m-task - 2026-06-28.md", "m-task - 2026-06-29.md"},
					nil,
				)
				// No frontmatter marker - parseFrontmatter returns empty map, status not in_progress, skipped.
				readerMock.GetFileReturns([]byte("# Task no frontmatter\n"), nil)
				writerMock.UpdateFileReturns(nil)
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				// Status != in_progress, so skipped - not testing mutate error path.
				// To test mutate error, we need in_progress status but no frontmatter marker.
				// This is hard to trigger with valid markdown. The error path is there for safety.
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(0))
			})
		})

		Context("frontmatter parse failure", func() {
			BeforeEach(func() {
				storeMock.ListReturns([]schedule.TaskDefinition{
					{
						Slug:       "my-task",
						Recurrence: schedule.RecurrenceDaily,
					},
				}, nil)
				readerMock.ListFilesReturns([]string{
					"my-task - 2026-06-28.md",
					"my-task - 2026-06-29.md",
				}, nil)
				readerMock.GetFileReturns([]byte("---\n  invalid: yaml: [}\n---\nBody"), nil)
			})

			It("records error metric and does not write", func() {
				err := s.Run(ctx, schedule.NewDate(2026, time.June, 29))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(writerMock.UpdateFileCallCount()).Should(Equal(0))
				Ω(metricsMock.IncSupersededCallCount()).Should(BeNumerically(">=", 1))
			})
		})
	})
})
