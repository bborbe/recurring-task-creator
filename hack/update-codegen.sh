#!/usr/bin/env bash
#
# Regenerates clientset + informers + listers + applyconfiguration
# for the Schedule CRD. Idempotent: a second run produces no diff.
#
# Sources kube_codegen.sh from the vendored k8s.io/code-generator module
# (resolved via `go list -m -f '{{.Dir}}'`, which works inside and outside
# the module directory). Do NOT add `go mod vendor` to the pre-codegen
# sequence — kube_codegen.sh is a shell script and `go mod vendor` only
# vendors .go files.
#
# Note: the DeepCopy for Schedule/ScheduleList/ScheduleSpec/ScheduleStatus/
# ScheduleTrigger/ScheduleTemplate is HAND-ROLLED in zz_generated.deepcopy.go
# because lib.TaskFrontmatter is map[string]interface{}; the standard
# deepcopy-gen (and controller-gen) cannot emit a DeepCopy for a literal
# interface{} value. The hand-rolled file uses runtime.DeepCopyJSON for
# the frontmatter field. gen_helpers is intentionally NOT invoked here.
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)
THIS_PKG="github.com/bborbe/recurring-task-creator"

CODEGEN_PKG=$(cd "${SCRIPT_ROOT}" && go list -m -f '{{.Dir}}' k8s.io/code-generator 2>/dev/null || echo "")
if [[ -z "${CODEGEN_PKG}" ]]; then
    echo "k8s.io/code-generator not found in go.mod. Run: go get k8s.io/code-generator@v0.36.1" >&2
    exit 1
fi

source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_client \
    --with-watch \
    --with-applyconfig \
    --output-dir "${SCRIPT_ROOT}/k8s/client" \
    --output-pkg "${THIS_PKG}/k8s/client" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/k8s/apis"
