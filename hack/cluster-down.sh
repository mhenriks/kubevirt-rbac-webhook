#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

main() {
    log_info "Stopping kubevirtci cluster..."
    kubevirtci::down
    log_info "Cluster stopped successfully"
}

main "$@"
