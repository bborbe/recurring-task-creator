# Changelog

## Unreleased

- feat: Add `pkg/tick` package that runs the hourly cron loop (initial tick at boot, then 1-hour ticker) calling `publisher.Publish` for every `schedule.TasksForDate` entry; per-task error isolation via glog + Prometheus counter; gauge for last-tick timestamp; `Europe/Berlin` civil date from injected clock
- feat: Add `pkg/publisher` package that builds a deterministic `task.CreateCommand` from `(schedule.TaskDefinition, schedule.Date)` and sends it via an injected `task.CreateCommandSender`; identifier is UUID5 of `"recurring-<slug>-<YYYY-MM-DD>"`; frontmatter is frozen at `assignee/status/page_type/priority/goals/recurring`
- feat: Add `pkg/schedule` package with 45-entry recurring-task inventory and `TasksForDate` pure function

## v0.0.2

- Remove flaky `gexec.Build` main test (skeleton inheritance; precommit covers compile check)
- Drop unused `k8s/recurring-task-creator-deploy.yaml` (StatefulSet is the deployment kind for this service)

## v0.0.1

- Initial commit
