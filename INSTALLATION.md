# Installation Guide

This guide covers all methods for installing the KubeVirt RBAC Webhook.

## Prerequisites

All installation methods require:
- Kubernetes cluster v1.28+
- KubeVirt installed
- `kubectl` configured with cluster access
- **cert-manager** (required for webhook TLS certificates)

## Installation Methods

### Method 1: Quick Install from Release (Recommended)

**Best for**: Production deployments, quick testing

Install the latest stable release:

```bash
kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/latest/download/install.yaml
```

Or install a specific version:

```bash
VERSION=v0.1.0
kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/${VERSION}/install.yaml
```

**Advantages:**
- ✅ Single command installation
- ✅ No build tools required
- ✅ Pre-built multi-platform images (amd64, arm64, s390x, ppc64le)
- ✅ Tested and versioned releases
- ✅ Easy rollback to specific versions

### Method 2: Install from Source

**Best for**: Development, customization, contributing

#### Prerequisites (additional)
- Go 1.24+
- Docker or Podman
- Container registry access
- make

#### Steps

1. **Clone the repository**:

```bash
git clone https://github.com/mhenriks/kubevirt-rbac-webhook
cd kubevirt-rbac-webhook
```

2. **Build and push the container image**:

```bash
# Build for your platform
make docker-build IMG=<your-registry>/kubevirt-rbac-webhook:latest

# Push to your registry
make docker-push IMG=<your-registry>/kubevirt-rbac-webhook:latest
```

3. **Deploy**:

```bash
make deploy IMG=<your-registry>/kubevirt-rbac-webhook:latest
```

**Advantages:**
- ✅ Full control over build process
- ✅ Test local changes immediately
- ✅ Customize for your environment

### Method 3: Install Using Kustomize

**Best for**: GitOps workflows, advanced customization

1. **Create a kustomization.yaml**:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: kubevirt-rbac-webhook-system

resources:
  - https://github.com/mhenriks/kubevirt-rbac-webhook/config/default?ref=v0.1.0

images:
  - name: ghcr.io/mhenriks/kubevirt-rbac-webhook
    newTag: v0.1.0
```

2. **Apply**:

```bash
kubectl apply -k .
```

**Advantages:**
- ✅ GitOps-friendly
- ✅ Easy to customize (patches, overlays)
- ✅ Version-controlled configuration

## Installing cert-manager

All methods require cert-manager for webhook TLS certificates.

### Quick Install

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

### Wait for cert-manager to be ready

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=60s
```

### Verify cert-manager

```bash
kubectl get pods -n cert-manager
```

## Post-Installation Verification

After installation with any method:

### 1. Check webhook pod is running

```bash
kubectl get pods -n kubevirt-rbac-webhook-system
```

Expected output:
```
NAME                                                 READY   STATUS    RESTARTS   AGE
kubevirt-rbac-webhook-controller-manager-xxxxx-xxx   1/1     Running   0          1m
```

### 2. Verify ClusterRoles were created

```bash
kubectl get clusterroles | grep kubevirt.io:vm-
```

Expected output:
```
kubevirt.io:vm-cdrom-user
kubevirt.io:vm-compute-admin
kubevirt.io:vm-devices-admin
kubevirt.io:vm-full-admin
kubevirt.io:vm-lifecycle-admin
kubevirt.io:vm-network-admin
kubevirt.io:vm-storage-admin
```

### 3. Check ValidatingWebhookConfiguration

```bash
kubectl get validatingwebhookconfigurations | grep kubevirt-rbac
```

Expected output:
```
kubevirt-rbac-validating-webhook
```

### 4. Verify webhook certificate

```bash
# Check if CA bundle was injected by cert-manager
kubectl get validatingwebhookconfigurations kubevirt-rbac-validating-webhook \
  -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | base64 -d | openssl x509 -noout -subject
```

If this shows a certificate subject, cert-manager is working correctly.

### 5. Check webhook logs

```bash
kubectl logs -n kubevirt-rbac-webhook-system \
  -l control-plane=controller-manager \
  --tail=50
```

Look for messages indicating the webhook server started successfully.

## What Gets Installed

The installation creates the following resources:

### Namespace
- `kubevirt-rbac-webhook-system` - Isolated namespace for webhook components

### ClusterRoles (Fine-grained RBAC)
- `kubevirt.io:vm-full-admin` - Unrestricted VM access (aggregated)
- `kubevirt.io:vm-storage-admin` - Storage management
- `kubevirt.io:vm-network-admin` - Network configuration
- `kubevirt.io:vm-compute-admin` - CPU/memory management
- `kubevirt.io:vm-devices-admin` - Device management
- `kubevirt.io:vm-lifecycle-admin` - Start/stop/restart
- `kubevirt.io:vm-cdrom-user` - CD-ROM media only

### Webhook Components
- `Deployment` - Webhook server with health checks
- `Service` - Webhook service endpoint
- `ValidatingWebhookConfiguration` - Intercepts VM updates

### RBAC
- `ServiceAccount` - For webhook pod identity
- `ClusterRole` - Minimal permissions (SubjectAccessReview)
- `ClusterRoleBinding` - Binds role to service account

### Certificates (cert-manager)
- `Issuer` - Self-signed certificate issuer
- `Certificate` - Webhook TLS certificate
- `Certificate` - Metrics TLS certificate (if enabled)

### Metrics (if enabled)
- `Service` - Metrics endpoint
- `ServiceMonitor` - Prometheus integration (if enabled)

## Configuration Options

### Environment Variables

The webhook deployment supports these environment variables:

```yaml
env:
  - name: ENABLE_WEBHOOKS
    value: "true"  # Set to "false" to disable webhooks
```

### Image Configuration

To use a different container image:

**Using kubectl patch:**
```bash
kubectl set image deployment/kubevirt-rbac-webhook-controller-manager \
  manager=your-registry/kubevirt-rbac-webhook:your-tag \
  -n kubevirt-rbac-webhook-system
```

**Using kustomize:**
```yaml
images:
  - name: ghcr.io/mhenriks/kubevirt-rbac-webhook
    newName: your-registry/kubevirt-rbac-webhook
    newTag: your-tag
```

## Upgrading

### From GitHub Release

To upgrade to a newer version:

```bash
# Install new version (replaces existing)
kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/v0.2.0/install.yaml
```

The installation is idempotent - applying a new version will update existing resources.

### Rolling Back

To rollback to a previous version:

```bash
VERSION=v0.1.0
kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/${VERSION}/install.yaml
```

### Checking Current Version

```bash
kubectl get deployment kubevirt-rbac-webhook-controller-manager \
  -n kubevirt-rbac-webhook-system \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Uninstalling

### If installed from GitHub release

```bash
kubectl delete -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/latest/download/install.yaml
```

Or for a specific version:

```bash
VERSION=v0.1.0
kubectl delete -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/${VERSION}/install.yaml
```

### If installed from source

```bash
make undeploy
```

### Verify uninstallation

```bash
# Check namespace is gone
kubectl get namespace kubevirt-rbac-webhook-system

# Check ClusterRoles are removed
kubectl get clusterroles | grep kubevirt.io:vm-

# Check webhook configuration is removed
kubectl get validatingwebhookconfigurations | grep kubevirt-rbac
```

All should return "not found" or empty results.

## Troubleshooting

### Webhook pod not starting

**Check pod status:**
```bash
kubectl describe pod -n kubevirt-rbac-webhook-system -l control-plane=controller-manager
```

**Common causes:**
- Image pull errors (check registry access)
- Resource constraints (check node resources)
- RBAC issues (check ServiceAccount permissions)

### Certificate errors

**Check cert-manager:**
```bash
kubectl get certificates -n kubevirt-rbac-webhook-system
kubectl describe certificate webhook-cert -n kubevirt-rbac-webhook-system
```

**Common causes:**
- cert-manager not installed
- cert-manager webhook not ready
- CA bundle not injected (check ValidatingWebhookConfiguration)

### Webhook not blocking unauthorized changes

**Check webhook configuration:**
```bash
kubectl get validatingwebhookconfigurations kubevirt-rbac-validating-webhook -o yaml
```

**Verify:**
- Webhook service is reachable
- CA bundle is present
- failurePolicy is not set to "Ignore"

**Check webhook logs:**
```bash
kubectl logs -n kubevirt-rbac-webhook-system \
  -l control-plane=controller-manager \
  -f
```

### Permission denied errors

Remember the **opt-in security model**:
- Users with standard `update virtualmachines` permission → Unrestricted (backwards compatible)
- Users with `virtualmachines/<subresource>` permissions → Restricted to those subresources

**Check user permissions:**
```bash
# Check what permissions a user has
kubectl auth can-i update virtualmachines --as=alice -n default
kubectl auth can-i update virtualmachines/storage-admin --as=alice -n default
```

## Advanced Configuration

### Custom ClusterRole Aggregation

The `vm-full-admin` role uses Kubernetes role aggregation. To include your custom permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-custom-vm-permissions
  labels:
    rbac.kubevirt.io/aggregate-to-vm-full-admin: "true"
rules:
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines/custom"]
    verbs: ["update"]
```

This role will automatically be aggregated into `vm-full-admin`.

### Network Policies

To enable network policies (restrict webhook access):

1. Uncomment in `config/default/kustomization.yaml`:
```yaml
resources:
  - ../network-policy
```

2. Label namespaces that should access the webhook:
```bash
kubectl label namespace my-app-namespace webhooks=enabled
```

### Prometheus Metrics

To enable Prometheus metrics:

1. Uncomment in `config/default/kustomization.yaml`:
```yaml
resources:
  - ../prometheus
```

2. Apply the configuration

3. Access metrics:
```bash
kubectl port-forward -n kubevirt-rbac-webhook-system \
  svc/kubevirt-rbac-webhook-controller-manager-metrics-service 8443:8443

curl -k https://localhost:8443/metrics
```

## Security Considerations

### Image Security

- Official images are signed and scanned
- Multi-platform images support diverse architectures
- Images use distroless base (minimal attack surface)
- Non-root user (UID 65532)

### RBAC Permissions

The webhook requires minimal permissions:
- `create` on `subjectaccessreviews` (check user permissions)
- Standard controller permissions for its own resources

### Network Security

- Webhook uses TLS (managed by cert-manager)
- Optional network policies for defense-in-depth
- Metrics can be protected with TLS

### Admission Control

- Webhook uses fail-closed by default (blocks on errors)
- Comprehensive validation prevents privilege escalation
- Backwards-compatible opt-in model

## Getting Help

- **Documentation**: [README.md](README.md), [QUICKSTART.md](QUICKSTART.md)
- **Issues**: https://github.com/mhenriks/kubevirt-rbac-webhook/issues
- **Development**: [EXTENDING.md](EXTENDING.md)
- **Releases**: [RELEASE.md](RELEASE.md)

## Next Steps

After installation:
1. **Read the Quick Start**: [QUICKSTART.md](QUICKSTART.md)
2. **Grant permissions**: Create RoleBindings for users
3. **Test webhook**: Verify RBAC enforcement works
4. **Monitor**: Set up logging and metrics
5. **Integrate**: Connect with your existing RBAC policies
