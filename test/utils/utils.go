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

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	prometheusOperatorVersion = "v0.77.1"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.16.3"
	certmanagerURLTmpl = "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// getKubectl returns the kubectl command to use.
// In kubevirtci mode, it returns the path to kubectl.sh wrapper.
// Otherwise, it returns standard "kubectl".
func getKubectl() string {
	if os.Getenv("USE_KUBEVIRTCI") == "true" {
		// Use kubevirtci's kubectl.sh wrapper
		dir, _ := GetProjectDir()
		return filepath.Join(dir, "_kubevirtci", "cluster-up", "kubectl.sh")
	}
	return "kubectl"
}

// newKubectlCommand creates a kubectl exec.Command with the appropriate binary.
func newKubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(getKubectl(), args...)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := newKubectlCommand("create", "-f", url)
	_, err := Run(cmd)
	return err
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := newKubectlCommand("delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsPrometheusCRDsInstalled checks if any Prometheus CRDs are installed
// by verifying the existence of key CRDs related to Prometheus.
func IsPrometheusCRDsInstalled() bool {
	// List of common Prometheus CRDs
	prometheusCRDs := []string{
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"prometheusagents.monitoring.coreos.com",
	}

	cmd := newKubectlCommand("get", "crds", "-o", "custom-columns=NAME:.metadata.name")
	output, err := Run(cmd)
	if err != nil {
		return false
	}
	crdList := GetNonEmptyLines(output)
	for _, crd := range prometheusCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := newKubectlCommand("delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}

	// Delete leftover leases in kube-system (not cleaned by default)
	kubeSystemLeases := []string{
		"cert-manager-cainjector-leader-election",
		"cert-manager-controller",
	}
	for _, lease := range kubeSystemLeases {
		cmd = newKubectlCommand("delete", "lease", lease,
			"-n", "kube-system", "--ignore-not-found", "--force", "--grace-period=0")
		if _, err := Run(cmd); err != nil {
			warnError(err)
		}
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := newKubectlCommand("apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = newKubectlCommand("wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	if _, err := Run(cmd); err != nil {
		return err
	}

	// Wait for the webhook's TLS secret to be created and the webhook to be fully functional.
	// This is necessary because the deployment being "Available" doesn't mean the webhook
	// certificates are trusted yet.
	_, _ = fmt.Fprintf(GinkgoWriter, "Waiting for cert-manager webhook to be fully functional...\n")
	cmd = newKubectlCommand("wait", "--for=jsonpath={.webhooks[0].clientConfig.caBundle}",
		"validatingwebhookconfigurations.admissionregistration.k8s.io",
		"cert-manager-webhook",
		"--timeout", "2m",
	)
	// If the above wait fails, fall back to a simple sleep
	if _, err := Run(cmd); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Falling back to sleep to wait for webhook readiness...\n")
		cmd = exec.Command("sleep", "30")
		if _, err := Run(cmd); err != nil {
			return err
		}
	}

	return nil
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsCertManagerCRDsInstalled() bool {
	// List of common Cert Manager CRDs
	certManagerCRDs := []string{
		"certificates.cert-manager.io",
		"issuers.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"certificaterequests.cert-manager.io",
		"orders.acme.cert-manager.io",
		"challenges.acme.cert-manager.io",
	}

	// Execute the kubectl command to get all CRDs
	cmd := newKubectlCommand("get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}

	// Check if any of the Cert Manager CRDs are present
	crdList := GetNonEmptyLines(output)
	for _, crd := range certManagerCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	// false positive
	// nolint:gosec
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", filename, err)
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		return fmt.Errorf("unable to find the code %q to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		if _, err = out.WriteString(strings.TrimPrefix(scanner.Text(), prefix)); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err = out.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
	}

	if _, err = out.Write(content[idx+len(target):]); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	// false positive
	// nolint:gosec
	if err = os.WriteFile(filename, out.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", filename, err)
	}

	return nil
}

// CreateServiceAccount creates a ServiceAccount in the specified namespace
func CreateServiceAccount(name, namespace string) error {
	cmd := newKubectlCommand("create", "serviceaccount", name, "-n", namespace)
	_, err := Run(cmd)
	return err
}

// DeleteServiceAccount deletes a ServiceAccount from the specified namespace
func DeleteServiceAccount(name, namespace string) {
	cmd := newKubectlCommand("delete", "serviceaccount", name, "-n", namespace, "--ignore-not-found")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// CreateRoleBinding creates a RoleBinding in the specified namespace
func CreateRoleBinding(name, namespace, clusterRole, serviceAccount string) error {
	cmd := newKubectlCommand("create", "rolebinding", name,
		"--clusterrole="+clusterRole,
		"--serviceaccount="+namespace+":"+serviceAccount,
		"-n", namespace)
	_, err := Run(cmd)
	return err
}

// DeleteRoleBinding deletes a RoleBinding from the specified namespace
func DeleteRoleBinding(name, namespace string) {
	cmd := newKubectlCommand("delete", "rolebinding", name, "-n", namespace, "--ignore-not-found")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// CreateClusterRoleBinding creates a ClusterRoleBinding
func CreateClusterRoleBinding(name, clusterRole, serviceAccount, namespace string) error {
	cmd := newKubectlCommand("create", "clusterrolebinding", name,
		"--clusterrole="+clusterRole,
		"--serviceaccount="+namespace+":"+serviceAccount)
	_, err := Run(cmd)
	return err
}

// DeleteClusterRoleBinding deletes a ClusterRoleBinding
func DeleteClusterRoleBinding(name string) {
	cmd := newKubectlCommand("delete", "clusterrolebinding", name, "--ignore-not-found")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// KubectlAs runs a kubectl command with impersonation as a ServiceAccount
func KubectlAs(serviceAccount, namespace string, args ...string) *exec.Cmd {
	asUser := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount)
	kubectlArgs := []string{"--as=" + asUser}
	kubectlArgs = append(kubectlArgs, args...)
	return exec.Command(getKubectl(), kubectlArgs...)
}

// ApplyYAML applies a YAML string to the cluster
func ApplyYAML(yaml string) error {
	cmd := newKubectlCommand("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	_, err := Run(cmd)
	return err
}

// ApplyYAMLAs applies a YAML string to the cluster with impersonation
func ApplyYAMLAs(yaml, serviceAccount, namespace string) error {
	asUser := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount)
	cmd := exec.Command(getKubectl(), "--as="+asUser, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	_, err := Run(cmd)
	return err
}

// DeleteYAML deletes resources from a YAML string
func DeleteYAML(yaml string) {
	cmd := newKubectlCommand("delete", "-f", "-", "--ignore-not-found")
	cmd.Stdin = strings.NewReader(yaml)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// PatchResource patches a resource with a JSON patch
func PatchResource(resourceType, name, namespace, patch string) error {
	cmd := newKubectlCommand("patch", resourceType, name, "-n", namespace,
		"--type=json", "-p", patch)
	_, err := Run(cmd)
	return err
}

// PatchResourceAs patches a resource with a JSON patch using impersonation
func PatchResourceAs(resourceType, name, namespace, patch, serviceAccount, saNamespace string) error {
	asUser := fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, serviceAccount)
	cmd := exec.Command(getKubectl(), "--as="+asUser, "patch", resourceType, name, "-n", namespace,
		"--type=json", "-p", patch)
	_, err := Run(cmd)
	return err
}

// GetResource gets a resource and returns its YAML
func GetResource(resourceType, name, namespace string) (string, error) {
	cmd := newKubectlCommand("get", resourceType, name, "-n", namespace, "-o", "yaml")
	return Run(cmd)
}

// WaitForResource waits for a resource to exist
func WaitForResource(resourceType, name, namespace string, timeout string) error {
	cmd := newKubectlCommand("wait", "--for=condition=Ready",
		resourceType+"/"+name, "-n", namespace, "--timeout="+timeout)
	_, err := Run(cmd)
	return err
}

// CreateTestVM creates a basic test VirtualMachine
func CreateTestVM(name, namespace string) error {
	vmYAML := fmt.Sprintf(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: %s
  namespace: %s
spec:
  running: false
  template:
    metadata:
      labels:
        kubevirt.io/vm: %s
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: containerdisk
          - disk:
              bus: virtio
            name: cloudinitdisk
        resources:
          requests:
            memory: 128Mi
      volumes:
      - containerDisk:
          image: quay.io/containerdisks/fedora:latest
        name: containerdisk
      - cloudInitNoCloud:
          userData: |
            #cloud-config
            password: fedora
            chpasswd: { expire: False }
        name: cloudinitdisk
`, name, namespace, name)

	return ApplyYAML(vmYAML)
}

// DeleteVM deletes a VirtualMachine
func DeleteVM(name, namespace string) {
	cmd := newKubectlCommand("delete", "vm", name, "-n", namespace, "--ignore-not-found")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsKubeVirtCRDsInstalled checks if KubeVirt CRDs are installed
func IsKubeVirtCRDsInstalled() bool {
	cmd := newKubectlCommand("get", "crd", "virtualmachines.kubevirt.io")
	_, err := Run(cmd)
	return err == nil
}

// IsDeploymentAvailable checks if a deployment exists and is available
func IsDeploymentAvailable(name, namespace string) bool {
	cmd := newKubectlCommand("get", "deployment", name, "-n", namespace)
	_, err := Run(cmd)
	return err == nil
}

// CreateNamespace creates a namespace
func CreateNamespace(name string) error {
	cmd := newKubectlCommand("create", "namespace", name)
	_, err := Run(cmd)
	return err
}

// DeleteNamespace deletes a namespace
func DeleteNamespace(name string) {
	cmd := newKubectlCommand("delete", "namespace", name, "--ignore-not-found", "--timeout=60s")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// NamespaceExists checks if a namespace exists
func NamespaceExists(name string) bool {
	cmd := newKubectlCommand("get", "namespace", name)
	_, err := Run(cmd)
	return err == nil
}

// CreateVMWithCDRom creates a test VM with a CD-ROM drive
func CreateVMWithCDRom(name, namespace string, hotpluggable bool) error {
	hotplugStr := ""
	if hotpluggable {
		hotplugStr = "true"
	} else {
		hotplugStr = "false"
	}

	vmYAML := fmt.Sprintf(`
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: %s
  namespace: %s
spec:
  running: false
  template:
    metadata:
      labels:
        kubevirt.io/vm: %s
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: containerdisk
          - cdrom:
              bus: sata
            name: cdrom-0
        resources:
          requests:
            memory: 128Mi
      volumes:
      - containerDisk:
          image: quay.io/containerdisks/fedora:latest
        name: containerdisk
      - dataVolume:
          name: blank-cdrom
          hotpluggable: %s
        name: cdrom-0
`, name, namespace, name, hotplugStr)

	return ApplyYAML(vmYAML)
}
