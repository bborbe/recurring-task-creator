# recurring-task-creator

Publishes `task.CreateCommand` events to Kafka on a schedule so the agent task-controller materializes recurring tasks (daily/weekly/monthly/quarterly/yearly) as Obsidian vault files. Replaces `jira-task-creator`.

## Run locally

```bash
make test
make run
```

## Deploy

```bash
make buca
```
