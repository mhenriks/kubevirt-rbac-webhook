# Quick Start Guide

This guide helps you quickly deploy and test the KubeVirt RBAC Webhook.

## Prerequisites

- Kubernetes cluster with KubeVirt installed
- `kubectl` configured to access the cluster
- **cert-manager installed** (required for webhook TLS certificates)

## Installation Steps

### Option 1: Quick Install (Recommended)

Install the latest stable release with a single command:

```bash
kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/latest/download/install.yaml
```

Or install a specific version:

```bash
VERSION=v0.1.0
kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/${VERSION}/install.yaml
```

### Option 2: Build from Source

For development or customization:

```bash
# Clone the repository
git clone https://github.com/mhenriks/kubevirt-rbac-webhook
cd kubevirt-rbac-webhook

# Build the webhook image
make docker-build IMG=<your-registry>/kubevirt-rbac-webhook:latest

# Push to your container registry
make docker-push IMG=<your-registry>/kubevirt-rbac-webhook:latest

# Deploy everything
make deploy IMG=<your-registry>/kubevirt-rbac-webhook:latest
```

Replace `<your-registry>` with your container registry (e.g., `docker.io/myuser`, `quay.io/myorg`, `ghcr.io/myuser`).

### 1. Install cert-manager (if not already installed)

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

Wait for cert-manager to be ready:

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=60s
```

### 2. Verify Installation

After installing the webhook, verify everything was created:

```bash
# Check webhook pod
kubectl get pods -n kubevirt-rbac-webhook-system

# Check ClusterRoles
kubectl get clusterroles | grep kubevirt.io:vm-

# Check ValidatingWebhookConfiguration
kubectl get validatingwebhookconfigurations | grep kubevirt-rbac
```

Expected ClusterRoles:
```
kubevirt.io:vm-full-admin           (aggregated - all permissions)
kubevirt.io:vm-storage-admin        (storage management)
kubevirt.io:vm-network-admin        (network management)
kubevirt.io:vm-compute-admin        (CPU/memory management)
kubevirt.io:vm-devices-admin        (device management)
kubevirt.io:vm-lifecycle-admin      (start/stop/restart)
kubevirt.io:vm-cdrom-user           (CD-ROM media only)
```

The installation includes:
- ✅ ValidatingWebhookConfiguration
- ✅ Fine-grained ClusterRoles with role aggregation
- ✅ Webhook Deployment and Service
- ✅ Required RBAC (minimal - just SubjectAccessReview permission)
- ✅ Certificate resources for webhook TLS (via cert-manager)

### 3. Webhook Certificate Details

The webhook requires TLS certificates, which are automatically managed by **cert-manager**.

The cert-manager annotation on the ValidatingWebhookConfiguration:
```yaml
cert-manager.io/inject-ca-from: kubevirt-rbac-webhook-system/webhook-cert
```

This tells cert-manager to automatically inject the CA bundle. Wait a few seconds after deployment for the CA to be injected.

#### Verifying Certificate Injection

```bash
# Check if CA bundle was injected
kubectl get validatingwebhookconfigurations kubevirt-rbac-validating-webhook \
  -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | base64 -d | openssl x509 -noout -subject

# If empty, wait a bit longer or check cert-manager logs
kubectl logs -n cert-manager -l app=webhook
```

## Testing

### Create a Test VM

```bash
cat <<EOF | kubectl apply -f -
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: test-vm
  namespace: default
spec:
  running: false
  template:
    metadata:
      labels:
        kubevirt.io/vm: test-vm
    spec:
      domain:
        devices:
          disks:
          - name: containerdisk
            disk:
              bus: virtio
        resources:
          requests:
            memory: 64M
      volumes:
      - name: containerdisk
        containerDisk:
          image: kubevirt/cirros-container-disk-demo
EOF
```

### Understanding Full-Admin vs Granular Permissions

**`vm-full-admin`** grants **unrestricted access to ALL VirtualMachine fields** (entire spec and metadata), not just the fields managed by granular roles. This is the most powerful permission.

**Important:** Users with Kubernetes built-in `admin` or `edit` roles automatically get `vm-full-admin` through role aggregation, giving them complete VM control.

To grant full VM admin access:
```bash
# Option 1: Grant vm-full-admin directly
kubectl create rolebinding bob-vm-admin \
  --clusterrole=kubevirt.io:vm-full-admin \
  --user=bob \
  --namespace=default

# Option 2: Grant Kubernetes admin role (includes vm-full-admin via aggregation)
kubectl create rolebinding bob-admin \
  --clusterrole=admin \
  --user=bob \
  --namespace=default
```

For more restricted access, use the granular roles below.

### Test Storage-Admin Permission

1. Create a test user with storage-admin permission:

```bash
# Create a RoleBinding for alice
kubectl create rolebinding alice-vm-storage-admin \
  --clusterrole=kubevirt.io:vm-storage-admin \
  --user=alice \
  --namespace=default
```

2. Try to modify storage (should succeed):

```bash
# As alice, add a hotpluggable disk and DataVolume
kubectl --as=alice patch vm test-vm --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/volumes/-",
    "value": {
      "name": "extra-disk",
      "dataVolume": {
        "name": "extra-disk-dv",
        "hotpluggable": true
      }
    }
  },
  {
    "op": "add",
    "path": "/spec/template/spec/domain/devices/disks/-",
    "value": {
      "name": "extra-disk",
      "disk": {
        "bus": "virtio"
      }
    }
  }
]'
```

This adds both:
- **Volume**: A DataVolume reference with `hotpluggable: true` (allows runtime addition/removal)
- **Disk**: How it's attached to the VM (virtio bus)

Note: The DataVolume `extra-disk-dv` should be created separately or exist in the namespace.

3. Try to modify CPU (should fail):

```bash
# As alice, try to change memory
kubectl --as=alice patch vm test-vm --type='json' -p='[
  {
    "op": "replace",
    "path": "/spec/template/spec/domain/resources/requests/memory",
    "value": "128M"
  }
]'
```

Expected error:
```
Error from server: user does not have permission to modify one or more VirtualMachine spec fields
```

### Test Backwards Compatibility

```bash
# User with standard edit role (no subresource permission)
kubectl create rolebinding bob-vm-editor \
  --clusterrole=edit \
  --user=bob \
  --namespace=default

# As bob, all modifications should succeed (backwards compatible)
kubectl --as=bob patch vm test-vm --type='json' -p='[
  {
    "op": "replace",
    "path": "/spec/template/spec/domain/resources/requests/memory",
    "value": "256M"
  }
]'

# This works! Bob has standard RBAC permissions, so all changes are allowed
```

### Test Per-VM Permissions

```bash
# Create a second VM
kubectl apply -f - <<EOF
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: prod-vm
  namespace: default
spec:
  running: false
  template:
    metadata:
      labels:
        kubevirt.io/vm: prod-vm
    spec:
      domain:
        devices:
          disks:
          - name: containerdisk
            disk:
              bus: virtio
        resources:
          requests:
            memory: 64M
      volumes:
      - name: containerdisk
        containerDisk:
          image: kubevirt/cirros-container-disk-demo
EOF

# Grant storage-admin ONLY for test-vm
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: carol-specific-vm-storage
  namespace: default
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines", "virtualmachines/storage-admin"]
  resourceNames: ["test-vm"]  # Only test-vm!
  verbs: ["get", "list", "watch", "update", "patch"]
EOF

kubectl create rolebinding carol-specific-vm \
  --role=carol-specific-vm-storage \
  --user=carol \
  --namespace=default

# Carol can modify test-vm storage
kubectl --as=carol patch vm test-vm --type='json' -p='[...]'  # Works!

# Carol CANNOT modify prod-vm (no permission for that specific VM)
kubectl --as=carol patch vm prod-vm --type='json' -p='[...]'  # Fails!
```

## Troubleshooting

### Check Webhook Logs

```bash
kubectl logs -n kubevirt-rbac-webhook-system \
  -l control-plane=controller-manager \
  -f
```

### Check Webhook Configuration

```bash
kubectl get validatingwebhookconfigurations
kubectl describe validatingwebhookconfiguration kubevirt-rbac-validating-webhook
```

### Disable Webhook for Testing

If you need to test without the webhook:

```bash
kubectl set env deployment/kubevirt-rbac-webhook-controller-manager \
  ENABLE_WEBHOOKS=false \
  -n kubevirt-rbac-webhook-system
```

### Common Issues

**Issue**: Webhook certificate errors

**Solution**: Ensure cert-manager is installed and the Certificate resources are created:
```bash
kubectl get certificates -n kubevirt-rbac-webhook-system
```

**Issue**: Permission denied but user has correct RoleBinding

**Solution**: Remember the opt-in model - if user has NO subresource permissions, they can do everything (backwards compatible). Only users WITH subresource permissions are restricted.

**Issue**: Changes not being blocked

**Solution**: Ensure the webhook is running:
```bash
kubectl get pods -n kubevirt-rbac-webhook-system
kubectl logs -n kubevirt-rbac-webhook-system -l control-plane=controller-manager
```

## Cleanup

### If installed from GitHub release:

```bash
kubectl delete -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/latest/download/install.yaml
```

Or for a specific version:

```bash
VERSION=v0.1.0
kubectl delete -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/${VERSION}/install.yaml
```

### If installed from source:

```bash
make undeploy
```

This removes:
- Webhook Deployment
- ValidatingWebhookConfiguration
- All ClusterRoles
- Service, RBAC, Certificates, etc.

## Next Steps

- Review [README.md](README.md) for architecture details
- Add more fine-grained ClusterRoles (see [EXTENDING.md](EXTENDING.md))
- Integrate with your existing RBAC policies
- Set up monitoring and alerts

## Tips

### Development Workflow

```bash
# Run webhook locally (requires kubeconfig)
make run

# Build, push, and deploy
make docker-build docker-push deploy IMG=your-registry/kubevirt-rbac-webhook:tag
```

### Adding New ClusterRoles

Simply add a new YAML file in `config/clusterroles/`:

```bash
cat > config/clusterroles/vm-network-admin.yaml <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubevirt.io:vm-network-admin
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines", "virtualmachines/network-admin"]
  verbs: ["get", "list", "watch", "update", "patch"]
EOF

# Add to kustomization
echo "- vm-network-admin.yaml" >> config/clusterroles/kustomization.yaml

# Redeploy (rebuild and push first if code changed)
make deploy IMG=your-registry/kubevirt-rbac-webhook:tag
```

### Testing Locally

```bash
# Set up a kind cluster
kind create cluster

# Install KubeVirt
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/v1.2.0/kubevirt-operator.yaml
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/v1.2.0/kubevirt-cr.yaml

# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Build, push, and deploy webhook
make docker-build docker-push deploy IMG=your-registry/kubevirt-rbac-webhook:latest
```
