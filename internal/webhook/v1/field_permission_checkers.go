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
	"k8s.io/apimachinery/pkg/api/equality"
	kubevirtiov1 "kubevirt.io/api/core/v1"
)

// FieldPermissionChecker defines an interface for checking permissions on specific field categories.
// Each checker is responsible for:
// 1. Detecting if its field category has changed
// 2. Neutralizing its fields in VM copies (for permission validation)
// 3. Declaring which RBAC subresource grants permission to modify its fields
type FieldPermissionChecker interface {
	// Name returns a human-readable name for this field category (e.g., "storage")
	Name() string

	// Subresource returns the RBAC subresource to check (e.g., "virtualmachines/storage-admin")
	Subresource() string

	// HasChanged returns true if this field category has changed between old and new VM
	HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool

	// Neutralize sets these fields to the same values in both VMs so they won't be detected in DeepEqual
	Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine)
}

// StoragePermissionChecker implements FieldPermissionChecker for storage-related fields.
// It handles permissions for:
// - Volumes (PVCs, DataVolumes, ConfigMaps, Secrets, etc.)
// - Disks (how volumes are attached to the VM)
// - Filesystems (virtio-fs mounts)
type StoragePermissionChecker struct{}

var _ FieldPermissionChecker = &StoragePermissionChecker{}

func (s *StoragePermissionChecker) Name() string {
	return "storage"
}

func (s *StoragePermissionChecker) Subresource() string {
	return "virtualmachines/storage-admin"
}

func (s *StoragePermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
	// Storage-admin is a SUPERSET - it covers ALL storage including CD-ROMs and filesystems
	// Compare ALL volume specifications (the backing storage)
	oldVolumes := oldVM.Spec.Template.Spec.Volumes
	newVolumes := newVM.Spec.Template.Spec.Volumes
	volumesChanged := !equality.Semantic.DeepEqual(oldVolumes, newVolumes)

	// Compare ALL disk specifications (how volumes are attached to the VM)
	oldDisks := oldVM.Spec.Template.Spec.Domain.Devices.Disks
	newDisks := newVM.Spec.Template.Spec.Domain.Devices.Disks
	disksChanged := !equality.Semantic.DeepEqual(oldDisks, newDisks)

	// Compare filesystems (virtio-fs mounts)
	oldFilesystems := oldVM.Spec.Template.Spec.Domain.Devices.Filesystems
	newFilesystems := newVM.Spec.Template.Spec.Domain.Devices.Filesystems
	filesystemsChanged := !equality.Semantic.DeepEqual(oldFilesystems, newFilesystems)

	// Storage has changed if volumes, disks, or filesystems have changed
	return volumesChanged || disksChanged || filesystemsChanged
}

func (s *StoragePermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return
	}

	// Storage-admin is a SUPERSET - neutralize ALL storage (including CD-ROMs and filesystems)
	oldVM.Spec.Template.Spec.Volumes = nil
	newVM.Spec.Template.Spec.Volumes = nil

	oldVM.Spec.Template.Spec.Domain.Devices.Disks = nil
	newVM.Spec.Template.Spec.Domain.Devices.Disks = nil

	oldVM.Spec.Template.Spec.Domain.Devices.Filesystems = nil
	newVM.Spec.Template.Spec.Domain.Devices.Filesystems = nil
}

// CdromUserPermissionChecker implements FieldPermissionChecker for CD-ROM related fields.
// It handles permissions for:
// - CD-ROM devices and their attachments
// - CD-ROM volumes
type CdromUserPermissionChecker struct{}

var _ FieldPermissionChecker = &CdromUserPermissionChecker{}

func (c *CdromUserPermissionChecker) Name() string {
	return "cdrom"
}

func (c *CdromUserPermissionChecker) Subresource() string {
	return "virtualmachines/cdrom-user"
}

func (c *CdromUserPermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
	// CD-ROM operations: inject (media), eject (media), swap (media)
	// Users can only change hotpluggable volumes attached to existing CD-ROM disks.
	// Users CANNOT add or remove CD-ROM disks themselves.

	// First verify that CD-ROM disk definitions haven't changed (count, names, config)
	oldCdromDisks := c.getCdromDisks(oldVM)
	newCdromDisks := c.getCdromDisks(newVM)

	// If the disk definitions changed, this is NOT a cdrom-user operation
	// (this would require higher privileges to modify the VM template)
	if !equality.Semantic.DeepEqual(oldCdromDisks, newCdromDisks) {
		return false
	}

	// Now check if hotpluggable volumes attached to those CD-ROM disks have changed
	oldCdromVolumes := c.getHotpluggableCdromVolumes(oldVM)
	newCdromVolumes := c.getHotpluggableCdromVolumes(newVM)

	// Compare the volumes - any change indicates inject/eject/swap of media
	return !equality.Semantic.DeepEqual(oldCdromVolumes, newCdromVolumes)
}

func (c *CdromUserPermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return
	}

	// Get the names of hotpluggable CD-ROM volumes to neutralize
	oldCdromNames := c.getHotpluggableCdromVolumeNames(oldVM)
	newCdromNames := c.getHotpluggableCdromVolumeNames(newVM)

	// Combine both sets of names
	cdromNames := make(map[string]bool)
	for name := range oldCdromNames {
		cdromNames[name] = true
	}
	for name := range newCdromNames {
		cdromNames[name] = true
	}

	// Remove hotpluggable CD-ROM volumes from both VMs
	// This neutralizes media changes (inject/eject/swap)
	oldVM.Spec.Template.Spec.Volumes = c.filterOutVolumes(oldVM.Spec.Template.Spec.Volumes, cdromNames)
	newVM.Spec.Template.Spec.Volumes = c.filterOutVolumes(newVM.Spec.Template.Spec.Volumes, cdromNames)

	// NOTE: We do NOT neutralize the CD-ROM disks themselves
	// Users cannot add/remove CD-ROM disks - only swap media in existing drives
	// If CD-ROM disk definitions change, that requires different permissions
}

// Helper methods

// getCdromDisks returns all CD-ROM disks from a VM
func (c *CdromUserPermissionChecker) getCdromDisks(vm *kubevirtiov1.VirtualMachine) []kubevirtiov1.Disk {
	if vm.Spec.Template == nil {
		return nil
	}

	var cdromDisks []kubevirtiov1.Disk
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.CDRom != nil {
			cdromDisks = append(cdromDisks, disk)
		}
	}
	return cdromDisks
}

// getHotpluggableCdromVolumes returns hotpluggable volumes that correspond to CD-ROM disks
func (c *CdromUserPermissionChecker) getHotpluggableCdromVolumes(vm *kubevirtiov1.VirtualMachine) []kubevirtiov1.Volume {
	if vm.Spec.Template == nil {
		return nil
	}

	// First, get the names of all CD-ROM disks
	cdromDiskNames := make(map[string]bool)
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.CDRom != nil {
			cdromDiskNames[disk.Name] = true
		}
	}

	// Now find volumes that match CD-ROM disk names and are hotpluggable
	var hotpluggableCdromVolumes []kubevirtiov1.Volume
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		// Check if this volume corresponds to a CD-ROM disk
		if cdromDiskNames[volume.Name] {
			// Check if the volume is hotpluggable
			if c.isVolumeHotpluggable(&volume) {
				hotpluggableCdromVolumes = append(hotpluggableCdromVolumes, volume)
			}
		}
	}

	return hotpluggableCdromVolumes
}

// getHotpluggableCdromVolumeNames returns the names of hotpluggable CD-ROM volumes
func (c *CdromUserPermissionChecker) getHotpluggableCdromVolumeNames(vm *kubevirtiov1.VirtualMachine) map[string]bool {
	names := make(map[string]bool)
	volumes := c.getHotpluggableCdromVolumes(vm)
	for _, vol := range volumes {
		names[vol.Name] = true
	}
	return names
}

// isVolumeHotpluggable checks if a volume is marked as hotpluggable
func (c *CdromUserPermissionChecker) isVolumeHotpluggable(volume *kubevirtiov1.Volume) bool {
	// Check various volume sources for the hotpluggable flag
	if volume.DataVolume != nil && volume.DataVolume.Hotpluggable {
		return true
	}
	if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.Hotpluggable {
		return true
	}
	// Add other volume source types as needed
	return false
}

// filterOutVolumes removes volumes with names in the provided set
func (c *CdromUserPermissionChecker) filterOutVolumes(volumes []kubevirtiov1.Volume, namesToRemove map[string]bool) []kubevirtiov1.Volume {
	var filtered []kubevirtiov1.Volume
	for _, vol := range volumes {
		if !namesToRemove[vol.Name] {
			filtered = append(filtered, vol)
		}
	}
	return filtered
}

// NetworkPermissionChecker implements FieldPermissionChecker for network-related fields.
// It handles permissions for:
// - Network interfaces (spec.template.spec.domain.devices.interfaces)
// - Networks (spec.template.spec.networks)
type NetworkPermissionChecker struct{}

var _ FieldPermissionChecker = &NetworkPermissionChecker{}

func (n *NetworkPermissionChecker) Name() string {
	return "network"
}

func (n *NetworkPermissionChecker) Subresource() string {
	return "virtualmachines/network-admin"
}

func (n *NetworkPermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return false
	}

	// Compare network interfaces
	oldInterfaces := oldVM.Spec.Template.Spec.Domain.Devices.Interfaces
	newInterfaces := newVM.Spec.Template.Spec.Domain.Devices.Interfaces
	interfacesChanged := !equality.Semantic.DeepEqual(oldInterfaces, newInterfaces)

	// Compare networks
	oldNetworks := oldVM.Spec.Template.Spec.Networks
	newNetworks := newVM.Spec.Template.Spec.Networks
	networksChanged := !equality.Semantic.DeepEqual(oldNetworks, newNetworks)

	return interfacesChanged || networksChanged
}

func (n *NetworkPermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return
	}

	// Neutralize network interfaces
	oldVM.Spec.Template.Spec.Domain.Devices.Interfaces = nil
	newVM.Spec.Template.Spec.Domain.Devices.Interfaces = nil

	// Neutralize networks
	oldVM.Spec.Template.Spec.Networks = nil
	newVM.Spec.Template.Spec.Networks = nil
}

// ComputePermissionChecker implements FieldPermissionChecker for compute-related fields.
// It handles permissions for:
// - CPU configuration (spec.template.spec.domain.cpu)
// - Memory and resource requests/limits (spec.template.spec.domain.resources)
type ComputePermissionChecker struct{}

var _ FieldPermissionChecker = &ComputePermissionChecker{}

func (c *ComputePermissionChecker) Name() string {
	return "compute"
}

func (c *ComputePermissionChecker) Subresource() string {
	return "virtualmachines/compute-admin"
}

func (c *ComputePermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return false
	}

	// Compare CPU configuration
	oldCPU := oldVM.Spec.Template.Spec.Domain.CPU
	newCPU := newVM.Spec.Template.Spec.Domain.CPU
	cpuChanged := !equality.Semantic.DeepEqual(oldCPU, newCPU)

	// Compare resource requirements (memory, limits, requests)
	oldResources := oldVM.Spec.Template.Spec.Domain.Resources
	newResources := newVM.Spec.Template.Spec.Domain.Resources
	resourcesChanged := !equality.Semantic.DeepEqual(oldResources, newResources)

	return cpuChanged || resourcesChanged
}

func (c *ComputePermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return
	}

	// Neutralize CPU
	oldVM.Spec.Template.Spec.Domain.CPU = nil
	newVM.Spec.Template.Spec.Domain.CPU = nil

	// Neutralize resources
	oldVM.Spec.Template.Spec.Domain.Resources = kubevirtiov1.ResourceRequirements{}
	newVM.Spec.Template.Spec.Domain.Resources = kubevirtiov1.ResourceRequirements{}
}

// DevicesPermissionChecker implements FieldPermissionChecker for device-related fields.
// It handles permissions for:
// - GPUs (spec.template.spec.domain.devices.gpus)
// - Host devices (spec.template.spec.domain.devices.hostDevices)
// - Watchdog (spec.template.spec.domain.devices.watchdog)
// - TPM (spec.template.spec.domain.devices.tpm)
// - Input devices (spec.template.spec.domain.devices.inputs)
// NOTE: Does NOT include disks, interfaces, or filesystems (covered by storage/network)
type DevicesPermissionChecker struct{}

var _ FieldPermissionChecker = &DevicesPermissionChecker{}

func (d *DevicesPermissionChecker) Name() string {
	return "devices"
}

func (d *DevicesPermissionChecker) Subresource() string {
	return "virtualmachines/devices-admin"
}

func (d *DevicesPermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return false
	}

	oldDevices := oldVM.Spec.Template.Spec.Domain.Devices
	newDevices := newVM.Spec.Template.Spec.Domain.Devices

	// Compare GPUs
	gpusChanged := !equality.Semantic.DeepEqual(oldDevices.GPUs, newDevices.GPUs)

	// Compare host devices
	hostDevicesChanged := !equality.Semantic.DeepEqual(oldDevices.HostDevices, newDevices.HostDevices)

	// Compare watchdog
	watchdogChanged := !equality.Semantic.DeepEqual(oldDevices.Watchdog, newDevices.Watchdog)

	// Compare TPM
	tpmChanged := !equality.Semantic.DeepEqual(oldDevices.TPM, newDevices.TPM)

	// Compare input devices
	inputsChanged := !equality.Semantic.DeepEqual(oldDevices.Inputs, newDevices.Inputs)

	return gpusChanged || hostDevicesChanged || watchdogChanged || tpmChanged || inputsChanged
}

func (d *DevicesPermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
	if oldVM.Spec.Template == nil || newVM.Spec.Template == nil {
		return
	}

	// Neutralize GPUs
	oldVM.Spec.Template.Spec.Domain.Devices.GPUs = nil
	newVM.Spec.Template.Spec.Domain.Devices.GPUs = nil

	// Neutralize host devices
	oldVM.Spec.Template.Spec.Domain.Devices.HostDevices = nil
	newVM.Spec.Template.Spec.Domain.Devices.HostDevices = nil

	// Neutralize watchdog
	oldVM.Spec.Template.Spec.Domain.Devices.Watchdog = nil
	newVM.Spec.Template.Spec.Domain.Devices.Watchdog = nil

	// Neutralize TPM
	oldVM.Spec.Template.Spec.Domain.Devices.TPM = nil
	newVM.Spec.Template.Spec.Domain.Devices.TPM = nil

	// Neutralize input devices
	oldVM.Spec.Template.Spec.Domain.Devices.Inputs = nil
	newVM.Spec.Template.Spec.Domain.Devices.Inputs = nil
}

// LifecyclePermissionChecker implements FieldPermissionChecker for VM lifecycle fields.
// It handles permissions for:
// - spec.running (bool: direct start/stop control)
// - spec.runStrategy (string: advanced lifecycle strategy like Always, Halted, Manual, etc.)
// Note: running and runStrategy are mutually exclusive in KubeVirt
type LifecyclePermissionChecker struct{}

var _ FieldPermissionChecker = &LifecyclePermissionChecker{}

func (l *LifecyclePermissionChecker) Name() string {
	return "lifecycle"
}

func (l *LifecyclePermissionChecker) Subresource() string {
	return "virtualmachines/lifecycle-admin"
}

func (l *LifecyclePermissionChecker) HasChanged(oldVM, newVM *kubevirtiov1.VirtualMachine) bool {
	// Check if running field has changed
	runningChanged := !equality.Semantic.DeepEqual(oldVM.Spec.Running, newVM.Spec.Running)

	// Check if runStrategy field has changed
	runStrategyChanged := !equality.Semantic.DeepEqual(oldVM.Spec.RunStrategy, newVM.Spec.RunStrategy)

	return runningChanged || runStrategyChanged
}

func (l *LifecyclePermissionChecker) Neutralize(oldVM, newVM *kubevirtiov1.VirtualMachine) {
	// Neutralize running field
	oldVM.Spec.Running = nil
	newVM.Spec.Running = nil

	// Neutralize runStrategy field
	oldVM.Spec.RunStrategy = nil
	newVM.Spec.RunStrategy = nil
}
