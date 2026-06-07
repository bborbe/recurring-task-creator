# Definition of Done

A change is done only when all of the following hold.

## Code Quality

- All exported types, functions, and methods have GoDoc comments.
- Errors are wrapped with `github.com/bborbe/errors`; no bare `return err` for non-trivial paths; no `fmt.Errorf` for error wrapping.
- No `time.Time` / `time.Now()` in business logic — use `github.com/bborbe/time` types and `CurrentDateTimeGetter` injection.
- No `context.Background()` in business logic — context flows from request/main.
- No direct `os.Getenv` outside main / startup config struct.
- Factory functions (`New*` / `Create*`) return interfaces, not structs, when there is more than one implementation or a mock is needed.

## Testing

- `make test` passes from the project root.
- Ginkgo v2 / Gomega style. New behavior has new specs.
- Counterfeiter mocks for collaborators with interfaces.
- No real network, real Kafka, or real K8s touched from unit tests.

## Build & Tooling

- `make precommit` passes from the project root (lint, test, security scan, license headers).
- `go.mod` has no stray `replace` or `exclude` directives unless explicitly required.
- No `LICENSE` file; no `claude*.yml` workflow files (this repo is private, solo, intentionally lean).

## Documentation

- `CHANGELOG.md` has an entry describing the user-visible change (or "internal: …" for refactors).
- `README.md` updated if a runnable command, env var, or deployment surface changed.

## Service Discipline

- No vault writes from this service; all task creation goes through `task.CreateCommandSender` to Kafka.
- `task.CreateCommand.TaskIdentifier` is a deterministic UUID5 — same input on a retry MUST produce the same identifier.
- No HTTP `/trigger` endpoint side effects beyond enqueueing a single poll/tick.

## Out of Scope (do NOT add)

- `LICENSE` file
- Claude PR review workflow (`.github/workflows/claude*.yml`)
- `README.md` "Status: Under Development" banner
- Runtime feature flags for individual recurring tasks (per task-file design decisions)
