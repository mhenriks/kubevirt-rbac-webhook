# KubeVirt RBAC Webhook

A Kubernetes validating admission webhook that provides fine-grained RBAC control for KubeVirt VirtualMachine resources. Instead of granting broad "update virtualmachines" permissions, this webhook enables administrators to define specific field-level permissions for VM operations like storage management.

## Overview

The webhook provides:

1. **Fine-grained ClusterRoles** - Defines specific permissions for VM operations
2. **Validating Webhook** - Enforces permission checks using SubjectAccessReview API
3. **Backwards Compatible** - Opt-in restrictions model that doesn't break existing RBAC

## Architecture

### ClusterRoles

The webhook deploys with fine-grained ClusterRoles using **Kubernetes role aggregation** (in `config/clusterroles/`):

#### `kubevirt.io:vm-full-admin` (Aggregated)
The complete admin role that grants **unrestricted access to all VirtualMachine fields** (spec and metadata).

**Important:** This role allows modifying the **entire VirtualMachine spec and metadata**, not just the fields covered by the granular roles. Users with this permission can change any field including those not explicitly managed by other checkers.

When granted, users get all VM management capabilities:
- Storage (volumes, disks, CD-ROMs, filesystems)
- Network (interfaces, networks)
- Compute (CPU, memory, resources)
- Devices (GPUs, host devices, watchdog, TPM, inputs)
- Lifecycle (start, stop, restart, runStrategy)
- **Any other spec or metadata fields**

Uses Kubernetes role aggregation to:
- Automatically include all roles labeled with `rbac.kubevirt.io/aggregate-to-vm-full-admin: "true"`
- Aggregate into Kubernetes built-in `admin` and `edit` ClusterRoles

#### `kubevirt.io:vm-storage-admin`
Allows users to modify **all VM storage** (volumes, disks, including CD-ROMs and filesystems):
- Add/remove volumes (PVCs, DataVolumes, ConfigMaps, Secrets, etc.)
- Modify disk attachments
- Configure filesystems (virtio-fs)
- Includes all CD-ROM operations (superset of cdrom-user)

#### `kubevirt.io:vm-network-admin`
Allows users to modify **VM network configuration**:
- Add/remove network interfaces
- Configure network attachments

#### `kubevirt.io:vm-compute-admin`
Allows users to modify **VM compute resources**:
- CPU configuration (cores, sockets, threads)
- Memory and resource requests/limits

#### `kubevirt.io:vm-devices-admin`
Allows users to modify **VM device configuration**:
- GPUs
- Host devices (PCI passthrough)
- Watchdog
- TPM (Trusted Platform Module)
- Input devices

#### `kubevirt.io:vm-lifecycle-admin`
Allows users to **control VM lifecycle** (start/stop/restart):
- Modify `spec.running` field
- Modify `spec.runStrategy` field
- Use KubeVirt subresource APIs: `start`, `stop`, `restart`, `softreboot`

#### `kubevirt.io:vm-cdrom-user`
Allows users to **only** inject, eject, and swap CD-ROM media (subset of storage-admin):
- Change hotpluggable CD-ROM volumes
- Cannot add/remove CD-ROM drives
- Cannot modify other storage

**Permission Hierarchy:**
- `vm-full-admin` → All VM permissions (aggregated)
- `vm-storage-admin` → Full storage control (superset: includes CD-ROMs + all other storage)
- `vm-cdrom-user` → CD-ROM media only (subset: only hotpluggable CD-ROM media)

### Validating Webhook

The webhook intercepts VirtualMachine update requests and enforces an **opt-in restrictions** security model (backwards compatible):

1. ✅ User has `virtualmachines/full-admin` → **Allow all changes to spec and metadata** (unrestricted)
2. ✅ User has standard `update virtualmachines` BUT NO subresource permissions → **Allow all changes** (backwards compatible)
3. ✅ User has `virtualmachines/storage-admin` + making storage changes → **Allow**
4. ✅ User has `virtualmachines/network-admin` + making network changes → **Allow**
5. ✅ User has `virtualmachines/compute-admin` + making CPU/memory changes → **Allow**
6. ✅ User has `virtualmachines/devices-admin` + making device changes → **Allow**
7. ✅ User has `virtualmachines/lifecycle-admin` + changing running/runStrategy → **Allow**
8. ✅ User has `virtualmachines/cdrom-user` + swapping CD-ROM media → **Allow**
9. ❌ User has `virtualmachines/storage-admin` + making non-storage changes → **Deny**
10. ❌ User has `virtualmachines/cdrom-user` + making storage changes → **Deny**

**Backwards Compatibility:** Users with existing `update virtualmachines` permissions continue to work as before. The fine-grained restrictions only apply when users are granted the new subresource permissions (opt-in model).

## Getting Started

### Prerequisites

- Kubernetes cluster (v1.28+)
- KubeVirt installed
- `kubectl` configured
- **Docker or Podman** (for building the webhook container image)
- **Container registry access** (to push your built image)
- **cert-manager installed** (required for webhook certificates)

### Installation

1. **Install cert-manager** (if not already installed):
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

Wait for cert-manager to be ready:
```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=60s
```

2. **Build and push the webhook container image**:
```bash
# Build the image
make docker-build IMG=<your-registry>/kubevirt-rbac-webhook:latest

# Push to your registry
make docker-push IMG=<your-registry>/kubevirt-rbac-webhook:latest
```

Replace `<your-registry>` with your container registry (e.g., `docker.io/myuser`, `quay.io/myorg`, `ghcr.io/myuser`).

3. **Deploy the webhook**:
```bash
make deploy IMG=<your-registry>/kubevirt-rbac-webhook:latest
```

This will create:
- Webhook Deployment and Service (using your built image)
- ValidatingWebhookConfiguration (with cert-manager CA injection)
- Fine-grained ClusterRoles (`kubevirt.io:vm-storage-admin`, etc.)
- Required RBAC permissions

4. **Verify the installation**:
```bash
# Check ClusterRoles were created
kubectl get clusterroles | grep kubevirt.io:vm-
# Should show: vm-full-admin, vm-storage-admin, vm-network-admin, vm-compute-admin,
#              vm-devices-admin, vm-lifecycle-admin, vm-cdrom-user

# Check webhook configuration
kubectl get validatingwebhookconfigurations | grep kubevirt-rbac

# Check webhook pod
kubectl get pods -n kubevirt-rbac-webhook-system
```

### Cleanup

To remove the webhook and all its resources:

```bash
make undeploy
```

This removes:
- Webhook Deployment
- ValidatingWebhookConfiguration
- All ClusterRoles
- Service, RBAC, etc.

### Usage Example

1. **Grant storage-admin permission to a user** (opt-in to restrictions):
```bash
kubectl create rolebinding user-vm-storage-admin \
  --clusterrole=kubevirt.io:vm-storage-admin \
  --user=alice \
  --namespace=default
```

2. **Test permissions (opt-in restrictions behavior)**:
   - User `alice` (storage-admin):
     - ✅ **Can** add/remove/modify ALL volumes and disks (including CD-ROMs)
     - ❌ **Cannot** change CPU, memory, or running state (opted-in to restrictions)
   - User `bob` (cdrom-user):
     - ✅ **Can** inject/eject/swap CD-ROM media in existing CD-ROM drives
     - ❌ **Cannot** modify other storage, CPU, memory, or add/remove CD-ROM drives (opted-in to restrictions)
   - User `charlie` (standard update permissions, no subresource):
     - ✅ **Can** modify all VM properties (backwards compatible)
   - User `dave` (no RBAC permissions):
     - ❌ **Cannot** modify VMs (standard Kubernetes RBAC)

## Development

### Quick Start with kubevirtci

For development and testing with real KubeVirt VirtualMachines, use kubevirtci:

```bash
# Start kubevirtci cluster with KubeVirt pre-installed
make cluster-up

# Access the cluster
_kubevirtci/cluster-up/kubectl.sh get nodes
_kubevirtci/cluster-up/kubectl.sh get vms -A

# Build, deploy webhook, and run e2e tests
make cluster-sync
make cluster-functest

# Stop cluster
make cluster-down
```

See [QUICKSTART_KUBEVIRTCI.md](QUICKSTART_KUBEVIRTCI.md) for quick start or [KUBEVIRTCI.md](KUBEVIRTCI.md) for detailed guide.

### Building

```bash
make build
```

### Running locally

```bash
make run        # Run webhook locally (requires kubeconfig)
```

### Testing

```bash
# Unit tests
make test

# E2E tests with kubevirtci (recommended)
make cluster-up cluster-sync cluster-functest

# E2E tests with kind (legacy)
make test-e2e
```

### Building Docker image

```bash
make docker-build docker-push IMG=<your-registry>/kubevirt-rbac-webhook:tag
```

## Configuration

### Environment Variables

- `ENABLE_WEBHOOKS`: Set to `false` to disable webhooks (default: `true`)

### Webhook Configuration

The webhook is enabled by default. To disable it temporarily, set the `ENABLE_WEBHOOKS=false` environment variable on the Deployment.

## Current Permissions

✅ **Implemented:**
- `vm-storage-admin`: Manage **all** storage (volumes, disks, including CD-ROMs) - **Superset**
- `vm-cdrom-user`: Inject/eject/swap CD-ROM media only (hotpluggable) - **Subset of storage-admin**

## Future Enhancements

Additional fine-grained permissions can be added:

- `vm-network-admin`: Manage network interfaces
- `vm-cpu-memory-admin`: Modify CPU/memory resources
- `vm-device-admin`: Manage devices (GPUs, etc.)

## Architecture Decisions

### Webhook Validation Flow
1. Extract user info from admission request
2. Check wildcard `update * in kubevirt.io` permission → allow if true (backwards compatible)
3. Check if user has ANY subresource permissions (e.g., `virtualmachines/storage-admin`, `virtualmachines/cdrom-user`)
4. If NO subresource permissions → allow (backwards compatible)
5. If YES subresource permissions → validate each change against those permissions (opt-in restrictions):
   - For each FieldPermissionChecker (storage, cdrom, etc.):
     - Check if that field category changed
     - If changed, verify user has permission for that subresource
     - If authorized, neutralize those fields from further checks
     - If unauthorized, deny immediately
6. After all checks, if any unauthorized changes remain → deny
7. Otherwise, allow the request

### Architecture: Dependency Injection for Testability

The webhook uses dependency injection for high testability:

```go
type PermissionChecker interface {
    CheckPermission(ctx, userInfo, namespace, vmName, subresource string) (bool, error)
}

type VirtualMachineCustomValidator struct {
    Client            client.Client
    FieldCheckers     []FieldPermissionChecker  // Injected list of checkers
    PermissionChecker PermissionChecker         // Injected permission checker
}
```

**Benefits:**
- **88.8% test coverage** (up from 61.8% before refactoring)
- Easy to mock in unit tests
- Production uses `SubjectAccessReviewPermissionChecker`
- Tests use `MockPermissionChecker`

### SubjectAccessReview
The webhook uses Kubernetes SubjectAccessReview API to check permissions dynamically, ensuring consistency with the cluster's RBAC configuration. Importantly, **the webhook passes the specific VM name** in the permission check, enabling resource-name-specific RBAC policies.

**Example: Per-VM Permissions**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: storage-admin-specific-vms
  namespace: default
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  resourceNames: ["test-vm", "dev-vm"]  # Only these specific VMs
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines/storage-admin"]
  resourceNames: ["test-vm", "dev-vm"]
  verbs: ["update"]
```

This enables administrators to grant granular permissions like "alice can manage storage on test-vm but not prod-vm".

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
