// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"
	"time"

	lib "github.com/bborbe/agent/lib"
	"github.com/bborbe/agent/lib/command/task"
	taskmocks "github.com/bborbe/agent/lib/command/task/mocks"
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
		pub = publisher.NewPublisher(sender, false)
	})

	capture := func() task.CreateCommand {
		_, cmd := sender.SendCommandArgsForCall(0)
		return cmd
	}

	Describe("identifier", func() {
		It("is the UUID5 of the canonical key", func() {
			def := schedule.TaskDefinition{
				Slug:          "weekly-review",
				TitleTemplate: "Weekly Review {{iso-week}}",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.January, 4),
			)).To(Succeed())
			captured := capture()
			expected := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte("recurring-weekly-review-2025W01"),
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
			localPub := publisher.NewPublisher(localSender, false)
			def := schedule.TaskDefinition{
				Slug:          slug,
				TitleTemplate: "t",
				Recurrence:    rec,
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
				TitleTemplate: "Title {{date}}",
				BodyTemplate:  "Body {{date}}",
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
			Entry("{{date}}", renderCase{
				name:     "{{date}}",
				template: "prefix {{date}} suffix",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "prefix 2025-01-04 suffix",
			}),
			Entry("{{iso-week}}", renderCase{
				name:     "{{iso-week}}",
				template: "Week {{iso-week}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Week 2025W01",
			}),
			Entry("{{next-iso-week}}", renderCase{
				name:     "{{next-iso-week}}",
				template: "Next {{next-iso-week}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Next 2025W02",
			}),
			Entry("{{month}}", renderCase{
				name:     "{{month}}",
				template: "Month {{month}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Month 2025-01",
			}),
			Entry("{{last-month}} with year roll-back", renderCase{
				name:     "{{last-month}}",
				template: "Last {{last-month}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Last 2024-12",
			}),
			Entry("{{quarter}}", renderCase{
				name:     "{{quarter}}",
				template: "Q {{quarter}}",
				date:     schedule.NewDate(2025, time.April, 1),
				want:     "Q 2025Q2",
			}),
			Entry("{{last-quarter}} with year roll-back", renderCase{
				name:     "{{last-quarter}}",
				template: "Last {{last-quarter}}",
				date:     schedule.NewDate(2025, time.January, 1),
				want:     "Last 2024Q4",
			}),
			Entry("{{year}}", renderCase{
				name:     "{{year}}",
				template: "Year {{year}}",
				date:     schedule.NewDate(2025, time.April, 1),
				want:     "Year 2025",
			}),
			Entry("{{last-year}}", renderCase{
				name:     "{{last-year}}",
				template: "Last {{last-year}}",
				date:     schedule.NewDate(2025, time.January, 1),
				want:     "Last 2024",
			}),
		)

		It("renders placeholders in BodyTemplate", func() {
			def := schedule.TaskDefinition{
				Slug:          "test-slug",
				TitleTemplate: "t",
				BodyTemplate:  "body contains {{date}}",
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
				TitleTemplate: "{{iso-week}}",
				Recurrence:    schedule.RecurrenceWeekly,
			}
			// 2024-12-30 (Monday) belongs to ISO 2025W01.
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2024, time.December, 30),
			)).To(Succeed())
			Expect(capture().Title).To(Equal("2025W01"))
		})
	})

	Describe("frontmatter", func() {
		It("has the full frozen shape", func() {
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
			Expect(fm).To(HaveKeyWithValue("assignee", "bborbe"))
			Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
			Expect(fm).To(HaveKeyWithValue("page_type", "task"))
			Expect(fm).To(HaveKeyWithValue("priority", 2))
			Expect(fm).To(HaveKeyWithValue("recurring", "weekly"))
			Expect(fm).To(HaveKeyWithValue(
				"goals",
				[]interface{}{"[[Migrate Personal Workflow from Atlassian to Obsidian]]"},
			))
			Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
			Expect(fm).To(HaveLen(7))
		})

		DescribeTable("recurring matches RecurrenceKind",
			func(kind schedule.RecurrenceKind) {
				def := schedule.TaskDefinition{
					Slug:          "test-slug",
					TitleTemplate: "t",
					Recurrence:    kind,
				}
				Expect(pub.Publish(
					context.Background(),
					def,
					schedule.NewDate(2025, time.January, 4),
				)).To(Succeed())
				Expect(capture().Frontmatter).To(HaveKeyWithValue("recurring", string(kind)))
			},
			Entry("daily", schedule.RecurrenceDaily),
			Entry("weekly", schedule.RecurrenceWeekly),
			Entry("monthly", schedule.RecurrenceMonthly),
			Entry("quarterly", schedule.RecurrenceQuarterly),
			Entry("yearly", schedule.RecurrenceYearly),
		)
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
			dryPub := publisher.NewPublisher(sender, true)
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
			Entry("monthly", schedule.RecurrenceMonthly),
			Entry("quarterly", schedule.RecurrenceQuarterly),
			Entry("yearly", schedule.RecurrenceYearly),
		)
	})
})
