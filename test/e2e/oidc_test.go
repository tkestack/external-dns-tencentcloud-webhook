package e2e_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OIDC credential mode", Label("oidc"), func() {

	Context("public zone", Label("public"), Ordered, func() {
		const release = "e2e-oidc-public"
		var valuesFile string

		BeforeAll(func() {
			root := projectRoot()
			valuesFile = "test/e2e/fixtures/values-oidc-public.local.yaml"
			runCmd("bash", "-c", fmt.Sprintf(
				"OUTPUT_DIR=%s %s --auth oidc --zone public --domain %s",
				filepath.Join(root, "test/e2e/fixtures"),
				filepath.Join(root, "scripts/setup.sh"),
				domain))
			runCmd("mv",
				filepath.Join(root, "test/e2e/fixtures/values-public.local.yaml"),
				filepath.Join(root, valuesFile))
		})

		AfterAll(func() {
			root := projectRoot()
			deleteTestService("e2e-oidc-svc")
			helmUninstall(release)
			roleName := fmt.Sprintf("external-dns-%s-public", clusterID)
			exec.Command("bash", "-c", fmt.Sprintf(
				"tccli cam DeleteRole --RoleName %s 2>/dev/null || true", roleName)).Run()
			exec.Command("rm", "-f", filepath.Join(root, valuesFile)).Run()
		})

		It("should deploy successfully", func() {
			helmInstall(release, valuesFile)
			waitForPodReady(release)
		})

		It("should use OIDC credential mode", func() {
			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("Using OIDC credential mode"))
		})

		It("should report zone-type=public", func() {
			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("zone-type=public"))
		})

		It("should sync DNS records successfully", func() {
			Eventually(func() string {
				out, _ := runCmdAllowFail("kubectl", "logs", "-n", "external-dns",
					"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", release),
					"-c", "external-dns", "--tail=20")
				return out
			}, 2*time.Minute, 10*time.Second).Should(ContainSubstring("All records are already up to date"))
		})

		It("should create DNS record for LoadBalancer service", func() {
			hostname := fmt.Sprintf("e2e-oidc.%s", domain)
			createTestService("e2e-oidc-svc", hostname)
			waitForServiceIP("e2e-oidc-svc", "default")
			waitForDNSRecord(release, hostname)
		})
	})

	Context("private zone", Label("private"), Ordered, func() {
		const release = "e2e-oidc-private"
		var valuesFile string

		BeforeAll(func() {
			root := projectRoot()
			valuesFile = "test/e2e/fixtures/values-oidc-private.local.yaml"
			runCmd("bash", "-c", fmt.Sprintf(
				"OUTPUT_DIR=%s %s --auth oidc --zone private --domain %s",
				filepath.Join(root, "test/e2e/fixtures"),
				filepath.Join(root, "scripts/setup.sh"),
				privateDomain))
			runCmd("mv",
				filepath.Join(root, "test/e2e/fixtures/values-private.local.yaml"),
				filepath.Join(root, valuesFile))
		})

		AfterAll(func() {
			root := projectRoot()
			deleteTestService("e2e-oidc-private-svc")
			helmUninstall(release)
			roleName := fmt.Sprintf("external-dns-%s-private", clusterID)
			exec.Command("bash", "-c", fmt.Sprintf(
				"tccli cam DeleteRole --RoleName %s 2>/dev/null || true", roleName)).Run()
			exec.Command("rm", "-f", filepath.Join(root, valuesFile)).Run()
		})

		It("should deploy successfully", func() {
			helmInstall(release, valuesFile)
			waitForPodReady(release)
		})

		It("should use OIDC credential mode", func() {
			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("Using OIDC credential mode"))
		})

		It("should report zone-type=private", func() {
			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("zone-type=private"))
		})

		It("should create DNS record for ClusterIP service", func() {
			hostname := fmt.Sprintf("e2e-oidc-priv.%s", privateDomain)
			createClusterIPService("e2e-oidc-private-svc", hostname)
			waitForDNSRecord(release, hostname)
		})
	})
})
