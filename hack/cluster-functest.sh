#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/common.sh"
source "${SCRIPT_DIR}/config.sh"

main() {
    log_info "Running e2e functional tests against kubevirtci cluster..."

    # Verify cluster is running
    if ! kubevirtci::is_running; then
        log_error "Cluster is not running. Start it with: make cluster-up"
        exit 1
    fi

    export KUBECONFIG=$(kubevirtci::kubeconfig)

    # Verify webhook is deployed
    if ! kubevirtci::kubectl get deployment -n ${WEBHOOK_NAMESPACE} controller-manager &>/dev/null; then
        log_error "Webhook not deployed. Deploy it with: make cluster-sync"
        exit 1
    fi

    # Run the e2e tests with kubevirtci mode
    log_info "Running e2e tests..."
    export USE_KUBEVIRTCI=true
    export PROJECT_IMAGE="${IMAGE_FULL}"
    (cd "${PROJECT_ROOT}" && go test ./test/e2e/... -v -ginkgo.v -timeout 30m)

    log_info ""
    log_info "✅ E2E tests completed successfully!"
}

main "$@"
