# Changelog

## Unreleased

- feat: Add `pkg/schedule` package with 45-entry recurring-task inventory and `TasksForDate` pure function

## v0.0.2

- Remove flaky `gexec.Build` main test (skeleton inheritance; precommit covers compile check)
- Drop unused `k8s/recurring-task-creator-deploy.yaml` (StatefulSet is the deployment kind for this service)

## v0.0.1

- Initial commit
