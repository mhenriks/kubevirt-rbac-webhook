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
    export KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER:-kind-1.34}
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
    if ! kubevirtci::kubectl wait --for=condition=Ready nodes --all --timeout=10m; then
        log_error "Cluster nodes did not become ready"
        log_error "Collecting cluster diagnostic information..."

        echo "====== Cluster Info ======"
        kubevirtci::kubectl cluster-info || true

        echo "====== Node Status ======"
        kubevirtci::kubectl get nodes -o wide || true
        kubevirtci::kubectl describe nodes || true

        echo "====== System Pods ======"
        kubevirtci::kubectl get pods -A || true

        exit 1
    fi

    # Check if nested virtualization is available
    log_info "Checking virtualization support..."
    if [ -e /dev/kvm ]; then
        if [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
            log_info "✓ /dev/kvm is accessible"
            log_info "   KubeVirt will use hardware acceleration (fast)"
        else
            log_warn "⚠ /dev/kvm exists but not accessible"
            log_warn "   Software emulation will be used (slower)"
            log_warn "   For Docker: Should work as-is (runs as root)"
            log_warn "   For rootless podman: Consider using Docker instead"
        fi
    else
        log_warn "⚠ /dev/kvm NOT available"
        log_warn "   Software emulation will be automatically enabled (VERY SLOW)"
        log_warn "   For CI: This is expected and acceptable for basic testing"
        log_warn "   For local dev: Enable virtualization in BIOS and kernel modules"
    fi

    # Install KubeVirt if not present
    log_info "Checking for KubeVirt installation..."
    if ! kubevirtci::kubectl get crd virtualmachines.kubevirt.io &>/dev/null; then
        log_warn "KubeVirt not found, installing..."

        # Get latest KubeVirt version
        KUBEVIRT_VERSION=$(curl -s https://api.github.com/repos/kubevirt/kubevirt/releases/latest | grep tag_name | cut -d '"' -f 4)
        log_info "Installing KubeVirt ${KUBEVIRT_VERSION}..."

        # Install KubeVirt operator
        kubevirtci::kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

        # Wait for operator to be ready
        log_info "Waiting for KubeVirt operator to be ready..."
        kubevirtci::kubectl wait --for=condition=Available -n kubevirt deployment/virt-operator --timeout=5m

        # Install KubeVirt CR
        kubevirtci::kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"

        # Enable software emulation if /dev/kvm is not available or not accessible
        NEED_EMULATION=false
        if [ ! -e /dev/kvm ]; then
            log_warn "/dev/kvm does not exist - emulation required"
            NEED_EMULATION=true
        elif [ ! -r /dev/kvm ] || [ ! -w /dev/kvm ]; then
            log_warn "/dev/kvm exists but is not accessible - emulation may be required"
            NEED_EMULATION=true
        fi

        if [ "$NEED_EMULATION" = true ]; then
            log_warn "Enabling software emulation in KubeVirt"
            log_warn "This will make VMs run VERY slowly - only suitable for basic testing"

            # Patch the KubeVirt CR to enable software emulation
            kubevirtci::kubectl patch kubevirt kubevirt -n kubevirt --type=merge -p '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true}}}}'
        else
            log_info "Hardware acceleration available - KubeVirt will use /dev/kvm"
        fi

        # Wait for KubeVirt to be ready
        log_info "Waiting for KubeVirt to be ready (this may take several minutes)..."
        if ! kubevirtci::kubectl wait --for=condition=Available -n kubevirt kv/kubevirt --timeout=10m; then
            log_error "KubeVirt deployment timed out or failed"
            log_error "Collecting diagnostic information..."

            echo "====== KubeVirt Resource Status ======"
            kubevirtci::kubectl get kv -n kubevirt -o yaml || true

            echo "====== KubeVirt Pods Status ======"
            kubevirtci::kubectl get pods -n kubevirt -o wide || true

            echo "====== KubeVirt Pods with Restart Counts ======"
            kubevirtci::kubectl get pods -n kubevirt -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[*].restartCount,READY:.status.containerStatuses[*].ready || true

            echo "====== KubeVirt DaemonSets (virt-handler) ======"
            kubevirtci::kubectl get daemonsets -n kubevirt -o wide || true
            kubevirtci::kubectl describe daemonsets -n kubevirt virt-handler || true

            echo "====== KubeVirt Events ======"
            kubevirtci::kubectl get events -n kubevirt --sort-by='.lastTimestamp' || true

            echo "====== KubeVirt Operator Logs ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-operator --tail=100 || true
            echo "====== KubeVirt Operator Previous Logs (if crash-looping) ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-operator --previous --tail=50 2>/dev/null || echo "No previous operator logs"

            echo "====== KubeVirt API Logs ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-api --tail=50 || true
            echo "====== KubeVirt API Previous Logs (if crash-looping) ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-api --previous --tail=50 2>/dev/null || echo "No previous API logs"

            echo "====== KubeVirt Controller Logs ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-controller --tail=50 || true
            echo "====== KubeVirt Controller Previous Logs (if crash-looping) ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-controller --previous --tail=50 2>/dev/null || echo "No previous controller logs"

            echo "====== KubeVirt Handler Logs (DaemonSet - runs on nodes) ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-handler --tail=100 --all-containers=true || true

            echo "====== KubeVirt Handler Previous Logs (if crash-looping) ======"
            kubevirtci::kubectl logs -n kubevirt -l kubevirt.io=virt-handler --previous --tail=100 --all-containers=true 2>/dev/null || echo "No previous logs available (pods may not have restarted yet)"

            echo "====== Node Status ======"
            kubevirtci::kubectl get nodes -o wide || true
            kubevirtci::kubectl describe nodes || true

            exit 1
        fi

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
        log_error "Collecting CRD diagnostic information..."

        echo "====== All CRDs ======"
        kubevirtci::kubectl get crds | grep -i kubevirt || true

        echo "====== KubeVirt Namespace Status ======"
        kubevirtci::kubectl get all -n kubevirt || true

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
