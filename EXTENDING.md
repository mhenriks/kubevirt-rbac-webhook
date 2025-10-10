# Extending the Webhook

This guide explains how to add new fine-grained permissions to the webhook.

## Adding a New Permission Type

Let's walk through adding a new permission type: `vm-network-admin` for managing VM network interfaces.

### Step 1: Define the ClusterRole

Create `config/clusterroles/vm-network-admin.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubevirt.io:vm-network-admin
  labels:
    app.kubernetes.io/managed-by: kubevirt-rbac-webhook
rules:
  - apiGroups:
      - kubevirt.io
    resources:
      - virtualmachines
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - kubevirt.io
    resources:
      - virtualmachines/network-admin
    verbs:
      - update
```

Add it to `config/clusterroles/kustomization.yaml`:

```yaml
resources:
- vm-storage-admin.yaml
- vm-cdrom-user.yaml
- vm-network-admin.yaml  # Add this line
```

### Step 2: Implement FieldPermissionChecker

Add the checker in `internal/webhook/v1/field_permission_checkers.go`:

```go
// NetworkPermissionChecker implements FieldPermissionChecker for network-related fields.
type NetworkPermissionChecker struct{}

var _ FieldPermissionChecker = &NetworkPermissionChecker{}

func (n *NetworkPermissionChecker) Name() string {
    return "network"
}

func (n *NetworkPermissionChecker) Subresource() string {
    return "virtualmachines/network-admin"
}

func (n *NetworkPermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    oldInterfaces := oldVM.Spec.Template.Spec.Domain.Devices.Interfaces
    newInterfaces := newVM.Spec.Template.Spec.Domain.Devices.Interfaces
    interfacesChanged := !reflect.DeepEqual(oldInterfaces, newInterfaces)

    oldNetworks := oldVM.Spec.Template.Spec.Networks
    newNetworks := newVM.Spec.Template.Spec.Networks
    networksChanged := !reflect.DeepEqual(oldNetworks, newNetworks)

    return interfacesChanged || networksChanged
}

func (n *NetworkPermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
    if oldVM.Spec.Template != nil && newVM.Spec.Template != nil {
        // Neutralize network interfaces
        oldVM.Spec.Template.Spec.Domain.Devices.Interfaces = nil
        newVM.Spec.Template.Spec.Domain.Devices.Interfaces = nil

        // Neutralize networks
        oldVM.Spec.Template.Spec.Networks = nil
        newVM.Spec.Template.Spec.Networks = nil
    }
}
```

### Step 3: Register the Checker

In `internal/webhook/v1/virtualmachine_webhook.go`, add the checker to the setup:

```go
func SetupVirtualMachineWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kubevirtiov1.VirtualMachine{}).
		WithValidator(&VirtualMachineCustomValidator{
			Client: mgr.GetClient(),
			// IMPORTANT: Order matters for hierarchical permissions (subset before superset)
			FieldCheckers: []FieldPermissionChecker{
				// Independent permissions (no hierarchy, can be in any order)
				&NetworkPermissionChecker{},    // Your new checker

				// Hierarchical permissions (subset before superset)
				&CdromUserPermissionChecker{},  // Subset: CD-ROM media only
				&StoragePermissionChecker{},    // Superset: All storage (including CD-ROMs)
			},
			PermissionChecker: &SubjectAccessReviewPermissionChecker{
				Client: mgr.GetClient(),
			},
		}).
		Complete()
}
```

That's it! The webhook automatically handles the new checker:
1. Checks if user has `virtualmachines/network-admin` permission
2. If they do, checks if network fields changed
3. If changed and they have permission, neutralizes those fields
4. If changed and they lack permission, denies the request

**Important Notes:**
- **Order Matters!** Checkers are processed most-specific-first (subsets before supersets)
- This allows hierarchical permissions where a subset permission (e.g., cdrom-user) can neutralize changes before a superset permission (e.g., storage-admin) sees them
- The webhook checks `HasChanged` on progressively neutralized copies, not the originals
- The webhook uses dependency injection for testability. The `FieldCheckers` list is injected at setup time.

### Step 4: Deploy and Test

1. Build and deploy:
```bash
make manifests
make docker-build docker-push deploy IMG=<your-registry>/kubevirt-rbac-webhook:tag
```

2. Verify the ClusterRole was created:
```bash
kubectl get clusterrole kubevirt.io:vm-network-admin -o yaml
```

3. Test the permission:
```bash
# Grant network-admin to a user
kubectl create rolebinding user-network-admin \
  --clusterrole=kubevirt.io:vm-network-admin \
  --user=charlie \
  --namespace=default

# Try to modify network (should succeed)
kubectl --as=charlie patch vm test-vm --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/networks/-",
    "value": {
      "name": "secondary",
      "multus": {"networkName": "mynetwork"}
    }
  }
]'

# Try to modify storage (should fail - charlie only has network-admin)
kubectl --as=charlie patch vm test-vm --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/volumes/-",
    "value": {
      "name": "disk2",
      "persistentVolumeClaim": {"claimName": "pvc2"}
    }
  }
]'
```

## Change Detection Patterns

### Simple Field Comparison

For simple fields, use direct comparison:

```go
func hasMemoryChanges(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    oldMem := oldVM.Spec.Template.Spec.Domain.Resources.Requests.Memory()
    newMem := newVM.Spec.Template.Spec.Domain.Resources.Requests.Memory()
    return !oldMem.Equal(*newMem)
}
```

### List Comparison

For lists, use `equality.Semantic.DeepEqual` (preferred for Kubernetes resources):

```go
import "k8s.io/apimachinery/pkg/api/equality"

func hasDeviceChanges(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    oldDevices := oldVM.Spec.Template.Spec.Domain.Devices.GPUs
    newDevices := newVM.Spec.Template.Spec.Domain.Devices.GPUs
    return !equality.Semantic.DeepEqual(oldDevices, newDevices)
}
```

**Why Semantic Equality?** Kubernetes' semantic equality properly handles resource semantics and ignores irrelevant fields like ResourceVersion and Generation.

### Nested Object Comparison

For complex nested objects, consider JSON marshaling:

```go
func hasDomainChanges(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    oldJSON, _ := json.Marshal(oldVM.Spec.Template.Spec.Domain)
    newJSON, _ := json.Marshal(newVM.Spec.Template.Spec.Domain)
    return string(oldJSON) != string(newJSON)
}
```

## Testing Your Extension

### Unit Tests

Add tests in `internal/webhook/v1/field_permission_checkers_test.go`:

```go
var _ = Describe("NetworkPermissionChecker", func() {
    var checker *NetworkPermissionChecker

    BeforeEach(func() {
        checker = &NetworkPermissionChecker{}
    })

    Context("HasChanged", func() {
        It("should detect interface changes", func() {
            oldVM := &kubevirtiov1.VirtualMachine{
                Spec: kubevirtiov1.VirtualMachineSpec{
                    Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
                        Spec: kubevirtiov1.VirtualMachineInstanceSpec{
                            Domain: kubevirtiov1.DomainSpec{
                                Devices: kubevirtiov1.Devices{
                                    Interfaces: []kubevirtiov1.Interface{
                                        {Name: "default"},
                                    },
                                },
                            },
                        },
                    },
                },
            }

            newVM := oldVM.DeepCopy()
            newVM.Spec.Template.Spec.Domain.Devices.Interfaces = append(
                newVM.Spec.Template.Spec.Domain.Devices.Interfaces,
                kubevirtiov1.Interface{Name: "secondary"},
            )

            Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
        })

        It("should not detect changes when interfaces are the same", func() {
            oldVM := &kubevirtiov1.VirtualMachine{
                Spec: kubevirtiov1.VirtualMachineSpec{
                    Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
                        Spec: kubevirtiov1.VirtualMachineInstanceSpec{
                            Domain: kubevirtiov1.DomainSpec{
                                Devices: kubevirtiov1.Devices{
                                    Interfaces: []kubevirtiov1.Interface{
                                        {Name: "default"},
                                    },
                                },
                            },
                        },
                    },
                },
            }

            newVM := oldVM.DeepCopy()
            Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
        })
    })

    Context("Neutralize", func() {
        It("should neutralize interfaces", func() {
            oldVM := &kubevirtiov1.VirtualMachine{
                Spec: kubevirtiov1.VirtualMachineSpec{
                    Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
                        Spec: kubevirtiov1.VirtualMachineInstanceSpec{
                            Domain: kubevirtiov1.DomainSpec{
                                Devices: kubevirtiov1.Devices{
                                    Interfaces: []kubevirtiov1.Interface{
                                        {Name: "default"},
                                    },
                                },
                            },
                        },
                    },
                },
            }

            newVM := oldVM.DeepCopy()
            newVM.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtiov1.Interface{
                {Name: "default"},
                {Name: "secondary"},
            }

            checker.Neutralize(oldVM, newVM)

            Expect(oldVM.Spec.Template.Spec.Domain.Devices.Interfaces).To(BeNil())
            Expect(newVM.Spec.Template.Spec.Domain.Devices.Interfaces).To(BeNil())
        })
    })
})
```

## Best Practices

### 1. Granular Permissions

Keep permissions focused on specific aspects:
- ✅ `vm-cpu-admin` - CPU configuration only
- ❌ `vm-resources-admin` - Too broad (CPU, memory, devices)

### 2. Consistent Naming

Follow the pattern: `kubevirt.io:vm-<aspect>-admin` or `kubevirt.io:vm-<aspect>-user`

### 3. Permission Hierarchy

**Hierarchical Model:** Permissions can have superset/subset relationships:

```
storage-admin (superset)
    ├── Can modify ALL storage (volumes, disks, CD-ROMs)
    └── Contains cdrom-user as a subset

cdrom-user (subset)
    └── Can ONLY modify CD-ROM media (hotpluggable)
```

**Implementation:**
- Order checkers **most specific first** (subset before superset)
- Subset permissions neutralize their changes first
- Superset permissions then see only remaining changes
- Users with superset permissions can do everything subset permissions can, plus more

**Example:**
- User with `storage-admin`: Can modify all storage including CD-ROMs
- User with `cdrom-user`: Can only modify CD-ROM media
- User with `both`: Effectively same as just storage-admin (superset covers it)

**Design Guideline:** When creating related permissions, consider if they should be:
- **Mutually exclusive** (network vs storage) - no special ordering needed
- **Hierarchical** (storage ⊃ cdrom) - order subset before superset

Note: The opt-in model means users without subresource permissions retain full access.

### 4. Error Messages

Provide clear error messages:
```go
return nil, fmt.Errorf("user does not have permission to modify VM %s (requires %s)",
    checker.Name(), checker.Subresource())
```

### 5. Testing with Dependency Injection

The webhook uses dependency injection for testability. Always add comprehensive unit tests:

```go
// In your test file
var _ = Describe("NetworkPermissionChecker Tests", func() {
    var (
        validator *VirtualMachineCustomValidator
        mockPerm  *MockPermissionChecker
    )

    BeforeEach(func() {
        mockPerm = &MockPermissionChecker{
            permissions: make(map[string]bool),
        }
        validator = &VirtualMachineCustomValidator{
            FieldCheckers: []FieldPermissionChecker{
                &NetworkPermissionChecker{},
            },
            PermissionChecker: mockPerm,
        }
    })

    It("should allow network changes with network-admin permission", func() {
        mockPerm.permissions["virtualmachines/network-admin"] = true
        // ... test logic
    })
})
```

### 6. Performance

- Check `virtualmachines/full-admin` permission first (users with this aggregated role have unrestricted access to all spec/metadata fields)
- Check for ANY subresource permissions before detailed validation
- Only perform change detection if user has opted-in (has subresource permissions)
- Use efficient comparison methods

### 7. Backward Compatibility

When adding new checks:
- Don't break existing permissions
- Users without your new subresource permission should work as before
- Document changes in release notes

## Common Patterns

### Read-Only Access

For view-only permissions (though not typically used with this webhook):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubevirt.io:vm-viewer
rules:
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines"]
    verbs: ["get", "list", "watch"]
```

### Limited Write Access

For operations like start/stop/restart (already implemented as `vm-lifecycle-admin`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubevirt.io:vm-lifecycle-admin
  labels:
    app.kubernetes.io/managed-by: kubevirt-rbac-webhook
    rbac.kubevirt.io/aggregate-to-vm-full-admin: "true"
rules:
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["subresources.kubevirt.io"]
    resources: ["virtualmachines/start", "virtualmachines/stop", "virtualmachines/restart", "virtualmachineinstances/softreboot"]
    verbs: ["update"]
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines/lifecycle-admin"]
    verbs: ["update"]
```

With a checker that validates the `running` and `runStrategy` fields:

```go
type LifecyclePermissionChecker struct{}

func (l *LifecyclePermissionChecker) Name() string {
    return "lifecycle"
}

func (l *LifecyclePermissionChecker) Subresource() string {
    return "virtualmachines/lifecycle-admin"
}

func (l *LifecyclePermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    // Use DeepEqual to properly compare pointer values
    runningChanged := !equality.Semantic.DeepEqual(oldVM.Spec.Running, newVM.Spec.Running)
    runStrategyChanged := !equality.Semantic.DeepEqual(oldVM.Spec.RunStrategy, newVM.Spec.RunStrategy)
    return runningChanged || runStrategyChanged
}

func (l *LifecyclePermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
    // Set both to nil to neutralize (consistent with other checkers)
    oldVM.Spec.Running = nil
    newVM.Spec.Running = nil
    oldVM.Spec.RunStrategy = nil
    newVM.Spec.RunStrategy = nil
}
```

## Documentation Updates

When adding new permissions, update:

1. **README.md** - Add to ClusterRoles section
2. **QUICKSTART.md** - Add testing example
3. **This file (EXTENDING.md)** - Add as an example
4. **Release Notes** - Describe the new feature

## Review Checklist

Before submitting your extension:

- [ ] ClusterRole YAML created in `config/clusterroles/`
- [ ] ClusterRole added to `kustomization.yaml`
- [ ] FieldPermissionChecker implemented
- [ ] Checker added to webhook's checker list
- [ ] Unit tests added
- [ ] Integration tests added (if applicable)
- [ ] Documentation updated
- [ ] Error messages clear and helpful
- [ ] Backwards compatible (users without new permission unaffected)
- [ ] Performance tested

## Development Workflow

```bash
# 1. Make your changes
vim config/clusterroles/vm-network-admin.yaml
vim internal/webhook/v1/field_permission_checkers.go
vim internal/webhook/v1/virtualmachine_webhook.go

# 2. Run tests
make test

# 3. Build
make build

# 4. Test locally (requires running cluster)
make run

# 5. Build container
make docker-build IMG=your-registry/kubevirt-rbac-webhook:dev

# 6. Deploy to test cluster
make docker-build docker-push deploy IMG=your-registry/kubevirt-rbac-webhook:dev

# 7. Test manually
kubectl apply -f test-vm.yaml
kubectl create rolebinding test --clusterrole=kubevirt.io:vm-network-admin --user=testuser
kubectl --as=testuser patch vm test-vm --type='json' -p='[...]'
```

## Need Help?

- Review existing implementations in `internal/webhook/v1/`
- Check KubeVirt API documentation for available fields
- Open an issue for design discussions
- Submit a draft PR for early feedback

## Examples from the Community

### Example: CD-ROM User (Implemented)

The `vm-cdrom-user` permission allows users to inject/eject/swap CD-ROM media without modifying other storage:

```go
// CdromUserPermissionChecker handles CD-ROM media operations
type CdromUserPermissionChecker struct{}

func (c *CdromUserPermissionChecker) Name() string {
    return "cdrom"
}

func (c *CdromUserPermissionChecker) Subresource() string {
    return "virtualmachines/cdrom-user"
}

func (c *CdromUserPermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    // Check CD-ROM disk definitions are unchanged (users can't add/remove drives)
    oldCdromDisks := c.getCdromDisks(oldVM)
    newCdromDisks := c.getCdromDisks(newVM)
    if !equality.Semantic.DeepEqual(oldCdromDisks, newCdromDisks) {
        return false  // Disk definitions changed - not a cdrom-user operation
    }

    // Check if hotpluggable CD-ROM volumes changed (inject/eject/swap media)
    oldCdromVolumes := c.getHotpluggableCdromVolumes(oldVM)
    newCdromVolumes := c.getHotpluggableCdromVolumes(newVM)
    return !equality.Semantic.DeepEqual(oldCdromVolumes, newCdromVolumes)
}

func (c *CdromUserPermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
    // Neutralize hotpluggable CD-ROM volumes only
    // CD-ROM disks themselves are NOT neutralized
    // ... (implementation filters out CD-ROM volumes)
}
```

**Key Features:**
- Only allows changing hotpluggable CD-ROM volumes
- Prevents adding/removing CD-ROM disk devices
- Provides granular subset of storage-admin permissions
- Users with storage-admin can do everything cdrom-user can do, plus more

### Example: CPU/Memory Admin

```yaml
# config/clusterroles/vm-resources-admin.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubevirt.io:vm-resources-admin
rules:
  - apiGroups: ["kubevirt.io"]
    resources: ["virtualmachines", "virtualmachines/resources-admin"]
    verbs: ["get", "list", "watch", "update", "patch"]
```

```go
// ResourcesPermissionChecker for CPU and memory
type ResourcesPermissionChecker struct{}

func (r *ResourcesPermissionChecker) Name() string {
    return "resources"
}

func (r *ResourcesPermissionChecker) Subresource() string {
    return "virtualmachines/resources-admin"
}

func (r *ResourcesPermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
    oldCPU := oldVM.Spec.Template.Spec.Domain.CPU
    newCPU := newVM.Spec.Template.Spec.Domain.CPU
    cpuChanged := !reflect.DeepEqual(oldCPU, newCPU)

    oldMem := oldVM.Spec.Template.Spec.Domain.Resources.Requests.Memory()
    newMem := newVM.Spec.Template.Spec.Domain.Resources.Requests.Memory()
    memChanged := !oldMem.Equal(*newMem)

    return cpuChanged || memChanged
}

func (r *ResourcesPermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
    if oldVM.Spec.Template != nil && newVM.Spec.Template != nil {
        oldVM.Spec.Template.Spec.Domain.CPU = nil
        newVM.Spec.Template.Spec.Domain.CPU = nil
        oldVM.Spec.Template.Spec.Domain.Resources = kubevirtiov1.ResourceRequirements{}
        newVM.Spec.Template.Spec.Domain.Resources = kubevirtiov1.ResourceRequirements{}
    }
}
```
