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
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/kubevirt-rbac-webhook/test/utils"
)

var (
	// Optional Environment Variables:
	// - CERT_MANAGER_INSTALL_SKIP=true: Skips CertManager installation during test setup.
	// - USE_KUBEVIRTCI=true: Use kubevirtci cluster (assumes cluster is already running)
	// These variables are useful if CertManager is already installed or if running
	// against kubevirtci cluster, avoiding re-installation and conflicts.
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"
	useKubevirtci          = os.Getenv("USE_KUBEVIRTCI") == "true"

	// isCertManagerAlreadyInstalled will be set true when CertManager CRDs be found on the cluster
	isCertManagerAlreadyInstalled = false

	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	// For kubevirtci, this defaults to localhost:5000/kubevirt-rbac-webhook:devel
	projectImage = getProjectImage()
)

func getProjectImage() string {
	if img := os.Getenv("PROJECT_IMAGE"); img != "" {
		return img
	}
	if useKubevirtci {
		return "localhost:5000/kubevirt-rbac-webhook:devel"
	}
	return "example.com/kubevirt-rbac-webhook:v0.0.1"
}

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purpose of being used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting kubevirt-rbac-webhook integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	if useKubevirtci {
		By("running tests against kubevirtci cluster")
		_, _ = fmt.Fprintf(GinkgoWriter, "Using kubevirtci cluster with image: %s\n", projectImage)

		By("verifying KubeVirt is installed")
		// Note: utils.IsCertManagerCRDsInstalled() and other utils functions
		// automatically use kubectl.sh wrapper in kubevirtci mode
		if !utils.IsKubeVirtCRDsInstalled() {
			ExpectWithOffset(1, false).To(BeTrue(), "KubeVirt must be installed in kubevirtci cluster")
		}

		By("checking if cert-manager is installed")
		isCertManagerAlreadyInstalled = utils.IsCertManagerCRDsInstalled()
		if !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager not found, deployment may fail...\n")
		}

		// For kubevirtci, we assume the webhook is already deployed via cluster-sync
		// Just verify it's running
		By("verifying webhook is deployed")
		if !utils.IsDeploymentAvailable("controller-manager", namespace) {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: Webhook not deployed. Run 'make cluster-sync' first\n")
		}

		// Create dedicated test namespace for RBAC tests
		By("creating test namespace for webhook RBAC tests")
		testNs := "webhook-rbac-test"
		if !utils.NamespaceExists(testNs) {
			Expect(utils.CreateNamespace(testNs)).To(Succeed(), "Failed to create test namespace")
		}
	} else {
		// Original kind-based workflow
		By("building the manager(Operator) image")
		cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

		By("loading the manager(Operator) image on Kind")
		err = utils.LoadImageToKindClusterWithName(projectImage)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

		// The tests-e2e are intended to run on a temporary cluster that is created and destroyed for testing.
		// To prevent errors when tests run in environments with CertManager already installed,
		// we check for its presence before execution.
		// Setup CertManager before the suite if not skipped and if not already installed
		if !skipCertManagerInstall {
			By("checking if cert manager is installed already")
			isCertManagerAlreadyInstalled = utils.IsCertManagerCRDsInstalled()
			if !isCertManagerAlreadyInstalled {
				_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager...\n")
				Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
			}
		}
	}
})

var _ = AfterSuite(func() {
	if useKubevirtci {
		// Clean up test namespace
		By("cleaning up test namespace")
		testNs := "webhook-rbac-test"
		utils.DeleteNamespace(testNs)
	} else {
		// Teardown CertManager after the suite if not skipped and if it was not already installed
		if !skipCertManagerInstall && !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
			utils.UninstallCertManager()
		}
	}
})
