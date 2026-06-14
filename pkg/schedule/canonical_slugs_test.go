// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule_test

import (
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/schedule"
)

var canonicalSlugs = []string{
	"backup-atlassian-confluence",
	"backup-atlassian-jira",
	"backup-google-drive",
	"backup-pictures",
	"capitalcom-apikey-dev",
	"capitalcom-apikey-prod",
	"check-bot-is-healthy",
	"check-ftmo-demo-accounts",
	"complete-longhorn-backups",
	"complete-rsync-backups",
	"docker-registry-gc",
	"home-assistant-update-backup",
	"lexoffice-invoices",
	"moneymoney-review",
	"monthly-review",
	"opnsense-update",
	"plan-month",
	"plan-next-week",
	"plan-year",
	"quarter-plan",
	"quarter-review",
	"rebuild-trading-dev-prod",
	"renew-gmail-oauth-tokens",
	"run-update-all",
	"run-update-all-saturday",
	"shutdown-k3s",
	"topic-backup-saturday",
	"trading-profits",
	"turn-off-fire",
	"turn-off-hell",
	"turn-off-sun",
	"turn-on-hell",
	"update-finances",
	"update-frontend",
	"update-go-mod",
	"update-inventar",
	"update-journal",
	"update-k3s",
	"update-library",
	"update-minio",
	"update-poste",
	"update-screego",
	"weekly-review",
	"world-apply",
	"yearly-review",
}

var _ = Describe("inventory canonical slug list", func() {
	It("matches the frozen sorted list of all slugs", func() {
		all := schedule.AllDefinitionsForTest()
		got := make([]string, len(all))
		for i, def := range all {
			got[i] = def.Slug
		}
		sort.Strings(got)
		Expect(got).To(Equal(canonicalSlugs))
		Expect(got).To(HaveLen(45))
	})
})
