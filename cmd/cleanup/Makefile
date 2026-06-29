include ../../Makefile.variables

# cmd/cleanup is a deployed binary; apply is handled by k8s/Makefile.
# This Makefile participates in the parent build/apply sweep.
# The actual k8s apply is done via the parent Makefile.docker's buca-cleanup target.
.PHONY: apply
apply:
	@echo "apply for cmd/cleanup is handled by k8s/Makefile"
