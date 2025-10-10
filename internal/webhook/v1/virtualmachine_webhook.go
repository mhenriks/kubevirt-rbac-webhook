/*
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
*/

package v1

import (
	"context"
	"fmt"

	authenticationv1 "k8s.io/api/authentication/v1"
	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtiov1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// nolint:unused
// log is for logging in this package.
var virtualmachinelog = logf.Log.WithName("virtualmachine-resource")

// SetupVirtualMachineWebhookWithManager registers the webhook for VirtualMachine in the manager.
func SetupVirtualMachineWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&kubevirtiov1.VirtualMachine{}).
		WithValidator(&VirtualMachineCustomValidator{
			Client: mgr.GetClient(),
			// IMPORTANT: Order matters for hierarchical permissions (subset before superset)
			FieldCheckers: []FieldPermissionChecker{
				// Independent permissions (no hierarchy, can be in any order)
				&NetworkPermissionChecker{},
				&ComputePermissionChecker{},
				&DevicesPermissionChecker{},
				&LifecyclePermissionChecker{},

				// Hierarchical permissions (subset before superset)
				&CdromUserPermissionChecker{}, // Subset: CD-ROM media only
				&StoragePermissionChecker{},   // Superset: All storage (including CD-ROMs)
			},
			PermissionChecker: &SubjectAccessReviewPermissionChecker{
				Client: mgr.GetClient(),
			},
		}).
		Complete()
}

// NOTE: The ValidatingWebhookConfiguration is managed statically via config/webhook/manifests.yaml
// and deployed with kustomize. This is a simple webhook-only deployment with no controllers or CRDs.
//
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// PermissionChecker defines an interface for checking RBAC permissions.
// This abstraction allows for easier testing by enabling mock implementations.
type PermissionChecker interface {
	// CheckPermission checks if a user has permission to update a specific subresource
	CheckPermission(ctx context.Context, userInfo authenticationv1.UserInfo, namespace, vmName, subresource string) (bool, error)
}

// SubjectAccessReviewPermissionChecker implements PermissionChecker using Kubernetes SubjectAccessReview.
type SubjectAccessReviewPermissionChecker struct {
	Client client.Client
}

var _ PermissionChecker = &SubjectAccessReviewPermissionChecker{}

// CheckPermission uses SubjectAccessReview to check if a user has permission for a subresource
// on a specific VM. This enables resource-name-specific RBAC policies.
func (p *SubjectAccessReviewPermissionChecker) CheckPermission(ctx context.Context, userInfo authenticationv1.UserInfo, namespace, vmName, subresource string) (bool, error) {
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User:   userInfo.Username,
			Groups: userInfo.Groups,
			UID:    userInfo.UID,
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "update",
				Group:     "kubevirt.io",
				Resource:  subresource,
				Name:      vmName,
			},
		},
	}

	err := p.Client.Create(ctx, sar)
	if err != nil {
		return false, fmt.Errorf("failed to create SubjectAccessReview: %w", err)
	}

	return sar.Status.Allowed, nil
}

// VirtualMachineCustomValidator struct is responsible for validating the VirtualMachine resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type VirtualMachineCustomValidator struct {
	Client            client.Client
	FieldCheckers     []FieldPermissionChecker
	PermissionChecker PermissionChecker
}

var _ webhook.CustomValidator = &VirtualMachineCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachine.
func (v *VirtualMachineCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	virtualmachine, ok := obj.(*kubevirtiov1.VirtualMachine)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachine object but got %T", obj)
	}
	virtualmachinelog.Info("Validation for VirtualMachine upon creation", "name", virtualmachine.GetName())

	// For create operations, we allow all creates (permission is handled by standard RBAC)
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachine.
func (v *VirtualMachineCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newVM, ok := newObj.(*kubevirtiov1.VirtualMachine)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachine object for the newObj but got %T", newObj)
	}
	oldVM, ok := oldObj.(*kubevirtiov1.VirtualMachine)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachine object for the oldObj but got %T", oldObj)
	}

	virtualmachinelog.Info("Validation for VirtualMachine upon update", "name", newVM.GetName())

	// Security Model: Opt-in Restrictions (Backwards Compatible)
	// Step 1: If user has "virtualmachines/full-admin" → allow everything
	//         IMPORTANT: full-admin grants UNRESTRICTED access to ALL spec/metadata fields,
	//         not just fields covered by granular roles. This is the highest permission level.
	//         (full-admin is an aggregated role and also aggregates to built-in admin/edit roles)
	// Step 2: Check if user has ANY granular subresource permissions (e.g., virtualmachines/storage-admin)
	//         - If NO subresource permissions → allow everything (backwards compatible)
	//         - If YES → proceed to granular checks (opt-in to restrictions)
	// Step 3: For users with subresource permissions, validate each change against those permissions
	// Step 4: Check neutralized object for unauthorized changes to spec or metadata
	// Step 5: Return success if all checks pass

	// Get user info from the admission request in the context
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admission request from context: %w", err)
	}

	userInfo := req.UserInfo

	// Step 1: If user has full-admin permission, allow everything
	// Check for virtualmachines/full-admin (aggregated role with all VM permissions)
	// Note: Users with Kubernetes built-in 'admin' or 'edit' roles also get full-admin via aggregation
	// IMPORTANT: full-admin allows changes to ALL spec/metadata fields, not just those covered by granular roles
	hasFullAdminPermission, err := v.PermissionChecker.CheckPermission(ctx, userInfo, newVM.Namespace, newVM.Name, "virtualmachines/full-admin")
	if err != nil {
		return nil, fmt.Errorf("failed to check 'virtualmachines/full-admin' permission: %w", err)
	}

	if hasFullAdminPermission {
		// User has full-admin permission, allow all changes (unrestricted access)
		return nil, nil
	}

	// Step 2: Check if user has ANY of the new subresource permissions
	// Check if user has any subresource permissions
	hasAnySubresource := false
	subresourcePermissions := make(map[string]bool)

	for _, checker := range v.FieldCheckers {
		hasPermission, err := v.PermissionChecker.CheckPermission(ctx, userInfo, newVM.Namespace, newVM.Name, checker.Subresource())
		if err != nil {
			return nil, fmt.Errorf("failed to check %s permission: %w", checker.Name(), err)
		}
		subresourcePermissions[checker.Subresource()] = hasPermission
		if hasPermission {
			hasAnySubresource = true
		}
	}

	// If user has NO subresource permissions, allow everything (backwards compatible)
	if !hasAnySubresource {
		return nil, nil
	}

	// Step 3: User has opted-in to granular permissions by having subresource permissions
	// Create copies that we'll mutate to "neutralize" permitted changes
	oldCopy := oldVM.DeepCopy()
	newCopy := newVM.DeepCopy()

	// Run all field-specific permission checks
	// IMPORTANT: Check HasChanged on the COPIES, not originals
	// This allows subset permissions (cdrom-user) to neutralize changes before
	// superset permissions (storage-admin) see them
	for _, checker := range v.FieldCheckers {
		if checker.HasChanged(oldCopy, newCopy) {
			// This field category has changes, check if user has permission
			hasPermission := subresourcePermissions[checker.Subresource()]

			if hasPermission {
				// User has permission for this field category, neutralize it
				checker.Neutralize(oldCopy, newCopy)
			} else {
				// User lacks this specific permission.
				// We'll only deny if ALL checkers run and changes remain
			}
		}
	}

	// Step 4: After all field-specific checks, see if any unauthorized changes remain
	// We need to check both Spec and Metadata, but ignore system-managed fields

	// Normalize system-managed metadata fields that we don't care about
	v.normalizeSystemMetadata(&oldCopy.ObjectMeta, &newCopy.ObjectMeta)

	// Check if Spec or Metadata has unauthorized changes
	specChanged := !equality.Semantic.DeepEqual(oldCopy.Spec, newCopy.Spec)
	metadataChanged := !equality.Semantic.DeepEqual(oldCopy.ObjectMeta, newCopy.ObjectMeta)

	if specChanged || metadataChanged {
		if metadataChanged {
			return nil, fmt.Errorf("user does not have permission to modify VirtualMachine metadata")
		}
		return nil, fmt.Errorf("user does not have permission to modify one or more VirtualMachine spec fields")
	}

	// Step 5: All changes were authorized
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachine.
func (v *VirtualMachineCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	virtualmachine, ok := obj.(*kubevirtiov1.VirtualMachine)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachine object but got %T", obj)
	}
	virtualmachinelog.Info("Validation for VirtualMachine upon deletion", "name", virtualmachine.GetName())

	// Deletion is handled by standard RBAC
	return nil, nil
}

// normalizeSystemMetadata sets system-managed metadata fields to the same values
// so they don't cause false positives when checking for user-initiated metadata changes
func (v *VirtualMachineCustomValidator) normalizeSystemMetadata(oldMeta, newMeta *metav1.ObjectMeta) {
	// These fields are managed by the API server and should be ignored
	oldMeta.ResourceVersion = ""
	newMeta.ResourceVersion = ""

	oldMeta.Generation = 0
	newMeta.Generation = 0

	oldMeta.ManagedFields = nil
	newMeta.ManagedFields = nil

	oldMeta.SelfLink = ""
	newMeta.SelfLink = ""

	// UID and timestamps are immutable, but normalize them anyway for consistency
	oldMeta.UID = ""
	newMeta.UID = ""

	oldMeta.CreationTimestamp = metav1.Time{}
	newMeta.CreationTimestamp = metav1.Time{}

	oldMeta.DeletionTimestamp = nil
	newMeta.DeletionTimestamp = nil

	oldMeta.DeletionGracePeriodSeconds = nil
	newMeta.DeletionGracePeriodSeconds = nil
}
