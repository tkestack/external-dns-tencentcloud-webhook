package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

// projectRoot returns the absolute path to the project root directory.
func projectRoot() string {
	// test/e2e/ → project root is ../../
	dir, _ := os.Getwd()
	return filepath.Join(dir, "..", "..")
}

func helmInstall(release, valuesFile string, extraValues ...string) {
	root := projectRoot()
	args := []string{
		"upgrade", "--install", release, "external-dns/external-dns",
		"--namespace", "external-dns", "--create-namespace",
		"-f", filepath.Join(root, "deploy/helm/values-base.yaml"),
		"-f", filepath.Join(root, "deploy/helm/values-china.yaml"),
		"-f", filepath.Join(root, valuesFile),
		"--wait", "--timeout", "90s",
	}
	if imageTag != "" {
		args = append(args, "--set", "provider.webhook.image.tag="+imageTag,
			"--set", "provider.webhook.image.pullPolicy=Always")
	}
	args = append(args, extraValues...)
	runCmd("helm", args...)
}

func helmUninstall(release string) {
	cmd := exec.Command("helm", "uninstall", release, "-n", "external-dns")
	cmd.Run() // ignore error if not exists
}

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "command failed: %s %s\noutput: %s", name, strings.Join(args, " "), string(out))
	return string(out)
}

func runCmdAllowFail(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func waitForPodReady(release string) {
	Eventually(func() string {
		out, _ := runCmdAllowFail("kubectl", "get", "pods", "-n", "external-dns",
			"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", release),
			"-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
		return out
	}, 90*time.Second, 5*time.Second).Should(Equal("True"))
}

func getWebhookLogs(release string) string {
	out, _ := runCmdAllowFail("kubectl", "logs", "-n", "external-dns",
		"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", release),
		"-c", "webhook", "--tail=200")
	return out
}

func createTestService(name, hostname string) {
	createTestServiceWithType(name, hostname, "LoadBalancer")
}

func createClusterIPService(name, hostname string) {
	createTestServiceWithType(name, hostname, "ClusterIP")
}

func createTestServiceWithType(name, hostname, svcType string) {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: default
  annotations:
    external-dns.alpha.kubernetes.io/hostname: %s
spec:
  type: %s
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: e2e-test-nonexistent`, name, hostname, svcType)

	cmd := exec.Command("kubectl", "apply", "-n", "default", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "failed to create test service: %s", string(out))
}

func deleteTestService(name string) {
	exec.Command("kubectl", "delete", "svc", name, "-n", "default", "--ignore-not-found").Run()
}

func ensureCredentialSecret() {
	runCmd("bash", "-c", strings.Join([]string{
		"kubectl create ns external-dns --dry-run=client -o yaml | kubectl apply -f -",
		"&&",
		`SECRET_ID=$(python3 -c "import json;print(json.load(open('$HOME/.tccli/default.credential'))['secretId'])")`,
		"&&",
		`SECRET_KEY=$(python3 -c "import json;print(json.load(open('$HOME/.tccli/default.credential'))['secretKey'])")`,
		"&&",
		`kubectl -n external-dns create secret generic tencentcloud-credentials \
			--from-literal=TENCENTCLOUD_SECRET_ID="$SECRET_ID" \
			--from-literal=TENCENTCLOUD_SECRET_KEY="$SECRET_KEY" \
			--dry-run=client -o yaml | kubectl apply -f -`,
	}, " "))
}

func waitForDNSRecord(release, hostname string) {
	// Wait for webhook to process a DNS change containing the target hostname,
	// or for external-dns to confirm records are up to date (already synced)
	Eventually(func() bool {
		webhookLogs := getWebhookLogs(release)
		if strings.Contains(webhookLogs, hostname) {
			return true
		}
		// If external-dns says "up to date" after service was created, record exists
		dnsLogs, _ := runCmdAllowFail("kubectl", "logs", "-n", "external-dns",
			"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", release),
			"-c", "external-dns", "--tail=5")
		return strings.Contains(dnsLogs, "All records are already up to date")
	}, 3*time.Minute, 10*time.Second).Should(BeTrue())
}

func waitForServiceIP(name, namespace string) string {
	var ip string
	Eventually(func() string {
		out, _ := runCmdAllowFail("kubectl", "get", "svc", name, "-n", namespace,
			"-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
		ip = out
		return out
	}, 3*time.Minute, 5*time.Second).ShouldNot(BeEmpty())
	return ip
}

func createTestServiceWithLine(name, hostname, line string) {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: default
  annotations:
    external-dns.alpha.kubernetes.io/hostname: %s
    external-dns.alpha.kubernetes.io/set-identifier: "%s"
    external-dns.alpha.kubernetes.io/webhook-record-line: "%s"
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: e2e-test-nonexistent`, name, hostname, line, line)

	cmd := exec.Command("kubectl", "apply", "-n", "default", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "failed to create test service with line: %s", string(out))
}

func waitForStableSync(release string) {
	// Wait for two consecutive "up to date" messages to confirm no update loop
	Eventually(func() bool {
		out, _ := runCmdAllowFail("kubectl", "logs", "-n", "external-dns",
			"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", release),
			"-c", "external-dns", "--tail=5")
		return strings.Count(out, "All records are already up to date") >= 2
	}, 3*time.Minute, 10*time.Second).Should(BeTrue())
}
