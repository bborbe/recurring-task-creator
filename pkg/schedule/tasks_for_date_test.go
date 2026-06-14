// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

func slugs(defs []schedule.TaskDefinition) []string {
	out := make([]string, len(defs))
	for i, def := range defs {
		out[i] = def.Slug
	}
	return out
}

var _ = Describe("TasksForDate", func() {
	It("returns the Saturday arm for 2025-01-04", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 4))
		Expect(slugs(defs)).To(ConsistOf(
			"shutdown-k3s", "turn-on-hell", "weekly-review",
			"check-ftmo-demo-accounts", "lexoffice-invoices",
			"moneymoney-review", "opnsense-update",
			"home-assistant-update-backup", "renew-gmail-oauth-tokens",
			"plan-next-week", "run-update-all-saturday", "topic-backup-saturday",
		))
	})

	It("returns the Sunday arm for 2025-01-05", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 5))
		Expect(slugs(defs)).To(ConsistOf(
			"complete-rsync-backups", "complete-longhorn-backups",
			"turn-off-hell", "turn-off-sun", "turn-off-fire",
			"docker-registry-gc", "rebuild-trading-dev-prod",
			"check-bot-is-healthy", "run-update-all",
		))
	})

	It("returns only update-finances for 2025-03-05 (Wednesday, DOM=5)", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.March, 5))
		Expect(slugs(defs)).To(ConsistOf("update-finances"))
	})

	It("returns monthly union + capitalcom pair for 2025-05-01 (19 slugs)", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.May, 1))
		Expect(slugs(defs)).To(ConsistOf(
			"backup-atlassian-confluence", "backup-atlassian-jira",
			"backup-google-drive", "backup-pictures",
			"monthly-review", "plan-month", "trading-profits",
			"update-frontend", "update-go-mod", "update-inventar",
			"update-journal", "world-apply", "update-screego",
			"update-poste", "update-minio", "update-library", "update-k3s",
			"capitalcom-apikey-prod", "capitalcom-apikey-dev",
		))
		Expect(defs).To(HaveLen(19))
	})

	It("returns monthly union + quarterly for 2025-04-01", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.April, 1))
		Expect(slugs(defs)).To(ConsistOf(
			"backup-atlassian-confluence", "backup-atlassian-jira",
			"backup-google-drive", "backup-pictures",
			"monthly-review", "plan-month", "trading-profits",
			"update-frontend", "update-go-mod", "update-inventar",
			"update-journal", "world-apply", "update-screego",
			"update-poste", "update-minio", "update-library", "update-k3s",
			"quarter-review", "quarter-plan",
		))
	})

	It("returns monthly + quarterly + yearly for 2025-01-01", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 1))
		Expect(slugs(defs)).To(ConsistOf(
			"backup-atlassian-confluence", "backup-atlassian-jira",
			"backup-google-drive", "backup-pictures",
			"monthly-review", "plan-month", "trading-profits",
			"update-frontend", "update-go-mod", "update-inventar",
			"update-journal", "world-apply", "update-screego",
			"update-poste", "update-minio", "update-library", "update-k3s",
			"quarter-review", "quarter-plan",
			"yearly-review", "plan-year",
		))
	})

	It("returns slugs in ascending sorted order on every call", func() {
		defs := schedule.TasksForDate(schedule.NewDate(2025, time.January, 1))
		got := slugs(defs)
		sorted := append([]string(nil), got...)
		sort.Strings(sorted)
		Expect(got).To(Equal(sorted))
	})

	It("is referentially transparent (deep equality on repeated calls)", func() {
		d := schedule.NewDate(2025, time.April, 1)
		a := schedule.TasksForDate(d)
		b := schedule.TasksForDate(d)
		Expect(a).To(Equal(b))
	})

	It("returns an empty slice for the zero Date (no panic)", func() {
		defs := schedule.TasksForDate(schedule.Date{})
		Expect(defs).To(BeEmpty())
		Expect(defs).NotTo(BeNil())
	})
})
