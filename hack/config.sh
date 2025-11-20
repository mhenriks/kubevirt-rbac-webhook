#!/usr/bin/env bash

# Configuration for kubevirt-rbac-webhook
# This file is sourced by hack/*.sh scripts and provides all configuration variables

# Image configuration
IMAGE_REGISTRY=${IMAGE_REGISTRY:-localhost:5000}
IMAGE_NAME=${IMAGE_NAME:-kubevirt-rbac-webhook}
IMAGE_TAG=${IMAGE_TAG:-devel}
IMAGE_FULL="${IMAGE_REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

# Namespace where webhook will be deployed
WEBHOOK_NAMESPACE=${WEBHOOK_NAMESPACE:-kubevirt-rbac-webhook-system}

# kubevirtci configuration
# KUBEVIRTCI_VERSION: Version/tag of kubevirtci to use
# KUBEVIRTCI_TAG: Required by kubevirtci's kubectl.sh (set during installation)
# NOTE: During 'make cluster-up', kubevirtci::install() appends KUBEVIRTCI_TAG
# to kubevirtci's cluster-up/hack/common.sh so kubectl.sh works directly
KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER:-kind-1.34}
KUBEVIRTCI_VERSION=${KUBEVIRTCI_VERSION:-2510141807-f21813f1}
KUBEVIRTCI_TAG=${KUBEVIRTCI_TAG:-$KUBEVIRTCI_VERSION}
KUBEVIRTCI_ROOT="${KUBEVIRTCI_ROOT:-$(pwd)/_kubevirtci}"
KUBEVIRTCI_PATH="${KUBEVIRTCI_ROOT}/cluster-up/"
KUBEVIRTCI_CLUSTER_PATH="${KUBEVIRTCI_ROOT}/cluster-up/cluster"
KUBEVIRTCI_CONFIG_PATH="${KUBEVIRTCI_ROOT}/_ci-configs"

export IMAGE_REGISTRY
export IMAGE_NAME
export IMAGE_TAG
export IMAGE_FULL
export WEBHOOK_NAMESPACE
export KUBEVIRT_PROVIDER
export KUBEVIRTCI_VERSION
export KUBEVIRTCI_TAG
export KUBEVIRTCI_ROOT
export KUBEVIRTCI_PATH
export KUBEVIRTCI_CLUSTER_PATH
export KUBEVIRTCI_CONFIG_PATH
