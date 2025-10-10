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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/equality"
	kubevirtiov1 "kubevirt.io/api/core/v1"
)

// Helper function for creating RunStrategy pointers in tests
func strategyPtr(s string) *kubevirtiov1.VirtualMachineRunStrategy {
	strategy := kubevirtiov1.VirtualMachineRunStrategy(s)
	return &strategy
}

var _ = Describe("Field Permission Checkers", func() {
	Describe("StoragePermissionChecker", func() {
		var checker *StoragePermissionChecker

		BeforeEach(func() {
			checker = &StoragePermissionChecker{}
		})

		It("should have correct name and subresource", func() {
			Expect(checker.Name()).To(Equal("storage"))
			Expect(checker.Subresource()).To(Equal("virtualmachines/storage-admin"))
		})

		Context("HasChanged", func() {
			It("should detect when filesystems are added", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Filesystems: []kubevirtiov1.Filesystem{},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Filesystems: []kubevirtiov1.Filesystem{
											{Name: "fs1"},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect when volumes are added", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Volumes: []kubevirtiov1.Volume{
									{Name: "volume1"},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect when disks are added", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{Name: "disk1"},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should not detect changes when storage is identical", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
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

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
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

				Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
			})
		})

		Context("Neutralize", func() {
			It("should set volumes, disks, and filesystems to nil in both VMs", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
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

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{Name: "disk2"},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{Name: "volume2"},
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, storage fields should be nil
				Expect(oldVM.Spec.Template.Spec.Volumes).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Volumes).To(BeNil())
				Expect(oldVM.Spec.Template.Spec.Domain.Devices.Disks).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Domain.Devices.Disks).To(BeNil())
				Expect(oldVM.Spec.Template.Spec.Domain.Devices.Filesystems).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Domain.Devices.Filesystems).To(BeNil())
			})

			It("should make storage-only changes invisible to DeepEqual", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
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

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2}, // Same
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{Name: "disk2"}, // Different
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{Name: "volume2"}, // Different
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, only storage changed, so Specs should be equal
				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeTrue())
			})

			It("should preserve non-storage differences", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
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

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 4}, // Different - non-storage
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{Name: "disk2"}, // Different - storage
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{Name: "volume2"}, // Different - storage
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, non-storage changes (CPU) should still be visible
				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeFalse())
			})
		})
	})

	Describe("CdromUserPermissionChecker", func() {
		var checker *CdromUserPermissionChecker

		BeforeEach(func() {
			checker = &CdromUserPermissionChecker{}
		})

		It("should have correct name and subresource", func() {
			Expect(checker.Name()).To(Equal("cdrom"))
			Expect(checker.Subresource()).To(Equal("virtualmachines/cdrom-user"))
		})

		Context("HasChanged", func() {
			It("should detect when hotpluggable CD-ROM media is injected", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect when hotpluggable CD-ROM media is ejected", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect when hotpluggable CD-ROM media is swapped", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "fedora-iso", // Changed
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should NOT detect changes when CD-ROM disk is added (returns false for higher privilege operation)", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				// Returns false because disk definitions changed - not a cdrom-user operation
				Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
			})

			It("should NOT detect changes when CD-ROM disk is removed (returns false for higher privilege operation)", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{},
									},
								},
								Volumes: []kubevirtiov1.Volume{},
							},
						},
					},
				}

				// Returns false because disk definitions changed - not a cdrom-user operation
				Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
			})

			It("should NOT detect changes when only non-hotpluggable volumes change", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: false, // Not hotpluggable
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "fedora-iso", // Changed but not hotpluggable
												Hotpluggable: false,
											},
										},
									},
								},
							},
						},
					},
				}

				// Should not detect as cdrom-user change since volumes aren't hotpluggable
				Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
			})

			It("should not detect changes when CD-ROM state is identical", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
			})
		})

		Context("Neutralize", func() {
			It("should neutralize hotpluggable CD-ROM volumes but NOT CD-ROM disks", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "fedora-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// CD-ROM volumes should be removed
				Expect(oldVM.Spec.Template.Spec.Volumes).To(HaveLen(0))
				Expect(newVM.Spec.Template.Spec.Volumes).To(HaveLen(0))

				// CD-ROM disks should still be present (NOT neutralized)
				Expect(oldVM.Spec.Template.Spec.Domain.Devices.Disks).To(HaveLen(1))
				Expect(newVM.Spec.Template.Spec.Domain.Devices.Disks).To(HaveLen(1))
			})

			It("should make CD-ROM media changes invisible to DeepEqual", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2}, // Same
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "fedora-iso", // Different
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, only CD-ROM media changed, so Specs should be equal
				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeTrue())
			})

			It("should preserve non-CD-ROM differences", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 4}, // Different - non-CD-ROM
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "fedora-iso", // Different - CD-ROM media
												Hotpluggable: true,
											},
										},
									},
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, non-CD-ROM changes (CPU) should still be visible
				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeFalse())
			})

			It("should not neutralize non-hotpluggable CD-ROM volumes", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: false, // Not hotpluggable
											},
										},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Disks: []kubevirtiov1.Disk{
											{
												Name: "cdrom1",
												DiskDevice: kubevirtiov1.DiskDevice{
													CDRom: &kubevirtiov1.CDRomTarget{
														Bus: "sata",
													},
												},
											},
										},
									},
								},
								Volumes: []kubevirtiov1.Volume{
									{
										Name: "cdrom1",
										VolumeSource: kubevirtiov1.VolumeSource{
											DataVolume: &kubevirtiov1.DataVolumeSource{
												Name:         "ubuntu-iso",
												Hotpluggable: false,
											},
										},
									},
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// Non-hotpluggable volumes should NOT be removed
				Expect(oldVM.Spec.Template.Spec.Volumes).To(HaveLen(1))
				Expect(newVM.Spec.Template.Spec.Volumes).To(HaveLen(1))
			})
		})
	})

	Describe("NetworkPermissionChecker", func() {
		var checker *NetworkPermissionChecker

		BeforeEach(func() {
			checker = &NetworkPermissionChecker{}
		})

		It("should have correct name and subresource", func() {
			Expect(checker.Name()).To(Equal("network"))
			Expect(checker.Subresource()).To(Equal("virtualmachines/network-admin"))
		})

		Context("HasChanged", func() {
			It("should detect when interfaces are added", func() {
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

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
											{Name: "secondary"},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect when networks are added", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
									{Name: "secondary"},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect when both interfaces and networks are modified", func() {
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
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
											{Name: "secondary"},
										},
									},
								},
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
									{Name: "secondary"},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should not detect changes when network state is identical", func() {
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
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
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
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeFalse())
			})
		})

		Context("Neutralize", func() {
			It("should neutralize both interfaces and networks", func() {
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
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
											{Name: "secondary"},
										},
									},
								},
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
									{Name: "secondary"},
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// Interfaces and networks should be nil
				Expect(oldVM.Spec.Template.Spec.Domain.Devices.Interfaces).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Domain.Devices.Interfaces).To(BeNil())
				Expect(oldVM.Spec.Template.Spec.Networks).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Networks).To(BeNil())
			})

			It("should make network-only changes invisible to DeepEqual", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
										},
									},
								},
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2}, // Same
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
											{Name: "secondary"}, // Different
										},
									},
								},
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
									{Name: "secondary"}, // Different
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, only network changed, so Specs should be equal
				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeTrue())
			})

			It("should preserve non-network differences", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
										},
									},
								},
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 4}, // Different - non-network
									Devices: kubevirtiov1.Devices{
										Interfaces: []kubevirtiov1.Interface{
											{Name: "default"},
											{Name: "secondary"}, // Different - network
										},
									},
								},
								Networks: []kubevirtiov1.Network{
									{Name: "default"},
									{Name: "secondary"}, // Different - network
								},
							},
						},
					},
				}

				checker.Neutralize(oldVM, newVM)

				// After neutralization, non-network changes (CPU) should still be visible
				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeFalse())
			})
		})
	})

	Describe("ComputePermissionChecker", func() {
		var checker *ComputePermissionChecker

		BeforeEach(func() {
			checker = &ComputePermissionChecker{}
		})

		It("should have correct name and subresource", func() {
			Expect(checker.Name()).To(Equal("compute"))
			Expect(checker.Subresource()).To(Equal("virtualmachines/compute-admin"))
		})

		Context("HasChanged", func() {
			It("should detect CPU changes", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 4},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect resource changes", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
									Resources: kubevirtiov1.ResourceRequirements{
										OvercommitGuestOverhead: true,
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should not detect changes when compute is identical", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
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
			It("should neutralize CPU and resources", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									CPU: &kubevirtiov1.CPU{Cores: 2},
									Resources: kubevirtiov1.ResourceRequirements{
										OvercommitGuestOverhead: true,
									},
								},
							},
						},
					},
				}

				newVM := oldVM.DeepCopy()
				newVM.Spec.Template.Spec.Domain.CPU.Cores = 4

				checker.Neutralize(oldVM, newVM)

				Expect(oldVM.Spec.Template.Spec.Domain.CPU).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Domain.CPU).To(BeNil())
				Expect(oldVM.Spec.Template.Spec.Domain.Resources).To(Equal(kubevirtiov1.ResourceRequirements{}))
				Expect(newVM.Spec.Template.Spec.Domain.Resources).To(Equal(kubevirtiov1.ResourceRequirements{}))
			})
		})
	})

	Describe("DevicesPermissionChecker", func() {
		var checker *DevicesPermissionChecker

		BeforeEach(func() {
			checker = &DevicesPermissionChecker{}
		})

		It("should have correct name and subresource", func() {
			Expect(checker.Name()).To(Equal("devices"))
			Expect(checker.Subresource()).To(Equal("virtualmachines/devices-admin"))
		})

		Context("HasChanged", func() {
			It("should detect GPU changes", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										GPUs: []kubevirtiov1.GPU{},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										GPUs: []kubevirtiov1.GPU{
											{Name: "gpu1"},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should detect host device changes", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										HostDevices: []kubevirtiov1.HostDevice{},
									},
								},
							},
						},
					},
				}

				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										HostDevices: []kubevirtiov1.HostDevice{
											{Name: "dev1"},
										},
									},
								},
							},
						},
					},
				}

				Expect(checker.HasChanged(oldVM, newVM)).To(BeTrue())
			})

			It("should not detect changes when devices are identical", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										GPUs: []kubevirtiov1.GPU{
											{Name: "gpu1"},
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
			It("should neutralize all device fields", func() {
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Template: &kubevirtiov1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtiov1.VirtualMachineInstanceSpec{
								Domain: kubevirtiov1.DomainSpec{
									Devices: kubevirtiov1.Devices{
										GPUs: []kubevirtiov1.GPU{
											{Name: "gpu1"},
										},
										HostDevices: []kubevirtiov1.HostDevice{
											{Name: "dev1"},
										},
									},
								},
							},
						},
					},
				}

				newVM := oldVM.DeepCopy()
				newVM.Spec.Template.Spec.Domain.Devices.GPUs = append(newVM.Spec.Template.Spec.Domain.Devices.GPUs, kubevirtiov1.GPU{Name: "gpu2"})

				checker.Neutralize(oldVM, newVM)

				Expect(oldVM.Spec.Template.Spec.Domain.Devices.GPUs).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Domain.Devices.GPUs).To(BeNil())
				Expect(oldVM.Spec.Template.Spec.Domain.Devices.HostDevices).To(BeNil())
				Expect(newVM.Spec.Template.Spec.Domain.Devices.HostDevices).To(BeNil())
			})
		})
	})

	Describe("LifecyclePermissionChecker", func() {
		var checker *LifecyclePermissionChecker

		BeforeEach(func() {
			checker = &LifecyclePermissionChecker{}
		})

		It("should have correct name and subresource", func() {
			Expect(checker.Name()).To(Equal("lifecycle"))
			Expect(checker.Subresource()).To(Equal("virtualmachines/lifecycle-admin"))
		})

		Context("HasChanged", func() {
			DescribeTable("should correctly detect lifecycle field changes",
				func(oldRunning *bool, oldStrategy *kubevirtiov1.VirtualMachineRunStrategy, newRunning *bool, newStrategy *kubevirtiov1.VirtualMachineRunStrategy, expectedChanged bool) {
					oldVM := &kubevirtiov1.VirtualMachine{
						Spec: kubevirtiov1.VirtualMachineSpec{
							Running:     oldRunning,
							RunStrategy: oldStrategy,
						},
					}

					newVM := &kubevirtiov1.VirtualMachine{
						Spec: kubevirtiov1.VirtualMachineSpec{
							Running:     newRunning,
							RunStrategy: newStrategy,
						},
					}

					Expect(checker.HasChanged(oldVM, newVM)).To(Equal(expectedChanged))
				},
				Entry("when spec.running changes from false to true", boolPtr(false), nil, boolPtr(true), nil, true),
				Entry("when spec.running changes from true to false", boolPtr(true), nil, boolPtr(false), nil, true),
				Entry("when spec.running changes from nil to true", nil, nil, boolPtr(true), nil, true),
				Entry("when spec.running changes from true to nil", boolPtr(true), nil, nil, nil, true),
				Entry("when spec.runStrategy changes from Always to Halted", nil, strategyPtr("Always"), nil, strategyPtr("Halted"), true),
				Entry("when spec.runStrategy changes from Always to Manual", nil, strategyPtr("Always"), nil, strategyPtr("Manual"), true),
				Entry("when spec.runStrategy changes from RerunOnFailure to Once", nil, strategyPtr("RerunOnFailure"), nil, strategyPtr("Once"), true),
				Entry("when spec.running is identical (true)", boolPtr(true), nil, boolPtr(true), nil, false),
				Entry("when spec.running is identical (nil)", nil, nil, nil, nil, false),
				Entry("when spec.runStrategy is identical", nil, strategyPtr("Always"), nil, strategyPtr("Always"), false),
				Entry("when both running and runStrategy are identical", boolPtr(true), strategyPtr("Always"), boolPtr(true), strategyPtr("Always"), false),
			)
		})

		Context("Neutralize", func() {
			It("should neutralize spec.running changes", func() {
				running := false
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Running: &running,
					},
				}

				runningNew := true
				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Running: &runningNew,
					},
				}

				checker.Neutralize(oldVM, newVM)

				Expect(oldVM.Spec.Running).To(BeNil())
				Expect(newVM.Spec.Running).To(BeNil())
			})

			It("should neutralize spec.runStrategy changes", func() {
				strategyAlways := kubevirtiov1.VirtualMachineRunStrategy("Always")
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						RunStrategy: &strategyAlways,
					},
				}

				strategyHalted := kubevirtiov1.VirtualMachineRunStrategy("Halted")
				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						RunStrategy: &strategyHalted,
					},
				}

				checker.Neutralize(oldVM, newVM)

				Expect(oldVM.Spec.RunStrategy).To(BeNil())
				Expect(newVM.Spec.RunStrategy).To(BeNil())
			})

			It("should neutralize both spec.running and spec.runStrategy", func() {
				running := false
				strategyAlways := kubevirtiov1.VirtualMachineRunStrategy("Always")
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Running:     &running,
						RunStrategy: &strategyAlways,
					},
				}

				runningNew := true
				strategyHalted := kubevirtiov1.VirtualMachineRunStrategy("Halted")
				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Running:     &runningNew,
						RunStrategy: &strategyHalted,
					},
				}

				checker.Neutralize(oldVM, newVM)

				Expect(oldVM.Spec.Running).To(BeNil())
				Expect(newVM.Spec.Running).To(BeNil())
				Expect(oldVM.Spec.RunStrategy).To(BeNil())
				Expect(newVM.Spec.RunStrategy).To(BeNil())
			})

			It("should make objects equal after neutralization when only lifecycle fields differ", func() {
				running := false
				oldVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Running: &running,
					},
				}

				runningNew := true
				newVM := &kubevirtiov1.VirtualMachine{
					Spec: kubevirtiov1.VirtualMachineSpec{
						Running: &runningNew,
					},
				}

				checker.Neutralize(oldVM, newVM)

				Expect(equality.Semantic.DeepEqual(oldVM.Spec, newVM.Spec)).To(BeTrue())
			})
		})
	})
})
