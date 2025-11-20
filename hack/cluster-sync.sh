#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/common.sh"
source "${SCRIPT_DIR}/config.sh"

main() {
    log_info "Syncing kubevirt-rbac-webhook to cluster..."

    # Verify cluster is running
    if ! kubevirtci::is_running; then
        log_error "Cluster is not running. Start it with: make cluster-up"
        exit 1
    fi

    export KUBECONFIG=$(kubevirtci::kubeconfig)

    # Build and push image to cluster registry
    # For kind providers, registry is on localhost:5000; for k8s-* providers, use cli.sh
    if [[ "${KUBEVIRT_PROVIDER}" == kind-* ]]; then
        log_info "Kind provider detected, using kubevirtci registry"

        # For kind providers, kubevirtci creates a registry on localhost:5000
        REGISTRY_ADDR="localhost:5000"
        log_info "Cluster registry: ${REGISTRY_ADDR}"

        # Build and push the webhook image to kubevirtci registry
        # Prefer podman (has --tls-verify flag), fallback to docker
        if command -v podman &> /dev/null; then
            log_info "Building and pushing webhook image with podman..."
            (cd "${PROJECT_ROOT}" && \
                make docker-build CONTAINER_TOOL=podman IMG="${REGISTRY_ADDR}/kubevirt-rbac-webhook:devel" && \
                podman push --tls-verify=false "${REGISTRY_ADDR}/kubevirt-rbac-webhook:devel"
            )
        else
            log_info "Building and pushing webhook image with docker..."
            # Docker requires insecure registry to be configured, but localhost:5000 is usually allowed by default
            (cd "${PROJECT_ROOT}" && \
                make docker-build CONTAINER_TOOL=docker IMG="${REGISTRY_ADDR}/kubevirt-rbac-webhook:devel" && \
                docker push "${REGISTRY_ADDR}/kubevirt-rbac-webhook:devel"
            )
        fi

        # Deploy using in-cluster registry name
        DEPLOY_IMAGE="registry:5000/kubevirt-rbac-webhook:devel"
    else
        log_info "Using kubevirtci registry for k8s-* provider"

        # Get the registry port from kubevirtci
        REGISTRY_PORT=$("${KUBEVIRTCI_ROOT}/cluster-up/cli.sh" ports registry)
        REGISTRY_ADDR="localhost:${REGISTRY_PORT}"
        log_info "Cluster registry: ${REGISTRY_ADDR}"

        # Build and push the webhook image to kubevirtci registry
        # Use podman with --tls-verify=false for HTTP registry
        log_info "Building and pushing webhook image..."
        (cd "${PROJECT_ROOT}" && \
            make docker-build CONTAINER_TOOL=podman IMG="${REGISTRY_ADDR}/kubevirt-rbac-webhook:devel" && \
            podman push --tls-verify=false "${REGISTRY_ADDR}/kubevirt-rbac-webhook:devel"
        )

        # Deploy using in-cluster registry name
        DEPLOY_IMAGE="registry:5000/kubevirt-rbac-webhook:devel"
    fi

    # Check and install cert-manager if needed
    if ! kubevirtci::kubectl get crd certificates.cert-manager.io &>/dev/null; then
        log_info "Installing cert-manager..."
        kubevirtci::kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
        kubevirtci::kubectl wait --for=condition=Available -n cert-manager deployment/cert-manager-webhook --timeout=2m
        log_info "cert-manager installed successfully"
    fi

    # Undeploy old version for clean reinstall
    log_info "Cleaning up previous deployment..."
    (cd "${PROJECT_ROOT}" && make undeploy || true)

    # Deploy the webhook
    log_info "Deploying webhook to cluster..."
    (cd "${PROJECT_ROOT}" && make deploy IMG="${DEPLOY_IMAGE}")

    # Wait for webhook to be ready
    log_info "Waiting for webhook deployment to be ready..."
    kubevirtci::kubectl wait --for=condition=Available \
        -n ${WEBHOOK_NAMESPACE} \
        deployment/controller-manager \
        --timeout=5m || {
        log_error "Webhook deployment failed to become ready"
        log_info "Pod status:"
        kubevirtci::kubectl get pods -n ${WEBHOOK_NAMESPACE}
        log_info "Pod logs:"
        kubevirtci::kubectl logs -n ${WEBHOOK_NAMESPACE} -l control-plane=controller-manager --tail=50
        exit 1
    }

    # Verify webhook is registered
    log_info "Verifying webhook registration..."
    if ! kubevirtci::kubectl get validatingwebhookconfigurations.admissionregistration.k8s.io kubevirt-rbac-validating-webhook &>/dev/null; then
        log_error "Webhook configuration not found!"
        exit 1
    fi

    log_info ""
    log_info "✅ Webhook deployed successfully!"
    log_info ""
    log_info "ClusterRoles:"
    kubevirtci::kubectl get clusterroles | grep "kubevirt.io:vm-" || true
    log_info ""
    log_info "Webhook pods:"
    kubevirtci::kubectl get pods -n ${WEBHOOK_NAMESPACE}
    log_info ""
    log_info "Next step: Run e2e tests with 'make cluster-functest'"
}

main "$@"
