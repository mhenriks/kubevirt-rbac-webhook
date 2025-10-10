# Project Summary

## Overview

This project implements a Kubernetes validating admission webhook for fine-grained RBAC control over KubeVirt VirtualMachine resources. It provides administrators with the ability to grant specific permissions (e.g., storage management) without giving full VM edit access, using a backwards-compatible opt-in model.

## What Was Built

### 1. Validating Admission Webhook

**VirtualMachineCustomValidator** (with Dependency Injection)
- Location: `internal/webhook/v1/virtualmachine_webhook.go`
- Purpose: Intercepts VirtualMachine update requests
- Architecture: Uses injected dependencies for high testability (89.8% coverage)
- Logic Flow (Opt-in Restrictions Model):
  1. Extract user information from admission request
  2. Check if user has `virtualmachines/full-admin` → allow if true (aggregated role with all permissions)
  3. Check if user has ANY granular subresource permissions (e.g., `virtualmachines/storage-admin`, `virtualmachines/lifecycle-admin`)
  4. If NO subresource permissions → allow (backwards compatible)
  5. If YES → validate specific changes against permissions:
     - Storage changes → require `virtualmachines/storage-admin` permission
     - Network changes → require `virtualmachines/network-admin` permission
     - Compute changes → require `virtualmachines/compute-admin` permission
     - Device changes → require `virtualmachines/devices-admin` permission
     - Lifecycle changes → require `virtualmachines/lifecycle-admin` permission
     - CD-ROM media changes → require `virtualmachines/cdrom-user` permission
  6. Use SubjectAccessReview API for permission checks (including VM name for per-resource policies)
  7. Allow or deny the request

**PermissionChecker Interface** (for testability)
- `SubjectAccessReviewPermissionChecker` - Production implementation using K8s API
- `MockPermissionChecker` - Test implementation for unit tests

### 2. Field Permission Checkers

**FieldPermissionChecker Interface**
- Location: `internal/webhook/v1/field_permission_checkers.go`
- Purpose: Extensible pattern for checking field-specific permissions
- Current implementations:
  - `StoragePermissionChecker` - All volumes, disks, CD-ROMs, and filesystems (superset)
  - `NetworkPermissionChecker` - Network interfaces and networks
  - `ComputePermissionChecker` - CPU and memory resources
  - `DevicesPermissionChecker` - GPUs, host devices, watchdog, TPM, inputs
  - `LifecyclePermissionChecker` - Running state and runStrategy
  - `CdromUserPermissionChecker` - Hotpluggable CD-ROM media only (subset)

### 3. Fine-Grained ClusterRoles with Aggregation

ClusterRole manifests in `config/clusterroles/` using Kubernetes role aggregation:

**`kubevirt.io:vm-full-admin`** (Aggregated)
- Allows: **Unrestricted access to all VirtualMachine spec and metadata fields**
- Uses Kubernetes role aggregation to automatically include all roles labeled `rbac.kubevirt.io/aggregate-to-vm-full-admin: "true"`
- Aggregates into Kubernetes built-in `admin` and `edit` ClusterRoles
- Use case: Full VM administrators who need complete control over all VM fields

**`kubevirt.io:vm-storage-admin`** (Superset)
- Allows: Viewing and modifying all VM storage (volumes, disks, CD-ROMs, filesystems)
- Aggregates into: vm-full-admin

**`kubevirt.io:vm-network-admin`**
- Allows: Modifying VM network configuration (interfaces, networks)
- Aggregates into: vm-full-admin

**`kubevirt.io:vm-compute-admin`**
- Allows: Modifying VM compute resources (CPU, memory)
- Aggregates into: vm-full-admin

**`kubevirt.io:vm-devices-admin`**
- Allows: Modifying VM devices (GPUs, host devices, watchdog, TPM, inputs)
- Aggregates into: vm-full-admin

**`kubevirt.io:vm-lifecycle-admin`**
- Allows: Controlling VM lifecycle (start, stop, restart, runStrategy)
- Aggregates into: vm-full-admin

**`kubevirt.io:vm-cdrom-user`** (Subset of storage-admin)
- Allows: Inject/eject/swap CD-ROM media only in existing CD-ROM drives (hotpluggable only)
- Aggregates into: vm-full-admin
- Opts users into restricted permissions (users without this role retain full access via standard RBAC)

## Architecture Highlights

### Simple Webhook Pattern

The webhook follows a standard Kubernetes admission webhook pattern:
```
make docker-build docker-push deploy IMG=your-registry/image:tag
    ↓
Builds image, pushes to registry, and creates:
  - Webhook Deployment (using your built image)
  - ValidatingWebhookConfiguration
  - ClusterRoles
  - Service, RBAC, Certificates
    ↓
Webhook enforces permissions immediately
```

**No controller, No CRD, No reconciliation** - Just a simple stateless webhook!

### Adding New ClusterRoles

To add a new ClusterRole, simply create a new YAML file in `config/clusterroles/`:

```yaml
# config/clusterroles/vm-network-admin.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubevirt.io:vm-network-admin
  labels:
    app.kubernetes.io/managed-by: kubevirt-rbac-webhook
rules:
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines/network-admin"]
    verbs: ["update"]
```

Then add it to `config/clusterroles/kustomization.yaml`:
```yaml
resources:
- vm-storage-admin.yaml
- vm-network-admin.yaml  # Add this
```

### Permission Enforcement Pattern (Opt-in Restrictions)

```
User Update Request
        ↓
ValidatingWebhook
        ↓
Has "update * in kubevirt.io"? → YES → Allow All Changes (backwards compatible)
        ↓ NO
        ↓
Has ANY subresource permissions? → NO → Allow All Changes (backwards compatible)
        ↓ YES (opted-in)
        ↓
Storage Changed?
        ↓
    YES → Has storage-admin? → YES → Allow (neutralize storage fields)
        ↓                      ↓ NO
        ↓                      DENY
        ↓
     NO → Continue checking other fields
        ↓
Any unauthorized changes remain? → YES → DENY
        ↓ NO
     Allow
```

**Security Model (Backwards Compatible):**
- ✅ Standard `update virtualmachines`: Can modify anything (backwards compatible)
- ✅ `storage-admin` subresource: Can ONLY modify storage (opted-in to restrictions)
- ❌ `storage-admin` + non-storage changes: Denied
- ❌ No RBAC permissions: Cannot modify VMs (standard Kubernetes RBAC)

### SubjectAccessReview Integration

The webhook uses Kubernetes' built-in SubjectAccessReview API to check permissions dynamically:

```go
sar := &authv1.SubjectAccessReview{
    Spec: authv1.SubjectAccessReviewSpec{
        User:   userInfo.Username,
        Groups: userInfo.Groups,
        ResourceAttributes: &authv1.ResourceAttributes{
            Namespace: namespace,
            Verb:      "update",
            Resource:  "virtualmachines/storage-admin",
            Name:      name,  // Enables per-VM RBAC policies
        },
    },
}
```

**Key Feature:** The webhook passes the specific VM name in the permission check, enabling resource-name-specific RBAC policies using `resourceNames` in Roles/ClusterRoles. This allows administrators to grant permissions like "alice can manage storage on test-vm but not prod-vm".

This ensures consistency with the cluster's RBAC configuration.

## File Structure

```
.
├── config/
│   ├── webhook/              # Webhook Deployment, Service, ValidatingWebhookConfiguration
│   ├── rbac/                 # Webhook's own RBAC (minimal - just SubjectAccessReview)
│   ├── clusterroles/         # Fine-grained ClusterRoles we deploy
│   │   └── vm-storage-admin.yaml
│   ├── certmanager/          # Certificate for webhook TLS
│   └── default/              # Kustomization to deploy everything
├── cmd/
│   └── main.go               # Webhook server (no controller)
└── internal/
    └── webhook/v1/
        ├── virtualmachine_webhook.go
        └── field_permission_checkers.go
```

## Key Implementation Details

### 1. Change Detection with Field Permission Checkers

The webhook uses an extensible pattern for field-specific permission checking:

```go
type FieldPermissionChecker interface {
    Name() string
    Subresource() string
    HasChanged(oldVM, newVM *VirtualMachine) bool
    Neutralize(oldVM, newVM *VirtualMachine)
}
```

Each checker:
1. Detects if its fields have changed
2. Returns the subresource name to check permissions against
3. Can "neutralize" authorized changes (set them equal in copies)

### 2. Permission Optimization

The webhook checks `virtualmachines/full-admin` permission first, then checks for ANY granular subresource permissions. This avoids unnecessary detailed permission checks for users with full admin access or standard RBAC (backwards compatible).

### 3. Stateless Webhook

No persistent state, no reconciliation loops, no finalizers. Just pure request validation.

## Current Capabilities

✅ **Implemented:**
- Fine-grained storage permission (`vm-storage-admin` for all storage including CD-ROMs - superset)
- Fine-grained CD-ROM permission (`vm-cdrom-user` for hotpluggable CD-ROM media only - subset)
- Backwards-compatible opt-in restrictions model
- Dynamic permission checking via SubjectAccessReview (with VM name)
- Webhook validation for update operations with 88.8% test coverage
- Extensible FieldPermissionChecker pattern
- Dependency injection architecture for high testability
- Hierarchical permission model (storage-admin ⊃ cdrom-user)

🔄 **Future Extensions:**
- Network management permission (`vm-network-admin`)
- CPU/Memory management permission (`vm-resources-admin`)
- Device management permission (`vm-device-admin`)
- Additional field-specific permissions

## Testing Strategy

### Unit Tests (88.8% coverage)
- Webhook validation logic (ValidateUpdate: 90.7% coverage)
- Field permission checker implementations (85-100% coverage)
- Metadata normalization (100% coverage)
- Mock-based testing using dependency injection
- 34 comprehensive test cases

### Integration Tests
- Full webhook validation flow
- SubjectAccessReview integration
- Permission enforcement

### Manual Testing
```bash
# Test storage-admin (opt-in to restrictions)
kubectl create rolebinding test --clusterrole=kubevirt.io:vm-storage-admin --user=alice
# Modify volumes (should succeed)
# Modify CPU (should fail - restricted)

# Test backwards compatibility
# User with standard update permissions but NO subresource permissions
# All modifications should succeed (backwards compatible)
```

## Configuration Options

### Environment Variables
- `ENABLE_WEBHOOKS`: Enable/disable webhook (default: `true`)

### Deployment Options
- Standard deployment with cert-manager (recommended)
- Manual certificate management
- Development mode with local webhook

## Dependencies

- **Kubernetes**: 1.28+
- **KubeVirt API**: v1.6.2
- **Controller Runtime**: v0.21.0 (webhook server only)
- **cert-manager**: For webhook TLS certificates

## Minimal RBAC Requirements

The webhook requires minimal permissions:
- `SubjectAccessReview` permissions: create

That's it! No ClusterRole management, no CRD permissions, no finalizers.

## Deployment Size

- **Container image**: ~50MB (estimated)
- **Memory**: ~50Mi (typical)
- **CPU**: ~50m (typical)
- **Replicas**: 1 (can scale for HA)

## Security Considerations

1. **Webhook Security**: Uses TLS with cert-manager or manual certs
2. **Permission Checks**: Uses official Kubernetes SubjectAccessReview API
3. **RBAC Isolation**: Webhook runs with minimal required permissions
4. **Audit Trail**: All permission denials are logged
5. **Per-VM Policies**: Supports resource-name-specific RBAC

## Extension Pattern

Adding new permissions follows a simple pattern:

1. Create ClusterRole YAML in `config/clusterroles/`
2. Implement FieldPermissionChecker for the new fields
3. Add checker to webhook's checker list
4. Build, push, and deploy with `make docker-build docker-push deploy IMG=your-registry/image:tag`

See `EXTENDING.md` for detailed guide.

## Known Limitations

1. **Webhook Startup**: Requires certificates before accepting requests
2. **Change Detection**: Uses simple DeepEqual, may not catch all semantic changes
3. **Permission Granularity**: Currently only storage; more aspects coming
4. **Single Point**: Webhook is a single point of policy enforcement (can scale for HA)

## Future Roadmap

### Phase 1 (Current)
- ✅ Storage admin permission (all storage including CD-ROMs - superset)
- ✅ CD-ROM user permission (hotpluggable media only - subset)
- ✅ Hierarchical permission model (storage-admin ⊃ cdrom-user)
- ✅ Opt-in restrictions model (backwards compatible)
- ✅ Webhook validation with SubjectAccessReview
- ✅ Per-VM RBAC support
- ✅ Dependency injection architecture
- ✅ 88.8% test coverage

### Phase 2 (Next)
- [ ] Network admin permission
- [ ] CPU/Memory admin permission
- [ ] Device admin permission
- [ ] Enhanced change detection

### Phase 3 (Future)
- [ ] Admission policy metrics
- [ ] Policy audit logging
- [ ] High availability mode
- [ ] Multi-tenant support

## Contributing

Contributions welcome! See `EXTENDING.md` for development guide.

Key areas for contribution:
- Additional fine-grained permissions
- Improved change detection algorithms
- Enhanced test coverage
- Documentation improvements

## License

Apache License 2.0 - See LICENSE file for details.

## Contact

- Issues: GitHub Issues
- Discussions: GitHub Discussions
- Documentation: See README.md, QUICKSTART.md, EXTENDING.md
