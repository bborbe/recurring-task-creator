include ../../Makefile.variables

run-once:
	go run -mod=mod . \
		-kafka-brokers="$(KAFKA_BROKERS)" \
		-v=2

# No-op so the recursive `apply` sweep from the parent Makefile.docker doesn't
# choke on this directory. cmd/run-once is a local smoke-test CLI, not a
# deployable artifact.
.PHONY: apply
apply:
	@echo "skip apply for cmd/run-once (local CLI, not deployed)"
