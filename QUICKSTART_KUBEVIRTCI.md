# Quick Start: kubevirtci Development

## TL;DR

```bash
# Start cluster (one time)
make cluster-up

# Use kubectl
./kubectl.sh get nodes
./kubectl.sh get vms -A

# Deploy webhook
make cluster-sync

# Run e2e tests
make cluster-functest

# Stop cluster
make cluster-down
```

## Details

### 1. Start kubevirtci Cluster

```bash
make cluster-up
```

This will:
- Clone kubevirtci (if needed)
- Start Kubernetes 1.34 cluster
- Install KubeVirt v1.6.2 automatically
- Takes ~5-10 minutes on first run

### 2. Access the Cluster

Use the `kubectl.sh` wrapper in the project root:

```bash
./kubectl.sh get nodes
./kubectl.sh get pods -n kubevirt
./kubectl.sh get vms -A
```

The wrapper automatically sets up all required kubevirtci environment variables (`KUBEVIRTCI_TAG`, `KUBEVIRT_PROVIDER`, etc.).

### 3. Build and Deploy Webhook

```bash
make cluster-sync
```

This will:
- Build webhook Docker image
- Load it into the cluster
- Install cert-manager
- Deploy webhook with all ClusterRoles
- Takes ~2-3 minutes

### 4. Run E2E Tests

```bash
make cluster-functest
```

This runs comprehensive RBAC validation tests:
- Full-admin, storage-admin, cdrom-user, network-admin, compute-admin, lifecycle-admin, devices-admin
- Backwards compatibility tests
- Combined permissions tests
- Takes ~5-10 minutes

### 5. Development Iteration

After making code changes:

```bash
make cluster-sync           # Rebuild and redeploy
make cluster-functest       # Run tests
```

### 6. Stop Cluster

```bash
make cluster-down
```

## How kubectl.sh Works

During `make cluster-up`, the `kubevirtci::install()` function appends the required `KUBEVIRTCI_TAG` environment variable to kubevirtci's `cluster-up/hack/common.sh` file. Since kubectl.sh automatically sources that file, it works out of the box without any manual configuration.

## Troubleshooting

### "FATAL: either KUBEVIRTCI_TAG or KUBEVIRTCI_GOCLI_CONTAINER must be set"

This means kubevirtci wasn't installed via `make cluster-up`. Run:
```bash
make cluster-clean
make cluster-up
```

The installation process appends `KUBEVIRTCI_TAG` to kubevirtci's config.

### Cluster not starting

```bash
make cluster-down
make cluster-clean
make cluster-up
```

### Webhook deployment failed

```bash
./kubectl.sh get pods -n kubevirt-rbac-webhook-system
./kubectl.sh logs -n kubevirt-rbac-webhook-system -l control-plane=controller-manager
```

## Next Steps

See [KUBEVIRTCI.md](KUBEVIRTCI.md) for comprehensive documentation.
