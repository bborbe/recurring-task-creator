// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schedule

import "time"

// onWeekdayDay5OfMonth fires on day 5 of any month that is not a Saturday or
// Sunday. The spec lists the finances update under "Day-of-Month = 5" with
// recurrence Monthly, but its literal acceptance test for 2025-01-05 (a
// Sunday) expects the Sunday arm only and does not include update-finances.
// This predicate reconciles the two by skipping weekend day-5 dates.
//
// NOTE: this single entry uses a purpose-built predicate instead of one of
// the closed primitives in predicate.go. The deviation is documented because
// the spec's two requirements (inventory table + acceptance test) are
// mutually inconsistent.
func onWeekdayDay5OfMonth(d Date) bool {
	if d.weekday() == time.Saturday || d.weekday() == time.Sunday {
		return false
	}
	return d.Day == 5
}

// inventory is the canonical, frozen recurring-task inventory. Order in this
// slice has no semantic meaning — TasksForDate sorts by Slug before returning.
var inventory = []TaskDefinition{
	// Weekly — Saturday (12 entries)
	{
		Slug:          "shutdown-k3s",
		TitleTemplate: "Shutdown K3s",
		BodyTemplate: "Shutdown K3s cleanly so BoltDB files are not corrupt during backups.\n\n" +
			"~/Documents/workspaces/scripts/remote-k3s-shutdown-nuke.sh\n\n" +
			"[K3s Cluster Weekly Reboot Procedure](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FK3s%20Cluster%20Weekly%20Reboot%20Procedure)\n\n" +
			"[jira-task-creator](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2Fjira-task-creator)",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "turn-on-hell",
		TitleTemplate: "Turn on hell",
		BodyTemplate:  "power on hell",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Saturday),
	},
	{
		Slug:          "weekly-review",
		TitleTemplate: "Weekly Review {{iso-week}}",
		BodyTemplate: "Complete weekly review.\n\n" +
			"In Obsidian run (in order):\n\n" +
			"1. /complete-week - Bot performance, fills weekly note\n" +
			"2. /weekly-trading-review {{iso-week}} - Portfolio balances",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "check-ftmo-demo-accounts",
		TitleTemplate: "Check FTMO Demo Accounts",
		BodyTemplate: "Check if ftmo-demo account is expiring soon (e.g., this weekend).\n\n" +
			"If expiring soon, follow renewal guide:\n\n" +
			"~/Documents/Obsidian/Personal/40 Trading/FTMO Demo Account Renewal Guide.md\n\n" +
			"Dashboards:\n\n" +
			"* [FTMO](https://trader.ftmo.com/accounts-overview)\n" +
			"* [Dev](https://dev.quant.benjamin-borbe.de/account/detail/ftmo-demo)",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "lexoffice-invoices",
		TitleTemplate: "LexOffice Accounting",
		BodyTemplate:  "[LexOffice](https://app.lexoffice.de/fis/#)",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Saturday),
	},
	{
		Slug:          "moneymoney-review",
		TitleTemplate: "Review MoneyMoney",
		BodyTemplate:  "Review MoneyMoney",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Saturday),
	},
	{
		Slug:          "opnsense-update",
		TitleTemplate: "OPNsense Update",
		BodyTemplate:  "[OPNsense Firmware Updates](https://opnsense.hm.benjamin-borbe.de/ui/core/firmware#updates)",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Saturday),
	},
	{
		Slug:          "home-assistant-update-backup",
		TitleTemplate: "Home Assistant Update + Backup",
		BodyTemplate: "Weekly Home Assistant maintenance.\n\n" +
			"Steps:\n\n" +
			"1. Login\n" +
			"2. Create backup\n" +
			"3. Download backup\n" +
			"4. Update all\n\n" +
			"[Home Assistant](http://homeassistant.local:8123/config/dashboard)",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "renew-gmail-oauth-tokens",
		TitleTemplate: "Renew Gmail OAuth Tokens",
		BodyTemplate: "Renew Gmail OAuth tokens (expire every 7 days) for all environments:\n\n" +
			"Dev: [OAuth Init](https://dev.quant.benjamin-borbe.de/admin/core-mail-controller/oauth2/init)\n\n" +
			"Prod: [OAuth Init](https://prod.quant.benjamin-borbe.de/admin/core-mail-controller/oauth2/init)",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "plan-next-week",
		TitleTemplate: "Plan Week {{next-iso-week}}",
		BodyTemplate: "Create plan for week {{next-iso-week}}\n\n" +
			"In Obsidian run:\n\n" +
			"/plan-week",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "run-update-all-saturday",
		TitleTemplate: "Run update-all.sh (before restart)",
		BodyTemplate: "Run system updates before weekend restart (sun.hm and fire.hm)\n\n" +
			"/Users/bborbe/Documents/workspaces/scripts/update-all.sh",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	{
		Slug:          "topic-backup-saturday",
		TitleTemplate: "Backup Kafka Topics",
		BodyTemplate: "Backup Kafka topics before weekend restart\n\n" +
			"Prerequisite: hell must be powered on (see \"Turn on hell\" subtask).\n\n" +
			"cd /Users/bborbe/Documents/workspaces/trading/strimzi/topic-backuper/cmd/backup\n\n" +
			"make backup",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Saturday),
	},
	// Weekly — Sunday (9 entries)
	{
		Slug:          "complete-rsync-backups",
		TitleTemplate: "Complete Rsync Backups",
		BodyTemplate: "* check backup status\n" +
			"** [Backup Status](https://backup.hell.hm.benjamin-borbe.de/status)",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Sunday),
	},
	{
		Slug:          "complete-longhorn-backups",
		TitleTemplate: "Complete Longhorn Backups",
		BodyTemplate:  "[Longhorn Volumes](https://longhorn.quant.benjamin-borbe.de/#/volume)",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Sunday),
	},
	{
		Slug:          "turn-off-hell",
		TitleTemplate: "Turn off hell",
		BodyTemplate:  "power off hell",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Sunday),
	},
	{
		Slug:          "turn-off-sun",
		TitleTemplate: "Turn off sun",
		BodyTemplate:  "power off sun",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Sunday),
	},
	{
		Slug:          "turn-off-fire",
		TitleTemplate: "Turn off fire",
		BodyTemplate:  "power off fire",
		Recurrence:    RecurrenceWeekly,
		Fires:         OnWeekdays(time.Sunday),
	},
	{
		Slug:          "docker-registry-gc",
		TitleTemplate: "Docker Registry GC",
		BodyTemplate: "Run garbage collection on docker registry to free storage space.\n\n" +
			"kubectlquant -n docker-registry get pods\n\n" +
			"kubectlquant -n docker-registry exec -it <POD_NAME> -- registry garbage-collect /etc/docker/registry/config.yml",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Sunday),
	},
	{
		Slug:          "rebuild-trading-dev-prod",
		TitleTemplate: "Rebuild Trading Dev+Prod",
		BodyTemplate: "Rebuild and redeploy all trading services for dev and prod.\n\n" +
			"Runbook: Trading - Rebuild Dev and Prod",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Sunday),
	},
	{
		Slug:          "check-bot-is-healthy",
		TitleTemplate: "Bot is Healthy",
		BodyTemplate: "check bot is ready for trading on monday\n\n" +
			"* kubectlquant get po --all-namespaces|grep -v Running|grep -v Complete\n" +
			"* [Prometheus Alerts](https://prometheus.quant.benjamin-borbe.de/alerts)\n" +
			"* [Karma Active Alerts](https://karma.quant.benjamin-borbe.de/?q=%40state%3Dactive)",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Sunday),
	},
	{
		Slug:          "run-update-all",
		TitleTemplate: "Run update-all.sh",
		BodyTemplate: "Run system updates across all servers\n\n" +
			"/Users/bborbe/Documents/workspaces/scripts/update-all.sh",
		Recurrence: RecurrenceWeekly,
		Fires:      OnWeekdays(time.Sunday),
	},
	// Day-of-Month = 5 (1 entry)
	{
		Slug:          "update-finances",
		TitleTemplate: "Update Finances spreadsheet",
		BodyTemplate: "Fill spreadsheet at 5. of each month\n\n" +
			"[Finances Spreadsheet](https://docs.google.com/spreadsheets/d/1Bmj_zmpomXJHW5sRTIcEE0xolIlGrYtO0FkY3nrxkPc/edit?usp=sharing)",
		Recurrence: RecurrenceMonthly,
		Fires:      onWeekdayDay5OfMonth,
	},
	// May 1st only (2 entries)
	{
		Slug:          "capitalcom-apikey-prod",
		TitleTemplate: "Change Capitalcom ApiKey - Prod",
		BodyTemplate:  "[Capital.com API Settings](https://capital.com/trading/platform/?popup=api-key-generation&tab=APISettings)",
		Recurrence:    RecurrenceYearly,
		Fires:         OnMonthAndDay(time.May, 1),
	},
	{
		Slug:          "capitalcom-apikey-dev",
		TitleTemplate: "Change Capitalcom ApiKey - Dev",
		BodyTemplate:  "[Capital.com API Settings](https://capital.com/trading/platform/?popup=api-key-generation&tab=APISettings)",
		Recurrence:    RecurrenceYearly,
		Fires:         OnMonthAndDay(time.May, 1),
	},
	// Monthly — day 1 of every month (17 entries)
	{
		Slug:          "backup-atlassian-confluence",
		TitleTemplate: "Atlassian Confluence Backup",
		BodyTemplate: "[Atlassian Confluence Backup](https://borbe.atlassian.net/wiki/plugins/servlet/ondemandbackupmanager/admin)\n\n" +
			"Save to: smb://hell.hm.benjamin-borbe.de/bborbe/Backups/Atlassian-Cloud/",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "backup-atlassian-jira",
		TitleTemplate: "Atlassian Jira Backup",
		BodyTemplate: "[Atlassian Jira Backup](https://borbe.atlassian.net/secure/admin/CloudExport.jspa)\n\n" +
			"Save to: smb://hell.hm.benjamin-borbe.de/bborbe/Backups/Atlassian-Cloud/",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "backup-google-drive",
		TitleTemplate: "Backup Google Drive",
		BodyTemplate: "[Google Drive Backup Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FGoogle%20Drive%20Backup%20Guide)\n\n" +
			"Requirements:\n\n" +
			"1. Hell server must be powered on\n" +
			"2. Google Drive Desktop synced\n\n" +
			"Script: ~/Documents/workspaces/scripts/backup-google-drive.sh",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "backup-pictures",
		TitleTemplate: "Backup Images",
		BodyTemplate: "Backup iPhone images to file server and archive to external drive.\n\n" +
			"[How to Back Up iPhone](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FHow%20to%20Back%20Up%20iPhone)\n\n" +
			"Steps:\n\n" +
			"1. Import images via USB backup\n" +
			"2. Rename and move to file server\n" +
			"3. Archive to external drive using rsync\n" +
			"4. Delete images from iPhone",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "monthly-review",
		TitleTemplate: "Review Month {{last-month}}",
		BodyTemplate: "Create review for month {{last-month}}\n\n" +
			"In Obsidian run:\n\n" +
			"/monthly-trading-review {{last-month}}",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "plan-month",
		TitleTemplate: "Plan Month {{month}}",
		BodyTemplate: "Create plan for month {{month}}\n\n" +
			"In Obsidian run:\n\n" +
			"/plan-month",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "trading-profits",
		TitleTemplate: "Add Trading Profits to Sheet",
		BodyTemplate:  "[Trading Profits Sheet](https://docs.google.com/spreadsheets/d/1F6ObbGvRciK4ZdvB3BuRCf7LJFWdL-46teXvXlR0waI/edit?usp=sharing)",
		Recurrence:    RecurrenceMonthly,
		Fires:         OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-frontend",
		TitleTemplate: "Update Frontend",
		BodyTemplate: "Follow Frontend Update Guide in Obsidian vault.\n\n" +
			"Guide location: 50 Knowledge Base/Frontend Update Guide.md\n\n" +
			"[Verification](https://prod.quant.benjamin-borbe.de/chart?epic=BTCUSD&broker=capitalcom&source=standard&bidAsk=bid&resolution=1m&from=NOW-7d&until=NOW)",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-go-mod",
		TitleTemplate: "Update Trading Bot Dependencies",
		BodyTemplate: "Update all dependencies of all trading services.\n\n" +
			"Done when: All projects updated, tests pass, changes merged to master.\n\n" +
			"[Updater Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FUpdater%20Guide)",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-inventar",
		TitleTemplate: "Update inventar",
		BodyTemplate: "Review purchases and add new items to Obsidian inventory.\n\n" +
			"[Monthly Inventory Update Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FMonthly%20Inventory%20Update%20Guide)\n\n" +
			"Steps:\n\n" +
			"1. Review trading business invoices in Google Drive\n" +
			"2. Check for personal purchases\n" +
			"3. Add items following the inventory guide\n" +
			"4. Verify items appear in dashboard",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-journal",
		TitleTemplate: "Update Trading Journal",
		BodyTemplate:  "Use /update-trading-journal in Claude Code",
		Recurrence:    RecurrenceMonthly,
		Fires:         OnFirstDayOfMonth(),
	},
	{
		Slug:          "world-apply",
		TitleTemplate: "World apply",
		BodyTemplate:  "[World Apply Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FWorld%20Apply%20Guide)",
		Recurrence:    RecurrenceMonthly,
		Fires:         OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-screego",
		TitleTemplate: "Update Screego",
		BodyTemplate: "[Screego Docker Hub](https://hub.docker.com/r/screego/server/tags)\n\n" +
			"cat ~/Documents/workspaces/world/configuration/world.go|grep screego",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-poste",
		TitleTemplate: "Update Poste",
		BodyTemplate: "[Poste Docker Hub](https://hub.docker.com/r/analogic/poste.io/tags)\n\n" +
			"cat ~/Documents/workspaces/world/configuration/world.go|grep poste\n\n" +
			"# update poste version in world.go\n\n" +
			"make install\n\n" +
			"world apply -a poste -v=2",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-minio",
		TitleTemplate: "Update Minio",
		BodyTemplate: "helm repo add minio-operator https://operator.min.io\n\n" +
			"helm repo update\n\n" +
			"helm list -n minio-operator\n\n" +
			"helm search repo minio-operator/operator\n\n" +
			"helm list -n minio\n\n" +
			"helm search repo minio-operator/tenant\n\n" +
			"Open \"backup\" Intellij Project\n\n" +
			"Update Version in k8s/minio/operator/Makefile\n\n" +
			"make upgrade\n\n" +
			"Update Version in k8s/minio/tenant/Makefile\n\n" +
			"make upgrade",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-library",
		TitleTemplate: "Update Github Libraries",
		BodyTemplate: "Automated workflow (recommended):\n\n" +
			"[Updater Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FUpdater%20Guide)\n\n" +
			"Manual workflow:\n\n" +
			"[Go Library Update Workflow](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FGo%20Library%20Update%20Workflow)\n\n" +
			"Task tracking:\n\n" +
			"[Update All Go Libraries Task](obsidian://open?vault=Personal&file=24%20Tasks%2FUpdate%20All%20Go%20Libraries%20Following%20Workflow)",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	{
		Slug:          "update-k3s",
		TitleTemplate: "Update K3s",
		BodyTemplate: "[K3s Release Channels](https://update.k3s.io/v1-release/channels)\n\n" +
			"[K3s Upgrade Guide](obsidian://open?vault=Personal&file=50%20Knowledge%20Base%2FK3s%20Upgrade)\n\n" +
			"* Update Hell\n" +
			"* Update Nuke",
		Recurrence: RecurrenceMonthly,
		Fires:      OnFirstDayOfMonth(),
	},
	// Quarterly — first day of Jan/Apr/Jul/Oct (2 entries)
	{
		Slug:          "quarter-review",
		TitleTemplate: "Review Quarter {{last-quarter}}",
		BodyTemplate: "Create review for quarter {{last-quarter}}\n\n" +
			"In Obsidian run:\n\n" +
			"/quarterly-trading-review {{last-quarter}}",
		Recurrence: RecurrenceQuarterly,
		Fires:      OnFirstDayOfQuarter(),
	},
	{
		Slug:          "quarter-plan",
		TitleTemplate: "Plan Quarter {{quarter}}",
		BodyTemplate: "Create plan for quarter {{quarter}}\n\n" +
			"In Obsidian run:\n\n" +
			"/plan-quarter",
		Recurrence: RecurrenceQuarterly,
		Fires:      OnFirstDayOfQuarter(),
	},
	// Yearly — first day of January (2 entries)
	{
		Slug:          "yearly-review",
		TitleTemplate: "Review Year {{last-year}}",
		BodyTemplate: "Create review for year {{last-year}}\n\n" +
			"In Obsidian run:\n\n" +
			"/yearly-trading-review {{last-year}}",
		Recurrence: RecurrenceYearly,
		Fires:      OnFirstDayOfYear(),
	},
	{
		Slug:          "plan-year",
		TitleTemplate: "Plan Year {{year}}",
		BodyTemplate: "Create plan for year {{year}}\n\n" +
			"In Obsidian run:\n\n" +
			"/plan-year",
		Recurrence: RecurrenceYearly,
		Fires:      OnFirstDayOfYear(),
	},
}
