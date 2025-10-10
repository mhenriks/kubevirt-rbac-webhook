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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtiov1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("VirtualMachine Webhook", func() {
	var (
		validator VirtualMachineCustomValidator
		ctx       context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		validator = VirtualMachineCustomValidator{
			Client: k8sClient,
		}
	})

	Context("ValidateCreate", func() {
		It("should allow VM creation", func() {
			vm := &kubevirtiov1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "default",
				},
				Spec: kubevirtiov1.VirtualMachineSpec{
					Running: boolPtr(false),
				},
			}

			warnings, err := validator.ValidateCreate(ctx, vm)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("ValidateDelete", func() {
		It("should allow VM deletion", func() {
			vm := &kubevirtiov1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "default",
				},
			}

			warnings, err := validator.ValidateDelete(ctx, vm)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	// Note: StoragePermissionChecker and other field checker tests are in field_permission_checkers_test.go

	Context("normalizeSystemMetadata", func() {
		It("should normalize system-managed fields", func() {
			oldMeta := metav1.ObjectMeta{
				Name:            "test-vm",
				Namespace:       "default",
				ResourceVersion: "12345",
				Generation:      5,
				UID:             "abc-123",
				Labels: map[string]string{
					"app": "test",
				},
				Annotations: map[string]string{
					"key": "value",
				},
			}

			newMeta := metav1.ObjectMeta{
				Name:            "test-vm",
				Namespace:       "default",
				ResourceVersion: "67890", // Different - should be normalized
				Generation:      6,       // Different - should be normalized
				UID:             "abc-123",
				Labels: map[string]string{
					"app": "test",
				},
				Annotations: map[string]string{
					"key": "value",
				},
			}

			validator.normalizeSystemMetadata(&oldMeta, &newMeta)

			// System fields should now be equal
			Expect(oldMeta.ResourceVersion).To(Equal(newMeta.ResourceVersion))
			Expect(oldMeta.Generation).To(Equal(newMeta.Generation))

			// User-managed fields should be unchanged
			Expect(oldMeta.Labels).To(Equal(newMeta.Labels))
			Expect(oldMeta.Annotations).To(Equal(newMeta.Annotations))
		})

		It("should detect label changes after normalization", func() {
			oldMeta := metav1.ObjectMeta{
				Name:            "test-vm",
				ResourceVersion: "12345",
				Labels: map[string]string{
					"app": "test",
				},
			}

			newMeta := metav1.ObjectMeta{
				Name:            "test-vm",
				ResourceVersion: "67890",
				Labels: map[string]string{
					"app":     "test",
					"version": "v1", // Added label
				},
			}

			validator.normalizeSystemMetadata(&oldMeta, &newMeta)

			// Labels should still be different
			Expect(equality.Semantic.DeepEqual(oldMeta, newMeta)).To(BeFalse())
		})

		It("should detect annotation changes after normalization", func() {
			oldMeta := metav1.ObjectMeta{
				Name:            "test-vm",
				ResourceVersion: "12345",
				Annotations: map[string]string{
					"key": "value",
				},
			}

			newMeta := metav1.ObjectMeta{
				Name:            "test-vm",
				ResourceVersion: "67890",
				Annotations: map[string]string{
					"key":  "value",
					"key2": "value2", // Added annotation
				},
			}

			validator.normalizeSystemMetadata(&oldMeta, &newMeta)

			// Annotations should still be different
			Expect(equality.Semantic.DeepEqual(oldMeta, newMeta)).To(BeFalse())
		})
	})

	Describe("ValidateUpdate", func() {
		var (
			validator *VirtualMachineCustomValidator
			mockPerm  *MockPermissionChecker
			oldVM     *kubevirtiov1.VirtualMachine
			newVM     *kubevirtiov1.VirtualMachine
		)

		BeforeEach(func() {
			mockPerm = &MockPermissionChecker{
				permissions: make(map[string]bool),
			}

			validator = &VirtualMachineCustomValidator{
				// IMPORTANT: Order matters for hierarchical permissions (subset before superset)
				FieldCheckers: []FieldPermissionChecker{
					// Independent permissions
					&NetworkPermissionChecker{},
					&ComputePermissionChecker{},
					&DevicesPermissionChecker{},

					// Hierarchical permissions (subset before superset)
					&CdromUserPermissionChecker{}, // Subset
					&StoragePermissionChecker{},   // Superset
				},
				PermissionChecker: mockPerm,
			}

			// Setup base VMs
			oldVM = &kubevirtiov1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "default",
				},
				Spec: kubevirtiov1.VirtualMachineSpec{
					Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtiov1.VirtualMachineInstanceSpec{
							Domain: kubevirtiov1.DomainSpec{
								CPU: &kubevirtiov1.CPU{Cores: 2},
								Devices: kubevirtiov1.Devices{
									Disks: []kubevirtiov1.Disk{
										{Name: "disk1"},
									},
								},
							},
							Volumes: []kubevirtiov1.Volume{
								{Name: "volume1"},
							},
						},
					},
				},
			}

			newVM = oldVM.DeepCopy()

			// Add admission request to context
			ctx = admission.NewContextWithRequest(ctx, admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{
						Username: "test-user",
						Groups:   []string{"test-group"},
					},
				},
			})
		})

		Context("with full-admin permissions", func() {
			It("should allow all changes when user has full-admin permission", func() {
				mockPerm.permissions["virtualmachines/full-admin"] = true

				// Make arbitrary changes (CPU, storage, network, etc.)
				newVM.Spec.Template.Spec.Domain.CPU.Cores = 4
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "volume2"})
				newVM.Spec.Template.Spec.Networks = append(newVM.Spec.Template.Spec.Networks, kubevirtiov1.Network{Name: "network2"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})
		})

		Context("without subresource permissions", func() {
			It("should allow all changes when user has no subresource permissions (backwards compatible)", func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = false
				mockPerm.permissions["virtualmachines/cdrom-user"] = false

				// Make storage changes
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "volume2"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})
		})

		Context("with storage-admin permission", func() {
			BeforeEach(func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = true
				mockPerm.permissions["virtualmachines/cdrom-user"] = false
			})

			It("should allow storage changes", func() {
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "volume2"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should allow filesystem changes", func() {
				newVM.Spec.Template.Spec.Domain.Devices.Filesystems = append(newVM.Spec.Template.Spec.Domain.Devices.Filesystems, kubevirtiov1.Filesystem{Name: "shared-fs"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should deny non-storage changes", func() {
				newVM.Spec.Template.Spec.Domain.CPU.Cores = 4

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("permission"))
				Expect(warnings).To(BeNil())
			})

			It("should deny metadata changes", func() {
				newVM.Labels = map[string]string{"new": "label"}

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("metadata"))
				Expect(warnings).To(BeNil())
			})
		})

		Context("with cdrom-user permission", func() {
			BeforeEach(func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = false
				mockPerm.permissions["virtualmachines/cdrom-user"] = true

				// Setup CD-ROM disk in both VMs
				cdromDisk := kubevirtiov1.Disk{
					Name: "cdrom1",
					DiskDevice: kubevirtiov1.DiskDevice{
						CDRom: &kubevirtiov1.CDRomTarget{
							Bus: "sata",
						},
					},
				}
				oldVM.Spec.Template.Spec.Domain.Devices.Disks = append(oldVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
				newVM.Spec.Template.Spec.Domain.Devices.Disks = append(newVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
			})

			It("should allow hotpluggable CD-ROM media changes", func() {
				// Add hotpluggable CD-ROM volume
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{
					Name: "cdrom1",
					VolumeSource: kubevirtiov1.VolumeSource{
						DataVolume: &kubevirtiov1.DataVolumeSource{
							Name:         "ubuntu-iso",
							Hotpluggable: true,
						},
					},
				})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should deny storage changes", func() {
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "regular-volume"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				// With hierarchical model, non-CD-ROM storage changes aren't caught by specific checker
				// so they fall through to the generic "spec fields" error
				Expect(err.Error()).To(ContainSubstring("permission"))
				Expect(warnings).To(BeNil())
			})
		})

		Context("with multiple permissions", func() {
			BeforeEach(func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = true
				mockPerm.permissions["virtualmachines/cdrom-user"] = true

				// Setup CD-ROM disk
				cdromDisk := kubevirtiov1.Disk{
					Name: "cdrom1",
					DiskDevice: kubevirtiov1.DiskDevice{
						CDRom: &kubevirtiov1.CDRomTarget{
							Bus: "sata",
						},
					},
				}
				oldVM.Spec.Template.Spec.Domain.Devices.Disks = append(oldVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
				newVM.Spec.Template.Spec.Domain.Devices.Disks = append(newVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
			})

			It("should allow both storage and CD-ROM changes", func() {
				// Add regular volume
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "volume2"})

				// Add hotpluggable CD-ROM volume
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{
					Name: "cdrom1",
					VolumeSource: kubevirtiov1.VolumeSource{
						DataVolume: &kubevirtiov1.DataVolumeSource{
							Name:         "ubuntu-iso",
							Hotpluggable: true,
						},
					},
				})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should still deny unauthorized changes", func() {
				newVM.Spec.Template.Spec.Domain.CPU.Cores = 4

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("permission"))
				Expect(warnings).To(BeNil())
			})
		})

		Context("with network-admin permission", func() {
			BeforeEach(func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = false
				mockPerm.permissions["virtualmachines/cdrom-user"] = false
				mockPerm.permissions["virtualmachines/network-admin"] = true
			})

			It("should allow network changes", func() {
				newVM.Spec.Template.Spec.Networks = append(newVM.Spec.Template.Spec.Networks, kubevirtiov1.Network{Name: "secondary"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should allow interface changes", func() {
				newVM.Spec.Template.Spec.Domain.Devices.Interfaces = append(newVM.Spec.Template.Spec.Domain.Devices.Interfaces, kubevirtiov1.Interface{Name: "eth1"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should deny storage changes", func() {
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "volume2"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("permission"))
				Expect(warnings).To(BeNil())
			})
		})

		Context("with compute-admin permission", func() {
			BeforeEach(func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = false
				mockPerm.permissions["virtualmachines/network-admin"] = false
				mockPerm.permissions["virtualmachines/compute-admin"] = true
			})

			It("should allow CPU changes", func() {
				newVM.Spec.Template.Spec.Domain.CPU.Cores = 4

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should allow resource changes", func() {
				newVM.Spec.Template.Spec.Domain.Resources.OvercommitGuestOverhead = true

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should deny storage changes", func() {
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{Name: "volume2"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("permission"))
				Expect(warnings).To(BeNil())
			})
		})

		Context("with devices-admin permission", func() {
			BeforeEach(func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = false
				mockPerm.permissions["virtualmachines/network-admin"] = false
				mockPerm.permissions["virtualmachines/devices-admin"] = true
			})

			It("should allow GPU changes", func() {
				newVM.Spec.Template.Spec.Domain.Devices.GPUs = append(newVM.Spec.Template.Spec.Domain.Devices.GPUs, kubevirtiov1.GPU{Name: "gpu1"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should allow host device changes", func() {
				newVM.Spec.Template.Spec.Domain.Devices.HostDevices = append(newVM.Spec.Template.Spec.Domain.Devices.HostDevices, kubevirtiov1.HostDevice{Name: "dev1"})

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should deny compute changes", func() {
				newVM.Spec.Template.Spec.Domain.CPU.Cores = 4

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("permission"))
				Expect(warnings).To(BeNil())
			})
		})

		Context("hierarchical permissions (superset/subset)", func() {
			It("should allow storage-admin to make CD-ROM changes (even without cdrom-user)", func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = true
				mockPerm.permissions["virtualmachines/cdrom-user"] = false // User does NOT have cdrom-user

				// Setup CD-ROM disk in both VMs
				cdromDisk := kubevirtiov1.Disk{
					Name: "cdrom1",
					DiskDevice: kubevirtiov1.DiskDevice{
						CDRom: &kubevirtiov1.CDRomTarget{
							Bus: "sata",
						},
					},
				}
				oldVM.Spec.Template.Spec.Domain.Devices.Disks = append(oldVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
				newVM.Spec.Template.Spec.Domain.Devices.Disks = append(newVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)

				// Add hotpluggable CD-ROM volume (CD-ROM change)
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{
					Name: "cdrom1",
					VolumeSource: kubevirtiov1.VolumeSource{
						DataVolume: &kubevirtiov1.DataVolumeSource{
							Name:         "ubuntu-iso",
							Hotpluggable: true,
						},
					},
				})

				// Should succeed because storage-admin (superset) covers CD-ROM changes
				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should allow storage-admin to make BOTH CD-ROM and regular storage changes", func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = true
				mockPerm.permissions["virtualmachines/cdrom-user"] = false // User does NOT have cdrom-user

				// Setup CD-ROM disk in both VMs
				cdromDisk := kubevirtiov1.Disk{
					Name: "cdrom1",
					DiskDevice: kubevirtiov1.DiskDevice{
						CDRom: &kubevirtiov1.CDRomTarget{
							Bus: "sata",
						},
					},
				}
				oldVM.Spec.Template.Spec.Domain.Devices.Disks = append(oldVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
				newVM.Spec.Template.Spec.Domain.Devices.Disks = append(newVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)

				// Make BOTH types of storage changes:
				// 1. Regular storage change
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{
					Name: "regular-disk",
				})

				// 2. CD-ROM media change
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{
					Name: "cdrom1",
					VolumeSource: kubevirtiov1.VolumeSource{
						DataVolume: &kubevirtiov1.DataVolumeSource{
							Name:         "ubuntu-iso",
							Hotpluggable: true,
						},
					},
				})

				// Should succeed - storage-admin covers BOTH changes
				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})

			It("should allow user with both permissions to make CD-ROM changes", func() {
				mockPerm.permissions["virtualmachines/full-admin"] = false
				mockPerm.permissions["*"] = false
				mockPerm.permissions["virtualmachines/storage-admin"] = true
				mockPerm.permissions["virtualmachines/cdrom-user"] = true // Has both

				// Setup CD-ROM disk
				cdromDisk := kubevirtiov1.Disk{
					Name: "cdrom1",
					DiskDevice: kubevirtiov1.DiskDevice{
						CDRom: &kubevirtiov1.CDRomTarget{
							Bus: "sata",
						},
					},
				}
				oldVM.Spec.Template.Spec.Domain.Devices.Disks = append(oldVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)
				newVM.Spec.Template.Spec.Domain.Devices.Disks = append(newVM.Spec.Template.Spec.Domain.Devices.Disks, cdromDisk)

				// Add hotpluggable CD-ROM volume
				newVM.Spec.Template.Spec.Volumes = append(newVM.Spec.Template.Spec.Volumes, kubevirtiov1.Volume{
					Name: "cdrom1",
					VolumeSource: kubevirtiov1.VolumeSource{
						DataVolume: &kubevirtiov1.DataVolumeSource{
							Name:         "fedora-iso",
							Hotpluggable: true,
						},
					},
				})

				// Should succeed (cdrom-user neutralizes, storage-admin would also cover it)
				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeNil())
			})
		})

		Context("error handling", func() {
			It("should handle permission check errors", func() {
				mockPerm.shouldError = true

				warnings, err := validator.ValidateUpdate(ctx, oldVM, newVM)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to check"))
				Expect(warnings).To(BeNil())
			})
		})
	})
})

// MockPermissionChecker is a mock implementation of PermissionChecker for testing.
type MockPermissionChecker struct {
	permissions map[string]bool
	shouldError bool
}

var _ PermissionChecker = &MockPermissionChecker{}

// CheckPermission returns the mocked permission result or an error if configured to do so.
func (m *MockPermissionChecker) CheckPermission(ctx context.Context, userInfo authenticationv1.UserInfo, namespace, vmName, subresource string) (bool, error) {
	if m.shouldError {
		return false, fmt.Errorf("mock permission check error")
	}
	return m.permissions[subresource], nil
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
