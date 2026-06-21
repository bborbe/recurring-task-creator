// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"
	"fmt"
	"time"

	lib "github.com/bborbe/agent/lib"
	"github.com/bborbe/agent/lib/command/task"
	taskmocks "github.com/bborbe/agent/lib/mocks"
	"github.com/bborbe/errors"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var _ = Describe("Publisher", func() {
	var (
		sender *taskmocks.TaskCreateCommandSender
		pub    publisher.Publisher
	)
	BeforeEach(func() {
		sender = &taskmocks.TaskCreateCommandSender{}
		sender.SendCommandReturns(nil)
		pub = publisher.NewPublisher(
			sender,
			publisher.NewRenderer(),
			publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
			publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
			false,
		)
	})

	capture := func() task.CreateCommand {
		_, cmd := sender.SendCommandArgsForCall(0)
		return cmd
	}

	Describe("identifier", func() {
		It("is the UUID5 of the canonical key", func() {
			def := schedule.TaskDefinition{
				Slug:          "weekly-review",
				TitleTemplate: "Weekly Review {{current_week}}",
				Recurrence:    schedule.RecurrenceWeekday,
				Weekday:       time.Saturday,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			captured := capture()
			expected := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte("recurring-weekly-review-2025W01-sat"),
			).String()
			Expect(string(captured.TaskIdentifier)).To(Equal(expected))
		})
	})

	Describe("period anchoring", func() {
		captureIdentifier := func(
			slug string,
			rec schedule.RecurrenceKind,
			date schedule.Date,
		) lib.TaskIdentifier {
			// Use a fresh sender per call so SendCommandArgsForCall(0)
			// always points at the most recent Publish — the parent
			// suite's capture() reads call index 0.
			localSender := &taskmocks.TaskCreateCommandSender{}
			localSender.SendCommandReturns(nil)
			localPub := publisher.NewPublisher(
				localSender,
				publisher.NewRenderer(),
				publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
				publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
				false,
			)
			def := schedule.TaskDefinition{
				Slug:          slug,
				TitleTemplate: "t",
				Recurrence:    rec,
			}
			if rec == schedule.RecurrenceWeekday {
				def.Weekday = time.Saturday
			}
			Expect(localPub.Publish(context.Background(), def, date)).To(Succeed())
			_, cmd := localSender.SendCommandArgsForCall(0)
			return cmd.TaskIdentifier
		}

		It("weekly: same ISO week, different civil dates produce the same identifier", func() {
			// 2025-06-09 (Mon) and 2025-06-15 (Sun) are both in ISO 2025W24.
			id1 := captureIdentifier(
				"w1",
				schedule.RecurrenceWeekly,
				schedule.NewDate(2025, time.June, 9),
			)
			id2 := captureIdentifier(
				"w1",
				schedule.RecurrenceWeekly,
				schedule.NewDate(2025, time.June, 15),
			)
			Expect(id1).To(Equal(id2))
		})

		It("monthly: same month, different civil dates produce the same identifier", func() {
			id1 := captureIdentifier(
				"m1",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2025, time.June, 1),
			)
			id2 := captureIdentifier(
				"m1",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2025, time.June, 30),
			)
			Expect(id1).To(Equal(id2))
		})

		It("quarterly: same quarter, different civil dates produce the same identifier", func() {
			id1 := captureIdentifier(
				"q1",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2025, time.April, 1),
			)
			id2 := captureIdentifier(
				"q1",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2025, time.June, 30),
			)
			Expect(id1).To(Equal(id2))
		})

		It("yearly: same year, different civil dates produce the same identifier", func() {
			id1 := captureIdentifier(
				"y1",
				schedule.RecurrenceYearly,
				schedule.NewDate(2025, time.January, 1),
			)
			id2 := captureIdentifier(
				"y1",
				schedule.RecurrenceYearly,
				schedule.NewDate(2025, time.December, 31),
			)
			Expect(id1).To(Equal(id2))
		})

		It("weekly: adjacent ISO weeks produce different identifiers", func() {
			// 2025-06-15 (Sun) is 2025W24; 2025-06-16 (Mon) is 2025W25.
			id1 := captureIdentifier(
				"w1",
				schedule.RecurrenceWeekly,
				schedule.NewDate(2025, time.June, 15),
			)
			id2 := captureIdentifier(
				"w1",
				schedule.RecurrenceWeekly,
				schedule.NewDate(2025, time.June, 16),
			)
			Expect(id1).NotTo(Equal(id2))
		})

		It("monthly: adjacent months produce different identifiers", func() {
			id1 := captureIdentifier(
				"m1",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2025, time.May, 31),
			)
			id2 := captureIdentifier(
				"m1",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2025, time.June, 1),
			)
			Expect(id1).NotTo(Equal(id2))
		})

		It("quarterly: adjacent quarters produce different identifiers", func() {
			id1 := captureIdentifier(
				"q1",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2025, time.June, 30),
			)
			id2 := captureIdentifier(
				"q1",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2025, time.July, 1),
			)
			Expect(id1).NotTo(Equal(id2))
		})

		It("yearly: adjacent years produce different identifiers", func() {
			id1 := captureIdentifier(
				"y1",
				schedule.RecurrenceYearly,
				schedule.NewDate(2025, time.December, 31),
			)
			id2 := captureIdentifier(
				"y1",
				schedule.RecurrenceYearly,
				schedule.NewDate(2026, time.January, 1),
			)
			Expect(id1).NotTo(Equal(id2))
		})

		It("daily: distinct civil dates produce distinct identifiers", func() {
			id1 := captureIdentifier(
				"d1",
				schedule.RecurrenceDaily,
				schedule.NewDate(2025, time.June, 14),
			)
			id2 := captureIdentifier(
				"d1",
				schedule.RecurrenceDaily,
				schedule.NewDate(2025, time.June, 15),
			)
			Expect(id1).NotTo(Equal(id2))
		})

		DescribeTable(
			"period-token byte-equality with the formatter output",
			func(rec schedule.RecurrenceKind, date schedule.Date, expectedToken string) {
				slug := "byte-eq-" + string(rec)
				def := schedule.TaskDefinition{
					Slug:          slug,
					TitleTemplate: "t",
					Recurrence:    rec,
				}
				if rec == schedule.RecurrenceWeekday {
					def.Weekday = time.Monday
				}
				Expect(pub.Publish(context.Background(), def, date)).To(Succeed())
				cmd := capture()
				expected := "recurring-" + slug + "-" + expectedToken
				want := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte(expected)).String()
				Expect(string(cmd.TaskIdentifier)).To(Equal(want))
			},
			Entry(
				"daily",
				schedule.RecurrenceDaily,
				schedule.NewDate(2025, time.June, 14),
				"2025-06-14",
			),
			Entry(
				"weekly",
				schedule.RecurrenceWeekly,
				schedule.NewDate(2025, time.June, 9),
				"2025W24",
			),
			Entry(
				"weekday",
				schedule.RecurrenceWeekday,
				schedule.NewDate(2025, time.June, 9),
				"2025W24-mon",
			),
			Entry(
				"monthly",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2025, time.June, 1),
				"2025-06",
			),
			Entry(
				"quarterly",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2025, time.April, 1),
				"2025Q2",
			),
			Entry(
				"yearly",
				schedule.RecurrenceYearly,
				schedule.NewDate(2025, time.January, 1),
				"2025",
			),
		)

		It("weekday: byte-equality with the formatter output (with weekday suffix)", func() {
			// 2025-06-09 (Mon) is in ISO 2025W24; with Weekday=time.Saturday
			// the period token must be "2025W24-sat".
			def := schedule.TaskDefinition{
				Slug:          "byte-eq-weekday",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekday,
				Weekday:       time.Saturday,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.June, 9),
			)).To(Succeed())
			cmd := capture()
			expected := "recurring-byte-eq-weekday-2025W24-sat"
			want := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte(expected),
			).String()
			Expect(string(cmd.TaskIdentifier)).To(Equal(want))
		})

		It("weekly: byte-equality with the formatter output (no weekday suffix)", func() {
			// After spec 009, RecurrenceWeekly is always-fire and the period
			// token is bare YYYYWww (no weekday suffix). The Weekday field is
			// ignored for this kind.
			def := schedule.TaskDefinition{
				Slug:          "byte-eq-weekly-bare",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.June, 9),
			)).To(Succeed())
			cmd := capture()
			expected := "recurring-byte-eq-weekly-bare-2025W24"
			want := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte(expected),
			).String()
			Expect(string(cmd.TaskIdentifier)).To(Equal(want))
		})
	})

	It(
		"buildPeriodToken: weekday token carries the entry's Weekday, not the date's weekday",
		func() {
			// 2026-06-17 is a Wednesday, in ISO 2026W25. With Weekday=time.Saturday
			// on the def, the period token must be "2026W25-sat" (NOT "2026W25-wed").
			def := schedule.TaskDefinition{
				Slug:          "weekday-takes-precedence",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekday,
				Weekday:       time.Saturday,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2026, time.June, 17),
			)).To(Succeed())
			captured := capture()
			expected := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte("recurring-weekday-takes-precedence-2026W25-sat"),
			).String()
			Expect(string(captured.TaskIdentifier)).To(Equal(expected))
		},
	)

	It("non-weekly kinds ignore the Weekday field (token is identical to Spec 6)", func() {
		for _, c := range []struct {
			rec schedule.RecurrenceKind
			d   schedule.Date
			tok string
		}{
			{schedule.RecurrenceDaily, schedule.NewDate(2025, time.June, 14), "2025-06-14"},
			{schedule.RecurrenceMonthly, schedule.NewDate(2025, time.June, 1), "2025-06"},
			{schedule.RecurrenceQuarterly, schedule.NewDate(2025, time.April, 1), "2025Q2"},
			{schedule.RecurrenceYearly, schedule.NewDate(2025, time.January, 1), "2025"},
		} {
			// Weekday deliberately non-zero to prove it is ignored for non-weekly kinds.
			def := schedule.TaskDefinition{
				Slug:          "non-weekly-" + string(c.rec),
				TitleTemplate: "t",
				Recurrence:    c.rec,
				Weekday:       time.Wednesday,
			}
			// Use a fresh sender per iteration so SendCommandArgsForCall(0)
			// always points at the most recent Publish.
			localSender := &taskmocks.TaskCreateCommandSender{}
			localSender.SendCommandReturns(nil)
			localPub := publisher.NewPublisher(
				localSender,
				publisher.NewRenderer(),
				publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
				publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
				false,
			)
			Expect(localPub.Publish(context.Background(), def, c.d)).To(Succeed())
			_, cmd := localSender.SendCommandArgsForCall(0)
			want := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte("recurring-non-weekly-"+string(c.rec)+"-"+c.tok),
			).String()
			Expect(string(cmd.TaskIdentifier)).To(Equal(want))
		}
	})

	Describe("errors", func() {
		It("returns a wrapped error for an unknown recurrence kind", func() {
			def := schedule.TaskDefinition{
				Slug:          "unknown-rec",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceKind("unknown"),
			}
			err := pub.Publish(context.Background(), def, schedule.NewDate(2025, time.June, 14))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown"))
			Expect(err.Error()).To(ContainSubstring("unknown-rec")) // slug is in the wrap
			Expect(sender.SendCommandCallCount()).To(Equal(0))
		})
	})

	Describe("determinism", func() {
		It("two calls with the same (def, date) produce deep-equal commands", func() {
			def := schedule.TaskDefinition{
				Slug:          "weekly-review",
				TitleTemplate: "Title {{current_date}}",
				BodyTemplate:  "Body {{current_date}}",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			date := schedule.NewDate(2025, time.January, 4)
			Expect(pub.Publish(context.Background(), def, date)).To(Succeed())
			Expect(pub.Publish(context.Background(), def, date)).To(Succeed())
			Expect(sender.SendCommandCallCount()).To(Equal(2))
			_, cmd1 := sender.SendCommandArgsForCall(0)
			_, cmd2 := sender.SendCommandArgsForCall(1)
			Expect(cmd1).To(Equal(cmd2))
		})
	})

	Describe("placeholder rendering", func() {
		type renderCase struct {
			name     string
			template string
			date     schedule.Date
			want     string
		}
		DescribeTable("renders placeholders in TitleTemplate",
			func(c renderCase) {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: c.template,
					Recurrence:    schedule.RecurrenceWeekly,
				}
				Expect(pub.Publish(context.Background(), def, c.date)).To(Succeed())
				Expect(capture().Title).To(Equal(c.want))
			},
			Entry("{{current_date}}", renderCase{
				name:     "{{current_date}}",
				template: "prefix {{current_date}} suffix",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "prefix 2025-01-04 suffix - 2025W01",
			}),
			Entry("{{current_week}}", renderCase{
				name:     "{{current_week}}",
				template: "Week {{current_week}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Week 2025W01 - 2025W01",
			}),
			Entry("{{next_week}}", renderCase{
				name:     "{{next_week}}",
				template: "Next {{next_week}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Next 2025W02 - 2025W01",
			}),
			Entry("{{current_month}}", renderCase{
				name:     "{{current_month}}",
				template: "Month {{current_month}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Month 2025-01 - 2025W01",
			}),
			Entry("{{last_month}} with year roll-back", renderCase{
				name:     "{{last_month}}",
				template: "Last {{last_month}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Last 2024-12 - 2025W01",
			}),
			Entry("{{current_quarter}}", renderCase{
				name:     "{{current_quarter}}",
				template: "Q {{current_quarter}}",
				date:     schedule.NewDate(2025, time.April, 1),
				want:     "Q 2025Q2 - 2025W14",
			}),
			Entry("{{last_quarter}} with year roll-back", renderCase{
				name:     "{{last_quarter}}",
				template: "Last {{last_quarter}}",
				date:     schedule.NewDate(2025, time.January, 1),
				want:     "Last 2024Q4 - 2025W01",
			}),
			Entry("{{current_year}}", renderCase{
				name:     "{{current_year}}",
				template: "Year {{current_year}}",
				date:     schedule.NewDate(2025, time.April, 1),
				want:     "Year 2025 - 2025W14",
			}),
			Entry("{{last_year}}", renderCase{
				name:     "{{last_year}}",
				template: "Last {{last_year}}",
				date:     schedule.NewDate(2025, time.January, 1),
				want:     "Last 2024 - 2025W01",
			}),
		)

		It("renders placeholders in BodyTemplate", func() {
			def := schedule.TaskDefinition{
				Slug:          "test-slug",
				TitleTemplate: "t",
				BodyTemplate:  "body contains {{current_date}}",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			Expect(capture().Body).To(Equal("body contains 2025-01-04"))
		})

		It("renders the ISO-week year (not calendar year) at year boundary", func() {
			def := schedule.TaskDefinition{
				Slug:          "test-slug",
				TitleTemplate: "{{current_week}}",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			// 2024-12-30 (Monday) belongs to ISO 2025W01.
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2024, time.December, 30),
			)).To(Succeed())
			Expect(capture().Title).To(Equal("2025W01 - 2025W01"))
		})
	})

	Describe("title suffix", func() {
		It("appends the period token to a monthly title", func() {
			def := schedule.TaskDefinition{
				Slug:          "monthly-15",
				TitleTemplate: "Update K3s",
				Recurrence:    schedule.RecurrenceMonthly,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2026, time.June, 15),
			)).To(Succeed())
			Expect(capture().Title).To(Equal("Update K3s - 2026-06"))
		})

		It("appends the period token to a weekday title (with weekday suffix)", func() {
			def := schedule.TaskDefinition{
				Slug:          "weekday-sat-1",
				TitleTemplate: "Shutdown K3s",
				Recurrence:    schedule.RecurrenceWeekday,
				Weekday:       time.Saturday,
			}
			// 2026-06-17 is a Wednesday in ISO 2026W25.
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2026, time.June, 17),
			)).To(Succeed())
			Expect(capture().Title).To(Equal("Shutdown K3s - 2026W25-sat"))
		})

		It("trims whitespace from the rendered title before appending the suffix", func() {
			def := schedule.TaskDefinition{
				Slug:          "trailing-space",
				TitleTemplate: "Trailing Space ",
				Recurrence:    schedule.RecurrenceMonthly,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2026, time.June, 15),
			)).To(Succeed())
			Expect(capture().Title).To(Equal("Trailing Space - 2026-06"))
		})

		It(
			"renders bare placeholders-only templates as '<token>' after substitution and suffix",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "placeholder-only",
					TitleTemplate: "{{current_month}}",
					Recurrence:    schedule.RecurrenceMonthly,
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2026, time.June, 15),
				)).To(Succeed())
				Expect(capture().Title).To(Equal("2026-06 - 2026-06"))
			},
		)

		DescribeTable(
			"appends '<bare> - <period-token>' for every RecurrenceKind",
			func(rec schedule.RecurrenceKind, date schedule.Date, expectedToken string) {
				localSender := &taskmocks.TaskCreateCommandSender{}
				localSender.SendCommandReturns(nil)
				localPub := publisher.NewPublisher(
					localSender,
					publisher.NewRenderer(),
					publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
					publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
					false,
				)
				def := schedule.TaskDefinition{
					Slug:          "kind-" + string(rec),
					TitleTemplate: "Bare",
					Recurrence:    rec,
					Weekday:       time.Saturday,
				}
				Expect(localPub.Publish(context.Background(), def, date)).To(Succeed())
				_, cmd := localSender.SendCommandArgsForCall(0)
				Expect(cmd.Title).To(Equal("Bare - " + expectedToken))
			},
			Entry(
				"daily",
				schedule.RecurrenceDaily,
				schedule.NewDate(2026, time.June, 15),
				"2026-06-15",
			),
			Entry(
				"weekly",
				schedule.RecurrenceWeekly,
				schedule.NewDate(2026, time.June, 17),
				"2026W25",
			),
			Entry(
				"weekday",
				schedule.RecurrenceWeekday,
				schedule.NewDate(2026, time.June, 17),
				"2026W25-sat",
			),
			Entry(
				"monthly",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2026, time.June, 15),
				"2026-06",
			),
			Entry(
				"quarterly",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2026, time.April, 1),
				"2026Q2",
			),
			Entry(
				"yearly",
				schedule.RecurrenceYearly,
				schedule.NewDate(2026, time.January, 1),
				"2026",
			),
		)

		DescribeTable(
			"applies PeriodOffset to period token (and to UUID5 input)",
			func(
				rec schedule.RecurrenceKind,
				date schedule.Date,
				offset int,
				expectedToken string,
			) {
				localSender := &taskmocks.TaskCreateCommandSender{}
				localSender.SendCommandReturns(nil)
				localPub := publisher.NewPublisher(
					localSender,
					publisher.NewRenderer(),
					publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
					publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
					false,
				)
				def := schedule.TaskDefinition{
					Slug:          "offset-" + string(rec),
					TitleTemplate: "Review",
					Recurrence:    rec,
					PeriodOffset:  offset,
				}
				Expect(localPub.Publish(context.Background(), def, date)).To(Succeed())
				_, cmd := localSender.SendCommandArgsForCall(0)
				Expect(cmd.Title).To(Equal("Review - " + expectedToken))

				expectedID, err := publisher.NewPeriodTokenBuilder().
					Build(context.Background(), def, date)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(expectedID)).To(Equal(expectedToken))
			},
			Entry(
				"monthly offset=-1 names prior month",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2026, time.July, 1),
				-1,
				"2026-06",
			),
			Entry(
				"monthly offset=-1 across year boundary",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2026, time.January, 1),
				-1,
				"2025-12",
			),
			Entry(
				"monthly offset=+1 names next month",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2026, time.June, 15),
				1,
				"2026-07",
			),
			Entry(
				"quarterly offset=-1 names prior quarter",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2026, time.July, 1),
				-1,
				"2026Q2",
			),
			Entry(
				"quarterly offset=-1 across year boundary",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2026, time.January, 1),
				-1,
				"2025Q4",
			),
			Entry(
				"quarterly offset=-3 wraps three quarters back",
				schedule.RecurrenceQuarterly,
				schedule.NewDate(2026, time.October, 1),
				-3,
				"2026Q1",
			),
			Entry(
				"yearly offset=-1 names prior year",
				schedule.RecurrenceYearly,
				schedule.NewDate(2026, time.January, 1),
				-1,
				"2025",
			),
			Entry(
				"yearly offset=+1 names next year",
				schedule.RecurrenceYearly,
				schedule.NewDate(2026, time.January, 1),
				1,
				"2027",
			),
			Entry(
				"monthly offset=0 unchanged (default behavior)",
				schedule.RecurrenceMonthly,
				schedule.NewDate(2026, time.June, 15),
				0,
				"2026-06",
			),
		)
	})

	Describe("per-kind render", func() {
		It("every recurrence kind renders to a title ending in ' - <period-token>'", func() {
			// Cross-check: prove that the publisher suffix is consistent with
			// buildPeriodToken for each recurrence kind. The static inventory
			// is gone; one representative entry per kind covers the same invariant.
			// 2026-06-15 is a Monday (CEST), suitable for all always-fire kinds
			// and for a Saturday weekday entry (which will produce the sat suffix).
			refDate := schedule.NewDate(2026, time.June, 15)
			fixture := []schedule.TaskDefinition{
				{Slug: "daily-x", TitleTemplate: "Daily", Recurrence: schedule.RecurrenceDaily},
				{Slug: "weekly-x", TitleTemplate: "Weekly", Recurrence: schedule.RecurrenceWeekly},
				{
					Slug:          "weekday-sat",
					TitleTemplate: "Sat Task",
					Recurrence:    schedule.RecurrenceWeekday,
					Weekday:       time.Saturday,
				},
				{
					Slug:          "monthly-x",
					TitleTemplate: "Monthly",
					Recurrence:    schedule.RecurrenceMonthly,
				},
				{
					Slug:          "quarterly-x",
					TitleTemplate: "Quarterly",
					Recurrence:    schedule.RecurrenceQuarterly,
				},
				{Slug: "yearly-x", TitleTemplate: "Yearly", Recurrence: schedule.RecurrenceYearly},
			}
			for _, def := range fixture {
				// Use a fresh sender per entry so SendCommandArgsForCall(0)
				// always points at the most recent Publish.
				localSender := &taskmocks.TaskCreateCommandSender{}
				localSender.SendCommandReturns(nil)
				localPub := publisher.NewPublisher(
					localSender,
					publisher.NewRenderer(),
					publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
					publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
					false,
				)
				Expect(localPub.Publish(context.Background(), def, refDate)).To(Succeed())
				_, cmd := localSender.SendCommandArgsForCall(0)
				expectedToken, err := publisher.NewPeriodTokenBuilder().
					Build(context.Background(), def, refDate)
				Expect(err).NotTo(HaveOccurred(), def.Slug)
				expectedSuffix := " - " + string(expectedToken)
				Expect(cmd.Title).To(HaveSuffix(expectedSuffix),
					"entry %q rendered title %q does not end with %q",
					def.Slug, cmd.Title, expectedSuffix)
			}
		})
	})

	Describe("frontmatter", func() {
		It(
			"emits the published-author defaults + forced provenance when the operator supplies no frontmatter",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				fm := capture().Frontmatter
				Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
				Expect(fm).To(HaveKeyWithValue("page_type", "task"))
				Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
				Expect(fm).To(HaveLen(3))
				Expect(fm).NotTo(HaveKey("recurring"))
			},
		)

		It(
			"merges operator-defined frontmatter from TaskDefinition.Frontmatter",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
					Frontmatter: lib.TaskFrontmatter{
						"assignee": "alice",
						"priority": 4,
						"goals":    []interface{}{"[[Example Goal]]"},
						"category": "ops",
					},
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				fm := capture().Frontmatter
				Expect(fm).To(HaveKeyWithValue("assignee", "alice"))
				Expect(fm).To(HaveKeyWithValue("priority", 4))
				Expect(fm).To(HaveKeyWithValue("goals", []interface{}{"[[Example Goal]]"}))
				Expect(fm).To(HaveKeyWithValue("category", "ops"))
				Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
				Expect(fm).To(HaveKeyWithValue("page_type", "task"))
				Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
				Expect(fm).To(HaveLen(7))
			},
		)

		It(
			"operator can override the published-author defaults (status, page_type)",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
					Frontmatter: lib.TaskFrontmatter{
						"status":    "draft",
						"page_type": "log",
					},
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				fm := capture().Frontmatter
				Expect(fm).To(HaveKeyWithValue("status", "draft"))
				Expect(fm).To(HaveKeyWithValue("page_type", "log"))
				Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
				Expect(fm).To(HaveLen(3))
			},
		)

		It(
			"operator cannot override the forced provenance key (created_by)",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
					Frontmatter: lib.TaskFrontmatter{
						"created_by": "operator-tries-to-impersonate",
					},
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				fm := capture().Frontmatter
				Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
				Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
				Expect(fm).To(HaveKeyWithValue("page_type", "task"))
				Expect(fm).To(HaveLen(3))
			},
		)

		It(
			"renders placeholders in operator-supplied string frontmatter values",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekday,
					Weekday:       time.Saturday,
					Frontmatter: lib.TaskFrontmatter{
						"planned_date": "{{current_date}}",
						"due_date":     "{{current_date}}",
						"period_week":  "{{current_week}}",
						"period_month": "{{current_month}}",
						// Non-string values must pass through unchanged.
						"priority": 4,
						"goals":    []interface{}{"[[Example Goal]]"},
					},
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2026, time.June, 20),
				)).To(Succeed())
				fm := capture().Frontmatter
				Expect(fm).To(HaveKeyWithValue("planned_date", "2026-06-20"))
				Expect(fm).To(HaveKeyWithValue("due_date", "2026-06-20"))
				Expect(fm).To(HaveKeyWithValue("period_week", "2026W25"))
				Expect(fm).To(HaveKeyWithValue("period_month", "2026-06"))
				Expect(fm).To(HaveKeyWithValue("priority", 4))
				Expect(fm).To(HaveKeyWithValue("goals", []interface{}{"[[Example Goal]]"}))
			},
		)

		It(
			"leaves operator strings without placeholders unchanged",
			func() {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
					Frontmatter: lib.TaskFrontmatter{
						"assignee": "alice",
						"category": "ops",
					},
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				fm := capture().Frontmatter
				Expect(fm).To(HaveKeyWithValue("assignee", "alice"))
				Expect(fm).To(HaveKeyWithValue("category", "ops"))
			},
		)

		It("does not depend on the entry's RecurrenceKind (no kind-specific keys)", func() {
			// After spec 008 the frontmatter shape is identical for every
			// RecurrenceKind — there is no kind-encoded field anymore. Two
			// entries with different kinds and otherwise identical definitions
			// produce the same Frontmatter.
			def1 := schedule.TaskDefinition{
				Slug:          "kind-a",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceDaily,
			}
			def2 := schedule.TaskDefinition{
				Slug:          "kind-b",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceYearly,
			}
			Expect(pub.Publish(
				context.Background(),
				def1,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			fm1 := capture().Frontmatter
			Expect(pub.Publish(
				context.Background(),
				def2,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			fm2 := capture().Frontmatter
			Expect(fm1).To(Equal(fm2))
		})
	})

	Describe("sender interaction", func() {
		It("calls the sender exactly once per valid Publish", func() {
			def := schedule.TaskDefinition{
				Slug:          "weekly-review",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			Expect(sender.SendCommandCallCount()).To(Equal(1))
		})

		It("does not call the sender when slug is empty", func() {
			err := pub.Publish(
				context.Background(),
				schedule.TaskDefinition{Slug: ""},
				schedule.NewDate(2025, time.January, 4),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty slug"))
			Expect(sender.SendCommandCallCount()).To(Equal(0))
		})

		It("does not call the sender when date is zero", func() {
			err := pub.Publish(
				context.Background(),
				schedule.TaskDefinition{
					Slug:          "weekly-review",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
				},
				schedule.Date{},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("zero date"))
			Expect(sender.SendCommandCallCount()).To(Equal(0))
		})

		It("does not call SendCommand when dryRun is true", func() {
			dryPub := publisher.NewPublisher(
				sender,
				publisher.NewRenderer(),
				publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
				publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
				true,
			)
			def := schedule.TaskDefinition{
				Slug:          "weekly-review",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			Expect(dryPub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			Expect(sender.SendCommandCallCount()).To(Equal(0))
		})

		It("wraps a sender error with the slug and ISO date", func() {
			ctx := context.Background()
			sender.SendCommandReturns(errors.Errorf(ctx, "broker down"))
			err := pub.Publish(
				ctx,
				schedule.TaskDefinition{
					Slug:          "weekly-review",
					TitleTemplate: "t",
					Recurrence:    schedule.RecurrenceWeekly,
				},
				schedule.NewDate(2025, time.January, 4),
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("weekly-review"))
			Expect(err.Error()).To(ContainSubstring("2025-01-04"))
			Expect(sender.SendCommandCallCount()).To(Equal(1))
		})
	})

	Describe("boundary contract", func() {
		DescribeTable("produced command passes task.CreateCommand.Validate",
			func(kind schedule.RecurrenceKind) {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "Title for " + string(kind),
					BodyTemplate:  "Body for " + string(kind),
					Recurrence:    kind,
				}
				if kind == schedule.RecurrenceWeekday {
					def.Weekday = time.Saturday
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				captured := capture()
				Expect(captured.Validate(context.Background())).To(Succeed())
			},
			Entry("daily", schedule.RecurrenceDaily),
			Entry("weekly", schedule.RecurrenceWeekly),
			Entry("weekday", schedule.RecurrenceWeekday),
			Entry("monthly", schedule.RecurrenceMonthly),
			Entry("quarterly", schedule.RecurrenceQuarterly),
			Entry("yearly", schedule.RecurrenceYearly),
		)
	})

	Describe("UUID5 stability for the 21 migrated weekday entries", func() {
		// Spec 009 migrated 21 entries from RecurrenceWeekly+Weekday to
		// RecurrenceWeekday+Weekday. The period token shape for these
		// entries is byte-identical to pre-spec-9 (YYYYWww-<abbrev>), so
		// the UUID5 input string "recurring-<slug>-<period-token>" is
		// byte-identical, so the identifier is byte-identical, so the
		// vault filename is byte-identical — no duplicates after deploy.
		//
		// This test enumerates all 21 slugs with the hand-derived pre-spec-9
		// expected input strings and asserts equality. If any of the 21
		// expected strings is wrong, the test fails and the deploy is
		// blocked. If the publisher's switch or the inventory migration
		// diverges from the pre-spec-9 shape, the test fails and the
		// regression is caught at build time.
		type stabilityCase struct {
			slug          string
			recurrence    schedule.RecurrenceKind
			weekday       time.Weekday
			date          schedule.Date
			expectedInput string
		}
		cases := []stabilityCase{
			// 12 Saturday entries
			{
				slug:          "weekday-sat-1",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-1-2026W25-sat",
			},
			{
				slug:          "weekday-sat-2",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-2-2026W25-sat",
			},
			{
				slug:          "weekly-review",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekly-review-2026W25-sat",
			},
			{
				slug:          "weekday-sat-3",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-3-2026W25-sat",
			},
			{
				slug:          "weekday-sat-4",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-4-2026W25-sat",
			},
			{
				slug:          "weekday-sat-5",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-5-2026W25-sat",
			},
			{
				slug:          "weekday-sat-6",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-6-2026W25-sat",
			},
			{
				slug:          "weekday-sat-7",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-7-2026W25-sat",
			},
			{
				slug:          "weekday-sat-8",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-8-2026W25-sat",
			},
			{
				slug:          "plan-next-week",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-plan-next-week-2026W25-sat",
			},
			{
				slug:          "weekday-sat-9",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-9-2026W25-sat",
			},
			{
				slug:          "weekday-sat-10",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Saturday,
				date:          schedule.NewDate(2026, time.June, 20),
				expectedInput: "recurring-weekday-sat-10-2026W25-sat",
			},
			// 9 Sunday entries
			{
				slug:          "weekday-sun-1",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-1-2026W25-sun",
			},
			{
				slug:          "weekday-sun-2",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-2-2026W25-sun",
			},
			{
				slug:          "weekday-sun-3",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-3-2026W25-sun",
			},
			{
				slug:          "weekday-sun-4",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-4-2026W25-sun",
			},
			{
				slug:          "weekday-sun-5",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-5-2026W25-sun",
			},
			{
				slug:          "weekday-sun-6",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-6-2026W25-sun",
			},
			{
				slug:          "weekday-sun-7",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-7-2026W25-sun",
			},
			{
				slug:          "weekday-sun-8",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-8-2026W25-sun",
			},
			{
				slug:          "weekday-sun-9",
				recurrence:    schedule.RecurrenceWeekday,
				weekday:       time.Sunday,
				date:          schedule.NewDate(2026, time.June, 21),
				expectedInput: "recurring-weekday-sun-9-2026W25-sun",
			},
		}
		DescribeTable(
			"produces byte-identical UUID5 input string to pre-spec-9",
			func(c stabilityCase) {
				localSender := &taskmocks.TaskCreateCommandSender{}
				localSender.SendCommandReturns(nil)
				localPub := publisher.NewPublisher(
					localSender,
					publisher.NewRenderer(),
					publisher.NewFrontmatterFormatter(publisher.NewRenderer()),
					publisher.NewTaskIdentifierCreator(publisher.NewPeriodTokenBuilder()),
					false,
				)
				def := schedule.TaskDefinition{
					Slug:          c.slug,
					TitleTemplate: "t",
					Recurrence:    c.recurrence,
					Weekday:       c.weekday,
				}
				Expect(localPub.Publish(context.Background(), def, c.date)).To(Succeed())
				_, cmd := localSender.SendCommandArgsForCall(0)
				want := uuid.NewSHA1(publisher.UuidNamespaceForTest(), []byte(c.expectedInput)).
					String()
				Expect(string(cmd.TaskIdentifier)).To(Equal(want),
					"entry %q identifier changed; UUID5 input string must be byte-identical to pre-spec-9",
					c.slug)
			},
			Entry(fmt.Sprintf("%02d-%s", 0, cases[0].slug), cases[0]),
			Entry(fmt.Sprintf("%02d-%s", 1, cases[1].slug), cases[1]),
			Entry(fmt.Sprintf("%02d-%s", 2, cases[2].slug), cases[2]),
			Entry(fmt.Sprintf("%02d-%s", 3, cases[3].slug), cases[3]),
			Entry(fmt.Sprintf("%02d-%s", 4, cases[4].slug), cases[4]),
			Entry(fmt.Sprintf("%02d-%s", 5, cases[5].slug), cases[5]),
			Entry(fmt.Sprintf("%02d-%s", 6, cases[6].slug), cases[6]),
			Entry(fmt.Sprintf("%02d-%s", 7, cases[7].slug), cases[7]),
			Entry(fmt.Sprintf("%02d-%s", 8, cases[8].slug), cases[8]),
			Entry(fmt.Sprintf("%02d-%s", 9, cases[9].slug), cases[9]),
			Entry(fmt.Sprintf("%02d-%s", 10, cases[10].slug), cases[10]),
			Entry(fmt.Sprintf("%02d-%s", 11, cases[11].slug), cases[11]),
			Entry(fmt.Sprintf("%02d-%s", 12, cases[12].slug), cases[12]),
			Entry(fmt.Sprintf("%02d-%s", 13, cases[13].slug), cases[13]),
			Entry(fmt.Sprintf("%02d-%s", 14, cases[14].slug), cases[14]),
			Entry(fmt.Sprintf("%02d-%s", 15, cases[15].slug), cases[15]),
			Entry(fmt.Sprintf("%02d-%s", 16, cases[16].slug), cases[16]),
			Entry(fmt.Sprintf("%02d-%s", 17, cases[17].slug), cases[17]),
			Entry(fmt.Sprintf("%02d-%s", 18, cases[18].slug), cases[18]),
			Entry(fmt.Sprintf("%02d-%s", 19, cases[19].slug), cases[19]),
			Entry(fmt.Sprintf("%02d-%s", 20, cases[20].slug), cases[20]),
		)
	})
})
