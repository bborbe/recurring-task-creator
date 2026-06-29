include ../../Makefile.variables

cleanup-run-once:
	go run -mod=mod . \
		-namespace=default \
		-git-rest-url="http://localhost:8080" \
		-v=2

# No-op so the recursive `apply` sweep from the parent Makefile.docker doesn't
# choke on this directory. cmd/cleanup-run-once is a local smoke-test CLI, not a
# deployable artifact.
.PHONY: apply
apply:
	@echo "skip apply for cmd/cleanup-run-once (local CLI, not deployed)"
