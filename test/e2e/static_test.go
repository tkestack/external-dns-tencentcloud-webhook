package e2e_test

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Static credential mode", Label("static"), func() {

	Context("public zone", Label("public"), Ordered, func() {
		const release = "e2e-static-public"

		BeforeAll(func() {
			ensureCredentialSecret()
		})

		AfterAll(func() {
			deleteTestService("e2e-static-svc")
			helmUninstall(release)
			exec.Command("kubectl", "delete", "secret", "tencentcloud-credentials",
				"-n", "external-dns", "--ignore-not-found").Run()
		})

		It("should deploy successfully", func() {
			helmInstall(release, "test/e2e/fixtures/values-static-public.yaml")
			waitForPodReady(release)
		})

		It("should use static credential mode", func() {
			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("zone-type=public"))
			Expect(logs).NotTo(ContainSubstring("OIDC"))
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
			hostname := fmt.Sprintf("e2e-static.%s", domain)
			createTestService("e2e-static-svc", hostname)
			waitForServiceIP("e2e-static-svc", "default")
			waitForDNSRecord(release, hostname)
		})
	})

	Context("private zone", Label("private"), Ordered, func() {
		const release = "e2e-static-private"

		BeforeAll(func() {
			ensureCredentialSecret()
		})

		AfterAll(func() {
			deleteTestService("e2e-static-private-svc")
			helmUninstall(release)
		})

		It("should deploy successfully", func() {
			helmInstall(release, "test/e2e/fixtures/values-static-private.yaml")
			waitForPodReady(release)
		})

		It("should use static credential mode with private zone", func() {
			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("zone-type=private"))
			Expect(logs).NotTo(ContainSubstring("OIDC"))
		})

		It("should sync DNS records successfully", func() {
			Eventually(func() string {
				out, _ := runCmdAllowFail("kubectl", "logs", "-n", "external-dns",
					"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", release),
					"-c", "external-dns", "--tail=20")
				return out
			}, 2*time.Minute, 10*time.Second).Should(ContainSubstring("All records are already up to date"))
		})

		It("should create DNS record for ClusterIP service", func() {
			hostname := fmt.Sprintf("e2e-static-priv.%s", privateDomain)
			createClusterIPService("e2e-static-private-svc", hostname)
			waitForDNSRecord(release, hostname)
		})
	})
})
