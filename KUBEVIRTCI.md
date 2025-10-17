# kubevirtci Integration Guide

This project uses [kubevirtci](https://github.com/kubevirt/kubevirtci) for development and e2e testing with real KubeVirt VirtualMachines.

## Why kubevirtci?

- **Native KubeVirt Support**: Comes with KubeVirt pre-installed and configured
- **Real VM Testing**: Test webhook RBAC validation with actual VirtualMachine resources
- **Realistic Environment**: Same environment as KubeVirt users
- **Nested Virtualization**: Handles complexity of running VMs in tests

## Quick Start

### 1. Start kubevirtci Cluster

```bash
make cluster-up
```

This will:
- Clone kubevirtci (if not already present in `_kubevirtci/`)
- Start a Kubernetes cluster with KubeVirt pre-installed
- Export KUBECONFIG for the cluster

### 2. Build and Deploy Webhook

```bash
make cluster-sync
```

This will:
- Build the webhook container image
- Load it into the kubevirtci cluster
- Install cert-manager (if needed)
- Deploy the webhook with all ClusterRoles
- Wait for webhook to be ready

### 3. Run E2E Tests

```bash
make cluster-functest
```

This will run comprehensive e2e tests including:
- Full-admin permission tests
- Storage-admin permission tests (volumes, disks, filesystems)
- CD-ROM user permission tests (media swap only)
- Network-admin permission tests
- Compute-admin permission tests (CPU, memory)
- Lifecycle-admin permission tests (start/stop)
- Devices-admin permission tests (GPUs, host devices)
- Backwards compatibility tests
- Combined permissions tests

### 4. Stop Cluster

```bash
make cluster-down
```

### 5. Clean Everything

```bash
make cluster-clean
```

This removes the kubevirtci directory and all cached data.

## Development Workflow

Typical development cycle:

```bash
# Start cluster (once)
make cluster-up

# Make code changes, then sync and test
make cluster-sync
make cluster-functest

# Or combine: sync + test
make cluster-sync && make cluster-functest

# When done
make cluster-down
```

## Configuration

Configuration is managed in `hack/config.sh`:

- `IMAGE_REGISTRY`: Container registry (default: `localhost:5000`)
- `IMAGE_NAME`: Image name (default: `kubevirt-rbac-webhook`)
- `IMAGE_TAG`: Image tag (default: `devel`)
- `KUBEVIRT_PROVIDER`: Kubernetes version (default: `k8s-1.34`)
- `KUBEVIRTCI_VERSION`: kubevirtci version/tag (default: `2510141807-f21813f1`)

## Manual kubectl Access

### Use kubectl.sh directly

After running `make cluster-up`, kubectl.sh works automatically:

```bash
_kubevirtci/cluster-up/kubectl.sh get nodes
_kubevirtci/cluster-up/kubectl.sh get vms -A
_kubevirtci/cluster-up/kubectl.sh get pods -n kubevirt
```

**How it works:**
- During `make cluster-up`, the installation appends `export KUBEVIRTCI_TAG=<version>` to kubevirtci's `cluster-up/hack/common.sh`
- kubectl.sh automatically sources that file, so all required environment variables are set

### Alternative: Use kubectl directly with KUBECONFIG

```bash
export KUBECONFIG=$(pwd)/_kubevirtci/_ci-configs/k8s-1.34/.kubeconfig
kubectl get nodes
kubectl get vms -A
```

### In Shell Scripts

Helper scripts source `hack/config.sh` and use the `kubevirtci::kubectl` function:

```bash
#!/usr/bin/env bash
source hack/common.sh
source hack/config.sh
kubevirtci::kubectl get nodes
```

## Troubleshooting

### Webhook Not Ready

```bash
kubectl get pods -n kubevirt-rbac-webhook-system
kubectl logs -n kubevirt-rbac-webhook-system -l control-plane=controller-manager
```

### KubeVirt Not Working

```bash
kubectl get pods -n kubevirt
kubectl get virt -n kubevirt
```

### Cluster Issues

```bash
# Restart cluster
make cluster-down
make cluster-up
make cluster-sync
```

### Clean Slate

```bash
make cluster-clean
make cluster-up
make cluster-sync
```

## CI Integration

The GitHub Actions workflow (`.github/workflows/test-e2e.yml`) automatically:
1. Sets up kubevirtci cluster
2. Builds and deploys webhook
3. Runs all e2e tests
4. Collects logs on failure
5. Cleans up cluster

## Architecture

The kubevirtci integration consists of:

### Helper Scripts (`hack/`)

- `common.sh`: Core kubevirtci functions (up, down, load image, etc.)
- `config.sh`: Configuration variables
- `cluster-up.sh`: Start cluster
- `cluster-down.sh`: Stop cluster
- `cluster-sync.sh`: Build and deploy
- `cluster-functest.sh`: Run e2e tests

### Test Utilities (`test/utils/`)

- VM creation helpers (`CreateTestVM`, `CreateVMWithCDRom`)
- RBAC helpers (`CreateServiceAccount`, `CreateRoleBinding`)
- Impersonation helpers (`KubectlAs`, `PatchResourceAs`)

### E2E Tests (`test/e2e/`)

- `webhook_rbac_test.go`: Comprehensive RBAC validation tests
- `e2e_suite_test.go`: Test suite setup (supports both kind and kubevirtci)
- `e2e_test.go`: Infrastructure tests (deployment, metrics, certificates)

## Comparison with kind

| Feature | kind | kubevirtci |
|---------|------|------------|
| KubeVirt | Manual install | Pre-installed |
| VirtualMachine CRDs | Manual install | Pre-installed |
| Nested VMs | Not supported | Supported |
| Setup time | Fast (~1 min) | Slower (~3-5 min) |
| Resource usage | Light | Heavier |
| Test realism | Basic | Production-like |

## Best Practices

1. **Keep cluster running during development** - Start once, sync many times
2. **Use `cluster-sync` after code changes** - Rebuilds and redeploys
3. **Run `cluster-functest` frequently** - Fast feedback on changes
4. **Clean up when switching branches** - `make cluster-clean && make cluster-up`
5. **Check webhook logs when debugging** - `kubectl logs -n kubevirt-rbac-webhook-system ...`

## Environment Variables

The test suite supports these environment variables:

- `USE_KUBEVIRTCI=true`: Use kubevirtci mode in tests
- `PROJECT_IMAGE`: Override webhook image (set by `cluster-functest`)
- `KUBECONFIG`: Path to cluster kubeconfig (set by kubevirtci)

## Related Documentation

- [kubevirtci GitHub](https://github.com/kubevirt/kubevirtci)
- [KubeVirt Documentation](https://kubevirt.io/user-guide/)
- [virt-template (reference implementation)](https://github.com/kubevirt/virt-template)
