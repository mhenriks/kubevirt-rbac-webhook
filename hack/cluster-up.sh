#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"
source "${SCRIPT_DIR}/config.sh"

main() {
    log_info "Starting kubevirtci cluster for kubevirt-rbac-webhook development..."

    # Install kubevirtci if needed
    kubevirtci::install

    # Change to kubevirtci directory and start cluster
    log_info "Starting kubevirtci cluster..."
    export KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER:-k8s-1.34}
    export KUBEVIRTCI_PATH="${KUBEVIRTCI_ROOT}/cluster-up/"
    export KUBEVIRTCI_CLUSTER_PATH="${KUBEVIRTCI_ROOT}/cluster-up/cluster"
    log_info "Using provider: ${KUBEVIRT_PROVIDER}"
    cd "${KUBEVIRTCI_ROOT}"
    make cluster-up
    cd - >/dev/null

    # Export kubeconfig
    export KUBECONFIG=$(kubevirtci::kubeconfig)
    log_info "Cluster is up. KUBECONFIG: ${KUBECONFIG}"

    # Wait for cluster to be ready
    log_info "Waiting for cluster to be ready (this may take a few minutes)..."
    kubevirtci::kubectl wait --for=condition=Ready nodes --all --timeout=10m || {
        log_error "Cluster nodes did not become ready"
        exit 1
    }

    # Install KubeVirt if not present
    log_info "Checking for KubeVirt installation..."
    if ! kubevirtci::kubectl get crd virtualmachines.kubevirt.io &>/dev/null; then
        log_warn "KubeVirt not found, installing..."

        # Get latest KubeVirt version
        KUBEVIRT_VERSION=$(curl -s https://api.github.com/repos/kubevirt/kubevirt/releases/latest | grep tag_name | cut -d '"' -f 4)
        log_info "Installing KubeVirt ${KUBEVIRT_VERSION}..."

        # Install KubeVirt operator
        kubevirtci::kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

        # Install KubeVirt CR
        kubevirtci::kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"

        # Wait for KubeVirt to be ready
        log_info "Waiting for KubeVirt to be ready (this may take several minutes)..."
        kubevirtci::kubectl wait --for=condition=Available -n kubevirt kv/kubevirt --timeout=10m || {
            log_error "KubeVirt deployment failed"
            exit 1
        }

        log_info "KubeVirt installed successfully"
    else
        log_info "KubeVirt is already installed"
    fi

    # Wait for KubeVirt CRDs to be fully ready
    log_info "Verifying KubeVirt CRDs..."
    for i in {1..30}; do
        if kubevirtci::kubectl get crd virtualmachines.kubevirt.io &>/dev/null; then
            log_info "KubeVirt CRDs are ready"
            break
        fi
        sleep 2
    done

    if ! kubevirtci::kubectl get crd virtualmachines.kubevirt.io &>/dev/null; then
        log_error "KubeVirt CRDs not found after installation!"
        exit 1
    fi

    # Check if cert-manager is installed
    if ! kubevirtci::kubectl get crd certificates.cert-manager.io &>/dev/null; then
        log_warn "cert-manager not found, it will be installed during deployment"
    else
        log_info "cert-manager is already installed"
    fi

    log_info ""
    log_info "✅ Cluster is ready for development!"
    log_info "KUBECONFIG: ${KUBECONFIG}"
    log_info ""
    log_info "Next steps:"
    log_info "  1. Build and deploy webhook: make cluster-sync"
    log_info "  2. Run e2e tests: make cluster-functest"
    log_info "  3. Stop cluster: make cluster-down"
}

main "$@"
