#!/usr/bin/env bash

# kubevirtci integration utilities
# Based on patterns from kubevirt/virt-template

set -e

# kubevirtci path configuration
KUBEVIRTCI_ROOT=${KUBEVIRTCI_ROOT:-$(pwd)/_kubevirtci}
KUBEVIRTCI_PATH="${KUBEVIRTCI_ROOT}/cluster-up/"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

kubevirtci::install() {
    if [ ! -d "${KUBEVIRTCI_ROOT}" ]; then
        log_info "Installing kubevirtci ${KUBEVIRTCI_VERSION}..."
        git clone https://github.com/kubevirt/kubevirtci.git "${KUBEVIRTCI_ROOT}"
        (cd "${KUBEVIRTCI_ROOT}" && git checkout "${KUBEVIRTCI_VERSION}")

        # Append KUBEVIRTCI_TAG to kubevirtci's common.sh
        # This allows kubectl.sh to work directly without manual environment setup
        echo "export KUBEVIRTCI_TAG=${KUBEVIRTCI_VERSION}" >> "${KUBEVIRTCI_ROOT}/cluster-up/hack/common.sh"

        log_info "kubevirtci installed successfully"
    else
        log_info "kubevirtci already installed at ${KUBEVIRTCI_ROOT}"
    fi
}

kubevirtci::up() {
    log_info "Starting kubevirtci cluster..."
    kubevirtci::install

    # Set the provider
    export KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER:-k8s-1.30}

    # Run cluster-up from kubevirtci root directory
    (cd "${KUBEVIRTCI_ROOT}" && make cluster-up)

    # Export kubeconfig
    export KUBECONFIG=$(kubevirtci::kubeconfig)
    log_info "Cluster is up. KUBECONFIG: ${KUBECONFIG}"
}

kubevirtci::down() {
    if [ -d "${KUBEVIRTCI_ROOT}" ]; then
        log_info "Stopping kubevirtci cluster..."
        (cd "${KUBEVIRTCI_ROOT}" && make cluster-down) || true
        log_info "Cluster stopped"
    else
        log_warn "kubevirtci not found at ${KUBEVIRTCI_ROOT}"
    fi
}

kubevirtci::kubeconfig() {
    local provider=${KUBEVIRT_PROVIDER:-k8s-1.34}
    echo "${KUBEVIRTCI_ROOT}/_ci-configs/${provider}/.kubeconfig"
}

kubevirtci::kubectl() {
    # Use kubevirtci's kubectl.sh wrapper which properly sets up the environment
    "${KUBEVIRTCI_ROOT}/cluster-up/kubectl.sh" "$@"
}

kubevirtci::ssh() {
    "${KUBEVIRTCI_ROOT}/cluster-up/ssh.sh" "$@"
}

# Get the dynamic cluster registry port
kubevirtci::registry() {
    local port=$("${KUBEVIRTCI_ROOT}/cluster-up/cli.sh" ports registry)
    echo "localhost:${port}"
}

# Check if cluster is running
kubevirtci::is_running() {
    local kubeconfig=$(kubevirtci::kubeconfig)
    if [ -f "${kubeconfig}" ]; then
        if kubectl --kubeconfig="${kubeconfig}" cluster-info &>/dev/null; then
            return 0
        fi
    fi
    return 1
}

# Note: Functions are not exported as they don't work across all shells (bash vs zsh)
# Scripts that need these functions should source this file directly
