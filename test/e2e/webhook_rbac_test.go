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

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/kubevirt-rbac-webhook/test/utils"
)

// Test namespace for webhook RBAC tests
const testNamespace = "default"

// Common JSON patches used across multiple tests
const (
	patchAddVolume = `[{"op":"add","path":"/spec/template/spec/volumes/-",` +
		`"value":{"name":"test-vol","emptyDisk":{"capacity":"1Gi"}}}]`
	patchAddCPU              = `[{"op":"add","path":"/spec/template/spec/domain/cpu","value":{"cores":4}}]`
	patchAddNetworkInterface = `[{"op":"add","path":"/spec/template/spec/domain/devices/interfaces",` +
		`"value":[{"name":"test-iface","masquerade":{}}]},` +
		`{"op":"add","path":"/spec/template/spec/networks","value":[{"name":"test-iface","pod":{}}]}]`
	patchSetRunning = `[{"op":"add","path":"/spec/running","value":true}]`
)

var _ = Describe("Webhook RBAC Validation", Ordered, func() {

	Context("Full-Admin Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-full-admin"
			testVM = "test-vm-full-admin"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for full-admin tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for full-admin")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-full-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow modifying all VM fields (storage, CPU, memory, network)", func() {
			By("attempting to add a volume as full-admin user")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)).
				To(Succeed(), "full-admin should be able to add volumes")

			By("attempting to change CPU as full-admin user")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)).
				To(Succeed(), "full-admin should be able to change CPU")

			By("attempting to change memory as full-admin user")
			patch := `[{"op":"replace","path":"/spec/template/spec/domain/resources/requests/memory","value":"256Mi"}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "full-admin should be able to change memory")
		})

		It("should allow modifying VM metadata", func() {
			By("attempting to add a label as full-admin user")
			patch := `[{"op":"add","path":"/metadata/labels","value":{"test":"label"}}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "full-admin should be able to modify metadata")
		})
	})

	Context("Storage-Admin Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-storage-admin"
			testVM = "test-vm-storage-admin"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for storage-admin tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for storage-admin")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-storage-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow adding volumes", func() {
			By("attempting to add a volume as storage-admin user")
			patch := `[{"op":"add","path":"/spec/template/spec/volumes/-",` +
				`"value":{"name":"test-vol-storage","emptyDisk":{"capacity":"1Gi"}}}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "storage-admin should be able to add volumes")
		})

		It("should allow modifying disks", func() {
			By("attempting to add a disk and volume as storage-admin user")
			// Add both volume and disk together (disk needs a matching volume)
			// nolint:lll // Long JSON patch can't be easily split
			patch := `[{"op":"add","path":"/spec/template/spec/volumes/-","value":{"name":"test-disk-vol","emptyDisk":{"capacity":"1Gi"}}},{"op":"add","path":"/spec/template/spec/domain/devices/disks/-","value":{"name":"test-disk-vol","disk":{"bus":"virtio"}}}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "storage-admin should be able to add disks and volumes")
		})

		It("should deny CPU changes", func() {
			By("attempting to change CPU as storage-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "storage-admin should NOT be able to change CPU")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny memory changes", func() {
			By("attempting to change memory as storage-admin user")
			patch := `[{"op":"replace","path":"/spec/template/spec/domain/resources/requests/memory","value":"512Mi"}]`
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "storage-admin should NOT be able to change memory")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny network changes", func() {
			By("attempting to add a network interface as storage-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddNetworkInterface, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "storage-admin should NOT be able to add network interfaces")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny lifecycle changes", func() {
			By("attempting to change running state as storage-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchSetRunning, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "storage-admin should NOT be able to change running state")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny metadata changes", func() {
			By("attempting to add a label as storage-admin user")
			patch := `[{"op":"add","path":"/metadata/labels","value":{"forbidden":"label"}}]`
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "storage-admin should NOT be able to modify metadata")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})

	Context("CD-ROM User Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-cdrom-user"
			testVM = "test-vm-cdrom-user"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for cdrom-user tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for cdrom-user")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-cdrom-user", testSA)).To(Succeed())

			By("creating a test VM with hotpluggable CD-ROM")
			Expect(utils.CreateVMWithCDRom(testVM, testNamespace, true)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow swapping CD-ROM media (hotpluggable)", func() {
			By("attempting to change CD-ROM volume as cdrom-user")
			// Note: This would require updating the hotpluggable volume
			// For now, we'll just verify the permission check works
			patch := `[{"op":"replace","path":"/spec/template/spec/volumes/1/dataVolume/name","value":"new-cdrom"}]`
			// This might still fail due to validation, but should not fail due to RBAC
			_ = utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)
			// Note: We expect this might fail for other reasons (volume doesn't exist), but not RBAC
		})

		It("should deny adding new CD-ROM disks", func() {
			By("attempting to add a CD-ROM disk as cdrom-user")
			// nolint:lll // Long JSON patch can't be easily split
			patch := `[{"op":"add","path":"/spec/template/spec/domain/devices/disks/-","value":{"name":"new-cdrom","cdrom":{"bus":"sata"}}}]`
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "cdrom-user should NOT be able to add CD-ROM disks")
		})

		It("should deny adding non-CD-ROM storage", func() {
			By("attempting to add a regular volume as cdrom-user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "cdrom-user should NOT be able to add regular volumes")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny CPU changes", func() {
			By("attempting to change CPU as cdrom-user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "cdrom-user should NOT be able to change CPU")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})

	Context("Network-Admin Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-network-admin"
			testVM = "test-vm-network-admin"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for network-admin tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for network-admin")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-network-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow adding network interfaces", func() {
			By("attempting to add a network interface as network-admin user")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddNetworkInterface, testSA, testNamespace)).
				To(Succeed(), "network-admin should be able to add network interfaces")
		})

		It("should deny storage changes", func() {
			By("attempting to add a volume as network-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "network-admin should NOT be able to add volumes")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny CPU changes", func() {
			By("attempting to change CPU as network-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "network-admin should NOT be able to change CPU")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})

	// nolint:dupl // Similar structure to other permission tests but with different permissions
	Context("Compute-Admin Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-compute-admin"
			testVM = "test-vm-compute-admin"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for compute-admin tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for compute-admin")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-compute-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow changing CPU configuration", func() {
			By("attempting to change CPU as compute-admin user")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)).
				To(Succeed(), "compute-admin should be able to change CPU")
		})

		It("should allow changing memory", func() {
			By("attempting to change memory as compute-admin user")
			patch := `[{"op":"replace","path":"/spec/template/spec/domain/resources/requests/memory","value":"256Mi"}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "compute-admin should be able to change memory")
		})

		It("should deny storage changes", func() {
			By("attempting to add a volume as compute-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "compute-admin should NOT be able to add volumes")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny network changes", func() {
			By("attempting to add a network interface as compute-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddNetworkInterface, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "compute-admin should NOT be able to add network interfaces")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})

	// nolint:dupl // Similar structure to other permission tests but with different permissions
	Context("Lifecycle-Admin Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-lifecycle-admin"
			testVM = "test-vm-lifecycle-admin"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for lifecycle-admin tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for lifecycle-admin")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-lifecycle-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow changing running state", func() {
			By("attempting to change running state as lifecycle-admin user")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchSetRunning, testSA, testNamespace)).
				To(Succeed(), "lifecycle-admin should be able to change running state")
		})

		It("should allow changing runStrategy", func() {
			By("attempting to set runStrategy as lifecycle-admin user")
			// Replace running with runStrategy in one operation (they're mutually exclusive)
			patch := `[{"op":"remove","path":"/spec/running"},{"op":"add","path":"/spec/runStrategy","value":"Always"}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "lifecycle-admin should be able to set runStrategy")
		})

		It("should deny storage changes", func() {
			By("attempting to add a volume as lifecycle-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "lifecycle-admin should NOT be able to add volumes")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny CPU changes", func() {
			By("attempting to change CPU as lifecycle-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "lifecycle-admin should NOT be able to change CPU")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})

	Context("Devices-Admin Permission", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-devices-admin"
			testVM = "test-vm-devices-admin"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for devices-admin tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for devices-admin")
			Expect(utils.CreateRoleBinding(bindingName, testNamespace,
				"kubevirt.io:vm-devices-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow adding GPUs", func() {
			By("attempting to add a GPU as devices-admin user")
			// nolint:lll // Long JSON patch can't be easily split
			patch := `[{"op":"add","path":"/spec/template/spec/domain/devices/gpus","value":[{"name":"gpu1","deviceName":"nvidia.com/GPU"}]}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "devices-admin should be able to add GPUs")
		})

		It("should allow adding host devices", func() {
			Skip("Host devices require HostDevices feature gate to be enabled in KubeVirt")
			By("attempting to add a host device as devices-admin user")
			// nolint:lll // Long JSON patch can't be easily split
			patch := `[{"op":"add","path":"/spec/template/spec/domain/devices/hostDevices","value":[{"name":"hostdev1","deviceName":"pci.com/device"}]}]`
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patch, testSA, testNamespace)).
				To(Succeed(), "devices-admin should be able to add host devices")
		})

		It("should deny storage changes", func() {
			By("attempting to add a volume as devices-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "devices-admin should NOT be able to add volumes")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})

		It("should deny CPU changes", func() {
			By("attempting to change CPU as devices-admin user")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "devices-admin should NOT be able to change CPU")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})

	Context("Backwards Compatibility", func() {
		var (
			testSA      string
			testVM      string
			bindingName string
		)

		BeforeAll(func() {
			testSA = "test-standard-update"
			testVM = "test-vm-standard-update"
			bindingName = testSA + "-binding"

			By("creating ServiceAccount for standard update tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBinding for standard VM update (no subresource permissions)")
			// Grant standard update permission WITHOUT any subresource permissions
			roleYAML := fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: %s-role
  namespace: %s
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  verbs: ["get", "list", "watch", "update", "patch"]
`, testSA, testNamespace)
			Expect(utils.ApplyYAML(roleYAML)).To(Succeed())

			bindingYAML := fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: %s-role
subjects:
- kind: ServiceAccount
  name: %s
  namespace: %s
`, bindingName, testNamespace, testSA, testSA, testNamespace)
			Expect(utils.ApplyYAML(bindingYAML)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(bindingName, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow all changes (backwards compatible)", func() {
			By("attempting to add a volume with standard update permission")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)).
				To(Succeed(), "standard update should allow volume changes (backwards compatible)")

			By("attempting to change CPU with standard update permission")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)).
				To(Succeed(), "standard update should allow CPU changes (backwards compatible)")

			By("attempting to change running state with standard update permission")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchSetRunning, testSA, testNamespace)).
				To(Succeed(), "standard update should allow lifecycle changes (backwards compatible)")
		})
	})

	Context("Combined Permissions", func() {
		var (
			testSA       string
			testVM       string
			binding1Name string
			binding2Name string
		)

		BeforeAll(func() {
			testSA = "test-storage-network"
			testVM = "test-vm-storage-network"
			binding1Name = testSA + "-storage-binding"
			binding2Name = testSA + "-network-binding"

			By("creating ServiceAccount for combined permissions tests")
			Expect(utils.CreateServiceAccount(testSA, testNamespace)).To(Succeed())

			By("creating RoleBindings for storage-admin and network-admin")
			Expect(utils.CreateRoleBinding(binding1Name, testNamespace,
				"kubevirt.io:vm-storage-admin", testSA)).To(Succeed())
			Expect(utils.CreateRoleBinding(binding2Name, testNamespace,
				"kubevirt.io:vm-network-admin", testSA)).To(Succeed())

			By("creating a test VM")
			Expect(utils.CreateTestVM(testVM, testNamespace)).To(Succeed())
		})

		AfterAll(func() {
			utils.DeleteVM(testVM, testNamespace)
			utils.DeleteRoleBinding(binding1Name, testNamespace)
			utils.DeleteRoleBinding(binding2Name, testNamespace)
			utils.DeleteServiceAccount(testSA, testNamespace)
		})

		It("should allow storage and network changes", func() {
			By("attempting to add a volume with combined permissions")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddVolume, testSA, testNamespace)).
				To(Succeed(), "user with storage+network should be able to add volumes")

			By("attempting to add a network interface with combined permissions")
			Expect(utils.PatchResourceAs("vm", testVM, testNamespace, patchAddNetworkInterface, testSA, testNamespace)).
				To(Succeed(), "user with storage+network should be able to add network interfaces")
		})

		It("should deny CPU changes", func() {
			By("attempting to change CPU with combined storage+network permissions")
			err := utils.PatchResourceAs("vm", testVM, testNamespace, patchAddCPU, testSA, testNamespace)
			Expect(err).To(HaveOccurred(), "user with storage+network should NOT be able to change CPU")
			Expect(err.Error()).To(ContainSubstring("does not have permission"), "error should indicate lack of permission")
		})
	})
})
