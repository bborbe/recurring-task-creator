// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"
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
			localPub := publisher.NewPublisher(localSender, false)
			def := schedule.TaskDefinition{
				Slug:          slug,
				TitleTemplate: "t",
				Recurrence:    rec,
				Weekday:       time.Saturday,
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

		It("weekly: byte-equality with the formatter output (with weekday suffix)", func() {
			// 2025-06-09 (Mon) is in ISO 2025W24; with Weekday=time.Saturday
			// the period token must be "2025W24-sat".
			def := schedule.TaskDefinition{
				Slug:          "byte-eq-weekly",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekly,
				Weekday:       time.Saturday,
			}
			Expect(pub.Publish(
				context.Background(),
				def,
				schedule.NewDate(2025, time.June, 9),
			)).To(Succeed())
			cmd := capture()
			expected := "recurring-byte-eq-weekly-2025W24-sat"
			want := uuid.NewSHA1(
				publisher.UuidNamespaceForTest(),
				[]byte(expected),
			).String()
			Expect(string(cmd.TaskIdentifier)).To(Equal(want))
		})
	})

	It(
		"buildPeriodToken: weekly token carries the entry's Weekday, not the date's weekday",
		func() {
			// 2026-06-17 is a Wednesday, in ISO 2026W25. With Weekday=time.Saturday
			// on the def, the period token must be "2026W25-sat" (NOT "2026W25-wed").
			def := schedule.TaskDefinition{
				Slug:          "weekday-takes-precedence",
				TitleTemplate: "t",
				Recurrence:    schedule.RecurrenceWeekly,
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
			localPub := publisher.NewPublisher(localSender, false)
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
				want:     "prefix 2025-01-04 suffix - 2025W01-sun",
			}),
			Entry("{{iso-week}}", renderCase{
				name:     "{{iso-week}}",
				template: "Week {{iso-week}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Week 2025W01 - 2025W01-sun",
			}),
			Entry("{{next-iso-week}}", renderCase{
				name:     "{{next-iso-week}}",
				template: "Next {{next-iso-week}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Next 2025W02 - 2025W01-sun",
			}),
			Entry("{{month}}", renderCase{
				name:     "{{month}}",
				template: "Month {{month}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Month 2025-01 - 2025W01-sun",
			}),
			Entry("{{last-month}} with year roll-back", renderCase{
				name:     "{{last-month}}",
				template: "Last {{last-month}}",
				date:     schedule.NewDate(2025, time.January, 4),
				want:     "Last 2024-12 - 2025W01-sun",
			}),
			Entry("{{quarter}}", renderCase{
				name:     "{{quarter}}",
				template: "Q {{quarter}}",
				date:     schedule.NewDate(2025, time.April, 1),
				want:     "Q 2025Q2 - 2025W14-sun",
			}),
			Entry("{{last-quarter}} with year roll-back", renderCase{
				name:     "{{last-quarter}}",
				template: "Last {{last-quarter}}",
				date:     schedule.NewDate(2025, time.January, 1),
				want:     "Last 2024Q4 - 2025W01-sun",
			}),
			Entry("{{year}}", renderCase{
				name:     "{{year}}",
				template: "Year {{year}}",
				date:     schedule.NewDate(2025, time.April, 1),
				want:     "Year 2025 - 2025W14-sun",
			}),
			Entry("{{last-year}}", renderCase{
				name:     "{{last-year}}",
				template: "Last {{last-year}}",
				date:     schedule.NewDate(2025, time.January, 1),
				want:     "Last 2024 - 2025W01-sun",
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
			Expect(capture().Title).To(Equal("2025W01 - 2025W01-sun"))
		})
	})

	Describe("title suffix", func() {
		It("appends the period token to a monthly title", func() {
			def := schedule.TaskDefinition{
				Slug:          "update-k3s",
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

		It("appends the period token to a weekly title (with weekday suffix)", func() {
			def := schedule.TaskDefinition{
				Slug:          "shutdown-k3s",
				TitleTemplate: "Shutdown K3s",
				Recurrence:    schedule.RecurrenceWeekly,
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
					TitleTemplate: "{{month}}",
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
				localPub := publisher.NewPublisher(localSender, false)
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
	})

	Describe("full-inventory render", func() {
		It("every inventory entry renders to a title ending in ' - <period-token>'", func() {
			// The full-inventory cross-check: prove that Prompt 1's publisher
			// suffix and Prompt 2's inventory cleanup are mutually consistent.
			// For each entry in schedule.Inventory() and the fixed reference
			// date 2026-06-15, the rendered Title must end with " - " followed
			// by the period token buildPeriodToken returns for the same input.
			refDate := schedule.NewDate(2026, time.June, 15)
			for _, def := range schedule.Inventory() {
				// Use a fresh sender per entry so SendCommandArgsForCall(0)
				// always points at the most recent Publish.
				localSender := &taskmocks.TaskCreateCommandSender{}
				localSender.SendCommandReturns(nil)
				localPub := publisher.NewPublisher(localSender, false)
				Expect(localPub.Publish(context.Background(), def, refDate)).To(Succeed())
				_, cmd := localSender.SendCommandArgsForCall(0)
				expectedToken, err := publisher.BuildPeriodTokenForTest(
					context.Background(),
					def.Recurrence,
					refDate,
					def.Weekday,
				)
				Expect(err).NotTo(HaveOccurred(), def.Slug)
				expectedSuffix := " - " + expectedToken
				Expect(cmd.Title).To(HaveSuffix(expectedSuffix),
					"entry %q rendered title %q does not end with %q",
					def.Slug, cmd.Title, expectedSuffix)
			}
		})
	})

	Describe("frontmatter", func() {
		It(
			"has the six-key shape (assignee, status, page_type, goals, priority, created_by)",
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
				Expect(fm).To(HaveKeyWithValue("assignee", "bborbe"))
				Expect(fm).To(HaveKeyWithValue("status", "in_progress"))
				Expect(fm).To(HaveKeyWithValue("page_type", "task"))
				Expect(fm).To(HaveKeyWithValue("priority", 2))
				Expect(fm).To(HaveKeyWithValue(
					"goals",
					[]interface{}{"[[Migrate Personal Workflow from Atlassian to Obsidian]]"},
				))
				Expect(fm).To(HaveKeyWithValue("created_by", "recurring-task-creator"))
				Expect(fm).To(HaveLen(6))
				// AC #4 explicit absence: the `recurring` key was removed by spec 008.
				Expect(fm).NotTo(HaveKey("recurring"))
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
