#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"
source "${SCRIPT_DIR}/config.sh"

main() {
    log_info "Stopping kubevirtci cluster..."

    # Run kubevirtci's cluster-down
    kubevirtci::down

    log_info "Cluster stopped successfully"
}

main "$@"
